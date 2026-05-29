package cli

import (
	"encoding/json"
	"fmt"

	"github.com/insajin/autopus-adk/pkg/design"
)

func resolveVisualGateArgs(args []bool) (bool, bool) {
	visualGate := true
	strictVisualGate := false
	if len(args) > 0 {
		visualGate = args[0]
	}
	if len(args) > 1 {
		strictVisualGate = args[1]
	}
	return visualGate, strictVisualGate
}

func writeVerifyVisualGate(root string, uiChanged, screenshots []string, artifacts []design.VisualArtifact, viewport string, ctx design.Context, maxFixAttempts int, playwrightErr error, strict bool, criticPath string) error {
	playwrightErrText := ""
	if playwrightErr != nil {
		playwrightErrText = playwrightErr.Error()
	}
	critic, err := design.LoadVisualCriticReport(root, criticPath)
	if err != nil {
		return fmt.Errorf("visual critic report 로드 실패: %w", err)
	}
	report := design.BuildVisualGateReport(design.VisualGateInput{
		UIChanged:      uiChanged,
		Screenshots:    screenshots,
		Artifacts:      artifacts,
		Viewport:       viewport,
		DesignContext:  ctx,
		MaxFixAttempts: maxFixAttempts,
		PlaywrightErr:  playwrightErrText,
		VisualCritic:   critic,
	})
	path, err := design.WriteVisualGateReport(root, report)
	if err != nil {
		return fmt.Errorf("visual gate report 저장 실패: %w", err)
	}
	fmt.Print(report.Summary(path))
	if strict && report.Verdict == "FAIL" {
		return fmt.Errorf("strict visual gate failed")
	}
	return nil
}

// collectScreenshots parses Playwright JSON output and returns screenshot file paths.
func collectScreenshots(output []byte) []string {
	return collectScreenshotsFromArtifacts(collectVisualArtifacts(output))
}

func collectVisualArtifacts(output []byte) []design.VisualArtifact {
	var result playwrightResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil
	}

	var artifacts []design.VisualArtifact
	for _, suite := range result.Suites {
		for _, spec := range suite.Specs {
			for _, test := range spec.Tests {
				for _, res := range test.Results {
					for _, att := range res.Attachments {
						if att.Path == "" {
							continue
						}
						kind := design.ClassifyVisualArtifact(att.Name, att.Path)
						if kind == "other" {
							continue
						}
						artifacts = append(artifacts, design.VisualArtifact{
							Name:        att.Name,
							Kind:        kind,
							ContentType: att.ContentType,
							Path:        design.RedactVisualPath(".", att.Path),
							LocalPath:   att.Path,
						})
					}
				}
			}
		}
	}
	return artifacts
}

func collectScreenshotsFromArtifacts(artifacts []design.VisualArtifact) []string {
	var paths []string
	for _, artifact := range artifacts {
		if artifact.Kind == "screenshot" || artifact.Kind == "actual" {
			paths = append(paths, artifact.Path)
		}
	}
	return paths
}
