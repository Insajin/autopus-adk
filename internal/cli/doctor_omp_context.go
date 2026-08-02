package cli

import (
	"sort"

	"github.com/insajin/autopus-adk/pkg/config"
)

type ompContextCurrentProbe struct {
	Version          string
	Reason           string
	IdentityVerified bool
	ConfigListSchema bool
	CompactionSchema bool
	MemorySchema     bool
	OverlayReadback  bool
}

type ompContextDoctorReceiptState struct {
	Status    string
	Freshness string
	Receipt   WorkflowContextRuntimeReceipt
}

type ompContextDoctorInput struct {
	Enabled              bool
	Profile              string
	RequestedHistoryMode string
	RequestedMemoryMode  string
	FallbackMode         string
	RuntimeRootPolicy    string
	Current              ompContextCurrentProbe
	Receipt              ompContextDoctorReceiptState
}

type ompContextDoctorCapability struct {
	ID        string
	Supported bool
	Required  bool
	Reason    string
}

type ompContextDoctorReport struct {
	Enabled               bool
	Profile               string
	Status                string
	Reason                string
	RequestedHistoryMode  string
	EffectiveHistoryMode  string
	RequestedMemoryMode   string
	EffectiveMemoryMode   string
	FallbackMode          string
	FallbackReason        string
	ReceiptStatus         string
	ReceiptFreshness      string
	CurrentVersion        string
	IdentityCurrent       bool
	Checkpoint            bool
	Rehydrated            bool
	ExactMatch            bool
	ArtifactCleanupCount  int
	RootClass             string
	MemoryInterception    bool
	MemoryProvenance      bool
	MemoryActiveInjection bool
	Capabilities          []ompContextDoctorCapability
}

// @AX:WARN [AUTO]: context doctor synthesis has cyclomatic complexity 27.
// @AX:REASON [AUTO]: gocyclo reports 27 across opt-in, capability, receipt, mode, and promotion-readiness projections.
func checkOMPContextDoctor(input ompContextDoctorInput) ompContextDoctorReport {
	if !input.Enabled {
		return ompContextDoctorReport{}
	}
	receipt := input.Receipt.Receipt
	report := ompContextDoctorReport{
		Enabled: true, Profile: ompContextDoctorSafeToken(input.Profile), Status: "supported", Reason: "fresh",
		RequestedHistoryMode: input.RequestedHistoryMode, EffectiveHistoryMode: input.RequestedHistoryMode,
		RequestedMemoryMode: input.RequestedMemoryMode, EffectiveMemoryMode: input.RequestedMemoryMode,
		FallbackMode: input.FallbackMode, FallbackReason: ompContextDoctorSafeReason(receipt.Fallback.Reason),
		ReceiptStatus: input.Receipt.Status, ReceiptFreshness: input.Receipt.Freshness,
		CurrentVersion:       ompContextDoctorSafeVersion(input.Current.Version),
		IdentityCurrent:      input.Current.IdentityVerified && receipt.Capabilities.Version == input.Current.Version,
		ArtifactCleanupCount: receipt.ArtifactCounts.AfterCleanup, RootClass: receipt.RootClass,
	}
	if report.FallbackReason == "redacted" && receipt.Fallback.Reason == "" {
		report.FallbackReason = "not_applicable"
	}
	currentReceipt := input.Receipt.Status == "valid" && input.Receipt.Freshness == "fresh" &&
		report.IdentityCurrent && receipt.Capabilities.ExecutableIdentity && receipt.Capabilities.AuthNoneLoopback
	receiptUsable := currentReceipt && receipt.Capabilities.ProbeSource == "installed-canary"
	report.Checkpoint = receiptUsable && orderedOMPContextPhases(receipt.PhaseSequence, "checkpointed")
	report.Rehydrated = receiptUsable && orderedOMPContextPhases(receipt.PhaseSequence, "checkpointed", "compacted", "rehydrated")
	report.ExactMatch = receiptUsable && receipt.ExactMatch
	report.MemoryInterception = currentReceipt && receipt.Capabilities.MemoryInterception
	report.MemoryProvenance = report.MemoryInterception && receiptUsable && !receipt.Capabilities.CheckedAt.IsZero()
	report.MemoryActiveInjection = false
	report.Capabilities = ompContextDoctorCapabilities(input, receiptUsable)

	if reason := ompContextDoctorCurrentFailure(input.Current); reason != "" {
		report.Status, report.Reason = "blocked", reason
		report.EffectiveHistoryMode = ompContextDoctorHistoryFallback(input.RequestedHistoryMode)
		report.EffectiveMemoryMode = config.OMPContextMemoryOff
		return report
	}
	if input.RequestedHistoryMode == config.OMPContextHistoryActive {
		if reason := ompContextDoctorActiveFailure(input, report, receiptUsable); reason != "" {
			report.Status, report.Reason = "blocked", reason
			report.EffectiveHistoryMode = config.OMPContextHistoryShadow
		}
	} else if input.RequestedHistoryMode == config.OMPContextHistoryShadow && !receiptUsable {
		report.Status, report.Reason = "degraded", ompContextDoctorReceiptReason(input.Receipt)
	}
	if input.RequestedMemoryMode == config.OMPContextMemoryShadow && !report.MemoryInterception {
		if report.Status == "supported" {
			report.Status, report.Reason = "degraded", "memory_interception_unproved"
		}
		report.EffectiveMemoryMode = config.OMPContextMemoryOff
	} else if input.RequestedMemoryMode == config.OMPContextMemoryShadow && !report.MemoryProvenance {
		if report.Status == "supported" {
			report.Status, report.Reason = "degraded", "memory_provenance_unproved"
		}
		report.EffectiveMemoryMode = config.OMPContextMemoryOff
	}
	return report
}

