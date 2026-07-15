package cli

import (
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/design"
)

func (collector *blobCollector) finalize() {
	collector.evidence.Projects = sortedProjectNames(collector.projectSet)
	collector.evidence.ExecutedProjects = append([]string(nil), collector.evidence.Projects...)
	finalByTest := map[string]*blobResult{}
	for _, result := range collector.results {
		current := finalByTest[result.TestID]
		if current == nil || result.Retry > current.Retry || result.Retry == current.Retry && result.Order > current.Order {
			finalByTest[result.TestID] = result
		}
	}
	if !collector.proofPresent && collector.proofDiagnostic != "" {
		collector.snapshotProof.Diagnostic = collector.proofDiagnostic
	}
	applySnapshotProofEvidence(&collector.evidence, collector.snapshotProof, collector.proofPresent)
	for _, result := range collector.resultSequence {
		if finalByTest[result.TestID] == result && result.Ended {
			collector.materializeResult(result)
		}
	}
}

func (collector *blobCollector) materializeResult(result *blobResult) {
	info := collector.tests[result.TestID]
	for _, step := range collector.stepsByResult[result.ResultID] {
		status := "FAIL"
		comparisonsEnabled, diagnostic := collector.snapshotComparisonStatus(info.Project)
		if comparisonsEnabled && result.Status == "passed" && step.Ended && !step.Failed {
			status = "PASS"
			diagnostic = ""
		}
		baseline := ""
		if !step.Anonymous {
			baseline = collector.baselinePath(info.SnapshotDir, step.Name)
		}
		collector.evidence.Assertions = append(collector.evidence.Assertions, design.VisualAssertion{
			Name: step.Name, Anonymous: step.Anonymous, TestID: result.TestID, Project: info.Project, Status: status,
			BaselinePath: baseline, ComparisonID: visualComparisonID(info.Project, result.TestID, step.Name, step.Name),
			ResultID: result.ResultID, Retry: result.Retry, Diagnostic: diagnostic,
		})
	}
	for _, attachment := range result.Attachments {
		artifact, ok := visualArtifactFromAttachment(attachment, info.Project, result.TestID, result.ResultID, result.Retry)
		if ok {
			artifact.LocalPath = ""
			collector.evidence.Artifacts = append(collector.evidence.Artifacts, artifact)
		}
	}
}

func (collector *blobCollector) snapshotComparisonStatus(project string) (bool, string) {
	if !collector.proofPresent {
		return false, "snapshot comparison enablement was not proven by the Autopus runtime reporter"
	}
	for _, candidate := range collector.snapshotProof.Projects {
		if candidate.Name != project {
			continue
		}
		if candidate.State == "enabled" {
			return true, ""
		}
		return false, "snapshot comparisons were " + candidate.State + " in the resolved Playwright project"
	}
	return false, "snapshot comparison enablement was not proven for the executed Playwright project"
}

func (collector *blobCollector) baselinePath(snapshotDir, name string) string {
	if snapshotDir == "" {
		return ""
	}
	baseline := filepath.Join(snapshotDir, filepath.FromSlash(name))
	if !filepath.IsAbs(baseline) && collector.rootDir != "" {
		baseline = filepath.Join(collector.rootDir, baseline)
	}
	return design.RedactVisualPath(".", baseline)
}
