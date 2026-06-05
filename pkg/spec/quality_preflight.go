package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateSpecSet validates the full SPEC authoring package before review.
func ValidateSpecSet(specDir string, doc *SpecDocument) []ValidationError {
	errs := ValidateSpec(doc)
	files, readErrs := readSpecValidationFiles(specDir)
	errs = append(errs, readErrs...)
	if len(readErrs) > 0 {
		return errs
	}

	errs = append(errs, validateRequiredSections(files)...)
	errs = append(errs, validateAuthoringPlaceholders(files)...)
	errs = append(errs, validateTraceability(files["spec.md"], files["research.md"], files["acceptance.md"])...)
	errs = append(errs, validateSelfVerifySummary(files["research.md"])...)
	errs = append(errs, validateCompletionDebtSeparation(files["research.md"])...)
	return errs
}

func readSpecValidationFiles(specDir string) (map[string]string, []ValidationError) {
	names := []string{"spec.md", "plan.md", "acceptance.md", "research.md"}
	files := make(map[string]string, len(names))
	var errs []ValidationError
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(specDir, name))
		if err != nil {
			errs = append(errs, ValidationError{
				Field:   name,
				Message: fmt.Sprintf("%s 읽기 실패: %v", name, err),
				Level:   "error",
			})
			continue
		}
		files[name] = string(data)
	}
	return files, errs
}

func validateRequiredSections(files map[string]string) []ValidationError {
	required := map[string][]string{
		"spec.md":       {"## Outcome Boundary", "## Requirements", "## Traceability Matrix"},
		"plan.md":       {"## Implementation Strategy", "## Visual Planning Brief", "## Feature Completion Scope", "## Tasks"},
		"acceptance.md": {"## Test Scenarios", "## Oracle Acceptance Notes"},
		"research.md": {
			"## Outcome Lock",
			"## Visual Planning Brief",
			"## Semantic Invariant Inventory",
			"## Feature Coverage Map",
			"## Completion Debt",
			"## Evolution Ideas",
			"## Reference Discipline",
			"## Reviewer Brief",
			"## Self-Verify Summary",
		},
	}

	var errs []ValidationError
	for name, sections := range required {
		for _, section := range sections {
			if !strings.Contains(files[name], section) {
				errs = append(errs, ValidationError{
					Field:   name,
					Message: fmt.Sprintf("authoring preflight 섹션 누락: %s", section),
					Level:   "error",
				})
			}
		}
	}
	return errs
}

func validateAuthoringPlaceholders(files map[string]string) []ValidationError {
	placeholders := []string{
		"[동작]",
		"[트리거]",
		"[비정상 상태]",
		"[대응]",
		"[시나리오 제목]",
		"[초기 상태]",
		"[예상 결과]",
		"[에지 케이스]",
		"[sanitized user request evidence]",
		"[ordering / parser / formula / state transition]",
		"[stdout/API field/file content]",
	}

	var errs []ValidationError
	for name, content := range files {
		for _, placeholder := range placeholders {
			if strings.Contains(content, placeholder) {
				errs = append(errs, ValidationError{
					Field:   name,
					Message: fmt.Sprintf("authoring preflight placeholder 미해결: %s", placeholder),
					Level:   "error",
				})
				break
			}
		}
	}
	return errs
}

func validateSelfVerifySummary(researchMD string) []ValidationError {
	body := sectionBody(researchMD, "## Self-Verify Summary")
	required := []string{"Q-CORR-04", "Q-COMP-05", "Q-COMP-06", "Q-COMP-07"}
	var errs []ValidationError
	for _, id := range required {
		if !strings.Contains(body, id) {
			errs = append(errs, ValidationError{
				Field:   "research.md",
				Message: fmt.Sprintf("Self-Verify Summary에 %s 결과가 없습니다", id),
				Level:   "error",
			})
		}
	}
	return errs
}

func validateCompletionDebtSeparation(researchMD string) []ValidationError {
	evolution := sectionBody(researchMD, "## Evolution Ideas")
	var errs []ValidationError
	if strings.Contains(evolution, "SPEC-") || strings.Contains(evolution, "AC-") || strings.Contains(evolution, "- [ ]") {
		errs = append(errs, ValidationError{
			Field:   "research.md",
			Message: "Evolution Ideas가 SPEC/acceptance/task 항목으로 승격되어 있습니다. optional advisory로만 남겨야 합니다",
			Level:   "error",
		})
	}
	return errs
}
