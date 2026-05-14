package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

func TestQAInitCmd_CreatesValidatedDesktopGUIJourneyPack(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src-tauri"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src-tauri", "Cargo.toml"), []byte("[package]\nname = \"desktop\"\n"), 0o644))

	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"init", "--project-dir", dir, "--format", "json"})

	require.NoError(t, cmd.Execute())
	payload := decodeJSONMap(t, out.Bytes())
	assert.Equal(t, "ok", payload["status"])
	data := payload["data"].(map[string]any)
	assert.Equal(t, "created", data["status"])

	journeyPath := filepath.Join(dir, ".autopus", "qa", "journeys", "desktop-gui-explore.yaml")
	assert.FileExists(t, journeyPath)
	created := data["created"].([]any)
	require.Len(t, created, 1)
	assert.Equal(t, "desktop-gui-explore", created[0].(map[string]any)["id"])

	pack, err := journey.LoadFile(journeyPath)
	require.NoError(t, err)
	require.NoError(t, journey.Validate(pack, dir))
	assert.Equal(t, "gui-explore", pack.Adapter.ID)
	assert.Contains(t, pack.Lanes, "gui-explore")
	assert.Contains(t, pack.GUI.AllowedOrigins, "http://127.0.0.1:1420")
}

func TestQAInitCmd_PreservesExistingJourneyPack(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src-tauri"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src-tauri", "Cargo.toml"), []byte("[package]\nname = \"desktop\"\n"), 0o644))
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	journeyPath := filepath.Join(journeyDir, "desktop-gui-explore.yaml")
	require.NoError(t, os.WriteFile(journeyPath, []byte("# user-owned journey pack\n"), 0o644))

	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"init", "--project-dir", dir, "--format", "json"})

	require.NoError(t, cmd.Execute())
	data := decodeJSONMap(t, out.Bytes())["data"].(map[string]any)
	assert.Equal(t, "skipped", data["status"])
	assert.NotContains(t, data, "created")
	body, err := os.ReadFile(journeyPath)
	require.NoError(t, err)
	assert.Equal(t, "# user-owned journey pack\n", string(body))
}

func TestQAInitCmd_NoopsWithoutDesktopGUISignals(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"init", "--project-dir", dir, "--format", "json"})

	require.NoError(t, cmd.Execute())
	data := decodeJSONMap(t, out.Bytes())["data"].(map[string]any)
	assert.Equal(t, "noop", data["status"])
	assert.NotEmpty(t, data["warnings"])
	assert.NoFileExists(t, filepath.Join(dir, ".autopus", "qa", "journeys", "desktop-gui-explore.yaml"))
}
