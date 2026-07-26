package orchestra

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// Round-continuation hooks should already be waiting when the next round starts.
// Two seconds gives the 200ms artifact poller ten observations while bounding
// sequential ready waits so a three-provider round retains recovery time.
const fileIPCRoundReadyGrace = 2 * time.Second

const (
	fileIPCRecoveryAttempt          = 1
	recoveryStageOwnerClosed        = "owner_closed"
	recoveryStageReplacementReady   = "replacement_ready"
	recoveryStagePromptSubmitted    = "prompt_submitted"
	recoveryStatusPass              = "pass"
	recoveryStatusFailed            = "failed"
	recoveryFailureOwnerClose       = "owner_close_failed"
	recoveryFailureReplacementReady = "replacement_ready_failed"
	recoveryFailurePromptSubmit     = "prompt_submit_failed"
)

type roundFileIPCResolution struct {
	pane      paneInfo
	delivered bool
	recovered bool
}

// @AX:NOTE: [AUTO] release recovery retires the old artifact owner before replacement to prevent duplicate provider-round ownership
func resolveRoundFileIPC(
	ctx context.Context,
	cfg OrchestraConfig,
	hookSession *HookSession,
	pi paneInfo,
	round int,
	prompt string,
	sendPrompt string,
) (roundFileIPCResolution, error) {
	resolution := roundFileIPCResolution{pane: pi}
	outcome, ipcErr := tryFileIPCWithTimeouts(
		ctx,
		hookSession,
		pi.provider.Name,
		round,
		sendPrompt,
		fileIPCRoundReadyGrace,
		defaultHookReleaseTimeout,
	)
	if outcome == fileIPCDelivered {
		resolution.delivered = true
		recordRoundFileIPCPrompt(cfg, pi.provider.Name, prompt, round, "pass", "")
		return resolution, nil
	}

	failureReason := "file IPC fallback activated before completion wait"
	failureCode := promptFailureFileIPCFallback
	if ipcErr != nil {
		failureReason = ipcErr.Error()
		failureCode = fileIPCPromptFailureCode(ipcErr)
	}
	recordRoundFileIPCPrompt(cfg, pi.provider.Name, prompt, round, "failed", failureCode)
	if outcome == fileIPCSafeFallback {
		return resolution, nil
	}
	if outcome != fileIPCReleaseFailure {
		return resolution, fmt.Errorf("unexpected file IPC outcome %d", outcome)
	}

	log.Printf(
		"[Round %d] %s file IPC release failed: %s -- replacing pane",
		round, pi.provider.Name, failureReason,
	)
	recovered, err := recoverPaneAfterFileIPCReleaseFailure(ctx, cfg, pi, round)
	resolution.pane = recovered
	if err != nil {
		return resolution, err
	}
	resolution.recovered = true
	return resolution, nil
}

func recordRoundFileIPCPrompt(
	cfg OrchestraConfig,
	provider string,
	prompt string,
	round int,
	status string,
	failureCode string,
) {
	if cfg.ReliabilityStore == nil {
		return
	}
	receipt := promptReceipt(cfg.RunID, provider, "file_ipc", prompt, round, status, failureCode)
	_ = cfg.ReliabilityStore.recordPrompt(receipt)
}

// A release failure leaves the old hook waiter owning the same provider/round
// artifact names. Close that pane before recreatePane resets those artifacts.
func recoverPaneAfterFileIPCReleaseFailure(
	ctx context.Context,
	cfg OrchestraConfig,
	pi paneInfo,
	round int,
) (paneInfo, error) {
	if _, err := recoveryHookSession(cfg); err != nil {
		return pi, fmt.Errorf("preflight file IPC pane recovery: %w", err)
	}

	retired, err := retireFileIPCReleaseOwner(ctx, cfg, pi, round)
	if err != nil {
		return pi, err
	}
	replacement, err := recreatePaneAfterRetirement(ctx, cfg, retired, round)
	if err != nil {
		recordFileIPCRecoveryTransition(
			cfg, pi.provider.Name, round, recoveryStageReplacementReady,
			recoveryStatusFailed, recoveryFailureReplacementReady,
		)
		retired.skipWait = true
		return retired, fmt.Errorf("relaunch after file IPC release failure: %w", err)
	}
	recordFileIPCRecoveryTransition(
		cfg, pi.provider.Name, round, recoveryStageReplacementReady,
		recoveryStatusPass, "",
	)
	return replacement, nil
}

func retireFileIPCReleaseOwner(
	ctx context.Context,
	cfg OrchestraConfig,
	pi paneInfo,
	round int,
) (paneInfo, error) {
	_ = cfg.Terminal.PipePaneStop(ctx, pi.paneID)
	if !closePaneSurface(cfg.Terminal, pi.paneID) {
		recordFileIPCRecoveryTransition(
			cfg, pi.provider.Name, round, recoveryStageOwnerClosed,
			recoveryStatusFailed, recoveryFailureOwnerClose,
		)
		return pi, fmt.Errorf("close unreleased hook owner %s", pi.paneID)
	}
	recordFileIPCRecoveryTransition(
		cfg, pi.provider.Name, round, recoveryStageOwnerClosed,
		recoveryStatusPass, "",
	)
	_ = os.Remove(pi.outputFile)
	cleanupPromptFiles(pi.promptFiles)
	_ = os.Remove(pi.responseFile)
	cleanupPromptFiles(pi.launchFiles)

	pi.paneID = ""
	pi.outputFile = ""
	pi.promptFiles = nil
	pi.responseFile = ""
	pi.launchFiles = nil
	return pi, nil
}

func submitFileIPCRecoveryPromptOnce(
	ctx context.Context,
	cfg OrchestraConfig,
	pi paneInfo,
	sendPrompt string,
	fileBacked bool,
	submitDelay time.Duration,
) error {
	var sendErr, enterErr error
	if shouldUseSendkeysPromptInput(pi.provider, fileBacked) {
		normalized := strings.ReplaceAll(sendPrompt, "\n", " ")
		sendErr, enterErr = sendPaneInputAndEnterSerialized(
			ctx, cfg.Terminal, pi.paneID, submitDelay,
			func() error {
				return cfg.Terminal.SendCommand(ctx, pi.paneID, normalized)
			},
			time.Second,
		)
	} else {
		sendErr, enterErr = sendPaneInputAndEnterSerialized(
			ctx, cfg.Terminal, pi.paneID, submitDelay,
			func() error {
				return cfg.Terminal.SendLongText(ctx, pi.paneID, sendPrompt)
			},
			time.Second,
		)
	}
	if enterErr != nil {
		recordFileIPCRecoveryTransition(
			cfg, pi.provider.Name, pi.directPromptRound, recoveryStagePromptSubmitted,
			recoveryStatusFailed, recoveryFailurePromptSubmit,
		)
		return &paneSubmitEnterError{err: enterErr}
	}
	status := recoveryStatusPass
	failureCode := ""
	if sendErr != nil {
		status = recoveryStatusFailed
		failureCode = recoveryFailurePromptSubmit
	}
	recordFileIPCRecoveryTransition(
		cfg, pi.provider.Name, pi.directPromptRound, recoveryStagePromptSubmitted,
		status, failureCode,
	)
	return sendErr
}

func recordFileIPCRecoveryTransition(
	cfg OrchestraConfig,
	provider string,
	round int,
	stage string,
	status string,
	failureCode string,
) {
	if cfg.ReliabilityStore == nil {
		return
	}
	_ = cfg.ReliabilityStore.recordRecoveryTransition(
		provider, round, fileIPCRecoveryAttempt, stage, status, failureCode,
	)
}
