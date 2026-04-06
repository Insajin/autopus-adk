package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// TaskHandler is invoked when a new task is received.
type TaskHandler func(ctx context.Context, taskID string, payload json.RawMessage) (*TaskResult, error)

// ServerConfig holds configuration for the A2A server.
type ServerConfig struct {
	BackendURL  string
	WorkerName  string
	WorkspaceID string
	Skills      []string
	Handler     TaskHandler
	AuthToken            string // Bearer token for backend auth (SEC-005)
	ApprovalCallback     func(ApprovalRequestParams)
	OnConnectionExhausted func() // called once when reconnect backoff reaches maxBackoff
}

// Server manages the A2A JSON-RPC protocol over WebSocket transport.
type Server struct {
	config       ServerConfig
	transport    *Transport
	handler      TaskHandler
	tasks        map[string]*Task
	taskContexts map[string]context.CancelFunc // per-task cancellable contexts (REQ-A2A-H02)
	approvalCB   func(ApprovalRequestParams)
	mu           sync.Mutex
	cancel       context.CancelFunc
}

// toWebSocketURL converts an http/https URL to a ws/wss URL for WebSocket dialing.
func toWebSocketURL(u string) string {
	switch {
	case strings.HasPrefix(u, "https://"):
		return "wss://" + u[len("https://"):]
	case strings.HasPrefix(u, "http://"):
		return "ws://" + u[len("http://"):]
	}
	return u
}

// NewServer creates a new A2A server with the given configuration.
func NewServer(config ServerConfig) *Server {
	return &Server{
		config:       config,
		handler:      config.Handler,
		tasks:        make(map[string]*Task),
		taskContexts: make(map[string]context.CancelFunc),
		approvalCB:   config.ApprovalCallback,
	}
}

// Start connects to the backend, registers an Agent Card, and enters the message loop.
func (s *Server) Start(ctx context.Context) error {
	ctx, s.cancel = context.WithCancel(ctx)

	tc := TransportConfig{
		URL:              toWebSocketURL(s.config.BackendURL) + "/ws/a2a",
		AuthToken:        s.config.AuthToken,
		HeartbeatSec:     30,
		ReconnectBaseSec: 3,
		ReconnectFactor:  2,
		MaxRetries:       4,
	}
	s.transport = NewTransport(tc)

	if err := s.transport.Connect(ctx); err != nil {
		return fmt.Errorf("a2a connect: %w", err)
	}

	card := AgentCard{
		Name:                s.config.WorkerName,
		Description:         "Autopus ADK Worker",
		URL:                 s.config.BackendURL,
		WorkspaceID:         s.config.WorkspaceID,
		Skills:              s.config.Skills,
		SupportedInputModes: []string{"text"},
	}
	if err := s.RegisterAgentCard(card); err != nil {
		return fmt.Errorf("a2a register card: %w", err)
	}

	go s.messageLoop(ctx)
	return nil
}

// RegisterAgentCard sends the agent card to the backend.
func (s *Server) RegisterAgentCard(card AgentCard) error {
	params, err := marshalJSON(card)
	if err != nil {
		return fmt.Errorf("marshal agent card: %w", err)
	}
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  MethodRegisterCard,
		Params:  params,
	}
	return s.sendJSON(req)
}

// UpdateTaskStatus sends a task status update to the backend via WebSocket.
func (s *Server) UpdateTaskStatus(taskID string, status TaskStatus, result *TaskResult) error {
	s.mu.Lock()
	if t, ok := s.tasks[taskID]; ok {
		t.Status = status
	}
	s.mu.Unlock()

	params := StatusUpdateParams{
		TaskID: taskID,
		Status: status,
		Result: result,
	}
	notif := JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  MethodStatusUpdate,
		Params:  params,
	}
	return s.sendJSON(notif)
}

// SetAuthToken updates the auth token used for backend communication.
func (s *Server) SetAuthToken(token string) {
	s.mu.Lock()
	s.config.AuthToken = token
	s.mu.Unlock()
}

// ReconnectTransport attempts to reconnect the WebSocket transport.
func (s *Server) ReconnectTransport(ctx context.Context) error {
	if s.transport == nil {
		return fmt.Errorf("transport not initialized")
	}
	return s.transport.Reconnect(ctx)
}

// Close shuts down the server and its transport.
func (s *Server) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.transport != nil {
		return s.transport.Close()
	}
	return nil
}

