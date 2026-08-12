package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/insajin/autopus-adk/pkg/orcarun"
	"github.com/insajin/autopus-adk/pkg/pipeline"
)

const (
	pipelineOrcaBinary              = "orca"
	pipelineOrcaBackendName         = "orca"
	pipelineOrcaTerminalKind        = "terminal"
	pipelineOrcaDefaultStartTimeout = 3 * time.Minute
	pipelineOrcaDefaultPhaseTimeout = 30 * time.Minute
	pipelineOrcaDefaultWaitWindow   = 5 * time.Minute
	pipelineOrcaDefaultReadLimit    = 200
	pipelineOrcaSettleTimeout       = 30 * time.Second
	pipelineOrcaFailureDetailLimit  = 200
)

// orcaOrchestrator is the process-plane seam the orca backend drives. Tests
// substitute a recording fake so no test path reaches a real orca CLI.
type orcaOrchestrator interface {
	CreateRun(ctx context.Context, objective string) (orcarun.Run, error)
	CreateTask(ctx context.Context, runID, title, spec string) (orcarun.Task, error)
	StartWorker(ctx context.Context, req orcarun.StartRequest) (orcarun.Worker, error)
	Wait(ctx context.Context, window time.Duration) (orcarun.Delivery, error)
	Ack(ctx context.Context, deliveryID string) error
	ReadTranscript(ctx context.Context, dispatchID string, limit int) (orcarun.Transcript, error)
	Release(ctx context.Context, dispatchID string) (orcarun.Settlement, error)
	Stop(ctx context.Context, dispatchID string) (orcarun.Settlement, error)
	Abandon(ctx context.Context, dispatchID string) (orcarun.Settlement, error)
	CloseTerminal(ctx context.Context, handle string) error
}

type pipelineOrcaBackendConfig struct {
	SpecID     string
	ProjectDir string
	// Agent is the orca agent used for phases whose launch route names none.
	Agent        string
	PhaseLaunch  map[pipeline.PhaseID]orcarun.Launch
	StartTimeout time.Duration
	PhaseTimeout time.Duration
	WaitWindow   time.Duration
	ReadLimit    int
	Client       orcaOrchestrator
}

// pipelineOrcaBackend runs pipeline phases as orca-supervised workers. The
// policy plane keeps phase ordering, gates, retries, and checkpointing; this
// backend only starts, watches, and settles one worker per phase attempt.
type pipelineOrcaBackend struct {
	mu      sync.Mutex
	config  pipelineOrcaBackendConfig
	runID   string
	pending map[string]struct{}
	closed  bool
}

var _ pipeline.PhaseBackend = (*pipelineOrcaBackend)(nil)
var _ pipeline.PhaseBackendCloser = (*pipelineOrcaBackend)(nil)

