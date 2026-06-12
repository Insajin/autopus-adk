package spec

import (
	"fmt"
	"strings"
)

// 모호한 언어 목록
var ambiguousWords = []string{"should", "might", "could", "possibly", "maybe", "perhaps"}

// moscowPriorities are the MoSCoW Priority-column tokens that can collide with
// ambiguous words (notably "Should"). When a markdown requirements-table cell
// holds exactly one of these tokens, it is the Priority column, not prose.
var moscowPriorities = map[string]bool{"must": true, "should": true, "nice": true}

// ambiguousScanText returns the portion of a requirement description that
// should be scanned for ambiguous wording. The requirements parser feeds an
// entire markdown table row as the Description, including the Priority cell, so
// a "Should" Priority value was misreported as ambiguous prose (issue #60).
// This drops cells that are exactly a MoSCoW priority token while keeping every
// other cell, so a genuine ambiguous word inside the description cell
// (e.g. "The system should log...") still triggers a warning.
func ambiguousScanText(description string) string {
	if !strings.Contains(description, "|") {
		return description
	}
	cells := strings.Split(description, "|")
	kept := make([]string, 0, len(cells))
	for _, cell := range cells {
		if moscowPriorities[strings.ToLower(strings.TrimSpace(cell))] {
			continue
		}
		kept = append(kept, cell)
	}
	return strings.Join(kept, " ")
}

// ValidateSpec는 SpecDocument의 유효성을 검증한다.
func ValidateSpec(doc *SpecDocument) []ValidationError {
	var errs []ValidationError

	// 필수 필드 검사
	if doc.ID == "" {
		errs = append(errs, ValidationError{
			Field:   "id",
			Message: "SPEC ID가 없습니다",
			Level:   "error",
		})
	}

	if doc.Title == "" {
		errs = append(errs, ValidationError{
			Field:   "title",
			Message: "SPEC 제목이 없습니다",
			Level:   "error",
		})
	}

	// 요구사항 섹션 검사
	if len(doc.Requirements) == 0 {
		errs = append(errs, ValidationError{
			Field:   "requirements",
			Message: "요구사항이 없습니다",
			Level:   "error",
		})
	}

	// 인수 기준 검사
	if len(doc.AcceptanceCriteria) == 0 {
		errs = append(errs, ValidationError{
			Field:   "acceptance_criteria",
			Message: "인수 기준이 없습니다",
			Level:   "error",
		})
	}

	// 모호한 언어 검사
	for _, req := range doc.Requirements {
		lower := strings.ToLower(ambiguousScanText(req.Description))
		for _, word := range ambiguousWords {
			if strings.Contains(lower, word) {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("requirement.%s", req.ID),
					Message: fmt.Sprintf("요구사항 %s에 모호한 언어 '%s'가 포함되어 있습니다", req.ID, word),
					Level:   "warning",
				})
				break
			}
		}
	}

	// SPEC-SPECREV-002 REQ-004: surface requirement-looking SHALL lines that no
	// EARS type recognized as warning-level errors so they are not silently
	// dropped. Flows to `auto spec validate` stderr like other warnings.
	if doc.RawContent != "" {
		if _, warnings, _ := ParseEARSWithWarnings(doc.RawContent); len(warnings) > 0 {
			for _, w := range warnings {
				errs = append(errs, ValidationError{
					Field:   "requirements",
					Message: w,
					Level:   "warning",
				})
			}
		}
	}

	return errs
}
