package orchestra

import (
	"context"
	"log"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

const paneBackendName = "pane"

// promptRegisterDelay is the short delay between pasting prompt text and Enter,
// mirroring promptSubmitDelay used by the legacy interactive sender.
const promptRegisterDelay = 100 * time.Millisecond
const backendCompletionInitialDelay = 5 * time.Second

// InteractivePaneBackend executes a single provider in an interactive terminal
// pane (cmux/tmux-style). It is the default execution backend (REQ-004) and
// drives session-ready gating (REQ-009), completion detection (REQ-010/011/012),
// and deterministic timeout handling (REQ-011). The terminal and run config are
// held on the backend because ProviderRequest carries no terminal reference.
type InteractivePaneBackend struct {
	cfg OrchestraConfig
}

// NewInteractivePaneBackend constructs a pane backend bound to the given config.
func NewInteractivePaneBackend(cfg OrchestraConfig) *InteractivePaneBackend {
	return &InteractivePaneBackend{cfg: cfg}
}

// Name returns "pane".
func (b *InteractivePaneBackend) Name() string { return paneBackendName }

// Execute drives one provider pane end-to-end. On any failure BEFORE a
// deterministic pane result is produced (split, launch, never-ready, send),
// it degrades to best-effort subprocess fallback (REQ-005/013). A completion
// timeout is a deterministic pane result, not a failure: it returns a
// TimedOut response with sanitized partial output (REQ-011).
//
// @AX:WARN [AUTO]: multi-stage terminal I/O with >=8 sequential failure branches; any new stage MUST preserve the fallback-before-deterministic-result contract.
// @AX:REASON: split/launch/ready-gate/send/wait stages each branch to paneFallback (fan-out 7); reordering or skipping the session-ready gate (REQ-009) silently sends the prompt before the CLI is live.
func (b *InteractivePaneBackend) Execute(ctx context.Context, req ProviderRequest) (*ProviderResponse, error) {
	if b.cfg.Terminal == nil {
		// Defensive: no terminal means we cannot run a pane at all.
		return paneFallback(ctx, req, "interactive pane execution failed: no terminal attached")
	}

	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	term := b.cfg.Terminal
	start := time.Now()

	paneID, err := term.SplitPane(ctx, terminal.Horizontal)
	if err != nil {
		return paneFallback(ctx, req, "interactive pane execution failed: SplitPane error: "+err.Error())
	}
	pi := paneInfo{paneID: paneID, provider: req.Config, role: req.Role}
	defer func() { cleanupInteractivePanes(term, []paneInfo{pi}) }()

	// SPEC-ORCH-022: export the hook session ID (and round) into the pane shell
	// BEFORE the provider CLI launches so the provider's completion hook
	// (Stop/AfterAgent) sees AUTOPUS_SESSION_ID and writes the done-file. The
	// pane is a fresh login shell that does not inherit the orchestrator env, so
	// os.Setenv alone is invisible to it. The structured spec review / orchestra
	// run paths drive Execute directly (not RunInteractivePaneOrchestra), so the
	// env injection that path performs in launchInteractiveSessions must be
	// mirrored here. Non-fatal: a send failure degrades to screen-poll completion.
	if b.cfg.HookMode && b.cfg.SessionID != "" {
		if envErr := SendSessionEnvToPane(ctx, term, paneID, b.cfg.SessionID); envErr != nil {
			log.Printf("pane_backend: SendSessionEnvToPane failed for %s (non-fatal): %v", req.Provider, envErr)
		} else {
			_ = term.SendCommand(ctx, paneID, "\n")
			if req.Round > 0 {
				_ = SendRoundEnvToPane(ctx, term, paneID, req.Round)
				_ = term.SendCommand(ctx, paneID, "\n")
			}
			time.Sleep(promptRegisterDelay)
		}
	}

	// Launch the provider CLI. For args-based providers the prompt rides on the
	// launch command; otherwise the prompt is sent only after session-ready.
	launchPrompt := ""
	if promptDeliveredAtLaunch(req.Config) {
		var promptFile string
		var responseFile string
		launchPrompt, promptFile, responseFile = panePromptText(b.cfg, req.Config, req.Round, req.Prompt)
		if promptFile != "" {
			pi.promptFiles = append(pi.promptFiles, promptFile)
		}
		pi.responseFile = responseFile
	}
	cmd, launchFile, launchErr := buildPaneLaunchCommand(b.cfg.WorkingDir, req.Config, launchPrompt)
	if launchErr != nil {
		return paneFallback(ctx, req, "interactive pane execution failed: launch command error: "+launchErr.Error())
	}
	if launchFile != "" {
		pi.launchFiles = append(pi.launchFiles, launchFile)
	}
	if err := term.SendLongText(ctx, paneID, cmd); err != nil {
		return paneFallback(ctx, req, "interactive pane execution failed: launch send error: "+err.Error())
	}
	if err := term.SendCommand(ctx, paneID, "\n"); err != nil {
		return paneFallback(ctx, req, "interactive pane execution failed: launch enter error: "+err.Error())
	}
	time.Sleep(promptRegisterDelay)
	completionBaseline := captureCompletionBaseline(ctx, term, paneID)

	if !promptDeliveredAtLaunch(req.Config) {
		// REQ-009: gate the prompt on session readiness. If the session never
		// becomes ready the prompt is NEVER sent and we degrade to fallback.
		ready := pollUntilSessionReady(ctx, term, paneID, SessionReadyPatterns(), startupTimeoutFor(req.Config))
		if !ready {
			return paneFallback(ctx, req,
				"interactive pane execution failed: session never became ready (prompt was not sent)")
		}

		// REQ-009: send the prompt only after ready when it was not delivered at launch.
		promptText, promptFile, responseFile := panePromptText(b.cfg, req.Config, req.Round, req.Prompt)
		if promptFile != "" {
			pi.promptFiles = append(pi.promptFiles, promptFile)
		}
		pi.responseFile = responseFile
		if err := term.SendLongText(ctx, paneID, promptText); err != nil {
			return paneFallback(ctx, req, "interactive pane execution failed: prompt send error: "+err.Error())
		}
		time.Sleep(promptRegisterDelay)
		if err := term.SendCommand(ctx, paneID, "\n"); err != nil {
			return paneFallback(ctx, req, "interactive pane execution failed: prompt enter error: "+err.Error())
		}
		time.Sleep(promptRegisterDelay)
		completionBaseline = captureCompletionBaseline(ctx, term, paneID)
	}
	sleepBeforeCompletion(ctx, b.cfg)

	// REQ-010/011/012: wait for completion using the resolved detector.
	// When HookMode is enabled and a SessionID is set, create a real HookSession
	// so resolveCompletionDetector can select FileIPCDetector (REQ-006/007).
	// On creation failure we degrade gracefully to nil so the non-hook poll
	// path is used instead (REQ-012 degrade — never fail hard here).
	hookSession := resolveHookSession(b.cfg)
	if hookSession != nil {
		defer hookSession.Cleanup()
	}
	completed := waitForCompletion(ctx, b.cfg, pi, DefaultCompletionPatterns(), completionBaseline, hookSession, req.Round)

	resp := b.collectResponse(ctx, req, pi, !completed)
	resp.Duration = time.Since(start)
	return resp, nil
}

func captureCompletionBaseline(ctx context.Context, term terminal.Terminal, paneID terminal.PaneID) string {
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	screen, err := readScreenBounded(readCtx, term, paneID, terminal.ReadScreenOpts{})
	if err != nil {
		return ""
	}
	return screen
}

func sleepBeforeCompletion(ctx context.Context, cfg OrchestraConfig) {
	delay := completionInitialDelay(cfg, backendCompletionInitialDelay)
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return
		}
		capDelay := remaining / 4
		if capDelay < promptRegisterDelay {
			capDelay = promptRegisterDelay
		}
		if delay > capDelay {
			delay = capDelay
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// resolveHookSession creates a HookSession when HookMode is enabled and a
// SessionID is configured. Returns nil on any failure so callers fall back to
// the screen-poll completion path (REQ-012 graceful degrade).
func resolveHookSession(cfg OrchestraConfig) *HookSession {
	if !cfg.HookMode || cfg.SessionID == "" {
		return nil
	}
	sess, err := NewHookSession(cfg.SessionID)
	if err != nil {
		log.Printf("pane_backend: HookSession creation failed for session %q, degrading to poll: %v", cfg.SessionID, err)
		return nil
	}
	return sess
}