func newPipelineOrcaBackend(config pipelineOrcaBackendConfig) (*pipelineOrcaBackend, error) {
	if strings.TrimSpace(config.SpecID) == "" {
		return nil, errors.New("pipeline: orca backend SPEC ID is required")
	}
	if strings.TrimSpace(config.ProjectDir) == "" {
		return nil, errors.New("pipeline: orca backend project directory is required")
	}
	if config.Client == nil {
		return nil, errors.New("pipeline: orca backend orchestrator client is required")
	}
	if config.StartTimeout <= 0 {
		config.StartTimeout = pipelineOrcaDefaultStartTimeout
	}
	if config.PhaseTimeout <= 0 {
		config.PhaseTimeout = pipelineOrcaDefaultPhaseTimeout
	}
	if config.WaitWindow <= 0 {
		config.WaitWindow = pipelineOrcaDefaultWaitWindow
	}
	if config.ReadLimit <= 0 {
		config.ReadLimit = pipelineOrcaDefaultReadLimit
	}
	launch := make(map[pipeline.PhaseID]orcarun.Launch, len(config.PhaseLaunch))
	for phase, entry := range config.PhaseLaunch {
		launch[phase] = entry
	}
	config.PhaseLaunch = launch
	return &pipelineOrcaBackend{config: config, pending: map[string]struct{}{}}, nil
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-EXECPLANE-002: this is the sole phase-to-orca supervised dispatch boundary.
// @AX:WARN [AUTO]: orca phase dispatch branches across routing, run reuse, dependency fail-closed, readiness, waiting, and settlement.
// @AX:REASON [AUTO]: Every exit must settle the started dispatch exactly once and report the outcome through PhaseResponse.
func (backend *pipelineOrcaBackend) Execute(
	ctx context.Context,
	request pipeline.PhaseRequest,
) (*pipeline.PhaseResponse, error) {
	launch, routed := backend.config.PhaseLaunch[request.PhaseID]
	agent := launch.Agent
	if strings.TrimSpace(agent) == "" {
		agent = backend.config.Agent
	}
	response := &pipeline.PhaseResponse{
		Provider: agent, Backend: pipelineOrcaBackendName,
		Role: pipelineOrcaPhaseRole(request.PhaseID), ExitCode: 1,
	}
	if !routed || strings.TrimSpace(agent) == "" {
		response.FailureClass = "execution_error"
		return response, fmt.Errorf("pipeline: orca backend has no launch route for phase %s", request.PhaseID)
	}
	runID, err := backend.ensureRun(ctx)
	if err != nil {
		response.FailureClass = "execution_error"
		return response, err
	}
	title := fmt.Sprintf("%s %s attempt %d", backend.config.SpecID, request.PhaseID, request.Attempt)
	task, err := backend.config.Client.CreateTask(ctx, runID, title, request.Prompt)
	if err != nil {
		response.FailureClass = "execution_error"
		return response, err
	}
	// INV-101: the policy plane owns phase ordering. A task that comes back
	// carrying dependency edges means a second DAG exists in the process
	// plane, so the attempt fails closed instead of running under it.
	if task.Deps != "[]" {
		response.FailureClass = "task_dependency_violation"
		return response, fmt.Errorf(
			"pipeline: orca task %s must carry no dependencies, got %q", task.ID, task.Deps)
	}
	worker, err := backend.startWorker(ctx, runID, task.ID, agent, launch)
	// The dispatch is tracked as soon as it exists, so even a failed start is
	// settled here or by Close and never left live.
	backend.trackDispatch(worker.DispatchID)
	if err != nil {
		settleErr := backend.settleDispatch(ctx, worker.DispatchID, false)
		response.FailureClass = pipelineOrcaFailureClass(
			"worker_start_error", worker.Stage, joinPipelineOrcaDetail("", settleErr))
		return response, err
	}
	if worker.State != orcarun.WorkerStateReady {
		// Readiness failed, so the process cannot be claimed stopped: fence
		// the dispatch with abandon rather than release.
		settleErr := backend.settleDispatch(ctx, worker.DispatchID, false)
		response.FailureClass = pipelineOrcaFailureClass(
			"worker_not_ready", worker.Stage, joinPipelineOrcaDetail(worker.LastError, settleErr))
		return response, nil
	}
	return backend.superviseWorker(ctx, worker.DispatchID, response)
}

// ensureRun creates the orca Run once per backend so every phase attempt of a
// pipeline run shares one supervision scope.
func (backend *pipelineOrcaBackend) ensureRun(ctx context.Context) (string, error) {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.closed {
		return "", errors.New("pipeline: orca backend is closed")
	}
	if backend.runID != "" {
		return backend.runID, nil
	}
	run, err := backend.config.Client.CreateRun(ctx, "autopus pipeline "+backend.config.SpecID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(run.ID) == "" {
		return "", errors.New("pipeline: orca run-create returned no run id")
	}
	backend.runID = run.ID
	return backend.runID, nil
}

// startWorker starts one supervised worker and closes any residual resource
// the start reported. REQ-105: residual terminals are not owned by the
// dispatch, so worker-release skips them and they leak unless closed here.
func (backend *pipelineOrcaBackend) startWorker(
	ctx context.Context,
	runID, taskID, agent string,
	launch orcarun.Launch,
) (orcarun.Worker, error) {
	startCtx, cancel := context.WithTimeout(ctx, backend.config.StartTimeout)
	worker, err := backend.config.Client.StartWorker(startCtx, orcarun.StartRequest{
		RunID: runID, TaskID: taskID, Agent: agent,
		Model: launch.Model, Effort: launch.Effort,
		Worktree: orcarun.WorktreeCurrent, Timeout: backend.config.PhaseTimeout,
	})
	cancel()
	if len(worker.Residual) > 0 {
		cleanupCtx, cleanupCancel := pipelineOrcaCleanupContext(ctx)
		for _, resource := range worker.Residual {
			if resource.Kind != pipelineOrcaTerminalKind || resource.ID == "" {
				continue
			}
			_ = backend.config.Client.CloseTerminal(cleanupCtx, resource.ID)
		}
		cleanupCancel()
	}
	return worker, err
}

// readWorkerOutput builds the phase output from the worker's report first and
// the transcript second.
//
// The report leads because orca requires every worker to send exactly one
// worker_done carrying an executive summary; that is the only channel with a
// guaranteed conclusion. The transcript is supplementary: it is delivered as
// tool calls far more often than as text, so on a live run six of seven phases
// had no assistant text at all. Both are kept when both exist, since the
// summary is deliberately short and the gate reads whatever is here.
func (backend *pipelineOrcaBackend) readWorkerOutput(
	ctx context.Context,
	dispatchID, report string,
) string {
	report = strings.TrimSpace(report)
	transcript, err := backend.config.Client.ReadTranscript(ctx, dispatchID, backend.config.ReadLimit)
	if err != nil {
		return report
	}
	detail := strings.TrimSpace(transcript.Text)
	switch {
	case report == "":
		return detail
	case detail == "" || detail == report:
		return report
	default:
		return report + "\n\n" + detail
	}
}
