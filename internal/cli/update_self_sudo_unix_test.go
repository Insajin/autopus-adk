//go:build !windows

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/selfupdate"
)

func TestInstallSelfUpdateReleaseWith_NonWritableFailsClosedWithoutDownload(t *testing.T) {
	readOnlyDir := t.TempDir()
	require.NoError(t, os.Chmod(readOnlyDir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(readOnlyDir, 0o700) })
	pathInfo := binaryPathInfo{
		ExecutablePath: filepath.Join(readOnlyDir, "auto"),
	}

	err := installSelfUpdateReleaseWith(
		&cobra.Command{Use: "update"},
		"0.50.90",
		&selfupdate.ReleaseInfo{TagName: "v0.50.91"},
		pathInfo,
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "privileged-update-required")
	require.Contains(t, err.Error(), "패키지 관리자")
}
