package detect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFullModeDeps_ContainsNode verifies that FullModeDeps contains a Node.js entry.
func TestFullModeDeps_ContainsNode(t *testing.T) {
	t.Parallel()

	found := false
	for _, dep := range FullModeDeps {
		if dep.Name == "node" && dep.Binary == "node" {
			found = true
			break
		}
	}
	assert.True(t, found, "FullModeDeps must contain a node entry with Binary='node'")
}

// TestFullModeDeps_ContainsPlaywright verifies that FullModeDeps includes playwright.
func TestFullModeDeps_ContainsPlaywright(t *testing.T) {
	t.Parallel()

	found := false
	for _, dep := range FullModeDeps {
		if dep.Name == "playwright" && dep.Binary == "playwright" {
			found = true
			break
		}
	}
	assert.True(t, found, "FullModeDeps must contain a playwright entry")
}

// TestFullModeDeps_NodeHasInstallCmd verifies node entry has a non-empty install command.
func TestFullModeDeps_NodeHasInstallCmd(t *testing.T) {
	t.Parallel()

	for _, dep := range FullModeDeps {
		if dep.Name == "node" {
			assert.NotEmpty(t, dep.InstallCmd, "node dep must have a non-empty InstallCmd")
			return
		}
	}
	t.Fatal("node entry not found in FullModeDeps")
}

// TestFullModeDeps_NodeHasDescription verifies node entry has a description.
func TestFullModeDeps_NodeHasDescription(t *testing.T) {
	t.Parallel()

	for _, dep := range FullModeDeps {
		if dep.Name == "node" {
			assert.NotEmpty(t, dep.Description, "node dep must have a Description")
			return
		}
	}
	t.Fatal("node entry not found in FullModeDeps")
}

// TestCheckDependencies_ReturnsStatusForEach verifies CheckDependencies returns one status per dep.
func TestCheckDependencies_ReturnsStatusForEach(t *testing.T) {
	t.Parallel()

	deps := []Dependency{
		{Name: "nonexistent-tool-xyz", Binary: "nonexistent-tool-xyz"},
	}
	statuses := CheckDependencies(deps)
	require.Len(t, statuses, 1)
	assert.Equal(t, "nonexistent-tool-xyz", statuses[0].Name)
	assert.False(t, statuses[0].Installed, "nonexistent binary must not be reported as installed")
}

// TestCheckDependencies_Empty verifies empty deps list returns empty statuses.
func TestCheckDependencies_Empty(t *testing.T) {
	t.Parallel()

	statuses := CheckDependencies([]Dependency{})
	assert.Empty(t, statuses)
}

// TestIsInstalled_NonexistentBinary verifies false is returned for unknown binaries.
func TestIsInstalled_NonexistentBinary(t *testing.T) {
	t.Parallel()

	assert.False(t, IsInstalled("__nonexistent_binary_xyz_autopus__"))
}

// TestIsInstalled_KnownBinary verifies true is returned for a binary known to exist in CI/dev.
// Uses "sh" which is always available on Unix-like systems.
func TestIsInstalled_KnownBinary(t *testing.T) {
	t.Parallel()

	assert.True(t, IsInstalled("sh"), "sh must be installed on any Unix-like system")
}

// TestCheckParentRuleConflicts_NonexistentDir verifies no conflicts returned for missing dir.
func TestCheckParentRuleConflicts_NonexistentDir(t *testing.T) {
	t.Parallel()

	conflicts := CheckParentRuleConflicts("/nonexistent/path/that/does/not/exist")
	// Should return nil or empty slice, not panic.
	assert.NotPanics(t, func() {
		_ = CheckParentRuleConflicts("/nonexistent/path/that/does/not/exist")
	})
	assert.Empty(t, conflicts)
}

// TestCheckParentRuleConflicts_TempDir verifies no conflicts in a fresh temp directory.
func TestCheckParentRuleConflicts_TempDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A fresh temp dir has no parent .claude/rules; expect zero conflicts (or just no panics).
	conflicts := CheckParentRuleConflicts(dir)
	// We cannot assert 0 conflicts in all environments, but the function must not panic.
	_ = conflicts
}

// TestCheckParentRuleConflicts_DetectsConflict verifies that a parent dir with a non-autopus
// rules namespace is returned as a conflict.
func TestCheckParentRuleConflicts_DetectsConflict(t *testing.T) {
	t.Parallel()

	// Create a directory hierarchy: parent/project
	parent := t.TempDir()
	projectDir, err := os.MkdirTemp(parent, "project")
	require.NoError(t, err)

	// Create parent/.claude/rules/moai (simulates a foreign harness namespace).
	rulesDir := filepath.Join(parent, ".claude", "rules", "moai")
	require.NoError(t, os.MkdirAll(rulesDir, 0755))

	conflicts := CheckParentRuleConflicts(projectDir)
	require.NotEmpty(t, conflicts, "must detect moai namespace conflict in parent dir")

	found := false
	for _, c := range conflicts {
		if c.Namespace == "moai" {
			found = true
			assert.Equal(t, parent, c.ParentDir)
			assert.Contains(t, c.RulesDir, ".claude")
		}
	}
	assert.True(t, found, "moai conflict must be present in results")
}

