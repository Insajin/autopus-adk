package cli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupPlaywrightTempDirPreservesRunAndCleanupErrors(t *testing.T) {
	t.Parallel()

	runErr := errors.New("playwright failed")
	cleanupErr := errors.New("remove failed")
	err := cleanupPlaywrightTempDir("/tmp/report", runErr, func(path string) error {
		assert.Equal(t, "/tmp/report", path)
		return cleanupErr
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, runErr)
	assert.ErrorIs(t, err, cleanupErr)
}

func TestCleanupPlaywrightTempDirReturnsCleanupErrorAfterSuccessfulRun(t *testing.T) {
	t.Parallel()

	cleanupErr := errors.New("remove failed")
	err := cleanupPlaywrightTempDir("/tmp/report", nil, func(string) error { return cleanupErr })

	assert.ErrorIs(t, err, cleanupErr)
}
