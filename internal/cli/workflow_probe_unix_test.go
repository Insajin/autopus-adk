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

// probeGrandchildLifetime is how long the fixture's backgrounded grandchild
// holds the inherited stdout pipe, and doubles as the probe ceiling injected
// through newLiveProberWithin. Only the inherited-pipe bound can return before
// it, so the elapsed assertion below keeps a wide margin instead of measuring
// how busy the machine is: the healthy path finishes in well under a second,
// and either regression — draining until the grandchild closes the pipe, or
// riding the context to its deadline — takes the full lifetime. The contract
// is that the pipe bound is alive, not that the probe is quick.
const probeGrandchildLifetime = 30 * time.Second

func TestNewLiveProberGrandchildPipeReturnsDegradedWithinBound(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\n(/bin/sleep 30) &\nprintf '2.1.200\\n'\n"), 0o755))
	t.Setenv("PATH", dir)
	started := time.Now()

	prober := newLiveProberWithin(probeGrandchildLifetime)

	assert.True(t, prober.present)
	assert.Empty(t, prober.version)
	assert.Less(t, time.Since(started), probeGrandchildLifetime/3,
		"probe must return on the inherited-pipe bound, not on the grandchild or the deadline")
}
