package orchestra

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type blockingDetector struct {
	calls int
}

func (d *blockingDetector) WaitForCompletion(ctx context.Context, _ paneInfo, _ []CompletionPattern, _ string, _ int) (bool, error) {
	d.calls++
	<-ctx.Done()
	return false, nil
}

func TestResolveCompletionDetector_MonitorDisabledForcesPolling(t *testing.T) {
	t.Parallel()

	mock := &signalMock{}
	mock.name = "cmux"

	resolved := resolveCompletionDetector(OrchestraConfig{
		Terminal:       mock,
		MonitorEnabled: false,
	}, nil)

	_, ok := resolved.detector.(*ScreenPollDetector)
	assert.True(t, ok)
	assert.False(t, resolved.eventDriven)
}

func TestResolveCompletionDetector_MonitorEnabledPrefersSignal(t *testing.T) {
	t.Parallel()

	mock := &signalMock{}
	mock.name = "cmux"

	resolved := resolveCompletionDetector(OrchestraConfig{
		Terminal:       mock,
		MonitorEnabled: true,
	}, nil)

	_, ok := resolved.detector.(*SignalDetector)
	assert.True(t, ok)
	assert.True(t, resolved.eventDriven)
}

func TestResolveCompletionDetector_HookModeUsesFileIPCFullBudget(t *testing.T) {
	t.Parallel()

	mock := newPlainMock()
	session := newTestHookSession(t)

	resolved := resolveCompletionDetector(OrchestraConfig{
		Terminal:       mock,
		MonitorEnabled: true,
		HookMode:       true,
	}, session)

	// SPEC-ORCH-022: the done-file IPC detector is a full-budget wait, NOT an
	// event-driven detector — it must not be capped by the monitor pattern timeout
	// and degrade to screen polling (which races the session-dir cleanup against
	// the provider's Stop hook).
	_, ok := resolved.detector.(*FileIPCDetector)
	assert.True(t, ok)
	assert.False(t, resolved.eventDriven)
}

func TestResolveCompletionDetector_HookModeMonitorDisabledStillUsesFileIPC(t *testing.T) {
	t.Parallel()

	// Regression for the SPEC-ORCH-022 BLOCKING bug: when the CC21 monitor feature
	// is OFF (features.Monitor=false → cfg.MonitorEnabled=false) the completion path
	// previously fell straight to the ScreenPollDetector, which returned the instant
	// the response rendered and let the deferred session-dir cleanup remove the
	// directory before the provider's Stop hook wrote the done file. The done-file
	// IPC floor must stay active whenever a hook session exists, regardless of the
	// monitor flag.
	mock := &signalMock{}
	mock.name = "cmux"
	session := newTestHookSession(t)

	resolved := resolveCompletionDetector(OrchestraConfig{
		Terminal:       mock,
		MonitorEnabled: false,
		HookMode:       true,
	}, session)

	_, ok := resolved.detector.(*FileIPCDetector)
	assert.True(t, ok)
	assert.False(t, resolved.eventDriven)
}

func TestWaitForCompletion_HookModeNonHookProviderFallsBackToScreenPoll(t *testing.T) {
	t.Parallel()

	mock := newPlainMock()
	mock.readScreenOutput = "> Type your message\n"
	session := newTestHookSession(t)
	session.SetHookProviders(map[string]bool{"claude": true})

	pi := paneInfo{
		paneID:       "pane-1",
		provider:     ProviderConfig{Name: "gemini", Binary: "agy"},
		role:         "reviewer",
		responseFile: "missing-response.md",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1600*time.Millisecond)
	defer cancel()

	ok := waitForCompletion(ctx, OrchestraConfig{
		Terminal: mock,
		HookMode: true,
	}, pi, DefaultCompletionPatterns(), "", session, 1)

	assert.True(t, ok, "non-hook agy provider must use screen completion instead of hook done-file wait")
}

func TestCompletionInitialDelay_MonitorEnabledShortensWait(t *testing.T) {
	t.Parallel()

	delay := completionInitialDelay(OrchestraConfig{MonitorEnabled: true}, 10*time.Second)
	assert.Equal(t, monitorInitialDelay, delay)
}

func TestWaitForCompletion_MonitorTimeoutFallsBackToPolling(t *testing.T) {
	t.Parallel()

	mock := newPlainMock()
	mock.readScreenOutput = "❯\n"
	detector := &blockingDetector{}
	pi := paneInfo{
		paneID:   "pane-1",
		provider: ProviderConfig{Name: "claude"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	start := time.Now()
	ok := waitForCompletion(ctx, OrchestraConfig{
		Terminal:           mock,
		MonitorEnabled:     true,
		MonitorTimeout:     50 * time.Millisecond,
		CompletionDetector: detector,
	}, pi, DefaultCompletionPatterns(), "", nil, 0)

	require.True(t, ok)
	assert.Equal(t, 1, detector.calls)
	assert.Greater(t, mock.readScreenCalls, 0)
	assert.GreaterOrEqual(t, time.Since(start), 2*screenPollInterval)
}
