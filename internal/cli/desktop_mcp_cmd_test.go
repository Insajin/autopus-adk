package cli

import (
	"bytes"
	"fmt"
	"strings"
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

func TestRuntimeMCPServe_FallsBackToEmbeddedServerWhenHelperMissing(t *testing.T) {
	originalResolver := resolveRuntimeHelper
	resolveRuntimeHelper = func() (string, error) {
		return "", fmt.Errorf("%w; test helper missing", errRuntimeHelperNotFound)
	}
	t.Cleanup(func() { resolveRuntimeHelper = originalResolver })
	t.Setenv("HOME", t.TempDir())

	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"codex","version":"test"}}}` + "\n"
	var out bytes.Buffer
	cmd := newMCPServerCmd()
	cmd.SetIn(strings.NewReader(input))
	cmd.SetOut(&out)

	require.NoError(t, runRuntimeMCPServe(cmd))
	assert.Contains(t, out.String(), `"jsonrpc":"2.0"`)
	assert.Contains(t, out.String(), `"serverInfo":{"name":"autopus-adk"`)
	assert.Contains(t, out.String(), `"protocolVersion":"2024-11-05"`)
}
