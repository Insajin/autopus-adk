package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

type pipelineOMPBackendConfig struct {
	Executable     string
	ProjectDir     string
	SpecID         string
	SpecDir        string
	SnapshotHash   string
	GitCommitHash  string
	RuntimeBase    string
	Environment    []string
	PhaseModels    map[pipeline.PhaseID]string
	MaxTime        time.Duration
	ownRuntimeBase bool
	executableID   pipelineOMPExecutableIdentity
}

type pipelineOMPBackend struct {
	mu       sync.Mutex
	config   pipelineOMPBackendConfig
	process  *pipelineOMPProcess
	protocol *pipelineOMPRPCProtocol
	closed   bool
}

var _ pipeline.PhaseBackend = (*pipelineOMPBackend)(nil)
var _ pipeline.PhaseBackendCloser = (*pipelineOMPBackend)(nil)

func newPipelineOMPBackend(config pipelineOMPBackendConfig) (*pipelineOMPBackend, error) {
	normalized, err := normalizePipelineOMPBackendConfig(config)
	if err != nil {
		return nil, err
	}
	return &pipelineOMPBackend{config: normalized}, nil
}

// @AX:WARN [AUTO]: Backend normalization has cyclomatic complexity 19 across authority and runtime checks.
// @AX:REASON [AUTO]: The RPC process must not start until every path, identity, permission, timeout, and copied input is canonical.
func normalizePipelineOMPBackendConfig(config pipelineOMPBackendConfig) (pipelineOMPBackendConfig, error) {
	for _, item := range []struct{ name, value string }{
		{"executable", config.Executable}, {"project directory", config.ProjectDir},
		{"SPEC ID", config.SpecID}, {"SPEC directory", config.SpecDir},
		{"snapshot hash", config.SnapshotHash}, {"git commit hash", config.GitCommitHash},
	} {
		if strings.TrimSpace(item.value) == "" || strings.ContainsRune(item.value, 0) {
			return config, fmt.Errorf("OMP pipeline %s is required", item.name)
		}
	}
	executable, executableID := config.Executable, config.executableID
	if executableID.info == nil {
		var err error
		executable, executableID, err = canonicalPipelineOMPExecutable(config.Executable)
		if err != nil {
			return config, err
		}
	} else if err := verifyPipelineOMPExecutable(executable, executableID); err != nil {
		return config, err
	}
	environment, err := normalizePipelineOMPEnvironment(config.Environment)
	if err != nil {
		return config, err
	}
	if info, err := os.Lstat(config.ProjectDir); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return config, errors.New("OMP pipeline project directory is unsafe")
	}
	if config.RuntimeBase == "" {
		base, err := os.MkdirTemp("", "autopus-omp-pipeline-")
		if err != nil {
			return config, err
		}
		config.RuntimeBase, config.ownRuntimeBase = base, true
	}
	baseInfo, err := os.Lstat(config.RuntimeBase)
	if err != nil || !baseInfo.IsDir() || baseInfo.Mode()&os.ModeSymlink != 0 || baseInfo.Mode().Perm()&0o077 != 0 {
		if config.ownRuntimeBase {
			_ = os.Remove(config.RuntimeBase)
		}
		return config, errors.New("OMP pipeline runtime base is unsafe")
	}
	if config.MaxTime <= 0 {
		config.MaxTime = 15 * time.Minute
	}
	config.ProjectDir = filepath.Clean(config.ProjectDir)
	config.SpecDir = filepath.Clean(config.SpecDir)
	config.Executable, config.executableID = executable, executableID
	config.Environment = environment
	config.PhaseModels = clonePipelineOMPPhaseModels(config.PhaseModels)
	return config, nil
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: this is the sole canonical phase-to-OMP RPC dispatch boundary.
// @AX:REASON [AUTO]: Sealed authority validation, one-process reuse, model selection, and fail-closed cleanup converge here.
// @AX:WARN [AUTO]: OMP phase dispatch has more than eight authority, lifecycle, initialization, and failure branches.
// @AX:REASON [AUTO]: Any mismatch or RPC failure must close the run-scoped process before another phase can reuse it.
func (backend *pipelineOMPBackend) Execute(
	ctx context.Context,
	request pipeline.PhaseRequest,
) (*pipeline.PhaseResponse, error) {
	response := &pipeline.PhaseResponse{
		Provider: "omp", Backend: "rpc-canonical-full", Role: string(request.PhaseID),
	}
	binding, err := request.OMPExecutionView.Binding()
	if err != nil {
		response.FailureClass = "execution_error"
		return response, err
	}
	expected := pipeline.OMPExecutionViewBinding{
		SpecID: backend.config.SpecID, SnapshotHash: backend.config.SnapshotHash,
		GitCommitHash: backend.config.GitCommitHash, PhaseID: request.PhaseID, Attempt: request.Attempt,
	}
	if binding != expected {
		response.FailureClass = "execution_error"
		return response, errors.New("pipeline: sealed OMP execution view binding mismatch")
	}
	snapshot, err := request.OMPExecutionView.Open(expected)
	if err != nil || snapshot.ProjectDir != backend.config.ProjectDir || snapshot.SpecDir != backend.config.SpecDir {
		response.FailureClass = "execution_error"
		return response, errors.New("pipeline: sealed OMP execution view authority mismatch")
	}
	if strings.HasPrefix(strings.TrimSpace(snapshot.Prompt), "/auto") {
		response.FailureClass = "execution_error"
		return response, errors.New("pipeline: managed OMP phase prompt cannot reissue /auto")
	}
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.closed {
		return response, errors.New("pipeline: OMP backend is closed")
	}
	if backend.process == nil {
		backend.process, err = startPipelineOMPProcess(ctx, backend.config)
		if err == nil {
			backend.protocol = newPipelineOMPRPCProtocol(backend.process)
			initCtx, cancel := context.WithTimeout(ctx, backend.config.MaxTime)
			err = backend.protocol.initialize(initCtx)
			cancel()
		}
		if err != nil {
			_ = backend.closeLocked()
			response.FailureClass = "execution_error"
			return response, err
		}
	}
	executeCtx, cancel := context.WithTimeout(ctx, backend.config.MaxTime)
	defer cancel()
	response.Output, err = backend.protocol.execute(executeCtx, backend.config.PhaseModels[request.PhaseID], snapshot.Prompt)
	if err != nil {
		response.FailureClass = "execution_error"
		_ = backend.closeLocked()
		return response, err
	}
	return response, nil
}

func (backend *pipelineOMPBackend) Close() error {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	return backend.closeLocked()
}

func (backend *pipelineOMPBackend) closeLocked() error {
	if backend.closed {
		return nil
	}
	backend.closed = true
	var err error
	if backend.process != nil {
		err = backend.process.Close()
	}
	if backend.config.ownRuntimeBase {
		err = errors.Join(err, os.Remove(backend.config.RuntimeBase))
	}
	return err
}

func clonePipelineOMPPhaseModels(input map[pipeline.PhaseID]string) map[pipeline.PhaseID]string {
	output := make(map[pipeline.PhaseID]string, len(input))
	for phase, model := range input {
		output[phase] = model
	}
	return output
}
