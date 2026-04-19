package setup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateMCPConfig_Valid(t *testing.T) {
	t.Parallel()

	config, err := GenerateMCPConfig(MCPConfigOptions{
		BackendURL:  "https://api.autopus.co",
		AuthToken:   "tok-123",
		WorkspaceID: "ws-456",
	})

	require.NoError(t, err)
	require.NotNil(t, config)

	srv, ok := config.MCPServers["autopus"]
	require.True(t, ok)
	assert.Equal(t, "https://api.autopus.co/mcp/sse", srv.URL)
	assert.Equal(t, "sse", srv.Transport)
	assert.Equal(t, "Bearer tok-123", srv.Headers["Authorization"])
	assert.Equal(t, "ws-456", srv.Headers["X-Workspace-ID"])
}

func TestGenerateMCPConfig_NoWorkspaceID(t *testing.T) {
	t.Parallel()

	config, err := GenerateMCPConfig(MCPConfigOptions{
		BackendURL: "https://api.autopus.co",
		AuthToken:  "tok-123",
	})

	require.NoError(t, err)
	_, hasWS := config.MCPServers["autopus"].Headers["X-Workspace-ID"]
	assert.False(t, hasWS, "X-Workspace-ID should not be set when empty")
}

func TestGenerateMCPConfig_MissingBackendURL(t *testing.T) {
	t.Parallel()

	_, err := GenerateMCPConfig(MCPConfigOptions{
		AuthToken: "tok-123",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BackendURL is required")
}

func TestGenerateMCPConfig_MissingAuthToken(t *testing.T) {
	t.Parallel()

	_, err := GenerateMCPConfig(MCPConfigOptions{
		BackendURL: "https://api.autopus.co",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AuthToken is required")
}

func TestWriteMCPConfig_Permissions(t *testing.T) {
	t.Parallel()

	config, err := GenerateMCPConfig(MCPConfigOptions{
		BackendURL: "https://api.autopus.co",
		AuthToken:  "secret",
	})
	require.NoError(t, err)

	dir := t.TempDir()
	path := filepath.Join(dir, "config", "worker-mcp.json")

	require.NoError(t, WriteMCPConfig(config, path))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestWriteMCPConfig_CreatesParentDirs(t *testing.T) {
	t.Parallel()

	config, err := GenerateMCPConfig(MCPConfigOptions{
		BackendURL: "https://api.autopus.co",
		AuthToken:  "secret",
	})
	require.NoError(t, err)

	dir := t.TempDir()
	path := filepath.Join(dir, "deep", "nested", "worker-mcp.json")

	require.NoError(t, WriteMCPConfig(config, path))

	_, err = os.Stat(path)
	require.NoError(t, err)
}

func TestWriteAndLoadMCPConfig_RoundTrip(t *testing.T) {
	t.Parallel()

	original, err := GenerateMCPConfig(MCPConfigOptions{
		BackendURL:  "https://api.autopus.co",
		AuthToken:   "round-trip-tok",
		WorkspaceID: "ws-rt",
	})
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "worker-mcp.json")
	require.NoError(t, WriteMCPConfig(original, path))

	loaded, err := LoadMCPConfig(path)
	require.NoError(t, err)

	srv := loaded.MCPServers["autopus"]
	assert.Equal(t, "https://api.autopus.co/mcp/sse", srv.URL)
	assert.Equal(t, "sse", srv.Transport)
	assert.Equal(t, "Bearer round-trip-tok", srv.Headers["Authorization"])
	assert.Equal(t, "ws-rt", srv.Headers["X-Workspace-ID"])
}

func TestLoadMCPConfig_FileNotFound(t *testing.T) {
	t.Parallel()

	_, err := LoadMCPConfig("/nonexistent/path/config.json")
	require.Error(t, err)
}

func TestWriteMCPConfig_InvalidDir(t *testing.T) {
	t.Parallel()

	config, err := GenerateMCPConfig(MCPConfigOptions{
		BackendURL: "https://api.autopus.co",
		AuthToken:  "secret",
	})
	require.NoError(t, err)

	// Write to a path where directory creation is impossible.
	err = WriteMCPConfig(config, "/dev/null/impossible/worker-mcp.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create config dir")
}

func TestWriteMCPConfig_ReadOnlyDir(t *testing.T) {
	t.Parallel()

	config, err := GenerateMCPConfig(MCPConfigOptions{
		BackendURL: "https://api.autopus.co",
		AuthToken:  "secret",
	})
	require.NoError(t, err)

	// Create a directory then make it read-only to prevent temp file creation.
	dir := t.TempDir()
	readOnly := filepath.Join(dir, "readonly")
	require.NoError(t, os.MkdirAll(readOnly, 0500))
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0700) })

	err = WriteMCPConfig(config, filepath.Join(readOnly, "worker-mcp.json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create temp file")
}

func TestWriteMCPConfig_RenameConflict(t *testing.T) {
	t.Parallel()

	config, err := GenerateMCPConfig(MCPConfigOptions{
		BackendURL: "https://api.autopus.co",
		AuthToken:  "secret",
	})
	require.NoError(t, err)

	dir := t.TempDir()
	target := filepath.Join(dir, "worker-mcp.json")

	// Create target as a non-empty directory to force rename failure.
	require.NoError(t, os.MkdirAll(filepath.Join(target, "blocker"), 0700))

	err = WriteMCPConfig(config, target)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rename mcp config")
}

func TestLoadMCPConfig_InvalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("not-json"), 0600))

	_, err := LoadMCPConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal mcp config")
}

func TestDefaultMCPConfigPath(t *testing.T) {
	t.Parallel()

	path := DefaultMCPConfigPath()
	assert.Contains(t, path, "worker-mcp.json")
	assert.Contains(t, path, ".config")
}
