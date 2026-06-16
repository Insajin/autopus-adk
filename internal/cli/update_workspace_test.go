package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestUpdateCmd_WorkspacePlanTargetsConfiguredReposAndSkipsMissingConfig(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	child := filepath.Join(root, "autopus-desktop")
	missing := filepath.Join(root, "scratch")

	makeWorkspaceRepo(t, root)
	makeWorkspaceRepo(t, child)
	makeWorkspaceRepo(t, missing)
	writeWorkspaceHarnessConfig(t, root, "meta-workspace")
	writeWorkspaceHarnessConfig(t, child, "autopus-desktop")

	var out bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update", "--dir", root, "--workspace", "--plan", "--yes"})
	require.NoError(t, cmd.Execute())

	output := out.String()
	assert.Contains(t, output, "Workspace update plan: 2 target(s), 1 skipped")
	assert.Contains(t, output, "[.]")
	assert.Contains(t, output, "[autopus-desktop]")
	assert.Contains(t, output, "scratch skipped: missing autopus.yaml")
	assert.NoFileExists(t, filepath.Join(child, ".codex", "config.toml"))
	assert.NoFileExists(t, filepath.Join(missing, ".codex", "config.toml"))
}

func TestUpdateCmd_SmartWorkspacePlanWithoutFlag(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	child := filepath.Join(root, "autopus-desktop")

	makeWorkspaceRepo(t, root)
	makeWorkspaceRepo(t, child)
	writeWorkspacePolicy(t, root)
	writeWorkspaceHarnessConfig(t, root, "meta-workspace")
	writeWorkspaceHarnessConfig(t, child, "autopus-desktop")

	var out bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update", "--dir", root, "--plan", "--yes"})
	require.NoError(t, cmd.Execute())

	output := out.String()
	assert.Contains(t, output, "Workspace update plan: 2 target(s)")
	assert.Contains(t, output, "[.]")
	assert.Contains(t, output, "[autopus-desktop]")
}

func TestUpdateCmd_LocalBypassesSmartWorkspaceDetection(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	child := filepath.Join(root, "autopus-desktop")

	makeWorkspaceRepo(t, root)
	makeWorkspaceRepo(t, child)
	writeWorkspacePolicy(t, root)
	writeWorkspaceHarnessConfig(t, root, "meta-workspace")
	writeWorkspaceHarnessConfig(t, child, "autopus-desktop")

	var out bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update", "--dir", root, "--local", "--plan", "--yes"})
	require.NoError(t, cmd.Execute())

	output := out.String()
	assert.Contains(t, output, "Preview: auto update")
	assert.NotContains(t, output, "Workspace update plan")
	assert.NotContains(t, output, "[autopus-desktop]")
}

func TestUpdateCmd_PositionalRepoTargetsWorkspace(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	child := filepath.Join(root, "autopus-desktop")
	sibling := filepath.Join(root, "autopus-adk")

	makeWorkspaceRepo(t, root)
	makeWorkspaceRepo(t, child)
	makeWorkspaceRepo(t, sibling)
	writeWorkspaceHarnessConfig(t, child, "autopus-desktop")
	writeWorkspaceHarnessConfig(t, sibling, "autopus-adk")

	var out bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update", "--dir", root, "autopus-desktop", "--plan", "--yes"})
	require.NoError(t, cmd.Execute())

	output := out.String()
	assert.Contains(t, output, "Workspace update plan: 1 target(s)")
	assert.Contains(t, output, "[autopus-desktop]")
	assert.NotContains(t, output, "[autopus-adk]")
}

func TestUpdateCmd_WorkspaceOnlyAppliesConfiguredChild(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	child := filepath.Join(root, "autopus-desktop")
	sibling := filepath.Join(root, "autopus-adk")

	makeWorkspaceRepo(t, root)
	makeWorkspaceRepo(t, child)
	makeWorkspaceRepo(t, sibling)
	writeWorkspaceHarnessConfig(t, child, "autopus-desktop")
	writeWorkspaceHarnessConfig(t, sibling, "autopus-adk")

	var out bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update", "--dir", root, "--workspace", "--only", "autopus-desktop", "--yes"})
	require.NoError(t, cmd.Execute())

	output := out.String()
	assert.Contains(t, output, "Workspace update: 1 target(s)")
	assert.Contains(t, output, "[autopus-desktop]")
	assert.NotContains(t, output, "[autopus-adk]")
	assert.FileExists(t, filepath.Join(child, ".codex", "config.toml"))
	assert.FileExists(t, filepath.Join(child, ".git", "hooks", "pre-commit"))
	assert.NoFileExists(t, filepath.Join(sibling, ".codex", "config.toml"))
}

