package rulecond_test

// SPEC-STICKYRULE-001 write-side containment oracles for the counter entry
// itself.
//
// Containing the directory components is only half the state path. The counter
// file is the leaf, and the leaf carries the same escape: a repository can
// commit `.autopus/runtime/sticky-rules/<64-hex>` as a symlink with git mode
// 120000, which survives a clone, and an unguarded write then creates or
// truncates whatever the link resolves to. The filename is no defence — it is a
// digest of the untrusted session_id, so the attacker chooses it, and a payload
// that omits session_id lands on the digest of the empty string.
//
// These rows plant the hostile entry at the running session's own key, which is
// what makes the write path exercise it. The sweep-only oracle in
// sticky_state_contain_test.go plants at a different key and therefore never
// reaches the write.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/rulecond"
)

// leafSession is the session whose own counter name carries the planted entry.
const leafSession = "S-leaf"

// outsideTree is a snapshot of every regular file under a directory and its
// bytes, so a row can prove that nothing outside the checkout was created,
// truncated, or rewritten rather than only that the file count held steady.
func outsideTree(t *testing.T, dir string) map[string]string {
	t.Helper()

	snapshot := make(map[string]string)
	for _, rel := range filesUnder(t, dir) {
		raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		require.NoError(t, err)
		snapshot[rel] = string(raw)
	}
	return snapshot
}

// plantedLeaf prepares one hostile counter entry at the running session's key,
// pointing at the given outside directory.
type plantedLeaf func(t *testing.T, leafPath, outside string)

// intactLeaf asserts the planted entry is still exactly what was planted, so a
// row proves the entry was refused rather than replaced, resolved, or deleted.
type intactLeaf func(t *testing.T, leafPath, outside string)

// stillSymlink is the check for both symlink rows.
func stillSymlink(t *testing.T, leafPath, _ string) {
	t.Helper()
	info, err := os.Lstat(leafPath)
	require.NoError(t, err, "the planted entry is refused, not deleted: it is repo content")
	assert.NotZero(t, info.Mode()&os.ModeSymlink,
		"the entry must remain a link, never be resolved or replaced by a counter file")
}

// stillHardlinked checks that the second directory entry still names the same
// outside inode and that neither view of it changed.
func stillHardlinked(t *testing.T, leafPath, outside string) {
	t.Helper()
	leaf, err := os.Lstat(leafPath)
	require.NoError(t, err, "the planted entry is refused, not deleted")
	target, err := os.Lstat(filepath.Join(outside, "victim"))
	require.NoError(t, err)
	assert.True(t, os.SameFile(leaf, target),
		"the entry must still be the outside inode, untouched")

	raw, err := os.ReadFile(leafPath)
	require.NoError(t, err)
	assert.Equal(t, "41\n", string(raw), "the shared inode must be byte-identical")
}

// dangling plants a symlink to a path that does not exist outside the checkout.
// Without the leaf guard the write creates that path as a fresh 0600 file.
func plantDanglingSymlink(t *testing.T, leafPath, outside string) {
	t.Helper()
	symlinkOrSkip(t, filepath.Join(outside, "created-by-the-hook"), leafPath)
}

// plantSymlinkToCounter plants a symlink to an existing outside file whose
// contents are already a bare integer, so the write path cannot distinguish it
// from state of its own and truncates it in place.
func plantSymlinkToCounter(t *testing.T, leafPath, outside string) {
	t.Helper()
	target := filepath.Join(outside, "victim")
	require.NoError(t, os.WriteFile(target, []byte("41\n"), 0o644))
	symlinkOrSkip(t, target, leafPath)
}

// plantHardlink plants a second directory entry for an outside inode. No amount
// of name resolution reveals it: the path stays inside the state directory while
// the bytes also live outside it.
func plantHardlink(t *testing.T, leafPath, outside string) {
	t.Helper()
	target := filepath.Join(outside, "victim")
	require.NoError(t, os.WriteFile(target, []byte("41\n"), 0o644))
	if err := os.Link(target, leafPath); err != nil {
		t.Skipf("hardlink unavailable: %v", err)
	}
}

