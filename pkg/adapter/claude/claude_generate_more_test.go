package claude_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/config"
)

// TestClaudeAdapter_Generate_CleansLegacyCommandDir verifies that Generate
// removes a pre-existing legacy .claude/commands/autopus directory.
func TestClaudeAdapter_Generate_CleansLegacyCommandDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := claude.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	// Simulate a legacy installation: create the commands/autopus directory.
	legacyDir := filepath.Join(dir, ".claude", "commands", "autopus")
	require.NoError(t, os.MkdirAll(legacyDir, 0o755))

	// Also create the legacy auto.md file.
	legacyMD := filepath.Join(dir, ".claude", "commands", "auto.md")
	require.NoError(t, os.WriteFile(legacyMD, []byte("old"), 0o644))

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	// Legacy directory must be removed.
	_, statErr := os.Stat(legacyDir)
	assert.True(t, os.IsNotExist(statErr), "legacy commands/autopus dir must be cleaned up")

	// Legacy auto.md must be removed.
	_, statErr = os.Stat(legacyMD)
	assert.True(t, os.IsNotExist(statErr), "legacy auto.md must be cleaned up")
}

// TestClaudeAdapter_Generate_ProducesFileSizeLimitRule verifies the dynamic
// file-size-limit.md is generated with expected content.
func TestClaudeAdapter_Generate_ProducesFileSizeLimitRule(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := claude.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	pf, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	// Confirm file-size-limit.md is in the generated file list.
	var found bool
	for _, f := range pf.Files {
		if strings.Contains(filepath.ToSlash(f.TargetPath), "file-size-limit.md") {
			found = true
			break
		}
	}
	assert.True(t, found, "file-size-limit.md must be present in generated files")

	// Confirm the file exists on disk.
	rulePath := filepath.Join(dir, ".claude", "rules", "autopus", "file-size-limit.md")
	data, readErr := os.ReadFile(rulePath)
	require.NoError(t, readErr, "file-size-limit.md must exist on disk")
	assert.NotEmpty(t, data, "file-size-limit.md must have content")
}

// TestClaudeAdapter_Generate_InstallsHooksAndPermissions verifies that
// Hook and permission settings are emitted during Generate.
func TestClaudeAdapter_Generate_InstallsHooksAndPermissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := claude.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	cfg.Hooks.PreCommitArch = true

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	// settings.json must be written by InstallHooks.
	data, readErr := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	require.NoError(t, readErr, "settings.json must exist after Generate with hooks")
	assert.Contains(t, string(data), "hooks", "settings.json must contain hook entries")
}

// TestClaudeAdapter_Generate_CopiesRootHookFiles verifies that named CC21
// root hook files are copied to .claude/hooks.
func TestClaudeAdapter_Generate_CopiesRootHookFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := claude.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	hooksDir := filepath.Join(dir, ".claude", "hooks")
	entries, readErr := os.ReadDir(hooksDir)
	require.NoError(t, readErr, ".claude/hooks must exist")
	assert.NotEmpty(t, entries, ".claude/hooks must contain hook files")
}
