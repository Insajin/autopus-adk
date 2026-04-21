package a2a

import (
	"encoding/json"
	"log"
)

// handleApproval processes an incoming approval request from the backend.
func (s *Server) handleApproval(req JSONRPCRequest) {
	var params ApprovalRequestParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Printf("[a2a] invalid approval params: %v", err)
		s.sendError(req.ID, -32602, "invalid params")
		return
	}
	// Update task status to input-required.
	_ = s.UpdateTaskStatus(params.TaskID, StatusInputRequired, nil)
	// Invoke callback if registered.
	if s.approvalCB != nil {
		s.approvalCB(params)
	}
}

// SendApprovalResponse sends the user's approval decision back to the backend.
func (s *Server) SendApprovalResponse(params ApprovalResponseParams) error {
	// Restore task status to working.
	_ = s.UpdateTaskStatus(params.TaskID, StatusWorking, nil)
	notif := JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  MethodApprovalResponse,
		Params:  params,
	}
	return s.sendJSON(notif)
}
