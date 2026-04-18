// Package cli tests additional internal orchestra helper functions.
package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

// TestBuildReviewPrompt_MissingFile verifies that a missing file aborts the
// prompt build instead of silently degrading the prompt with a "읽기 실패"
// marker (GitHub issue #37). Degrading produced prompts that sent providers
// the file path string plus an ENOENT, causing every provider to respond
// "리뷰 불가".
func TestBuildReviewPrompt_MissingFile(t *testing.T) {
	t.Parallel()

	_, err := buildReviewPrompt([]string{"/nonexistent/file.go"})
	require.Error(t, err, "missing file must abort prompt build")
	assert.Contains(t, err.Error(), "/nonexistent/file.go",
		"error must cite the offending path so users can diagnose")
}

// TestBuildReviewPrompt_EmptyFilesReturnsFallback confirms empty input still
// produces the project-level review prompt without error.
func TestBuildReviewPrompt_EmptyFilesReturnsFallback(t *testing.T) {
	t.Parallel()

	prompt, err := buildReviewPrompt(nil)
	require.NoError(t, err)
	assert.Contains(t, prompt, "현재 프로젝트의 코드를 리뷰")
}

// TestBuildSecurePrompt_MissingFile_Aborts parallels the review test for the
// secure command (same bug surface, same root cause).
func TestBuildSecurePrompt_MissingFile_Aborts(t *testing.T) {
	t.Parallel()

	_, err := buildSecurePrompt([]string{"/nonexistent/file.go"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/nonexistent/file.go")
}

// TestBuildFileContents_EmptySlice verifies buildFileContents returns empty
// string (and no error) for a nil/empty file list.
func TestBuildFileContents_EmptySlice(t *testing.T) {
	t.Parallel()

	result, err := buildFileContents(nil)
	require.NoError(t, err)
	assert.Empty(t, result)

	result2, err := buildFileContents([]string{})
	require.NoError(t, err)
	assert.Empty(t, result2)
}

// TestBuildFileContents_AllExistingInlined verifies all-readable files are
// inlined with the expected delimiter format.
func TestBuildFileContents_AllExistingInlined(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := filepath.Join(dir, "a.go")
	b := filepath.Join(dir, "b.go")
	require.NoError(t, os.WriteFile(a, []byte("package a\n"), 0o644))
	require.NoError(t, os.WriteFile(b, []byte("package b\n"), 0o644))

	result, err := buildFileContents([]string{a, b})
	require.NoError(t, err)
	assert.Contains(t, result, "package a")
	assert.Contains(t, result, "package b")
	assert.NotContains(t, result, "읽기 실패")
}

// TestBuildFileContents_MissingFileReturnsError guarantees a single missing
// entry forces the whole batch to abort — callers must not proceed with
// partial content (GitHub issue #37).
func TestBuildFileContents_MissingFileReturnsError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	existing := filepath.Join(dir, "real.go")
	require.NoError(t, os.WriteFile(existing, []byte("package real\n"), 0o644))

	result, err := buildFileContents([]string{existing, "/nonexistent/missing.go"})
	require.Error(t, err)
	assert.Empty(t, result, "callers must not receive partial content when any read fails")
	assert.Contains(t, err.Error(), "/nonexistent/missing.go")
}

// TestResolveStrategy_EmptyCommandStrategy verifies the global default is used
// when the command exists but has an empty strategy string.
func TestResolveStrategy_EmptyCommandStrategy(t *testing.T) {
	t.Parallel()

	conf := &config.OrchestraConf{
		DefaultStrategy: "pipeline",
		Commands: map[string]config.CommandEntry{
			"review": {Strategy: ""}, // empty strategy in command
		},
	}

	result := resolveStrategy(conf, "review", "")
	assert.Equal(t, "pipeline", result,
		"global default must be used when command strategy is empty")
}

// TestResolveProviderNames_EmptyCommandProviders verifies the global provider
// fallback is used when the command's provider list is empty.
func TestResolveProviderNames_EmptyCommandProviders(t *testing.T) {
	t.Parallel()

	conf := &config.OrchestraConf{
		Providers: map[string]config.ProviderEntry{
			"claude": {Binary: "claude"},
			"gemini": {Binary: "gemini"},
		},
		Commands: map[string]config.CommandEntry{
			"plan": {Providers: nil}, // empty providers list
		},
	}

	names := resolveProviderNames(conf, "plan", nil)
	// Should fall back to all global providers.
	assert.Len(t, names, 2)
}

// TestBuildProviderConfigs_MixedKnownUnknown verifies mixed known/unknown
// providers are handled correctly in a single call.
func TestBuildProviderConfigs_MixedKnownUnknown(t *testing.T) {
	t.Parallel()

	configs := buildProviderConfigs([]string{"claude", "my-tool"})
	require.Len(t, configs, 2)

	assert.Equal(t, "claude", configs[0].Binary, "claude must use hardcoded binary")
	assert.Equal(t, "my-tool", configs[1].Binary, "unknown provider must use name as binary")
}

// TestResolveRounds verifies debate round resolution logic.
func TestResolveRounds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		strategy string
		rounds   int
		expected int
	}{
		{"explicit rounds respected", "debate", 5, 5},
		{"debate defaults to 2", "debate", 0, 2},
		{"non-debate zero rounds", "consensus", 0, 0},
		{"non-debate explicit rounds", "pipeline", 3, 3},
		{"empty strategy zero rounds", "", 0, 0},
		{"negative rounds treated as zero", "debate", -1, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolveRounds(tt.strategy, tt.rounds)
			assert.Equal(t, tt.expected, got)
		})
	}
}
