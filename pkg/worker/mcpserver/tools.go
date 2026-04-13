package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// httpClient is the shared HTTP client with a 10-second timeout.
var httpClient = &http.Client{Timeout: 10 * time.Second}

// handleExecuteTask creates a new task via POST /api/v1/tasks.
func (s *MCPServer) handleExecuteTask(ctx context.Context, params json.RawMessage) (any, error) {
	return s.doPost(ctx, "/api/v1/tasks", params)
}

// handleSearchKnowledge searches knowledge via the workspace-scoped Knowledge Hub API.
func (s *MCPServer) handleSearchKnowledge(ctx context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Query      string   `json:"query"`
		Limit      int      `json:"limit,omitempty"`
		Categories []string `json:"categories,omitempty"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	body, err := json.Marshal(map[string]any{
		"query":      p.Query,
		"limit":      p.Limit,
		"categories": p.Categories,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}
	return s.doPost(ctx, fmt.Sprintf("/api/v1/workspaces/%s/knowledge/search", s.workspaceID), body)
}

// handleGetExecutionStatus retrieves execution status via GET /api/v1/executions/{id}.
func (s *MCPServer) handleGetExecutionStatus(ctx context.Context, params json.RawMessage) (any, error) {
	id, err := extractID(params)
	if err != nil {
		return nil, err
	}
	return s.doGet(ctx, fmt.Sprintf("/api/v1/executions/%s", id))
}

// handleListAgents lists available agents via GET /api/v1/agents.
func (s *MCPServer) handleListAgents(ctx context.Context, _ json.RawMessage) (any, error) {
	return s.doGet(ctx, "/api/v1/agents")
}

// handleApproveExecution approves an execution via POST /api/v1/executions/{id}/approve.
func (s *MCPServer) handleApproveExecution(ctx context.Context, params json.RawMessage) (any, error) {
	id, err := extractID(params)
	if err != nil {
		return nil, err
	}
	return s.doPost(ctx, fmt.Sprintf("/api/v1/executions/%s/approve", id), nil)
}

// handleManageWorkspace handles GET/PUT on /api/v1/workspaces/{id}.
func (s *MCPServer) handleManageWorkspace(ctx context.Context, params json.RawMessage) (any, error) {
	var p struct {
		ID     string          `json:"id"`
		Action string          `json:"action"` // "get" or "update"
		Data   json.RawMessage `json:"data,omitempty"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	wsID := p.ID
	if wsID == "" {
		wsID = s.workspaceID
	}
	path := fmt.Sprintf("/api/v1/workspaces/%s", wsID)

	if p.Action == "update" {
		return s.doPut(ctx, path, p.Data)
	}
	return s.doGet(ctx, path)
}

// doGet performs an authenticated GET request to the backend.
func (s *MCPServer) doGet(ctx context.Context, path string) (any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.backendURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	return s.executeRequest(req)
}

// doPost performs an authenticated POST request to the backend.
func (s *MCPServer) doPost(ctx context.Context, path string, body json.RawMessage) (any, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.backendURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return s.executeRequest(req)
}

// doPut performs an authenticated PUT request to the backend.
func (s *MCPServer) doPut(ctx context.Context, path string, body json.RawMessage) (any, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, s.backendURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return s.executeRequest(req)
}

// executeRequest sends the HTTP request with Bearer auth and returns the parsed body.
func (s *MCPServer) executeRequest(req *http.Request) (any, error) {
	req.Header.Set("Authorization", "Bearer "+s.authToken)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("backend request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("backend error %d: %s", resp.StatusCode, string(data))
	}

	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return string(data), nil
	}
	return result, nil
}

// extractID extracts the "id" field from JSON params.
func extractID(params json.RawMessage) (string, error) {
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	if p.ID == "" {
		return "", fmt.Errorf("id is required")
	}
	return p.ID, nil
}