func ompContextDoctorCurrentFailure(current ompContextCurrentProbe) string {
	if !current.IdentityVerified {
		return "identity_unverified"
	}
	if current.Reason == "config_readback_unproved" {
		return current.Reason
	}
	if !current.ConfigListSchema || !current.CompactionSchema || !current.MemorySchema {
		return "config_schema_unproved"
	}
	if !current.OverlayReadback {
		return "overlay_readback_unproved"
	}
	return ""
}

// @AX:WARN [AUTO]: active-mode failure classification has cyclomatic complexity 26.
// @AX:REASON [AUTO]: gocyclo reports 26 because version, capability, receipt, promotion, and effective-mode failures are ordered by precedence.
func ompContextDoctorActiveFailure(input ompContextDoctorInput, report ompContextDoctorReport, receiptUsable bool) string {
	if input.Receipt.Status == "valid" && input.Receipt.Freshness == "fresh" &&
		input.Receipt.Receipt.Capabilities.Version != input.Current.Version {
		return "version_stale"
	}
	if !receiptUsable {
		return ompContextDoctorReceiptReason(input.Receipt)
	}
	receipt := input.Receipt.Receipt
	if receipt.Mode.RequestedHistoryMode != input.RequestedHistoryMode ||
		receipt.Mode.EffectiveHistoryMode != config.OMPContextHistoryActive ||
		receipt.Mode.EffectiveMemoryMode != input.RequestedMemoryMode {
		return "mode_readback_mismatch"
	}
	if !report.Checkpoint || !report.Rehydrated || !report.ExactMatch || receipt.Outcome != WorkflowContextOutcomeAdmitted {
		return "installed_lifecycle_unproved"
	}
	if !receipt.Capabilities.SettingsSchema || !receipt.Capabilities.OverlayReadback ||
		!receipt.Capabilities.PreCompactionEvent || !receipt.Capabilities.PostCompactionEvent ||
		!receipt.Capabilities.CanonicalInjection || !receipt.Capabilities.AdmissionBlocking {
		return "installed_lifecycle_unproved"
	}
	if !ompContextDoctorPersistenceProved(input.RuntimeRootPolicy, receipt) {
		return "persistence_control_unproved"
	}
	if !receipt.Cleanup.Attempted || !receipt.Cleanup.Verified || !receipt.Capabilities.CleanupReadback ||
		receipt.ArtifactCounts.AfterCleanup != 0 || receipt.Cleanup.UserRootAccessCount != 0 {
		return "cleanup_unproved"
	}
	if receipt.Mode.OverlayHash == "" || receipt.Mode.OverlayHash != receipt.Mode.ReadbackHash {
		return "overlay_readback_unproved"
	}
	return ""
}

func ompContextDoctorReceiptReason(state ompContextDoctorReceiptState) string {
	if state.Status == "missing" {
		return "receipt_missing"
	}
	if state.Status != "valid" {
		return "receipt_invalid"
	}
	if state.Freshness != "fresh" {
		return "receipt_stale"
	}
	return "installed_lifecycle_unproved"
}

func ompContextDoctorHistoryFallback(requested string) string {
	if requested == config.OMPContextHistoryActive {
		return config.OMPContextHistoryShadow
	}
	return requested
}

func orderedOMPContextPhases(actual []string, required ...string) bool {
	next := 0
	for _, phase := range actual {
		if next < len(required) && phase == required[next] {
			next++
		}
	}
	return next == len(required)
}

func ompContextDoctorPersistenceProved(policy string, receipt WorkflowContextRuntimeReceipt) bool {
	return policy == config.OMPContextRuntimeNoSession && receipt.RootClass == policy && receipt.Capabilities.NoSession ||
		policy == config.OMPContextRuntimeIsolatedTaskOwned && receipt.RootClass == policy && receipt.Capabilities.IsolatedTaskRoot
}

func sortOMPContextDoctorCapabilities(rows []ompContextDoctorCapability) {
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
}
