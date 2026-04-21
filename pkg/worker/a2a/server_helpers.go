package a2a

import (
	"encoding/json"
	"fmt"
	"log"
)

func (s *Server) sendJSON(v any) error {
	if s.transport == nil {
		return fmt.Errorf("transport not initialized")
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return s.transport.Send(data)
}

func (s *Server) sendResult(taskID string, id json.RawMessage, result any) {
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: id, Result: result}
	if err := s.sendJSON(resp); err != nil {
		s.notifyDispatchIssue(taskID, "result_response", err)
		log.Printf("[a2a] send result error: %v", err)
	}
}

func (s *Server) sendError(id json.RawMessage, code int, msg string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &JSONRPCError{Code: code, Message: msg},
	}
	if err := s.sendJSON(resp); err != nil {
		log.Printf("[a2a] send error response error: %v", err)
	}
}
