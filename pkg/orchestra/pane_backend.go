package orchestra

import (
	"context"
	"log"
	"strings"
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

// Execute drives one provider pane end-to-end. Subprocess fallback is allowed
// only before pane provisioning commits. A non-empty pane ID is the commit
// point; launch, readiness, prompt, and collection failures after that point
// remain pane-path failures. A completion timeout returns a deterministic
// TimedOut pane response with sanitized partial output (REQ-011).
//
// @AX:WARN [AUTO]: multi-stage terminal I/O with >=8 sequential failure branches; any new stage MUST preserve the SplitPane commit-point contract.
// @AX:REASON: only terminal absence and SplitPane failure may invoke subprocess fallback; every failure after a non-empty pane ID must report ExecutedBackend=pane without running a provider subprocess.
func (b *InteractivePaneBackend) Execute(ctx context.Context, req ProviderRequest) (*ProviderResponse, error) {
	if b.cfg.Terminal == nil {
		// Defensive: no terminal means we cannot run a pane at all.
		return paneProvisioningFallback(ctx, b.cfg, req, "interactive pane execution failed: no terminal attached")
	}

	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	term := b.cfg.Terminal
	start := time.Now()

	paneID, err := splitPaneSerialized(ctx, term, terminal.Horizontal)
	if paneID == "" {
		if err != nil {
			return paneProvisioningFallback(ctx, b.cfg, req, "interactive pane execution failed: SplitPane error: "+err.Error())
		}
		return paneProvisioningFallback(ctx, b.cfg, req, "interactive pane execution failed: SplitPane returned an empty pane ID")
	}
	if err != nil {
		closePaneSurface(term, paneID)
		return paneExecutionFailure(req, "interactive pane execution failed after SplitPane committed pane "+string(paneID)+": "+err.Error())
	}
	pi := paneInfo{paneID: paneID, provider: req.Config, role: req.Role}
	defer func() { cleanupInteractivePanes(term, []paneInfo{pi}) }()

	// Create and reset the provider-scoped hook attempt before launch. Structured
	// reviewer reprompts reuse the same session/provider/round, so stale done or
	// result artifacts would otherwise complete the new pane immediately. The
	// reset is intentionally provider-scoped because sibling providers execute in
	// parallel within the same HookSession directory.
	hookSession := resolveHookSession(b.cfg)
	if hookSession != nil && hookSession.HasHook(req.Config.Name) {
		if resetErr := hookSession.ResetAttempt(req.Config.Name, req.Round); resetErr != nil {
			return paneExecutionFailure(req, "interactive pane execution failed: reset hook attempt: "+resetErr.Error())
		}
	}

	// SPEC-ORCH-022: export the hook session ID (and round) into the pane shell
	// BEFORE the provider CLI launches so the provider's completion hook
	// (Stop/AfterAgent) sees AUTOPUS_SESSION_ID and writes the done-file. The
	// pane is a fresh login shell that does not inherit the orchestrator env, so
	// os.Setenv alone is invisible to it. The structured spec review / orchestra
	// run paths drive Execute directly (not RunInteractivePaneOrchestra), so the
	// env injection that path performs in launchInteractiveSessions must be
	// mirrored here. Non-fatal: a send failure degrades to screen-poll completion.
	if b.cfg.HookMode && b.cfg.SessionID != "" {
		envErr, enterErr := sendPaneInputAndEnterSerialized(ctx, term, paneID, 0, func() error {
			return SendSessionEnvToPane(ctx, term, paneID, b.cfg.SessionID)
		})
		if envErr != nil {
			log.Printf("pane_backend: SendSessionEnvToPane failed for %s (non-fatal): %v", req.Provider, envErr)
		} else {
			if enterErr != nil {
				log.Printf("pane_backend: session-env Enter failed for %s (non-fatal): %v", req.Provider, enterErr)
			}
			if req.Round > 0 {
				roundErr, roundEnterErr := sendPaneInputAndEnterSerialized(ctx, term, paneID, 0, func() error {
					return SendRoundEnvToPane(ctx, term, paneID, req.Round)
				})
				if roundErr != nil || roundEnterErr != nil {
					log.Printf("pane_backend: round env failed for %s (non-fatal): send=%v enter=%v", req.Provider, roundErr, roundEnterErr)
				}
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
		return paneExecutionFailure(req, "interactive pane execution failed: launch command error: "+launchErr.Error())
	}
	if launchFile != "" {
		pi.launchFiles = append(pi.launchFiles, launchFile)
	}
	launchSendErr, launchEnterErr := sendPaneInputAndEnterSerialized(ctx, term, paneID, promptRegisterDelay, func() error {
		return term.SendLongText(ctx, paneID, cmd)
	})
	if launchSendErr != nil {
		return paneExecutionFailure(req, "interactive pane execution failed: launch send error: "+launchSendErr.Error())
	}
	if launchEnterErr != nil {
		return paneExecutionFailure(req, "interactive pane execution failed: launch enter error: "+launchEnterErr.Error())
	}
	time.Sleep(promptRegisterDelay)
	completionBaseline := captureCompletionBaseline(ctx, term, paneID)

	if !promptDeliveredAtLaunch(req.Config) {
		// REQ-009: gate the prompt on session readiness. If the session never
		// becomes ready the prompt is never sent and the committed pane fails.
		ready := waitForPaneReady(ctx, term, paneID, SessionReadyPatterns(), startupTimeoutFor(req.Config), hookSession, req.Config.Name, req.Round)
		if !ready {
			return paneExecutionFailure(req,
				"interactive pane execution failed: session never became ready (prompt was not sent)")
		}

		// REQ-009: send the prompt only after ready when it was not delivered at launch.
		promptText, promptFile, responseFile := panePromptText(b.cfg, req.Config, req.Round, req.Prompt)
		if promptFile != "" {
			pi.promptFiles = append(pi.promptFiles, promptFile)
		}
		pi.responseFile = responseFile
		promptSendErr, promptEnterErr := sendPaneInputAndEnterSerialized(ctx, term, paneID, panePromptSubmitDelay(req.Config), func() error {
			return sendPanePromptInput(ctx, term, paneID, req.Config, promptText, promptFile != "")
		})
		if promptSendErr != nil {
			return paneExecutionFailure(req, "interactive pane execution failed: prompt send error: "+promptSendErr.Error())
		}
		if promptEnterErr != nil {
			return paneExecutionFailure(req, "interactive pane execution failed: prompt enter error: "+promptEnterErr.Error())
		}
		time.Sleep(promptRegisterDelay)
		completionBaseline = captureCompletionBaseline(ctx, term, paneID)
	}
	sleepBeforeCompletion(ctx, b.cfg)

	// REQ-010/011/012: wait for completion using the resolved detector. The hook
	// session (created before launch) lets resolveCompletionDetector select
	// FileIPCDetector (REQ-006/007); a nil session degrades to the poll path.
	completed := waitForCompletion(ctx, b.cfg, pi, DefaultCompletionPatterns(), completionBaseline, hookSession, req.Round)

	resp := b.collectResponse(ctx, req, pi, !completed, hookSession)
	resp.Duration = time.Since(start)
	return resp, nil
}

func sendPanePromptInput(ctx context.Context, term terminal.Terminal, paneID terminal.PaneID, provider ProviderConfig, promptText string, fileBacked bool) error {
	if shouldUseSendkeysPromptInput(provider, fileBacked) {
		normalized := strings.ReplaceAll(promptText, "\n", " ")
		return term.SendCommand(ctx, paneID, normalized)
	}
	return term.SendLongText(ctx, paneID, promptText)
}

func shouldUseSendkeysPromptInput(provider ProviderConfig, fileBacked bool) bool {
	if strings.EqualFold(strings.TrimSpace(provider.InteractiveInput), "sendkeys") {
		return true
	}
	if !fileBacked {
		return false
	}
	return isCodexInteractiveProvider(provider)
}

func isCodexInteractiveProvider(provider ProviderConfig) bool {
	name := strings.EqualFold(strings.TrimSpace(provider.Name), "codex")
	binary := strings.TrimSpace(provider.Binary)
	return name || binary == "codex" || strings.HasSuffix(binary, "/codex")
}

func panePromptSubmitDelay(provider ProviderConfig) time.Duration {
	if isCodexInteractiveProvider(provider) {
		return 750 * time.Millisecond
	}
	return promptRegisterDelay
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
	sess.ApplyProviderHooks(cfg.Providers)
	return sess
}
