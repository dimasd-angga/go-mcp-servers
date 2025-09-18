package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// registerTools wires every Postgres tool to the server.
func (p *PostgresServer) registerTools() {
	p.registerQueryRows()
	p.registerExecute()
	p.registerListTables()
	p.registerDescribeTable()
	p.registerListSchemas()
	p.registerGetTableIndexes()
	p.registerCountRows()
	p.registerExplainQuery()
}

// ----- query_rows --------------------------------------------------------

func (p *PostgresServer) registerQueryRows() {
	p.mcp.AddTool(
		mcp.NewTool("query_rows",
			mcp.WithDescription("Execute a read-only SELECT/WITH query and return rows as a JSON array. "+
				"Enforces PG_MAX_ROWS and PG_QUERY_TIMEOUT. Non-SELECT statements require PG_ALLOW_WRITE=true."),
			mcp.WithString("sql", mcp.Required(), mcp.Description("SQL statement")),
		),
		p.handleQueryRows,
	)
}

func (p *PostgresServer) handleQueryRows(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	q, _ := req.GetArguments()["sql"].(string)
	q = strings.TrimSpace(q)
	if q == "" {
		return mcp.NewToolResultError("sql is required"), nil
	}
	upper := strings.ToUpper(q)
	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") && !p.allowWrite {
		return mcp.NewToolResultError("only SELECT/WITH queries allowed; set PG_ALLOW_WRITE=true for mutations"), nil
	}

	qctx, cancel := p.queryCtx(ctx)
	defer cancel()

	// Append LIMIT for SELECT/WITH safety. We do not modify user-supplied
	// non-SELECT queries (they go through execute).
	limited := q
	if !strings.Contains(upper, " LIMIT ") {
		limited = fmt.Sprintf("%s LIMIT %d", q, p.maxRows)
	}
	rows, err := p.db.QueryContext(qctx, limited)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query: %v", err)), nil
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("columns: %v", err)), nil
	}

	results := make([]map[string]any, 0)
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scan: %v", err)), nil
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = normalize(vals[i])
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rows: %v", err)), nil
	}
	out, _ := json.MarshalIndent(results, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}

func normalize(v any) any {
	switch t := v.(type) {
	case []byte:
		return string(t)
	default:
		return t
	}
}

// ----- execute -----------------------------------------------------------

func (p *PostgresServer) registerExecute() {
	p.mcp.AddTool(
		mcp.NewTool("execute",
			mcp.WithDescription("Execute INSERT/UPDATE/DELETE or DDL. Returns rows affected. "+
				"Requires PG_ALLOW_WRITE=true."),
			mcp.WithString("sql", mcp.Required(), mcp.Description("SQL statement")),
		),
		p.handleExecute,
	)
}

func (p *PostgresServer) handleExecute(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !p.allowWrite {
		return mcp.NewToolResultError("writes disabled; set PG_ALLOW_WRITE=true"), nil
	}
	q, _ := req.GetArguments()["sql"].(string)
	if strings.TrimSpace(q) == "" {
		return mcp.NewToolResultError("sql is required"), nil
	}
	qctx, cancel := p.queryCtx(ctx)
	defer cancel()
	res, err := p.db.ExecContext(qctx, q)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("exec: %v", err)), nil
	}
	affected, _ := res.RowsAffected()
	return mcp.NewToolResultText(fmt.Sprintf(`{"rows_affected": %d}`, affected)), nil
}

// ----- list_tables -------------------------------------------------------

func (p *PostgresServer) registerListTables() {
	p.mcp.AddTool(
		mcp.NewTool("list_tables",
			mcp.WithDescription("List user tables in a schema. Excludes system schemas."),
			mcp.WithString("schema", mcp.Description("Schema name. Default 'public'.")),
		),
		p.handleListTables,
	)
}

func (p *PostgresServer) handleListTables(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	schema, _ := req.GetArguments()["schema"].(string)
	if schema == "" {
		schema = "public"
	}
	qctx, cancel := p.queryCtx(ctx)
	defer cancel()
	rows, err := p.db.QueryContext(qctx,
		`SELECT table_name FROM information_schema.tables
		 WHERE table_schema = $1 AND table_type = 'BASE TABLE'
		 ORDER BY table_name`, schema)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query: %v", err)), nil
	}
	defer rows.Close()
	names := make([]string, 0)
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scan: %v", err)), nil
		}
		names = append(names, n)
	}
	out, _ := json.MarshalIndent(names, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}

// ----- describe_table ----------------------------------------------------

func (p *PostgresServer) registerDescribeTable() {
	p.mcp.AddTool(
		mcp.NewTool("describe_table",
			mcp.WithDescription("Return column definitions for a table: name, type, nullable, default."),
			mcp.WithString("table", mcp.Required(), mcp.Description("Table name. May be schema-qualified as 'schema.table'.")),
		),
		p.handleDescribeTable,
	)
}

type columnInfo struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Nullable string  `json:"nullable"`
	Default  *string `json:"default,omitempty"`
}

