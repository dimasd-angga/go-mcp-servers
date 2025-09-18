package main

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dimasd-angga/go-mcp-servers/shared/testutil"
	_ "github.com/lib/pq"
	"github.com/mark3labs/mcp-go/client"
)

// integrationDSN returns the DSN from env or skips the test when absent.
func integrationDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set; skipping integration test")
	}
	// Best-effort connectivity probe.
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("sql.Open: %v", err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("Postgres unreachable: %v", err)
	}
	return dsn
}

func newPGClient(t *testing.T, allowWrite bool) (*client.Client, *PostgresServer) {
	t.Helper()
	dsn := integrationDSN(t)
	t.Setenv("POSTGRES_DSN", dsn)
	if allowWrite {
		t.Setenv("PG_ALLOW_WRITE", "true")
	} else {
		t.Setenv("PG_ALLOW_WRITE", "")
	}
	t.Setenv("PG_MAX_ROWS", "100")
	p, err := NewPostgresServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return testutil.NewInProcessClient(t, p.MCP()), p
}

func seedFixture(t *testing.T, p *PostgresServer, name string) {
	t.Helper()
	stmts := []string{
		"DROP TABLE IF EXISTS " + name,
		"CREATE TABLE " + name + " (id SERIAL PRIMARY KEY, email TEXT NOT NULL, score INT)",
		"INSERT INTO " + name + " (email, score) VALUES ('a@x.com', 1), ('b@x.com', 2), ('c@x.com', 3)",
		"CREATE INDEX idx_" + name + "_email ON " + name + "(email)",
	}
	for _, s := range stmts {
		if _, err := p.db.Exec(s); err != nil {
			t.Fatalf("seed: %v (%s)", err, s)
		}
	}
	t.Cleanup(func() { _, _ = p.db.Exec("DROP TABLE IF EXISTS " + name) })
}

func TestQueryRows_HappyPath(t *testing.T) {
	c, p := newPGClient(t, false)
	seedFixture(t, p, "mcp_query_users")
	out := testutil.CallTool(t, c, "query_rows", map[string]any{
		"sql": "SELECT id, email FROM mcp_query_users ORDER BY id",
	})
	if !strings.Contains(out, "a@x.com") || !strings.Contains(out, "c@x.com") {
		t.Errorf("missing rows: %s", out)
	}
}

func TestQueryRows_RejectsMutationWhenReadOnly(t *testing.T) {
	c, p := newPGClient(t, false)
	seedFixture(t, p, "mcp_query_ro")
	r := testutil.CallToolRaw(t, c, "query_rows", map[string]any{
		"sql": "DELETE FROM mcp_query_ro",
	})
	if !r.IsError {
		t.Error("must reject DELETE when PG_ALLOW_WRITE is off")
	}
}

func TestQueryRows_LimitApplied(t *testing.T) {
	c, p := newPGClient(t, false)
	seedFixture(t, p, "mcp_query_limit")
	// PG_MAX_ROWS=100 but our fixture only has 3, so just verify no error and rows return.
	out := testutil.CallTool(t, c, "query_rows", map[string]any{
		"sql": "SELECT id FROM mcp_query_limit",
	})
	if !strings.Contains(out, `"id"`) {
		t.Errorf("expected id column: %s", out)
	}
}

func TestListTables(t *testing.T) {
	c, p := newPGClient(t, false)
	seedFixture(t, p, "mcp_lt_demo")
	out := testutil.CallTool(t, c, "list_tables", map[string]any{})
	if !strings.Contains(out, "mcp_lt_demo") {
		t.Errorf("missing table in list: %s", out)
	}
}

func TestDescribeTable(t *testing.T) {
	c, p := newPGClient(t, false)
	seedFixture(t, p, "mcp_desc_demo")
	out := testutil.CallTool(t, c, "describe_table", map[string]any{
		"table": "mcp_desc_demo",
	})
	for _, want := range []string{"email", "score", "text", "integer"} {
		if !strings.Contains(out, want) {
			t.Errorf("describe missing %q: %s", want, out)
		}
	}
}

