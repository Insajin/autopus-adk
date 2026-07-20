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

func TestClaudeVersionGrandchildPipeReturnsWithinBound(t *testing.T) {
	script := filepath.Join(t.TempDir(), "claude")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\n(/bin/sleep 5) &\nprintf '2.1.200\\n'\n"), 0o755))
	patchExecCommand(t, func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, script)
	})
	started := time.Now()

	version, err := claudeVersion()

	assert.Error(t, err)
	assert.Empty(t, version)
	assert.Less(t, time.Since(started), 2*time.Second)
}
