package evidence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

const (
	desktopObservationAdapterID      = "desktop-accessibility-observe"
	desktopObservationArtifact       = "desktop_observation"
	desktopObservationArtifactName   = "desktop-observation.json"
	desktopObservationCheckType      = "desktop_accessibility_semantic"
	desktopObservationFailureSummary = "desktop observation blocked"
	desktopObservationLane           = "desktop-native"
	desktopObservationSourceSpec     = "SPEC-QAMESH-012"
	desktopObservationStepID         = "step-1"
)

func isDesktopObservationContract(manifest Manifest) bool {
	if manifest.OracleResults.DesktopObservation != nil ||
		equalDesktopMarker(manifest.SourceRefs.Adapter, desktopObservationAdapterID) ||
		equalDesktopMarker(manifest.SourceRefs.SourceSpec, desktopObservationSourceSpec) {
		return true
	}
	if manifest.SchemaVersion != SchemaVersionV2 {
		return false
	}
	if equalDesktopMarker(manifest.SourceRefs.JourneyID, desktopObservationAdapterID) ||
		equalDesktopMarker(manifest.ScenarioRef, desktopObservationAdapterID) ||
		equalDesktopMarker(manifest.Runner.Name, desktopObservationAdapterID) {
		return true
	}
	for _, check := range manifest.OracleResults.Checks {
		if check.ID == desktopobserve.DeterministicCheckSemanticLandmarks ||
			equalDesktopMarker(check.Type, desktopObservationCheckType) {
			return true
		}
	}
	for _, artifact := range manifest.Artifacts {
		if equalDesktopMarker(artifact.Kind, desktopObservationArtifact) {
			return true
		}
	}
	for _, ref := range manifest.SourceRefs.AcceptanceRefs {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(ref)), "AC-QAMESH12-") {
			return true
		}
	}
	return false
}

func validateDesktopObservationContract(manifest Manifest) error {
	if manifest.OracleResults.DesktopObservation == nil {
		return fmt.Errorf("desktop observation manifest provenance or typed oracle contract is invalid")
	}
	observation, _, err := canonicalDesktopObservation(*manifest.OracleResults.DesktopObservation)
	if err != nil {
		return err
	}
	if !validDesktopObservationStructure(manifest, observation) {
		return fmt.Errorf("desktop observation manifest provenance or typed oracle contract is invalid")
	}
	return validateDesktopObservationOutcome(manifest, observation)
}

func validDesktopObservationStructure(manifest Manifest, observation desktopobserve.ObservationEvidence) bool {
	if len(manifest.Artifacts) != 1 || len(manifest.OracleResults.Checks) != 1 ||
		len(observation.DeterministicChecks) != 1 {
		return false
	}
	artifact := manifest.Artifacts[0]
	check := manifest.OracleResults.Checks[0]
	typedCheck := observation.DeterministicChecks[0]
	wantStatus, wantSummary := string(typedCheck.Status), ""
	if typedCheck.Status == desktopobserve.CheckBlocked {
		wantSummary = desktopObservationFailureSummary
	}
	return check.ID == typedCheck.ID && check.Type == desktopObservationCheckType &&
		check.Status == wantStatus && check.Expected == "" && check.Actual == "" &&
		len(check.ArtifactRefs) == 0 && check.FailureSummary == wantSummary &&
		manifest.SchemaVersion == SchemaVersionV2 && manifest.Surface == "desktop" &&
		manifest.Lane == desktopObservationLane && manifest.ScenarioRef == desktopObservationAdapterID &&
		manifest.Runner == (Runner{Name: desktopObservationAdapterID}) &&
		manifest.SourceRefs.Adapter == desktopObservationAdapterID &&
		manifest.SourceRefs.SourceSpec == desktopObservationSourceSpec &&
		manifest.SourceRefs.JourneyID == desktopObservationAdapterID &&
		manifest.SourceRefs.StepID == desktopObservationStepID && validDesktopObservationAcceptanceRefs(manifest.SourceRefs.AcceptanceRefs) &&
		len(manifest.SourceRefs.OwnedPaths) == 0 && len(manifest.SourceRefs.DoNotModifyPaths) == 0 &&
		len(manifest.SourceRefs.OracleThresholds) == 0 && manifest.SourceRefs.Mobile == nil &&
		manifest.RepairPromptRef == "" && manifest.ReproductionCommand == "" &&
		manifest.OracleResults.DesktopObservation != nil && manifest.OracleResults.A11y == nil &&
		manifest.OracleResults.Desktop == nil && len(manifest.RedactionStatus.Findings) == 0 &&
		artifact.Kind == desktopObservationArtifact && artifact.Publishable &&
		(artifact.Redaction == "pre_redacted_and_scanned" || artifact.Redaction == "typed_allowlist_and_scanned") &&
		strings.ToLower(filepath.Ext(artifact.Path)) == ".json"
}

