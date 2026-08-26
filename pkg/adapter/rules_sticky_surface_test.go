package adapter_test

// SPEC-STICKYRULE-001 REQ-STICKYRULE-SCHEMA-01 / REQ-STICKYRULE-MAP-01.
// Scenario S7 asserts the sticky flag is orthogonal: it adds a second delivery
// moment without relocating or reshaping any rule, and it reaches exactly the
// two rules the mapping designates. The frontmatter propagation half of S8 lives
// in rules_sticky_frontmatter_test.go, which owns the shared helpers.

import (
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentfs "github.com/insajin/autopus-adk/content"
)

// stickyHookFiredRules is the SPEC-CONDRULE-001 hook-fired set that the S7
// baseline of 11 subtracts from the 14 content rules.
var stickyHookFiredRules = []string{"lore-commit.md", "shell-portability.md", "worktree-safety.md"}

func stickyContentRuleNames(t *testing.T) []string {
	t.Helper()
	entries, err := contentfs.FS.ReadDir("rules")
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, e.Name())
		}
	}
	return names
}

// stickyHookFiredPresent returns the rule sources that carry SPEC-CONDRULE-001
// hook-fired metadata, i.e. a `condition:` frontmatter key.
func stickyHookFiredPresent(t *testing.T) []string {
	t.Helper()
	var fired []string
	for _, name := range stickyContentRuleNames(t) {
		frontmatter, ok := stickyFrontmatterOf(stickyContentSource(t, name))
		if !ok {
			continue
		}
		for _, line := range strings.Split(frontmatter, "\n") {
			if strings.HasPrefix(line, "condition:") {
				fired = append(fired, name)
				break
			}
		}
	}
	return fired
}

// S7: the sticky flag is orthogonal — it relocates nothing and adds no paths
// field. The expected claude rule set is derived from the sources' own
// SPEC-CONDRULE-001 hook-fired metadata, and the pinned baseline of 11 is
// asserted exactly whenever that relocation metadata is present in the tree.
func TestStickyFrontmatter_S7_ClaudeRuleSurfaceIsUnchanged(t *testing.T) {
	files := stickyGenerate(t, "claude")

	var got []string
	for _, f := range files {
		p := filepath.ToSlash(f.TargetPath)
		if path.Dir(p) == ".claude/rules/autopus" {
			got = append(got, path.Base(p))
		}
	}

	hookFired := stickyHookFiredPresent(t)
	fired := make(map[string]bool, len(hookFired))
	for _, name := range hookFired {
		fired[name] = true
	}
	var want []string
	for _, name := range stickyContentRuleNames(t) {
		if !fired[name] {
			want = append(want, name)
		}
	}

	assert.ElementsMatch(t, want, got,
		".claude/rules/autopus must hold every non-hook-fired content rule")
	assert.Contains(t, got, "language-policy.md")
	assert.Contains(t, got, "objective-reasoning.md")

	if len(hookFired) == len(stickyHookFiredRules) {
		assert.ElementsMatch(t, stickyHookFiredRules, hookFired,
			"the hook-fired set S7 subtracts must be the SPEC-CONDRULE-001 triple")
		assert.Len(t, got, 11, "S7 baseline: 14 content rules minus 3 hook-fired")
	}

	for _, rule := range stickyRules {
		frontmatter, _ := stickySplitFrontmatter(t, stickyRuleEmission(t, files, "claude", rule))
		for _, line := range strings.Split(frontmatter, "\n") {
			assert.False(t, strings.HasPrefix(line, "paths:"),
				"%s must not gain a paths field from the sticky flag", rule)
		}
	}
}

// TestStickyFrontmatter_MapsExactlyTwoRules is the REQ-STICKYRULE-MAP-01 census.
// It scans every emitted rule on every platform rather than sampling a single
// control, so a rule that picks up the key from a template edit or a synthesized
// default is caught wherever it appears.
func TestStickyFrontmatter_MapsExactlyTwoRules(t *testing.T) {
	for _, platform := range []string{"claude", "opencode", "gemini"} {
		t.Run(platform, func(t *testing.T) {
			rules := generatePlatformRules(t, platform)
			require.NotEmpty(t, rules, "%s must emit rules", platform)

			var sticky []string
			for name, content := range rules {
				if strings.Contains(content, stickyKeyLine) {
					sticky = append(sticky, name)
				}
			}
			assert.ElementsMatch(t, stickyRules, sticky,
				"%s must mark exactly language-policy and objective-reasoning sticky", platform)
		})
	}
}
