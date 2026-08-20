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
// command, or extension from omp's own discovery output, so only directory-form
// patterns are safe here. .omp/config.yml is deliberately absent: omp reads it
// directly as settings rather than through a discovery glob (REQ-011).
//
// Measured on omp 17.3.5: a .gitignore holding `.omp/rules/autopus-*.md` drops
// all 14 relocated rules from the session (domain rules 9 -> 0, TTSR 3 -> 0),
// while `.omp/rules/` keeps all 14. The rule surface therefore has to be
// ignored by directory, never by file-name glob. The extension surface behaves
// the same way: `.omp/extensions/autopus-*.ts` stops autopus-pipeline.ts from
// registering its command while `.omp/extensions/` leaves it registered.
var ompDiscoveryRoots = []string{
	".agents/rules/",
	".agents/skills/",
	".agents/commands/",
	".omp/agents/",
	".omp/rules/",
	".omp/extensions/",
}

// TestGitignorePatterns_OMPUsesDirectoryFormsOnly covers REQ-011/S8.
func TestGitignorePatterns_OMPUsesDirectoryFormsOnly(t *testing.T) {
	t.Parallel()

	patterns := map[string]bool{}
	for _, pattern := range gitignorePatterns {
		patterns[pattern] = true
	}

	// The rule surface is ignored by directory, not by file-name glob: the glob
	// form suppresses omp's own rule discovery (measured, omp 17.3.5), and
	// omitting the pattern entirely leaves generated files unignored, which the
	// doctor hygiene check reports.
	for _, required := range []string{
		".omp/rules/", ".omp/agents/", ".omp/config.yml", ".omp/extensions/",
	} {
		assert.True(t, patterns[required], "missing ADK-authored omp gitignore pattern %q", required)
	}

	// `/.omp/` would swallow .omp/RULES.md, the user-owned native sticky rule
	// file that omp Clean is required to preserve. A file-name glob scoped to
	// the ADK prefix is forbidden for the discovery reason above.
	for _, forbidden := range []string{
		"/.omp/", ".omp/", ".omp/RULES.md",
		".omp/rules/autopus-*.md", ".omp/extensions/autopus-*.ts",
	} {
		assert.False(t, patterns[forbidden],
			"gitignore pattern %q is not a legal omp pattern", forbidden)
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
		".omp/rules/autopus-branding.md",
		".omp/agents/executor.md",
		".omp/config.yml",
	}
	// Collateral of the directory-form rule pattern: omp scans .omp/rules/
	// non-recursively, so ADK rules share the directory with the user's own
	// files and `.omp/rules/` ignores both. This is the accepted trade — the
	// file-name glob that would spare mine.md also hides the ADK rules from omp
	// itself. Negation cannot narrow it back: git never re-includes a file whose
	// parent directory is excluded (verified), so the escape hatch is `git add
	// -f`, asserted below, or keeping personal rules outside .omp/rules/.
	ignoredUserRules := []string{".omp/rules/mine.md"}
	userOwned := []string{
		".omp/RULES.md",
		".agents/rules/my-own-rule.md",
	}
	mustBeIgnored := append(append([]string{}, generated...), ignoredUserRules...)
	for _, rel := range append(append([]string{}, mustBeIgnored...), userOwned...) {
		writeOMPBoundaryFile(t, dir, rel)
	}

	for _, rel := range mustBeIgnored {
		cmd := exec.Command("git", "-C", dir, "check-ignore", "--no-index", "--quiet", rel)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("expected %s to be ignored by the directory-form rule pattern: %v\n%s", rel, err, out)
		}
	}
	for _, rel := range userOwned {
		cmd := exec.Command("git", "-C", dir, "check-ignore", "--no-index", "--quiet", rel)
		if err := cmd.Run(); err == nil {
			t.Fatalf("user-owned omp path %s must not be ignored", rel)
		}
	}

	// The documented escape hatch has to keep working, otherwise a user with
	// tracked rules under .omp/rules/ has no recourse at all.
	forced := ".omp/rules/mine.md"
	if out, err := exec.Command("git", "-C", dir, "add", "-f", forced).CombinedOutput(); err != nil {
		t.Fatalf("git add -f %s failed: %v\n%s", forced, err, out)
	}
	tracked, err := exec.Command("git", "-C", dir, "ls-files", "--cached").Output()
	require.NoError(t, err)
	assert.Contains(t, strings.Split(strings.TrimSpace(string(tracked)), "\n"), forced,
		"git add -f must still track a user rule inside the ignored directory")
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
