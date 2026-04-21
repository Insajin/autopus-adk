package setup

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectMultiRepo_NestedReposWithoutRootRepo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createGoRepo(t, dir, "api", "example.com/api", "git@example.com/api.git")
	createGoRepo(t, dir, "worker", "example.com/worker", "git@example.com/worker.git")
	createNodeRepo(t, dir, "web", "@example/web", "git@example.com/web.git")
	createNodeRepo(t, dir, "docs", "@example/docs", "git@example.com/docs.git")
	createRustRepo(t, dir, "desktop", "desktop-shell", "git@example.com/desktop.git")

	info := DetectMultiRepo(dir)
	require.NotNil(t, info)
	assert.True(t, info.IsMultiRepo)

	absDir, err := filepath.Abs(dir)
	require.NoError(t, err)
	assert.Equal(t, absDir, info.WorkspaceRoot)
	require.Len(t, info.Components, 5)

	byName := make(map[string]RepoComponent, len(info.Components))
	for _, component := range info.Components {
		byName[component.Name] = component
	}

	assert.Equal(t, "example.com/api", byName["api"].ModulePath)
	assert.Equal(t, "Go", byName["api"].PrimaryLanguage)
	assert.Equal(t, "git@example.com/api.git", byName["api"].RemoteURL)
	assert.Equal(t, "@example/web", byName["web"].PackageName)
	assert.Equal(t, "JavaScript", byName["web"].PrimaryLanguage)
	assert.Equal(t, "git@example.com/web.git", byName["web"].RemoteURL)
	assert.Equal(t, "desktop-shell", byName["desktop"].ModulePath)
	assert.Equal(t, "Rust", byName["desktop"].PrimaryLanguage)
}

func TestDetectMultiRepo_RootRepoWithNestedReposIncludesRootComponent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeGitConfig(t, dir, "git@example.com/root.git")
	writeFile(t, dir, "go.mod", "module example.com/root\n\ngo 1.23\n")
	createGoRepo(t, dir, "bridge", "example.com/bridge", "git@example.com/bridge.git")
	createNodeRepo(t, dir, "web", "@example/web", "git@example.com/web.git")

	info := DetectMultiRepo(dir)
	require.NotNil(t, info)
	require.Len(t, info.Components, 3)

	var hasRoot bool
	for _, component := range info.Components {
		if component.Path == "." {
			hasRoot = true
			assert.Equal(t, "example.com/root", component.ModulePath)
			assert.Equal(t, "git@example.com/root.git", component.RemoteURL)
		}
	}
	assert.True(t, hasRoot)
}

func TestDetectMultiRepo_ImmediateChildrenOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createGoRepo(t, filepath.Join(dir, "services"), "bridge", "example.com/bridge", "git@example.com/bridge.git")
	createNodeRepo(t, filepath.Join(dir, "apps"), "web", "@example/web", "git@example.com/web.git")

	info := DetectMultiRepo(dir)
	assert.Nil(t, info)
}

func TestScan_SingleRepoDoesNotSetMultiRepo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeGitConfig(t, dir, "git@example.com/single.git")
	writeFile(t, dir, "go.mod", "module example.com/single\n\ngo 1.23\n")
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")

	info, err := Scan(dir)
	require.NoError(t, err)
	assert.Nil(t, info.MultiRepo)
}

func TestScan_EmptyWorkspaceDoesNotSetMultiRepo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	info, err := Scan(dir)
	require.NoError(t, err)
	assert.Nil(t, info.MultiRepo)
}

func TestScan_MultiRepoAggregatesComponentSignals(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createGoRepo(t, dir, "api", "example.com/api", "git@example.com/api.git")
	writeFile(t, filepath.Join(dir, "api"), "cmd/server/main.go", "package main\n\nfunc main() {}\n")
	createNodeRepo(t, dir, "web", "@example/web", "git@example.com/web.git")
	writeFile(t, filepath.Join(dir, "web"), "src/index.js", "console.log('web')\n")

	info, err := Scan(dir)
	require.NoError(t, err)
	require.NotNil(t, info.MultiRepo)

	assert.Contains(t, languageNames(info.Languages), "Go")
	assert.Contains(t, languageNames(info.Languages), "JavaScript")
	assert.Contains(t, buildFilePaths(info.BuildFiles), "api/go.mod")
	assert.Contains(t, buildFilePaths(info.BuildFiles), "web/package.json")
	assert.Contains(t, entryPointPaths(info.EntryPoints), "api/cmd/server/main.go")
	assert.Contains(t, entryPointPaths(info.EntryPoints), "web/src/index.js")
}

