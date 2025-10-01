package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dimasd-angga/go-mcp-servers/shared/testutil"
	"github.com/mark3labs/mcp-go/client"
)

func newHClient(t *testing.T, allowedHosts string) (*client.Client, *HTTPServer) {
	t.Helper()
	t.Setenv("HTTP_ALLOWED_HOSTS", allowedHosts)
	t.Setenv("HTTP_DEFAULT_TIMEOUT", "5")
	h, err := NewHTTPServer()
	if err != nil {
		t.Fatal(err)
	}
	return testutil.NewInProcessClient(t, h.MCP()), h
}

func TestHTTPGet_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "ok")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	host = strings.Split(host, ":")[0]
	c, _ := newHClient(t, host)

	out := testutil.CallTool(t, c, "http_get", map[string]any{"url": srv.URL + "/x"})
	var resp httpResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != 200 {
		t.Errorf("status: %d", resp.Status)
	}
	if !strings.Contains(resp.Body, `"ok"`) {
		t.Errorf("body: %s", resp.Body)
	}
	if resp.Headers["X-Test"] != "ok" {
		t.Errorf("header missing: %v", resp.Headers)
	}
}

func TestHTTPGet_HostBlocked(t *testing.T) {
	c, _ := newHClient(t, "only-this-host.example")
	r := testutil.CallToolRaw(t, c, "http_get", map[string]any{
		"url": "https://other.example.com/",
	})
	if !r.IsError {
		t.Error("expected block on disallowed host")
	}
}

func TestHTTPPost_BodyAndHeaders(t *testing.T) {
	var seenBody, seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 1024)
		n, _ := r.Body.Read(b)
		seenBody = string(b[:n])
		seenAuth = r.Header.Get("X-Custom")
		w.WriteHeader(201)
		_, _ = w.Write([]byte("created"))
	}))
	defer srv.Close()
	host := strings.Split(strings.TrimPrefix(srv.URL, "http://"), ":")[0]
	c, _ := newHClient(t, host)

	out := testutil.CallTool(t, c, "http_post", map[string]any{
		"url":     srv.URL + "/items",
		"body":    `{"name":"thing"}`,
		"headers": `{"X-Custom":"yes"}`,
	})
	var resp httpResponse
	_ = json.Unmarshal([]byte(out), &resp)
	if resp.Status != 201 {
		t.Errorf("status: %d", resp.Status)
	}
	if seenBody != `{"name":"thing"}` {
		t.Errorf("body seen: %q", seenBody)
	}
	if seenAuth != "yes" {
		t.Errorf("custom header: %q", seenAuth)
	}
}

func TestHTTPRequest_Generic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("want PATCH, got %s", r.Method)
		}
		w.WriteHeader(204)
	}))
	defer srv.Close()
	host := strings.Split(strings.TrimPrefix(srv.URL, "http://"), ":")[0]
	c, _ := newHClient(t, host)

	out := testutil.CallTool(t, c, "http_request", map[string]any{
		"method": "patch",
		"url":    srv.URL + "/x",
	})
	if !strings.Contains(out, `"status": 204`) {
		t.Errorf("expected 204 status, got %s", out)
	}
}

func TestParseJSON_RootAndPath(t *testing.T) {
	c, _ := newHClient(t, "")
	root := testutil.CallTool(t, c, "parse_json", map[string]any{
		"input": `{"a": 1, "b": ["x", "y"]}`,
	})
	if !strings.Contains(root, `"a": 1`) {
		t.Errorf("root parse failed: %s", root)
	}
	deep := testutil.CallTool(t, c, "parse_json", map[string]any{
		"input": `{"a": 1, "b": ["x", "y"]}`,
		"path":  "b.1",
	})
	if !strings.Contains(deep, `"y"`) {
		t.Errorf("path extract failed: %s", deep)
	}
}

func TestParseJSON_InvalidJSON(t *testing.T) {
	c, _ := newHClient(t, "")
	r := testutil.CallToolRaw(t, c, "parse_json", map[string]any{"input": "{not json"})
	if !r.IsError {
		t.Error("invalid JSON should error")
	}
}

func TestParseHTML_StripsTags(t *testing.T) {
	c, _ := newHClient(t, "")
	got := testutil.CallTool(t, c, "parse_html", map[string]any{
		"input": "<h1>Title</h1><p>Body</p><style>.x{}</style>",
	})
	if got != "Title Body" {
		t.Errorf("want 'Title Body', got %q", got)
	}
}

func TestMaxResponseSize_Truncates(t *testing.T) {
	t.Setenv("HTTP_MAX_RESPONSE_SIZE", "10")
	t.Setenv("HTTP_ALLOWED_HOSTS", "")
	h, err := NewHTTPServer()
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("this body is more than ten bytes long"))
	}))
	defer srv.Close()
	c := testutil.NewInProcessClient(t, h.MCP())
	out := testutil.CallTool(t, c, "http_get", map[string]any{"url": srv.URL + "/"})
	var resp httpResponse
	_ = json.Unmarshal([]byte(out), &resp)
	if !resp.Truncated {
		t.Errorf("expected truncated=true: %+v", resp)
	}
	if resp.BodyLen != 10 {
		t.Errorf("body should be capped at 10, got %d", resp.BodyLen)
	}
}
