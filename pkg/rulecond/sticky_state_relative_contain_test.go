package rulecond_test

// SPEC-STICKYRULE-001 write-side containment oracles for RELATIVE in-root
// symlink targets.
//
// The existing containment rows all plant a symlink whose target is an absolute
// path outside the checkout, and os.Root refuses an absolute target outright.
// That refusal is structural and unconditional, so those rows hold even with the
// package's own Lstat guards removed: os.Root alone answers them, and the guards
// are never the thing doing the refusing.
//
// A relative target that resolves inside the handle's own root is the case
// os.Root admits rather than refuses. It never leaves the root, so containment
// has nothing to say about it, and the only thing standing between the runtime
// and a followed link is the explicit Lstat check each layer performs:
//
//	openCounter          in sticky_state.go     — the counter entry itself
//	descendStateComponent in sticky_state_dir.go — each directory component
//
// The rows below are therefore the pins for those two checks. A relative link
// cannot reach outside the checkout, so what it corrupts is bounded — another
// session's counter, or a sibling directory the runtime relocates its whole
// state tree into — but the guard being removed is the same guard that stops the
// absolute case from being reachable through a second name.

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/rulecond"
)

const (
	// relLeafSession is the session whose own counter name carries the link.
	relLeafSession = "S-rel-leaf"
	// relDecoySession owns the in-directory counter the planted link points at.
	relDecoySession = "S-rel-decoy"
	// relComponentSession drives the directory-component rows.
	relComponentSession = "S-rel-component"
	// relocatedDirName is the sibling directory a planted component link points
	// at. It is a bare name, so it resolves inside the parent handle's root.
	relocatedDirName = "relocated"
)

// TestStickyFire_RelativeInRootLeafSymlinkIsRefused pins the openCounter Lstat
// pre-check.
//
// The link sits at the running session's own counter name and points, by bare
// relative name, at another session's counter in the same directory. os.Root
// resolves it happily — the target never leaves the state root — and the
// resolved file is a plain 0600 regular file with one link, so every post-open
// check passes too. Only the Lstat pre-check sees that the name itself is a
// link.
//
// Without that check the invocation reads through the link, treats another
// session's index as its own, and writes the successor back into that session's
// file, which both corrupts the victim's counter and injects at a cadence
// position this session never reached.
func TestStickyFire_RelativeInRootLeafSymlinkIsRefused(t *testing.T) {
	f := newStickyFixture(t)
	f.writeShippedStickyPair()
	require.NoError(t, os.MkdirAll(f.stateDir(), 0o755))

	// A fresh counter: reading it as this session's own state yields index 1,
	// which is an injection point, so a followed link is observable on stdout as
	// well as on disk.
	decoyName := rulecond.StickyStateKey(relDecoySession)
	decoy := filepath.Join(f.stateDir(), decoyName)
	require.NoError(t, os.WriteFile(decoy, []byte("0\n"), 0o600))

	leaf := filepath.Join(f.stateDir(), rulecond.StickyStateKey(relLeafSession))
	symlinkOrSkip(t, decoyName, leaf)

	stdout, stderr := f.submit(relLeafSession)

	assert.Empty(t, stdout,
		"a refused counter entry injects nothing, even when the link target reads as a fresh counter")
	assert.Empty(t, stderr,
		"a refused counter entry is whole-run benign, not a reported violation")

	raw, err := os.ReadFile(decoy)
	require.NoError(t, err)
	assert.Equal(t, "0\n", string(raw),
		"another session's counter must be byte-identical: no session may reach it through its own name")
	stillSymlink(t, leaf, "")
}

// TestStickyFire_RelativeInRootStateComponentIsRefused pins the
// descendStateComponent Lstat check at every component of the state path.
//
// Each row replaces one component with a link to a sibling directory by bare
// relative name, so the target stays inside the root the descent already holds a
// handle on and os.Root admits it. Without the Lstat check the descent opens the
// link, and the entire counter tree — every write and the retention sweep that
// follows it — relocates into the sibling.
func TestStickyFire_RelativeInRootStateComponentIsRefused(t *testing.T) {
	stateDir := filepath.ToSlash(rulecond.StickyStateDirRelPath)
	rows := []struct {
		name string
		rel  string
	}{
		{name: "state directory itself", rel: stateDir},
		{name: "runtime parent", rel: path.Dir(stateDir)},
		{name: "autopus parent", rel: path.Dir(path.Dir(stateDir))},
	}

	for _, tt := range rows {
		t.Run(tt.name, func(t *testing.T) {
			f := newStickyFixture(t)
			f.writeShippedStickyPair()

			link := filepath.Join(f.root, filepath.FromSlash(tt.rel))
			require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o755))
			relocated := filepath.Join(filepath.Dir(link), relocatedDirName)
			require.NoError(t, os.MkdirAll(relocated, 0o755))
			symlinkOrSkip(t, relocatedDirName, link)

			stdout, stderr := f.submit(relComponentSession)

			assert.Empty(t, stdout,
				"a refused state component injects nothing")
			assert.Empty(t, stderr,
				"an unusable state directory is whole-run benign, not a reported violation")
			assert.Empty(t, filesUnder(t, relocated),
				"no counter may be created inside the directory the planted link points at")
			stillSymlink(t, link, "")
		})
	}
}
