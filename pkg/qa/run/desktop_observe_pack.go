package run

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

func executeDesktopObservationPack(
	opts Options,
	pack journey.Pack,
	runDir string,
) (AdapterResult, string, []IndexCheck) {
	started := time.Now().UTC()
	runner := opts.desktopRunner
	if runner == nil {
		runner = newProductionDesktopObservationRunner(opts)
	}
	outcome, err := runner.Run(context.Background(), desktopRequestFromPack(opts, pack))
	ended := time.Now().UTC()
	if err != nil {
		return desktopObservationContractFailure(pack), "", []IndexCheck{desktopObservationErrorCheck(pack)}
	}

	status := "blocked"
	actual := string(outcome.Verdict)
	failureSummary := ""
	if outcome.Verdict == desktopobserve.VerdictPassed {
		status = "passed"
	} else if outcome.ReasonCode != nil {
		failureSummary = string(*outcome.ReasonCode)
		actual = failureSummary
	}
	check := IndexCheck{
		ID: desktopobserve.DeterministicCheckSemanticLandmarks, JourneyID: pack.ID, Adapter: pack.Adapter.ID,
		Status: status, Expected: "verdict=passed", Actual: actual, FailureSummary: failureSummary,
	}
	result := commandResult{
		Status: status, FailureSummary: failureSummary, StartedAt: started, EndedAt: ended,
		DurationMS: ended.Sub(started).Milliseconds(),
	}
	manifest := buildManifest(opts, pack, result, []IndexCheck{check})
	evidence := desktopObservationEvidence(outcome)
	manifest.OracleResults.DesktopObservation = &evidence
	if !applyDesktopObservationManifestProfile(&manifest, evidence) {
		return desktopObservationContractFailure(pack), "", []IndexCheck{desktopObservationErrorCheck(pack)}
	}
	artifactPath, cleanup, err := writeDesktopObservationArtifact(evidence)
	if err != nil {
		return desktopObservationContractFailure(pack), "", []IndexCheck{desktopObservationErrorCheck(pack)}
	}
	defer cleanup()
	manifest.Artifacts = []qaevidence.ArtifactRef{{
		Kind: "desktop_observation", Path: artifactPath, Publishable: true,
		Redaction: "typed_allowlist_and_scanned",
	}}
	manifestPath, err := qaevidence.WriteFinalManifest(manifest, manifestOutputDir(runDir, pack.ID))
	if err != nil {
		check.Status = "blocked"
		check.FailureSummary = "desktop observation publication failed"
		return AdapterResult{
			Adapter: pack.Adapter.ID, JourneyID: pack.ID, Status: "blocked",
			FailureSummary: check.FailureSummary,
		}, "", []IndexCheck{check}
	}
	return AdapterResult{
		Adapter: pack.Adapter.ID, JourneyID: pack.ID, Status: status,
		QAMESHManifestPath: manifestPath, RepairPromptAvailable: status != "passed",
		FailureSummary: failureSummary, DesktopObservation: &evidence,
	}, manifestPath, []IndexCheck{check}
}

func applyDesktopObservationManifestProfile(
	manifest *qaevidence.Manifest,
	evidence desktopobserve.ObservationEvidence,
) bool {
	if manifest == nil || len(evidence.DeterministicChecks) != 1 {
		return false
	}
	typedCheck := evidence.DeterministicChecks[0]
	failureSummary := ""
	if typedCheck.Status == desktopobserve.CheckBlocked {
		failureSummary = "desktop observation blocked"
	}
	manifest.Runner = qaevidence.Runner{Name: "desktop-accessibility-observe"}
	manifest.OracleResults.Checks = []qaevidence.CheckResult{{
		ID: typedCheck.ID, Type: "desktop_accessibility_semantic",
		Status: string(typedCheck.Status), FailureSummary: failureSummary,
	}}
	manifest.SourceRefs.OwnedPaths = nil
	manifest.SourceRefs.DoNotModifyPaths = nil
	manifest.SourceRefs.OracleThresholds = nil
	manifest.SourceRefs.Mobile = nil
	manifest.RepairPromptRef = ""
	manifest.ReproductionCommand = ""
	return true
}