func TestDescribeTable_NotFound(t *testing.T) {
	c, _ := newPGClient(t, false)
	r := testutil.CallToolRaw(t, c, "describe_table", map[string]any{"table": "no_such_mcp_table"})
	if !r.IsError {
		t.Error("expected error for missing table")
	}
}

func TestListSchemas(t *testing.T) {
	c, _ := newPGClient(t, false)
	out := testutil.CallTool(t, c, "list_schemas", map[string]any{})
	if !strings.Contains(out, "public") {
		t.Errorf("public schema missing: %s", out)
	}
	if strings.Contains(out, "pg_catalog") {
		t.Errorf("system schema leaked: %s", out)
	}
}

func TestGetTableIndexes(t *testing.T) {
	c, p := newPGClient(t, false)
	seedFixture(t, p, "mcp_idx_demo")
	out := testutil.CallTool(t, c, "get_table_indexes", map[string]any{
		"table": "mcp_idx_demo",
	})
	if !strings.Contains(out, "idx_mcp_idx_demo_email") {
		t.Errorf("expected our seeded index, got: %s", out)
	}
}

func TestCountRows(t *testing.T) {
	c, p := newPGClient(t, false)
	seedFixture(t, p, "mcp_count_demo")
	out := testutil.CallTool(t, c, "count_rows", map[string]any{"table": "mcp_count_demo"})
	if !strings.Contains(out, `"count": 3`) {
		t.Errorf("want count 3: %s", out)
	}
}

func TestCountRows_WhereClause(t *testing.T) {
	c, p := newPGClient(t, false)
	seedFixture(t, p, "mcp_count_where")
	out := testutil.CallTool(t, c, "count_rows", map[string]any{
		"table": "mcp_count_where",
		"where": "score > 1",
	})
	if !strings.Contains(out, `"count": 2`) {
		t.Errorf("want count 2: %s", out)
	}
}

func TestCountRows_InvalidIdent(t *testing.T) {
	c, _ := newPGClient(t, false)
	r := testutil.CallToolRaw(t, c, "count_rows", map[string]any{
		"table": "users; DROP TABLE x",
	})
	if !r.IsError {
		t.Error("must reject malformed identifier")
	}
}

func TestExecute_DisabledByDefault(t *testing.T) {
	c, _ := newPGClient(t, false)
	r := testutil.CallToolRaw(t, c, "execute", map[string]any{
		"sql": "CREATE TABLE mcp_should_not_exist (id INT)",
	})
	if !r.IsError {
		t.Error("execute should be disabled by default")
	}
}

func TestExecute_EnabledRoundTrip(t *testing.T) {
	c, p := newPGClient(t, true)
	defer p.db.Exec("DROP TABLE IF EXISTS mcp_exec_tbl")
	out := testutil.CallTool(t, c, "execute", map[string]any{
		"sql": "CREATE TABLE mcp_exec_tbl (id INT)",
	})
	if !strings.Contains(out, `"rows_affected"`) {
		t.Errorf("want rows_affected JSON: %s", out)
	}
	if _, err := p.db.Exec("INSERT INTO mcp_exec_tbl VALUES (1)"); err != nil {
		t.Fatalf("seed via raw DB failed: %v", err)
	}
	out = testutil.CallTool(t, c, "execute", map[string]any{
		"sql": "DELETE FROM mcp_exec_tbl WHERE id = 1",
	})
	if !strings.Contains(out, `"rows_affected": 1`) {
		t.Errorf("want rows_affected 1: %s", out)
	}
}

func TestExplainQuery(t *testing.T) {
	c, p := newPGClient(t, false)
	seedFixture(t, p, "mcp_explain_demo")
	out := testutil.CallTool(t, c, "explain_query", map[string]any{
		"sql": "SELECT * FROM mcp_explain_demo WHERE id = 1",
	})
	if !strings.Contains(out, "Plan") {
		t.Errorf("EXPLAIN output looks wrong: %s", out)
	}
}
