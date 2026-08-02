package omp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
)

var ompReadinessCapabilityIDs = []string{
	"identity.version",
	"launch.rpc",
	"launch.no_session",
	"launch.cwd",
	"launch.model",
	"config.overlay_readback",
	"catalog.models_json",
	"rpc.command_discovery",
	"rpc.tool_events",
	"rpc.terminal",
}

// OMPProbeRunner makes every readiness subprocess injectable and bounded by
// the context supplied for that individual probe.
type OMPProbeRunner interface {
	Run(ctx context.Context, executable string, args ...string) ([]byte, error)
}

type OMPReadinessOptions struct {
	Root       string
	Executable string
	Runner     OMPProbeRunner
	Timeout    time.Duration
	MaxOutput  int
	Selectors  []string
}

type OMPCapabilityResult struct {
	ID        string `json:"id"`
	Supported bool   `json:"supported"`
	Reason    string `json:"reason"`
}

type OMPSelectorResolution struct {
	Selector      string `json:"selector"`
	ResolvedModel string `json:"resolved_model,omitempty"`
	Status        string `json:"status"`
	Reason        string `json:"reason"`
}

type OMPReadinessReport struct {
	Executable          string                  `json:"executable"`
	Version             string                  `json:"version,omitempty"`
	Capabilities        []OMPCapabilityResult   `json:"capabilities"`
	CatalogReason       string                  `json:"catalog_reason"`
	SelectorResolutions []OMPSelectorResolution `json:"selector_resolutions,omitempty"`
}

type ompProbeResult struct {
	output []byte
	reason string
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-002: readiness exposes stable capability IDs and semantic reason enums only.
// @AX:REASON [AUTO]: doctor text/JSON consumers depend on this report shape, and raw subprocess output or
// task-owned absolute paths must not cross this public boundary.
func ProbeOMPReadiness(ctx context.Context, opts OMPReadinessOptions) (report OMPReadinessReport) {
	opts = normalizeOMPReadinessOptions(opts)
	report = OMPReadinessReport{
		Executable:   filepath.Base(opts.Executable),
		Capabilities: make([]OMPCapabilityResult, 0, len(ompReadinessCapabilityIDs)),
	}

	overlay, overlayErr := createOMPReadinessOverlay()
	if overlay != "" {
		defer func() {
			if err := os.RemoveAll(filepath.Dir(overlay)); err != nil {
				report = markOMPReadinessOverlayCleanupFailure(report)
			}
		}()
	}

	if overlayErr != nil {
		return ompIdentityFailureReport(report, opts.Selectors, "output_invalid")
	}
	if runner, ok := opts.Runner.(commandOMPProbeRunner); ok {
		runner, resolved := configureOMPProbeRunner(runner, opts.Executable, overlay)
		opts.Runner = runner
		if resolved != "" {
			opts.Executable = resolved
			report.Executable = filepath.Base(resolved)
		}
	}

	version := runOMPProbe(ctx, opts, "--version")
	versionResult, observedVersion := evaluateOMPVersion(version)
	report.Version = observedVersion
	report.Capabilities = append(report.Capabilities, versionResult)
	if !versionResult.Supported {
		return ompIdentityFailureReport(report, opts.Selectors, "identity_unverified")
	}

	help := runOMPProbe(ctx, opts, "--help")
	report.Capabilities = append(report.Capabilities,
		evaluateOMPHelpCapability("launch.rpc", help, "--mode", "rpc"),
		evaluateOMPHelpCapability("launch.no_session", help, "--no-session"),
		evaluateOMPHelpCapability("launch.cwd", help, "--cwd"),
		evaluateOMPHelpCapability("launch.model", help, "--model"),
	)

	configResult := runOMPProbe(ctx, opts,
		"--config", overlay, "config", "get", "skills.customDirectories", "--json")
	report.Capabilities = append(report.Capabilities, evaluateOMPConfigCapability(configResult))

	catalogResult := runOMPProbe(ctx, opts, "--config", overlay, "models", "--json")
	models, catalogReason, catalogCapability := evaluateOMPCatalog(catalogResult)
	report.CatalogReason = catalogReason
	report.Capabilities = append(report.Capabilities, catalogCapability)
	report.SelectorResolutions = resolveOMPSelectors(opts.Selectors, models, catalogReason)

	var rpcResult ompProbeResult
	if runner, ok := opts.Runner.(commandOMPProbeRunner); ok {
		rpcResult = runOMPReadinessBehavioralProbe(ctx, opts, runner, overlay)
	} else {
		rpcResult = runOMPProbe(ctx, opts, "--config", overlay, "--mode", "rpc",
			"--no-session", "--cwd", opts.Root)
	}
	report.Capabilities = append(report.Capabilities, evaluateOMPRPCCapabilities(rpcResult)...)
	return report
}

func markOMPReadinessOverlayCleanupFailure(report OMPReadinessReport) OMPReadinessReport {
	for index := range report.Capabilities {
		if report.Capabilities[index].ID == "config.overlay_readback" {
			report.Capabilities[index].Supported = false
			report.Capabilities[index].Reason = "output_invalid"
			break
		}
	}
	return report
}

func ompIdentityFailureReport(
	report OMPReadinessReport,
	selectors []string,
	reason string,
) OMPReadinessReport {
	for _, id := range ompReadinessCapabilityIDs[len(report.Capabilities):] {
		report.Capabilities = append(report.Capabilities, OMPCapabilityResult{ID: id, Reason: reason})
	}
	report.CatalogReason = reason
	report.SelectorResolutions = resolveOMPSelectors(selectors, nil, reason)
	return report
}

func normalizeOMPReadinessOptions(opts OMPReadinessOptions) OMPReadinessOptions {
	if opts.Executable == "" {
		opts.Executable = cliBinary
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Second
	}
	if opts.MaxOutput <= 0 {
		opts.MaxOutput = 64 * 1024
	}
	if opts.Root == "" {
		opts.Root = "."
	}
	if opts.Runner == nil {
		opts.Runner = commandOMPProbeRunner{maxOutput: opts.MaxOutput}
	}
	return opts
}

func createOMPReadinessOverlay() (string, error) {
	dir, err := os.MkdirTemp("", "autopus-omp-readiness-")
	if err != nil {
		return "", err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	for _, child := range []string{
		"home", "xdg-config", "xdg-cache", "xdg-data", "xdg-state", "tmp", "pi-agent",
	} {
		if err := os.Mkdir(filepath.Join(dir, child), 0o700); err != nil {
			_ = os.RemoveAll(dir)
			return "", err
		}
	}
	path := filepath.Join(dir, "config.yml")
	content := []byte("skills:\n  customDirectories:\n    - .agents/skills\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	return path, nil
}

func runOMPProbe(parent context.Context, opts OMPReadinessOptions, args ...string) ompProbeResult {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, opts.Timeout)
	defer cancel()
	output, err := opts.Runner.Run(ctx, opts.Executable, args...)
	if err != nil {
		reason := classifyOMPProbeError(ctx, err)
		if reason == "output_oversized" || len(output) > opts.MaxOutput {
			return ompProbeResult{reason: "output_oversized"}
		}
		return ompProbeResult{output: output, reason: reason}
	}
	if len(output) > opts.MaxOutput {
		return ompProbeResult{reason: "output_oversized"}
	}
	return ompProbeResult{output: output}
}

func classifyOMPProbeError(ctx context.Context, err error) string {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "timeout"
	}
	if isOMPOutputLimitError(err) {
		return "output_oversized"
	}
	return "exit_nonzero"
}
