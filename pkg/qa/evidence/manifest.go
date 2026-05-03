package evidence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
	OracleThresholds map[string]any `json:"oracle_thresholds,omitempty"`
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

func LoadManifest(path string) (Manifest, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	return manifest, nil
}

func ResolveArtifactPaths(manifest Manifest, baseDir string) (Manifest, error) {
	if baseDir == "" {
		return manifest, nil
	}
	base, err := realAbsPath(baseDir)
	if err != nil {
		return Manifest{}, err
	}
	for index, artifact := range manifest.Artifacts {
		if artifact.Path != "" && !filepath.IsAbs(artifact.Path) {
			artifact.Path = filepath.Join(base, artifact.Path)
		}
		resolved, err := realAbsPath(artifact.Path)
		if err != nil {
			return Manifest{}, err
		}
		if !isPathWithin(resolved, base) {
			return Manifest{}, fmt.Errorf("artifact path outside evidence input root: %s", RedactText(artifact.Path))
		}
		manifest.Artifacts[index].Path = resolved
	}
	return manifest, nil
}

func (m Manifest) Validate() error {
	required := map[string]string{
		"schema_version":  m.SchemaVersion,
		"qa_result_id":    m.QAResultID,
		"surface":         m.Surface,
		"lane":            m.Lane,
		"scenario_ref":    m.ScenarioRef,
		"runner.name":     m.Runner.Name,
		"status":          m.Status,
		"started_at":      m.StartedAt,
		"ended_at":        m.EndedAt,
		"retention_class": m.RetentionClass,
	}
	for field, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("missing required field %s", field)
		}
	}
	if !isSupportedSchemaVersion(m.SchemaVersion) {
		return fmt.Errorf("unsupported schema_version %q", m.SchemaVersion)
	}
	if !isSupportedSurface(m.SchemaVersion, m.Surface) {
		return fmt.Errorf("unsupported surface %q", m.Surface)
	}
	switch m.Status {
	case "passed", "failed", "skipped", "blocked", "in_progress":
	default:
		return fmt.Errorf("unsupported status %q", m.Status)
	}
	if len(m.Artifacts) == 0 {
		return fmt.Errorf("missing required field artifacts")
	}
	if err := validateOracleResults(m); err != nil {
		return err
	}
	if err := validateSourceRefs(m); err != nil {
		return err
	}
	if strings.TrimSpace(m.RedactionStatus.Status) == "" {
		return fmt.Errorf("missing required field redaction_status.status")
	}
	if m.RedactionStatus.Status != "passed" {
		return fmt.Errorf("redaction_status.status must be passed before publication")
	}
	return validateSurfaceContract(m)
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-QAMESH-001: final manifest writing is the publish boundary for sanitized QA evidence.
// @AX:REASON: CLI evidence output, artifact copying, redaction checks, and feedback bundle generation depend on rejecting unsafe publishable artifacts here.
func WriteFinalManifest(manifest Manifest, outputDir string) (string, error) {
	manifest = NormalizeManifest(manifest)
	if err := manifest.Validate(); err != nil {
		return "", err
	}
	normalized := manifest
	normalized.Artifacts = make([]ArtifactRef, 0, len(manifest.Artifacts))
	parentDir := filepath.Dir(outputDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return "", err
	}
	tempDir, err := os.MkdirTemp(parentDir, "."+safePathSegment(filepath.Base(outputDir))+"-*")
	if err != nil {
		return "", err
	}
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.RemoveAll(tempDir)
		}
	}()
	for _, artifact := range manifest.Artifacts {
		copied, err := sanitizeArtifact(artifact, tempDir)
		if err != nil {
			return "", err
		}
		normalized.Artifacts = append(normalized.Artifacts, copied)
	}
	if entries, err := os.ReadDir(outputDir); err == nil {
		if len(entries) > 0 {
			return "", fmt.Errorf("output directory must be empty: %s", RedactText(outputDir))
		}
		if err := os.Remove(outputDir); err != nil {
			return "", err
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	path := filepath.Join(tempDir, "manifest.json")
	body, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return "", err
	}
	text := RedactText(string(body))
	if err := AssertSafeText(text, path); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append([]byte(text), '\n'), 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tempDir, outputDir); err != nil {
		return "", err
	}
	cleanupTemp = false
	return filepath.Join(outputDir, "manifest.json"), nil
}

func NormalizeManifest(manifest Manifest) Manifest {
	if manifest.SchemaVersion == "" {
		manifest.SchemaVersion = SchemaVersion
	}
	if manifest.OracleResults.A11y != nil &&
		(manifest.OracleResults.A11y.CriticalCount > 0 || manifest.OracleResults.A11y.SeriousCount > 0) {
		manifest.Status = "failed"
	}
	if hasCheckStatus(manifest.OracleResults.Checks, "failed") {
		manifest.Status = "failed"
	} else if manifest.Status == "passed" && hasCheckStatus(manifest.OracleResults.Checks, "blocked") {
		manifest.Status = "blocked"
	}
	return manifest
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
