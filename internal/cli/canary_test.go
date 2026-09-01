package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCmd_CanaryDryRunJSON_SuppliesNoDefaultStagingTargets(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte("project: canary-test\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".autopus", "project"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".autopus", "project", "canary.md"), []byte("# Canary\n"), 0o644))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"canary", "--dry-run", "--project-dir", dir, "--format", "json"})

	require.NoError(t, cmd.Execute())
	payload := decodeJSONMap(t, out.Bytes())
	assertCommonJSONEnvelope(t, payload, "auto canary")
	assert.Equal(t, "ok", payload["status"])

	data := payload["data"].(map[string]any)
	assert.Equal(t, "PASS", data["verdict"])
	assert.Equal(t, "SKIPPED", data["endpoint"])
	assert.Equal(t, "SKIPPED", data["browser"])
	flags := data["flags"].(map[string]any)
	// A third-party project must never inherit Autopus staging hosts: the
	// canary-explicit lane would probe someone else's infrastructure.
	assert.Equal(t, "", flags["frontend_url"])
	assert.Equal(t, "", flags["api_url"])
}

func TestRootCmd_CanaryDryRunJSON_FailsWhenResultCannotBeStored(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	projectPath := filepath.Join(dir, "not-a-directory")
	require.NoError(t, os.WriteFile(projectPath, []byte("not a directory\n"), 0o644))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"canary", "--dry-run", "--project-dir", projectPath, "--format", "json"})

	require.Error(t, cmd.Execute())
	payload := decodeJSONMap(t, out.Bytes())
	assertCommonJSONEnvelope(t, payload, "auto canary")
	assert.Equal(t, "error", payload["status"])

	data := payload["data"].(map[string]any)
	assert.Equal(t, "FAIL", data["verdict"])
}

func TestResolveCanaryTargets_SharedURLOverridesBothChecks(t *testing.T) {
	t.Parallel()

	targets := resolveCanaryTargets(canaryOptions{url: "https://preview.example.com/"})

	assert.Equal(t, "https://preview.example.com", targets.FrontendURL)
	assert.Equal(t, "https://preview.example.com", targets.APIURL)
	assert.Equal(t, []string{"/"}, targets.BrowserPaths)
}

func TestResolveCanaryTargets_UnsetURLsStayEmpty(t *testing.T) {
	t.Parallel()

	targets := resolveCanaryTargets(canaryOptions{})

	assert.Empty(t, targets.FrontendURL)
	assert.Empty(t, targets.APIURL)
}

func TestResolveCanaryTargets_NormalizesBrowserPaths(t *testing.T) {
	t.Parallel()

	targets := resolveCanaryTargets(canaryOptions{
		frontendURL:  "https://app.example.com",
		browserPaths: []string{"login", "  ", "/docs"},
	})

	assert.Equal(t, []string{"/login", "/docs"}, targets.BrowserPaths)
	assert.Empty(t, targets.APIURL)
}
