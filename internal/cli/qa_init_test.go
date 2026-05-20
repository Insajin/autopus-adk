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
	assertContainsCreatedID(t, created, "desktop-gui-explore")
	assert.FileExists(t, filepath.Join(dir, ".autopus", "qa", "journeys", "canary-explicit.yaml"))
	assert.FileExists(t, filepath.Join(dir, ".github", "workflows", "autopus-qa-release.yml"))
	assert.FileExists(t, filepath.Join(dir, ".autopus", "qa", "domain-readiness", "catalog.json"))

	pack, err := journey.LoadFile(journeyPath)
	require.NoError(t, err)
	require.NoError(t, journey.Validate(pack, dir))
	assert.Equal(t, "gui-explore", pack.Adapter.ID)
	assert.Contains(t, pack.Lanes, "gui-explore")
	assert.Contains(t, pack.GUI.AllowedOrigins, "http://127.0.0.1:1420")
}

func TestQAInitCmd_CreatesDetectedGoFastJourneyPack(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n\ngo 1.26\n"), 0o644))

	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"init", "--project-dir", dir, "--format", "json"})

	require.NoError(t, cmd.Execute())
	data := decodeJSONMap(t, out.Bytes())["data"].(map[string]any)
	assert.Equal(t, "created", data["status"])

	journeyPath := filepath.Join(dir, ".autopus", "qa", "journeys", "go-fast.yaml")
	assert.FileExists(t, journeyPath)
	pack, err := journey.LoadFile(journeyPath)
	require.NoError(t, err)
	require.NoError(t, journey.Validate(pack, dir))
	assert.Equal(t, "go-test", pack.Adapter.ID)
	assert.Equal(t, []string{"go", "test", "./..."}, pack.Command.Argv)
	assert.FileExists(t, filepath.Join(dir, ".autopus", "qa", "journeys", "canary-explicit.yaml"))
	assert.FileExists(t, filepath.Join(dir, ".github", "workflows", "autopus-qa-release.yml"))
	assert.FileExists(t, filepath.Join(dir, ".autopus", "qa", "domain-readiness", "catalog.json"))
}

func TestQAInitCmd_DefaultScaffoldsReleaseWorkflowGate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	packageJSON := []byte(`{
  "scripts": {
    "test": "vitest run",
    "build": "vite build"
  },
  "devDependencies": {
    "@playwright/test": "^1.0.0",
    "vitest": "^1.0.0"
  }
}`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), packageJSON, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte("{}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "playwright.config.ts"), []byte("export default {}\n"), 0o644))

	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"init", "--project-dir", dir, "--format", "json"})

	require.NoError(t, cmd.Execute())
	data := decodeJSONMap(t, out.Bytes())["data"].(map[string]any)
	assert.Equal(t, "created", data["status"])

	for _, name := range []string{"node-fast.yaml", "browser-staging-playwright.yaml", "canary-explicit.yaml"} {
		path := filepath.Join(dir, ".autopus", "qa", "journeys", name)
		assert.FileExists(t, path)
		pack, err := journey.LoadFile(path)
		require.NoError(t, err)
		require.NoError(t, journey.Validate(pack, dir))
	}
	workflowPath := filepath.Join(dir, ".github", "workflows", "autopus-qa-release.yml")
	body, err := os.ReadFile(workflowPath)
	require.NoError(t, err)
	assert.Contains(t, string(body), "auto qa release --project-dir . --profile")
	assert.Contains(t, string(body), "npm ci")
	assert.Contains(t, string(body), "npx playwright install chromium --with-deps")
	assert.FileExists(t, filepath.Join(dir, ".autopus", "qa", "domain-readiness", "catalog.json"))
}

func TestQABootstrapCmd_UsesReleaseWorkflowDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n\ngo 1.26\n"), 0o644))

	cmd := newQACmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"bootstrap", "--project-dir", dir, "--format", "json"})

	require.NoError(t, cmd.Execute())
	payload := decodeJSONMap(t, out.Bytes())
	assertCommonJSONEnvelope(t, payload, "qa bootstrap")
	data := payload["data"].(map[string]any)
	assert.Equal(t, "created", data["status"])
	assert.FileExists(t, filepath.Join(dir, ".autopus", "qa", "journeys", "go-fast.yaml"))
	assert.FileExists(t, filepath.Join(dir, ".autopus", "qa", "journeys", "canary-explicit.yaml"))
	assert.FileExists(t, filepath.Join(dir, ".github", "workflows", "autopus-qa-release.yml"))
	assert.FileExists(t, filepath.Join(dir, ".autopus", "qa", "domain-readiness", "catalog.json"))
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
	cmd.SetArgs([]string{"init", "--project-dir", dir, "--local-only", "--format", "json"})

	require.NoError(t, cmd.Execute())
	data := decodeJSONMap(t, out.Bytes())["data"].(map[string]any)
	assert.Equal(t, "created", data["status"])
	assertContainsCreatedID(t, data["created"].([]any), "domain-readiness-catalog")
	body, err := os.ReadFile(journeyPath)
	require.NoError(t, err)
	assert.Equal(t, "# user-owned journey pack\n", string(body))
}

func TestQAInitCmd_LocalOnlySkipsReleaseWorkflow(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n\ngo 1.26\n"), 0o644))

	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"init", "--project-dir", dir, "--local-only", "--format", "json"})

	require.NoError(t, cmd.Execute())
	data := decodeJSONMap(t, out.Bytes())["data"].(map[string]any)
	assert.Equal(t, "created", data["status"])
	assert.FileExists(t, filepath.Join(dir, ".autopus", "qa", "journeys", "go-fast.yaml"))
	assert.FileExists(t, filepath.Join(dir, ".autopus", "qa", "domain-readiness", "catalog.json"))
	assert.NoFileExists(t, filepath.Join(dir, ".autopus", "qa", "journeys", "canary-explicit.yaml"))
	assert.NoFileExists(t, filepath.Join(dir, ".github", "workflows", "autopus-qa-release.yml"))
}

func TestQAInitCmd_LocalOnlyNoopsWithoutQASignals(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"init", "--project-dir", dir, "--local-only", "--format", "json"})

	require.NoError(t, cmd.Execute())
	data := decodeJSONMap(t, out.Bytes())["data"].(map[string]any)
	assert.Equal(t, "noop", data["status"])
	assert.NotEmpty(t, data["warnings"])
	assert.NoFileExists(t, filepath.Join(dir, ".autopus", "qa", "journeys", "desktop-gui-explore.yaml"))
	assert.NoFileExists(t, filepath.Join(dir, ".autopus", "qa", "domain-readiness", "catalog.json"))
}

func assertContainsCreatedID(t *testing.T, created []any, id string) {
	t.Helper()
	for _, item := range created {
		if item.(map[string]any)["id"] == id {
			return
		}
	}
	t.Fatalf("created files did not include %q: %#v", id, created)
}
