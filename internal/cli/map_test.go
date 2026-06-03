package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCmd_RegistersMap(t *testing.T) {
	t.Parallel()

	cmd, _, err := NewRootCmd().Find([]string{"map"})
	require.NoError(t, err)
	require.NotNil(t, cmd)
	assert.Equal(t, "map", cmd.Name())
}

func TestMapPayload_IncludesNestedRepoState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	initTestGitRepo(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".mcp.json\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte("{}\n"), 0o644))
	runGitCommand(t, dir, "add", "-f", ".mcp.json")
	runGitCommand(t, dir, "commit", "-m", "track mcp")

	child := filepath.Join(dir, "child")
	require.NoError(t, os.MkdirAll(child, 0o755))
	initTestGitRepo(t, child)
	writeTestFile(t, child, "go.mod", "module example.com/child\n\ngo 1.22\n")
	writeTestFile(t, child, "main.go", "package main\n")

	payload, warnings, err := buildMapPayload(dir)
	require.NoError(t, err)
	assert.True(t, payload.MultiRepo)
	require.Len(t, payload.Repositories, 2)

	byPath := map[string]mapRepoPayload{}
	for _, repo := range payload.Repositories {
		byPath[repo.Path] = repo
	}
	assert.Contains(t, byPath["."].TrackedIgnored, ".mcp.json")
	assert.True(t, byPath["child"].Dirty)
	assert.NotEmpty(t, warnings)
}

func TestMapCmd_Text(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	initTestGitRepo(t, dir)
	cmd := newMapCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := cmd.RunE(cmd, []string{dir})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Workspace Map")
}
