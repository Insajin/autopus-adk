package rulecond_test

// SPEC-STICKYRULE-001 write-side containment oracles.
//
// The sticky read path already refuses an in-project symlink through the
// trusted-root contract. The state path is the other half: it is the only
// contained location this runtime creates, writes, and sweeps. A cloned
// repository can commit `.autopus`, `.autopus/runtime`, or `sticky-rules` itself
// as a symlink, because git mode 120000 survives a clone, and an uncontained
// join would then put counter files inside the link target and point the
// pruneStickyState retention loop at whatever lives there. Real trees hold
// exactly the names that loop deletes: `.git/lfs/objects` and container blob
// stores are directories of 64-character hex filenames.

import (
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/rulecond"
)

// stateComponentLinks enumerates the in-project components leading to the
// counter tree. Each one is a place a repository can plant a symlink, and each
// must be refused, not just the last.
func stateComponentLinks() []struct {
	name string
	rel  string
} {
	stateDir := filepath.ToSlash(rulecond.StickyStateDirRelPath)
	return []struct {
		name string
		rel  string
	}{
		{name: "state directory itself", rel: stateDir},
		{name: "runtime parent", rel: path.Dir(stateDir)},
		{name: "autopus parent", rel: path.Dir(path.Dir(stateDir))},
	}
}

// TestStickyFire_SymlinkedStateComponentIsRefused is the write-side containment
// oracle. A relocated state frame must produce no write and no deletion inside
// the link target, and must stay the whole-run benign "unusable state directory"
// case of REQ-STICKYRULE-FIRE-08 rather than becoming observable output.
func TestStickyFire_SymlinkedStateComponentIsRefused(t *testing.T) {
	for _, tt := range stateComponentLinks() {
		t.Run(tt.name, func(t *testing.T) {
			f := newStickyFixture(t)
			f.writeShippedStickyPair()

			// The link target holds the sub-path the relocated frame would
			// resolve through, plus one aged 64-hex file: precisely what the
			// retention sweep deletes.
			outside := t.TempDir()
			suffix := strings.Trim(strings.TrimPrefix(
				filepath.ToSlash(rulecond.StickyStateDirRelPath), tt.rel), "/")
			victimDir := filepath.Join(outside, filepath.FromSlash(suffix))
			require.NoError(t, os.MkdirAll(victimDir, 0o755))
			victim := seedStateEntry(t, victimDir, "victim", time.Now().Add(-30*24*time.Hour))
			before := filesUnder(t, outside)

			link := filepath.Join(f.root, filepath.FromSlash(tt.rel))
			require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o755))
			symlinkOrSkip(t, outside, link)

			stdout, stderr := f.submit("S-symstate")

			assert.Empty(t, stdout,
				"a refused state directory injects nothing")
			assert.Empty(t, stderr,
				"an unusable state directory is whole-run benign, not a reported violation")
			assert.True(t, stateEntryExists(t, victimDir, victim),
				"an aged 64-hex file inside the link target must not be swept")
			assert.Equal(t, before, filesUnder(t, outside),
				"no counter may be created and no entry removed inside the link target")
		})
	}
}

// TestStickyFire_RegularStateDirectoryStillCounts is the control row: the
// containment frame must not change what a plain checkout does.
func TestStickyFire_RegularStateDirectoryStillCounts(t *testing.T) {
	f := newStickyFixture(t)
	f.writeShippedStickyPair()

	stdout, stderr := f.submit("S-plain")
	assert.NotEmpty(t, stickyContext(t, stdout), "prompt index 1 is an injection point")
	assert.Empty(t, stderr)
	assert.Equal(t, 1, f.counter("S-plain"))

	_, stderr = f.submit("S-plain")
	assert.Empty(t, stderr)
	assert.Equal(t, 2, f.counter("S-plain"), "the counter advances across invocations")

	info, err := os.Lstat(f.stateDir())
	require.NoError(t, err)
	assert.True(t, info.Mode().IsDir(),
		"the state directory is created as a real directory, never as a link")
}

// TestStickyFire_SymlinkedStateEntryIsNeitherFollowedNorRemoved covers a digest-
// named symlink planted inside an otherwise valid state directory. The sweep is
// driven past its entry cap so oldest-first removal actually runs, and the
// planted link is the oldest candidate: without the regular-file guard it is the
// first thing removed.
//
// This row reaches the retention sweep only. The planted key belongs to a
// session that never submits, so the write path never looks at it — which is
// exactly how a leaf-write escape survived here. The write path is covered by
// sticky_state_leaf_contain_test.go, where the planted entry sits at the running
// session's own key.
func TestStickyFire_SymlinkedStateEntryIsNeitherFollowedNorRemoved(t *testing.T) {
	f := newStickyFixture(t)
	f.writeShippedStickyPair()
	require.NoError(t, os.MkdirAll(f.stateDir(), 0o755))

	outside := t.TempDir()
	target := filepath.Join(outside, "payload")
	require.NoError(t, os.WriteFile(target, []byte("7\n"), 0o644))

	link := filepath.Join(f.stateDir(), rulecond.StickyStateKey("planted"))
	symlinkOrSkip(t, target, link)

	// Every real entry is stamped later than the link, so the link sorts oldest.
	now := time.Now()
	seeded := make([]string, 0, 205)
	for i := 0; i < 205; i++ {
		modTime := now.Add(time.Duration(i+1) * time.Minute)
		seeded = append(seeded, seedStateEntry(t, f.stateDir(), "kept-"+strconv.Itoa(i), modTime))
	}

	_, stderr := f.submit("S-entry")
	assert.Empty(t, stderr, "pruning is silent")

	linkInfo, err := os.Lstat(link)
	require.NoError(t, err, "the planted entry must survive the sweep")
	assert.NotZero(t, linkInfo.Mode()&os.ModeSymlink,
		"the planted entry is skipped, not replaced or resolved")
	assert.FileExists(t, target, "the link target must never be removed")

	assert.True(t, stateEntryExists(t, f.stateDir(), seeded[len(seeded)-1]),
		"the newest real entry survives")
	assert.False(t, stateEntryExists(t, f.stateDir(), seeded[0]),
		"the cap still removes the oldest real entries, so the sweep genuinely ran")
}
