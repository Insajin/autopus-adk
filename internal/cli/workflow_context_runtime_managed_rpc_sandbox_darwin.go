//go:build darwin

package cli

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strconv"
)

const workflowContextManagedRPCSandboxExecutable = "/usr/bin/sandbox-exec"

// @AX:WARN [AUTO]: Darwin sandbox configuration has more than 15 manual cyclomatic decision points.
// @AX:REASON [AUTO]: URL shape, exact loopback endpoint, port range, sandbox identity, and command integrity checks gate network isolation.
func configureWorkflowContextManagedRPCSandbox(cmd *exec.Cmd, endpoint string) (bool, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false, errors.New("managed OMP sandbox endpoint is invalid")
	}
	ip := net.ParseIP(parsed.Hostname())
	port, portErr := strconv.Atoi(parsed.Port())
	if ip == nil || !ip.IsLoopback() || portErr != nil || port < 1 || port > 65535 {
		return false, errors.New("managed OMP sandbox endpoint is not exact loopback")
	}
	info, err := os.Lstat(workflowContextManagedRPCSandboxExecutable)
	if err != nil || !info.Mode().IsRegular() || cmd == nil || cmd.Path == "" || len(cmd.Args) == 0 {
		return false, errors.New("managed OMP network sandbox is unavailable")
	}
	profile := fmt.Sprintf(`(version 1)
(allow default)
(deny network*)
(allow network-outbound (remote ip "localhost:%d"))
`, port)
	originalPath := cmd.Path
	originalArgs := append([]string(nil), cmd.Args[1:]...)
	cmd.Path = workflowContextManagedRPCSandboxExecutable
	cmd.Args = append([]string{
		workflowContextManagedRPCSandboxExecutable, "-p", profile, originalPath,
	}, originalArgs...)
	return true, nil
}
