package setup

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// buildPlistContent produces valid XML with the binary path and log dir embedded.
func TestBuildPlistContent_ContainsRequiredFields(t *testing.T) {
	t.Parallel()

	plist := buildPlistContent("/usr/local/bin/autopus", "/tmp/logs")

	assert.Contains(t, plist, "co.autopus.worker")
	assert.Contains(t, plist, "/usr/local/bin/autopus")
	assert.Contains(t, plist, "/tmp/logs/autopus-worker.out.log")
	assert.Contains(t, plist, "/tmp/logs/autopus-worker.err.log")
	assert.Contains(t, plist, "<true/>") // KeepAlive + RunAtLoad
	assert.True(t, strings.HasPrefix(strings.TrimSpace(plist), "<?xml"))
}

// buildSystemdUnit embeds the binary path as ExecStart.
func TestBuildSystemdUnit_ContainsBinaryPath(t *testing.T) {
	t.Parallel()

	unit := buildSystemdUnit("/usr/local/bin/autopus")

	assert.Contains(t, unit, "[Unit]")
	assert.Contains(t, unit, "[Service]")
	assert.Contains(t, unit, "[Install]")
	assert.Contains(t, unit, "ExecStart=/usr/local/bin/autopus worker start")
	assert.Contains(t, unit, "Restart=always")
	assert.Contains(t, unit, "WantedBy=default.target")
}
