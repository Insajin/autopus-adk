package host

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"

	worker "github.com/insajin/autopus-adk/pkg/worker"
	"github.com/insajin/autopus-adk/pkg/worker/sidecarcontract"
)

const (
	SidecarProtocolName    = sidecarcontract.ProtocolName
	SidecarProtocolVersion = sidecarcontract.ProtocolVersion
	ContractName           = sidecarcontract.ContractName
	ContractMajor          = sidecarcontract.ContractMajor
)

// Event is a machine-readable sidecar NDJSON envelope.
type Event struct {
	Protocol        string             `json:"protocol"`
	ProtocolVersion string             `json:"protocol_version"`
	Contract        string             `json:"contract"`
	ContractMajor   string             `json:"contract_major"`
	Event           string             `json:"event"`
	Timestamp       time.Time          `json:"timestamp"`
	RuntimeID       string             `json:"runtime_id,omitempty"`
	WorkspaceID     string             `json:"workspace_id,omitempty"`
	Provider        string             `json:"provider,omitempty"`
	TaskID          string             `json:"task_id,omitempty"`
	TraceID         string             `json:"trace_id,omitempty"`
	CorrelationID   string             `json:"correlation_id,omitempty"`
	Phase           string             `json:"phase,omitempty"`
	Message         string             `json:"message,omitempty"`
	Metrics         *EventMetrics      `json:"metrics,omitempty"`
	Execution       *ExecutionContext  `json:"execution,omitempty"`
	Result          *TaskResultSummary `json:"result,omitempty"`
	Approval        *ApprovalState     `json:"approval,omitempty"`
	Error           *ErrorPayload      `json:"error,omitempty"`
}

// EventMetrics carries non-sensitive task metrics.
type EventMetrics struct {
	CostUSD    float64 `json:"cost_usd,omitempty"`
	DurationMS int64   `json:"duration_ms,omitempty"`
}

// ApprovalState carries approval gate metadata without secrets.
type ApprovalState struct {
	ApprovalID string `json:"approval_id,omitempty"`
	TraceID    string `json:"trace_id,omitempty"`
	Action     string `json:"action,omitempty"`
	RiskLevel  string `json:"risk_level,omitempty"`
	Context    string `json:"context,omitempty"`
	Resolution string `json:"resolution,omitempty"`
}

// ExecutionContext describes the retained worker execution boundary for desktop.
type ExecutionContext struct {
	WorkspaceID   string `json:"workspace_id,omitempty"`
	RootWorkDir   string `json:"root_workdir,omitempty"`
	ActiveWorkDir string `json:"active_workdir,omitempty"`
	WorktreePath  string `json:"worktree_path,omitempty"`
	Mode          string `json:"mode,omitempty"`
	BoundaryHint  string `json:"boundary_hint,omitempty"`
}

// TaskResultSummary captures the retained terminal task outcome for desktop.
type TaskResultSummary struct {
	Status       string        `json:"status,omitempty"`
	Summary      string        `json:"summary,omitempty"`
	ErrorMessage string        `json:"error_message,omitempty"`
	CostLabel    string        `json:"cost_label,omitempty"`
	DurationMS   int64         `json:"duration_ms,omitempty"`
	SessionID    string        `json:"session_id,omitempty"`
	Artifacts    []ArtifactRef `json:"artifacts,omitempty"`
}

// ArtifactRef is a desktop-safe reference to a worker artifact.
type ArtifactRef struct {
	Name     string `json:"name,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	Preview  string `json:"preview,omitempty"`
	Source   string `json:"source,omitempty"`
}

// ErrorPayload is the machine-readable error contract for sidecar events.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NDJSONEmitter writes contract events to a line-delimited JSON stream.
type NDJSONEmitter struct {
	mu  sync.Mutex
	enc *json.Encoder
}

// NewNDJSONEmitter creates a new sidecar NDJSON emitter.
func NewNDJSONEmitter(w io.Writer) *NDJSONEmitter {
	return &NDJSONEmitter{enc: json.NewEncoder(w)}
}

// Emit writes a single sidecar event envelope.
func (e *NDJSONEmitter) Emit(event Event) error {
	event.Protocol = SidecarProtocolName
	event.ProtocolVersion = SidecarProtocolVersion
	event.Contract = ContractName
	event.ContractMajor = ContractMajor
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.enc.Encode(event)
}

// Observer adapts worker host events into contract events.
func (e *NDJSONEmitter) Observer(cfg RuntimeConfig) worker.HostObserver {
	return worker.HostObserverFunc(func(hostEvent worker.HostEvent) {
		_ = e.Emit(eventFromHostEvent(cfg, hostEvent))
	})
}

// RunSidecar launches the shared host assembly and exposes runtime events as NDJSON.
func RunSidecar(ctx context.Context, input Input, output io.Writer) error {
	emitter := NewNDJSONEmitter(output)
	_ = emitter.Emit(Event{
		Event:   "runtime.starting",
		Message: "resolving shared worker host assembly",
	})

	runtime, err := NewRuntime(input)
	if err != nil {
		_ = emitter.Emit(Event{
			Event:   "runtime.stopped",
			Message: "worker host assembly failed before startup",
			Error:   errorPayload(err, "runtime_start_failed", "worker host assembly failed"),
		})
		return err
	}

	cfg := runtime.Config()
	runtime.AddObserver(emitter.Observer(cfg))
	if err := runtime.Start(ctx); err != nil {
		_ = emitter.Emit(Event{
			Event:       "runtime.stopped",
			RuntimeID:   cfg.WorkerName,
			WorkspaceID: cfg.WorkspaceID,
			Provider:    cfg.ProviderName,
			Message:     "worker loop failed to start",
			Error:       errorPayload(err, "runtime_start_failed", "worker loop failed to start"),
		})
		return err
	}

	_ = emitter.Emit(Event{
		Event:       "runtime.ready",
		RuntimeID:   cfg.WorkerName,
		WorkspaceID: cfg.WorkspaceID,
		Provider:    cfg.ProviderName,
		Message:     "worker loop is ready for supervision",
	})

	<-ctx.Done()
	stopErr := runtime.Close()
	stoppedEvent := Event{
		Event:       "runtime.stopped",
		RuntimeID:   cfg.WorkerName,
		WorkspaceID: cfg.WorkspaceID,
		Provider:    cfg.ProviderName,
		Message:     "worker loop stopped",
	}
	if stopErr != nil {
		stoppedEvent.Message = "worker loop stopped with error"
		stoppedEvent.Error = errorPayload(stopErr, "runtime_stop_failed", "worker loop failed to stop cleanly")
	}
	_ = emitter.Emit(stoppedEvent)
	return stopErr
}
