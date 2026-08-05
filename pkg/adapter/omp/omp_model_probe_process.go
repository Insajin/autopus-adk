package omp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/processprobe"
)

// OMPModelProbeProcess pins one executable identity and runs metadata-only
// probes with bounded output. The default constructor uses an empty temporary
// profile; the installed-profile constructor passes only the current user's
// canonical HOME and PI_CODING_AGENT_DIR locator.
type OMPModelProbeProcess struct {
	executable string
	identity   os.FileInfo
	maxOutput  int
	home       string
	profile    string
}

func NewOMPModelProbeProcess(executable string, maxOutput int) (*OMPModelProbeProcess, error) {
	return newOMPModelProbeProcess(executable, maxOutput, "", "")
}

// NewOMPInstalledModelProbeProcess pins OMP while allowing metadata discovery
// from the installed user profile. No ambient environment other than HOME and
// the canonical PI_CODING_AGENT_DIR locator is forwarded to the child.
func NewOMPInstalledModelProbeProcess(executable string, maxOutput int) (*OMPModelProbeProcess, error) {
	home, err := safeOMPModelProbeDirectory(os.Getenv("HOME"), "HOME")
	if err != nil {
		return nil, err
	}
	profile := ""
	if locator := os.Getenv("PI_CODING_AGENT_DIR"); locator != "" {
		profile, err = safeOMPModelProbeDirectory(locator, "PI_CODING_AGENT_DIR")
		if err != nil {
			return nil, err
		}
	}
	return newOMPModelProbeProcess(executable, maxOutput, home, profile)
}

func newOMPModelProbeProcess(executable string, maxOutput int, home, profile string) (*OMPModelProbeProcess, error) {
	if executable == "" || maxOutput <= 0 {
		return nil, fmt.Errorf("OMP model probe process inputs are invalid")
	}
	path, err := exec.LookPath(executable)
	if err != nil {
		return nil, fmt.Errorf("resolve OMP model probe executable: %w", err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return nil, fmt.Errorf("canonicalize OMP model probe executable: %w", err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("absolutize OMP model probe executable: %w", err)
	}
	identity, err := os.Lstat(path)
	if err != nil || !identity.Mode().IsRegular() || identity.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("OMP model probe executable identity is invalid")
	}
	return &OMPModelProbeProcess{
		executable: path, identity: identity, maxOutput: maxOutput, home: home, profile: profile,
	}, nil
}

func safeOMPModelProbeDirectory(path, name string) (string, error) {
	if path == "" || !filepath.IsAbs(path) || strings.ContainsAny(path, "\x00\r\n") {
		return "", fmt.Errorf("OMP model probe %s locator is invalid", name)
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("canonicalize OMP model probe %s locator: %w", name, err)
	}
	info, err := os.Lstat(canonical)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("OMP model probe %s locator is invalid", name)
	}
	return canonical, nil
}

func (process *OMPModelProbeProcess) Run(ctx context.Context, args ...string) ([]byte, error) {
	if process == nil || process.executable == "" || process.identity == nil {
		return nil, fmt.Errorf("OMP model probe executable is not pinned")
	}
	current, err := os.Lstat(process.executable)
	if err != nil || !os.SameFile(process.identity, current) || !current.Mode().IsRegular() {
		return nil, fmt.Errorf("OMP model probe executable identity changed")
	}
	sandbox, err := os.MkdirTemp("", "autopus-omp-model-probe-*")
	if err != nil {
		return nil, fmt.Errorf("create OMP model probe sandbox: %w", err)
	}
	defer func() { _ = os.RemoveAll(sandbox) }()
	for _, name := range []string{"home", "tmp", "config", "data", "state", "cache"} {
		if err := os.Mkdir(filepath.Join(sandbox, name), 0o700); err != nil {
			return nil, fmt.Errorf("prepare OMP model probe sandbox: %w", err)
		}
	}
	command := exec.CommandContext(ctx, process.executable, args...)
	command.Dir = sandbox
	command.WaitDelay = time.Second
	home := filepath.Join(sandbox, "home")
	if process.home != "" {
		home = process.home
	}
	command.Env = []string{
		"HOME=" + home,
		"TMPDIR=" + filepath.Join(sandbox, "tmp"),
		"XDG_CONFIG_HOME=" + filepath.Join(sandbox, "config"),
		"XDG_DATA_HOME=" + filepath.Join(sandbox, "data"),
		"XDG_STATE_HOME=" + filepath.Join(sandbox, "state"),
		"XDG_CACHE_HOME=" + filepath.Join(sandbox, "cache"),
		"LANG=C", "LC_ALL=C",
	}
	if process.profile != "" {
		command.Env = append(command.Env, "PI_CODING_AGENT_DIR="+process.profile)
	}
	return processprobe.OutputLimited(command, process.maxOutput)
}