func TestUpdateCmd_OnlyImpliesWorkspace(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	child := filepath.Join(root, "autopus-desktop")

	makeWorkspaceRepo(t, root)
	makeWorkspaceRepo(t, child)
	writeWorkspaceHarnessConfig(t, child, "autopus-desktop")

	var out bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update", "--dir", root, "--only", "autopus-desktop", "--plan", "--yes"})
	require.NoError(t, cmd.Execute())

	output := out.String()
	assert.Contains(t, output, "Workspace update plan: 1 target(s)")
	assert.Contains(t, output, "[autopus-desktop]")
}

func TestUpdateCmd_GlobalAutoAllowsSmartWorkspaceApply(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	child := filepath.Join(root, "autopus-desktop")

	makeWorkspaceRepo(t, root)
	makeWorkspaceRepo(t, child)
	writeWorkspacePolicy(t, root)
	writeWorkspaceHarnessConfig(t, child, "autopus-desktop")

	var out bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--auto", "update", "--dir", root})
	require.NoError(t, cmd.Execute())

	output := out.String()
	assert.Contains(t, output, "Workspace update: 1 target(s), 1 skipped")
	assert.Contains(t, output, ". skipped: missing autopus.yaml")
	assert.FileExists(t, filepath.Join(child, ".codex", "config.toml"))
}

func TestUpdateCmd_WorkspaceApplyPreflightsAllConfigsBeforeWriting(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	child := filepath.Join(root, "autopus-desktop")

	makeWorkspaceRepo(t, root)
	makeWorkspaceRepo(t, child)
	writeWorkspaceHarnessConfig(t, root, "meta-workspace")
	require.NoError(t, os.WriteFile(filepath.Join(child, "autopus.yaml"), []byte("mode:\n\tbroken"), 0644))

	var out bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update", "--dir", root, "--workspace", "--yes"})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "autopus-desktop 사전 검증 실패")
	assert.Contains(t, err.Error(), "설정 로드 실패")
	assert.NoFileExists(t, filepath.Join(root, ".codex", "config.toml"))
	assert.NoFileExists(t, filepath.Join(child, ".codex", "config.toml"))
}

func TestUpdateCmd_WorkspaceApplyPreflightsPlatformPreviewBeforeWriting(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	child := filepath.Join(root, "autopus-desktop")

	makeWorkspaceRepo(t, root)
	makeWorkspaceRepo(t, child)
	writeWorkspaceHarnessConfig(t, root, "meta-workspace")
	writeWorkspaceHarnessConfig(t, child, "autopus-desktop")
	require.NoError(t, os.MkdirAll(filepath.Join(child, ".autopus"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(child, ".autopus", "codex-manifest.json"), []byte("{broken"), 0644))

	var out bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update", "--dir", root, "--workspace", "--yes"})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "autopus-desktop 사전 검증 실패")
	assert.Contains(t, err.Error(), "매니페스트")
	assert.NoFileExists(t, filepath.Join(root, ".codex", "config.toml"))
	assert.NoFileExists(t, filepath.Join(child, ".codex", "config.toml"))
}

func TestUpdateCmd_WorkspaceApplyRequiresYes(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	makeWorkspaceRepo(t, root)
	writeWorkspaceHarnessConfig(t, root, "meta-workspace")

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"update", "--dir", root, "--workspace"})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--yes")
}

func TestUpdateCmd_LocalConflictsWithWorkspaceTarget(t *testing.T) {
	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"update", "--local", "autopus-desktop", "--plan"})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--local")
}

func makeWorkspaceRepo(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0755))
}

func writeWorkspaceHarnessConfig(t *testing.T, dir, projectName string) {
	t.Helper()
	cfg := config.DefaultFullConfig(projectName)
	cfg.Platforms = []string{"codex"}
	require.NoError(t, config.Save(dir, cfg))
}

func writeWorkspacePolicy(t *testing.T, dir string) {
	t.Helper()
	policyPath := filepath.Join(dir, ".autopus", "project", "workspace.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(policyPath), 0755))
	require.NoError(t, os.WriteFile(policyPath, []byte("# Workspace\n"), 0644))
}
