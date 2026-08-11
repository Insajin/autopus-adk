package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/orcarun"
	"github.com/insajin/autopus-adk/pkg/pipeline"
	"github.com/stretchr/testify/require"
)

// TestPipelineOrcaBackend_CreatesTaskWithoutDependencies covers S102: the task
// carries the phase prompt and an identifiable title, and the policy plane
// keeps ordering because no dependency edge is ever requested.
func TestPipelineOrcaBackend_CreatesTaskWithoutDependencies(t *testing.T) {
	fake := newOrcaFakeClient()
	fake.deliveries = []orcarun.Delivery{
		orcaWorkerDoneDelivery("delivery_1", "ctx_1", orcarun.OutcomeSucceeded, "body"),
	}
	backend := newOrcaTestBackend(t, fake, nil)

	response, err := backend.Execute(context.Background(), orcaPhaseRequest(pipeline.PhasePlan, 2))
	require.NoError(t, err)
	require.Equal(t, 0, response.ExitCode)
	require.Equal(t, "PHASE OUTPUT", response.Output)
	require.Equal(t, "orca", response.Backend)
	require.Equal(t, "claude", response.Provider)
	require.Equal(t, "planner", response.Role)

	require.Len(t, fake.taskCalls, 1)
	require.Equal(t, "run_test", fake.taskCalls[0].RunID)
	require.Equal(t, "phase prompt for plan", fake.taskCalls[0].Spec)
	require.Equal(t, "SPEC-EXECPLANE-002 plan attempt 2", fake.taskCalls[0].Title)
}

// TestPipelineOrcaBackend_FailsClosedOnTaskDependencies covers S102: a task
// that came back with dependency edges means a second DAG exists in the
// process plane, so no worker may start under it.
func TestPipelineOrcaBackend_FailsClosedOnTaskDependencies(t *testing.T) {
	for _, deps := range []string{`["task_other"]`, ""} {
		fake := newOrcaFakeClient()
		fake.task.Deps = deps
		backend := newOrcaTestBackend(t, fake, nil)

		response, err := backend.Execute(context.Background(), orcaPhaseRequest(pipeline.PhaseImplement, 1))
		require.Error(t, err)
		require.Contains(t, err.Error(), "must carry no dependencies")
		require.Equal(t, "task_dependency_violation", response.FailureClass)
		require.Equal(t, 1, response.ExitCode)
		require.Zero(t, fake.countCalls("worker-start"))
	}
}

// TestPipelineOrcaBackend_OneDispatchPerAttempt covers S103: every Execute
// starts exactly one worker and settles it exactly once, and a retry is a new
// attempt with a new dispatch. The Run is created once and reused.
func TestPipelineOrcaBackend_OneDispatchPerAttempt(t *testing.T) {
	fake := newOrcaFakeClient()
	for attempt := 1; attempt <= 3; attempt++ {
		fake.deliveries = append(fake.deliveries, orcaWorkerDoneDelivery(
			fmt.Sprintf("delivery_%d", attempt), fmt.Sprintf("ctx_%d", attempt),
			orcarun.OutcomeSucceeded, "body"))
	}
	backend := newOrcaTestBackend(t, fake, nil)

	for attempt := 1; attempt <= 3; attempt++ {
		response, err := backend.Execute(context.Background(), orcaPhaseRequest(pipeline.PhaseValidate, attempt))
		require.NoError(t, err)
		require.Equal(t, 0, response.ExitCode)
	}
	require.Equal(t, 1, fake.countCalls("run-create"))
	require.Equal(t, 3, fake.countCalls("worker-start"))
	require.Equal(t, []string{"ctx_1", "ctx_2", "ctx_3"}, fake.released)
	require.Empty(t, fake.abandoned)
}

