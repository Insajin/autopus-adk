package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/insajin/autopus-adk/pkg/pipeline"
	"github.com/insajin/autopus-adk/pkg/spec"
)

type resolvedPipelineSpec struct {
	Dir          string
	Path         string
	SnapshotHash string
}

func resolvePipelineSpec(specID string) (resolvedPipelineSpec, error) {
	resolved, err := spec.ResolveSpecDir(".", specID)
	if err != nil {
		return resolvedPipelineSpec{}, fmt.Errorf("SPEC not found: %s: %w", specID, err)
	}
	hash, err := pipelineSpecSnapshotHash(resolved.SpecDir)
	if err != nil {
		return resolvedPipelineSpec{}, fmt.Errorf("SPEC snapshot failed: %s: %w", specID, err)
	}
	return resolvedPipelineSpec{Dir: resolved.SpecDir, Path: resolved.SpecPath, SnapshotHash: hash}, nil
}

func pipelineSpecSnapshotHash(specDir string) (string, error) {
	h := sha256.New()
	for _, name := range []string{"spec.md", "plan.md", "acceptance.md", "research.md"} {
		path := filepath.Join(specDir, name)
		body, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) && name != "spec.md" {
				continue
			}
			return "", err
		}
		_, _ = h.Write([]byte(name))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(body)
		_, _ = h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

type pipelineProviderBackend struct {
	provider orchestra.ProviderConfig
	backend  orchestra.ExecutionBackend
}

func newPipelineProviderBackend(platform string) (pipeline.PhaseBackend, error) {
	if platform != "claude" && platform != "codex" && platform != "gemini" {
		return nil, fmt.Errorf("pipeline: unsupported platform %q", platform)
	}
	providers := buildProviderConfigs([]string{platform})
	if len(providers) != 1 || providers[0].Binary == "" {
		return nil, fmt.Errorf("pipeline: no provider config for platform %q", platform)
	}
	provider := providers[0]
	if _, err := exec.LookPath(provider.Binary); err != nil {
		return nil, fmt.Errorf("pipeline: platform %q executable %q is unavailable: %w", platform, provider.Binary, err)
	}
	return &pipelineProviderBackend{provider: provider, backend: orchestra.NewSubprocessBackendImpl()}, nil
}

func (b *pipelineProviderBackend) Execute(ctx context.Context, req pipeline.PhaseRequest) (*pipeline.PhaseResponse, error) {
	resp, err := b.backend.Execute(ctx, orchestra.ProviderRequest{
		Provider: b.provider.Name,
		Prompt:   req.Prompt,
		Role:     string(req.PhaseID),
		Round:    req.Attempt,
		Timeout:  b.provider.ExecutionTimeout,
		Config:   b.provider,
	})
	phaseResp := &pipeline.PhaseResponse{
		Provider: b.provider.Name, Backend: "subprocess", Role: string(req.PhaseID),
	}
	if resp != nil {
		phaseResp.Output = resp.Output
		phaseResp.ExitCode = resp.ExitCode
		phaseResp.TimedOut = resp.TimedOut
		phaseResp.Artifact = resp.Receipt
		if resp.Provider != "" {
			phaseResp.Provider = resp.Provider
		}
		if resp.ExecutedBackend != "" {
			phaseResp.Backend = resp.ExecutedBackend
		}
		if resp.Role != "" {
			phaseResp.Role = resp.Role
		}
	}
	if err != nil {
		phaseResp.FailureClass = pipelinePhaseFailureClass(resp, err)
		return phaseResp, err
	}
	if resp == nil {
		phaseResp.FailureClass = "execution_error"
		return phaseResp, fmt.Errorf("pipeline provider %s returned no response", b.provider.Name)
	}
	if resp.TimedOut {
		phaseResp.FailureClass = "timeout"
		return phaseResp, fmt.Errorf("pipeline provider %s timed out", b.provider.Name)
	}
	if resp.EmptyOutput {
		phaseResp.FailureClass = "empty_output"
		return phaseResp, fmt.Errorf("pipeline provider %s returned empty output", b.provider.Name)
	}
	return phaseResp, nil
}

func pipelinePhaseFailureClass(resp *orchestra.ProviderResponse, err error) string {
	if resp != nil {
		if resp.TimedOut {
			return "timeout"
		}
		if resp.EmptyOutput || strings.TrimSpace(resp.Output) == "" {
			return "empty_output"
		}
	}
	if err != nil && (strings.Contains(strings.ToLower(err.Error()), "timeout") ||
		strings.Contains(strings.ToLower(err.Error()), "deadline exceeded")) {
		return "timeout"
	}
	return "execution_error"
}

func persistPipelineBlockedReceipt(specID, snapshotHash, gitHash string, strategy pipeline.Strategy, blocker error) error {
	if err := os.MkdirAll(pipelineStateDir, 0o755); err != nil {
		return err
	}
	receipt := pipeline.NewBlockedRunReceipt(specID, strategy, blocker.Error())
	statuses := make(map[string]pipeline.CheckpointStatus, len(pipeline.DefaultPhases()))
	for _, phase := range pipeline.DefaultPhases() {
		statuses[string(phase.ID)] = pipeline.CheckpointStatusPending
	}
	cp := pipeline.Checkpoint{
		Version:       pipeline.CheckpointVersion,
		RouteVersion:  pipeline.PipelineRouteVersion,
		SpecID:        specID,
		SnapshotHash:  snapshotHash,
		GitCommitHash: gitHash,
		TaskStatus:    statuses,
		Receipt:       &receipt,
	}
	path := specCheckpointPath(specID)
	if _, err := os.Stat(path); err == nil {
		path = filepath.Join(pipelineStateDir, specID+".blocked.yaml")
	} else if !os.IsNotExist(err) {
		return err
	}
	return cp.SaveFile(path)
}

func pipelineBlockedError(specID, snapshotHash, gitHash string, strategy pipeline.Strategy, cause error) error {
	if err := persistPipelineBlockedReceipt(specID, snapshotHash, gitHash, strategy, cause); err != nil {
		return fmt.Errorf("%w (persist blocked receipt: %v)", cause, err)
	}
	return cause
}
