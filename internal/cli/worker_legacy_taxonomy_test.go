package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerCmd_SeparatesHarnessCoreFromLegacyRuntime(t *testing.T) {
	t.Parallel()

	cmd := newWorkerCmd()

	assert.Contains(t, cmd.Long, "`worker validate` remains harness-core")
	assert.Contains(t, cmd.Long, "legacy local-host compatibility")
	assert.Contains(t, cmd.Long, "`auto desktop ...`")
}

func TestWorkerLegacyRuntimeCommands_DiscloseNonCanonicalMode(t *testing.T) {
	t.Parallel()

	cmd := newWorkerCmd()
	legacyCommands := []string{
		"start",
		"stop",
		"status",
		"logs",
		"restart",
		"history",
		"cost",
		"setup",
	}

	for _, name := range legacyCommands {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			child, _, err := cmd.Find([]string{name})
			require.NoError(t, err)
			assert.False(t, child.Hidden, "%s should remain discoverable as an explicit legacy command", name)
			assert.Contains(t, strings.ToLower(child.Short), "legacy")
			assert.Contains(t, child.Long, "Legacy local-host worker mode only")
			assert.Contains(t, child.Long, "`auto connect`")
			assert.Contains(t, child.Long, "`auto desktop ...`")
		})
	}
}

func TestWorkerCompatibilityShims_RemainHiddenAndDelegateToDesktop(t *testing.T) {
	t.Parallel()

	cmd := newWorkerCmd()
	compatCommands := []string{"sidecar", "session", "ensure", "mcp-server"}

	for _, name := range compatCommands {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			child, _, err := cmd.Find([]string{name})
			require.NoError(t, err)
			assert.True(t, child.Hidden, "%s should not be advertised as a canonical runtime path", name)
			assert.Contains(t, child.Long, "Compatibility")
			assert.Contains(t, child.Long, "desktop")
		})
	}
}
