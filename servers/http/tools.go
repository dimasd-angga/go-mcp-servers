package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"golang.org/x/net/html"
)

func (h *HTTPServer) registerTools() {
	h.addRequest("http_get", "GET")
	h.addRequest("http_post", "POST")
	h.addRequest("http_put", "PUT")
	h.addRequest("http_delete", "DELETE")
	h.addCustomRequest()
	h.addParseJSON()
	h.addParseHTML()
}

type httpResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	BodyLen int               `json:"body_len"`
	Truncated bool            `json:"truncated,omitempty"`
}

func (h *HTTPServer) addRequest(toolName, method string) {
	opts := []mcp.ToolOption{
		mcp.WithDescription(fmt.Sprintf("Make a %s request. Optional JSON body and headers. "+
			"Subject to HTTP_ALLOWED_HOSTS allowlist and HTTP_DEFAULT_TIMEOUT.", method)),
		mcp.WithString("url", mcp.Required(), mcp.Description("Full URL")),
	}
	if method == "POST" || method == "PUT" {
		opts = append(opts,
			mcp.WithString("body", mcp.Description("Request body, typically JSON.")),
		)
	}
	opts = append(opts, mcp.WithString("headers", mcp.Description("Optional JSON object of extra headers.")))
	h.mcp.AddTool(mcp.NewTool(toolName, opts...),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return h.doRequest(ctx, method, req.GetArguments())
		},
	)
}

func (h *HTTPServer) addCustomRequest() {
	h.mcp.AddTool(
		mcp.NewTool("http_request",
			mcp.WithDescription("Generic HTTP request. Specify method, URL, body, headers."),
			mcp.WithString("method", mcp.Required(), mcp.Description("GET, POST, PUT, DELETE, PATCH, ...")),
			mcp.WithString("url", mcp.Required()),
			mcp.WithString("body", mcp.Description("Request body")),
			mcp.WithString("headers", mcp.Description("Optional JSON object of headers")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			method, _ := args["method"].(string)
			method = strings.ToUpper(strings.TrimSpace(method))
			if method == "" {
				return mcp.NewToolResultError("method required"), nil
			}
			return h.doRequest(ctx, method, args)
		},
	)
}

func (h *HTTPServer) doRequest(ctx context.Context, method string, args map[string]any) (*mcp.CallToolResult, error) {
	rawURL, _ := args["url"].(string)
	if rawURL == "" {
		return mcp.NewToolResultError("url required"), nil
	}
	if err := h.hostAllowed(rawURL); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var bodyReader io.Reader
	if b, ok := args["body"].(string); ok && b != "" {
		bodyReader = bytes.NewBufferString(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("new request: %v", err)), nil
	}
	req.Header.Set("User-Agent", h.userAgent)
	if h.defaultAuth != "" {
		req.Header.Set("Authorization", h.defaultAuth)
	}
	if bodyReader != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	if headersJSON, ok := args["headers"].(string); ok && headersJSON != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid headers JSON: %v", err)), nil
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("request: %v", err)), nil
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, h.maxResponseSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("read body: %v", err)), nil
	}
	truncated := int64(len(data)) > h.maxResponseSize
	if truncated {
		data = data[:h.maxResponseSize]
	}

	out := httpResponse{
		Status:    resp.StatusCode,
		Headers:   make(map[string]string, len(resp.Header)),
		Body:      string(data),
		BodyLen:   len(data),
		Truncated: truncated,
	}
	for k, v := range resp.Header {
		if len(v) > 0 {
			out.Headers[k] = v[0]
		}
	}

	body, _ := json.MarshalIndent(out, "", "  ")
	return mcp.NewToolResultText(string(body)), nil
}

// ----- parse_json -------------------------------------------------------

func (h *HTTPServer) addParseJSON() {
	h.mcp.AddTool(
		mcp.NewTool("parse_json",
			mcp.WithDescription("Parse a JSON string. Optionally extract a value at a dotted path "+
				"(e.g. 'data.items.0.name'). Returns the pretty-printed JSON of the matched value."),
			mcp.WithString("input", mcp.Required(), mcp.Description("JSON string to parse")),
			mcp.WithString("path", mcp.Description("Optional dotted path; '.0' indexes a list.")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			input, _ := args["input"].(string)
			var v any
			if err := json.Unmarshal([]byte(input), &v); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("parse: %v", err)), nil
			}
			path, _ := args["path"].(string)
			if path != "" {
				val, err := traversePath(v, path)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				v = val
			}
			body, _ := json.MarshalIndent(v, "", "  ")
			return mcp.NewToolResultText(string(body)), nil
		},
	)
}

func traversePath(v any, path string) (any, error) {
	for _, seg := range strings.Split(path, ".") {
		if seg == "" {
			continue
		}
		switch cur := v.(type) {
		case map[string]any:
			next, ok := cur[seg]
			if !ok {
				return nil, fmt.Errorf("path: key %q not found", seg)
			}
			v = next
		case []any:
			var idx int
			if _, err := fmt.Sscanf(seg, "%d", &idx); err != nil {
				return nil, fmt.Errorf("path: expected list index at %q", seg)
			}
			if idx < 0 || idx >= len(cur) {
				return nil, fmt.Errorf("path: index %d out of bounds", idx)
			}
			v = cur[idx]
		default:
			return nil, fmt.Errorf("path: cannot descend into %T at %q", v, seg)
		}
	}
	return v, nil
}

// ----- parse_html -------------------------------------------------------

func (h *HTTPServer) addParseHTML() {
	h.mcp.AddTool(
		mcp.NewTool("parse_html",
			mcp.WithDescription("Extract visible text content from HTML, stripping tags and "+
				"script/style content. Whitespace is normalized."),
			mcp.WithString("input", mcp.Required(), mcp.Description("HTML string")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			input, _ := req.GetArguments()["input"].(string)
			text, err := extractText(input)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(text), nil
		},
	)
}

func extractText(htmlStr string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return "", fmt.Errorf("parse html: %w", err)
	}
	var buf bytes.Buffer
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "noscript":
				return
			}
		}
		if n.Type == html.TextNode {
			t := strings.TrimSpace(n.Data)
			if t != "" {
				if buf.Len() > 0 {
					buf.WriteByte(' ')
				}
				buf.WriteString(t)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(doc)
	return buf.String(), nil
}
