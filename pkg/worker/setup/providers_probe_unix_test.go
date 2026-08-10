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

// probeGrandchildLifetime is how long the fixture's backgrounded grandchild
// holds the inherited stdout pipe, and doubles as the probe ceiling. Only the
// inherited-pipe bound can return before it, so the elapsed assertion below
// keeps a wide margin instead of measuring how busy the machine is: the healthy
// path finishes in well under a second, and either regression — draining until
// the grandchild closes the pipe, or riding the context to its deadline — takes
// the full lifetime.
const probeGrandchildLifetime = 30 * time.Second

func TestDetectVersionGrandchildPipeReturnsUnknownWithinBound(t *testing.T) {
	script := filepath.Join(t.TempDir(), "opencode")
	require.NoError(t, os.WriteFile(script, []byte(
		"#!/bin/sh\n(/bin/sleep 30) &\nprintf '1.2.3\\n'\n",
	), 0o755))
	started := time.Now()

	version := detectVersionWithin(script, probeGrandchildLifetime)

	assert.Equal(t, "unknown", version)
	assert.Less(t, time.Since(started), probeGrandchildLifetime/3,
		"probe must return on the inherited-pipe bound, not on the grandchild or the deadline")
}
