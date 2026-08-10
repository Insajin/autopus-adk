//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package platform

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// probeGrandchildLifetime is how long the fixture's backgrounded grandchild
// holds the inherited stdout pipe, and doubles as the probe ceiling the exec
// seam injects. Only the inherited-pipe bound can return before it, so the
// elapsed assertion below keeps a wide margin instead of measuring how busy
// the machine is: the healthy path finishes in well under a second, and either
// regression — draining until the grandchild closes the pipe, or riding the
// context to its deadline — takes the full lifetime. The contract is that the
// pipe bound is alive, not that the probe is quick.
const probeGrandchildLifetime = 30 * time.Second

func TestClaudeVersionGrandchildPipeReturnsWithinBound(t *testing.T) {
	script := filepath.Join(t.TempDir(), "claude")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\n(/bin/sleep 30) &\nprintf '2.1.200\\n'\n"), 0o755))
	// The fixture supplies the ceiling instead of claudeVersion's own 2s one:
	// that default sits too close to the pipe bound to tell the two apart.
	ctx, cancel := context.WithTimeout(context.Background(), probeGrandchildLifetime)
	defer cancel()
	patchExecCommand(t, func(context.Context, string, ...string) *exec.Cmd {
		return exec.CommandContext(ctx, script)
	})
	started := time.Now()

	version, err := claudeVersion()

	assert.Error(t, err)
	assert.Empty(t, version)
	assert.Less(t, time.Since(started), probeGrandchildLifetime/3,
		"probe must return on the inherited-pipe bound, not on the grandchild or the deadline")
}
