package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

// ompDiscoveryRoots are the directories omp scans with a gitignore-aware glob.
// A file-name-form pattern under one of these removes the matching rule, skill,
// or command from omp's own discovery output, so only directory-form patterns
// are safe here. .omp/config.yml is deliberately absent: omp reads it directly
// as settings rather than through a discovery glob (REQ-011).
var ompDiscoveryRoots = []string{
	".agents/rules/",
	".agents/skills/",
	".agents/commands/",
	".omp/agents/",
	".omp/rules/",
}

// TestGitignorePatterns_OMPUsesDirectoryFormsOnly covers REQ-011/S8.
func TestGitignorePatterns_OMPUsesDirectoryFormsOnly(t *testing.T) {
	t.Parallel()

	patterns := map[string]bool{}
	for _, pattern := range gitignorePatterns {
		patterns[pattern] = true
	}

	for _, required := range []string{".agents/rules/autopus/", ".omp/agents/", ".omp/config.yml"} {
		assert.True(t, patterns[required], "missing ADK-authored omp gitignore pattern %q", required)
	}

	// `/.omp/` would swallow the user-owned native surface (.omp/rules/,
	// .omp/RULES.md) that omp Clean is required to preserve.
	for _, forbidden := range []string{"/.omp/", ".omp/", ".omp/rules/", ".omp/RULES.md"} {
		assert.False(t, patterns[forbidden],
			"gitignore pattern %q would hide the user-owned omp surface", forbidden)
	}

	for _, pattern := range gitignorePatterns {
		for _, root := range ompDiscoveryRoots {
			if strings.HasPrefix(pattern, root) && !strings.HasSuffix(pattern, "/") {
				t.Fatalf("file-name-form pattern %q under discovery root %q would drop the file from omp discovery",
					pattern, root)
			}
		}
	}
}

// TestUpdateGitignore_OMPIgnoresGeneratedButNotUserSurface checks the same
// boundary through git itself, so a pattern that reads correctly but resolves
// differently cannot pass.
func TestUpdateGitignore_OMPIgnoresGeneratedButNotUserSurface(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, updateGitignore(dir))
	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	generated := []string{
		".agents/rules/autopus/branding.md",
		".omp/agents/executor.md",
		".omp/config.yml",
	}
	userOwned := []string{
		".omp/rules/my-rule.md",
		".omp/RULES.md",
		".agents/rules/my-own-rule.md",
	}
	for _, rel := range append(append([]string{}, generated...), userOwned...) {
		writeOMPBoundaryFile(t, dir, rel)
	}

	for _, rel := range generated {
		cmd := exec.Command("git", "-C", dir, "check-ignore", "--no-index", "--quiet", rel)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("expected ADK-authored omp path %s to be ignored: %v\n%s", rel, err, out)
		}
	}
	for _, rel := range userOwned {
		cmd := exec.Command("git", "-C", dir, "check-ignore", "--no-index", "--quiet", rel)
		if err := cmd.Run(); err == nil {
			t.Fatalf("user-owned omp path %s must not be ignored", rel)
		}
	}
}

// TestPlatformAddOMP_RegistersNoOrchestraProvider covers REQ-018/S14. omp has
// no orchestra provider mapping, and the unknown-provider fallback would have
// registered ProviderEntry{Binary: "omp"} — orchestra wiring is an explicit
// non-goal for this platform.
func TestPlatformAddOMP_RegistersNoOrchestraProvider(t *testing.T) {
	t.Parallel()

	assert.Empty(t, config.PlatformToProvider("omp"),
		"omp must have no orchestra provider mapping")

	dir := t.TempDir()
	initCmd := NewRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "omp-boundary", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	before, err := config.Load(dir)
	require.NoError(t, err)
	require.True(t, before.Orchestra.Enabled, "S14 requires orchestra.enabled to be true")
	require.NotContains(t, before.Orchestra.Providers, "omp")
	baseline := before.Orchestra.Providers

	addCmd := NewRootCmd()
	addCmd.SetArgs([]string{"platform", "add", "omp", "--dir", dir})
	require.NoError(t, addCmd.Execute())

	after, err := config.Load(dir)
	require.NoError(t, err)
	assert.Contains(t, after.Platforms, "omp", "omp surface generation must still succeed")
	assert.NotContains(t, after.Orchestra.Providers, "omp",
		"platform add must not register an orchestra provider for omp")
	assert.Equal(t, baseline, after.Orchestra.Providers,
		"existing orchestra provider entries must be unchanged in count and content")
}

func writeOMPBoundaryFile(t *testing.T, dir, rel string) {
	t.Helper()

	path := filepath.Join(dir, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("x\n"), 0o644))
}
