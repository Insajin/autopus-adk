//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectInstalledPlatformsDoesNotRunVersionProbe(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "executed")
	script := filepath.Join(dir, "opencode")
	require.NoError(t, os.WriteFile(script,
		[]byte("#!/bin/sh\nprintf executed > \"$AUTOPUS_TEST_EXEC_MARKER\"\n/bin/sleep 5\n"), 0o755))
	t.Setenv("AUTOPUS_TEST_EXEC_MARKER", marker)
	t.Setenv("PATH", dir)
	started := time.Now()

	platforms := detectInstalledPlatforms()

	assert.Equal(t, []string{"opencode"}, platforms)
	assert.Less(t, time.Since(started), 2*time.Second)
	assert.NoFileExists(t, marker)
}