func validateDesktopObservationOutcome(manifest Manifest, observation desktopobserve.ObservationEvidence) error {
	check := manifest.OracleResults.Checks[0]
	passed := observation.RuntimeReceipt.ReasonCode == nil
	consistent := passed && manifest.Status == "passed" && check.Status == "passed" ||
		!passed && manifest.Status == "blocked" && check.Status == "blocked"
	if !consistent {
		return fmt.Errorf("desktop observation manifest, check, receipt, and projection outcomes are inconsistent")
	}
	return nil
}

func canonicalDesktopObservation(
	observation desktopobserve.ObservationEvidence,
) (desktopobserve.ObservationEvidence, []byte, error) {
	body, _ := json.Marshal(observation)
	decoded, err := desktopobserve.DecodeObservationEvidence(body)
	if err != nil {
		return desktopobserve.ObservationEvidence{}, nil, fmt.Errorf("validate desktop observation: %w", err)
	}
	canonical, _ := json.Marshal(decoded)
	return decoded, canonical, nil
}

func copyDesktopObservationArtifact(
	artifact ArtifactRef,
	inlineBody []byte,
	outputDir string,
) (ArtifactRef, error) {
	raw, err := readDesktopObservationArtifact(artifact.Path, desktopObservationFileOps{
		lstat: os.Lstat,
		open:  os.Open,
	})
	if err != nil || RedactText(string(raw)) != string(raw) || AssertSafeText(string(raw), artifact.Path) != nil {
		return ArtifactRef{}, fmt.Errorf("desktop observation artifact is not pre-redacted safe text")
	}
	decodedArtifact, err := desktopobserve.DecodeObservationEvidence(raw)
	if err != nil {
		return ArtifactRef{}, fmt.Errorf("decode desktop observation artifact: %w", err)
	}
	artifactBody, _ := json.Marshal(decodedArtifact)
	if !bytes.Equal(inlineBody, artifactBody) {
		return ArtifactRef{}, fmt.Errorf("desktop observation inline and artifact payloads differ")
	}

	artifactDir := filepath.Join(outputDir, "artifacts", desktopObservationArtifact)
	target := filepath.Join(artifactDir, desktopObservationArtifactName)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return ArtifactRef{}, err
	}
	if err := os.WriteFile(target, append(artifactBody, '\n'), 0o644); err != nil {
		return ArtifactRef{}, err
	}
	artifact.Path = filepath.ToSlash(filepath.Join("artifacts", desktopObservationArtifact, desktopObservationArtifactName))
	return artifact, nil
}

type desktopObservationFileOps struct {
	lstat func(string) (os.FileInfo, error)
	open  func(string) (*os.File, error)
}

func readDesktopObservationArtifact(path string, ops desktopObservationFileOps) ([]byte, error) {
	pathInfo, err := ops.lstat(path)
	if err != nil || pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("desktop observation artifact must be a regular file")
	}
	if pathInfo.Size() < 0 || pathInfo.Size() > desktopobserve.MaxEnvelopeBytes {
		return nil, desktopobserve.ErrEnvelopeTooLarge
	}
	file, err := ops.open(path)
	if err != nil {
		return nil, fmt.Errorf("open desktop observation artifact: %w", err)
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil || !fileInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("desktop observation artifact descriptor must be regular")
	}
	currentInfo, currentErr := ops.lstat(path)
	if currentErr != nil || currentInfo.Mode()&os.ModeSymlink != 0 || !currentInfo.Mode().IsRegular() ||
		!os.SameFile(pathInfo, fileInfo) || !os.SameFile(currentInfo, fileInfo) {
		return nil, fmt.Errorf("desktop observation artifact identity changed before read")
	}
	if fileInfo.Size() < 0 || fileInfo.Size() > desktopobserve.MaxEnvelopeBytes {
		return nil, desktopobserve.ErrEnvelopeTooLarge
	}
	raw, err := io.ReadAll(io.LimitReader(file, desktopobserve.MaxEnvelopeBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read desktop observation artifact: %w", err)
	}
	if len(raw) > desktopobserve.MaxEnvelopeBytes {
		return nil, desktopobserve.ErrEnvelopeTooLarge
	}
	return raw, nil
}

func validDesktopObservationAcceptanceRefs(refs []string) bool {
	if len(refs) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if !strings.HasPrefix(ref, "AC-QAMESH12-") || len(ref) != len("AC-QAMESH12-000") {
			return false
		}
		value, err := strconv.Atoi(strings.TrimPrefix(ref, "AC-QAMESH12-"))
		if err != nil || value < 1 || value > 18 {
			return false
		}
		if _, duplicate := seen[ref]; duplicate {
			return false
		}
		seen[ref] = struct{}{}
	}
	return true
}

func equalDesktopMarker(value, marker string) bool {
	return strings.EqualFold(strings.TrimSpace(value), marker)
}
