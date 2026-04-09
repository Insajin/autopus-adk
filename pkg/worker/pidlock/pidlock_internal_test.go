// Package pidlock tests internal error paths via white-box testing.

//go:build !windows

package pidlock

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAcquire_FlockError verifies that Acquire returns a "pidlock: flock" error
// when the flock call fails. Uses the package-level flockFunc hook.
func TestAcquire_FlockError(t *testing.T) {
	// Not parallel — modifies the package-level flockFunc variable.
	orig := flockFunc
	defer func() { flockFunc = orig }()

	// Override flockFunc to return an error on LOCK_EX attempts.
	flockFunc = func(fd, how int) error {
		if how == lockEX|lockNB {
			return syscall.EWOULDBLOCK
		}
		return orig(fd, how)
	}

	// Given: a fresh lock path with no existing file
	dir := t.TempDir()
	lock := New(dir + "/worker.pid")

	// When: acquiring — flockFunc will fail
	err := lock.Acquire()

	// Then: a flock error is returned
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pidlock: flock")
}
