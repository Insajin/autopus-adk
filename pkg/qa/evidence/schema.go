package evidence

import (
	"fmt"
	"strings"
)

func isSupportedSchemaVersion(version string) bool {
	switch version {
	case SchemaVersionV1, SchemaVersionV2:
		return true
	default:
		return false
	}
}

func isSupportedSurface(version, surface string) bool {
	switch version {
	case SchemaVersionV1:
		return surface == "browser" || surface == "desktop"
	case SchemaVersionV2:
		switch surface {
		case "cli", "backend", "frontend", "desktop", "package", "custom", "multi":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func validateOracleResults(manifest Manifest) error {
	if manifest.SchemaVersion == SchemaVersionV1 {
		if manifest.OracleResults.A11y == nil && manifest.OracleResults.Desktop == nil {
			return fmt.Errorf("missing required field oracle_results")
		}
		return nil
	}
	if len(manifest.OracleResults.Checks) == 0 {
		return fmt.Errorf("missing required field oracle_results.checks")
	}
	for index, check := range manifest.OracleResults.Checks {
		if strings.TrimSpace(check.ID) == "" {
			return fmt.Errorf("missing required field oracle_results.checks[%d].id", index)
		}
		if strings.TrimSpace(check.Type) == "" {
			return fmt.Errorf("missing required field oracle_results.checks[%d].type", index)
		}
		switch check.Status {
		case "passed", "failed", "skipped", "blocked":
		default:
			return fmt.Errorf("unsupported oracle_results.checks[%d].status %q", index, check.Status)
		}
		if (check.Status == "failed" || check.Status == "blocked") && strings.TrimSpace(check.FailureSummary) == "" {
			return fmt.Errorf("missing required field oracle_results.checks[%d].failure_summary", index)
		}
	}
	return nil
}

func validateSourceRefs(manifest Manifest) error {
	if strings.TrimSpace(manifest.SourceRefs.SourceSpec) == "" {
		return fmt.Errorf("missing required field source_refs.source_spec")
	}
	if len(manifest.SourceRefs.AcceptanceRefs) == 0 {
		return fmt.Errorf("missing required field source_refs.acceptance_refs")
	}
	if manifest.SchemaVersion != SchemaVersionV2 {
		return nil
	}
	if strings.TrimSpace(manifest.SourceRefs.JourneyID) == "" {
		return fmt.Errorf("missing required field source_refs.journey_id")
	}
	if strings.TrimSpace(manifest.SourceRefs.StepID) == "" {
		return fmt.Errorf("missing required field source_refs.step_id")
	}
	if strings.TrimSpace(manifest.SourceRefs.Adapter) == "" {
		return fmt.Errorf("missing required field source_refs.adapter")
	}
	return nil
}

func hasCheckStatus(checks []CheckResult, status string) bool {
	for _, check := range checks {
		if check.Status == status {
			return true
		}
	}
	return false
}
