package rulecond_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/rulecond"
)

// TestReadBody_SiblingDirectoryIsNotWithinRoot pins the trailing separator in
// withinRoot. The conditional body root is `<...>/conditional`, so a sibling
// directory `<...>/conditional-evil` shares a bare string prefix with it. A
// containment check written as strings.HasPrefix(resolved, root) would admit
// the sibling and hand the dispatcher an out-of-root read.
func TestReadBody_SiblingDirectoryIsNotWithinRoot(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	bodyRoot := f.bodyRoot()

	// `<parent>/conditional-evil/leak.md` is a sibling of the body root, not a
	// child, yet `conditional-evil` begins with `conditional`.
	sibling := bodyRoot + "-evil"
	require.NoError(t, os.MkdirAll(sibling, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sibling, "leak.md"),
		[]byte("# leak\n\n"+decoySecret+"\n"), 0o644))

	// The location itself carries no `..`, so only the post-EvalSymlinks
	// containment check can refuse it.
	require.NoError(t, os.Symlink(
		filepath.Join("..", filepath.Base(sibling), "leak.md"),
		filepath.Join(bodyRoot, "sibling.md")))

	data, err := rulecond.ReadBody(bodyRoot, "sibling.md")
	require.Error(t, err, "a sibling-directory symlink must not be contained")
	assert.Empty(t, data)
	assert.NotContains(t, string(data), decoySecret)

	var violation *rulecond.ViolationError
	require.ErrorAs(t, err, &violation)
	assert.Equal(t, rulecond.ReasonSymlinkEscape, violation.Reason)
	assert.NotContains(t, err.Error(), decoySecret)
	assert.NotContains(t, err.Error(), sibling)
}

// TestReadBody_ContainedSymlinkStaysAdmissible is the negative control for the
// containment check: acceptance.md fixes that a symlink whose target stays
// inside the root remains readable, so the escape guard must not degrade into
// a blanket symlink ban.
func TestReadBody_ContainedSymlinkStaysAdmissible(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	bodyRoot := f.bodyRoot()
	f.writeBody("real.md", "# real\n\nCONTAINED-BODY\n")
	require.NoError(t, os.Symlink("real.md", filepath.Join(bodyRoot, "alias.md")))

	data, err := rulecond.ReadBody(bodyRoot, "alias.md")
	require.NoError(t, err, "a symlink resolving inside the root stays admissible")
	assert.Contains(t, string(data), "CONTAINED-BODY")
}

// TestReadBody_ExactCapBoundary pins MaxBodyBytes as an exact upper bound
// rather than an approximation, per the acceptance tolerance note.
func TestReadBody_ExactCapBoundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		size    int
		wantErr bool
	}{
		{name: "one byte under the cap", size: rulecond.MaxBodyBytes - 1},
		{name: "exactly at the cap", size: rulecond.MaxBodyBytes},
		{name: "one byte over the cap", size: rulecond.MaxBodyBytes + 1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			f.writeBody("body.md", padTo("CAP-BODY", tt.size))

			data, err := rulecond.ReadBody(f.bodyRoot(), "body.md")
			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, data, "no byte of an oversize body may be returned")
				var violation *rulecond.ViolationError
				require.ErrorAs(t, err, &violation)
				assert.Equal(t, rulecond.ReasonBodyTooLarge, violation.Reason)
				return
			}
			require.NoError(t, err)
			assert.Len(t, data, tt.size, "a body at or under the cap reads whole")
		})
	}
}

// TestCompileClaude_HostileRuleNameFailsGeneration is the remaining half of the
// S14 compile-time clause for REQ-CONDRULE-COMPILE-06. A rule name is what
// becomes the manifest body location, so a name that would compile into an
// out-of-root location must fail generation by name rather than emit an entry
// the dispatcher later refuses.
func TestCompileClaude_HostileRuleNameFailsGeneration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		ruleName   string
		wantReason rulecond.Reason
	}{
		{name: "absolute", ruleName: "/etc/passwd", wantReason: rulecond.ReasonAbsolutePath},
		{name: "parent traversal", ruleName: "../../../secrets", wantReason: rulecond.ReasonPathEscape},
		{name: "nested separator", ruleName: "sub/dir", wantReason: rulecond.ReasonPathEscape},
		{name: "backslash separator", ruleName: `sub\dir`, wantReason: rulecond.ReasonPathEscape},
		{name: "empty name", ruleName: "   ", wantReason: rulecond.ReasonBadExtension},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			compiled, err := rulecond.CompileClaude([]*rulecond.Rule{
				hookFiredRule("lore-commit", condLoreCommit),
				hookFiredRule(tt.ruleName, condWorktreeSafety),
			})

			require.Error(t, err, "generation must fail rather than emit a refused entry")
			assert.Nil(t, compiled, "no partial surface may be returned")
			assert.Contains(t, err.Error(), string(tt.wantReason),
				"the error must carry the reason code")

			var violation *rulecond.ViolationError
			require.ErrorAs(t, err, &violation)
			assert.Equal(t, tt.wantReason, violation.Reason)
		})
	}
}

// TestCompileClaude_HostileRuleNameErrorNamesTheRule keeps the operator-facing
// half of REQ-CONDRULE-COMPILE-06 explicit: the failure identifies which rule
// blocked generation.
func TestCompileClaude_HostileRuleNameErrorNamesTheRule(t *testing.T) {
	t.Parallel()

	_, err := rulecond.CompileClaude([]*rulecond.Rule{
		hookFiredRule("lore-commit", condLoreCommit),
		hookFiredRule("../../../secrets", condWorktreeSafety),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "../../../secrets", "the error must name the offending rule")
	assert.True(t, strings.Contains(err.Error(), string(rulecond.ReasonPathEscape)))
}

// TestCompileClaude_BodyAtCapCompiles is the negative control for the
// compile-time size check, so the guard cannot drift below the shared cap.
func TestCompileClaude_BodyAtCapCompiles(t *testing.T) {
	t.Parallel()

	atCap := hookFiredRule("worktree-safety", condWorktreeSafety)
	atCap.Body = padTo("AT-CAP", rulecond.MaxBodyBytes)

	compiled, err := rulecond.CompileClaude([]*rulecond.Rule{atCap})
	require.NoError(t, err, "a body exactly at the cap must still compile")
	require.Len(t, compiled.Bodies, 1)
	assert.Len(t, compiled.Bodies[0].Content, rulecond.MaxBodyBytes)
}
