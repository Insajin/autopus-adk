package evidence

import "strings"

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-QAMESH-001: QAMESH manifest schema is shared by CLI, browser, and desktop evidence producers.
// @AX:REASON: Changing schema_version or JSON field contracts breaks cross-surface evidence ingestion and feedback generation.
const (
	SchemaVersionV1 = "qamesh.evidence.v1"
	SchemaVersionV2 = "qamesh.evidence.v2"
	SchemaVersion   = SchemaVersionV1
)

type Manifest struct {
	SchemaVersion       string          `json:"schema_version"`
	QAResultID          string          `json:"qa_result_id"`
	Surface             string          `json:"surface"`
	Lane                string          `json:"lane"`
	ScenarioRef         string          `json:"scenario_ref"`
	Runner              Runner          `json:"runner"`
	Status              string          `json:"status"`
	StartedAt           string          `json:"started_at"`
	EndedAt             string          `json:"ended_at"`
	DurationMS          int64           `json:"duration_ms"`
	Artifacts           []ArtifactRef   `json:"artifacts"`
	OracleResults       OracleResults   `json:"oracle_results"`
	RedactionStatus     RedactionStatus `json:"redaction_status"`
	SourceRefs          SourceRefs      `json:"source_refs"`
	RepairPromptRef     string          `json:"repair_prompt_ref,omitempty"`
	RetentionClass      string          `json:"retention_class"`
	ReproductionCommand string          `json:"reproduction_command,omitempty"`
}

type Runner struct {
	Name    string `json:"name"`
	Command string `json:"command,omitempty"`
	Version string `json:"version,omitempty"`
}

type ArtifactRef struct {
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	Publishable bool   `json:"publishable"`
	Redaction   string `json:"redaction"`
}

type OracleResults struct {
	A11y    *A11yOracle    `json:"a11y,omitempty"`
	Desktop *DesktopOracle `json:"desktop,omitempty"`
	Checks  []CheckResult  `json:"checks,omitempty"`
}

type CheckResult struct {
	ID             string   `json:"id"`
	Type           string   `json:"type"`
	Status         string   `json:"status"`
	Expected       string   `json:"expected,omitempty"`
	Actual         string   `json:"actual,omitempty"`
	ArtifactRefs   []string `json:"artifact_refs,omitempty"`
	FailureSummary string   `json:"failure_summary,omitempty"`
}

type A11yOracle struct {
	CriticalCount int      `json:"critical_count"`
	SeriousCount  int      `json:"serious_count,omitempty"`
	FailedTargets []string `json:"failed_targets"`
}

type DesktopOracle struct {
	TimeoutClassification string `json:"timeout_classification,omitempty"`
}

type RedactionStatus struct {
	Status   string    `json:"status"`
	Findings []Finding `json:"findings,omitempty"`
}

type SourceRefs struct {
	SourceSpec       string         `json:"source_spec,omitempty"`
	AcceptanceRefs   []string       `json:"acceptance_refs,omitempty"`
	OwnedPaths       []string       `json:"owned_paths,omitempty"`
	DoNotModifyPaths []string       `json:"do_not_modify_paths,omitempty"`
	JourneyID        string         `json:"journey_id,omitempty"`
	StepID           string         `json:"step_id,omitempty"`
	Adapter          string         `json:"adapter,omitempty"`
	Mobile           *MobileRefs    `json:"mobile,omitempty"`
	OracleThresholds map[string]any `json:"oracle_thresholds,omitempty"`
}

type MobileRefs struct {
	FlowID            string `json:"flow_id"`
	AppArtifactDigest string `json:"app_artifact_digest"`
	DeviceRef         string `json:"device_ref"`
}

type LocatorContract struct {
	Strategy       string `json:"strategy"`
	Value          string `json:"value"`
	FallbackReason string `json:"fallback_reason,omitempty"`
	StableTestID   string `json:"stable_test_id,omitempty"`
}

type LocatorValidationResult struct {
	Accepted               []string `json:"accepted"`
	Rejected               []string `json:"rejected"`
	FallbackReasonRequired bool     `json:"fallback_reason_required"`
	StableTestIDRequired   bool     `json:"stable_test_id_required"`
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-QAMESH-001: locator validation keeps deterministic QA selectors accessibility-first.
// @AX:REASON: Browser evidence producers and acceptance checks rely on rejecting non-semantic selectors unless fallback metadata is supplied.
func ValidateLocatorConvention(locators []LocatorContract) LocatorValidationResult {
	result := LocatorValidationResult{Accepted: []string{}, Rejected: []string{}}
	for _, locator := range locators {
		strategy := strings.ToLower(strings.TrimSpace(locator.Strategy))
		switch strategy {
		case "role", "label", "text", "alt", "alttext":
			result.Accepted = append(result.Accepted, strategy)
		case "testid", "test-id":
			if locator.FallbackReason != "" && locator.StableTestID != "" {
				result.Accepted = append(result.Accepted, strategy)
			} else {
				result.Rejected = append(result.Rejected, strategy)
				result.FallbackReasonRequired = true
				result.StableTestIDRequired = true
			}
		default:
			result.Rejected = append(result.Rejected, strategy)
			result.FallbackReasonRequired = true
			result.StableTestIDRequired = true
		}
	}
	return result
}