// TestPipelineOrcaBackend_SettlementDependsOnWorkerDone covers S105(a)/(b): a
// settled worker is released, and a worker that never reported is abandoned
// because its process cannot be claimed stopped.
func TestPipelineOrcaBackend_SettlementDependsOnWorkerDone(t *testing.T) {
	t.Run("worker_done releases", func(t *testing.T) {
		fake := newOrcaFakeClient()
		fake.deliveries = []orcarun.Delivery{
			orcaWorkerDoneDelivery("delivery_1", "ctx_1", orcarun.OutcomeSucceeded, "body"),
		}
		backend := newOrcaTestBackend(t, fake, nil)

		_, err := backend.Execute(context.Background(), orcaPhaseRequest(pipeline.PhasePlan, 1))
		require.NoError(t, err)
		require.Equal(t, []string{"ctx_1"}, fake.released)
		require.Empty(t, fake.abandoned)
	})

	t.Run("deadline abandons", func(t *testing.T) {
		fake := newOrcaFakeClient()
		backend := newOrcaTestBackend(t, fake, func(config *pipelineOrcaBackendConfig) {
			config.PhaseTimeout = 40 * time.Millisecond
			config.WaitWindow = 10 * time.Millisecond
		})

		response, err := backend.Execute(context.Background(), orcaPhaseRequest(pipeline.PhasePlan, 1))
		require.NoError(t, err)
		require.True(t, response.TimedOut)
		require.Equal(t, []string{"ctx_1"}, fake.abandoned)
		require.Empty(t, fake.released)
	})
}

// TestPipelineOrcaBackend_ClosesResidualTerminals covers S105(c): a readiness
// failure leaves a terminal the dispatch never owned, and worker-release skips
// it, so the backend closes the reported handle directly.
func TestPipelineOrcaBackend_ClosesResidualTerminals(t *testing.T) {
	fake := newOrcaFakeClient()
	fake.worker = orcarun.Worker{
		DispatchID: "ctx_start_failed", State: "failed", Stage: "agent_readiness",
		LastError: "Agent startup blocked: codex-interactive-prompt",
		Residual: []orcarun.Resource{
			{Kind: "terminal", Role: "agent", ID: "term_leaked", Action: "created"},
			{Kind: "worktree", Role: "worktree", ID: "wt_kept", Action: "reused"},
		},
	}
	backend := newOrcaTestBackend(t, fake, nil)

	response, err := backend.Execute(context.Background(), orcaPhaseRequest(pipeline.PhaseImplement, 1))
	require.NoError(t, err)
	require.Equal(t, 1, response.ExitCode)
	require.Contains(t, response.FailureClass, "worker_not_ready")
	require.Contains(t, response.FailureClass, "stage=agent_readiness")
	require.Contains(t, response.FailureClass, "codex-interactive-prompt")
	require.Equal(t, []string{"term_leaked"}, fake.closedTerms)
	require.Equal(t, []string{"ctx_start_failed"}, fake.abandoned)
	require.Empty(t, fake.released)
}

// TestPipelineOrcaBackend_CloseSettlesInFlightDispatch covers S105(d): Close
// abandons a dispatch that is still supervised, and the racing attempt does
// not settle the same dispatch a second time.
func TestPipelineOrcaBackend_CloseSettlesInFlightDispatch(t *testing.T) {
	fake := newOrcaFakeClient()
	fake.waitGate = make(chan struct{})
	fake.waitEntered = make(chan struct{}, 1)
	fake.deliveries = []orcarun.Delivery{
		orcaWorkerDoneDelivery("delivery_1", "ctx_1", orcarun.OutcomeSucceeded, "body"),
	}
	backend := newOrcaTestBackend(t, fake, nil)

	finished := make(chan struct{})
	go func() {
		defer close(finished)
		_, _ = backend.Execute(context.Background(), orcaPhaseRequest(pipeline.PhasePlan, 1))
	}()

	select {
	case <-fake.waitEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("worker never entered the supervision wait")
	}
	require.NoError(t, backend.Close())
	close(fake.waitGate)
	<-finished

	require.Equal(t, []string{"ctx_1"}, fake.abandoned)
	require.Empty(t, fake.released)
	require.NoError(t, backend.Close(), "Close must be idempotent")
	require.Equal(t, []string{"ctx_1"}, fake.abandoned)
}

