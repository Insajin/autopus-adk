// Package mcpserver implements a JSON-RPC 2.0 MCP server over stdio.
package mcpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
)

// ToolHandler processes an MCP tool call and returns the result.
type ToolHandler func(ctx context.Context, params json.RawMessage) (any, error)

// MCPServer is a JSON-RPC 2.0 MCP server that reads from stdin and writes to stdout.
type MCPServer struct {
	backendURL  string
	authToken   string
	workspaceID string
	tools       map[string]ToolHandler
	resources   *ResourceRegistry
	mu          sync.Mutex
	writer      io.Writer
}

// jsonRPCRequest represents an incoming JSON-RPC 2.0 request.
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// jsonRPCResponse represents an outgoing JSON-RPC 2.0 response.
type jsonRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id"`
	Result  any           `json:"result,omitempty"`
	Error   *jsonRPCError `json:"error,omitempty"`
}

// jsonRPCError represents a JSON-RPC 2.0 error object.
type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// toolDescriptor describes a tool for the tools/list response.
type toolDescriptor struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// New creates a new MCP server connected to the given backend.
// It is an alias for NewMCPServer for convenience.
func New(backendURL, authToken, workspaceID string) *MCPServer {
	return NewMCPServer(backendURL, authToken, workspaceID)
}

// NewMCPServerFromConfig creates an MCP server from a validated config.
func NewMCPServerFromConfig(cfg Config) (*MCPServer, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid MCP config: %w", err)
	}
	return NewMCPServer(cfg.BackendURL, cfg.AuthToken, cfg.WorkspaceID), nil
}

// NewMCPServer creates a new MCP server connected to the given backend.
// @AX:ANCHOR[AUTO]: public constructor — NewMCPServer/New are the primary entry points; signature is referenced by worker setup and CLI
func NewMCPServer(backendURL, authToken, workspaceID string) *MCPServer {
	s := &MCPServer{
		backendURL:  backendURL,
		authToken:   authToken,
		workspaceID: workspaceID,
		tools:       make(map[string]ToolHandler),
		writer:      os.Stdout,
	}
	s.resources = NewResourceRegistry(backendURL, authToken)
	s.registerTools()
	return s
}

// StartSSE starts the SSE transport on the given addr (host:port).
func (s *MCPServer) StartSSE(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.Handle("/mcp/sse", s.SSEHandler())
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background()) //nolint:errcheck
	}()
	return srv.ListenAndServe()
}

// Start reads line-delimited JSON-RPC requests from stdin and dispatches them.
func (s *MCPServer) Start(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)
	// Allow large messages up to 1MB.
	// @AX:NOTE[AUTO]: magic constant — 1MB scanner buffer cap; oversized JSON-RPC messages are silently truncated without error
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(nil, -32700, "parse error")
			continue
		}
		s.dispatch(ctx, &req)
	}
	return scanner.Err()
}

func (s *MCPServer) dispatch(ctx context.Context, req *jsonRPCRequest) {
	switch req.Method {
	case "initialize":
		// @AX:NOTE[AUTO]: magic constant — protocolVersion "2024-11-05" is MCP spec version; update when upgrading MCP protocol
		s.sendResult(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]string{"name": "autopus-adk", "version": "0.1.0"},
			"capabilities": map[string]any{
				"tools":     map[string]any{},
				"resources": map[string]any{},
			},
		})
	case "ping":
		s.sendResult(req.ID, map[string]any{})
	case "notifications/initialized":
		return
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolsCall(ctx, req)
	case "resources/list":
		s.handleResourcesList(req)
	case "resources/read":
		s.handleResourcesRead(ctx, req)
	default:
		if req.isNotification() {
			return
		}
		s.sendError(req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func (r *jsonRPCRequest) isNotification() bool {
	return r.ID == nil
}

func (s *MCPServer) handleToolsList(req *jsonRPCRequest) {
	var list []toolDescriptor
	for name := range s.tools {
		list = append(list, toolDescriptor{Name: name, Description: name})
	}
	s.sendResult(req.ID, map[string]any{"tools": list})
}

func (s *MCPServer) handleToolsCall(ctx context.Context, req *jsonRPCRequest) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		s.sendError(req.ID, -32602, "invalid params")
		return
	}
	handler, ok := s.tools[p.Name]
	if !ok {
		s.sendError(req.ID, -32602, fmt.Sprintf("unknown tool: %s", p.Name))
		return
	}
	result, err := handler(ctx, p.Arguments)
	if err != nil {
		s.sendError(req.ID, -32000, err.Error())
		return
	}
	s.sendResult(req.ID, result)
}

func (s *MCPServer) handleResourcesList(req *jsonRPCRequest) {
	s.sendResult(req.ID, map[string]any{"resources": s.resources.ListResources()})
}

func (s *MCPServer) handleResourcesRead(ctx context.Context, req *jsonRPCRequest) {
	var p struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		s.sendError(req.ID, -32602, "invalid params")
		return
	}
	data, err := s.resources.Get(ctx, p.URI)
	if err != nil {
		s.sendError(req.ID, -32000, err.Error())
		return
	}
	s.sendResult(req.ID, data)
}

func (s *MCPServer) sendResult(id any, result any) {
	s.writeResponse(jsonRPCResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *MCPServer) sendError(id any, code int, msg string) {
	s.writeResponse(jsonRPCResponse{JSONRPC: "2.0", ID: id, Error: &jsonRPCError{Code: code, Message: msg}})
}

func (s *MCPServer) writeResponse(resp jsonRPCResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, _ := json.Marshal(resp)
	fmt.Fprintf(s.writer, "%s\n", data)
}

func (s *MCPServer) registerTools() {
	s.tools["execute_task"] = s.handleExecuteTask
	s.tools["search_knowledge"] = s.handleSearchKnowledge
	s.tools["get_execution_status"] = s.handleGetExecutionStatus
	s.tools["list_agents"] = s.handleListAgents
	s.tools["approve_execution"] = s.handleApproveExecution
	s.tools["manage_workspace"] = s.handleManageWorkspace
}
