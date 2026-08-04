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

const workflowContextLiveSandboxExecutable = "/usr/bin/sandbox-exec"

func workflowContextLiveSandboxRequired() bool { return true }

func configureWorkflowContextLiveSandbox(cmd *exec.Cmd, endpoint string) (bool, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.Path != "" ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return false, errors.New("sandbox-endpoint-invalid")
	}
	ip := net.ParseIP(parsed.Hostname())
	port, portErr := strconv.Atoi(parsed.Port())
	if ip == nil || !ip.IsLoopback() || portErr != nil || port < 1 || port > 65535 {
		return false, errors.New("sandbox-endpoint-not-loopback")
	}
	info, err := os.Lstat(workflowContextLiveSandboxExecutable)
	if err != nil || !info.Mode().IsRegular() || cmd == nil || cmd.Path == "" || len(cmd.Args) == 0 {
		return false, errors.New("sandbox-unavailable")
	}
	profile := fmt.Sprintf(`(version 1)
(allow default)
(deny network*)
(allow network-outbound (remote ip "localhost:%d"))
`, port)
	originalPath := cmd.Path
	originalArgs := append([]string(nil), cmd.Args[1:]...)
	cmd.Path = workflowContextLiveSandboxExecutable
	cmd.Args = append([]string{workflowContextLiveSandboxExecutable, "-p", profile, originalPath}, originalArgs...)
	return true, nil
}
