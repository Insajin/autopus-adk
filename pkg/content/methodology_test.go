// Package content_test는 방법론 콘텐츠 패키지의 테스트이다.
package content_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/content"
)

func TestLoadMethodology_TDD(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tddYAML := `name: tdd
review_gate: true
enforce_rules:
  - "테스트 없이 코드 작성 금지"
stages:
  - name: red
    description: 실패하는 테스트 작성
    rules:
      - "테스트 전 코드 작성 시 거부"
    required_before: ""
  - name: green
    description: 최소 구현으로 테스트 통과
    required_before: red
  - name: refactor
    description: 코드 정리
    required_before: green
`
	err := os.WriteFile(filepath.Join(dir, "tdd.yaml"), []byte(tddYAML), 0644)
	require.NoError(t, err)

	def, err := content.LoadMethodology(filepath.Join(dir, "tdd.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "tdd", def.Name)
	assert.True(t, def.ReviewGate)
	assert.Len(t, def.Stages, 3)
	assert.Equal(t, "red", def.Stages[0].Name)
}

func TestLoadMethodology_NotFound(t *testing.T) {
	t.Parallel()

	_, err := content.LoadMethodology("/nonexistent/path.yaml")
	assert.Error(t, err)
}

func TestGenerateInstruction_TDD(t *testing.T) {
	t.Parallel()

	def := &content.MethodologyDef{
		Name:         "tdd",
		ReviewGate:   true,
		EnforceRules: []string{"테스트 없이 코드 작성 금지"},
		Stages: []content.Stage{
			{Name: "red", Description: "실패하는 테스트 작성", Rules: []string{"테스트 전 코드 작성 시 거부"}},
			{Name: "green", Description: "최소 구현으로 테스트 통과"},
			{Name: "refactor", Description: "코드 정리"},
		},
	}

	instruction := content.GenerateInstruction(def)
	// TDD: "테스트 전 코드 작성 시 거부" 규칙 포함 필수
	assert.Contains(t, instruction, "테스트 전 코드 작성 시 거부")
	assert.Contains(t, instruction, "RED")
	assert.Contains(t, instruction, "GREEN")
	assert.Contains(t, instruction, "REFACTOR")
}

func TestGenerateInstruction_DDD(t *testing.T) {
	t.Parallel()

	def := &content.MethodologyDef{
		Name: "ddd",
		Stages: []content.Stage{
			{Name: "analyze", Description: "현재 동작 분석"},
			{Name: "preserve", Description: "기존 동작 보존"},
			{Name: "improve", Description: "개선"},
		},
	}

	instruction := content.GenerateInstruction(def)
	assert.Contains(t, instruction, "ANALYZE")
	assert.Contains(t, instruction, "PRESERVE")
	assert.Contains(t, instruction, "IMPROVE")
}

func TestLoadMethodologyFromFS(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"methodology/tdd.yaml": &fstest.MapFile{
			Data: []byte(`name: tdd
review_gate: true
enforce_rules:
  - "테스트 없이 코드 작성 금지"
stages:
  - name: red
    description: 실패하는 테스트 작성
  - name: green
    description: 최소 구현으로 테스트 통과
  - name: refactor
    description: 코드 정리
`),
		},
	}

	def, err := content.LoadMethodologyFromFS(fsys, "methodology/tdd.yaml")
	require.NoError(t, err)
	assert.Equal(t, "tdd", def.Name)
	assert.True(t, def.ReviewGate)
	assert.Len(t, def.Stages, 3)
	assert.Equal(t, "red", def.Stages[0].Name)
}

func TestLoadMethodologyFromFS_NotFound(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{}
	_, err := content.LoadMethodologyFromFS(fsys, "methodology/nonexistent.yaml")
	assert.Error(t, err)
}

func TestGenerateInstruction_DoubleDiamond(t *testing.T) {
	t.Parallel()

	def := &content.MethodologyDef{
		Name: "double-diamond",
		Stages: []content.Stage{
			{Name: "discover", Description: "문제 발견"},
			{Name: "define", Description: "문제 정의"},
			{Name: "develop", Description: "해결책 개발"},
			{Name: "deliver", Description: "최종 산출물"},
		},
	}

	instruction := content.GenerateInstruction(def)
	assert.Contains(t, instruction, "Discover")
	assert.Contains(t, instruction, "Define")
	assert.Contains(t, instruction, "Develop")
	assert.Contains(t, instruction, "Deliver")
}

func TestGenerateInstruction_Generic(t *testing.T) {
	t.Parallel()

	def := &content.MethodologyDef{
		Name: "custom-method",
		Stages: []content.Stage{
			{Name: "stage1", Description: "첫 번째 단계", Rules: []string{"규칙 A"}},
			{Name: "stage2", Description: "두 번째 단계"},
		},
	}

	instruction := content.GenerateInstruction(def)
	assert.Contains(t, instruction, "custom-method")
	assert.Contains(t, instruction, "stage1")
	assert.Contains(t, instruction, "첫 번째 단계")
	assert.Contains(t, instruction, "규칙 A")
}
