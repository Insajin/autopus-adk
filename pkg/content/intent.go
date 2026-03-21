package content

import (
	"fmt"
	"strings"
)

// IntentRule은 인텐트 라우팅 규칙이다.
type IntentRule struct {
	// Pattern은 매칭 패턴 (정규식)이다.
	Pattern string
	// TargetSkill은 대상 스킬 이름이다 (TargetAgent와 상호 배타적).
	TargetSkill string
	// TargetAgent는 대상 에이전트 이름이다 (TargetSkill과 상호 배타적).
	TargetAgent string
	// Priority는 규칙 우선순위이다 (높을수록 먼저 평가).
	Priority int
}

// DefaultRules는 기본 인텐트 라우팅 규칙을 반환한다.
func DefaultRules() []IntentRule {
	return []IntentRule{
		{Pattern: `plan.*feature|기능.*기획|feature.*plan`, TargetSkill: "planning", Priority: 10},
		{Pattern: `debug.*error|error.*fix|버그.*수정|수정.*버그`, TargetAgent: "debugger", Priority: 20},
		{Pattern: `test.*write|write.*test|테스트.*작성|작성.*테스트`, TargetSkill: "tdd", Priority: 15},
		{Pattern: `architect.*design|design.*arch|아키텍처.*설계`, TargetAgent: "architect", Priority: 18},
		{Pattern: `security.*audit|audit.*security|보안.*감사`, TargetAgent: "security-auditor", Priority: 25},
		{Pattern: `review.*code|code.*review|코드.*리뷰`, TargetAgent: "reviewer", Priority: 12},
		{Pattern: `brainstorm|아이디어.*발산|발산.*아이디어`, TargetSkill: "brainstorming", Priority: 8},
		{Pattern: `commit.*message|커밋.*메시지`, TargetSkill: "lore-commit", Priority: 14},
		{Pattern: `refactor.*code|코드.*리팩토링`, TargetSkill: "ast-refactoring", Priority: 11},
		{Pattern: `search.*context|컨텍스트.*검색`, TargetSkill: "context-search", Priority: 9},
		{Pattern: `spec.*작성|spec.*생성|SPEC.*write|SPEC.*create|스펙.*작성`, TargetAgent: "spec-writer", Priority: 16},
		{Pattern: `spec.*review|spec.*리뷰|SPEC.*리뷰|리뷰.*게이트|review.*gate`, TargetSkill: "spec-review", Priority: 17},
	}
}

// GenerateIntentGateInstruction은 인텐트 게이트 지침 텍스트를 생성한다.
func GenerateIntentGateInstruction(rules []IntentRule) string {
	var sb strings.Builder

	sb.WriteString("# Intent Gate Instructions\n\n")
	sb.WriteString("사용자 요청을 분석하여 적절한 스킬 또는 에이전트로 자동 라우팅합니다.\n\n")
	sb.WriteString("## 라우팅 규칙\n\n")
	sb.WriteString("우선순위 순서로 평가됩니다 (높은 숫자 = 높은 우선순위):\n\n")

	// 우선순위 내림차순 정렬
	sorted := sortRulesByPriority(rules)
	for _, rule := range sorted {
		target := ""
		if rule.TargetSkill != "" {
			target = fmt.Sprintf("→ 스킬: `%s`", rule.TargetSkill)
		} else if rule.TargetAgent != "" {
			target = fmt.Sprintf("→ 에이전트: `%s`", rule.TargetAgent)
		}
		sb.WriteString(fmt.Sprintf("- 패턴: `%s` %s (우선순위: %d)\n", rule.Pattern, target, rule.Priority))
	}

	sb.WriteString("\n## 적용 방법\n\n")
	sb.WriteString("1. 사용자 요청에서 키워드를 추출합니다\n")
	sb.WriteString("2. 우선순위 순서로 패턴을 매칭합니다\n")
	sb.WriteString("3. 매칭된 첫 번째 규칙의 스킬/에이전트를 활성화합니다\n")
	sb.WriteString("4. 매칭 규칙이 없으면 기본 워크플로우를 사용합니다\n")

	return sb.String()
}

// sortRulesByPriority는 규칙을 우선순위 내림차순으로 정렬한다.
func sortRulesByPriority(rules []IntentRule) []IntentRule {
	sorted := make([]IntentRule, len(rules))
	copy(sorted, rules)

	// 간단한 삽입 정렬 (규칙 수가 적어 효율 충분)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].Priority > sorted[j-1].Priority; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	return sorted
}
