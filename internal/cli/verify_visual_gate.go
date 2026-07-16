package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/design"
)

func writeVerifyVisualGateEvidence(root string, uiChanged, screenshots []string, evidence visualEvidence, viewport string, ctx design.Context, maxFixAttempts int, playwrightErr error, strict bool, criticPath string) error {
	playwrightErrText := ""
	if playwrightErr != nil {
		playwrightErrText = publicPlaywrightError(playwrightErr)
	}
	critic, err := design.LoadVisualCriticReport(root, criticPath)
	if err != nil {
		return fmt.Errorf("visual critic report 로드 실패: %w", err)
	}
	legacy := design.BuildVisualGateReport(design.VisualGateInput{
		UIChanged: uiChanged, Screenshots: screenshots, Artifacts: projectLegacyArtifacts(evidence.Artifacts),
		Viewport: viewport, DesignContext: ctx, MaxFixAttempts: maxFixAttempts,
		PlaywrightErr: playwrightErrText, VisualCritic: critic,
	})
	report := design.BuildVisualGateReportV2(design.VisualGateInputV2{
		Strict: strict, UIChanged: uiChanged, Screenshots: screenshots, Artifacts: evidence.Artifacts,
		Assertions: evidence.Assertions, RequiredProjects: evidence.RequiredProjects,
		ExecutedProjects: evidence.ExecutedProjects, Viewport: viewport, DesignContext: ctx,
		MaxFixAttempts: maxFixAttempts, PlaywrightErr: playwrightErrText, VisualCritic: critic,
		SnapshotProof: evidence.SnapshotProof,
	})
	if err := design.WriteVisualGateReportBundle(root, legacy, report); err != nil {
		return fmt.Errorf("visual gate report 저장 실패: %w", err)
	}
	path := filepath.Join(root, ".autopus", "design", "verify", "latest.v2.json")
	fmt.Print(visualGateV2Summary(report, path))
	if strict && report.Verdict == "FAIL" {
		return fmt.Errorf("strict visual gate failed")
	}
	return nil
}

func promoteVisualArtifacts(artifacts []design.VisualArtifact) []design.VisualArtifactV2 {
	out := make([]design.VisualArtifactV2, 0, len(artifacts))
	for _, artifact := range artifacts {
		out = append(out, design.VisualArtifactV2{
			Name: artifact.Name, Kind: artifact.Kind, ContentType: artifact.ContentType,
			Path: artifact.Path, LocalPath: artifact.LocalPath,
		})
	}
	return out
}

func projectLegacyArtifacts(artifacts []design.VisualArtifactV2) []design.VisualArtifact {
	out := make([]design.VisualArtifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		out = append(out, design.VisualArtifact{
			Name: artifact.Name, Kind: artifact.Kind, ContentType: artifact.ContentType,
			Path: artifact.Path, LocalPath: artifact.LocalPath,
		})
	}
	return out
}

func projectDesignSnapshotProof(proof snapshotComparisonProof) design.SnapshotComparisonProof {
	projected := design.SnapshotComparisonProof{
		Status:            snapshotProofOverallStatus(proof),
		Diagnostic:        proof.Diagnostic,
		PlaywrightVersion: proof.PlaywrightVersion,
		UpdateSnapshots:   proof.UpdateSnapshots,
	}
	for _, project := range proof.Projects {
		projected.Projects = append(projected.Projects, design.SnapshotComparisonProject{
			Name: project.Name, ComparisonStatus: project.State,
		})
	}
	return projected
}

func visualGateV2Summary(report design.VisualGateReportV2, path string) string {
	var summary strings.Builder
	fmt.Fprintf(&summary, "visual gate: %s (%s)\n", report.Verdict, filepath.ToSlash(path))
	for _, check := range report.Checks {
		fmt.Fprintf(&summary, "  - %s: %s — %s\n", check.ID, check.Status, check.Message)
	}
	return summary.String()
}
