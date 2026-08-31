package run

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/capture"
	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

const (
	guiCaptureDirEnv         = "AUTOPUS_QAMESH_GUI_CAPTURE_DIR"
	guiCapturePolicyPathEnv  = "AUTOPUS_QAMESH_GUI_CAPTURE_POLICY_PATH"
	guiCaptureIndexPathEnv   = "AUTOPUS_QAMESH_GUI_CAPTURE_INDEX_PATH"
	guiCaptureCheckID        = "gui-capture-contract"
	guiCaptureCheckType      = "gui_capture_contract"
	guiCaptureDirName        = capture.DirName
	guiCapturePolicyFileName = "gui-capture-policy.json"
	guiCapturePolicySchema   = "autopus.qamesh.gui_capture_policy.v1"
	// RetentionLocalMedia marks a manifest whose journey left raw capture media
	// on the machine. It is not a publication class: published artifacts are
	// still text-only, but a human reading the evidence needs to know local
	// screenshots, traces, or videos exist and must not be shipped.
	RetentionLocalMedia = "local-redacted-local-media"
	// RetentionLocalRedacted is the default publication-safe retention class.
	RetentionLocalRedacted = "local-redacted"
)

// guiCaptureInput is the runtime handoff to the producer. The harness allocates
// the directory and declares the policy; the project's own Playwright run fills
// it in. The harness never drives the browser itself.
type guiCaptureInput struct {
	Env       []string
	Dir       string
	IndexPath string
}

// prepareGUICaptureInput allocates the capture directory and writes the
// effective policy the producer must honor. It is a no-op for packs that did not
// opt into typed capture, so packs written before this contract keep working.
func prepareGUICaptureInput(pack journey.Pack, artifactDir string) (guiCaptureInput, error) {
	policy := pack.GUI.Capture
	if pack.Adapter.ID != "gui-explore" || !policy.Enabled() {
		return guiCaptureInput{}, nil
	}
	dir, err := filepath.Abs(filepath.Join(artifactDir, guiCaptureDirName))
	if err != nil {
		return guiCaptureInput{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return guiCaptureInput{}, err
	}
	document := map[string]any{
		"schema_version":   guiCapturePolicySchema,
		"journey_id":       pack.ID,
		"allowed_origins":  cleanedList(pack.GUI.AllowedOrigins),
		"mode":             policy.Mode,
		"streams":          policy.Streams,
		"screenshot":       policy.Screenshot,
		"console_severity": policy.ConsoleSeverity,
		"retain_local":     policy.RetainLocal,
		"replay_script":    policy.ReplayScript,
	}
	body, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return guiCaptureInput{}, err
	}
	policyPath := filepath.Join(dir, guiCapturePolicyFileName)
	if err := os.WriteFile(policyPath, append(body, '\n'), 0o644); err != nil {
		return guiCaptureInput{}, err
	}
	indexPath := filepath.Join(dir, capture.IndexFileName)
	return guiCaptureInput{
		Dir:       dir,
		IndexPath: indexPath,
		Env: []string{
			guiCaptureDirEnv + "=" + dir,
			guiCapturePolicyPathEnv + "=" + policyPath,
			guiCaptureIndexPathEnv + "=" + indexPath,
		},
	}, nil
}

// applyGUICaptureOracle turns the producer-authored capture index into a
// deterministic check and a publishable artifact.
//
// Every failure path blocks rather than degrades: a GUI journey that promised
// capture evidence and did not deliver it is not a passing journey, because the
// whole point of the contract is that missing evidence is visible.
func applyGUICaptureOracle(pack journey.Pack, result *commandResult) (IndexCheck, bool) {
	policy := pack.GUI.Capture
	if pack.Adapter.ID != "gui-explore" || !policy.Enabled() {
		return IndexCheck{}, false
	}
	check := IndexCheck{
		ID:        guiCaptureCheckID,
		JourneyID: pack.ID,
		Adapter:   pack.Adapter.ID,
		Expected:  "capture index conforms to gui.capture policy",
	}
	index, findings := loadConformingCaptureIndex(pack, policy, result)
	if len(findings) > 0 {
		return blockCaptureCheck(check, result, findings), true
	}
	publishedPath, err := capture.WritePublished(index, result.CaptureDir)
	if err != nil {
		return blockCaptureCheck(check, result, []string{err.Error()}), true
	}
	result.CaptureArtifact = &qaevidence.ArtifactRef{
		Kind:        capture.ArtifactKind,
		Path:        publishedPath,
		Publishable: true,
		Redaction:   "text_redacted_and_scanned",
	}
	result.CaptureLocalMedia = index.Totals.MediaBytes > 0
	check.Status = "passed"
	check.Actual = fmt.Sprintf("steps=%d actions=%d console_errors=%d network_failures=%d screenshots=%d",
		index.Totals.Steps, index.Totals.Actions, index.Totals.ConsoleErrors,
		index.Totals.NetworkFailures, index.Totals.Screenshots)
	return check, true
}

// loadConformingCaptureIndex applies every gate in order and returns the first
// set of findings. Order matters: a malformed index cannot be conformance-checked,
// and an index that fails conformance must not have its media trusted.
func loadConformingCaptureIndex(pack journey.Pack, policy capture.Policy, result *commandResult) (capture.Index, []string) {
	if result.CaptureDir == "" || result.CaptureIndexPath == "" {
		return capture.Index{}, []string{"capture runtime was not prepared for this journey"}
	}
	index, err := capture.LoadIndex(result.CaptureIndexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return capture.Index{}, []string{fmt.Sprintf("producer did not write %s into the capture directory", capture.IndexFileName)}
		}
		return capture.Index{}, []string{"capture index is unreadable: " + err.Error()}
	}
	if err := capture.Validate(index); err != nil {
		return capture.Index{}, []string{"capture index is invalid: " + err.Error()}
	}
	if index.JourneyID != pack.ID {
		return capture.Index{}, []string{fmt.Sprintf("capture index journey_id %q does not match pack %q", index.JourneyID, pack.ID)}
	}
	if findings := capture.Conform(index, policy); len(findings) > 0 {
		return capture.Index{}, findings
	}
	if findings := capture.VerifyLocalMedia(index, result.CaptureDir); len(findings) > 0 {
		return capture.Index{}, findings
	}
	return index, nil
}

func blockCaptureCheck(check IndexCheck, result *commandResult, findings []string) IndexCheck {
	summary := "gui capture contract rejected the journey: " + strings.Join(findings, "; ")
	check.Status = "blocked"
	check.Actual = "capture contract not satisfied"
	check.FailureSummary = summary
	result.Status = "blocked"
	result.FailureSummary = summary
	return check
}

// retentionClassFor keeps the manifest honest about what is left on disk.
func retentionClassFor(result commandResult) string {
	if result.CaptureLocalMedia {
		return RetentionLocalMedia
	}
	return RetentionLocalRedacted
}
