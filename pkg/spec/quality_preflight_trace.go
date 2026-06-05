package spec

import (
	"fmt"
	"strings"
)

func validateTraceability(specMD, researchMD, acceptanceMD string) []ValidationError {
	var errs []ValidationError
	traceRows := tableRows(sectionBody(specMD, "## Traceability Matrix"))
	if len(traceRows) == 0 {
		errs = append(errs, ValidationError{
			Field:   "spec.md",
			Message: "Traceability Matrix에 Requirement, Plan Task, Acceptance Scenario, Semantic Invariant 연결 행이 없습니다",
			Level:   "error",
		})
	}

	invariantRows := tableRows(sectionBody(researchMD, "## Semantic Invariant Inventory"))
	if len(invariantRows) == 0 {
		body := strings.ToLower(sectionBody(researchMD, "## Semantic Invariant Inventory"))
		if !strings.Contains(body, "none") && !strings.Contains(body, "n/a") {
			errs = append(errs, ValidationError{
				Field:   "research.md",
				Message: "Semantic Invariant Inventory가 비어 있습니다. invariant가 없으면 None/N/A 사유를 기록해야 합니다",
				Level:   "error",
			})
		}
	}

	for _, row := range invariantRows {
		if len(row) < 5 {
			continue
		}
		invariantID := strings.TrimSpace(row[0])
		acceptanceIDs := splitReferenceCells(row[4])
		if invariantID == "" || strings.EqualFold(invariantID, "none") {
			continue
		}
		if len(acceptanceIDs) == 0 {
			errs = append(errs, ValidationError{
				Field:   "research.md",
				Message: fmt.Sprintf("%s invariant가 acceptance ID와 연결되지 않았습니다", invariantID),
				Level:   "error",
			})
			continue
		}
		if !strings.Contains(specMD, invariantID) {
			errs = append(errs, ValidationError{
				Field:   "spec.md",
				Message: fmt.Sprintf("Traceability Matrix가 %s invariant를 참조하지 않습니다", invariantID),
				Level:   "error",
			})
		}
		for _, id := range acceptanceIDs {
			if !strings.Contains(acceptanceMD, id) {
				errs = append(errs, ValidationError{
					Field:   "acceptance.md",
					Message: fmt.Sprintf("%s acceptance ID가 acceptance.md에 없습니다", id),
					Level:   "error",
				})
			}
		}
	}

	if acceptanceLooksStructuralOnly(acceptanceMD) {
		errs = append(errs, ValidationError{
			Field:   "acceptance.md",
			Message: "Must acceptance가 structural-only 신호만 포함합니다. concrete expected output 또는 explicit tolerance가 필요합니다",
			Level:   "error",
		})
	}
	return errs
}
