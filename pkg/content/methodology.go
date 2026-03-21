package content

import (
	"fmt"
	"io/fs"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// MethodologyDef는 방법론 정의이다.
type MethodologyDef struct {
	// Name은 방법론 이름이다.
	Name string `yaml:"name"`
	// Stages는 방법론 단계 목록이다.
	Stages []Stage `yaml:"stages"`
	// EnforceRules는 강제 적용 규칙이다.
	EnforceRules []string `yaml:"enforce_rules"`
	// ReviewGate는 리뷰 게이트 활성화 여부이다.
	ReviewGate bool `yaml:"review_gate"`
}

// Stage는 방법론의 단일 단계이다.
type Stage struct {
	// Name은 단계 이름이다.
	Name string `yaml:"name"`
	// Description은 단계 설명이다.
	Description string `yaml:"description"`
	// Rules는 단계별 규칙이다.
	Rules []string `yaml:"rules"`
	// RequiredBefore는 이전에 완료해야 할 단계이다.
	RequiredBefore string `yaml:"required_before"`
}

// LoadMethodology는 YAML 파일에서 방법론을 로드한다.
func LoadMethodology(path string) (*MethodologyDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("방법론 파일 읽기 실패 %s: %w", path, err)
	}

	var def MethodologyDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("YAML 파싱 실패: %w", err)
	}

	return &def, nil
}

// LoadMethodologyFromFS loads methodology from an embedded filesystem.
func LoadMethodologyFromFS(fsys fs.FS, path string) (*MethodologyDef, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("방법론 파일 읽기 실패 %s: %w", path, err)
	}

	var def MethodologyDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("YAML 파싱 실패: %w", err)
	}

	return &def, nil
}

// GenerateInstruction은 방법론 정의에서 지침 텍스트를 생성한다.
// TDD: "테스트 전 코드 작성 시 거부" 규칙 포함
// DDD: ANALYZE-PRESERVE-IMPROVE 사이클
// DD: Discover→Define→Develop→Deliver
func GenerateInstruction(def *MethodologyDef) string {
	switch strings.ToLower(def.Name) {
	case "tdd":
		return generateTDDInstruction(def)
	case "ddd":
		return generateDDDInstruction(def)
	case "double-diamond", "dd":
		return generateDoubleDiamondInstruction(def)
	default:
		return generateGenericInstruction(def)
	}
}

// generateTDDInstruction은 TDD 지침을 생성한다.
func generateTDDInstruction(def *MethodologyDef) string {
	var sb strings.Builder
	sb.WriteString("# TDD (Test-Driven Development) 방법론\n\n")
	sb.WriteString("## 핵심 원칙\n\n")
	sb.WriteString("**테스트 전 코드 작성 시 거부** — RED-GREEN-REFACTOR 사이클을 반드시 준수합니다.\n\n")

	sb.WriteString("## 단계\n\n")
	for _, stage := range def.Stages {
		sb.WriteString(fmt.Sprintf("### %s\n", strings.ToUpper(stage.Name)))
		sb.WriteString(fmt.Sprintf("%s\n\n", stage.Description))
		for _, rule := range stage.Rules {
			sb.WriteString(fmt.Sprintf("- %s\n", rule))
		}
		sb.WriteString("\n")
	}

	if len(def.EnforceRules) > 0 {
		sb.WriteString("## 강제 규칙\n\n")
		for _, rule := range def.EnforceRules {
			sb.WriteString(fmt.Sprintf("- %s\n", rule))
		}
		sb.WriteString("\n")
	}

	if def.ReviewGate {
		sb.WriteString("## 리뷰 게이트\n\n")
		sb.WriteString("각 단계 완료 후 리뷰 게이트를 통과해야 다음 단계로 진행할 수 있습니다.\n")
	}

	return sb.String()
}

// generateDDDInstruction은 DDD 지침을 생성한다.
func generateDDDInstruction(def *MethodologyDef) string {
	var sb strings.Builder
	sb.WriteString("# DDD (Disciplined Design Development) 방법론\n\n")
	sb.WriteString("## ANALYZE-PRESERVE-IMPROVE 사이클\n\n")
	sb.WriteString("기존 코드를 분석하고, 동작을 보존하며, 점진적으로 개선합니다.\n\n")

	sb.WriteString("## 단계\n\n")
	for _, stage := range def.Stages {
		label := strings.ToUpper(stage.Name)
		sb.WriteString(fmt.Sprintf("### %s\n", label))
		sb.WriteString(fmt.Sprintf("%s\n\n", stage.Description))
	}

	if len(def.EnforceRules) > 0 {
		sb.WriteString("## 강제 규칙\n\n")
		for _, rule := range def.EnforceRules {
			sb.WriteString(fmt.Sprintf("- %s\n", rule))
		}
	}

	return sb.String()
}

// generateDoubleDiamondInstruction은 Double Diamond 지침을 생성한다.
func generateDoubleDiamondInstruction(def *MethodologyDef) string {
	var sb strings.Builder
	sb.WriteString("# Double Diamond 방법론\n\n")
	sb.WriteString("## Discover → Define → Develop → Deliver\n\n")
	sb.WriteString("문제를 발산적으로 탐색하고 수렴하는 4단계 프로세스입니다.\n\n")

	sb.WriteString("## 단계\n\n")
	labels := map[string]string{
		"discover": "Discover",
		"define":   "Define",
		"develop":  "Develop",
		"deliver":  "Deliver",
	}

	for _, stage := range def.Stages {
		label := stage.Name
		if l, ok := labels[strings.ToLower(stage.Name)]; ok {
			label = l
		}
		sb.WriteString(fmt.Sprintf("### %s\n", label))
		sb.WriteString(fmt.Sprintf("%s\n\n", stage.Description))
	}

	return sb.String()
}

// generateGenericInstruction은 일반 방법론 지침을 생성한다.
func generateGenericInstruction(def *MethodologyDef) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s 방법론\n\n", def.Name))
	sb.WriteString("## 단계\n\n")
	for _, stage := range def.Stages {
		sb.WriteString(fmt.Sprintf("### %s\n", stage.Name))
		sb.WriteString(fmt.Sprintf("%s\n\n", stage.Description))
		for _, rule := range stage.Rules {
			sb.WriteString(fmt.Sprintf("- %s\n", rule))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
