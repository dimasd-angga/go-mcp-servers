package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func (h *HAServer) registerTools() {
	h.addGetStates()
	h.addGetState()
	h.addGetAttributes()
	h.addCallService()
	h.addGetHistory()
	h.addListAutomations()
	h.addToggleAutomation()
	h.addFireEvent()
}

type haEntity struct {
	EntityID    string         `json:"entity_id"`
	State       string         `json:"state"`
	Attributes  map[string]any `json:"attributes,omitempty"`
	LastChanged string         `json:"last_changed,omitempty"`
}

// ----- get_states -------------------------------------------------------

func (h *HAServer) addGetStates() {
	h.mcp.AddTool(
		mcp.NewTool("get_states",
			mcp.WithDescription("Return entity states. Optionally filter by domain (e.g. 'light', 'switch')."),
			mcp.WithString("domain", mcp.Description("Optional domain filter")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			domain, _ := req.GetArguments()["domain"].(string)
			status, body, err := h.getStates(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("api: %v", err)), nil
			}
			if status != 200 {
				return mcp.NewToolResultError(fmt.Sprintf("HA returned %d: %s", status, string(body))), nil
			}
			var entities []haEntity
			if err := json.Unmarshal(body, &entities); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("decode: %v", err)), nil
			}
			if domain != "" {
				prefix := domain + "."
				filtered := entities[:0]
				for _, e := range entities {
					if strings.HasPrefix(e.EntityID, prefix) {
						filtered = append(filtered, e)
					}
				}
				entities = filtered
			}
			out, _ := json.MarshalIndent(entities, "", "  ")
			return mcp.NewToolResultText(string(out)), nil
		},
	)
}

// ----- get_state --------------------------------------------------------

func (h *HAServer) addGetState() {
	h.mcp.AddTool(
		mcp.NewTool("get_state",
			mcp.WithDescription("Return the state of a single entity by entity_id (e.g. 'light.kitchen')."),
			mcp.WithString("entity_id", mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, _ := req.GetArguments()["entity_id"].(string)
			if id == "" {
				return mcp.NewToolResultError("entity_id required"), nil
			}
			status, body, err := h.callAPI(ctx, "GET", "/api/states/"+url.PathEscape(id), nil)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("api: %v", err)), nil
			}
			if status == 404 {
				return mcp.NewToolResultError("entity not found: " + id), nil
			}
			if status != 200 {
				return mcp.NewToolResultError(fmt.Sprintf("HA returned %d: %s", status, string(body))), nil
			}
			return mcp.NewToolResultText(string(body)), nil
		},
	)
}

// ----- get_attributes ---------------------------------------------------

func (h *HAServer) addGetAttributes() {
	h.mcp.AddTool(
		mcp.NewTool("get_attributes",
			mcp.WithDescription("Return only the attributes object for an entity."),
			mcp.WithString("entity_id", mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, _ := req.GetArguments()["entity_id"].(string)
			status, body, err := h.callAPI(ctx, "GET", "/api/states/"+url.PathEscape(id), nil)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("api: %v", err)), nil
			}
			if status == 404 {
				return mcp.NewToolResultError("entity not found: " + id), nil
			}
			if status != 200 {
				return mcp.NewToolResultError(fmt.Sprintf("HA returned %d", status)), nil
			}
			var e haEntity
			if err := json.Unmarshal(body, &e); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("decode: %v", err)), nil
			}
			out, _ := json.MarshalIndent(e.Attributes, "", "  ")
			return mcp.NewToolResultText(string(out)), nil
		},
	)
}

// ----- call_service -----------------------------------------------------

func (h *HAServer) addCallService() {
	h.mcp.AddTool(
		mcp.NewTool("call_service",
			mcp.WithDescription("Call a HA service like 'light.turn_on'. "+
				"Extra service data passed as a JSON object string."),
			mcp.WithString("domain", mcp.Required(), mcp.Description("Service domain, e.g. 'light'")),
			mcp.WithString("service", mcp.Required(), mcp.Description("Service name, e.g. 'turn_on'")),
			mcp.WithString("entity_id", mcp.Description("Target entity_id, e.g. 'light.kitchen'")),
			mcp.WithString("data", mcp.Description("Extra service data as a JSON object string")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			domain, _ := args["domain"].(string)
			service, _ := args["service"].(string)
			if domain == "" || service == "" {
				return mcp.NewToolResultError("domain and service are required"), nil
			}
			payload := map[string]any{}
			if id, _ := args["entity_id"].(string); id != "" {
				payload["entity_id"] = id
			}
			if extra, _ := args["data"].(string); extra != "" {
				var m map[string]any
				if err := json.Unmarshal([]byte(extra), &m); err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("invalid data JSON: %v", err)), nil
				}
				for k, v := range m {
					payload[k] = v
				}
			}
			status, body, err := h.callAPI(ctx, "POST",
				"/api/services/"+url.PathEscape(domain)+"/"+url.PathEscape(service), payload)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("api: %v", err)), nil
			}
			if status < 200 || status >= 300 {
				return mcp.NewToolResultError(fmt.Sprintf("HA returned %d: %s", status, string(body))), nil
			}
			return mcp.NewToolResultText(string(body)), nil
		},
	)
}

