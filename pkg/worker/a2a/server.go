package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
)

// TaskHandler is invoked when a new task is received.
type TaskHandler func(ctx context.Context, taskID string, payload json.RawMessage) (*TaskResult, error)

// ServerConfig holds configuration for the A2A server.
type ServerConfig struct {
	BackendURL            string
	WorkerName            string
	WorkspaceID           string
	Skills                []string
	Providers             []string
	ExecutionLanes        []string
	Handler               TaskHandler
	AuthToken             string // Bearer token for backend auth (SEC-005)
	ApprovalCallback      func(ApprovalRequestParams)
	DispatchIssueCallback func(DispatchIssue)
	OnConnectionExhausted func() // called once when reconnect backoff reaches maxBackoff
}

// DispatchIssue captures a transport failure during platform-facing task reconciliation.
type DispatchIssue struct {
	TaskID  string
	Stage   string
	Message string
}

// Server manages the A2A JSON-RPC protocol over WebSocket transport.
type Server struct {
	config          ServerConfig
	transport       *Transport
	handler         TaskHandler
	tasks           map[string]*Task
	taskContexts    map[string]context.CancelFunc // per-task cancellable contexts (REQ-A2A-H02)
	approvalCB      func(ApprovalRequestParams)
	dispatchIssueCB func(DispatchIssue)
	mu              sync.Mutex
	cancel          context.CancelFunc
	heartbeat       *Heartbeat
	restPoller      *RESTPoller
	reconnectMu     sync.Mutex
	// reconnectGen counts completed transport reconnects. Three drivers call
	// ReconnectTransport for the same connection loss — the receive loop, the
	// heartbeat timeout, and external callers — and reconnectMu only serializes
	// them, so each in turn tore down the connection its predecessor had just
	// established and re-registered the card again. That produced a duplicate
	// registration per loss and, on a backend that accepts one connection at a
	// time, `ws write: websocket: close sent` or `dial: connection refused` for
	// whoever ran second. The generation lets a waiter recognize that the
	// reconnect it wanted already happened.
	reconnectGen atomic.Uint64
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
		config:          config,
		handler:         config.Handler,
		tasks:           make(map[string]*Task),
		taskContexts:    make(map[string]context.CancelFunc),
		approvalCB:      config.ApprovalCallback,
		dispatchIssueCB: config.DispatchIssueCallback,
	}
}

// @AX:ANCHOR [AUTO] lifecycle entry point — connects transport, wires heartbeat, spawns messageLoop goroutine — fan_in: 3 (cmd/worker, integration tests, reload path)
// Start connects to the backend, registers an Agent Card, and enters the message loop.
func (s *Server) Start(ctx context.Context) error {
	ctx, s.cancel = context.WithCancel(ctx)

	// @AX:NOTE [AUTO] magic constants — HeartbeatSec:30, ReconnectBaseSec:3, MaxRetries:4 are hardcoded; consider promoting to ServerConfig
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

	// Wire up JSON-RPC heartbeat (S1).
	s.heartbeat = NewHeartbeatWithJSONRPC(func(msg []byte) error {
		return s.transport.Send(msg)
	}, func() {
		log.Printf("[a2a] heartbeat timeout — connection may be lost")
		go func() {
			if err := s.ReconnectTransport(ctx); err != nil {
				log.Printf("[a2a] heartbeat reconnect failed: %v", err)
			}
		}()
	})
	s.heartbeat.Start(ctx)

	if err := s.RegisterAgentCard(s.agentCard()); err != nil {
		return fmt.Errorf("a2a register card: %w", err)
	}

	go s.messageLoop(ctx)
	return nil
}

func (s *Server) agentCard() AgentCard {
	card := NewCardBuilder(s.config.WorkerName, s.config.BackendURL).
		WithProviders(s.config.Providers).
		WithExecutionLanes(s.config.ExecutionLanes).
		Build()
	return AgentCard{
		Name:                      card.Name,
		Description:               card.Description,
		URL:                       card.URL,
		WorkspaceID:               s.config.WorkspaceID,
		Providers:                 card.Providers,
		Skills:                    mergeSkills(card.Skills, s.config.Skills),
		ExecutionLanes:            card.ExecutionLanes,
		Capabilities:              card.Capabilities,
		UnsupportedModelOverrides: card.UnsupportedModelOverrides,
		SupportedInputModes:       []string{"text"},
	}
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
	if err := s.sendJSON(notif); err != nil {
		s.notifyDispatchIssue(taskID, "status_update", err)
		return err
	}
	return nil
}

// SetAuthToken updates the auth token used for backend communication.
func (s *Server) SetAuthToken(token string) {
	s.mu.Lock()
	s.config.AuthToken = token
	transport := s.transport
	restPoller := s.restPoller
	s.mu.Unlock()

	if transport != nil {
		transport.SetAuthToken(token)
	}
	if restPoller != nil {
		restPoller.SetAuthToken(token)
	}
}

// SetRESTPoller attaches a REST poller that activates when WebSocket connection is exhausted.
func (s *Server) SetRESTPoller(p *RESTPoller) {
	s.mu.Lock()
	s.restPoller = p
	s.mu.Unlock()
}

// transportGeneration identifies the connection a caller is holding. Pair it
// with reconnectTransportFrom so a driver asks to replace the exact connection
// it saw fail rather than whatever is current by the time it acquires the lock.
func (s *Server) transportGeneration() uint64 {
	return s.reconnectGen.Load()
}

// ReconnectTransport attempts to reconnect the WebSocket transport. External
// callers hold no generation, so they reconnect the current one.
func (s *Server) ReconnectTransport(ctx context.Context) error {
	return s.reconnectTransportFrom(ctx, s.transportGeneration())
}

// reconnectTransportFrom replaces the connection identified by observed.
//
// Drivers are coalesced, not queued: if the generation moved past observed, the
// connection this caller wanted replaced is already gone and reconnecting again
// would only tear down its fresh replacement. Coalescing can at worst defer
// recovery of a connection that broke again inside the same window by one
// receive-loop iteration, which the loop's backoff already handles.
func (s *Server) reconnectTransportFrom(ctx context.Context, observed uint64) error {
	s.reconnectMu.Lock()
	defer s.reconnectMu.Unlock()

	if s.transport == nil {
		return fmt.Errorf("transport not initialized")
	}
	if s.reconnectGen.Load() != observed {
		return nil
	}
	if err := s.transport.Reconnect(ctx); err != nil {
		return err
	}
	if s.heartbeat != nil {
		s.heartbeat.Ack()
	}
	if err := s.RegisterAgentCard(s.agentCard()); err != nil {
		return fmt.Errorf("re-register agent card: %w", err)
	}
	if s.restPoller != nil {
		s.restPoller.Stop()
	}
	s.reconnectGen.Add(1)
	return nil
}

// Close shuts down the server and its transport.
func (s *Server) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.restPoller != nil {
		s.restPoller.Stop()
	}
	if s.transport != nil {
		return s.transport.Close()
	}
	return nil
}

func (s *Server) notifyDispatchIssue(taskID, stage string, err error) {
	if s.dispatchIssueCB == nil || err == nil {
		return
	}
	s.dispatchIssueCB(DispatchIssue{
		TaskID:  taskID,
		Stage:   stage,
		Message: err.Error(),
	})
}
