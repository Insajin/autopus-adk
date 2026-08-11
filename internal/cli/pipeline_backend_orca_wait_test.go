package cli

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/orcarun"
	"github.com/insajin/autopus-adk/pkg/pipeline"
	"github.com/stretchr/testify/require"
)

// TestPipelineOrcaBackend_BoundsWaitAndOutput covers S104: the supervision loop
// stops at the phase deadline instead of waiting forever, each wait is capped
// by the rolling window, and the transcript read carries the configured limit.
func TestPipelineOrcaBackend_BoundsWaitAndOutput(t *testing.T) {
	fake := newOrcaFakeClient()
	// Empty batches are checkpoints, not failures: they are acknowledged and
	// the loop keeps waiting until the deadline.
	fake.deliveries = []orcarun.Delivery{
		{RunID: "run_test", DeliveryID: "delivery_empty_1"},
		{RunID: "run_test", DeliveryID: "delivery_empty_2"},
	}
	backend := newOrcaTestBackend(t, fake, func(config *pipelineOrcaBackendConfig) {
		config.PhaseTimeout = 60 * time.Millisecond
		config.WaitWindow = 15 * time.Millisecond
	})

	started := time.Now()
	response, err := backend.Execute(context.Background(), orcaPhaseRequest(pipeline.PhaseImplement, 1))
	elapsed := time.Since(started)

	require.NoError(t, err)
	require.True(t, response.TimedOut)
	require.Equal(t, 1, response.ExitCode)
	require.Contains(t, response.FailureClass, "worker_wait_timeout")
	require.Less(t, elapsed, time.Second, "the wait must end at the phase deadline")

	require.Equal(t, []string{"delivery_empty_1", "delivery_empty_2"}, fake.acks)
	require.Equal(t, []int{7}, fake.readLimits, "the transcript read must carry the configured limit")
	require.NotEmpty(t, fake.waitWindows)
	for _, window := range fake.waitWindows {
		require.Positive(t, window)
		require.LessOrEqual(t, window, 15*time.Millisecond)
	}
}

// TestPipelineOrcaBackend_IgnoresForeignDispatchMessages covers the FIFO batch
// semantics: a delivery may carry messages for another dispatch, and only a
// worker_done naming this dispatch may end the attempt. Every delivery that
// has an id is acknowledged so the batch is not replayed forever.
func TestPipelineOrcaBackend_IgnoresForeignDispatchMessages(t *testing.T) {
	fake := newOrcaFakeClient()
	fake.deliveries = []orcarun.Delivery{
		// No delivery id: nothing to acknowledge.
		{RunID: "run_test", Count: 1, Messages: []orcarun.Message{{
			ID: "msg_noack", Type: orcarun.MessageWorkerDone, DispatchID: "ctx_other",
			Outcome: orcarun.OutcomeSucceeded,
		}}},
		orcaWorkerDoneDelivery("delivery_foreign", "ctx_other", orcarun.OutcomeSucceeded, "not mine"),
		orcaWorkerDoneDelivery("delivery_mine", "ctx_1", orcarun.OutcomeSucceeded, "mine"),
	}
	backend := newOrcaTestBackend(t, fake, nil)

	response, err := backend.Execute(context.Background(), orcaPhaseRequest(pipeline.PhasePlan, 1))
	require.NoError(t, err)
	require.Equal(t, 0, response.ExitCode)
	require.False(t, response.TimedOut)
	require.Equal(t, []string{"delivery_foreign", "delivery_mine"}, fake.acks)
	require.Equal(t, []string{"ctx_1"}, fake.released)
}

// TestPipelineOrcaBackend_KeepsWaitingThroughEscalations covers REQ-104: an
// escalation is not an answerable path, so it is recorded as evidence while
// the wait runs to the deadline.
func TestPipelineOrcaBackend_KeepsWaitingThroughEscalations(t *testing.T) {
	fake := newOrcaFakeClient()
	fake.transcript = orcarun.Transcript{Source: "transcript", Status: "empty"}
	fake.deliveries = []orcarun.Delivery{{
		RunID: "run_test", DeliveryID: "delivery_escalation", Count: 1,
		Messages: []orcarun.Message{{
			ID: "msg_escalation", Type: orcarun.MessageEscalation, Subject: "needs a decision",
			Body: "which migration path?", DispatchID: "ctx_1",
		}},
	}}
	backend := newOrcaTestBackend(t, fake, func(config *pipelineOrcaBackendConfig) {
		config.PhaseTimeout = 40 * time.Millisecond
		config.WaitWindow = 10 * time.Millisecond
	})

	response, err := backend.Execute(context.Background(), orcaPhaseRequest(pipeline.PhaseReview, 1))
	require.NoError(t, err)
	require.True(t, response.TimedOut)
	require.Contains(t, response.FailureClass, "worker_awaiting_human")
	require.Contains(t, response.Output, "needs a decision")
	require.Equal(t, []string{"delivery_escalation"}, fake.acks)
	require.Equal(t, []string{"ctx_1"}, fake.abandoned)
}

