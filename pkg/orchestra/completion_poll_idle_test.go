package orchestra

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shouldIdleComplete must require the FULL idleThresh of output-file silence,
// anchored to the file's own mtime — not wall-clock since the wait began. This
// keeps a long-running provider that briefly pauses mid-response from being
// completed early (which would truncate the capture).
func TestShouldIdleComplete_RequiresFullOutputSilence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := filepath.Join(dir, "out.txt")
	require.NoError(t, os.WriteFile(f, []byte("partial"), 0o644))

	// Output paused 40s ago, threshold 60s -> provider may still resume; do NOT complete.
	stamp := time.Now().Add(-40 * time.Second)
	require.NoError(t, os.Chtimes(f, stamp, stamp))
	assert.False(t, shouldIdleComplete(f, 60*time.Second, ""),
		"40s silence below 60s threshold must not complete (mid-response pause)")

	// Output quiet 70s, threshold 60s, no working indicator -> complete.
	stamp = time.Now().Add(-70 * time.Second)
	require.NoError(t, os.Chtimes(f, stamp, stamp))
	assert.True(t, shouldIdleComplete(f, 60*time.Second, ""),
		"70s silence at/above 60s threshold completes")

	// Working indicator on screen defers completion even when output is idle.
	assert.False(t, shouldIdleComplete(f, 60*time.Second, "Generating..."),
		"working indicator must defer idle completion")

	// No output file -> no idle signal -> never complete via this path.
	assert.False(t, shouldIdleComplete("", 60*time.Second, ""),
		"missing output file yields no idle completion")
}

// Guard against the hang risk of the rejected screen-reset approach: once the
// output file has been quiet for the provider's IdleThreshold and the final
// screen shows an UNRECOGNIZED prompt (no completion pattern, no working
// indicator), WaitForCompletion must still fire the idle fallback rather than
// spin until the safety deadline.
func TestWaitForCompletion_IdleFallbackFiresOnUnrecognizedFinalScreen(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := filepath.Join(dir, "out.txt")
	require.NoError(t, os.WriteFile(f, []byte("final answer"), 0o644))
	stamp := time.Now().Add(-5 * time.Second)
	require.NoError(t, os.Chtimes(f, stamp, stamp))

	mock := newPlainMock()
	mock.readScreenOutput = "the answer is forty two"

	detector := &ScreenPollDetector{term: mock, safetyDeadline: 2 * time.Second}
	pi := paneInfo{
		paneID:     "pane-1",
		provider:   ProviderConfig{Name: "opencode", IdleThreshold: 1 * time.Second},
		outputFile: f,
	}
	ok, err := detector.WaitForCompletion(context.Background(), pi, DefaultCompletionPatterns(), "", 0)
	assert.NoError(t, err)
	assert.True(t, ok,
		"idle fallback must complete once output is quiet >= IdleThreshold, even with an unrecognized prompt")
}
