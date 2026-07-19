package evidence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func LoadManifest(path string) (Manifest, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	if err := rejectManifestDuplicateKeys(body); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	if isDesktopObservationContract(manifest) || hasDesktopObservationJSONMarker(body) {
		return decodeDesktopObservationManifest(body)
	}
	return manifest, nil
}

func decodeDesktopObservationManifest(body []byte) (Manifest, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return Manifest{}, fmt.Errorf("parse Q12 manifest: %w: %v", desktopobserve.ErrUnknownField, err)
		}
		return Manifest{}, fmt.Errorf("parse Q12 manifest: %w", err)
	}
	if err := requireManifestJSONEOF(decoder); err != nil {
		return Manifest{}, fmt.Errorf("parse Q12 manifest: %w", err)
	}
	if manifest.OracleResults.DesktopObservation == nil {
		return Manifest{}, fmt.Errorf("validate Q12 manifest: desktop observation typed oracle is required")
	}
	raw, err := rawDesktopObservation(body)
	if err != nil {
		return Manifest{}, fmt.Errorf("parse Q12 manifest: %w", err)
	}
	observation, err := desktopobserve.DecodeObservationEvidence(raw)
	if err != nil {
		return Manifest{}, fmt.Errorf("parse Q12 manifest: %w", err)
	}
	manifest.OracleResults.DesktopObservation = &observation
	if err := manifest.Validate(); err != nil {
		return Manifest{}, fmt.Errorf("validate Q12 manifest: %w", err)
	}
	return manifest, nil
}

func rawDesktopObservation(body []byte) ([]byte, error) {
	var document struct {
		OracleResults struct {
			DesktopObservation json.RawMessage `json:"desktop_observation"`
		} `json:"oracle_results"`
	}
	if err := json.Unmarshal(body, &document); err != nil || len(document.OracleResults.DesktopObservation) == 0 {
		return nil, desktopobserve.ErrMalformedEnvelope
	}
	return document.OracleResults.DesktopObservation, nil
}

func hasDesktopObservationJSONMarker(body []byte) bool {
	var document map[string]json.RawMessage
	if json.Unmarshal(body, &document) != nil {
		return false
	}
	var oracleResults map[string]json.RawMessage
	if json.Unmarshal(document["oracle_results"], &oracleResults) != nil {
		return false
	}
	_, exists := oracleResults["desktop_observation"]
	return exists
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
	if err := validateMobilePublication(manifest); err != nil {
		return "", err
	}
	normalized := manifest
	desktopObservation := isDesktopObservationContract(manifest)
	var desktopObservationBody []byte
	if desktopObservation {
		observation, canonicalBody, err := canonicalDesktopObservation(*manifest.OracleResults.DesktopObservation)
		if err != nil {
			return "", err
		}
		normalized.OracleResults.DesktopObservation = &observation
		desktopObservationBody = canonicalBody
	}
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
		var copied ArtifactRef
		if desktopObservation {
			copied, err = copyDesktopObservationArtifact(artifact, desktopObservationBody, tempDir)
		} else {
			copied, err = sanitizeArtifact(artifact, tempDir)
		}
		if err != nil {
			return "", err
		}
		normalized.Artifacts = append(normalized.Artifacts, copied)
	}
	path := filepath.Join(tempDir, "manifest.json")
	body, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return "", err
	}
	text := string(body)
	if desktopObservation {
		if RedactText(text) != text {
			return "", fmt.Errorf("desktop observation requires publication-time redaction to be a no-op")
		}
	} else {
		text = RedactText(text)
	}
	if err := AssertSafeText(text, path); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append([]byte(text), '\n'), 0o644); err != nil {
		return "", err
	}
	if desktopObservation {
		if err := scanDesktopObservationPublication(tempDir, normalized); err != nil {
			return "", err
		}
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
