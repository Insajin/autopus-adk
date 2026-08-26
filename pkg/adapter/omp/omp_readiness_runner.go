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
	workingDirectory   string
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
	commandArgs := append(append([]string(nil), runner.prefixArgs...), args...)
	cmd := exec.CommandContext(ctx, path, commandArgs...)
	if dir == "" {
		dir = runner.workingDirectory
	}
	cmd.Dir = dir
	cmd.Env = mergeOMPProbeEnvironment(os.Environ(), runner.environment)
	return cmd, nil
}

func configureOMPProbeRunner(
	runner commandOMPProbeRunner,
	executable, sandbox, root string,
) (commandOMPProbeRunner, string) {
	resolved, err := resolveOMPProbeExecutable(executable)
	runner.resolvedExecutable, runner.resolveErr = resolved, err
	runner.environment = ompProbeTaskEnvironment(sandbox)
	runner.workingDirectory = root
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

func ompProbeTaskEnvironment(sandbox string) []string {
	return []string{
		"HOME=" + filepath.Join(sandbox, "home"),
		"XDG_CONFIG_HOME=" + filepath.Join(sandbox, "xdg-config"),
		"XDG_CACHE_HOME=" + filepath.Join(sandbox, "xdg-cache"),
		"XDG_DATA_HOME=" + filepath.Join(sandbox, "xdg-data"),
		"XDG_STATE_HOME=" + filepath.Join(sandbox, "xdg-state"),
		"TMPDIR=" + filepath.Join(sandbox, "tmp"),
		"PI_CODING_AGENT_DIR=" + filepath.Join(sandbox, "pi-agent"),
		"NO_PROXY=127.0.0.1,localhost,::1",
		"no_proxy=127.0.0.1,localhost,::1",
		"HTTP_PROXY=http://127.0.0.1:1",
		"HTTPS_PROXY=http://127.0.0.1:1",
		"ALL_PROXY=http://127.0.0.1:1",
	}
}