// TestPipelineOrcaBackend_FallsBackToWorkerDoneBody covers REQ-104: an
// unreadable or empty transcript degrades the output, never the attempt.
func TestPipelineOrcaBackend_FallsBackToWorkerDoneBody(t *testing.T) {
	fake := newOrcaFakeClient()
	fake.transcript = orcarun.Transcript{Source: "terminal", Status: "empty"}
	fake.deliveries = []orcarun.Delivery{
		orcaWorkerDoneDelivery("delivery_1", "ctx_1", orcarun.OutcomeSucceeded, "body of record"),
	}
	backend := newOrcaTestBackend(t, fake, nil)

	response, err := backend.Execute(context.Background(), orcaPhaseRequest(pipeline.PhasePlan, 1))
	require.NoError(t, err)
	require.Equal(t, 0, response.ExitCode)
	require.Equal(t, "body of record", response.Output)
}

// TestPipelineOrcaBackend_ReportedFailureStillReleases covers REQ-105: a worker
// that settled with a failed outcome is a settled worker, so it is released
// while the attempt is reported as failed.
func TestPipelineOrcaBackend_ReportedFailureStillReleases(t *testing.T) {
	fake := newOrcaFakeClient()
	fake.deliveries = []orcarun.Delivery{
		orcaWorkerDoneDelivery("delivery_1", "ctx_1", orcarun.OutcomeFailed, "gate failed"),
	}
	backend := newOrcaTestBackend(t, fake, nil)

	response, err := backend.Execute(context.Background(), orcaPhaseRequest(pipeline.PhaseValidate, 1))
	require.NoError(t, err)
	require.Equal(t, 1, response.ExitCode)
	require.False(t, response.TimedOut)
	require.Contains(t, response.FailureClass, "worker_reported_failed")
	require.Equal(t, []string{"ctx_1"}, fake.released)
	require.Empty(t, fake.abandoned)
}

// TestPipelineOrcaBackend_WaitFailureAbandonsDispatch covers REQ-105: losing
// the delivery channel means the worker cannot be claimed stopped.
func TestPipelineOrcaBackend_WaitFailureAbandonsDispatch(t *testing.T) {
	fake := newOrcaFakeClient()
	fake.waitErr = errors.New("consumer_fenced")
	backend := newOrcaTestBackend(t, fake, nil)

	response, err := backend.Execute(context.Background(), orcaPhaseRequest(pipeline.PhasePlan, 1))
	require.ErrorContains(t, err, "consumer_fenced")
	require.Contains(t, response.FailureClass, "worker_wait_error")
	require.Equal(t, []string{"ctx_1"}, fake.abandoned)
	require.Empty(t, fake.released)
}

// TestPipelineOrcaBackend_CancelledContextStillSettles covers REQ-105 for a
// cancelled run: settlement uses a detached context so the dispatch is fenced
// even though the phase context is already done.
func TestPipelineOrcaBackend_CancelledContextStillSettles(t *testing.T) {
	fake := newOrcaFakeClient()
	fake.waitGate = make(chan struct{})
	fake.waitEntered = make(chan struct{}, 1)
	backend := newOrcaTestBackend(t, fake, nil)

	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan *pipeline.PhaseResponse, 1)
	go func() {
		response, _ := backend.Execute(ctx, orcaPhaseRequest(pipeline.PhasePlan, 1))
		finished <- response
	}()

	select {
	case <-fake.waitEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("worker never entered the supervision wait")
	}
	cancel()

	select {
	case response := <-finished:
		require.Equal(t, 1, response.ExitCode)
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation did not end the attempt")
	}
	require.Equal(t, []string{"ctx_1"}, fake.abandoned)
	require.Empty(t, fake.released)
}
