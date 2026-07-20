package cli

import (
	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/insajin/autopus-adk/pkg/terminal"
)

// applyHookMode enables hook-IPC completion collection on the orchestra config
// when a pane-capable terminal and installed completion hooks are present.
// SPEC-ORCH-021 wired HookMode only for the runOrchestraCommand path; the
// structured spec review and orchestra run paths set only Terminal, leaving
// HookMode false so the relaxed CLAUDECODE guard (SPEC-ORCH-022 T5) routed into
// the interactive pane backend without hook collection and fell back to screen
// polling. This closes that gap.
func applyHookMode(cfg *orchestra.OrchestraConfig) {
	cfg.HookMode = false
	cfg.SessionID = ""
	if !hookCollectionEligible(cfg.Terminal, cfg.SubprocessMode, isHookModeAvailable()) {
		return
	}
	cfg.HookMode = true
	cfg.SessionID = newOrchSessionID()
	// MonitorEnabled/Timeout carry the optional CC21 SignalDetector fast-path; the
	// FileIPCDetector floor stays active via HookMode regardless of their values.
	runtime := resolveCC21MonitorRuntime(cfg.Terminal, nil)
	cfg.MonitorEnabled = cfg.MonitorEnabled || runtime.Enabled
	if cfg.MonitorTimeout <= 0 {
		cfg.MonitorTimeout = runtime.PatternTimeout
	}
}

// hookCollectionEligible reports whether hook-IPC done-file collection can run:
// a pane-capable terminal (non-nil, non-plain, subprocess not forced) plus
// installed completion hooks. Intentionally NOT gated on
// platform.DetectFeatures().Monitor — done-file IPC (FileIPCDetector) does not
// depend on the CC21 monitor pattern-detection feature, and gating on it made
// the relaxed CLAUDECODE pane route silently fall back to screen polling
// whenever that feature flag was off (the 0/N path this SPEC fixes). It mirrors
// the pane-capability predicate used by SelectBackend so routing and hook
// activation stay consistent.
func hookCollectionEligible(term terminal.Terminal, subprocessMode, hookAvailable bool) bool {
	if subprocessMode || !hookAvailable || term == nil || term.Name() == "plain" {
		return false
	}
	return true
}

// newOrchSessionID returns a hook session ID matching the safe pattern enforced
// by NewHookSession and SendSessionEnvToPane ([a-zA-Z0-9_-]+).
func newOrchSessionID() string {
	return orchestra.NewSessionID()
}