// messageLoop reads incoming messages and dispatches them.
// Applies backoff on consecutive receive errors to avoid tight CPU loops (SEC-006).
// When backoff reaches maxBackoff, OnConnectionExhausted is fired once to activate REST polling fallback.
func (s *Server) messageLoop(ctx context.Context) {
	const maxBackoff = 30 * time.Second
	backoff := time.Duration(0)
	exhaustedFired := false

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		data, err := s.transport.Receive()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[a2a] receive error: %v", err)

			// Exponential backoff on consecutive errors.
			if backoff == 0 {
				backoff = 500 * time.Millisecond
			} else if backoff < maxBackoff {
				backoff *= 2
			}

			// Fire exhausted callback once when backoff ceiling is reached.
			if backoff >= maxBackoff && !exhaustedFired {
				exhaustedFired = true
				if s.config.OnConnectionExhausted != nil {
					s.config.OnConnectionExhausted()
				}
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			continue
		}

		// Successful receive: reset backoff and allow exhausted to fire again on future failures.
		if backoff > 0 {
			backoff = 0
			exhaustedFired = false
		}
		s.handleMessage(ctx, data)
	}
}

// handleMessage parses a JSON-RPC message and routes it by method.
// JSON-RPC responses (e.g., registration ack) have no method — they are silently ignored.
func (s *Server) handleMessage(ctx context.Context, msg []byte) {
	var req JSONRPCRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		log.Printf("[a2a] invalid message: %v", err)
		return
	}

	// Skip JSON-RPC responses (no method field) — e.g., registration ack.
	if req.Method == "" {
		return
	}

	switch req.Method {
	case MethodSendMessage:
		s.handleSendMessage(ctx, req)
	case MethodCancelTask:
		s.handleCancelTask(req)
	case MethodApproval:
		s.handleApproval(req)
	default:
		log.Printf("[a2a] unknown method: %s", req.Method)
	}
}

// handleSendMessage extracts the task payload, caches the security policy, and dispatches.
func (s *Server) handleSendMessage(ctx context.Context, req JSONRPCRequest) {
	var params SendMessageParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Printf("[a2a] invalid SendMessage params: %v", err)
		s.sendError(req.ID, -32602, "invalid params")
		return
	}

	// SEC-007: Reject duplicate task IDs to prevent state overwrites.
	s.mu.Lock()
	if _, exists := s.tasks[params.TaskID]; exists {
		s.mu.Unlock()
		log.Printf("[a2a] duplicate task ID rejected: %s", params.TaskID)
		s.sendError(req.ID, -32602, fmt.Sprintf("duplicate task ID: %s", params.TaskID))
		return
	}
	s.tasks[params.TaskID] = &Task{ID: params.TaskID, Status: StatusWorking}
	s.mu.Unlock()

	if err := cacheSecurityPolicy(params.TaskID, params.SecurityPolicy); err != nil {
		log.Printf("[a2a] cache policy error: %v", err)
	}

	// Notify backend of working status.
	_ = s.UpdateTaskStatus(params.TaskID, StatusWorking, nil)

	// Dispatch asynchronously.
	go s.dispatchTask(ctx, req.ID, params)
}

// dispatchTask runs the task handler and reports the result.
// Uses a per-task cancellable context so individual tasks can be canceled (REQ-A2A-H02).
// Applies SecurityPolicy.TimeoutSec as a hard deadline when configured.
func (s *Server) dispatchTask(ctx context.Context, reqID json.RawMessage, params SendMessageParams) {
	// Apply SecurityPolicy timeout as a hard deadline for task execution.
	// When TimeoutSec > 0, the subprocess is killed after the deadline via
	// exec.CommandContext propagation. When 0, fall back to cancel-only (no deadline).
	var taskCtx context.Context
	var cancel context.CancelFunc
	if params.SecurityPolicy.TimeoutSec > 0 {
		timeout := time.Duration(params.SecurityPolicy.TimeoutSec) * time.Second
		taskCtx, cancel = context.WithTimeout(ctx, timeout)
		log.Printf("[a2a] task %s: applying timeout %ds from SecurityPolicy", params.TaskID, params.SecurityPolicy.TimeoutSec)
	} else {
		taskCtx, cancel = context.WithCancel(ctx)
	}
	s.mu.Lock()
	s.taskContexts[params.TaskID] = cancel
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		delete(s.taskContexts, params.TaskID)
		s.mu.Unlock()
	}()

	result, err := s.handler(taskCtx, params.TaskID, params.Payload)
	if err != nil {
		failResult := &TaskResult{Status: StatusFailed, Error: err.Error()}
		_ = s.UpdateTaskStatus(params.TaskID, StatusFailed, failResult)
		s.sendResult(reqID, failResult)
		return
	}
	result.Status = StatusCompleted
	_ = s.UpdateTaskStatus(params.TaskID, StatusCompleted, result)
	s.sendResult(reqID, result)
}

// handleCancelTask marks a task as canceled.
func (s *Server) handleCancelTask(req JSONRPCRequest) {
	var p struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		s.sendError(req.ID, -32602, "invalid params")
		return
	}
	// Cancel the per-task context if it exists (REQ-A2A-H02).
	s.mu.Lock()
	if cancelFn, ok := s.taskContexts[p.TaskID]; ok {
		cancelFn()
	}
	s.mu.Unlock()

	_ = s.UpdateTaskStatus(p.TaskID, StatusCanceled, nil)
	s.sendResult(req.ID, map[string]string{"status": "canceled"})
}

