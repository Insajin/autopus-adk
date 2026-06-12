package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExistingPackWarningFormatsRelativePath asserts the warning message uses a
// slash-separated path relative to the project dir.
func TestExistingPackWarningFormatsRelativePath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	packPath := filepath.Join(dir, ".autopus", "qa", "journeys", "go-fast.yaml")
	warn := existingPackWarning(dir, packPath, os.ErrNotExist)

	assert.Contains(t, warn, "existing Journey Pack ignored")
	assert.Contains(t, warn, ".autopus/qa/journeys/go-fast.yaml")
}

// TestLoadJourneyCoverageWarnsOnInvalidYAML asserts a malformed journey YAML file
// is skipped with a warning rather than crashing.
func TestLoadJourneyCoverageWarnsOnInvalidYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(journeyDir, "broken.yaml"),
		[]byte("{: not yaml"),
		0o644,
	))

	cov, warnings := loadJourneyCoverage(dir)
	assert.Empty(t, cov.lanes)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "existing Journey Pack ignored")
}

// TestFastStarterCoversAllStacks asserts fastStarter returns a starter for each
// supported stack and returns false for an unknown stack.
func TestFastStarterCoversAllStacks(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		signals   projectSignals
		wantID    string
		wantFound bool
	}{
		{
			name:      "go",
			signals:   projectSignals{Stack: "go"},
			wantID:    "go-fast",
			wantFound: true,
		},
		{
			name:      "python",
			signals:   projectSignals{Stack: "python"},
			wantID:    "python-fast",
			wantFound: true,
		},
		{
			name:      "rust",
			signals:   projectSignals{Stack: "rust"},
			wantID:    "rust-fast",
			wantFound: true,
		},
		{
			name:      "unknown stack",
			signals:   projectSignals{Stack: "unknown"},
			wantFound: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			starter, ok := fastStarter(tc.signals)
			assert.Equal(t, tc.wantFound, ok)
			if tc.wantFound {
				assert.Equal(t, tc.wantID, starter.ID)
				assert.NotEmpty(t, starter.Body)
			}
		})
	}
}

// TestNodeFastStarterPicksJestWhenNoTestScript asserts nodeFastStarter falls through
// to jest when there is no "test" script but jest is a dependency.
func TestNodeFastStarterPicksJestWhenNoTestScript(t *testing.T) {
	t.Parallel()

	signals := projectSignals{
		Stack:          "node",
		PackageManager: "npm",
		Package: packageManifest{
			Scripts:         map[string]string{},
			Dependencies:    map[string]string{},
			DevDependencies: map[string]string{"jest": "^29.0.0"},
		},
	}
	starter, ok := nodeFastStarter(signals)
	require.True(t, ok)
	assert.Equal(t, "jest-fast", starter.ID)
}

// TestNodeFastStarterPicksVitestSignal asserts vitest dependency picks the vitest
// starter when there is no "test" script.
func TestNodeFastStarterPicksVitestSignal(t *testing.T) {
	t.Parallel()

	signals := projectSignals{
		Stack:          "node",
		PackageManager: "npm",
		Package: packageManifest{
			Scripts:         map[string]string{},
			Dependencies:    map[string]string{},
			DevDependencies: map[string]string{"vitest": "^2.0.0"},
		},
	}
	starter, ok := nodeFastStarter(signals)
	require.True(t, ok)
	assert.Equal(t, "vitest-fast", starter.ID)
}

// TestNodeFastStarterFallsBackToBuildScript asserts a build-only project without
// test/vitest/jest gets the node-build-fast starter.
func TestNodeFastStarterFallsBackToBuildScript(t *testing.T) {
	t.Parallel()

	signals := projectSignals{
		Stack:          "node",
		PackageManager: "npm",
		Package: packageManifest{
			Scripts:         map[string]string{"build": "vite build"},
			Dependencies:    map[string]string{},
			DevDependencies: map[string]string{},
		},
	}
	starter, ok := nodeFastStarter(signals)
	require.True(t, ok)
	assert.Equal(t, "node-build-fast", starter.ID)
}

// TestNodeFastStarterReturnsFalseWithNoSignals asserts an empty Node project
// with no scripts, vitest, or jest returns (_, false).
func TestNodeFastStarterReturnsFalseWithNoSignals(t *testing.T) {
	t.Parallel()

	signals := projectSignals{
		Stack:          "node",
		PackageManager: "npm",
		Package: packageManifest{
			Scripts:         map[string]string{},
			Dependencies:    map[string]string{},
			DevDependencies: map[string]string{},
		},
	}
	_, ok := nodeFastStarter(signals)
	assert.False(t, ok)
}

// TestInitCreatesPythonFastStarterAndWorkflow asserts a Python project detected by
// pytest.ini gets a python-fast starter and the workflow's Python setup step.
func TestInitCreatesPythonFastStarterAndWorkflow(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pytest.ini"), "[pytest]\n")
	writeFile(t, filepath.Join(dir, "requirements.txt"), "pytest\n")

	result, err := Init(Options{ProjectDir: dir, ProjectDirExplicit: true, Release: true, Workflow: workflowGitHubActions})
	require.NoError(t, err)

	assertCreatedID(t, result, "python-fast")
	wfPath := filepath.Join(dir, ".github", "workflows", "autopus-qa-release.yml")
	require.FileExists(t, wfPath)
	body, err := os.ReadFile(wfPath)
	require.NoError(t, err)
	assert.Contains(t, string(body), "Setup Python")
}

// TestInitCreatesRustFastStarterAndWorkflow asserts a top-level Cargo.toml project
// gets a rust-fast starter and the Rust workflow setup step.
func TestInitCreatesRustFastStarterAndWorkflow(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Cargo.toml"), "[package]\nname = \"mylib\"\n")

	result, err := Init(Options{ProjectDir: dir, ProjectDirExplicit: true, Release: true, Workflow: workflowGitHubActions})
	require.NoError(t, err)

	assertCreatedID(t, result, "rust-fast")
	wfPath := filepath.Join(dir, ".github", "workflows", "autopus-qa-release.yml")
	require.FileExists(t, wfPath)
	body, err := os.ReadFile(wfPath)
	require.NoError(t, err)
	assert.Contains(t, string(body), "Setup Rust")
}

// TestDetectWorkspaceQATargetsNoMatchingChildReturnsSkipScaffold exercises the
// resolveProjectDir path where child repos exist but none scores > 0.
func TestDetectWorkspaceQATargetsNoMatchingChildReturnsSkipScaffold(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// A child repo with no QA signals (no go.mod, no package.json).
	child := filepath.Join(root, "empty-repo")
	mkdirAll(t, filepath.Join(child, ".git"))

	result, err := Init(Options{ProjectDir: root})
	require.NoError(t, err)

	assert.Equal(t, "noop", result.Status)
	assert.True(t, strings.Contains(strings.Join(result.Warnings, "\n"), "no supported QA signals") ||
		strings.Contains(strings.Join(result.Warnings, "\n"), "no child repository"))
}
