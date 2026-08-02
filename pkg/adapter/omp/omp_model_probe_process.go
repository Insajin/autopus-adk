package omp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/insajin/autopus-adk/pkg/processprobe"
)

// OMPModelProbeProcess pins one executable identity and runs metadata-only
// probes inside a credential-free temporary home with bounded output.
type OMPModelProbeProcess struct {
	executable string
	identity   os.FileInfo
	maxOutput  int
}

func NewOMPModelProbeProcess(executable string, maxOutput int) (*OMPModelProbeProcess, error) {
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
	return &OMPModelProbeProcess{executable: path, identity: identity, maxOutput: maxOutput}, nil
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
	for _, name := range []string{"home", "tmp", "config", "data", "cache"} {
		if err := os.Mkdir(filepath.Join(sandbox, name), 0o700); err != nil {
			return nil, fmt.Errorf("prepare OMP model probe sandbox: %w", err)
		}
	}
	command := exec.CommandContext(ctx, process.executable, args...)
	command.Dir = sandbox
	command.WaitDelay = time.Second
	command.Env = []string{
		"HOME=" + filepath.Join(sandbox, "home"),
		"TMPDIR=" + filepath.Join(sandbox, "tmp"),
		"XDG_CONFIG_HOME=" + filepath.Join(sandbox, "config"),
		"XDG_DATA_HOME=" + filepath.Join(sandbox, "data"),
		"XDG_CACHE_HOME=" + filepath.Join(sandbox, "cache"),
		"LANG=C", "LC_ALL=C",
	}
	return processprobe.OutputLimited(command, process.maxOutput)
}
