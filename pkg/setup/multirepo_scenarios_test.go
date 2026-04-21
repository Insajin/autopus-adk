package setup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateScenarios_AddsCrossRepoScenario(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	info := &ProjectInfo{
		Name: "workspace",
		MultiRepo: &MultiRepoInfo{
			IsMultiRepo: true,
			Components: []RepoComponent{
				{Name: "bridge", Path: "bridge", PrimaryLanguage: "Go"},
				{Name: "protocol", Path: "protocol", PrimaryLanguage: "Go"},
			},
			Dependencies: []RepoDependency{
				{Source: "bridge", Target: "protocol", Type: "replace"},
			},
		},
	}

	err := generateScenarios(dir, info)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".autopus", "project", "scenarios.md"))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "bridge-protocol")
	assert.Contains(t, content, "bridge integrates with protocol")
	assert.Contains(t, content, "(cd bridge && go test ./...)")
	assert.Contains(t, content, "(cd protocol && go test ./...)")
}

func TestGenerateScenarios_CrossRepoScenarioNumbersFollowCobraScenarios(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/testcli\n\ngo 1.21\n")
	writeFile(t, dir, "cmd/root.go", `package cmd

import "github.com/spf13/cobra"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run:   func(cmd *cobra.Command, args []string) {},
}
`)
	info := &ProjectInfo{
		Name: "workspace",
		MultiRepo: &MultiRepoInfo{
			IsMultiRepo: true,
			Dependencies: []RepoDependency{
				{Source: "bridge", Target: "protocol", Type: "replace"},
			},
		},
	}

	err := generateScenarios(dir, info)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".autopus", "project", "scenarios.md"))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "### S1: version")
	assert.Contains(t, content, "### S2: bridge-protocol")
}

func TestGenerateScenarios_UsesLanguageSpecificCommandsAndPaths(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	info := &ProjectInfo{
		Name: "workspace",
		MultiRepo: &MultiRepoInfo{
			IsMultiRepo: true,
			Components: []RepoComponent{
				{Name: "dashboard", Path: "apps/dashboard", PrimaryLanguage: "TypeScript"},
				{Name: "shared-ui", Path: "packages/shared-ui", PrimaryLanguage: "JavaScript"},
			},
			Dependencies: []RepoDependency{
				{Source: "dashboard", Target: "shared-ui", Type: "file"},
			},
		},
	}

	err := generateScenarios(dir, info)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".autopus", "project", "scenarios.md"))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "(cd apps/dashboard && npm test)")
	assert.Contains(t, content, "(cd packages/shared-ui && npm test)")
	assert.NotContains(t, content, "cd ..")
	assert.NotContains(t, content, "go test ./...")
}
