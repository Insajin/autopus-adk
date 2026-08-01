package rulecond_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/rulecond"
)

const decoySecret = "EXAMPLEFAKESECRET"

// hostileLayout plants the S14 decoys around and inside the conditional body
// root and returns the prepared fixture.
func hostileLayout(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)

	decoy := "# secrets\n\n" + decoySecret + "\n"
	// The repository-root decoy and the decoy that `../../../secrets.md`
	// actually resolves to. Both must stay unreachable.
	require.NoError(t, os.WriteFile(filepath.Join(f.root, "secrets.md"), []byte(decoy), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(f.root, ".claude", "secrets.md"), []byte(decoy), 0o644))

	f.writeBody("lore-commit.md", sourceRuleBody(t, "lore-commit"))
	f.writeBody("notes.txt", "# notes\n")
	f.writeBody("huge.md", padTo("HUGE-BODY", rulecond.MaxBodyBytes+1))
	require.NoError(t, os.MkdirAll(filepath.Join(f.bodyRoot(), "subdir"), 0o755))
	require.NoError(t, os.Symlink(
		filepath.Join("..", "..", "..", "secrets.md"),
		filepath.Join(f.bodyRoot(), "escape.md")))

	return f
}

// containmentRows is the S14 manifest-body table.
var containmentRows = []struct {
	name   string
	body   string
	reason rulecond.Reason
}{
	{name: "evil-abs", body: "/etc/passwd", reason: rulecond.ReasonAbsolutePath},
	{name: "evil-escape", body: "../../../secrets.md", reason: rulecond.ReasonPathEscape},
	{name: "evil-symlink", body: "escape.md", reason: rulecond.ReasonSymlinkEscape},
	{name: "evil-dir", body: "subdir", reason: rulecond.ReasonNotRegularFile},
	{name: "evil-ext", body: "notes.txt", reason: rulecond.ReasonBadExtension},
	{name: "evil-huge", body: "huge.md", reason: rulecond.ReasonBodyTooLarge},
	{name: "evil-reentry", body: "../conditional/lore-commit.md", reason: rulecond.ReasonPathEscape},
}

// TestReadBody_ContainmentViolations is the S14 read-side oracle for
// REQ-CONDRULE-FIRE-07 and REQ-CONDRULE-FIRE-08.
func TestReadBody_ContainmentViolations(t *testing.T) {
	t.Parallel()
	f := hostileLayout(t)

	t.Run("control body reads", func(t *testing.T) {
		data, err := rulecond.ReadBody(f.bodyRoot(), "lore-commit.md")
		require.NoError(t, err)
		assert.Contains(t, string(data), loreCommitSignature)
	})

	for _, row := range containmentRows {
		t.Run(row.name+" "+string(row.reason), func(t *testing.T) {
			data, err := rulecond.ReadBody(f.bodyRoot(), row.body)
			require.Error(t, err, "row %s must be refused", row.name)
			assert.Empty(t, data, "no byte of a refused body may be returned")

			var violation *rulecond.ViolationError
			require.ErrorAs(t, err, &violation, "a containment failure must be fail-closed")
			assert.Equal(t, row.reason, violation.Reason)

			msg := err.Error()
			assert.NotContains(t, msg, decoySecret)
			assert.NotContains(t, msg, f.bodyRoot(), "the resolved path must not leak")
			assert.NotContains(t, msg, "/etc/passwd")
		})
	}
}

// TestReadBody_MissingBodyIsBenignAbsence separates the fail-open class of
// REQ-CONDRULE-FIRE-09 from the fail-closed class.
func TestReadBody_MissingBodyIsBenignAbsence(t *testing.T) {
	t.Parallel()
	f := hostileLayout(t)

	data, err := rulecond.ReadBody(f.bodyRoot(), "absent.md")
	require.Error(t, err)
	assert.Empty(t, data)
	assert.True(t, errors.Is(err, fs.ErrNotExist),
		"a missing body is benign absence, not a containment violation")

	var violation *rulecond.ViolationError
	assert.False(t, errors.As(err, &violation),
		"a missing body must not be reported as a containment violation")
}

// TestFire_HostileManifestConfinesBodyReads is the S14 dispatcher oracle.
func TestFire_HostileManifestConfinesBodyReads(t *testing.T) {
	t.Parallel()
	f := hostileLayout(t)

	rules := []rulecond.ManifestRule{bashRule("lore-commit", condLoreCommit, "lore-commit.md")}
	for _, row := range containmentRows {
		rules = append(rules, bashRule(row.name, condLoreCommit, row.body))
	}
	f.writeManifest(rules...)

	stdout, stderr := f.fire(bashPayload("git commit -m x"))

	ctx := injectedContext(t, stdout)
	assert.Equal(t, []string{"lore-commit"}, firedRuleNames(t, stdout),
		"only the contained control body may inject")
	assert.Contains(t, ctx, loreCommitSignature)
	assert.NotContains(t, ctx, decoySecret)
	assert.NotContains(t, ctx, "root:x:0:0")
	assert.NotContains(t, ctx, "HUGE-BODY")

	lines := stderrLines(stderr)
	require.Len(t, lines, len(containmentRows),
		"one diagnostic line per suppressed rule")
	for _, row := range containmentRows {
		want := row.name + " " + string(row.reason)
		assert.Equal(t, 1, countLines(lines, want), "stderr line %q", want)
	}

	assert.NotContains(t, stderr, "/etc/passwd")
	assert.NotContains(t, stderr, "secrets.md")
	assert.NotContains(t, stderr, decoySecret)
	assert.NotContains(t, stderr, f.bodyRoot())
}

func stderrLines(stderr string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(stderr), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func countLines(lines []string, want string) int {
	n := 0
	for _, line := range lines {
		if line == want {
			n++
		}
	}
	return n
}
