package cli

import (
	"encoding/json"
	"path"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/design"
)

func writeVerifyVisualGate(root string, uiChanged, screenshots []string, artifacts []design.VisualArtifact, assertions []design.VisualAssertion, projects []string, viewport string, ctx design.Context, maxFixAttempts int, playwrightErr error, strict bool, criticPath string) error {
	evidence := visualEvidence{
		Artifacts:        promoteVisualArtifacts(artifacts),
		Assertions:       assertions,
		Projects:         append([]string(nil), projects...),
		ExecutedProjects: append([]string(nil), projects...),
		RequiredProjects: append([]string(nil), projects...),
	}
	proof := missingSnapshotProof(verifyProjectSelection{Filters: projects}, "snapshot comparison proof is missing")
	proof.RequiredProjects = append([]string(nil), projects...)
	applySnapshotProofEvidence(&evidence, proof, true)
	return writeVerifyVisualGateEvidence(root, uiChanged, screenshots, evidence, viewport, ctx, maxFixAttempts, playwrightErr, strict, criticPath)
}

// collectScreenshots parses Playwright JSON output and returns screenshot file paths.
func collectScreenshots(output []byte) []string {
	return collectScreenshotsFromArtifacts(collectVisualEvidence(output).Artifacts)
}

func collectVisualArtifacts(output []byte) []design.VisualArtifactV2 {
	return collectVisualEvidence(output).Artifacts
}

type visualEvidence struct {
	Artifacts               []design.VisualArtifactV2      `json:"artifacts"`
	Assertions              []design.VisualAssertion       `json:"assertions"`
	Projects                []string                       `json:"projects"`
	ExecutedProjects        []string                       `json:"executed_projects"`
	RequiredProjects        []string                       `json:"required_projects"`
	SnapshotProof           design.SnapshotComparisonProof `json:"snapshot_proof"`
	SnapshotProofStatus     string                         `json:"snapshot_proof_status"`
	SnapshotProofDiagnostic string                         `json:"snapshot_proof_diagnostic,omitempty"`
}

func collectVisualEvidence(output []byte) visualEvidence {
	if evidence, isBlob := collectBlobVisualEvidence(output); isBlob {
		return evidence
	}
	return collectJSONVisualEvidence(output)
}

func collectJSONVisualEvidence(output []byte) visualEvidence {
	var result playwrightResult
	if err := json.Unmarshal(output, &result); err != nil {
		return visualEvidence{}
	}

	evidence := visualEvidence{}
	projectSet := map[string]struct{}{}
	for _, suite := range result.Suites {
		collectJSONSuite(suite, &evidence, projectSet)
	}
	evidence.Projects = sortedProjectNames(projectSet)
	evidence.ExecutedProjects = append([]string(nil), evidence.Projects...)
	if len(result.SnapshotProof) > 0 {
		proof, err := decodeSnapshotComparisonProof(result.SnapshotProof)
		if err == nil {
			applySnapshotProofEvidence(&evidence, proof, true)
		} else {
			applySnapshotProofEvidence(&evidence, missingSnapshotProof(verifyProjectSelection{NoFilter: true}, "snapshot comparison proof is invalid"), false)
		}
	} else {
		applySnapshotProofEvidence(&evidence, snapshotComparisonProof{}, false)
	}
	return evidence
}

func collectJSONSuite(suite playwrightSuite, evidence *visualEvidence, projects map[string]struct{}) {
	for _, spec := range suite.Specs {
		for _, test := range spec.Tests {
			project := strings.TrimSpace(test.ProjectName)
			if project != "" && len(test.Results) > 0 {
				projects[project] = struct{}{}
			}
			testID := firstNonEmpty(test.TestID, test.ID, spec.ID)
			result, ok := finalPlaywrightResult(test.Results)
			if !ok {
				continue
			}
			for _, attachment := range result.Attachments {
				if artifact, ok := visualArtifactFromAttachment(attachment, project, testID, result.ID, result.Retry); ok {
					evidence.Artifacts = append(evidence.Artifacts, artifact)
				}
			}
		}
	}
	for _, nested := range suite.Suites {
		collectJSONSuite(nested, evidence, projects)
	}
}

