package rulecond_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/rulecond"
)

// hostileStickyLayout builds the S12 layout: one contained control rule plus one
// entry per containment or size violation, and a decoy secret one level above
// the sticky body root.
func hostileStickyLayout(t *testing.T) *stickyFixture {
	t.Helper()
	f := newStickyFixture(t)

	require.NoError(t, os.WriteFile(filepath.Join(f.root, "secrets.md"),
		[]byte("# secrets\n\n"+decoySecret+"\n"), 0o644))

	f.writeStickyBody(ruleLanguagePolicy+".md", sourceRuleFile(t, ruleLanguagePolicy))
	f.writeStickyBody("notes.txt", "# notes\n\nNOTES-MARKER\n")
	f.writeStickyBody("huge.md", syntheticStickyFile("huge", "HUGE-MARKER", 4001))
	require.NoError(t, os.MkdirAll(filepath.Join(f.stickyRoot(), "autopus"), 0o755))
	symlinkOrSkip(t, "../../../secrets.md", filepath.Join(f.stickyRoot(), "escape.md"))

	f.writeStickyManifest(
		rulecond.StickyRule{Name: ruleLanguagePolicy, Body: ruleLanguagePolicy + ".md"},
		rulecond.StickyRule{Name: "evil-absolute", Body: "/etc/passwd"},
		rulecond.StickyRule{Name: "evil-escape", Body: "../../../secrets.md"},
		rulecond.StickyRule{Name: "evil-symlink", Body: "escape.md"},
		rulecond.StickyRule{Name: "evil-dir", Body: "autopus"},
		rulecond.StickyRule{Name: "evil-ext", Body: "notes.txt"},
		rulecond.StickyRule{Name: "evil-huge", Body: "huge.md"},
	)
	return f
}

// TestStickyFire_BodyReadsAreConfinedToTheStickyRoot is the S12 containment
// oracle for REQ-STICKYRULE-FIRE-06, REQ-STICKYRULE-FIRE-07, and
// REQ-STICKYRULE-FIRE-09. Without it the runtime is an arbitrary-file-read to
// context-injection primitive that fires on a fixed cadence.
func TestStickyFire_BodyReadsAreConfinedToTheStickyRoot(t *testing.T) {
	f := hostileStickyLayout(t)

	stdout, stderr := f.submitAt("S-hostile", 1)
	ctx := stickyContext(t, stdout)

	assert.Contains(t, ctx, injectableBody(t, ruleLanguagePolicy),
		"the contained control rule still injects alongside six refusals")
	for _, forbidden := range []string{
		decoySecret,   // the escaped and symlinked decoy body
		"HUGE-MARKER", // the 4001-byte body
		"NOTES-MARKER",
		"/etc/passwd",
		"secrets.md",
	} {
		assert.NotContains(t, ctx, forbidden,
			"a refused body must not contribute a single byte to additionalContext")
	}
	for _, name := range []string{"evil-absolute", "evil-escape", "evil-symlink", "evil-dir", "evil-ext", "evil-huge"} {
		assert.NotContains(t, ctx, name, "a refused rule is not named in the injected context")
	}

	assert.ElementsMatch(t, []string{
		"evil-absolute " + string(rulecond.ReasonAbsolutePath),
		"evil-escape " + string(rulecond.ReasonPathEscape),
		"evil-symlink " + string(rulecond.ReasonSymlinkEscape),
		"evil-dir " + string(rulecond.ReasonNotRegularFile),
		"evil-ext " + string(rulecond.ReasonBadExtension),
		"evil-huge " + string(rulecond.ReasonBodyTooLarge),
	}, stderrLines(stderr), "each refusal reports its rule name and reason code exactly once")

	for _, leak := range []string{"/etc/passwd", "secrets.md", decoySecret, f.stickyRoot()} {
		assert.NotContains(t, stderr, leak,
			"a fail-closed diagnostic carries no resolved path and no file content")
	}
}

// TestStickyFire_MissingBodyIsBenignNotAViolation separates the two per-rule
// cases of REQ-STICKYRULE-FIRE-08: an absent body is silent, a refused body is
// named on stderr.
func TestStickyFire_MissingBodyIsBenignNotAViolation(t *testing.T) {
	f := newStickyFixture(t)
	f.writeStickyBody(ruleLanguagePolicy+".md", sourceRuleFile(t, ruleLanguagePolicy))
	f.writeStickyManifest(
		rulecond.StickyRule{Name: ruleLanguagePolicy, Body: ruleLanguagePolicy + ".md"},
		rulecond.StickyRule{Name: ruleObjectiveReasoning, Body: ruleObjectiveReasoning + ".md"},
	)

	stdout, stderr := f.submitAt("S-missing", 1)
	ctx := stickyContext(t, stdout)

	assert.Contains(t, ctx, injectableBody(t, ruleLanguagePolicy),
		"every other sticky rule still injects")
	assert.NotContains(t, ctx, ruleObjectiveReasoning,
		"the rule whose body is gone is skipped entirely")
	assert.Empty(t, stderr,
		"a body file that simply does not exist is benign absence, not a violation")
}

// TestStickyFire_SymlinkedStickyRootIsRefused covers a cloned repository that
// ships the sticky body root itself as a symlink (git mode 120000 survives a
// clone), which would otherwise relocate the containment frame.
func TestStickyFire_SymlinkedStickyRootIsRefused(t *testing.T) {
	f := newStickyFixture(t)
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, ruleLanguagePolicy+".md"),
		[]byte("# harmless\n"+rootEscapeSecret+"\n"), 0o644))

	require.NoError(t, os.RemoveAll(f.stickyRoot()))
	symlinkOrSkip(t, outside, f.stickyRoot())
	f.writeStickyManifest(
		rulecond.StickyRule{Name: ruleLanguagePolicy, Body: ruleLanguagePolicy + ".md"},
	)

	stdout, stderr := f.submitAt("S-symroot", 1)

	assert.Empty(t, stickyContext(t, stdout),
		"a relocated sticky root injects nothing")
	assert.NotContains(t, stdout, rootEscapeSecret)
	assert.Equal(t, []string{ruleLanguagePolicy + " " + string(rulecond.ReasonSymlinkRoot)},
		stderrLines(stderr))
}
