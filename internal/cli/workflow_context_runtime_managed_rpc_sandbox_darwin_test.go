//go:build darwin

package cli

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigureWorkflowContextManagedRPCSandbox_AllowsExactIPv4LoopbackEndpoint(t *testing.T) {
	cmd := exec.Command("/usr/bin/true", "sentinel")
	sandboxed, err := configureWorkflowContextManagedRPCSandbox(cmd, "http://127.0.0.1:43123")
	require.NoError(t, err)
	require.True(t, sandboxed)
	require.Equal(t, workflowContextManagedRPCSandboxExecutable, cmd.Path)
	require.Equal(t, []string{
		workflowContextManagedRPCSandboxExecutable,
		"-p",
		`(version 1)
(allow default)
(deny network*)
(allow network-outbound (remote ip "localhost:43123"))
`,
		"/usr/bin/true",
		"sentinel",
	}, cmd.Args)
}

func TestConfigureWorkflowContextManagedRPCSandbox_RejectsOtherLoopbackAddresses(t *testing.T) {
	for _, endpoint := range []string{
		"http://127.0.0.2:43123",
		"http://[::1]:43123",
		"http://[::ffff:127.0.0.1]:43123",
	} {
		t.Run(endpoint, func(t *testing.T) {
			cmd := exec.Command("/usr/bin/true", "sentinel")
			originalPath := cmd.Path
			originalArgs := append([]string(nil), cmd.Args...)

			sandboxed, err := configureWorkflowContextManagedRPCSandbox(cmd, endpoint)
			require.False(t, sandboxed)
			require.ErrorContains(t, err, "not exact loopback")
			require.Equal(t, originalPath, cmd.Path)
			require.Equal(t, originalArgs, cmd.Args)
		})
	}
}
