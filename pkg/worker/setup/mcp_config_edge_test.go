package setup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// WriteMCPConfig: path with missing intermediate dirs must still succeed.
func TestWriteMCPConfig_NestedDirCreation(t *testing.T) {
	t.Parallel()

	cfg, err := GenerateMCPConfig(MCPConfigOptions{
		BackendURL:  "https://api.autopus.co",
		AuthToken:   "tok-nested",
		WorkspaceID: "ws-nested",
	})
	require.NoError(t, err)

	dest := filepath.Join(t.TempDir(), "a", "b", "c", "worker-mcp.json")
	require.NoError(t, WriteMCPConfig(cfg, dest))

	info, err := os.Stat(dest)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

// DefaultMCPConfigPath returns a non-empty path containing "autopus".
func TestDefaultMCPConfigPath_ContainsAutopus(t *testing.T) {
	t.Parallel()

	p := DefaultMCPConfigPath()
	assert.NotEmpty(t, p)
	assert.Contains(t, p, "autopus")
}

// LoadMCPConfig on invalid JSON returns an error.
func TestLoadMCPConfig_InvalidJSONErrors(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("{bad json"), 0o600))

	_, err := LoadMCPConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}