func TestMapCrossRepoDeps_GoAndNPMReferences(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createGoRepo(t, dir, "protocol", "example.com/protocol", "git@example.com/protocol.git")
	createGoRepo(t, dir, "bridge", "example.com/bridge", "git@example.com/bridge.git")
	writeFile(t, filepath.Join(dir, "bridge"), "go.mod", `module example.com/bridge

go 1.23

require example.com/protocol v0.0.0

replace example.com/protocol => ../protocol
`)
	createNodeRepo(t, dir, "shared-ui", "@example/shared-ui", "git@example.com/shared-ui.git")
	createNodeRepo(t, dir, "dashboard", "@example/dashboard", "git@example.com/dashboard.git")
	writeFile(t, filepath.Join(dir, "dashboard"), "package.json", `{
  "name": "@example/dashboard",
  "dependencies": {
    "@example/shared-ui": "^1.0.0",
    "@example/local-shared": "file:../shared-ui"
  }
}`)

	info := DetectMultiRepo(dir)
	require.NotNil(t, info)
	info.Dependencies = MapCrossRepoDeps(info.Components)

	assert.Contains(t, dependencyKeys(info.Dependencies), "bridge->protocol:replace")
	assert.Contains(t, dependencyKeys(info.Dependencies), "bridge->protocol:require")
	assert.Contains(t, dependencyKeys(info.Dependencies), "dashboard->shared-ui:package")
	assert.Contains(t, dependencyKeys(info.Dependencies), "dashboard->shared-ui:file")
}

func createGoRepo(t *testing.T, root, name, modulePath, remote string) {
	t.Helper()
	repoDir := filepath.Join(root, name)
	writeGitConfig(t, repoDir, remote)
	writeFile(t, repoDir, "go.mod", "module "+modulePath+"\n\ngo 1.23\n")
}

func createNodeRepo(t *testing.T, root, name, packageName, remote string) {
	t.Helper()
	repoDir := filepath.Join(root, name)
	writeGitConfig(t, repoDir, remote)
	writeFile(t, repoDir, "package.json", "{\n  \"name\": \""+packageName+"\",\n  \"scripts\": {\n    \"test\": \"npm test\"\n  }\n}\n")
}

func createRustRepo(t *testing.T, root, name, packageName, remote string) {
	t.Helper()
	repoDir := filepath.Join(root, name)
	writeGitConfig(t, repoDir, remote)
	writeFile(t, repoDir, "Cargo.toml", "[package]\nname = \""+packageName+"\"\nversion = \"0.1.0\"\n")
}

func writeGitConfig(t *testing.T, dir, remote string) {
	t.Helper()
	writeFile(t, dir, ".git/config", "[remote \"origin\"]\n\turl = "+remote+"\n")
}

func languageNames(languages []Language) []string {
	names := make([]string, 0, len(languages))
	for _, language := range languages {
		names = append(names, language.Name)
	}
	return names
}

func buildFilePaths(buildFiles []BuildFile) []string {
	paths := make([]string, 0, len(buildFiles))
	for _, buildFile := range buildFiles {
		paths = append(paths, buildFile.Path)
	}
	return paths
}

func entryPointPaths(entryPoints []EntryPoint) []string {
	paths := make([]string, 0, len(entryPoints))
	for _, entryPoint := range entryPoints {
		paths = append(paths, entryPoint.Path)
	}
	return paths
}

func dependencyKeys(deps []RepoDependency) []string {
	keys := make([]string, 0, len(deps))
	for _, dep := range deps {
		keys = append(keys, dep.Source+"->"+dep.Target+":"+dep.Type)
	}
	return keys
}
