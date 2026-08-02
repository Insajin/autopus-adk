package rulecond_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/rulecond"
)

const (
	// stickyStateMaxEntries and stickyStateMaxAge are the REQ-STICKYRULE-STATE-01
	// retention bounds.
	stickyStateMaxEntries = 200
	stickyStateMaxAge     = 7 * 24 * time.Hour
)

// seedStateEntry writes one counter file with an explicit modification time and
// returns its filename.
func seedStateEntry(t *testing.T, dir, label string, modTime time.Time) string {
	t.Helper()
	name := rulecond.StickyStateKey(label)
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte("3\n"), 0o644))
	require.NoError(t, os.Chtimes(path, modTime, modTime))
	return name
}

func stateEntryExists(t *testing.T, dir, name string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

// TestStickyFire_StateStaysBounded is the S9 oracle for REQ-STICKYRULE-STATE-01
// and INV-107. The directory starts at 250 entries, 40 of them 8 days old.
func TestStickyFire_StateStaysBounded(t *testing.T) {
	f := newStickyFixture(t)
	f.writeShippedStickyPair()
	require.NoError(t, os.MkdirAll(f.stateDir(), 0o755))

	now := time.Now()

	// 40 entries past the 7-day retention bound.
	aged := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		modTime := now.Add(-stickyStateMaxAge - time.Duration(24+i)*time.Hour)
		aged = append(aged, seedStateEntry(t, f.stateDir(), fmt.Sprintf("aged-%d", i), modTime))
	}

	// 210 in-retention entries, oldest first, each a minute apart.
	recent := make([]string, 0, 210)
	for i := 0; i < 210; i++ {
		modTime := now.Add(-time.Duration(210-i) * time.Minute)
		recent = append(recent, seedStateEntry(t, f.stateDir(), fmt.Sprintf("recent-%d", i), modTime))
	}
	require.Len(t, filesUnder(t, f.stateDir()), 250, "the fixture starts at 250 entries")

	_, stderr := f.submit("S-current")
	assert.Empty(t, stderr, "pruning is silent")

	for i, name := range aged {
		assert.False(t, stateEntryExists(t, f.stateDir(), name),
			"aged-%d is older than 7 days and must be deleted", i)
	}

	assert.True(t, stateEntryExists(t, f.stateDir(), rulecond.StickyStateKey("S-current")),
		"the current session entry must survive its own write")
	assert.Equal(t, 1, f.counter("S-current"))

	remaining := filesUnder(t, f.stateDir())
	assert.LessOrEqual(t, len(remaining), stickyStateMaxEntries,
		"the state directory is capped at %d entries", stickyStateMaxEntries)

	// Oldest-first removal: survival must be monotonic in modification time, so
	// once a recent entry survives every newer entry survives too.
	firstSurvivor := -1
	for i, name := range recent {
		alive := stateEntryExists(t, f.stateDir(), name)
		if alive && firstSurvivor < 0 {
			firstSurvivor = i
		}
		if firstSurvivor >= 0 {
			assert.True(t, alive,
				"recent-%d is newer than the oldest survivor recent-%d and must not be removed",
				i, firstSurvivor)
		}
	}
	require.GreaterOrEqual(t, firstSurvivor, 0, "in-retention entries must not all be removed")
	assert.Positive(t, firstSurvivor,
		"250 seeded entries plus a new one exceed the cap, so the oldest in-retention entries are removed")
	assert.True(t, stateEntryExists(t, f.stateDir(), recent[len(recent)-1]),
		"the newest in-retention entry must survive")
}

// TestStickyFire_UnusableStateDirectoryIsBenign is the REQ-STICKYRULE-FIRE-08
// whole-run benign row for state: no stdout, no stderr, no error.
func TestStickyFire_UnusableStateDirectoryIsBenign(t *testing.T) {
	f := newStickyFixture(t)
	f.writeShippedStickyPair()

	// A regular file where the state directory belongs makes every state
	// operation fail. That is benign absence, not a violation.
	require.NoError(t, os.MkdirAll(filepath.Dir(f.stateDir()), 0o755))
	require.NoError(t, os.WriteFile(f.stateDir(), []byte("not a directory"), 0o644))

	stdout, stderr := f.submit("S-alpha")

	assert.Empty(t, stdout, "an unusable state directory writes no stdout")
	assert.Empty(t, stderr, "an unusable state directory writes no stderr")
}
