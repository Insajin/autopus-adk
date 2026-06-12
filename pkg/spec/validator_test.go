package spec_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/insajin/autopus-adk/pkg/spec"
)

func TestValidateSpec_ValidDocument(t *testing.T) {
	t.Parallel()

	doc := &spec.SpecDocument{
		ID:    "SPEC-AUTH-001",
		Title: "사용자 인증",
		Requirements: []spec.Requirement{
			{ID: "REQ-001", Type: spec.EARSUbiquitous, Description: "시스템은 SHALL 인증을 제공합니다."},
		},
		AcceptanceCriteria: []spec.Criterion{
			{ID: "AC-001", Description: "로그인이 성공해야 한다"},
		},
	}

	errs := spec.ValidateSpec(doc)
	// 오류 없어야 함 (경고는 있을 수 있음)
	for _, e := range errs {
		assert.NotEqual(t, "error", e.Level, "예상치 않은 오류: %s", e.Message)
	}
}

func TestValidateSpec_MissingID(t *testing.T) {
	t.Parallel()

	doc := &spec.SpecDocument{
		Title: "제목만 있음",
	}

	errs := spec.ValidateSpec(doc)
	assert.NotEmpty(t, errs)

	found := false
	for _, e := range errs {
		if e.Field == "id" && e.Level == "error" {
			found = true
		}
	}
	assert.True(t, found, "ID 누락 오류가 있어야 합니다")
}

func TestValidateSpec_MissingTitle(t *testing.T) {
	t.Parallel()

	doc := &spec.SpecDocument{
		ID: "SPEC-001",
	}

	errs := spec.ValidateSpec(doc)
	assert.NotEmpty(t, errs)

	found := false
	for _, e := range errs {
		if e.Field == "title" && e.Level == "error" {
			found = true
		}
	}
	assert.True(t, found, "Title 누락 오류가 있어야 합니다")
}

func TestValidateSpec_AmbiguousLanguageWarning(t *testing.T) {
	t.Parallel()

	doc := &spec.SpecDocument{
		ID:    "SPEC-AMB-001",
		Title: "모호한 언어 테스트",
		Requirements: []spec.Requirement{
			{ID: "REQ-001", Type: spec.EARSUbiquitous, Description: "시스템은 should 응답합니다."},
			{ID: "REQ-002", Type: spec.EARSEventDriven, Description: "WHEN 요청하면 THEN might 처리됩니다."},
		},
	}

	errs := spec.ValidateSpec(doc)

	// 모호한 언어 경고 확인
	warnings := 0
	for _, e := range errs {
		if e.Level == "warning" {
			warnings++
		}
	}
	assert.Greater(t, warnings, 0, "모호한 언어에 대한 경고가 있어야 합니다")
}

// Issue #60: a requirements-table row whose Priority column is the MoSCoW token
// "Should" must NOT raise an ambiguous-language warning. The description cell
// itself carries no ambiguous wording.
func TestValidateSpec_PriorityShouldColumn_NoAmbiguousWarning(t *testing.T) {
	t.Parallel()

	doc := &spec.SpecDocument{
		ID:    "SPEC-PRIO-001",
		Title: "Priority 열 오탐",
		Requirements: []spec.Requirement{
			// Parser feeds the entire markdown table row as Description,
			// including the Priority cell "Should".
			{ID: "REQ-001", Type: spec.EARSEventDriven,
				Description: "| REQ-001 | Event-driven | Should | WHEN 요청이 도착하면, THE SYSTEM SHALL 기록한다 |"},
		},
		AcceptanceCriteria: []spec.Criterion{
			{ID: "AC-001", Description: "기록된다"},
		},
	}

	errs := spec.ValidateSpec(doc)
	for _, e := range errs {
		assert.NotContains(t, e.Message, "모호한 언어",
			"Priority column 'Should' must not trigger an ambiguous-language warning (issue #60): %s", e.Message)
	}
}

// Issue #60 regression guard: a genuine ambiguous word inside the description
// cell (not the Priority column) must still warn.
func TestValidateSpec_AmbiguousWordInDescriptionCell_StillWarns(t *testing.T) {
	t.Parallel()

	doc := &spec.SpecDocument{
		ID:    "SPEC-PRIO-002",
		Title: "진짜 모호어",
		Requirements: []spec.Requirement{
			{ID: "REQ-001", Type: spec.EARSUbiquitous,
				Description: "| REQ-001 | Ubiquitous | Must | The system should log the request |"},
		},
		AcceptanceCriteria: []spec.Criterion{
			{ID: "AC-001", Description: "기록된다"},
		},
	}

	errs := spec.ValidateSpec(doc)
	found := false
	for _, e := range errs {
		if e.Level == "warning" && strings.Contains(e.Message, "모호한 언어") {
			found = true
		}
	}
	assert.True(t, found, "real ambiguous 'should' inside the description cell must still warn")
}

func TestValidateSpec_EmptyAcceptanceCriteria_Error(t *testing.T) {
	t.Parallel()

	doc := &spec.SpecDocument{
		ID:    "SPEC-GATE-001",
		Title: "Acceptance Gate",
		Requirements: []spec.Requirement{
			{ID: "REQ-001", Type: spec.EARSUbiquitous, Description: "시스템은 SHALL 게이트를 검증합니다."},
		},
		// AcceptanceCriteria intentionally empty
	}

	errs := spec.ValidateSpec(doc)

	found := false
	for _, e := range errs {
		if e.Field == "acceptance_criteria" && e.Level == "error" {
			found = true
		}
	}
	assert.True(t, found, "empty acceptance criteria must produce Level='error'")
}

func TestValidateSpec_EmptyRequirements(t *testing.T) {
	t.Parallel()

	doc := &spec.SpecDocument{
		ID:    "SPEC-EMPTY-001",
		Title: "빈 요구사항",
	}

	errs := spec.ValidateSpec(doc)

	found := false
	for _, e := range errs {
		if e.Field == "requirements" {
			found = true
		}
	}
	assert.True(t, found)
}

// S12 (SPEC-SPECREV-002 REQ-004): ValidateSpec surfaces an unrecognized SHALL
// line in RawContent as exactly one warning-level ValidationError.
func TestValidateSpec_SurfacesUnrecognizedShallWarning(t *testing.T) {
	t.Parallel()

	doc := &spec.SpecDocument{
		ID:    "SPEC-X-001",
		Title: "제목",
		Requirements: []spec.Requirement{
			{ID: "REQ-001", Type: spec.EARSEventDriven, Description: "WHEN x THEN y"},
		},
		AcceptanceCriteria: []spec.Criterion{
			{ID: "AC-001", Description: "ok"},
		},
		RawContent: "The button SHALL respond to the click",
	}

	errs := spec.ValidateSpec(doc)

	count := 0
	for _, e := range errs {
		if e.Level == "warning" && strings.Contains(e.Message, "The button SHALL respond to the click") {
			count++
		}
	}
	assert.Equal(t, 1, count, "exactly one warning naming the unrecognized SHALL line")
}
