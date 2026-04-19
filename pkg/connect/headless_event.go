package connect

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// HeadlessEvent is a single NDJSON line written to stdout in headless mode.
type HeadlessEvent struct {
	Step          string `json:"step"`                     // "server_auth", "workspace", "openai_oauth", "complete"
	Action        string `json:"action,omitempty"`         // "login_required" when user action is needed
	Status        string `json:"status,omitempty"`         // "success", "error"
	URL           string `json:"url,omitempty"`            // verification URL
	Code          string `json:"code,omitempty"`           // user code
	ExpiresIn     int    `json:"expires_in,omitempty"`     // seconds until expiration
	WorkspaceID   string `json:"workspace_id,omitempty"`   // workspace identifier
	WorkspaceName string `json:"workspace_name,omitempty"` // workspace display name
	Provider      string `json:"provider,omitempty"`       // "openai"
	Error         string `json:"error,omitempty"`          // error message on failure
}

// emitWriter holds the output writer for headless events.
// Defaults to os.Stdout; overridable in tests.
var emitWriter io.Writer = os.Stdout

// EmitEvent writes a single NDJSON event to stdout.
func EmitEvent(ev HeadlessEvent) {
	data, _ := json.Marshal(ev)
	fmt.Fprintln(emitWriter, string(data))
}

// SwapEmitWriter replaces the writer used by EmitEvent and returns the previous one.
// Intended for use in tests only.
func SwapEmitWriter(w io.Writer) io.Writer {
	prev := emitWriter
	emitWriter = w
	return prev
}
