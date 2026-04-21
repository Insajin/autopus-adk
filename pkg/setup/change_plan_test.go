package setup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildGeneratePlan_NoWriteAndClassification(t *testing.T) {
	t.Parallel()

	projectDir := setupGoProject(t)

	plan, err := BuildGeneratePlan(projectDir, nil)
	require.NoError(t, err)
	require.NotNil(t, plan)

	assert.NoDirExists(t, filepath.Join(projectDir, ".autopus", "docs"))
	assert.NoFileExists(t, filepath.Join(projectDir, ".autopus", "project", "scenarios.md"))
	assert.NotEmpty(t, plan.Fingerprint)

	classes := make(map[ChangeClass]bool)
	for _, change := range plan.Changes {
		classes[change.Class] = true
		assert.NotEmpty(t, change.Reason)
	}

	assert.True(t, classes[ChangeClassTrackedDocs], "tracked docs should be previewed")
	assert.True(t, classes[ChangeClassGeneratedSurface], "generated surface should be previewed")
	assert.True(t, classes[ChangeClassRuntimeState], "runtime metadata should be previewed")
}

func TestApplyChangePlan_RejectsStalePreview(t *testing.T) {
	t.Parallel()

	projectDir := setupGoProject(t)

	plan, err := BuildGeneratePlan(projectDir, nil)
	require.NoError(t, err)

	writeFile(t, projectDir, "Makefile", "build:\n\tgo build ./...\n")

	result, err := ApplyChangePlan(plan)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrStaleChangePlan)
	assert.True(t, errors.Is(err, ErrStaleChangePlan))
	assert.NoDirExists(t, filepath.Join(projectDir, ".autopus", "docs"))
}

func TestBuildGeneratePlan_MultiRepoWorkspaceHint(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	writeGitConfig(t, projectDir, "git@example.com/root.git")
	writeFile(t, projectDir, "go.mod", "module example.com/root\n\ngo 1.23\n")
	writeFile(t, projectDir, "main.go", "package main\n\nfunc main() {}\n")
	writeGitConfig(t, filepath.Join(projectDir, "bridge"), "git@example.com/bridge.git")
	writeFile(t, filepath.Join(projectDir, "bridge"), "go.mod", "module example.com/bridge\n\ngo 1.23\n")

	plan, err := BuildGeneratePlan(projectDir, nil)
	require.NoError(t, err)
	require.NotEmpty(t, plan.WorkspaceHints)

	hint := plan.WorkspaceHints[0]
	assert.Equal(t, WorkspaceHintKindMultiRepo, hint.Kind)
	assert.Contains(t, hint.Message, "multi-repo workspace detected")
	assert.Contains(t, hint.Message, "owning repo")

	info, statErr := os.Stat(hint.SourceOfTruth)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}
