//go:build !windows

package worker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// terminateProcessGroup with nil cmd returns no_process action and no signals.
func TestTerminateProcessGroup_NilCmd(t *testing.T) {
	t.Parallel()

	evt := terminateProcessGroup(nil, "task-nil")
	assert.Equal(t, "interrupted", evt.Event)
	assert.False(t, evt.SIGTERMSent)
	assert.False(t, evt.SIGKILLSent)
	assert.Contains(t, evt.ActionSequence, "no_process")
}

// prepareCommandProcessGroup with nil must not panic.
func TestPrepareCommandProcessGroup_NilNoOp(t *testing.T) {
	t.Parallel()

	assert.NotPanics(t, func() {
		prepareCommandProcessGroup(nil)
	})
}
