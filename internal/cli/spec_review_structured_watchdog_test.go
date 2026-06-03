package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

type blockingStructuredPaneBackend struct{}

func (b *blockingStructuredPaneBackend) Execute(ctx context.Context, _ orchestra.ProviderRequest) (*orchestra.ProviderResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Second):
		return &orchestra.ProviderResponse{Output: `{"verdict":"PASS","summary":"late","findings":[]}`}, nil
	}
}

func (b *blockingStructuredPaneBackend) Name() string {
	return "pane"
}

func captureSpecReviewStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()
	require.NoError(t, w.Close())
	os.Stderr = old
	return <-done
}

func TestSpecReviewWatchdogSeconds_SumsPerProviderTimeouts(t *testing.T) {
	providers := []orchestra.ProviderConfig{
		{Name: "claude", ExecutionTimeout: 480 * time.Second},
		{Name: "codex", ExecutionTimeout: 420 * time.Second},
		{Name: "gemini"}, // no ExecutionTimeout -> uses the fallback
	}
	// 480 + 420 + 150 (fallback for gemini) + 30 base + 3*10 per-provider slack.
	got := specReviewWatchdogSeconds(providers, 150)
	assert.Equal(t, 1110, got)
	// Regression guard: the shared review deadline MUST outlast the longest
	// single provider timeout, otherwise sequential pane execution cancels it
	// mid-run and reports a spurious 0/N watchdog timeout.
	assert.Greater(t, got, 480, "watchdog must outlast the longest per-provider timeout")
}

func TestSpecReviewWatchdogSeconds_FallbackWhenNoProviders(t *testing.T) {
	assert.Equal(t, 200, specReviewWatchdogSeconds(nil, 200))
}

func TestRunStructuredSpecReviewOrchestra_WatchdogSynthesizesPaneFailures(t *testing.T) {
	backend := &blockingStructuredPaneBackend{}

	origFactory := specReviewBackendFactory
	specReviewBackendFactory = func(orchestra.OrchestraConfig) orchestra.ExecutionBackend { return backend }
	defer func() { specReviewBackendFactory = origFactory }()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	var result *orchestra.OrchestraResult
	var err error
	start := time.Now()
	stderr := captureSpecReviewStderr(t, func() {
		result, err = runStructuredSpecReviewOrchestra(ctx, orchestra.OrchestraConfig{
			Providers: []orchestra.ProviderConfig{
				{Name: "claude", Binary: "claude"},
				{Name: "codex", Binary: "codex"},
			},
			Prompt: "Review this SPEC",
		})
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Less(t, time.Since(start), time.Second, "watchdog must return instead of silently waiting on pane backend")
	require.Len(t, result.Responses, 2)
	require.Len(t, result.FailedProviders, 2)
	assert.Equal(t, "timeout", result.FailedProviders[0].FailureClass)
	assert.Equal(t, "spec_review_timeout", result.FailedProviders[0].TimeoutSource)
	assert.Equal(t, "pane", result.FailedProviders[0].CollectionMode)
	assert.True(t, result.Responses[0].TimedOut)
	assert.Contains(t, result.FailedProviders[0].Error, "backend=pane")
	assert.Contains(t, result.FailedProviders[0].Error, "stage=provider_execution")
	assert.Contains(t, result.FailedProviders[1].Error, "stage=queued")
	assert.Contains(t, stderr, "SPEC 리뷰 백엔드: pane")
	assert.Contains(t, stderr, "mode=sequential")
	assert.Contains(t, stderr, "SPEC 리뷰 provider 시작: claude")
	assert.Contains(t, stderr, "SPEC 리뷰 provider 실패: claude")
}
