package gemini

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdate_NoManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	pf, err := a.Update(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, pf)
	_, statErr := os.Stat(filepath.Join(dir, "GEMINI.md"))
	assert.NoError(t, statErr)
}

func TestUpdate_WithManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	cfg.ProjectName = "updated-project"
	pf, err := a.Update(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, pf)
	assert.NotEmpty(t, pf.Files)

	data, err := os.ReadFile(filepath.Join(dir, "GEMINI.md"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "updated-project")
}

func TestUpdate_UserModifiedFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	// Modify a managed file to trigger backup
	settingsPath := filepath.Join(dir, ".gemini", "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{"user":"modified"}`), 0644))

	pf, err := a.Update(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, pf)
}

func TestUpdate_DeletedManagedFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	skillsDir := filepath.Join(dir, ".gemini", "skills", "autopus")
	entries, _ := os.ReadDir(skillsDir)
	if len(entries) > 0 {
		require.NoError(t, os.RemoveAll(filepath.Join(skillsDir, entries[0].Name())))
	}

	pf, err := a.Update(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, pf)
}

// --- Marker ---

func TestInjectMarkerSection_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	result, err := a.injectMarkerSection(cfg)
	require.NoError(t, err)
	assert.Contains(t, result, markerBegin)
	assert.Contains(t, result, "test-project")
}

func TestInjectMarkerSection_ExistingMarker(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	existing := "# My Rules\n\n" + markerBegin + "\nold content\n" + markerEnd + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "GEMINI.md"), []byte(existing), 0644))
	result, err := a.injectMarkerSection(cfg)
	require.NoError(t, err)
	assert.Contains(t, result, "My Rules")
	assert.NotContains(t, result, "old content")
}

func TestReplaceMarkerSection(t *testing.T) {
	t.Parallel()
	result := replaceMarkerSection(
		"before\n"+markerBegin+"\nold\n"+markerEnd+"\nafter",
		markerBegin+"\nnew\n"+markerEnd,
	)
	assert.Contains(t, result, "new")
	assert.NotContains(t, result, "old")
}

func TestRemoveMarkerSection(t *testing.T) {
	t.Parallel()
	result := removeMarkerSection("header\n" + markerBegin + "\ncontent\n" + markerEnd + "\nfooter")
	assert.NotContains(t, result, markerBegin)
}

func TestClean_OnlyGeminiMD(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	content := "header\n" + markerBegin + "\ncontent\n" + markerEnd + "\nfooter"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "GEMINI.md"), []byte(content), 0644))
	require.NoError(t, a.Clean(context.Background()))
	data, _ := os.ReadFile(filepath.Join(dir, "GEMINI.md"))
	assert.NotContains(t, string(data), markerBegin)
}
