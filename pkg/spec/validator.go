package spec

import (
	"fmt"
	"strings"
)

// 모호한 언어 목록
var ambiguousWords = []string{"should", "might", "could", "possibly", "maybe", "perhaps"}

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
		lower := strings.ToLower(req.Description)
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

	return errs
}