// TestStickyFire_HostileCounterEntryIsNeverWrittenThrough is the leaf half of
// the write-side containment oracle. Each row plants a hostile entry at the
// running session's own counter name and requires the invocation to stay the
// whole-run benign case of REQ-STICKYRULE-FIRE-08: no stdout, no stderr, no
// error, and no byte created, truncated, or removed outside the checkout.
func TestStickyFire_HostileCounterEntryIsNeverWrittenThrough(t *testing.T) {
	rows := []struct {
		name   string
		plant  plantedLeaf
		intact intactLeaf
	}{
		{
			name:   "dangling symlink out of the checkout",
			plant:  plantDanglingSymlink,
			intact: stillSymlink,
		},
		{
			name:   "symlink to an outside counter-shaped file",
			plant:  plantSymlinkToCounter,
			intact: stillSymlink,
		},
		{
			name:   "hardlink to an outside file",
			plant:  plantHardlink,
			intact: stillHardlinked,
		},
	}

	for _, tt := range rows {
		t.Run(tt.name, func(t *testing.T) {
			f := newStickyFixture(t)
			f.writeShippedStickyPair()
			require.NoError(t, os.MkdirAll(f.stateDir(), 0o755))

			outside := t.TempDir()
			leafPath := filepath.Join(f.stateDir(), rulecond.StickyStateKey(leafSession))
			tt.plant(t, leafPath, outside)
			before := outsideTree(t, outside)

			stdout, stderr := f.submit(leafSession)

			assert.Empty(t, stdout,
				"a refused counter entry injects nothing")
			assert.Empty(t, stderr,
				"a refused counter entry is whole-run benign, not a reported violation")
			assert.Equal(t, before, outsideTree(t, outside),
				"no file outside the checkout may be created, truncated, or rewritten")
			tt.intact(t, leafPath, outside)
		})
	}
}

// TestStickyFire_HostileCounterEntryDoesNotBlockOtherSessions pins the blast
// radius: one poisoned counter name suppresses that session only, because every
// session writes its own digest-named entry.
func TestStickyFire_HostileCounterEntryDoesNotBlockOtherSessions(t *testing.T) {
	f := newStickyFixture(t)
	f.writeShippedStickyPair()
	require.NoError(t, os.MkdirAll(f.stateDir(), 0o755))

	outside := t.TempDir()
	symlinkOrSkip(t, filepath.Join(outside, "created-by-the-hook"),
		filepath.Join(f.stateDir(), rulecond.StickyStateKey(leafSession)))

	poisoned, stderr := f.submit(leafSession)
	assert.Empty(t, poisoned)
	assert.Empty(t, stderr)

	healthy, stderr := f.submit("S-healthy")
	assert.Empty(t, stderr)
	assert.NotEmpty(t, stickyContext(t, healthy),
		"an unrelated session still injects at its first prompt")
	assert.Equal(t, 1, f.counter("S-healthy"))
	assert.Empty(t, filesUnder(t, outside),
		"the refused session created nothing outside the checkout")
}

// TestStickyFire_EmptySessionIDUsesTheSameGuardedPath covers the reachable
// degenerate name. A payload that omits session_id lands on the digest of the
// empty string, which is a constant every caller can compute, so the leaf guard
// has to hold there too rather than relying on the name being unguessable.
func TestStickyFire_EmptySessionIDUsesTheSameGuardedPath(t *testing.T) {
	f := newStickyFixture(t)
	f.writeShippedStickyPair()
	require.NoError(t, os.MkdirAll(f.stateDir(), 0o755))

	outside := t.TempDir()
	victim := filepath.Join(outside, "victim")
	require.NoError(t, os.WriteFile(victim, []byte("41\n"), 0o644))
	symlinkOrSkip(t, victim, filepath.Join(f.stateDir(), rulecond.StickyStateKey("")))

	stdout, stderr := f.submitRaw(`{"hook_event_name":"UserPromptSubmit"}`, stickyDefaultCadence)

	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
	raw, err := os.ReadFile(victim)
	require.NoError(t, err)
	assert.Equal(t, "41\n", string(raw),
		"the outside file must be byte-identical after the refused write")
}
