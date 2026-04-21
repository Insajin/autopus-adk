package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkerMCPServerCmd_UsesNewNameWithLegacyAlias(t *testing.T) {
	t.Parallel()

	cmd := newWorkerMCPServerCmd()

	assert.Equal(t, "mcp-server", cmd.Use)
	assert.Contains(t, cmd.Aliases, "mcp-serve")
	assert.True(t, cmd.Hidden)
}
