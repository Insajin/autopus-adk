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

// rootEscapeSecret stands in for any file the dispatcher must never read.
const rootEscapeSecret = "PRIVATE-NOTES-ACQUISITION-PRICE"

// symlinkOrSkip creates a symlink, skipping where the platform forbids one.
func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
}

// TestFire_SymlinkedBodyRootIsRefused covers the conditional body root shipped
// as a symlink. EvalSymlinks on the root moved the containment frame with it,
// so a sibling file passed the within-root comparison and injected under a
// harmless rule name.
func TestFire_SymlinkedBodyRootIsRefused(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "x.md"),
		[]byte("# harmless\n"+rootEscapeSecret+"\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".claude", "hooks", "autopus"), 0o755))
	symlinkOrSkip(t, outside, filepath.Join(root, ".claude", "hooks", "autopus", "conditional"))

	f := &fixture{t: t, root: root}
	f.writeManifest(bashRule("harmless-lint-rule", condLoreCommit, "x.md"))

	stdout, stderr := f.fire(bashPayload("git commit -m x"))

	assert.Empty(t, stdout, "a relocated body root must inject nothing")
	assert.NotContains(t, stdout, rootEscapeSecret)
	assert.Equal(t, "harmless-lint-rule "+string(rulecond.ReasonSymlinkRoot),
		strings.TrimSpace(stderr), "the suppression is fail-closed and named")
}

// TestFire_SymlinkedClaudeDirRelocatesManifest covers the same class one level
// up: `.claude` itself is the symlink, which moves the manifest out of the
// checkout before any rule is known.
func TestFire_SymlinkedClaudeDirRelocatesManifest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	bodyDir := filepath.Join(outside, "hooks", "autopus", "conditional")
	require.NoError(t, os.MkdirAll(bodyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bodyDir, "x.md"),
		[]byte("# harmless\n"+rootEscapeSecret+"\n"), 0o644))
	require.NoError(t, os.WriteFile(
		filepath.Join(outside, "hooks", "autopus", "conditional-rules.json"),
		[]byte(`{"version":"1","rules":[{"name":"harmless-lint-rule","event":"PreToolUse",`+
			`"matcher":"Bash","conditions":["\\bgit\\s+commit\\b"],"body":"x.md"}]}`), 0o644))
	symlinkOrSkip(t, outside, filepath.Join(root, ".claude"))

	f := &fixture{t: t, root: root}
	stdout, stderr := f.fire(bashPayload("git commit -m x"))

	assert.Empty(t, stdout, "a relocated manifest must inject nothing")
	assert.NotContains(t, stdout, rootEscapeSecret)
	assert.Equal(t, "conditional-rules "+string(rulecond.ReasonSymlinkRoot),
		strings.TrimSpace(stderr), "the run-level refusal is reported once")
}

// TestFire_HardlinkedBodyIsRefused covers the link the path resolver cannot
// see: the body sits inside the root by every path check, but its bytes are
// also reachable from outside it.
func TestFire_HardlinkedBodyIsRefused(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.md")
	require.NoError(t, os.WriteFile(secret,
		[]byte("# harmless\n"+rootEscapeSecret+"\n"), 0o644))
	if err := os.Link(secret, filepath.Join(f.bodyRoot(), "x.md")); err != nil {
		t.Skipf("hardlink unavailable: %v", err)
	}
	f.writeManifest(bashRule("harmless-lint-rule", condLoreCommit, "x.md"))

	stdout, stderr := f.fire(bashPayload("git commit -m x"))

	assert.Empty(t, stdout, "a hardlinked body must inject nothing")
	assert.NotContains(t, stdout, rootEscapeSecret)
	assert.Equal(t, "harmless-lint-rule "+string(rulecond.ReasonHardLink),
		strings.TrimSpace(stderr))
}

// TestFire_MatcherCannotEscapeItsAnchor covers the matcher that closes the
// `^(?:...)$` group early. `Bash)|(.*` made a Bash-scoped rule fire on Write.
func TestFire_MatcherCannotEscapeItsAnchor(t *testing.T) {
	t.Parallel()

	hostileMatchers := []string{
		`Bash)|(.*`,
		`Bash)|(`,
		`.*`,
		`Bash|.*`,
	}

	for _, matcher := range hostileMatchers {
		t.Run(matcher, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			f.writeBody("x.md", "# body\nBASH-SCOPED-BODY\n")
			f.writeManifest(rulecond.ManifestRule{
				Name:       "bash-only-rule",
				Event:      preToolUse,
				Matcher:    matcher,
				Conditions: []string{`.*`},
				Body:       "x.md",
			})

			stdout, _ := f.fire(filePayload("Write", "/tmp/a.go"))

			assert.Empty(t, stdout, "a Bash-scoped rule must not fire on Write")
			assert.NotContains(t, stdout, "BASH-SCOPED-BODY")
		})
	}
}

// TestFire_LiteralMatchersStillFire is the positive control for the matcher
// change: the compiler only ever emits literal tool names, and both shapes it
// emits must keep working.
func TestFire_LiteralMatchersStillFire(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		matcher string
		stdin   string
	}{
		{name: "bash", matcher: bashMatcher, stdin: bashPayload("git commit -m x")},
		{name: "edit alternation on Write", matcher: editMatcher, stdin: filePayload("Write", "/tmp/a.go")},
		{name: "edit alternation on MultiEdit", matcher: editMatcher, stdin: filePayload("MultiEdit", "/tmp/a.go")},
		// An empty matcher applies to every tool that has a condition subject.
		// Read and the other subject-less tools are excluded one layer earlier,
		// by ConditionSubject, not by the matcher.
		{name: "empty matches every tool", matcher: "", stdin: filePayload("Write", "/tmp/a.go")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			f.writeBody("x.md", "# body\nLEGIT-BODY-FIRED\n")
			f.writeManifest(rulecond.ManifestRule{
				Name:       "legit-rule",
				Event:      preToolUse,
				Matcher:    tt.matcher,
				Conditions: []string{`.*`},
				Body:       "x.md",
			})

			stdout, stderr := f.fire(tt.stdin)

			assert.Contains(t, injectedContext(t, stdout), "LEGIT-BODY-FIRED",
				"a legitimate matcher must still fire")
			assert.Empty(t, stderr)
		})
	}
}

// TestResolveBodyRoot_ToleratesSymlinkedSystemPrefix keeps the fix from
// over-refusing. t.TempDir hands back a path under /var on macOS, which is
// itself a symlink to /private/var, and that prefix sits above the project root.
func TestResolveBodyRoot_ToleratesSymlinkedSystemPrefix(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.writeBody("lore-commit.md", "# body\nPREFIX-TOLERATED\n")
	f.writeManifest(bashRule("lore-commit", condLoreCommit, "lore-commit.md"))

	stdout, stderr := f.fire(bashPayload("git commit -m x"))

	assert.Contains(t, injectedContext(t, stdout), "PREFIX-TOLERATED")
	assert.Empty(t, stderr)

	resolved, err := rulecond.ResolveBodyRoot(f.root)
	require.NoError(t, err, "a legitimate project root must resolve")
	assert.True(t, strings.HasSuffix(filepath.ToSlash(resolved), rulecond.BodyRootRelPath))
}
