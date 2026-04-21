package orchestra

import (
	"context"
	"fmt"
	"os"
	"time"
)

// RunPaneOrchestraDetached launches providers in panes and returns a job ID
// without waiting for completion. Only works with pane-capable terminals.
// @AX:NOTE [AUTO] REQ-1 detach entry point — must return in <2s; no collectPaneResults or cleanupPanes; fan_in=2 (orchestra.go auto-detach, tests)
func RunPaneOrchestraDetached(ctx context.Context, cfg OrchestraConfig) (string, error) {
	if cfg.Terminal == nil || cfg.Terminal.Name() == "plain" {
		return "", fmt.Errorf("detach mode requires a pane-capable terminal")
	}

	jobID := randomHex() + randomHex() // 16 hex chars
	runID := ensureRunID(&cfg)
	promptArtifact := sanitizeArtifact(cfg.Prompt)
	store, err := newReliabilityStore(runID)
	if err != nil {
		return "", fmt.Errorf("create reliability store: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "autopus-orch-")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	panes, _, err := splitProviderPanes(ctx, cfg)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", fmt.Errorf("split panes: %w", err)
	}

	sendPaneCommands(ctx, cfg, panes)

	timeout := cfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 120
	}

	providerNames := make([]string, len(cfg.Providers))
	paneIDs := make(map[string]string, len(panes))
	outputFiles := make(map[string]string, len(panes))
	for i, pi := range panes {
		providerNames[i] = pi.provider.Name
		paneIDs[pi.provider.Name] = string(pi.paneID)
		outputFiles[pi.provider.Name] = pi.outputFile
	}

	job := &Job{
		ID:          jobID,
		RunID:       runID,
		Strategy:    cfg.Strategy,
		Providers:   providerNames,
		Prompt:      promptArtifact.Preview,
		PromptHash:  promptArtifact.Hash,
		CreatedAt:   time.Now(),
		TimeoutAt:   time.Now().Add(time.Duration(timeout) * time.Second),
		Status:      JobStatusRunning,
		Dir:         tmpDir,
		ArtifactDir: store.artifactDir(),
		Results:     map[string]*ProviderResponse{},
		PaneIDs:     paneIDs,
		Terminal:    cfg.Terminal.Name(),
		Judge:       cfg.JudgeProvider,
		OutputFiles: outputFiles,
	}

	if err := job.Save(); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", fmt.Errorf("save job: %w", err)
	}

	return jobID, nil
}

// ShouldDetach determines whether auto-detach should activate based on
// terminal type, TTY status, and the no-detach flag.
// @AX:NOTE [AUTO] REQ-1 decision logic — plain terminal always returns false; mirrors CLI --no-detach flag semantics
func ShouldDetach(terminalName string, isTTY bool, noDetach bool) bool {
	if noDetach || !isTTY {
		return false
	}
	return terminalName != "plain"
}