// ----- get_history ------------------------------------------------------

func (h *HAServer) addGetHistory() {
	h.mcp.AddTool(
		mcp.NewTool("get_history",
			mcp.WithDescription("Return state history for an entity over the last N hours. Default 24."),
			mcp.WithString("entity_id", mcp.Required()),
			mcp.WithNumber("hours", mcp.Description("Window size in hours; default 24, max 168 (one week).")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			id, _ := args["entity_id"].(string)
			hours := 24
			if v, ok := args["hours"].(float64); ok && v > 0 {
				hours = int(v)
				if hours > 168 {
					hours = 168
				}
			}
			start := time.Now().Add(-time.Duration(hours) * time.Hour).UTC().Format("2006-01-02T15:04:05")
			path := fmt.Sprintf("/api/history/period/%s?filter_entity_id=%s",
				url.PathEscape(start), url.QueryEscape(id))
			status, body, err := h.callAPI(ctx, "GET", path, nil)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("api: %v", err)), nil
			}
			if status != 200 {
				return mcp.NewToolResultError(fmt.Sprintf("HA returned %d", status)), nil
			}
			return mcp.NewToolResultText(string(body)), nil
		},
	)
}

// ----- list_automations -------------------------------------------------

func (h *HAServer) addListAutomations() {
	h.mcp.AddTool(
		mcp.NewTool("list_automations",
			mcp.WithDescription("List automations and their on/off state. Backed by get_states with domain=automation."),
		),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			status, body, err := h.getStates(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("api: %v", err)), nil
			}
			if status != 200 {
				return mcp.NewToolResultError(fmt.Sprintf("HA returned %d", status)), nil
			}
			var entities []haEntity
			if err := json.Unmarshal(body, &entities); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("decode: %v", err)), nil
			}
			type row struct {
				ID    string `json:"entity_id"`
				State string `json:"state"`
			}
			out := make([]row, 0)
			for _, e := range entities {
				if strings.HasPrefix(e.EntityID, "automation.") {
					out = append(out, row{ID: e.EntityID, State: e.State})
				}
			}
			body, _ = json.MarshalIndent(out, "", "  ")
			return mcp.NewToolResultText(string(body)), nil
		},
	)
}

// ----- toggle_automation ------------------------------------------------

func (h *HAServer) addToggleAutomation() {
	h.mcp.AddTool(
		mcp.NewTool("toggle_automation",
			mcp.WithDescription("Enable or disable an automation by entity_id."),
			mcp.WithString("entity_id", mcp.Required(), mcp.Description("automation.<name>")),
			mcp.WithBoolean("enabled", mcp.Required(), mcp.Description("true to enable, false to disable")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			id, _ := args["entity_id"].(string)
			enabled, _ := args["enabled"].(bool)
			service := "turn_off"
			if enabled {
				service = "turn_on"
			}
			payload := map[string]any{"entity_id": id}
			status, body, err := h.callAPI(ctx, "POST",
				"/api/services/automation/"+service, payload)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("api: %v", err)), nil
			}
			if status < 200 || status >= 300 {
				return mcp.NewToolResultError(fmt.Sprintf("HA returned %d: %s", status, string(body))), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf(`{"entity_id":"%s","new_state":"%s"}`, id, service)), nil
		},
	)
}

// ----- fire_event -------------------------------------------------------

func (h *HAServer) addFireEvent() {
	h.mcp.AddTool(
		mcp.NewTool("fire_event",
			mcp.WithDescription("Fire a Home Assistant event with optional JSON object data."),
			mcp.WithString("event_type", mcp.Required()),
			mcp.WithString("data", mcp.Description("Optional JSON object string")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			et, _ := args["event_type"].(string)
			if et == "" {
				return mcp.NewToolResultError("event_type required"), nil
			}
			var payload any
			if d, _ := args["data"].(string); d != "" {
				var m map[string]any
				if err := json.Unmarshal([]byte(d), &m); err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("invalid data JSON: %v", err)), nil
				}
				payload = m
			}
			status, body, err := h.callAPI(ctx, "POST",
				"/api/events/"+url.PathEscape(et), payload)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("api: %v", err)), nil
			}
			if status < 200 || status >= 300 {
				return mcp.NewToolResultError(fmt.Sprintf("HA returned %d: %s", status, string(body))), nil
			}
			return mcp.NewToolResultText(string(body)), nil
		},
	)
}
