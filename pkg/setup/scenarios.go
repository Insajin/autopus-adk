package setup

import (
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/e2e"
)

// @AX:NOTE [AUTO] @AX:REASON: design choice — extraction failures are non-fatal; writes minimal empty ScenarioSet on error to avoid blocking setup flow; fan_in=2 (engine.go:Generate and engine.go:Update)
// generateScenarios extracts and writes scenarios.md from the project codebase.
func generateScenarios(projectDir string, info *ProjectInfo) error {
	content, err := renderScenariosContent(projectDir, info)
	if err != nil {
		return err
	}
	return writeGeneratedFile(filepath.Join(projectDir, ".autopus", "project", "scenarios.md"), content)
}

func renderScenariosContent(projectDir string, info *ProjectInfo) ([]byte, error) {
	absDir, _ := filepath.Abs(projectDir)

	scenarios, err := e2e.ExtractCobra(absDir)
	if err != nil {
		scenarios = []e2e.Scenario{}
	}
	scenarios = append(scenarios, generateCrossRepoScenarios(info, len(scenarios))...)

	set := &e2e.ScenarioSet{
		ProjectName: info.Name,
		ProjectType: "Library",
		Binary:      "N/A",
		Build:       "N/A",
		Scenarios:   scenarios,
	}
	return e2e.RenderScenarios(set)
}

func writeGeneratedFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0644)
}

func generateCrossRepoScenarios(info *ProjectInfo, offset int) []e2e.Scenario {
	if info == nil || info.MultiRepo == nil || !info.MultiRepo.IsMultiRepo {
		return nil
	}

	components := make(map[string]RepoComponent, len(info.MultiRepo.Components))
	for _, component := range info.MultiRepo.Components {
		components[component.Name] = component
	}

	seen := make(map[string]bool)
	var scenarios []e2e.Scenario
	for _, dep := range info.MultiRepo.Dependencies {
		key := dep.Source + "->" + dep.Target
		if seen[key] {
			continue
		}
		seen[key] = true

		source := components[dep.Source]
		target := components[dep.Target]
		scenarios = append(scenarios, e2e.Scenario{
			Number:       offset + len(scenarios) + 1,
			ID:           dep.Source + "-" + dep.Target,
			Description:  dep.Source + " integrates with " + dep.Target,
			Command:      buildCrossRepoCommand(source, target),
			Precondition: dep.Source + " and " + dep.Target + " repositories are available in the workspace",
			Env:          "N/A",
			Expect:       "Cross-repo dependency remains compatible",
			Verify: []string{
				"exit_code(0)",
				"stdout_contains(\"PASS\")",
			},
			Depends:  "N/A",
			Requires: "workspace",
			Status:   "active",
		})
	}
	return scenarios
}

func buildCrossRepoCommand(source, target RepoComponent) string {
	return "`" + buildRepoCheck(source) + " && " + buildRepoCheck(target) + "`"
}

func buildRepoCheck(component RepoComponent) string {
	return "(cd " + component.Path + " && " + repoVerificationCommand(component) + ")"
}

func repoVerificationCommand(component RepoComponent) string {
	switch component.PrimaryLanguage {
	case "TypeScript", "JavaScript":
		return "npm test"
	case "Rust":
		return "cargo test"
	case "Python":
		return "pytest"
	default:
		return "go test ./..."
	}
}
