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
	"config.intent_tracing",
	"rpc.protocol_v2",
	"rpc.state",
	"rpc.commands",
	"rpc.dump_tools_pre_intent",
	"rpc.subagent_subscription",
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

// ProbeOMPReadiness inspects only local metadata, configuration, and RPC state.
// It never selects a model, sends a prompt, starts a provider, or writes through
// an agent tool.
// @AX:ANCHOR [AUTO]: preserve this provider-free readiness API and its semantic report boundary.
// @AX:REASON [AUTO]: doctor and external adapter consumers depend on stable capability IDs without model selection, prompt submission, provider startup, or agent-tool writes.
func ProbeOMPReadiness(ctx context.Context, opts OMPReadinessOptions) OMPReadinessReport {
	opts = normalizeOMPReadinessOptions(opts)
	report := OMPReadinessReport{
		Executable:    filepath.Base(opts.Executable),
		Capabilities:  make([]OMPCapabilityResult, 0, len(ompReadinessCapabilityIDs)),
		CatalogReason: "not_probed_provider_free",
	}

	sandbox, err := createOMPReadinessSandbox()
	if err != nil {
		return ompIdentityFailureReport(report, nil, "output_invalid")
	}
	defer func() { _ = os.RemoveAll(sandbox) }()
	if runner, ok := opts.Runner.(commandOMPProbeRunner); ok {
		runner, resolved := configureOMPProbeRunner(runner, opts.Executable, sandbox, opts.Root)
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
		return ompIdentityFailureReport(report, nil, "identity_unverified")
	}

	help := runOMPProbe(ctx, opts, "--help")
	report.Capabilities = append(report.Capabilities,
		evaluateOMPHelpCapability("launch.rpc", help, "--mode", "rpc"),
		evaluateOMPHelpCapability("launch.no_session", help, "--no-session"),
		evaluateOMPHelpCapability("launch.cwd", help, "--cwd"),
	)
	intent := runOMPProbe(ctx, opts, "config", "get", "tools.intentTracing", "--json")
	report.Capabilities = append(report.Capabilities, evaluateOMPIntentTracing(intent))

	var rpcResult ompProbeResult
	if runner, ok := opts.Runner.(commandOMPProbeRunner); ok {
		rpcResult = runOMPReadinessProviderFreeProbe(ctx, opts, runner)
	} else {
		rpcResult = runOMPProbe(ctx, opts, ompProviderFreeRPCArgs(opts.Root)...)
	}
	report.Capabilities = append(report.Capabilities, evaluateOMPProviderFreeRPC(rpcResult)...)
	return report
}

func ompIdentityFailureReport(
	report OMPReadinessReport,
	_ []string,
	reason string,
) OMPReadinessReport {
	for _, id := range ompReadinessCapabilityIDs[len(report.Capabilities):] {
		report.Capabilities = append(report.Capabilities, OMPCapabilityResult{ID: id, Reason: reason})
	}
	return report
}

func normalizeOMPReadinessOptions(opts OMPReadinessOptions) OMPReadinessOptions {
	if opts.Executable == "" {
		opts.Executable = cliBinary
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Second
	}
	// @AX:NOTE [AUTO]: 256 KiB bounds the complete native RPC transcript while allowing all provider-free capability frames.
	if opts.MaxOutput <= 0 {
		opts.MaxOutput = 256 * 1024
	}
	if opts.Root == "" {
		opts.Root = "."
	}
	if opts.Runner == nil {
		opts.Runner = commandOMPProbeRunner{maxOutput: opts.MaxOutput}
	}
	return opts
}

func createOMPReadinessSandbox() (string, error) {
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
	return dir, nil
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