func desktopRequestFromPack(opts Options, pack journey.Pack) DesktopObservationRunRequest {
	policy := desktopobserve.OraclePolicy{}
	seenNames := make(map[string]bool)
	for _, landmark := range pack.DesktopObservation.RequiredLandmarks {
		policy.MinimumLandmarks = append(policy.MinimumLandmarks, desktopobserve.LandmarkRequirement{
			Role: desktopobserve.Role(landmark.Role), Name: landmark.Name,
			RequiredState: desktopobserve.SemanticStateKey(landmark.RequiredState),
		})
		if !seenNames[landmark.Name] {
			seenNames[landmark.Name] = true
			policy.AllowedNames = append(policy.AllowedNames, landmark.Name)
		}
	}
	for _, safeName := range []string{"Disclosure", "Status"} {
		if !seenNames[safeName] {
			seenNames[safeName] = true
			policy.AllowedNames = append(policy.AllowedNames, safeName)
		}
	}
	operations := make([]desktopobserve.Operation, 0, len(pack.DesktopObservation.Operations))
	for _, operation := range pack.DesktopObservation.Operations {
		operations = append(operations, desktopobserve.Operation(operation))
	}
	return DesktopObservationRunRequest{
		RuntimeProvider: opts.RuntimeProvider,
		Operations:      operations,
		AppRef:          pack.DesktopObservation.AppRef,
		WindowRef:       pack.DesktopObservation.WindowRef,
		Policy:          policy,
		Redactor: func(value string) (string, error) {
			return value, nil
		},
	}
}

func desktopObservationEvidence(outcome desktopobserve.OracleOutcome) desktopobserve.ObservationEvidence {
	evidence := desktopobserve.ObservationEvidence{
		DeterministicChecks: append([]desktopobserve.DeterministicCheck(nil), outcome.DeterministicChecks...),
		RuntimeReceipt:      outcome.RuntimeReceipt,
	}
	evidence.RuntimeReceipt.CapabilitySummary = append(
		[]desktopobserve.CapabilityStatus(nil), outcome.RuntimeReceipt.CapabilitySummary...,
	)
	if outcome.RuntimeReceipt.ReasonCode != nil {
		reason := *outcome.RuntimeReceipt.ReasonCode
		evidence.RuntimeReceipt.ReasonCode = &reason
	}
	if outcome.RuntimeReceipt.NextStep != nil {
		nextStep := *outcome.RuntimeReceipt.NextStep
		evidence.RuntimeReceipt.NextStep = &nextStep
	}
	if outcome.SemanticProjection == nil {
		return evidence
	}
	projection := *outcome.SemanticProjection
	projection.Root = desktopObservationPublicationNode(outcome.SemanticProjection.Root)
	projection.CanonicalJSON = append([]byte(nil), outcome.SemanticProjection.CanonicalJSON...)
	evidence.SemanticProjection = &projection
	return evidence
}

func desktopObservationPublicationNode(source desktopobserve.SemanticNode) desktopobserve.SemanticNode {
	published := source
	published.SemanticState = desktopobserve.SemanticState{
		Enabled:  copyDesktopObservationBool(source.SemanticState.Enabled),
		Expanded: copyDesktopObservationBool(source.SemanticState.Expanded),
		Focused:  copyDesktopObservationBool(source.SemanticState.Focused),
		Selected: copyDesktopObservationBool(source.SemanticState.Selected),
	}
	published.Frame = nil
	published.AdvertisedActions = append([]desktopobserve.Action(nil), source.AdvertisedActions...)
	published.Children = make([]desktopobserve.SemanticNode, len(source.Children))
	for index := range source.Children {
		published.Children[index] = desktopObservationPublicationNode(source.Children[index])
	}
	return published
}

func copyDesktopObservationBool(source *bool) *bool {
	if source == nil {
		return nil
	}
	value := *source
	return &value
}

func writeDesktopObservationArtifact(evidence desktopobserve.ObservationEvidence) (string, func(), error) {
	body, err := json.Marshal(evidence)
	if err != nil {
		return "", func() {}, err
	}
	file, err := os.CreateTemp("", "qamesh-desktop-observation-*.json")
	if err != nil {
		return "", func() {}, err
	}
	path := file.Name()
	cleanup := func() { _ = os.Remove(path) }
	if _, err := file.Write(append(body, '\n')); err != nil {
		_ = file.Close()
		cleanup()
		return "", func() {}, err
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return path, cleanup, nil
}

func desktopObservationContractFailure(pack journey.Pack) AdapterResult {
	return AdapterResult{
		Adapter: pack.Adapter.ID, JourneyID: pack.ID, Status: "blocked",
		FailureSummary: "desktop observation contract failed",
	}
}

func desktopObservationErrorCheck(pack journey.Pack) IndexCheck {
	return IndexCheck{
		ID: desktopobserve.DeterministicCheckSemanticLandmarks, JourneyID: pack.ID, Adapter: pack.Adapter.ID,
		Status: "blocked", Expected: "verdict=passed", Actual: "contract_error",
		FailureSummary: "desktop observation contract failed",
	}
}
