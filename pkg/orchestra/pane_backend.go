package orchestra

import (
	"context"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

const paneBackendName = "pane"

// promptRegisterDelay is the short delay between pasting prompt text and Enter,
// mirroring promptSubmitDelay used by the legacy interactive sender.
const promptRegisterDelay = 100 * time.Millisecond

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
	pi := paneInfo{paneID: paneID, provider: req.Config}
	defer func() { cleanupInteractivePanes(term, []paneInfo{pi}) }()

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
	cmd := buildInteractiveLaunchCmdWithCWD(req.Config, launchPrompt, b.cfg.WorkingDir)
	if err := term.SendLongText(ctx, paneID, cmd); err != nil {
		return paneFallback(ctx, req, "interactive pane execution failed: launch send error: "+err.Error())
	}
	if err := term.SendCommand(ctx, paneID, "\n"); err != nil {
		return paneFallback(ctx, req, "interactive pane execution failed: launch enter error: "+err.Error())
	}

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
	}

	// REQ-010/011/012: wait for completion using the resolved detector. hookSession
	// is nil when HookMode is false, which makes resolveCompletionDetector pick the
	// non-hook (poll/monitor) path automatically (REQ-012 degrade).
	var hookSession *HookSession
	completed := waitForCompletion(ctx, b.cfg, pi, DefaultCompletionPatterns(), "", hookSession, req.Round)

	resp := b.collectResponse(ctx, req, pi, !completed)
	resp.Duration = time.Since(start)
	return resp, nil
}
