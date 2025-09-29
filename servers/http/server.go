package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

// HTTPServer is the MCP server for outbound HTTP requests with an optional
// host allowlist, default Authorization header, and response size cap.
type HTTPServer struct {
	mcp             *server.MCPServer
	httpClient      *http.Client
	allowedHosts    map[string]struct{}
	defaultAuth     string
	userAgent       string
	maxResponseSize int64
}

func (h *HTTPServer) MCP() *server.MCPServer { return h.mcp }

// SetHTTPClient is exposed for tests that need to inject an httptest client.
func (h *HTTPServer) SetHTTPClient(c *http.Client) { h.httpClient = c }

// AllowedHosts returns a copy of the allowlist for logging/inspection.
func (h *HTTPServer) AllowedHosts() []string {
	out := make([]string, 0, len(h.allowedHosts))
	for k := range h.allowedHosts {
		out = append(out, k)
	}
	return out
}

// NewHTTPServer builds the server from environment configuration:
//
//	HTTP_DEFAULT_TIMEOUT   seconds, default 30
//	HTTP_MAX_RESPONSE_SIZE bytes, default 5MB
//	HTTP_AUTH_HEADER       optional default Authorization value
//	HTTP_USER_AGENT        default User-Agent string
//	HTTP_ALLOWED_HOSTS     comma-separated host allowlist; empty = no restriction
func NewHTTPServer() (*HTTPServer, error) {
	timeout := 30
	if v := os.Getenv("HTTP_DEFAULT_TIMEOUT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid HTTP_DEFAULT_TIMEOUT %q", v)
		}
		timeout = n
	}
	maxSize := int64(5 * 1024 * 1024)
	if v := os.Getenv("HTTP_MAX_RESPONSE_SIZE"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid HTTP_MAX_RESPONSE_SIZE %q", v)
		}
		maxSize = n
	}

	allowed := map[string]struct{}{}
	if v := strings.TrimSpace(os.Getenv("HTTP_ALLOWED_HOSTS")); v != "" {
		for _, h := range strings.Split(v, ",") {
			if h = strings.TrimSpace(strings.ToLower(h)); h != "" {
				allowed[h] = struct{}{}
			}
		}
	}

	ua := os.Getenv("HTTP_USER_AGENT")
	if ua == "" {
		ua = "go-mcp-servers/http/1.0"
	}

	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        20,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	mcp := server.NewMCPServer("http", "1.0.0", server.WithToolCapabilities(true))
	h := &HTTPServer{
		mcp:             mcp,
		httpClient:      client,
		allowedHosts:    allowed,
		defaultAuth:     os.Getenv("HTTP_AUTH_HEADER"),
		userAgent:       ua,
		maxResponseSize: maxSize,
	}
	h.registerTools()
	return h, nil
}

// hostAllowed checks the URL's host against the allowlist.
// An empty allowlist means anything goes.
func (h *HTTPServer) hostAllowed(rawURL string) error {
	if len(h.allowedHosts) == 0 {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return fmt.Errorf("missing host in URL")
	}
	if _, ok := h.allowedHosts[host]; ok {
		return nil
	}
	return fmt.Errorf("host %q not in HTTP_ALLOWED_HOSTS", host)
}
