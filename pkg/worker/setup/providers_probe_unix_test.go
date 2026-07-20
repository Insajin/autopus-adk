//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package setup

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectVersionGrandchildPipeReturnsUnknownWithinBound(t *testing.T) {
	script := filepath.Join(t.TempDir(), "opencode")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\n(/bin/sleep 5) &\nprintf '1.2.3\\n'\n"), 0o755))
	started := time.Now()

	version := detectVersion(script)

	assert.Equal(t, "unknown", version)
	assert.Less(t, time.Since(started), 2*time.Second)
}