func (p *PostgresServer) handleDescribeTable(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	raw, _ := req.GetArguments()["table"].(string)
	schema, tbl := "public", raw
	if parts := strings.SplitN(raw, ".", 2); len(parts) == 2 {
		schema, tbl = parts[0], parts[1]
	}
	qctx, cancel := p.queryCtx(ctx)
	defer cancel()
	rows, err := p.db.QueryContext(qctx,
		`SELECT column_name, data_type, is_nullable, column_default
		 FROM information_schema.columns
		 WHERE table_schema = $1 AND table_name = $2
		 ORDER BY ordinal_position`, schema, tbl)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query: %v", err)), nil
	}
	defer rows.Close()
	cols := make([]columnInfo, 0)
	for rows.Next() {
		var c columnInfo
		if err := rows.Scan(&c.Name, &c.Type, &c.Nullable, &c.Default); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scan: %v", err)), nil
		}
		cols = append(cols, c)
	}
	if len(cols) == 0 {
		return mcp.NewToolResultError(fmt.Sprintf("table not found: %s.%s", schema, tbl)), nil
	}
	out, _ := json.MarshalIndent(cols, "", "  ")
	return mcp.NewToolResultText(string(out)), nil
}

// ----- list_schemas ------------------------------------------------------

func (p *PostgresServer) registerListSchemas() {
	p.mcp.AddTool(
		mcp.NewTool("list_schemas",
			mcp.WithDescription("List user schemas in the database. Excludes pg_* and information_schema."),
		),
		p.handleListSchemas,
	)
}

func (p *PostgresServer) handleListSchemas(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	qctx, cancel := p.queryCtx(ctx)
	defer cancel()
	rows, err := p.db.QueryContext(qctx,
		`SELECT schema_name FROM information_schema.schemata
		 WHERE schema_name NOT LIKE 'pg_%' AND schema_name <> 'information_schema'
		 ORDER BY schema_name`)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query: %v", err)), nil
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scan: %v", err)), nil
		}
		out = append(out, n)
	}
	body, _ := json.MarshalIndent(out, "", "  ")
	return mcp.NewToolResultText(string(body)), nil
}

// ----- get_table_indexes -------------------------------------------------

func (p *PostgresServer) registerGetTableIndexes() {
	p.mcp.AddTool(
		mcp.NewTool("get_table_indexes",
			mcp.WithDescription("List indexes on a table including the index definition."),
			mcp.WithString("table", mcp.Required(), mcp.Description("Table name; optionally schema-qualified.")),
		),
		p.handleGetTableIndexes,
	)
}

type indexInfo struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
}

func (p *PostgresServer) handleGetTableIndexes(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	raw, _ := req.GetArguments()["table"].(string)
	schema, tbl := "public", raw
	if parts := strings.SplitN(raw, ".", 2); len(parts) == 2 {
		schema, tbl = parts[0], parts[1]
	}
	qctx, cancel := p.queryCtx(ctx)
	defer cancel()
	rows, err := p.db.QueryContext(qctx,
		`SELECT indexname, indexdef FROM pg_indexes
		 WHERE schemaname = $1 AND tablename = $2
		 ORDER BY indexname`, schema, tbl)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query: %v", err)), nil
	}
	defer rows.Close()
	out := make([]indexInfo, 0)
	for rows.Next() {
		var i indexInfo
		if err := rows.Scan(&i.Name, &i.Definition); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scan: %v", err)), nil
		}
		out = append(out, i)
	}
	body, _ := json.MarshalIndent(out, "", "  ")
	return mcp.NewToolResultText(string(body)), nil
}

// ----- count_rows --------------------------------------------------------

func (p *PostgresServer) registerCountRows() {
	p.mcp.AddTool(
		mcp.NewTool("count_rows",
			mcp.WithDescription("Return COUNT(*) for a table, optionally with a WHERE clause. "+
				"WHERE text is appended verbatim — only use trusted input."),
			mcp.WithString("table", mcp.Required(), mcp.Description("Table name; optionally schema-qualified.")),
			mcp.WithString("where", mcp.Description("Optional WHERE clause body, without the WHERE keyword.")),
		),
		p.handleCountRows,
	)
}

func (p *PostgresServer) handleCountRows(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	table, _ := args["table"].(string)
	where, _ := args["where"].(string)
	if !validIdent(table) {
		return mcp.NewToolResultError("invalid table name"), nil
	}
	q := fmt.Sprintf("SELECT COUNT(*)::bigint FROM %s", table)
	if w := strings.TrimSpace(where); w != "" {
		q += " WHERE " + w
	}
	qctx, cancel := p.queryCtx(ctx)
	defer cancel()
	var n int64
	if err := p.db.QueryRowContext(qctx, q).Scan(&n); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("count: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf(`{"count": %d}`, n)), nil
}

// validIdent accepts simple identifiers and optionally schema-qualified
// identifiers, both made of letters, digits, and underscores.
func validIdent(s string) bool {
	if s == "" {
		return false
	}
	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for i, r := range p {
			isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_'
			isDigit := r >= '0' && r <= '9'
			if !isLetter && !(i > 0 && isDigit) {
				return false
			}
		}
	}
	return true
}

// ----- explain_query -----------------------------------------------------

func (p *PostgresServer) registerExplainQuery() {
	p.mcp.AddTool(
		mcp.NewTool("explain_query",
			mcp.WithDescription("Return EXPLAIN (FORMAT JSON) for a query without executing it. "+
				"Read-only — never adds ANALYZE."),
			mcp.WithString("sql", mcp.Required(), mcp.Description("Query to explain")),
		),
		p.handleExplainQuery,
	)
}

func (p *PostgresServer) handleExplainQuery(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	q, _ := req.GetArguments()["sql"].(string)
	if strings.TrimSpace(q) == "" {
		return mcp.NewToolResultError("sql is required"), nil
	}
	qctx, cancel := p.queryCtx(ctx)
	defer cancel()
	row := p.db.QueryRowContext(qctx, "EXPLAIN (FORMAT JSON) "+q)
	var out string
	if err := row.Scan(&out); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("explain: %v", err)), nil
	}
	return mcp.NewToolResultText(out), nil
}