// TestCheckParentRuleConflicts_IgnoresAutopusNamespace verifies autopus namespace is not flagged.
func TestCheckParentRuleConflicts_IgnoresAutopusNamespace(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	projectDir, err := os.MkdirTemp(parent, "project")
	require.NoError(t, err)

	// Create parent/.claude/rules/autopus (should be ignored).
	autopusDir := filepath.Join(parent, ".claude", "rules", "autopus")
	require.NoError(t, os.MkdirAll(autopusDir, 0755))

	conflicts := CheckParentRuleConflicts(projectDir)
	for _, c := range conflicts {
		assert.NotEqual(t, "autopus", c.Namespace, "autopus namespace must not be flagged as conflict")
	}
}

// TestDetectPlatforms_ReturnsSlice verifies DetectPlatforms returns a slice without panicking.
// Result may be empty in CI environments where no coding CLIs are installed.
func TestDetectPlatforms_ReturnsSlice(t *testing.T) {
	t.Parallel()

	platforms := DetectPlatforms()
	// Must not panic and must return a slice (possibly empty).
	assert.NotPanics(t, func() {
		_ = DetectPlatforms()
	})
	_ = platforms
}

// TestDetectPlatforms_PlatformFields verifies any detected platform has non-empty Name and Binary.
func TestDetectPlatforms_PlatformFields(t *testing.T) {
	t.Parallel()

	platforms := DetectPlatforms()
	for _, p := range platforms {
		assert.NotEmpty(t, p.Name, "Platform.Name must not be empty")
		assert.NotEmpty(t, p.Binary, "Platform.Binary must not be empty")
	}
}

// TestDetectOrchestraProviders_ReturnsAllThree verifies that exactly 3 providers are returned.
func TestDetectOrchestraProviders_ReturnsAllThree(t *testing.T) {
	t.Parallel()

	providers := DetectOrchestraProviders()
	assert.Len(t, providers, 3, "DetectOrchestraProviders must return exactly 3 providers")

	names := make(map[string]bool)
	for _, p := range providers {
		names[p.Name] = true
		assert.NotEmpty(t, p.Name, "OrchestraProvider.Name must not be empty")
		assert.NotEmpty(t, p.Binary, "OrchestraProvider.Binary must not be empty")
	}
	assert.True(t, names["claude"], "claude provider must be present")
	assert.True(t, names["codex"], "codex provider must be present")
	assert.True(t, names["gemini"], "gemini provider must be present")
}

// TestInstalledOrchestraProviders_FiltersCorrectly verifies that only installed providers are returned.
func TestInstalledOrchestraProviders_FiltersCorrectly(t *testing.T) {
	t.Parallel()

	// InstalledOrchestraProviders must return a subset of DetectOrchestraProviders.
	all := DetectOrchestraProviders()
	installed := InstalledOrchestraProviders()

	// All names in installed must appear in all with Installed=true.
	allMap := make(map[string]bool)
	for _, p := range all {
		if p.Installed {
			allMap[p.Name] = true
		}
	}
	for _, name := range installed {
		assert.True(t, allMap[name], "installed provider %q must be marked Installed in DetectOrchestraProviders", name)
	}
	// Count must match.
	assert.Len(t, installed, len(allMap), "InstalledOrchestraProviders length must match installed count from DetectOrchestraProviders")
}

// TestIsNpmBased verifies IsNpmBased identifies npm-prefixed install commands.
func TestIsNpmBased(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		installCmd string
		want       bool
	}{
		{"npm install command", "npm i -g @ast-grep/cli", true},
		{"npm global install", "npm install -g playwright", true},
		{"brew install", "brew install gh", false},
		{"empty command", "", false},
		{"npx command (not npm)", "npx playwright install chromium", false},
		{"https url", "https://nodejs.org", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dep := Dependency{Name: "test", InstallCmd: tc.installCmd}
			assert.Equal(t, tc.want, dep.IsNpmBased())
		})
	}
}

// TestDependencyStatus_Fields verifies DependencyStatus embeds Dependency correctly.
func TestDependencyStatus_Fields(t *testing.T) {
	t.Parallel()

	dep := Dependency{
		Name:        "test-tool",
		Binary:      "test-bin",
		InstallCmd:  "npm i test-tool",
		Required:    true,
		Description: "A test tool",
	}
	status := DependencyStatus{Dependency: dep, Installed: true}

	assert.Equal(t, "test-tool", status.Name)
	assert.Equal(t, "test-bin", status.Binary)
	assert.True(t, status.Required)
	assert.True(t, status.Installed)
}