func finalPlaywrightResult(results []playwrightTestResult) (playwrightTestResult, bool) {
	if len(results) == 0 {
		return playwrightTestResult{}, false
	}
	final := results[0]
	for _, result := range results[1:] {
		if result.Retry >= final.Retry {
			final = result
		}
	}
	return final, true
}

func visualArtifactFromAttachment(attachment playwrightAttachment, project, testID, resultID string, retry int) (design.VisualArtifactV2, bool) {
	if strings.TrimSpace(attachment.Path) == "" {
		return design.VisualArtifactV2{}, false
	}
	kind := design.ClassifyVisualArtifact(attachment.Name, attachment.Path)
	if kind == "other" {
		return design.VisualArtifactV2{}, false
	}
	return design.VisualArtifactV2{
		Name:         attachment.Name,
		Kind:         kind,
		ContentType:  attachment.ContentType,
		Path:         design.RedactVisualPath(".", attachment.Path),
		LocalPath:    attachment.Path,
		ComparisonID: visualComparisonID(project, testID, attachment.Name, attachment.Path),
		ResultID:     resultID,
		Retry:        retry,
	}, true
}

func visualComparisonID(project, testID, name, pathValue string) string {
	fileName := portableBase(pathValue)
	if strings.Contains(portableBase(name), ".") {
		fileName = name
	}
	fileName, ok := normalizeScreenshotName(fileName)
	if !ok {
		fileName = portableBase(fileName)
	}
	dir := path.Dir(fileName)
	base := path.Base(fileName)
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for _, suffix := range []string{"-expected", "-actual", "-diff", "_expected", "_actual", "_diff"} {
		stem = strings.TrimSuffix(stem, suffix)
	}
	if stem == "" {
		stem = "screenshot"
	}
	artifactName := stem + ext
	if dir != "." {
		artifactName = path.Join(dir, artifactName)
	}
	return strings.Join([]string{comparisonSegment(project, "default"), comparisonSegment(testID, "test"), artifactName}, "/")
}

func comparisonSegment(value, fallback string) string {
	value = strings.TrimSpace(portableBase(value))
	if value == "" || value == "." {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sortedProjectNames(projects map[string]struct{}) []string {
	out := make([]string, 0, len(projects))
	for project := range projects {
		out = append(out, project)
	}
	sort.Strings(out)
	return out
}

func screenshotNameFromStep(category, title string) (name string, anonymous, ok bool) {
	// Blob step events omit apiName, so only the exact built-in expect title is trusted.
	if category != "expect" {
		return "", false, false
	}
	if title == `Expect "toHaveScreenshot"` {
		return "", true, true
	}
	const prefix = `Expect "toHaveScreenshot(`
	const suffix = `)"`
	if !strings.HasPrefix(title, prefix) || !strings.HasSuffix(title, suffix) {
		return "", false, false
	}
	name, ok = normalizeScreenshotName(strings.TrimSuffix(strings.TrimPrefix(title, prefix), suffix))
	return name, false, ok
}

func normalizeScreenshotName(name string) (string, bool) {
	if name == "" || strings.TrimSpace(name) != name || strings.ContainsAny(name, "\x00\"") {
		return "", false
	}
	name = strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(name, "/") || hasWindowsVolumePrefix(name) {
		return "", false
	}
	cleaned := path.Clean(name)
	if cleaned != name || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", false
	}
	if !strings.EqualFold(path.Ext(cleaned), ".png") {
		return "", false
	}
	return cleaned, true
}

func hasWindowsVolumePrefix(name string) bool {
	return len(name) >= 2 && ((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z')) && name[1] == ':'
}

func collectScreenshotsFromArtifacts(artifacts []design.VisualArtifactV2) []string {
	var paths []string
	for _, artifact := range artifacts {
		if artifact.Kind == "screenshot" || artifact.Kind == "actual" {
			paths = append(paths, artifact.Path)
		}
	}
	return paths
}
