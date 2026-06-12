package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseSpecFile_FrontmatterStatus parses YAML frontmatter for status and title.
func TestParseSpecFile_FrontmatterStatus(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "spec.md")
	content := "---\ntitle: My Spec\nstatus: approved\n---\n\n# Body"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	status, title := parseSpecFile(path)
	assert.Equal(t, "approved", status)
	assert.Equal(t, "My Spec", title)
}

// TestParseSpecFile_BoldMarkdownFallback parses **Status**: patterns.
func TestParseSpecFile_BoldMarkdownFallback(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "spec.md")
	content := "# SPEC-X-001: Some Title\n\n**Status**: draft\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	status, title := parseSpecFile(path)
	assert.Equal(t, "draft", status)
	assert.Equal(t, "Some Title", title)
}

// TestParseSpecFile_H1TitleWithColonParsed parses "# ID: Title" heading.
func TestParseSpecFile_H1TitleWithColonParsed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "spec.md")
	content := "# SPEC-001: My Feature\n\n**Status**: implemented\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	status, title := parseSpecFile(path)
	assert.Equal(t, "implemented", status)
	assert.Equal(t, "My Feature", title)
}

// TestParseSpecFile_EmptyFile returns the default "draft" status (ParseSpecMetadata default).
func TestParseSpecFile_EmptyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(path, []byte{}, 0o644))

	// ParseSpecMetadata returns "draft" as the default when no status is found.
	status, title := parseSpecFile(path)
	assert.Equal(t, "draft", status)
	assert.Empty(t, title)
}

// TestParseSpecFile_NoFrontmatterBoldStatus falls back to bold-markdown scan.
func TestParseSpecFile_NoFrontmatterBoldStatus(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "spec.md")
	// No YAML frontmatter — bold Status is picked up by the scanner.
	content := "# SPEC-002: Title Only\n\n**Status**: in_progress\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	status, title := parseSpecFile(path)
	// Bold markdown status takes precedence when no frontmatter is present.
	assert.Equal(t, "in_progress", status)
	assert.Equal(t, "Title Only", title)
}

// TestScanAllSpecs_SubmodulePropagated verifies module field is set for submodule specs.
func TestScanAllSpecs_SubmodulePropagated(t *testing.T) {
	t.Parallel()

	base := t.TempDir()

	// Top-level spec.
	topDir := filepath.Join(base, ".autopus", "specs", "SPEC-ROOT-001")
	require.NoError(t, os.MkdirAll(topDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(topDir, "spec.md"),
		[]byte("---\nstatus: completed\ntitle: Root SPEC\n---\n"), 0o644))

	// Submodule spec.
	subDir := filepath.Join(base, "mymodule", ".autopus", "specs", "SPEC-SUB-001")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "spec.md"),
		[]byte("# SPEC-SUB-001: Sub Feature\n\n**Status**: draft\n"), 0o644))

	all := scanAllSpecs(base)
	require.GreaterOrEqual(t, len(all), 2)

	modules := map[string]string{}
	for _, e := range all {
		modules[e.id] = e.module
	}
	assert.Equal(t, "", modules["SPEC-ROOT-001"])
	assert.Equal(t, "mymodule", modules["SPEC-SUB-001"])
}
