// Package setup — coverage tests for daemon_install pure functions.
package setup

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBuildPlistContent verifies the plist template includes the binary path and log dir.
func TestBuildPlistContent(t *testing.T) {
	t.Parallel()

	plist := buildPlistContent("/usr/local/bin/auto", "/var/log/autopus")

	assert.Contains(t, plist, "/usr/local/bin/auto")
	assert.Contains(t, plist, "/var/log/autopus")
	assert.Contains(t, plist, "co.autopus.worker")
	assert.Contains(t, plist, "worker")
	assert.Contains(t, plist, "start")
}

// TestBuildSystemdUnit verifies the systemd unit includes the binary path.
func TestBuildSystemdUnit(t *testing.T) {
	t.Parallel()

	unit := buildSystemdUnit("/usr/local/bin/auto")

	assert.Contains(t, unit, "/usr/local/bin/auto")
	assert.Contains(t, unit, "Autopus Worker Daemon")
	assert.Contains(t, unit, "worker start")
	assert.Contains(t, unit, "Restart=always")
}
