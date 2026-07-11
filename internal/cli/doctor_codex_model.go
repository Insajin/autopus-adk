package cli

import (
	"fmt"
	"io"

	"github.com/insajin/autopus-adk/internal/cli/tui"
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/config"
)

const doctorCodexSupervisorPolicyCheckID = "doctor.codex.supervisor_model_policy"

type doctorCodexModelDiagnosis struct {
	status      string
	detail      string
	warningCode string
}

func checkCodexModelOwnershipText(w io.Writer, dir string, cfg *config.HarnessConfig) bool {
	tui.SectionHeader(w, "Codex Model Ownership")
	diagnosis := diagnoseCodexModelOwnership(dir, cfg)
	switch diagnosis.status {
	case "warn":
		tui.SKIP(w, diagnosis.detail)
		return false
	case "skip":
		tui.Info(w, diagnosis.detail)
		return true
	default:
		tui.OK(w, diagnosis.detail)
		return true
	}
}

func (r *doctorJSONReport) collectCodexModelOwnershipCheck(dir string, cfg *config.HarnessConfig) {
	diagnosis := diagnoseCodexModelOwnership(dir, cfg)
	severity := "info"
	if diagnosis.status == "warn" {
		severity = "warning"
		r.status = jsonStatusWarn
		if diagnosis.warningCode != "" {
			r.warnings = append(r.warnings, jsonMessage{
				Code:    diagnosis.warningCode,
				Message: diagnosis.detail,
			})
		}
	}
	r.checks = append(r.checks, jsonCheck{
		ID:       doctorCodexSupervisorPolicyCheckID,
		Severity: severity,
		Status:   diagnosis.status,
		Detail:   diagnosis.detail,
	})
}

func diagnoseCodexModelOwnership(dir string, cfg *config.HarnessConfig) doctorCodexModelDiagnosis {
	if cfg == nil || !containsPlatform(cfg.Platforms, "codex") {
		return doctorCodexModelDiagnosis{
			status: "skip",
			detail: "Codex project model ownership check is not applicable",
		}
	}
	if cfg.Quality.SupervisorModelPolicy != "" {
		if cfg.Quality.SupervisorModelPolicy == config.SupervisorModelPolicyInherit {
			ownership, err := codex.InspectSupervisorOverrideOwnership(dir)
			if err != nil {
				return doctorCodexInspectionError(err)
			}
			if ownership.HasManagedOverride {
				return doctorCodexModelDiagnosis{
					status: "warn",
					detail: "supervisor model policy is inherit but a managed project model/effort override is still present; " +
						"run 'auto update', then start a new Codex session",
					warningCode: "codex_inherit_policy_not_applied",
				}
			}
		}
		return doctorCodexModelDiagnosis{
			status: "pass",
			detail: fmt.Sprintf("supervisor model policy: %s", cfg.Quality.SupervisorModelPolicy),
		}
	}

	inspection, err := codex.InspectLegacySupervisorModel(dir, cfg)
	if err != nil {
		return doctorCodexInspectionError(err)
	}

	if inspection.Migratable {
		return doctorCodexModelDiagnosis{
			status: "warn",
			detail: "legacy Autopus-managed project model/effort overrides the user's Codex default; " +
				"run 'auto update', then start a new Codex session",
			warningCode: "legacy_codex_model_shadowing",
		}
	}
	if inspection.HasProjectOverride && isAmbiguousCodexModelReason(inspection.Reason) {
		return doctorCodexModelDiagnosis{
			status: "warn",
			detail: "project model/effort override has ambiguous legacy ownership; choose " +
				"'auto quality supervisor inherit --apply' or " +
				"'auto quality supervisor quality --apply'",
			warningCode: "ambiguous_codex_model_ownership",
		}
	}
	if inspection.UserOwned && inspection.HasProjectOverride {
		return doctorCodexModelDiagnosis{
			status: "pass",
			detail: "project model/effort override is user-owned",
		}
	}
	if inspection.Reason == codex.LegacySupervisorReasonConfigMissing {
		return doctorCodexModelDiagnosis{
			status: "skip",
			detail: "Codex project model ownership check is not applicable",
		}
	}
	return doctorCodexModelDiagnosis{
		status: "pass",
		detail: "no legacy project model/effort override detected",
	}
}

func doctorCodexInspectionError(err error) doctorCodexModelDiagnosis {
	return doctorCodexModelDiagnosis{
		status:      "warn",
		detail:      fmt.Sprintf("Codex model ownership inspection failed: %v", err),
		warningCode: "codex_model_inspection_failed",
	}
}

func isAmbiguousCodexModelReason(reason string) bool {
	switch reason {
	case codex.LegacySupervisorReasonGeneratedHeaderMissing,
		codex.LegacySupervisorReasonManifestMissing,
		codex.LegacySupervisorReasonManifestEntryMissing,
		codex.LegacySupervisorReasonChecksumDrift,
		codex.LegacySupervisorReasonManifestPolicyMismatch:
		return true
	default:
		return false
	}
}
