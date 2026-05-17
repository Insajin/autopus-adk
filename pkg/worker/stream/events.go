package stream

import "encoding/json"

// EventType constants for Claude stream-json output.
const (
	EventSystemInit       = "system.init"
	EventTaskStarted      = "system.task_started"
	EventTaskProgress     = "system.task_progress"
	EventTaskNotification = "system.task_notification"
	EventResult           = "result"
	EventError            = "error"
	EventToolCall         = "tool_call"
)

// Event represents a parsed stream-json event.
type Event struct {
	Type    string          // top-level type (e.g., "system", "result")
	Subtype string          // parsed from "type.subtype" format (e.g., "init")
	Raw     json.RawMessage // original JSON line
}

// ResultData holds extracted data from a "result" event.
type ResultData struct {
	CostUSD        float64 `json:"cost_usd"`
	DurationMS     int64   `json:"duration_ms"`
	SessionID      string  `json:"session_id"`
	Output         string  `json:"output,omitempty"`
	Result         string  `json:"result,omitempty"`
	IsError        bool    `json:"is_error,omitempty"`
	APIErrorStatus int     `json:"api_error_status,omitempty"`
}

// InitData holds data from a "system.init" event.
type InitData struct {
	MCPServers []string `json:"mcp_servers,omitempty"`
	Tools      []string `json:"tools,omitempty"`
}
