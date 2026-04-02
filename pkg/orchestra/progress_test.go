package orchestra

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgressTracker_MarkRunning(t *testing.T) {
	t.Parallel()
	pt := NewProgressTracker([]string{"claude", "codex"})
	pt.isTTY = false
	var buf bytes.Buffer
	pt.writer = &buf

	pt.MarkRunning("claude")
	s := pt.providers["claude"]
	require.NotNil(t, s)
	assert.Equal(t, StatusRunning, s.status)
}

func TestProgressTracker_MarkDone(t *testing.T) {
	t.Parallel()
	pt := NewProgressTracker([]string{"claude"})
	pt.isTTY = false
	var buf bytes.Buffer
	pt.writer = &buf

	pt.MarkRunning("claude")
	pt.MarkDone("claude")

	s := pt.providers["claude"]
	assert.Equal(t, StatusDone, s.status)
	assert.True(t, s.elapsed > 0)
	assert.Contains(t, buf.String(), "claude")
}

func TestProgressTracker_MarkFailed(t *testing.T) {
	t.Parallel()
	pt := NewProgressTracker([]string{"gemini"})
	pt.isTTY = false
	var buf bytes.Buffer
	pt.writer = &buf

	pt.MarkRunning("gemini")
	pt.MarkFailed("gemini")

	s := pt.providers["gemini"]
	assert.Equal(t, StatusFailed, s.status)
	assert.Contains(t, buf.String(), "gemini")
}

func TestProviderStatus_String(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "✓", StatusDone.String())
	assert.Equal(t, "✗", StatusFailed.String())
}
