//go:build unix

package rulecond_test

// SPEC-STICKYRULE-001 liveness oracle for the counter entry.
//
// Every other containment row asks where a write landed. This one asks whether
// the invocation returned at all. A FIFO is the entry type that turns a refusal
// bug into a hang rather than an escape: the runtime opens the counter name
// O_RDWR, which succeeds on a pipe, and then reads it — and a read of a pipe
// whose only writer is the reading process itself never reaches EOF. The hook
// runs on UserPromptSubmit, so a blocked open or read is a deadlock on every
// prompt of every session in that checkout, not a suppressed injection.
//
// Two independent checks in openCounter stop this: the Lstat pre-check on the
// name, and the regular-file check on the opened descriptor. Either alone is
// sufficient, so this row goes red only when both are gone. It is the pin for
// the failure mode rather than for one line, and it is the only row in the
// package that would catch losing both at once.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/rulecond"
)

const (
	// fifoSession is the session whose own counter name carries the pipe.
	fifoSession = "S-fifo"
	// fifoDeadline bounds the invocation. A healthy run returns immediately, so
	// this is generous enough to survive a loaded machine while still failing
	// loudly instead of hanging the suite until the package timeout fires.
	fifoDeadline = 10 * time.Second
)

// fireOutcome carries one StickyFire result back from the bounding goroutine.
type fireOutcome struct {
	stdout string
	stderr string
	err    error
}

// TestStickyFire_FifoAtTheCounterNameNeverBlocks requires a pipe planted at the
// running session's own counter name to be refused promptly and benignly.
func TestStickyFire_FifoAtTheCounterNameNeverBlocks(t *testing.T) {
	f := newStickyFixture(t)
	f.writeShippedStickyPair()
	require.NoError(t, os.MkdirAll(f.stateDir(), 0o755))

	fifo := filepath.Join(f.stateDir(), rulecond.StickyStateKey(fifoSession))
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}

	// StickyFire runs on its own goroutine and the result arrives on a buffered
	// channel, so a regression fails on the deadline below rather than wedging
	// the test binary until the package timeout kills every other test with it.
	done := make(chan fireOutcome, 1)
	go func() {
		var out, errOut bytes.Buffer
		err := rulecond.StickyFire(f.root, stickyEvent, stickyDefaultCadence,
			strings.NewReader(stickyPayload(fifoSession)), &out, &errOut)
		done <- fireOutcome{stdout: out.String(), stderr: errOut.String(), err: err}
	}()

	select {
	case got := <-done:
		require.NoError(t, got.err,
			"the sticky runtime must stay advisory and never surface an error")
		assert.Empty(t, got.stdout,
			"a refused counter entry injects nothing")
		assert.Empty(t, got.stderr,
			"a refused counter entry is whole-run benign, not a reported violation")
	case <-time.After(fifoDeadline):
		t.Fatalf("StickyFire did not return within %s: a pipe at the counter name "+
			"blocks the hook, which deadlocks every prompt in the checkout", fifoDeadline)
	}

	info, err := os.Lstat(fifo)
	require.NoError(t, err, "the planted entry is refused, not deleted: it is repo content")
	assert.NotZero(t, info.Mode()&os.ModeNamedPipe,
		"the entry must remain the pipe that was planted, never be replaced by a counter file")
}
