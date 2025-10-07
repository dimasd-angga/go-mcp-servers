package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

// HAServer is the MCP server for Home Assistant operations via the HA REST API.
type HAServer struct {
	mcp        *server.MCPServer
	baseURL    string
	token      string
	httpClient *http.Client
}

func (h *HAServer) MCP() *server.MCPServer { return h.mcp }
func (h *HAServer) BaseURL() string         { return h.baseURL }

// SetHTTPClient lets tests inject an httptest-backed client.
func (h *HAServer) SetHTTPClient(c *http.Client) { h.httpClient = c }

// NewHAServer reads HA_URL, HA_TOKEN, HA_TIMEOUT.
func NewHAServer() (*HAServer, error) {
	base := strings.TrimRight(os.Getenv("HA_URL"), "/")
	if base == "" {
		return nil, fmt.Errorf("HA_URL environment variable is required")
	}
	token := os.Getenv("HA_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("HA_TOKEN environment variable is required")
	}
	timeout := 10
	if v := os.Getenv("HA_TIMEOUT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid HA_TIMEOUT %q", v)
		}
		timeout = n
	}

	mcp := server.NewMCPServer("homeassistant", "1.0.0", server.WithToolCapabilities(true))
	h := &HAServer{
		mcp:        mcp,
		baseURL:    base,
		token:      token,
		httpClient: &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}
	h.registerTools()
	return h, nil
}

// callAPI performs an authenticated request against the HA REST API.
// path begins with "/" — for example "/api/states".
func (h *HAServer) callAPI(ctx context.Context, method, path string, body any) (int, []byte, error) {
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("marshal: %w", err)
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, h.baseURL+path, rdr)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, data, nil
}
