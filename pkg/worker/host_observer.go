package worker

// HostEventType identifies a machine-consumable worker host event.
type HostEventType string

const (
	HostEventRuntimeDegraded   HostEventType = "runtime_degraded"
	HostEventTaskReceived      HostEventType = "task_received"
	HostEventTaskProgress      HostEventType = "task_progress"
	HostEventTaskCompleted     HostEventType = "task_completed"
	HostEventTaskFailed        HostEventType = "task_failed"
	HostEventApprovalRequested HostEventType = "approval_requested"
	HostEventApprovalResolved  HostEventType = "approval_resolved"
)

// HostEvent carries host-neutral task, progress, and approval signals.
type HostEvent struct {
	Type          HostEventType
	TaskID        string
	ApprovalID    string
	TraceID       string
	CorrelationID string
	Phase         string
	Message       string
	Action        string
	RiskLevel     string
	Context       string
	CostUSD       float64
	DurationMS    int64
	Execution     *HostExecutionContext
	Result        *HostResult
}

// HostExecutionContext describes the retained worker filesystem boundary.
type HostExecutionContext struct {
	WorkspaceID   string
	RootWorkDir   string
	ActiveWorkDir string
	WorktreePath  string
	Mode          string
	BoundaryHint  string
}

// HostArtifact is a desktop-safe projection of a worker artifact.
type HostArtifact struct {
	Name     string
	MimeType string
	Preview  string
	Source   string
}

// HostResult summarizes the retained terminal outcome for desktop.
type HostResult struct {
	Status       string
	Summary      string
	ErrorMessage string
	CostLabel    string
	DurationMS   int64
	SessionID    string
	Artifacts    []HostArtifact
}

// HostObserver receives host-neutral worker events.
type HostObserver interface {
	OnHostEvent(HostEvent)
}

// HostObserverFunc adapts a function into a HostObserver.
type HostObserverFunc func(HostEvent)

// OnHostEvent implements HostObserver.
func (fn HostObserverFunc) OnHostEvent(event HostEvent) {
	fn(event)
}

func (wl *WorkerLoop) AddHostObserver(observer HostObserver) {
	if observer == nil {
		return
	}
	wl.observerMu.Lock()
	defer wl.observerMu.Unlock()
	wl.hostObservers = append(wl.hostObservers, observer)
}

func (wl *WorkerLoop) emitHostEvent(event HostEvent) {
	wl.observerMu.RLock()
	observers := append([]HostObserver(nil), wl.hostObservers...)
	wl.observerMu.RUnlock()

	for _, observer := range observers {
		observer.OnHostEvent(event)
	}
}

func (wl *WorkerLoop) hasHostObservers() bool {
	wl.observerMu.RLock()
	defer wl.observerMu.RUnlock()
	return len(wl.hostObservers) > 0
}
