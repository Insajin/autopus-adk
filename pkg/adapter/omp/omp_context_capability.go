package omp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/detect"
)

var ompContextCapabilityIDs = []string{
	"identity.version", "config.compaction_schema", "config.memory_schema", "config.overlay_readback",
	"lifecycle.pre_compaction", "lifecycle.post_compaction", "admission.canonical_reinjection",
	"admission.blocking", "persistence.no_session", "artifact.cleanup", "memory.interception",
}

type OMPContextCapabilityRunner interface {
	Run(ctx context.Context, executable string, args ...string) ([]byte, error)
}

type OMPContextCapabilityOptions struct {
	Executable           string
	Runner               OMPContextCapabilityRunner
	Timeout              time.Duration
	MaxOutput            int
	RequestedHistoryMode string
	RequestedMemoryMode  string
}

type OMPContextCapability struct {
	ID        string `json:"id"`
	Supported bool   `json:"supported"`
	Reason    string `json:"reason"`
}

type OMPContextLifecycleEvidence struct {
	PreCompaction        bool `json:"pre_compaction"`
	PostCompaction       bool `json:"post_compaction"`
	CanonicalReinjection bool `json:"canonical_reinjection"`
	AdmissionBlocking    bool `json:"admission_blocking"`
	CleanupVerified      bool `json:"cleanup_verified"`
	MemoryIntercepted    bool `json:"memory_intercepted"`
}

type OMPContextCapabilityReport struct {
	Version              string                 `json:"version,omitempty"`
	Capabilities         []OMPContextCapability `json:"capabilities"`
	ActiveEligible       bool                   `json:"active_eligible"`
	EffectiveHistoryMode string                 `json:"effective_history_mode"`
	EffectiveMemoryMode  string                 `json:"effective_memory_mode"`
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-003: public boundary for observed OMP context-lifecycle capabilities.
// @AX:REASON [AUTO]: external CLI evidence is reduced to stable semantic capability IDs before doctor or runtime admission.
// @AX:WARN [AUTO]: context capability probing has cyclomatic complexity 17.
// @AX:REASON [AUTO]: gocyclo reports 17 across identity, help, lifecycle, config, and effective-mode evidence classification.
func ProbeOMPContextCapabilities(ctx context.Context, opts OMPContextCapabilityOptions) OMPContextCapabilityReport {
	opts = normalizeOMPContextCapabilityOptions(opts)
	report := OMPContextCapabilityReport{Capabilities: make([]OMPContextCapability, 0, len(ompContextCapabilityIDs))}
	version, versionReason := runOMPContextCapabilityProbe(ctx, opts, "--version")
	identity := strings.TrimSpace(string(version))
	identityOK := versionReason == "" && detect.OMPVersionMatchesIdentity(identity)
	report.Capabilities = append(report.Capabilities, ompContextCapabilityResult("identity.version", identityOK, versionReason, "identity_unverified"))
	if identityOK {
		report.Version = identity
	}
	compaction, compactionReason := runOMPContextCapabilityProbe(ctx, opts, "config", "get", "compaction", "--json")
	memory, memoryReason := runOMPContextCapabilityProbe(ctx, opts, "config", "get", "memory", "--json")
	compactionOK := compactionReason == "" && json.Valid(compaction)
	memoryOK := memoryReason == "" && json.Valid(memory)
	report.Capabilities = append(report.Capabilities,
		ompContextCapabilityResult("config.compaction_schema", compactionOK, compactionReason, "schema_unavailable"),
		ompContextCapabilityResult("config.memory_schema", memoryOK, memoryReason, "schema_unavailable"),
		ompContextCapabilityResult("config.overlay_readback", compactionOK && memoryOK, firstOMPContextReason(compactionReason, memoryReason), "readback_unverified"),
	)
	help, helpReason := runOMPContextCapabilityProbe(ctx, opts, "--help")
	noSession := helpReason == "" && bytes.Contains(help, []byte("--no-session"))
	rpc, rpcReason := runOMPContextCapabilityProbe(ctx, opts, "--mode", "rpc", "--no-session")
	evidence := ParseOMPContextLifecycleEvidence(rpc)
	report.Capabilities = append(report.Capabilities,
		ompContextEventCapability("lifecycle.pre_compaction", evidence.PreCompaction, rpcReason),
		ompContextEventCapability("lifecycle.post_compaction", evidence.PostCompaction, rpcReason),
		ompContextEventCapability("admission.canonical_reinjection", evidence.CanonicalReinjection, rpcReason),
		ompContextEventCapability("admission.blocking", evidence.AdmissionBlocking, rpcReason),
		ompContextCapabilityResult("persistence.no_session", noSession, helpReason, "flag_missing"),
		ompContextEventCapability("artifact.cleanup", evidence.CleanupVerified, rpcReason),
		ompContextEventCapability("memory.interception", evidence.MemoryIntercepted, rpcReason),
	)
	report.ActiveEligible = identityOK && compactionOK && memoryOK && noSession && evidence.PreCompaction &&
		evidence.PostCompaction && evidence.CanonicalReinjection && evidence.AdmissionBlocking && evidence.CleanupVerified
	report.EffectiveHistoryMode = effectiveOMPContextHistoryMode(opts.RequestedHistoryMode, report.ActiveEligible)
	report.EffectiveMemoryMode = effectiveOMPContextMemoryMode(opts.RequestedMemoryMode, identityOK && memoryOK && evidence.MemoryIntercepted)
	return report
}

func ParseOMPContextLifecycleEvidence(data []byte) OMPContextLifecycleEvidence {
	var evidence OMPContextLifecycleEvidence
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		var event struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(line, &event) != nil {
			continue
		}
		switch event.Type {
		case "auto_compaction_start":
			evidence.PreCompaction = true
		case "auto_compaction_end":
			evidence.PostCompaction = true
		case "autopus_context_reinjection_ready":
			evidence.CanonicalReinjection = true
		case "autopus_context_admission_blocked":
			evidence.AdmissionBlocking = true
		case "autopus_context_cleanup_verified":
			evidence.CleanupVerified = true
		case "autopus_context_memory_intercepted":
			evidence.MemoryIntercepted = true
		}
	}
	return evidence
}

func normalizeOMPContextCapabilityOptions(opts OMPContextCapabilityOptions) OMPContextCapabilityOptions {
	if opts.Executable == "" {
		opts.Executable = "omp"
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 3 * time.Second
	}
	if opts.MaxOutput <= 0 {
		opts.MaxOutput = 64 * 1024
	}
	return opts
}

func runOMPContextCapabilityProbe(parent context.Context, opts OMPContextCapabilityOptions, args ...string) ([]byte, string) {
	if opts.Runner == nil {
		return nil, "runner_missing"
	}
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, opts.Timeout)
	defer cancel()
	output, err := opts.Runner.Run(ctx, opts.Executable, args...)
	if len(output) > opts.MaxOutput {
		return nil, "output_oversized"
	}
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, "timeout"
		}
		return nil, "exit_nonzero"
	}
	return output, ""
}
