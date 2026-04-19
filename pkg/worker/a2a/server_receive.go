package a2a

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

// @AX:NOTE [AUTO] magic constants — maxBackoff:30s, initial backoff:500ms; controls REST fallback activation latency
// @AX:ANCHOR [AUTO] connection resilience loop — fires OnConnectionExhausted and starts RESTPoller on exhaustion; do not remove without updating fallback path — fan_in: 3 (Start, transport error, ctx cancel)
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
			backoff, exhaustedFired = s.handleReceiveError(ctx, err, backoff, exhaustedFired, maxBackoff)
			if ctx.Err() != nil {
				return
			}
			if backoff > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
				}
			}
			continue
		}

		backoff, exhaustedFired = s.resetReceiveBackoff(backoff, exhaustedFired)
		s.handleMessage(ctx, data)
	}
}

func (s *Server) handleReceiveError(ctx context.Context, err error, backoff time.Duration, exhaustedFired bool, maxBackoff time.Duration) (time.Duration, bool) {
	if ctx.Err() != nil {
		return 0, exhaustedFired
	}

	log.Printf("[a2a] receive error: %v", err)
	if reconnectErr := s.ReconnectTransport(ctx); reconnectErr == nil {
		log.Printf("[a2a] transport recovered after receive error")
		return s.resetReceiveBackoff(backoff, exhaustedFired)
	} else {
		log.Printf("[a2a] reconnect attempt after receive error failed: %v", reconnectErr)
	}

	backoff = nextReceiveBackoff(backoff, maxBackoff)
	if backoff >= maxBackoff && !exhaustedFired {
		exhaustedFired = true
		if s.config.OnConnectionExhausted != nil {
			s.config.OnConnectionExhausted()
		}
		if s.restPoller != nil {
			s.restPoller.Start(ctx)
		}
	}

	return backoff, exhaustedFired
}

func nextReceiveBackoff(backoff time.Duration, maxBackoff time.Duration) time.Duration {
	if backoff == 0 {
		return 500 * time.Millisecond
	}
	if backoff < maxBackoff {
		return backoff * 2
	}
	return backoff
}

func (s *Server) resetReceiveBackoff(backoff time.Duration, exhaustedFired bool) (time.Duration, bool) {
	if backoff > 0 && exhaustedFired && s.restPoller != nil {
		s.restPoller.Stop()
	}
	return 0, false
}

// @AX:WARN [AUTO] high cyclomatic complexity — switch + nested nil checks; adding new methods here risks missing the heartbeat ack path
// handleMessage parses a JSON-RPC message and routes it by method.
// Heartbeat ack responses (no method, result with status "ok") update the heartbeat lastAck.
func (s *Server) handleMessage(ctx context.Context, msg []byte) {
	var req JSONRPCRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		log.Printf("[a2a] invalid message: %v", err)
		return
	}

	if req.Method == "" {
		s.handleResponse(msg)
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

// handleResponse processes JSON-RPC response messages (no method field).
// Detects heartbeat ack responses and calls Ack() on the heartbeat instance.
func (s *Server) handleResponse(msg []byte) {
	var resp struct {
		Result map[string]string `json:"result"`
	}
	if err := json.Unmarshal(msg, &resp); err != nil {
		return
	}
	if resp.Result["status"] == "ok" && s.heartbeat != nil {
		s.heartbeat.Ack()
	}
}
