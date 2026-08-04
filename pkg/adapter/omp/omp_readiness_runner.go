package omp

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/processprobe"
)

type commandOMPProbeRunner struct {
	maxOutput          int
	resolvedExecutable string
	resolveErr         error
	environment        []string
	prefixArgs         []string
}

type ompProbeInvocation struct {
	args []string
	env  []string
}

func (runner commandOMPProbeRunner) Run(ctx context.Context, executable string, args ...string) ([]byte, error) {
	return runner.run(ctx, executable, "", nil, args...)
}

func (runner commandOMPProbeRunner) run(
	ctx context.Context,
	executable, dir string,
	stdin []byte,
	args ...string,
) ([]byte, error) {
	cmd, err := runner.command(ctx, executable, dir, args...)
	if err != nil {
		return nil, err
	}
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	return processprobe.OutputLimited(cmd, runner.maxOutput)
}

func (runner commandOMPProbeRunner) runRPC(
	ctx context.Context,
	executable, dir string,
	stdin []byte,
	args ...string,
) ([]byte, error) {
	cmd, err := runner.command(ctx, executable, dir, args...)
	if err != nil {
		return nil, err
	}
	return runOMPReadinessRPCCommand(ctx, cmd, stdin, runner.maxOutput)
}

func (runner commandOMPProbeRunner) command(
	ctx context.Context,
	executable, dir string,
	args ...string,
) (*exec.Cmd, error) {
	path := runner.resolvedExecutable
	if runner.resolveErr != nil {
		return nil, runner.resolveErr
	}
	if path == "" {
		var err error
		path, err = resolveOMPProbeExecutable(executable)
		if err != nil {
			return nil, err
		}
	}
	invocation := prepareOMPProbeInvocation(args)
	commandArgs := append(append([]string(nil), runner.prefixArgs...), invocation.args...)
	cmd := exec.CommandContext(ctx, path, commandArgs...)
	cmd.Dir = dir
	overrides := append(append([]string(nil), invocation.env...), runner.environment...)
	cmd.Env = mergeOMPProbeEnvironment(os.Environ(), overrides)
	return cmd, nil
}

func configureOMPProbeRunner(
	runner commandOMPProbeRunner,
	executable, overlay string,
) (commandOMPProbeRunner, string) {
	resolved, err := resolveOMPProbeExecutable(executable)
	runner.resolvedExecutable, runner.resolveErr = resolved, err
	runner.environment = ompProbeTaskEnvironment(overlay)
	return runner, resolved
}

func resolveOMPProbeExecutable(executable string) (string, error) {
	path, err := exec.LookPath(executable)
	if err != nil {
		return "", err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

func prepareOMPProbeInvocation(args []string) ompProbeInvocation {
	invocation := ompProbeInvocation{args: append([]string(nil), args...)}
	if profileDir := ompProbeProfileDir(args); profileDir != "" {
		invocation.env = append(invocation.env, "PI_CODING_AGENT_DIR="+profileDir)
	}
	if index, overlay, ok := ompConfigReadbackOverlay(args); ok {
		invocation.args = append(append([]string(nil), args[:index]...), args[index+2:]...)
		invocation.env = append(invocation.env, "PI_CONFIG_FILES="+overlay)
	}
	return invocation
}

func ompConfigReadbackOverlay(args []string) (int, string, bool) {
	for index := 0; index+1 < len(args); index++ {
		if args[index] != "--config" {
			continue
		}
		remaining := append(append([]string(nil), args[:index]...), args[index+2:]...)
		if len(remaining) >= 2 && remaining[0] == "config" && remaining[1] == "get" {
			return index, args[index+1], true
		}
	}
	return 0, "", false
}

func mergeOMPProbeEnvironment(base, overrides []string) []string {
	values := make(map[string]string, len(overrides))
	order := make([]string, 0, len(overrides))
	for _, value := range overrides {
		if key, _, found := strings.Cut(value, "="); found {
			if _, exists := values[key]; !exists {
				order = append(order, key)
			}
			values[key] = value
		}
	}
	merged := make([]string, 0, len(base)+len(overrides))
	for _, value := range base {
		key, _, found := strings.Cut(value, "=")
		if found && allowedOMPProbeBaseEnvironment(key) && values[key] == "" {
			merged = append(merged, value)
		}
	}
	for _, key := range order {
		merged = append(merged, values[key])
	}
	return merged
}

func allowedOMPProbeBaseEnvironment(key string) bool {
	return key == "PATH" || key == "LANG" || key == "TZ" || key == "SYSTEMROOT" ||
		key == "WINDIR" || key == "PATHEXT" || strings.HasPrefix(key, "LC_")
}

func ompProbeTaskEnvironment(overlay string) []string {
	base := filepath.Dir(overlay)
	return []string{
		"HOME=" + filepath.Join(base, "home"),
		"XDG_CONFIG_HOME=" + filepath.Join(base, "xdg-config"),
		"XDG_CACHE_HOME=" + filepath.Join(base, "xdg-cache"),
		"XDG_DATA_HOME=" + filepath.Join(base, "xdg-data"),
		"XDG_STATE_HOME=" + filepath.Join(base, "xdg-state"),
		"TMPDIR=" + filepath.Join(base, "tmp"),
		"PI_CODING_AGENT_DIR=" + filepath.Join(base, "pi-agent"),
		"PI_CONFIG_FILES=" + overlay,
		"NO_PROXY=127.0.0.1,localhost,::1",
		"no_proxy=127.0.0.1,localhost,::1",
		"HTTP_PROXY=http://127.0.0.1:1",
		"HTTPS_PROXY=http://127.0.0.1:1",
		"ALL_PROXY=http://127.0.0.1:1",
	}
}

func ompProbeProfileDir(args []string) string {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == "--config" {
			return filepath.Dir(args[index+1])
		}
	}
	return ""
}
