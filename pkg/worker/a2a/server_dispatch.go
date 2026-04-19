package a2a

import (
	"context"
	"encoding/json"
	"log"
)

// @AX:WARN [AUTO] concurrent map mutation — tasks map guarded by mu; ensure lock is held for all reads/writes to s.tasks
// handleSendMessage extracts the task payload, caches the security policy, and dispatches.
func (s *Server) handleSendMessage(ctx context.Context, req JSONRPCRequest) {
	var params SendMessageParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Printf("[a2a] invalid SendMessage params: %v", err)
		s.sendError(req.ID, -32602, "invalid params")
		return
	}

	if err := s.enqueueAndDispatchTask(ctx, req.ID, params); err != nil {
		log.Printf("[a2a] send message rejected: %v", err)
		s.sendError(req.ID, -32602, err.Error())
		return
	}
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
