//go:build !darwin || !cgo

package run

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecureDesktopSpawn_UnsupportedBuildFailsClosed(t *testing.T) {
	t.Parallel()
	assert.False(t, secureDesktopSpawnSupported())
	_, err := startSecureDesktopProcess(secureDesktopSpawnSpec{})
	assert.ErrorIs(t, err, errDesktopProviderUnavailable)
	assert.ErrorIs(t, secureDesktopReapProcessGroup(123), errDesktopProviderUnavailable)
}
