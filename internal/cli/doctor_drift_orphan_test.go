package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

// writeManifestFile creates an empty-body manifest file under dir/.autopus.
func writeManifestFile(t *testing.T, dir, name string) {
	t.Helper()
	autopusDir := filepath.Join(dir, ".autopus")
	require.NoError(t, os.MkdirAll(autopusDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(autopusDir, name), []byte("{}"), 0o644))
}

// TestOrphanManifest_GeminiCliOrphan is the S4 oracle: with platforms
// [claude-code, codex, antigravity-cli, opencode] and both an antigravity-cli
// and a gemini-cli manifest present, only the gemini-cli manifest is orphan —
// count 1, exact path, and a removal hint with the alias successor.
func TestOrphanManifest_GeminiCliOrphan(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.HarnessConfig{Platforms: []string{"claude-code", "codex", "antigravity-cli", "opencode"}}

	writeManifestFile(t, dir, "antigravity-cli-manifest.json")
	writeManifestFile(t, dir, "gemini-cli-manifest.json")

	res := detectOrphanManifests(dir, cfg)
	assert.True(t, res.Present, "manifests exist so the check is not skipped")
	assert.Equal(t, []string{".autopus/gemini-cli-manifest.json"}, res.Paths,
		"only the unconfigured gemini-cli manifest is orphan")
	assert.Equal(t, "antigravity-cli", res.Aliases["gemini-cli"], "legacy alias records its successor")

	check := orphanManifestCheck(res)
	assert.Equal(t, "doctor.drift.orphan_manifest", check.ID)
	assert.Equal(t, "warn", check.Status)
	assert.Equal(t, "warning", check.Severity)
	assert.Contains(t, check.Detail, ".autopus/gemini-cli-manifest.json")
	assert.NotContains(t, check.Detail, "antigravity-cli-manifest.json",
		"the configured manifest is not listed as orphan")
	assert.Contains(t, check.Detail, "rm", "detail carries the removal hint")
	assert.Contains(t, check.Detail, "superseded by antigravity-cli")
}

// TestOrphanManifest_AllConfigured_Pass verifies a clean install: every manifest
// maps to a configured platform, so the check is present but passes with no
// orphan paths.
func TestOrphanManifest_AllConfigured_Pass(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.HarnessConfig{Platforms: []string{"claude-code", "codex"}}
	writeManifestFile(t, dir, "claude-code-manifest.json")
	writeManifestFile(t, dir, "codex-manifest.json")

	res := detectOrphanManifests(dir, cfg)
	assert.True(t, res.Present)
	assert.Empty(t, res.Paths)
	assert.Equal(t, "pass", orphanManifestCheck(res).Status)
}

// TestOrphanManifest_NoManifests_SilentSkip verifies REQ-008: with no
// .autopus/*-manifest.json present, the check is skipped entirely.
func TestOrphanManifest_NoManifests_SilentSkip(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.HarnessConfig{Platforms: []string{"claude-code"}}

	res := detectOrphanManifests(dir, cfg)
	assert.False(t, res.Present, "no manifests → no check emitted")
	assert.Empty(t, res.Paths)
}
