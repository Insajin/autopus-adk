package scaffold

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHasQAScaffoldSignalsDetectsGoModule asserts a Go module reports signals while
// an empty directory does not.
func TestHasQAScaffoldSignalsDetectsGoModule(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/x\n\ngo 1.26\n")
	assert.True(t, HasQAScaffoldSignals(dir))

	empty := t.TempDir()
	assert.False(t, HasQAScaffoldSignals(empty))
}

// TestDetectWorkspaceQATargetsRanksDesktopHighest asserts a multi-repo workspace
// returns scored targets with the desktop+playwright repo ranked above the plain
// Go repo, and that child repos are reported.
func TestDetectWorkspaceQATargetsRanksDesktopHighest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	desktop := filepath.Join(root, "autopus-desktop")
	mkdirAll(t, filepath.Join(desktop, ".git"))
	mkdirAll(t, filepath.Join(desktop, "src-tauri"))
	writeFile(t, filepath.Join(desktop, "src-tauri", "tauri.conf.json"), "{}\n")
	writeFile(t, filepath.Join(desktop, "package.json"), `{
  "scripts": {"test": "vitest run", "build": "vite build"},
  "devDependencies": {"@playwright/test": "^1.0.0"}
}`)

	api := filepath.Join(root, "api")
	mkdirAll(t, filepath.Join(api, ".git"))
	writeFile(t, filepath.Join(api, "go.mod"), "module example.com/api\n\ngo 1.26\n")

	targets, hasChildRepos, err := DetectWorkspaceQATargets(root)
	require.NoError(t, err)

	assert.True(t, hasChildRepos)
	require.GreaterOrEqual(t, len(targets), 2)
	// Desktop GUI signals score 90+ so it must rank first.
	assert.Equal(t, "autopus-desktop", targets[0].RelPath)
	assert.Greater(t, targets[0].Score, targets[1].Score)
	assert.Contains(t, strings.Join(targets[0].Reasons, ","), "desktop GUI signals")
}

// TestDetectWorkspaceQATargetsNoChildRepos asserts a directory without git child
// repos reports no child repos and no targets.
func TestDetectWorkspaceQATargetsNoChildRepos(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// A non-git child directory must not count as a repo.
	mkdirAll(t, filepath.Join(root, "plain", "src"))

	targets, hasChildRepos, err := DetectWorkspaceQATargets(root)
	require.NoError(t, err)

	assert.False(t, hasChildRepos)
	assert.Empty(t, targets)
}

// TestDetectWorkspaceQATargetsSkipsVendorAndDotDirs asserts ignored directory names
// are not treated as QA targets even with a .git marker.
func TestDetectWorkspaceQATargetsSkipsVendorAndDotDirs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, name := range []string{"node_modules", "vendor", "dist", ".hidden"} {
		dir := filepath.Join(root, name)
		mkdirAll(t, filepath.Join(dir, ".git"))
		writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/"+name+"\n\ngo 1.26\n")
	}

	targets, hasChildRepos, err := DetectWorkspaceQATargets(root)
	require.NoError(t, err)

	assert.False(t, hasChildRepos)
	assert.Empty(t, targets)
}

// TestInitCreatesPythonWorkflowSetupStep asserts the GitHub Actions workflow for a
// Python project includes the Python setup step.
func TestInitCreatesPythonWorkflowSetupStep(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), "[project]\nname = \"svc\"\n")
	writeFile(t, filepath.Join(dir, "pytest.ini"), "[pytest]\n")

	signals := detectSignals(dir)
	require.Equal(t, "python", signals.Stack)

	workflow := renderGitHubActionsWorkflow(signals)
	assert.Contains(t, workflow, "Setup Python")
	assert.Contains(t, workflow, "actions/setup-python@v5")
	assert.NotContains(t, workflow, "Setup Rust")
}

// TestRenderWorkflowIncludesRustSetupStep asserts a Rust (Cargo, non-Tauri) project
// emits the Rust toolchain setup step.
func TestRenderWorkflowIncludesRustSetupStep(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Cargo.toml"), "[package]\nname = \"svc\"\n")

	signals := detectSignals(dir)
	require.Equal(t, "rust", signals.Stack)

	workflow := renderGitHubActionsWorkflow(signals)
	assert.Contains(t, workflow, "Setup Rust")
	assert.Contains(t, workflow, "dtolnay/rust-toolchain@stable")
}

// TestRenderWorkflowNodeSetupStepHonorsPackageManager asserts pnpm/yarn lockfiles
// drive the correct install command in the node setup step.
func TestRenderWorkflowNodeSetupStepHonorsPackageManager(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"scripts":{"test":"vitest run"}}`)
	writeFile(t, filepath.Join(dir, "pnpm-lock.yaml"), "lockfileVersion: '9.0'\n")

	signals := detectSignals(dir)
	step := nodeSetupStep(signals)
	assert.Contains(t, step, "pnpm install --frozen-lockfile")
	assert.Contains(t, step, "cache: pnpm")
}
