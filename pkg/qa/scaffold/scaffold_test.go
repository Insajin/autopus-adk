package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitResolvesWorkspaceTargetIntoDesktopRepo(t *testing.T) {
	root := t.TempDir()
	desktopDir := filepath.Join(root, "autopus-desktop")
	adkDir := filepath.Join(root, "autopus-adk")
	mkdirAll(t, filepath.Join(root, ".git"))
	mkdirAll(t, filepath.Join(desktopDir, ".git"))
	mkdirAll(t, filepath.Join(desktopDir, "src-tauri"))
	mkdirAll(t, filepath.Join(adkDir, ".git"))
	writeFile(t, filepath.Join(adkDir, "go.mod"), "module example.com/adk\n\ngo 1.26\n")
	writeFile(t, filepath.Join(desktopDir, "package.json"), `{
  "scripts": {
    "test": "vitest run",
    "build": "vite build"
  },
  "devDependencies": {
    "@playwright/test": "^1.0.0"
  }
}`)
	writeFile(t, filepath.Join(desktopDir, "src-tauri", "tauri.conf.json"), "{}\n")

	result, err := Init(Options{ProjectDir: root, Release: true, Workflow: workflowGitHubActions})
	require.NoError(t, err)

	assert.Equal(t, "created", result.Status)
	assert.Equal(t, realPath(t, desktopDir), result.ProjectDir)
	assert.Equal(t, realPath(t, root), result.RequestedProjectDir)
	assert.Equal(t, realPath(t, root), result.WorkspaceRoot)
	assert.Contains(t, result.TargetReason, "autopus-desktop")
	assertCreatedID(t, result, "desktop-gui-explore")
	assertCreatedID(t, result, "github-actions-release-gate")
	assert.FileExists(t, filepath.Join(desktopDir, ".autopus", "qa", "journeys", "desktop-gui-explore.yaml"))
	assert.NoFileExists(t, filepath.Join(root, ".autopus", "qa", "journeys", "canary-explicit.yaml"))
	assertNextStepContains(t, result.NextSteps, "auto qa plan --project-dir autopus-desktop --format json")
}

func TestInitNoopsWhenWorkspaceTargetIsAmbiguous(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"api-a", "api-b"} {
		dir := filepath.Join(root, name)
		mkdirAll(t, filepath.Join(dir, ".git"))
		writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/"+name+"\n\ngo 1.26\n")
	}

	result, err := Init(Options{ProjectDir: root})
	require.NoError(t, err)

	assert.Equal(t, "noop", result.Status)
	assert.Equal(t, realPath(t, root), result.ProjectDir)
	assert.Equal(t, realPath(t, root), result.WorkspaceRoot)
	assert.Empty(t, result.Created)
	assert.Contains(t, strings.Join(result.Warnings, "\n"), "multiple QA target repositories match equally")
	assert.NoFileExists(t, filepath.Join(root, ".autopus", "qa", "journeys", "go-fast.yaml"))
}

func TestInitHonorsExplicitProjectDir(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "product")
	mkdirAll(t, filepath.Join(child, ".git"))
	writeFile(t, filepath.Join(child, "go.mod"), "module example.com/product\n\ngo 1.26\n")

	result, err := Init(Options{ProjectDir: root, ProjectDirExplicit: true})
	require.NoError(t, err)

	assert.Equal(t, "noop", result.Status)
	assert.Equal(t, realPath(t, root), result.ProjectDir)
	assert.Empty(t, result.RequestedProjectDir)
	assert.Empty(t, result.Created)
	assert.Contains(t, strings.Join(result.Warnings, "\n"), "no supported QA signals detected")
	assert.NoFileExists(t, filepath.Join(child, ".autopus", "qa", "journeys", "go-fast.yaml"))
}

func TestInitCreatesNodePlaywrightAndReleaseWorkflow(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{
  "scripts": {
    "test": "vitest run",
    "build": "vite build"
  },
  "devDependencies": {
    "@playwright/test": "^1.0.0",
    "vitest": "^1.0.0"
  }
}`)
	writeFile(t, filepath.Join(dir, "package-lock.json"), "{}\n")
	writeFile(t, filepath.Join(dir, "playwright.config.ts"), "export default {}\n")

	result, err := Init(Options{ProjectDir: dir, ProjectDirExplicit: true, Release: true, Workflow: workflowGitHubActions})
	require.NoError(t, err)

	assert.Equal(t, "created", result.Status)
	assertCreatedID(t, result, "node-fast")
	assertCreatedID(t, result, "browser-staging-playwright")
	assertCreatedID(t, result, "canary-explicit")
	assertCreatedID(t, result, "github-actions-release-gate")
	assert.FileExists(t, filepath.Join(dir, ".github", "workflows", "autopus-qa-release.yml"))
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(path, 0o755))
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}

func assertCreatedID(t *testing.T, result Result, id string) {
	t.Helper()
	for _, created := range result.Created {
		if created.ID == id {
			return
		}
	}
	t.Fatalf("created files did not include %q: %#v", id, result.Created)
}

func assertNextStepContains(t *testing.T, steps []string, expected string) {
	t.Helper()
	for _, step := range steps {
		if strings.Contains(step, expected) {
			return
		}
	}
	t.Fatalf("next steps did not include %q: %#v", expected, steps)
}

func realPath(t *testing.T, path string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(path)
	require.NoError(t, err)
	return real
}
