//go:build !darwin

package cli

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigureWorkflowContextManagedRPCSandbox_UnsupportedPlatformFailsClosed(t *testing.T) {
	cmd := exec.Command(os.Args[0])
	sandboxed, err := configureWorkflowContextManagedRPCSandbox(cmd, "http://127.0.0.1:43123")
	require.False(t, sandboxed)
	require.ErrorContains(t, err, "managed OMP network sandbox is unsupported")
}
