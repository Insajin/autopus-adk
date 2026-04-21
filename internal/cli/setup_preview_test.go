package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	setupdocs "github.com/insajin/autopus-adk/pkg/setup"
)

func TestSetupGenerateCmd_PreviewNoWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/preview\n\ngo 1.23\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))

	var out bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"setup", "generate", dir, "--plan"})
	require.NoError(t, cmd.Execute())

	assert.NoDirExists(t, filepath.Join(dir, ".autopus", "docs"))
	assert.Contains(t, out.String(), ".autopus/docs/index.md")
	assert.Contains(t, out.String(), "[tracked_docs] create")
}

func TestSetupUpdateCmd_PreviewNoWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/preview\n\ngo 1.23\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))

	_, err := setupdocs.Generate(dir, nil)
	require.NoError(t, err)

	commandsPath := filepath.Join(dir, ".autopus", "docs", "commands.md")
	before, err := os.ReadFile(commandsPath)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "Makefile"), []byte("build:\n\tgo build ./...\n"), 0o644))

	var out bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"setup", "update", dir, "--plan"})
	require.NoError(t, cmd.Execute())

	after, err := os.ReadFile(commandsPath)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "preview must not rewrite docs")
	assert.Contains(t, out.String(), "[tracked_docs] update")
	assert.Contains(t, out.String(), ".autopus/project/scenarios.md")
}

func TestSetupGenerateCmd_PreviewShowsMultiRepoHint(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("[remote \"origin\"]\n\turl = git@example.com/root.git\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/root\n\ngo 1.23\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))

	childDir := filepath.Join(dir, "bridge")
	require.NoError(t, os.MkdirAll(filepath.Join(childDir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(childDir, ".git", "config"), []byte("[remote \"origin\"]\n\turl = git@example.com/bridge.git\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "go.mod"), []byte("module example.com/bridge\n\ngo 1.23\n"), 0o644))

	var out bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"setup", "generate", dir, "--plan"})
	require.NoError(t, cmd.Execute())

	output := out.String()
	assert.Contains(t, output, "multi-repo workspace detected")
	assert.Contains(t, output, "owning repo")
	assert.Contains(t, output, "repo: "+filepath.Base(dir))
	assert.Contains(t, output, "source-of-truth: "+filepath.ToSlash(dir))
	assert.NoDirExists(t, filepath.Join(dir, ".autopus", "docs"))
}
