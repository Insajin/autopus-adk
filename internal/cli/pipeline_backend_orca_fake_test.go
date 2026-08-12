package cli

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/orcarun"
	"github.com/insajin/autopus-adk/pkg/pipeline"
	"github.com/stretchr/testify/require"
)

// orcaFakeClient records every orchestration call in order. It is the only
// orchestrator the tests use, so no test path reaches a real orca CLI.
type orcaFakeClient struct {
	mu sync.Mutex

	calls       []string
	taskCalls   []orcaFakeTaskCall
	startCalls  []orcarun.StartRequest
	acks        []string
	readLimits  []int
	waitWindows []time.Duration
	closedTerms []string
	released    []string
	abandoned   []string
	stopped     []string
	stopErr     error

	runID      string
	runErr     error
	task       orcarun.Task
	taskErr    error
	worker     orcarun.Worker
	workerErr  error
	deliveries []orcarun.Delivery
	waitErr    error
	transcript orcarun.Transcript
	dispatches int

	// waitGate blocks every Wait until closed; waitEntered is signalled once
	// the first Wait is in flight.
	waitGate    chan struct{}
	waitEntered chan struct{}
}

type orcaFakeTaskCall struct{ RunID, Title, Spec string }

func newOrcaFakeClient() *orcaFakeClient {
	return &orcaFakeClient{
		runID:      "run_test",
		task:       orcarun.Task{ID: "task_test", Status: "ready", Deps: "[]"},
		transcript: orcarun.Transcript{Source: "transcript", Status: "captured", Text: "PHASE OUTPUT"},
	}
}

func (f *orcaFakeClient) record(op string) {
	f.calls = append(f.calls, op)
}

func (f *orcaFakeClient) CreateRun(_ context.Context, objective string) (orcarun.Run, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("run-create")
	if f.runErr != nil {
		return orcarun.Run{}, f.runErr
	}
	return orcarun.Run{ID: f.runID, Objective: objective, CoordinatorHandle: "term_coordinator"}, nil
}

func (f *orcaFakeClient) CreateTask(_ context.Context, runID, title, spec string) (orcarun.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("task-create")
	f.taskCalls = append(f.taskCalls, orcaFakeTaskCall{RunID: runID, Title: title, Spec: spec})
	if f.taskErr != nil {
		return orcarun.Task{}, f.taskErr
	}
	return f.task, nil
}

func (f *orcaFakeClient) StartWorker(_ context.Context, req orcarun.StartRequest) (orcarun.Worker, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("worker-start")
	f.startCalls = append(f.startCalls, req)
	if f.workerErr != nil {
		return f.worker, f.workerErr
	}
	worker := f.worker
	f.dispatches++
	if worker.DispatchID == "" {
		worker.DispatchID = fmt.Sprintf("ctx_%d", f.dispatches)
	}
	if worker.State == "" {
		worker.State = orcarun.WorkerStateReady
	}
	worker.RunID, worker.TaskID = req.RunID, req.TaskID
	return worker, nil
}

func (f *orcaFakeClient) Wait(ctx context.Context, window time.Duration) (orcarun.Delivery, error) {
	f.mu.Lock()
	f.record("check-wait")
	f.waitWindows = append(f.waitWindows, window)
	gate, entered, waitErr := f.waitGate, f.waitEntered, f.waitErr
	f.mu.Unlock()
	if entered != nil {
		select {
		case entered <- struct{}{}:
		default:
		}
	}
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return orcarun.Delivery{}, ctx.Err()
		}
	}
	if waitErr != nil {
		return orcarun.Delivery{}, waitErr
	}
	f.mu.Lock()
	if len(f.deliveries) > 0 {
		delivery := f.deliveries[0]
		f.deliveries = f.deliveries[1:]
		f.mu.Unlock()
		return delivery, nil
	}
	f.mu.Unlock()
	// The real CLI blocks for the whole window and reports a timeout.
	select {
	case <-ctx.Done():
		return orcarun.Delivery{}, ctx.Err()
	case <-time.After(window):
	}
	return orcarun.Delivery{RunID: f.runID, TimedOut: true}, nil
}