// TestPipelineOrcaBackend_SettlesDispatchWhenStartFails covers REQ-105 for a
// transport failure that still produced a dispatch.
func TestPipelineOrcaBackend_SettlesDispatchWhenStartFails(t *testing.T) {
	fake := newOrcaFakeClient()
	fake.worker = orcarun.Worker{DispatchID: "ctx_partial"}
	fake.workerErr = errors.New("orca worker-start transport failure")
	backend := newOrcaTestBackend(t, fake, nil)

	response, err := backend.Execute(context.Background(), orcaPhaseRequest(pipeline.PhasePlan, 1))
	require.ErrorContains(t, err, "transport failure")
	require.Contains(t, response.FailureClass, "worker_start_error")
	require.Equal(t, []string{"ctx_partial"}, fake.abandoned)
}

// TestPipelineOrcaBackend_NeverPassesTierNames covers S107: only opaque
// provider identifiers cross the process-plane boundary. The comparison is on
// whole values, so an opaque id such as "claude-opus-5" is not a violation.
func TestPipelineOrcaBackend_NeverPassesTierNames(t *testing.T) {
	tiers := map[string]bool{"balanced": true, "ultra": true, "opus": true, "sonnet": true, "haiku": true}
	fake := newOrcaFakeClient()
	phases := []pipeline.PhaseID{
		pipeline.PhasePlan, pipeline.PhaseTestScaffold, pipeline.PhaseImplement,
		pipeline.PhaseValidate, pipeline.PhaseReview,
	}
	for index := range phases {
		fake.deliveries = append(fake.deliveries, orcaWorkerDoneDelivery(
			fmt.Sprintf("delivery_%d", index+1), fmt.Sprintf("ctx_%d", index+1),
			orcarun.OutcomeSucceeded, "body"))
	}
	backend := newOrcaTestBackend(t, fake, nil)

	for _, phase := range phases {
		_, err := backend.Execute(context.Background(), orcaPhaseRequest(phase, 1))
		require.NoError(t, err)
	}
	require.Len(t, fake.startCalls, len(phases))
	for _, start := range fake.startCalls {
		require.Equal(t, orcarun.WorktreeCurrent, start.Worktree)
		for field, value := range map[string]string{
			"agent": start.Agent, "model": start.Model, "effort": start.Effort,
		} {
			require.NotEmpty(t, value, "%s must be set", field)
			require.False(t, tiers[strings.ToLower(value)], "%s carried tier name %q", field, value)
		}
	}
}

// TestPipelineOrcaBackend_RoutesAgentAndRefusesUnroutedPhases covers REQ-101: a
// phase launch without an agent falls back to the backend default, and a phase
// with no route at all is refused instead of started under a guessed agent.
func TestPipelineOrcaBackend_RoutesAgentAndRefusesUnroutedPhases(t *testing.T) {
	t.Run("default agent", func(t *testing.T) {
		fake := newOrcaFakeClient()
		fake.deliveries = []orcarun.Delivery{
			orcaWorkerDoneDelivery("delivery_1", "ctx_1", orcarun.OutcomeSucceeded, "body"),
		}
		backend := newOrcaTestBackend(t, fake, func(config *pipelineOrcaBackendConfig) {
			config.Agent = "opencode"
			config.PhaseLaunch[pipeline.PhasePlan] = orcarun.Launch{Model: "sol-1", Effort: "high"}
		})

		response, err := backend.Execute(context.Background(), orcaPhaseRequest(pipeline.PhasePlan, 1))
		require.NoError(t, err)
		require.Equal(t, "opencode", response.Provider)
		require.Equal(t, "opencode", fake.startCalls[0].Agent)
	})

	t.Run("unrouted phase", func(t *testing.T) {
		fake := newOrcaFakeClient()
		backend := newOrcaTestBackend(t, fake, func(config *pipelineOrcaBackendConfig) {
			delete(config.PhaseLaunch, pipeline.PhaseReview)
		})

		response, err := backend.Execute(context.Background(), orcaPhaseRequest(pipeline.PhaseReview, 1))
		require.ErrorContains(t, err, "no launch route for phase review")
		require.Equal(t, 1, response.ExitCode)
		require.Zero(t, fake.countCalls("worker-start"))
		require.Zero(t, fake.countCalls("run-create"))
	})
}
