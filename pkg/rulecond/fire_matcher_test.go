package rulecond_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/rulecond"
)

// matcherRule builds a manifest rule with an explicit tool matcher so the
// matcher field, rather than the condition, is what discriminates.
func matcherRule(name, matcher, condition, body string) rulecond.ManifestRule {
	return rulecond.ManifestRule{
		Name:       name,
		Event:      preToolUse,
		Matcher:    matcher,
		Conditions: []string{condition},
		Body:       body,
	}
}

// TestFire_MatcherGatesIndependentlyOfCondition proves the manifest matcher is
// load-bearing for REQ-CONDRULE-FIRE-02. Every S2 row where a Bash rule stays
// silent on an Edit call is also explained by a condition mismatch, so this
// pins the matcher with a condition that matches both subjects.
func TestFire_MatcherGatesIndependentlyOfCondition(t *testing.T) {
	t.Parallel()

	// `/repo/x.go` and `go build ./...` both contain "go", so only the matcher
	// can keep the Bash-scoped rule off the Edit call.
	const shared = `go`

	tests := []struct {
		name    string
		payload string
		want    []string
	}{
		{
			name:    "bash rule fires on the bash tool",
			payload: bashPayload("go build ./..."),
			want:    []string{"bash-only"},
		},
		{
			name:    "bash rule stays silent on the edit tool",
			payload: filePayload("Edit", "/repo/x.go"),
			want:    []string{"edit-only"},
		},
		{
			name:    "edit rule stays silent on the bash tool",
			payload: bashPayload("go test ./..."),
			want:    []string{"bash-only"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			f.writeBody("bash-only.md", "# bash-only\n\nBASH-BODY\n")
			f.writeBody("edit-only.md", "# edit-only\n\nEDIT-BODY\n")
			f.writeManifest(
				matcherRule("bash-only", bashMatcher, shared, "bash-only.md"),
				matcherRule("edit-only", editMatcher, shared, "edit-only.md"),
			)

			stdout, stderr := f.fire(tt.payload)
			assert.Equal(t, tt.want, firedRuleNames(t, stdout))
			assert.Empty(t, stderr)
		})
	}
}

// TestFire_MatcherIsWholeNameAnchored pins the `^(?:...)$` wrapper in
// compileMatcher. An unanchored matcher `Edit` would substring-match the tool
// name `MultiEdit` and fire a rule scoped to a different tool.
func TestFire_MatcherIsWholeNameAnchored(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.writeBody("edit-exact.md", "# edit-exact\n\nEDIT-EXACT-BODY\n")
	f.writeManifest(matcherRule("edit-exact", "Edit", `\.go$`, "edit-exact.md"))

	t.Run("exact tool name matches", func(t *testing.T) {
		stdout, _ := f.fire(filePayload("Edit", "/repo/a.go"))
		assert.Equal(t, []string{"edit-exact"}, firedRuleNames(t, stdout))
	})

	t.Run("tool name containing the matcher does not match", func(t *testing.T) {
		stdout, stderr := f.fire(filePayload("MultiEdit", "/repo/a.go"))
		assert.Equal(t, []string{}, firedRuleNames(t, stdout),
			"an unanchored matcher would fire `Edit` on `MultiEdit`")
		assert.Empty(t, stdout)
		assert.Empty(t, stderr)
	})
}

// TestFire_EmptyMatcherAppliesToEveryTool pins the Claude Code convention that
// an absent matcher is a wildcard rather than a non-match.
func TestFire_EmptyMatcherAppliesToEveryTool(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.writeBody("any-tool.md", "# any-tool\n\nANY-TOOL-BODY\n")
	f.writeManifest(matcherRule("any-tool", "", `\.go$`, "any-tool.md"))

	for _, tool := range []string{"Edit", "Write", "MultiEdit"} {
		stdout, _ := f.fire(filePayload(tool, "/repo/a.go"))
		assert.Equal(t, []string{"any-tool"}, firedRuleNames(t, stdout), "tool %s", tool)
	}

	// A tool outside the condition-subject set still yields no subject, so an
	// empty matcher never widens REQ-CONDRULE-FIRE-02.
	stdout, _ := f.fire(filePayload("Read", "/repo/a.go"))
	assert.Empty(t, stdout, "an empty matcher must not bypass the condition-subject set")
}

// TestFire_UncompilableRegexSuppressesTheWholeRun is the multi-rule
// discriminator for REQ-CONDRULE-FIRE-03. The S4 row uses a single-rule
// manifest, so it cannot tell whole-run fail-open apart from skipping only the
// broken rule. Compiling lazily inside the match loop would let one hostile
// manifest entry silently disable the rules sorted after it.
func TestFire_UncompilableRegexSuppressesTheWholeRun(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.writeBody("lore-commit.md", sourceRuleBody(t, "lore-commit"))
	f.writeBody("broken.md", "# broken\n\nBROKEN-BODY\n")

	// `broken` sorts before `lore-commit`, so a lazy compile would still inject
	// lore-commit; the whole run must go silent instead.
	f.writeManifest(
		bashRule("broken", "a(", "broken.md"),
		bashRule("lore-commit", condLoreCommit, "lore-commit.md"),
	)

	stdout, stderr := f.fire(bashPayload("git commit -m x"))

	require.Empty(t, stdout,
		"one uncompilable stored regex fails the whole run open, not a partial rule set")
	assert.Empty(t, stderr, "an uncompilable regex is benign absence, not a fail-closed violation")
	assert.Equal(t, []string{}, firedRuleNames(t, stdout))
}

// TestFire_ValidManifestIsUnaffectedByRuleOrder confirms the all-or-nothing
// compile does not depend on manifest ordering once every regex is valid.
func TestFire_ValidManifestIsUnaffectedByRuleOrder(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.writeBody("lore-commit.md", sourceRuleBody(t, "lore-commit"))
	f.writeBody("worktree-safety.md", sourceRuleBody(t, "worktree-safety"))
	f.writeManifest(
		bashRule("worktree-safety", condWorktreeSafety, "worktree-safety.md"),
		bashRule("lore-commit", condLoreCommit, "lore-commit.md"),
	)

	stdout, stderr := f.fire(bashPayload("git gc --prune=now && git commit -m x"))

	assert.Equal(t, []string{"lore-commit", "worktree-safety"}, firedRuleNames(t, stdout),
		"matched rules are emitted in rule-name order regardless of manifest order")
	assert.Empty(t, stderr)
}
