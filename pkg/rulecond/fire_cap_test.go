package rulecond_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/rulecond"
)

// padTo returns a body of exactly size bytes whose first line is marker.
func padTo(marker string, size int) string {
	head := "# " + marker + "\n"
	if len(head) >= size {
		return head[:size]
	}
	return head + strings.Repeat("x", size-len(head))
}

// TestFire_AggregateCapDropsInReverseNameOrder is the S9 truncation oracle for
// REQ-CONDRULE-FIRE-05.
func TestFire_AggregateCapDropsInReverseNameOrder(t *testing.T) {
	t.Parallel()

	require.Equal(t, 8000, rulecond.MaxContextBytes)
	require.Equal(t, 4000, rulecond.MaxBodyBytes)

	f := newFixture(t)
	f.writeBody("lore-commit.md", padTo("LORE-BODY", 3500))
	f.writeBody("shell-portability.md", padTo("SHELL-BODY", 3500))
	f.writeBody("worktree-safety.md", padTo("WORKTREE-BODY", 3000))
	f.writeManifest(
		bashRule("lore-commit", condLoreCommit, "lore-commit.md"),
		bashRule("shell-portability", condShellPortable, "shell-portability.md"),
		bashRule("worktree-safety", condWorktreeSafety, "worktree-safety.md"),
	)

	command := "timeout 5 git gc && git commit -m x"
	stdout, stderr := f.fire(bashPayload(command))

	ctx := injectedContext(t, stdout)
	assert.Equal(t, []string{"lore-commit", "shell-portability"}, firedRuleNames(t, stdout),
		"the first two rules by name order are kept")
	assert.LessOrEqual(t, len(ctx), rulecond.MaxContextBytes,
		"additionalContext must stay at or under the aggregate cap")

	assert.Contains(t, ctx, "LORE-BODY")
	assert.Contains(t, ctx, "SHELL-BODY")
	assert.NotContains(t, ctx, "WORKTREE-BODY", "the dropped rule body must be absent")

	notice := truncationNotice(t, ctx)
	assert.Contains(t, notice, "worktree-safety", "the omitted rule must be named")
	assert.NotContains(t, notice, command, "the notice must not echo tool input")
	assert.NotContains(t, notice, "git gc")
	assert.Empty(t, stderr, "aggregate truncation is not a fail-closed violation")
}

// TestFire_PerBodyCapPrecedesAggregateCap is the REQ-CONDRULE-FIRE-08 oracle:
// an oversize body is skipped whole, and the remaining rules still inject.
func TestFire_PerBodyCapPrecedesAggregateCap(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.writeBody("lore-commit.md", padTo("LORE-BODY", 1200))
	f.writeBody("worktree-safety.md", padTo("WORKTREE-OVERSIZE", rulecond.MaxBodyBytes+1))
	f.writeManifest(
		bashRule("lore-commit", condLoreCommit, "lore-commit.md"),
		bashRule("worktree-safety", condWorktreeSafety, "worktree-safety.md"),
	)

	stdout, stderr := f.fire(bashPayload("git gc --prune=now && git commit -m x"))

	ctx := injectedContext(t, stdout)
	assert.Equal(t, []string{"lore-commit"}, firedRuleNames(t, stdout))
	assert.Contains(t, ctx, "LORE-BODY")
	assert.NotContains(t, ctx, "WORKTREE-OVERSIZE", "no part of an oversize body may inject")
	assert.Equal(t, "worktree-safety "+string(rulecond.ReasonBodyTooLarge),
		strings.TrimSpace(stderr), "oversize is a fail-closed violation")
}

// truncationNotice returns the single truncation line from an injected context.
func truncationNotice(t *testing.T, ctx string) string {
	t.Helper()
	var found []string
	for _, line := range strings.Split(ctx, "\n") {
		if strings.HasPrefix(line, rulecond.TruncationPrefix) {
			found = append(found, line)
		}
	}
	require.Len(t, found, 1, "exactly one one-line truncation notice is expected")
	return found[0]
}
