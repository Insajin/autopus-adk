package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateCmd_WorkspaceApplyRollsBackCommittedTargetWhenLaterWriteFails(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	child := filepath.Join(root, "autopus-desktop")

	makeWorkspaceRepo(t, root)
	makeWorkspaceRepo(t, child)
	writeWorkspaceHarnessConfig(t, root, "meta-workspace")
	writeWorkspaceHarnessConfig(t, child, "autopus-desktop")
	require.NoError(t, os.MkdirAll(filepath.Join(child, ".codex", "config.toml"), 0755))

	var out bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update", "--dir", root, "--workspace", "--yes"})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "autopus-desktop 업데이트 실패")
	assert.NoFileExists(t, filepath.Join(root, "AGENTS.md"))
	assert.NoFileExists(t, filepath.Join(root, ".codex", "config.toml"))
	assert.DirExists(t, filepath.Join(child, ".codex", "config.toml"))
}
