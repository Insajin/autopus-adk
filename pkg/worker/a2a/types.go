package a2a

import "encoding/json"

// A2A JSON-RPC method constants.
const (
	MethodSendMessage      = "tasks/send"
	MethodSendSubscribe    = "tasks/sendSubscribe"
	MethodGetTask          = "tasks/get"
	MethodCancelTask       = "tasks/cancel"
	MethodRegisterCard     = "agent/register"
	MethodHeartbeat        = "agent/heartbeat"
	MethodStatusUpdate     = "tasks/statusUpdate"
	MethodApproval         = "tasks/approval"
	MethodApprovalResponse = "tasks/approvalResponse"
)

// TaskStatus represents the lifecycle state of an A2A task.
type TaskStatus string

const (
	StatusWorking       TaskStatus = "working"
	StatusInputRequired TaskStatus = "input-required"
	StatusCompleted     TaskStatus = "completed"
	StatusFailed        TaskStatus = "failed"
	StatusCanceled      TaskStatus = "canceled"
)

// JSONRPCRequest is the inbound JSON-RPC 2.0 request envelope.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse is the outbound JSON-RPC 2.0 response envelope.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC error object.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// JSONRPCNotification is a JSON-RPC 2.0 notification (no id).
type JSONRPCNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// AgentCard describes the worker's capabilities for registration.
type AgentCard struct {
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	URL                 string   `json:"url"`
	WorkspaceID         string   `json:"workspace_id,omitempty"`
	Skills              []string `json:"skills"`
	SupportedInputModes []string `json:"supported_input_modes"`
}

// Task represents an A2A task with status tracking.
type Task struct {
	ID        string      `json:"id"`
	Status    TaskStatus  `json:"status"`
	Artifacts []Artifact  `json:"artifacts,omitempty"`
	Metadata  TaskMeta    `json:"metadata,omitempty"`
}

// Artifact holds a single result artifact from task execution.
type Artifact struct {
	Name     string `json:"name"`
	MimeType string `json:"mime_type,omitempty"`
	Data     string `json:"data"`
}

// TaskMeta contains optional metadata attached to a task.
type TaskMeta struct {
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// SecurityPolicy defines the security constraints for task execution.
type SecurityPolicy struct {
	AllowNetwork  bool     `json:"allow_network"`
	AllowFS       bool     `json:"allow_fs"`
	AllowedPaths  []string `json:"allowed_paths,omitempty"`
	TimeoutSec    int      `json:"timeout_sec,omitempty"`
}

// SendMessageParams is the payload for the tasks/send method.
type SendMessageParams struct {
	TaskID         string          `json:"task_id"`
	Payload        json.RawMessage `json:"payload"`
	SecurityPolicy SecurityPolicy  `json:"security_policy"`
}

// TaskResult holds the outcome of a completed or failed task.
type TaskResult struct {
	Status    TaskStatus `json:"status"`
	Artifacts []Artifact `json:"artifacts,omitempty"`
	Error     string     `json:"error,omitempty"`
}

// ApprovalRequestParams holds approval request payload from the backend.
type ApprovalRequestParams struct {
	TaskID    string `json:"task_id"`
	Action    string `json:"action"`
	RiskLevel string `json:"risk_level"`
	Context   string `json:"context"`
}

// ApprovalResponseParams holds the user's approval decision.
type ApprovalResponseParams struct {
	TaskID   string `json:"task_id"`
	Decision string `json:"decision"` // "approve", "deny", "skip"
}

// StatusUpdateParams is sent to the backend to update task state.
type StatusUpdateParams struct {
	TaskID string      `json:"task_id"`
	Status TaskStatus  `json:"status"`
	Result *TaskResult `json:"result,omitempty"`
}
