package spec_test

import (
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
