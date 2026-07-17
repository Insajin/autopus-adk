//go:build windows

package selfupdate

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestIsCrossDeviceError_WindowsNotSameDevice(t *testing.T) {
	t.Parallel()

	err := &os.LinkError{Op: "rename", Err: windows.ERROR_NOT_SAME_DEVICE}
	require.True(t, isCrossDeviceError(err))
}
