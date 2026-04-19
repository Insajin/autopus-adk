package cli

import (
	"time"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/platform"
	"github.com/insajin/autopus-adk/pkg/terminal"
)

type cc21MonitorRuntime struct {
	Enabled        bool
	HookMode       bool
	PatternTimeout time.Duration
}

func resolveCC21MonitorRuntime(term terminal.Terminal, cfg *config.HarnessConfig) cc21MonitorRuntime {
	features := platform.DetectFeatures()
	runtime := cc21MonitorRuntime{
		Enabled: features.Monitor,
	}
	if cfg != nil && cfg.Features.CC21.MonitorPatternTimeoutMS > 0 {
		runtime.PatternTimeout = time.Duration(cfg.Features.CC21.MonitorPatternTimeoutMS) * time.Millisecond
	}
	if term != nil && term.Name() != "plain" && runtime.Enabled && isHookModeAvailable() {
		runtime.HookMode = true
	}
	return runtime
}
