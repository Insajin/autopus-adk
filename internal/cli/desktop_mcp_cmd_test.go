package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDesktopCmd_ExposesDesktopRuntimeSurface(t *testing.T) {
	t.Parallel()

	cmd := newDesktopCmd()

	assert.Equal(t, "desktop", cmd.Use)
	assert.Len(t, cmd.Commands(), 5)

	status, _, err := cmd.Find([]string{"status"})
	require.NoError(t, err)
	assert.Equal(t, "status", status.Use)

	session, _, err := cmd.Find([]string{"session"})
	require.NoError(t, err)
	assert.Equal(t, "session", session.Use)

	sidecar, _, err := cmd.Find([]string{"sidecar"})
	require.NoError(t, err)
	assert.Equal(t, "sidecar", sidecar.Use)

	ensure, _, err := cmd.Find([]string{"ensure"})
	require.NoError(t, err)
	assert.Equal(t, "ensure", ensure.Use)
}

func TestMCPCmd_ExposesServerSurface(t *testing.T) {
	t.Parallel()

	cmd := newMCPCmd()

	assert.Equal(t, "mcp", cmd.Use)
	server, _, err := cmd.Find([]string{"server"})
	require.NoError(t, err)
	assert.Equal(t, "server", server.Use)
	assert.Contains(t, server.Aliases, "serve")
}
