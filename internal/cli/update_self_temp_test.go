package cli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithSelfUpdateTempDir_DoesNotRunAfterCreateFailure(t *testing.T) {
	sentinel := errors.New("injected temp directory failure")
	originalMkdirTemp := makeSelfUpdateTempDir
	t.Cleanup(func() { makeSelfUpdateTempDir = originalMkdirTemp })
	makeSelfUpdateTempDir = func(string, string) (string, error) {
		return "", sentinel
	}
	called := false

	err := withSelfUpdateTempDir(func(string) error {
		called = true
		return nil
	})

	require.ErrorIs(t, err, sentinel)
	require.False(t, called)
}
