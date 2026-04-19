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

func TestResolveCompletionDetector_MonitorEnabledUsesFileIPC(t *testing.T) {
	t.Parallel()

	mock := newPlainMock()
	session := newTestHookSession(t)

	resolved := resolveCompletionDetector(OrchestraConfig{
		Terminal:       mock,
		MonitorEnabled: true,
		HookMode:       true,
	}, session)

	_, ok := resolved.detector.(*FileIPCDetector)
	assert.True(t, ok)
	assert.True(t, resolved.eventDriven)
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
	assert.GreaterOrEqual(t, time.Since(start), 4*time.Second)
}
