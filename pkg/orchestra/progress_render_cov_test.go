package orchestra

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// newBufferTracker builds a ProgressTracker writing into a buffer so render
// output can be asserted without touching a real terminal.
func newBufferTracker(names []string, tty bool) (*ProgressTracker, *bytes.Buffer) {
	pt := NewProgressTracker(names)
	buf := &bytes.Buffer{}
	pt.writer = buf
	pt.isTTY = tty
	return pt, buf
}

// TestRenderHeartbeat_NonTTY verifies the non-TTY heartbeat emits a "still
// waiting" line only for running providers.
func TestRenderHeartbeat_NonTTY(t *testing.T) {
	t.Parallel()
	pt, buf := newBufferTracker([]string{"alpha", "beta"}, false)

	pt.providers["alpha"].status = StatusRunning
	pt.providers["alpha"].started = time.Now().Add(-3 * time.Second)
	pt.providers["beta"].status = StatusDone

	pt.RenderHeartbeat()
	out := buf.String()
	assert.Contains(t, out, "alpha", "running provider should appear")
	assert.Contains(t, out, "still waiting")
	assert.NotContains(t, out, "beta", "done provider must not be heartbeated")
}

// TestRenderHeartbeat_TTY verifies the TTY branch renders all providers via
// renderTTY (ANSI control sequences present).
func TestRenderHeartbeat_TTY(t *testing.T) {
	t.Parallel()
	pt, buf := newBufferTracker([]string{"gamma"}, true)
	pt.providers["gamma"].status = StatusRunning
	pt.providers["gamma"].started = time.Now()

	pt.RenderHeartbeat()
	out := buf.String()
	assert.Contains(t, out, "gamma")
	assert.Contains(t, out, "\033[2K", "TTY render should emit clear-line ANSI code")
}

// TestRenderTTY_RepeatMovesCursor verifies the second render emits the
// cursor-up ANSI sequence once content was already rendered.
func TestRenderTTY_RepeatMovesCursor(t *testing.T) {
	t.Parallel()
	pt, buf := newBufferTracker([]string{"one", "two"}, true)
	pt.providers["one"].status = StatusDone
	pt.providers["one"].elapsed = 2 * time.Second

	pt.renderTTY() // first render: no cursor-up yet
	firstLen := buf.Len()
	buf.Reset()

	pt.renderTTY() // second render: should move cursor up by 2 lines
	out := buf.String()
	assert.Positive(t, firstLen)
	assert.Contains(t, out, "\033[2A", "should move cursor up by provider count")
}

// TestMarkRunningDone_NonTTY verifies state transitions render structured log
// lines through the non-TTY path and assert on observable status content.
func TestMarkRunningDone_NonTTY(t *testing.T) {
	t.Parallel()
	pt, buf := newBufferTracker([]string{"svc"}, false)
	pt.MarkRunning("svc")
	pt.MarkDone("svc")
	out := buf.String()
	assert.Contains(t, out, "svc")
	assert.Contains(t, out, "running")
	assert.Contains(t, out, "done")
}
