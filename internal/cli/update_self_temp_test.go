package cli

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/selfupdate"
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

func TestInstallSelfUpdateWrappers_RejectMissingReleaseAssetsWithoutNetwork(t *testing.T) {
	pathInfo := binaryPathInfo{
		ExecutablePath: filepath.Join(t.TempDir(), "auto"),
	}
	info := &selfupdate.ReleaseInfo{TagName: "v0.50.91"}
	cmd := &cobra.Command{Use: "update"}
	wrappers := []struct {
		name string
		run  func(*cobra.Command, string, *selfupdate.ReleaseInfo, binaryPathInfo) error
	}{
		{name: "direct self update", run: installSelfUpdateRelease},
		{name: "freshness update", run: installFreshUpdateRelease},
	}

	for _, wrapper := range wrappers {
		t.Run(wrapper.name, func(t *testing.T) {
			err := wrapper.run(cmd, "0.50.90", info, pathInfo)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "다운로드 URL")
		})
	}
}
