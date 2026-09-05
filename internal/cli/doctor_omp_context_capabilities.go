package cli

import "github.com/insajin/autopus-adk/pkg/config"

func ompContextDoctorCapabilities(input ompContextDoctorInput, receiptUsable bool) []ompContextDoctorCapability {
	receipt := input.Receipt.Receipt
	active := input.RequestedHistoryMode == config.OMPContextHistoryActive
	memory := input.RequestedMemoryMode == config.OMPContextMemoryShadow
	persistence := receiptUsable && ompContextDoctorPersistenceProved(input.RuntimeRootPolicy, receipt)
	cleanup := receiptUsable && receipt.Cleanup.Attempted && receipt.Cleanup.Verified &&
		receipt.Capabilities.CleanupReadback && receipt.ArtifactCounts.AfterCleanup == 0 && receipt.Cleanup.UserRootAccessCount == 0
	rows := []ompContextDoctorCapability{
		ompContextDoctorCapabilityRow("identity.version", input.Current.IdentityVerified, true, "version_verified", "identity_unverified"),
		ompContextDoctorCapabilityRow("config.compaction_schema", input.Current.CompactionSchema, true, "config_schema_verified", "config_schema_unproved"),
		ompContextDoctorCapabilityRow("config.memory_schema", input.Current.MemorySchema, true, "config_schema_verified", "config_schema_unproved"),
		ompContextDoctorCapabilityRow("config.overlay_readback", input.Current.OverlayReadback, true, "overlay_readback_verified", "overlay_readback_unproved"),
		ompContextDoctorCapabilityRow("lifecycle.pre_compaction", receiptUsable && receipt.Capabilities.PreCompactionEvent, active, "installed_lifecycle_proved", "installed_lifecycle_unproved"),
		ompContextDoctorCapabilityRow("lifecycle.post_compaction", receiptUsable && receipt.Capabilities.PostCompactionEvent, active, "installed_lifecycle_proved", "installed_lifecycle_unproved"),
		ompContextDoctorCapabilityRow("admission.canonical_reinjection", receiptUsable && receipt.Capabilities.CanonicalInjection, active, "installed_lifecycle_proved", "installed_lifecycle_unproved"),
		ompContextDoctorCapabilityRow("admission.blocking", receiptUsable && receipt.Capabilities.AdmissionBlocking, active, "installed_lifecycle_proved", "installed_lifecycle_unproved"),
		ompContextDoctorCapabilityRow("persistence.no_session", persistence, active, ompContextDoctorPersistenceReason(input.RuntimeRootPolicy), "persistence_control_unproved"),
		ompContextDoctorCapabilityRow("artifact.cleanup", cleanup, active, "cleanup_verified", "cleanup_unproved"),
		ompContextDoctorCapabilityRow("memory.interception", receiptUsable && receipt.Capabilities.MemoryInterception, memory, "memory_interception_proved", "memory_interception_unproved"),
		// The probe rows above all pass on omp/18.1.x, and the compaction
		// benefit is still absent: a standalone cohort measured 0 basis points
		// of median context reduction there against 2000 required, while the
		// pinned omp/17.2.7 cleared the floor on the identical workload.
		//
		// Reporting only what the handshake proves would tell an operator that
		// context compaction is healthy while it silently does nothing. This row
		// is advisory rather than required, because a version this repository
		// has not measured is unknown, not broken.
		// Built inline rather than through ompContextDoctorCapabilityRow: that
		// helper replaces the reason with "not_requested" for advisory rows,
		// which would throw away the only thing this row exists to say.
		{
			ID:        "compaction.measured_reduction",
			Supported: ompContextReductionMeasuredGood(input.Current.Version),
			Required:  false,
			Reason:    ompContextReductionReason(input.Current.Version),
		},
	}
	sortOMPContextDoctorCapabilities(rows)
	return rows
}

func ompContextDoctorCapabilityRow(id string, supported, required bool, passReason, failReason string) ompContextDoctorCapability {
	reason := failReason
	if supported {
		reason = passReason
	} else if !required {
		reason = "not_requested"
	}
	return ompContextDoctorCapability{ID: id, Supported: supported, Required: required, Reason: reason}
}

func ompContextDoctorPersistenceReason(policy string) string {
	if policy == config.OMPContextRuntimeNoSession {
		return "no_session_proved"
	}
	return "isolated_task_owned_proved"
}
