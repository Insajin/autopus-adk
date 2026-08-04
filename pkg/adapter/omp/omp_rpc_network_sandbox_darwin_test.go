//go:build darwin

package omp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ompRPCSandboxExecutable = "/usr/bin/sandbox-exec"

func TestOMPRPCNetworkSandboxProfile_UsesDenyDefaultAndExactPort(t *testing.T) {
	profile, address, err := ompRPCNetworkSandboxProfile("http://127.0.0.1:43127")
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:43127", address)
	assert.Contains(t, profile, "(deny network*)")
	assert.Contains(t, profile, `(remote ip "localhost:43127")`)
	assert.Equal(t, 1, strings.Count(profile, "(allow network-outbound"))

	cmd := exec.Command("/usr/bin/true", "fixture")
	require.NoError(t, configureOMPRPCNetworkSandbox(cmd, "http://127.0.0.1:43127"))
	assert.Equal(t, ompRPCSandboxExecutable, cmd.Path)
	assert.Equal(t, "/usr/bin/true", cmd.Args[3])
	assert.Equal(t, "fixture", cmd.Args[4])
}

func TestOMPRPCNetworkSandboxProfile_RejectsNonExactEndpoint(t *testing.T) {
	for _, endpoint := range []string{
		"http://192.0.2.1:43127",
		"http://127.0.0.1",
		"https://127.0.0.1:43127",
		"http://user@127.0.0.1:43127",
		"http://127.0.0.1:43127/v1",
	} {
		_, _, err := ompRPCNetworkSandboxProfile(endpoint)
		require.Error(t, err, endpoint)
	}
}

func configureOMPRPCNetworkSandbox(cmd *exec.Cmd, allowedEndpoint string) error {
	profile, _, err := ompRPCNetworkSandboxProfile(allowedEndpoint)
	if err != nil {
		return err
	}
	info, err := os.Lstat(ompRPCSandboxExecutable)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("network sandbox unavailable")
	}
	if cmd == nil || cmd.Path == "" || len(cmd.Args) == 0 {
		return fmt.Errorf("network sandbox command invalid")
	}
	if err := probeOMPRPCSandboxProfile(profile); err != nil {
		return err
	}

	originalPath := cmd.Path
	originalArgs := append([]string(nil), cmd.Args[1:]...)
	cmd.Path = ompRPCSandboxExecutable
	cmd.Args = append([]string{
		ompRPCSandboxExecutable,
		"-p", profile,
		originalPath,
	}, originalArgs...)
	return nil
}

func probeOMPRPCSandboxProfile(profile string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ompRPCSandboxExecutable, "-p", profile, "/usr/bin/true")
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("network sandbox profile probe timeout")
		}
		return fmt.Errorf("network sandbox profile probe failed")
	}
	return nil
}

func probeOMPRPCNetworkSandbox(
	ctx context.Context,
	allowedEndpoint string,
) (evidence ompRPCNetworkSandboxEvidence, resultErr error) {
	_, allowedAddress, err := ompRPCNetworkSandboxProfile(allowedEndpoint)
	if err != nil {
		return evidence, err
	}
	_, portText, err := net.SplitHostPort(allowedAddress)
	if err != nil {
		return evidence, fmt.Errorf("network sandbox endpoint invalid")
	}
	otherLoopback, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return evidence, fmt.Errorf("network sandbox loopback probe unavailable")
	}
	defer func() {
		if closeErr := otherLoopback.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("network sandbox loopback probe close failed: %w", closeErr))
		}
	}()
	nonLoopback, err := listenOMPLocalInterface(0)
	if err != nil {
		return evidence, err
	}
	defer func() {
		if closeErr := nonLoopback.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("network sandbox local-interface probe close failed: %w", closeErr))
		}
	}()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestOMPRPCNetworkSandboxDialHelper$")
	cmd.Env = []string{
		"AUTOPUS_OMP_NETWORK_SANDBOX_HELPER=1",
		"AUTOPUS_OMP_NETWORK_ALLOWED=" + allowedAddress,
		"AUTOPUS_OMP_NETWORK_OTHER_LOOPBACK=" + otherLoopback.Addr().String(),
		"AUTOPUS_OMP_NETWORK_NON_LOOPBACK=" + nonLoopback.Addr().String(),
		"AUTOPUS_OMP_NETWORK_TEST_NET=" + net.JoinHostPort("192.0.2.1", portText),
	}
	if err := configureOMPRPCNetworkSandbox(cmd, allowedEndpoint); err != nil {
		return evidence, err
	}
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return evidence, fmt.Errorf("network sandbox probe timeout")
		}
		return evidence, fmt.Errorf("network sandbox profile or counterexample probe failed")
	}
	return ompRPCNetworkSandboxEvidence{
		AllowedEndpointConnections: 1,
		DeniedOtherLoopback:        1,
		DeniedNonLoopback:          2,
	}, nil
}

func ompRPCNetworkSandboxProfile(endpoint string) (profile, address string, err error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", "", fmt.Errorf("network sandbox endpoint invalid")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", "", fmt.Errorf("network sandbox endpoint invalid")
	}
	host := net.ParseIP(parsed.Hostname())
	portText := parsed.Port()
	port, portErr := strconv.Atoi(portText)
	if host == nil || !host.IsLoopback() || portErr != nil || port < 1 || port > 65535 {
		return "", "", fmt.Errorf("network sandbox endpoint must be exact loopback host and port")
	}
	address = net.JoinHostPort(parsed.Hostname(), portText)
	profile = fmt.Sprintf(`(version 1)
(allow default)
(deny network*)
(allow network-outbound (remote ip "localhost:%d"))
`, port)
	return profile, address, nil
}

func listenOMPLocalInterface(port int) (net.Listener, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("network sandbox local-interface probe unavailable")
	}
	for _, networkInterface := range interfaces {
		if networkInterface.Flags&net.FlagUp == 0 || networkInterface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, addressErr := networkInterface.Addrs()
		if addressErr != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, parseErr := net.ParseCIDR(address.String())
			if parseErr != nil || ip.To4() == nil || ip.IsLoopback() || ip.IsUnspecified() {
				continue
			}
			listener, listenErr := net.ListenTCP("tcp4", &net.TCPAddr{IP: ip, Port: port})
			if listenErr == nil {
				return listener, nil
			}
		}
	}
	return nil, fmt.Errorf("network sandbox local-interface probe unavailable")
}