func (f *orcaFakeClient) Ack(_ context.Context, deliveryID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("check-ack")
	f.acks = append(f.acks, deliveryID)
	return nil
}

func (f *orcaFakeClient) ReadTranscript(_ context.Context, dispatchID string, limit int) (orcarun.Transcript, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("worker-read")
	f.readLimits = append(f.readLimits, limit)
	transcript := f.transcript
	transcript.DispatchID = dispatchID
	return transcript, nil
}

func (f *orcaFakeClient) Release(_ context.Context, dispatchID string) (orcarun.Settlement, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("worker-release")
	f.released = append(f.released, dispatchID)
	return orcarun.Settlement{DispatchID: dispatchID, State: "released"}, nil
}

func (f *orcaFakeClient) Abandon(_ context.Context, dispatchID string) (orcarun.Settlement, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("worker-abandon")
	f.abandoned = append(f.abandoned, dispatchID)
	return orcarun.Settlement{DispatchID: dispatchID, State: "abandoned"}, nil
}

func (f *orcaFakeClient) Stop(_ context.Context, dispatchID string) (orcarun.Settlement, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("worker-stop")
	if f.stopErr != nil {
		return orcarun.Settlement{}, f.stopErr
	}
	f.stopped = append(f.stopped, dispatchID)
	return orcarun.Settlement{DispatchID: dispatchID, State: "stopped"}, nil
}

func (f *orcaFakeClient) CloseTerminal(_ context.Context, handle string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("terminal-close")
	f.closedTerms = append(f.closedTerms, handle)
	return nil
}

func (f *orcaFakeClient) countCalls(op string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, call := range f.calls {
		if call == op {
			count++
		}
	}
	return count
}

// orcaTestLaunches routes every canonical phase to an opaque provider model id.
// "claude-opus-5" is deliberately present: it proves the tier-vocabulary check
// compares whole values instead of searching for substrings.
func orcaTestLaunches() map[pipeline.PhaseID]orcarun.Launch {
	return map[pipeline.PhaseID]orcarun.Launch{
		pipeline.PhasePlan:         {Agent: "claude", Model: "claude-opus-5", Effort: "xhigh"},
		pipeline.PhaseTestScaffold: {Agent: "codex", Model: "gpt-5.6-sol", Effort: "max"},
		pipeline.PhaseImplement:    {Agent: "codex", Model: "gpt-5.6-sol", Effort: "max"},
		pipeline.PhaseValidate:     {Agent: "claude", Model: "claude-sonnet-5", Effort: "high"},
		pipeline.PhaseReview:       {Agent: "gemini", Model: "gemini-3.5-pro", Effort: "medium"},
	}
}

func newOrcaTestBackend(
	t *testing.T,
	fake *orcaFakeClient,
	mutate func(*pipelineOrcaBackendConfig),
) *pipelineOrcaBackend {
	t.Helper()
	config := pipelineOrcaBackendConfig{
		SpecID: "SPEC-EXECPLANE-002", ProjectDir: t.TempDir(),
		PhaseLaunch:  orcaTestLaunches(),
		StartTimeout: time.Second, PhaseTimeout: 2 * time.Second,
		WaitWindow: 20 * time.Millisecond, ReadLimit: 7, Client: fake,
	}
	if mutate != nil {
		mutate(&config)
	}
	backend, err := newPipelineOrcaBackend(config)
	require.NoError(t, err)
	t.Cleanup(func() { _ = backend.Close() })
	return backend
}

func orcaWorkerDoneDelivery(deliveryID, dispatchID, outcome, body string) orcarun.Delivery {
	return orcarun.Delivery{
		RunID: "run_test", DeliveryID: deliveryID, Count: 1,
		Messages: []orcarun.Message{{
			ID: "msg_" + dispatchID, Type: orcarun.MessageWorkerDone, Subject: "done",
			Body: body, TaskID: "task_test", DispatchID: dispatchID, Outcome: outcome,
		}},
	}
}

func orcaPhaseRequest(phase pipeline.PhaseID, attempt int) pipeline.PhaseRequest {
	return pipeline.PhaseRequest{Prompt: "phase prompt for " + string(phase), PhaseID: phase, Attempt: attempt}
}
