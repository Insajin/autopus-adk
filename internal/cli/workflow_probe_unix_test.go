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

func TestNewLiveProberGrandchildPipeReturnsDegradedWithinBound(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\n(/bin/sleep 5) &\nprintf '2.1.200\\n'\n"), 0o755))
	t.Setenv("PATH", dir)
	started := time.Now()

	prober := newLiveProber()

	assert.True(t, prober.present)
	assert.Empty(t, prober.version)
	assert.Less(t, time.Since(started), 2*time.Second)
}
