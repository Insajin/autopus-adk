package orchestra

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

const recreatePipeRetryBaseDelay = 200 * time.Millisecond
const recreatePostReadyDelay = 200 * time.Millisecond

// needsSurfaceCheck returns true if the provider's surface should be validated
// before sending prompts in Round 2+. All providers are checked because cmux
// surfaces can become stale after long rounds regardless of CLI persistence.
func needsSurfaceCheck(_ ProviderConfig) bool {
	return true
}

// validateSurface checks whether a pane's surface is still active by attempting
// a lightweight ReadScreen call. Returns true if the surface is valid. (R1)
func validateSurface(ctx context.Context, term terminal.Terminal, paneID terminal.PaneID) bool {
	_, err := readScreenBounded(ctx, term, paneID, terminal.ReadScreenOpts{})
	return err == nil
}

// recreatePane closes a stale pane and creates a fresh one with the provider's
// CLI session relaunched. The round parameter is used to set AUTOPUS_ROUND env
// before CLI launch. For args providers in round > 1, the CLI is launched in
// REPL mode (without the original prompt). Returns the updated paneInfo on
// success. (R2, R3, R4)
func recreatePane(ctx context.Context, cfg OrchestraConfig, pi paneInfo, round int) (paneInfo, error) {
	oldPaneID := pi.paneID

	// Prepare a replacement while the old pane remains available. The old pane
	// is retired only after the new CLI session reaches the commit point.
	newPaneID, err := splitPaneSerialized(ctx, cfg.Terminal, terminal.Horizontal)
	if err != nil {
		return pi, fmt.Errorf("recreatePane SplitPane for %s: %w", pi.provider.Name, err)
	}

	// Create new temp output file.
	safeName := sanitizeProviderName(pi.provider.Name)
	tmpFile, err := os.CreateTemp("", "autopus-orch-"+safeName+"-")
	if err != nil {
		closePaneSurface(cfg.Terminal, newPaneID)
		return pi, fmt.Errorf("recreatePane CreateTemp for %s: %w", pi.provider.Name, err)
	}
	if err := tmpFile.Close(); err != nil {
		closePaneSurface(cfg.Terminal, newPaneID)
		_ = os.Remove(tmpFile.Name())
		return pi, fmt.Errorf("recreatePane close output for %s: %w", pi.provider.Name, err)
	}

	// Start pipe capture on new pane with retry — cmux surfaces need time to initialize.
	// Pipe capture is used only for idle fallback detection (isOutputIdle).
	// If it fails after retries, the pane is still usable for SendLongText/ReadScreen
	// and completion detection via screen polling or signal. Non-fatal.
	outputPath := tmpFile.Name()
	var pipeErr error
	for attempt := range 3 {
		if attempt > 0 {
			delay := time.Duration(attempt) * recreatePipeRetryBaseDelay
			log.Printf("[recreatePane] %s PipePaneStart attempt %d failed, waiting %v...", pi.provider.Name, attempt, delay)
			time.Sleep(delay)
		}
		if pipeErr = cfg.Terminal.PipePaneStart(ctx, newPaneID, tmpFile.Name()); pipeErr == nil {
			break
		}
	}
	if pipeErr != nil {
		// Pipe capture failed — disable idle fallback by clearing outputFile.
		// The pane itself is still functional for interactive I/O.
		log.Printf("[recreatePane] %s PipePaneStart failed after retries (non-fatal, idle fallback disabled): %v", pi.provider.Name, pipeErr)
		_ = os.Remove(tmpFile.Name())
		outputPath = ""
	}

	// Set round env on new pane before launching CLI.
	if round > 1 && pi.provider.InteractiveInput == "args" {
		roundErr, roundEnterErr := sendPaneInputAndEnterSerialized(ctx, cfg.Terminal, newPaneID, 0, func() error {
			return SendRoundEnvToPane(ctx, cfg.Terminal, newPaneID, round)
		})
		if roundErr != nil || roundEnterErr != nil {
			log.Printf("[recreatePane] %s round env failed (non-fatal): send=%v enter=%v", pi.provider.Name, roundErr, roundEnterErr)
		}
	}

	// Relaunch CLI session. For args providers in round > 1, launch in REPL
	// mode without the original prompt — the round prompt will be sent via
	// SendLongText later by the caller.
	cmd := buildInteractiveLaunchCmdWithCWD(pi.provider, "", cfg.WorkingDir)
	launchErr, launchEnterErr := sendPaneInputAndEnterSerialized(ctx, cfg.Terminal, newPaneID, promptRegisterDelay, func() error {
		return cfg.Terminal.SendLongText(ctx, newPaneID, cmd)
	})
	if launchErr != nil {
		closePaneSurface(cfg.Terminal, newPaneID)
		_ = os.Remove(tmpFile.Name())
		return pi, fmt.Errorf("recreatePane launch for %s: %w", pi.provider.Name, launchErr)
	}
	if launchEnterErr != nil {
		closePaneSurface(cfg.Terminal, newPaneID)
		_ = os.Remove(tmpFile.Name())
		return pi, fmt.Errorf("recreatePane launch enter for %s: %w", pi.provider.Name, launchEnterErr)
	}

	// Wait for session readiness.
	patterns := SessionReadyPatterns()
	timeout := startupTimeoutFor(pi.provider)
	if !pollUntilSessionReady(ctx, cfg.Terminal, newPaneID, patterns, timeout) {
		closePaneSurface(cfg.Terminal, newPaneID)
		_ = os.Remove(tmpFile.Name())
		return pi, fmt.Errorf("recreatePane session for %s did not become ready after %s", pi.provider.Name, timeout)
	}

	// Post-ready stabilization: allow the CLI and cmux surface to fully
	// initialize before accepting paste-buffer input. Without this delay,
	// paste-buffer fails with exit status 1 on newly created surfaces.
	time.Sleep(recreatePostReadyDelay)

	retirePaneAfterReplacement(ctx, cfg.Terminal, pi)

	// R3: Log successful recreation.
	log.Printf("[Surface] %s pane recreated: %s → %s", pi.provider.Name, oldPaneID, newPaneID)

	return paneInfo{
		paneID:     newPaneID,
		outputFile: outputPath,
		provider:   pi.provider,
		skipWait:   false,
	}, nil
}

func retirePaneAfterReplacement(ctx context.Context, term terminal.Terminal, pi paneInfo) {
	_ = term.PipePaneStop(ctx, pi.paneID)
	closePaneSurface(term, pi.paneID)
	_ = os.Remove(pi.outputFile)
	cleanupPromptFiles(pi.promptFiles)
	_ = os.Remove(pi.responseFile)
	cleanupPromptFiles(pi.launchFiles)
}
