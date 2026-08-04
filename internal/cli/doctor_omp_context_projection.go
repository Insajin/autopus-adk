package cli

import (
	"fmt"
	"sort"
	"strings"
)

func appendOMPContextDoctorChecks(checks []jsonCheck, report ompContextDoctorReport) []jsonCheck {
	if !report.Enabled {
		return checks
	}
	checks = append(checks, projectOMPContextDoctorChecks(report)...)
	sort.SliceStable(checks, func(i, j int) bool { return checks[i].ID < checks[j].ID })
	return checks
}

func projectOMPContextDoctorChecks(report ompContextDoctorReport) []jsonCheck {
	if !report.Enabled {
		return nil
	}
	severity, status := ompContextDoctorLevel(report.Status)
	checks := []jsonCheck{
		{
			ID: "doctor.platform.omp.context.mode", Severity: severity, Status: status,
			Detail: fmt.Sprintf("context status=%s reason=%s profile=%s requested_history=%s effective_history=%s requested_memory=%s effective_memory=%s",
				ompContextDoctorSafeToken(report.Status), ompContextDoctorSafeReason(report.Reason),
				ompContextDoctorSafeToken(report.Profile), ompContextDoctorSafeToken(report.RequestedHistoryMode),
				ompContextDoctorSafeToken(report.EffectiveHistoryMode), ompContextDoctorSafeToken(report.RequestedMemoryMode),
				ompContextDoctorSafeToken(report.EffectiveMemoryMode)),
		},
		{
			ID: "doctor.platform.omp.context.receipt", Severity: severity, Status: status,
			Detail: fmt.Sprintf("receipt=%s freshness=%s identity_current=%t version=%s",
				ompContextDoctorSafeToken(report.ReceiptStatus), ompContextDoctorSafeToken(report.ReceiptFreshness),
				report.IdentityCurrent, ompContextDoctorSafeVersion(report.CurrentVersion)),
		},
		{
			ID: "doctor.platform.omp.context.lifecycle", Severity: severity, Status: status,
			Detail: fmt.Sprintf("checkpoint=%t rehydrated=%t exact_match=%t",
				report.Checkpoint, report.Rehydrated, report.ExactMatch),
		},
		{
			ID: "doctor.platform.omp.context.fallback", Severity: severity, Status: status,
			Detail: fmt.Sprintf("fallback=%s reason=%s",
				ompContextDoctorSafeToken(report.FallbackMode), ompContextDoctorSafeReason(report.FallbackReason)),
		},
		{
			ID: "doctor.platform.omp.context.artifact", Severity: severity, Status: status,
			Detail: fmt.Sprintf("cleanup_count=%d root_class=%s",
				safeOMPContextDoctorCount(report.ArtifactCleanupCount), ompContextDoctorSafeToken(report.RootClass)),
		},
		{
			ID: "doctor.platform.omp.context.memory", Severity: severity, Status: status,
			Detail: fmt.Sprintf("interception=%t provenance=%t active_injection=%t",
				report.MemoryInterception, report.MemoryProvenance, report.MemoryActiveInjection),
		},
	}
	for _, capability := range report.Capabilities {
		capSeverity, capStatus := "info", "pass"
		if capability.Required && !capability.Supported {
			capSeverity, capStatus = ompContextDoctorLevel(report.Status)
		}
		checks = append(checks, jsonCheck{
			ID:       "doctor.platform.omp.context.capability." + ompContextDoctorSafeToken(capability.ID),
			Severity: capSeverity, Status: capStatus,
			Detail: fmt.Sprintf("capability=%s supported=%t required=%t reason=%s",
				ompContextDoctorSafeToken(capability.ID), capability.Supported, capability.Required,
				ompContextDoctorSafeReason(capability.Reason)),
		})
	}
	sort.SliceStable(checks, func(i, j int) bool { return checks[i].ID < checks[j].ID })
	return checks
}

func ompContextDoctorLevel(value string) (string, string) {
	switch value {
	case "supported":
		return "info", "pass"
	case "degraded":
		return "warning", "warn"
	default:
		return "error", "fail"
	}
}

func ompContextDoctorSafeToken(value string) string {
	value = ompDoctorToken(value)
	lower := strings.ToLower(value)
	for _, marker := range []string{"secret", "password", "bearer", "api_key", "api-key", "provider"} {
		if strings.Contains(lower, marker) {
			return "redacted"
		}
	}
	return value
}

func ompContextDoctorSafeVersion(value string) string {
	if ompDoctorVersion.MatchString(strings.TrimSpace(value)) {
		return strings.TrimSpace(value)
	}
	return "redacted"
}

func ompContextDoctorSafeReason(value string) string {
	allowed := map[string]bool{
		"fresh": true, "not_applicable": true, "not_requested": true,
		"identity_unverified": true, "config_readback_unproved": true, "config_schema_unproved": true,
		"config_schema_verified": true, "overlay_readback_verified": true, "overlay_readback_unproved": true,
		"installed_lifecycle_proved": true, "installed_lifecycle_unproved": true,
		"receipt_missing": true, "receipt_invalid": true, "receipt_stale": true, "version_stale": true,
		"mode_readback_mismatch": true, "persistence_control_unproved": true,
		"no_session_proved": true, "isolated_task_owned_proved": true,
		"cleanup_verified": true, "cleanup_unproved": true,
		"memory_interception_proved": true, "memory_interception_unproved": true,
		"memory_provenance_unproved": true, "verified": true,
		"required-source-changed": true, "ephemeral-state-unavailable": true,
		"runtime-cleanup-failed": true, "rehydration-verification-failed": true,
	}
	if allowed[strings.TrimSpace(value)] {
		return strings.TrimSpace(value)
	}
	return "redacted"
}

func safeOMPContextDoctorCount(value int) int {
	if value < 0 || value > 1_000_000 {
		return -1
	}
	return value
}
