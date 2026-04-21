package setup

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender_MultiRepoSections(t *testing.T) {
	t.Parallel()

	info := &ProjectInfo{
		Name: "workspace",
		MultiRepo: &MultiRepoInfo{
			IsMultiRepo:   true,
			WorkspaceRoot: "/tmp/workspace",
			Components: []RepoComponent{
				{
					Name:            "autopus-co",
					Path:            ".",
					RemoteURL:       "git@example.com/autopus-co.git",
					PrimaryLanguage: "Markdown",
					Role:            "meta workspace",
				},
				{
					Name:            "autopus-adk",
					Path:            "autopus-adk",
					RemoteURL:       "git@example.com/autopus-adk.git",
					PrimaryLanguage: "Go",
					ModulePath:      "github.com/example/autopus-adk",
					Role:            "CLI and harness source",
				},
				{
					Name:            "autopus-desktop",
					Path:            "autopus-desktop",
					RemoteURL:       "git@example.com/autopus-desktop.git",
					PrimaryLanguage: "TypeScript",
					Role:            "desktop shell",
				},
			},
			Dependencies: []RepoDependency{
				{Source: "autopus-desktop", Target: "autopus-adk", Type: "file"},
			},
		},
	}

	ds := Render(info, nil)

	assert.Contains(t, ds.Index, "## Repositories")
	assert.Contains(t, ds.Index, "autopus-adk")
	assert.Contains(t, ds.Architecture, "## Workspace")
	assert.Contains(t, ds.Architecture, "multi-repo")
	assert.Contains(t, ds.Architecture, "git@example.com/autopus-adk.git")
	assert.Contains(t, ds.Architecture, "autopus-desktop -> autopus-adk")
	assert.Contains(t, ds.Architecture, "## Development Workflow")
	assert.Contains(t, ds.Structure, "## Repository Boundaries")
	assert.Contains(t, ds.Structure, "[git repo] autopus-adk/")
	assert.Contains(t, ds.Structure, "git@example.com/autopus-desktop.git")
}

func TestRender_MultiRepoDocsStayUnderLineLimit(t *testing.T) {
	t.Parallel()

	components := make([]RepoComponent, 0, 8)
	deps := make([]RepoDependency, 0, 7)
	for i, name := range []string{"root", "api", "web", "desktop", "protocol", "tap", "docs", "worker"} {
		path := name
		if i == 0 {
			path = "."
		}
		components = append(components, RepoComponent{
			Name:            name,
			Path:            path,
			RemoteURL:       "git@example.com/" + name + ".git",
			PrimaryLanguage: "Go",
			Role:            "service",
		})
		if i > 0 {
			deps = append(deps, RepoDependency{
				Source: name,
				Target: "root",
				Type:   "require",
			})
		}
	}

	info := &ProjectInfo{
		Name: "workspace",
		MultiRepo: &MultiRepoInfo{
			IsMultiRepo:   true,
			WorkspaceRoot: "/tmp/workspace",
			Components:    components,
			Dependencies:  deps,
		},
	}

	ds := Render(info, nil)
	require.LessOrEqual(t, len(strings.Split(ds.Architecture, "\n")), maxDocLines)
	require.LessOrEqual(t, len(strings.Split(ds.Structure, "\n")), maxDocLines)
}
