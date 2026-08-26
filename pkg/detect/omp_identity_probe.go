package detect

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/processprobe"
)

const (
	ompIdentityTimeout   = 5 * time.Second
	ompIdentityWaitDelay = 250 * time.Millisecond
	ompIdentityMaxOutput = 64 * 1024
)

// ProbeOMPIdentity executes only the bounded metadata-only version command in a
// private directory with a credential-free environment. It returns the exact
// release identity on success and never degrades an execution error to a match.
// @AX:ANCHOR [AUTO]: preserve this bounded metadata-only OMP identity boundary.
// @AX:REASON [AUTO]: platform detection, installed-platform discovery, and the OMP adapter depend on exact release output without inheriting user credentials or configuration.
func ProbeOMPIdentity(parent context.Context, executable string) (string, bool) {
	path, err := exec.LookPath(executable)
	if err != nil {
		return "", false
	}
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, ompIdentityTimeout)
	defer cancel()

	sandbox, err := os.MkdirTemp("", "autopus-omp-identity-")
	if err != nil {
		return "", false
	}
	defer func() { _ = os.RemoveAll(sandbox) }()
	for _, name := range []string{"home", "tmp", "config", "data", "state", "cache"} {
		if err := os.Mkdir(filepath.Join(sandbox, name), 0o700); err != nil {
			return "", false
		}
	}

	cmd := exec.CommandContext(ctx, path, "--version")
	cmd.Dir = sandbox
	cmd.WaitDelay = ompIdentityWaitDelay
	cmd.Env = ompIdentityEnvironment(sandbox)
	output, err := processprobe.OutputLimited(cmd, ompIdentityMaxOutput)
	if err != nil {
		return "", false
	}
	version := strings.TrimSpace(string(output))
	if !OMPVersionMatchesIdentity(version) {
		return "", false
	}
	return version, true
}

func ompIdentityEnvironment(sandbox string) []string {
	return []string{
		"HOME=" + filepath.Join(sandbox, "home"),
		"TMPDIR=" + filepath.Join(sandbox, "tmp"),
		"XDG_CONFIG_HOME=" + filepath.Join(sandbox, "config"),
		"XDG_DATA_HOME=" + filepath.Join(sandbox, "data"),
		"XDG_STATE_HOME=" + filepath.Join(sandbox, "state"),
		"XDG_CACHE_HOME=" + filepath.Join(sandbox, "cache"),
		"PATH=/usr/bin:/bin",
		"LANG=C",
		"LC_ALL=C",
	}
}
