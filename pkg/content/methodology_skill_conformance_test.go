// Package content_test는 방법론 YAML과 스킬 문서의 규칙 정합성 테스트이다.
package content_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/content"
)

// methodologySkillAnchors는 content/methodology/<name>.yaml의 enforce_rules 각
// 항목을, 그 규칙을 실제로 에이전트에게 전달하는 content/skills/<name>.md의
// 문장에 대응시킨다. 방법론 YAML은 게이트가 읽고 스킬 문서는 에이전트가 읽으므로,
// 한쪽만 고치면 이 표가 깨진다. 규칙이 조용히 사라지지 않게 하는 것이 목적이다.
//
// 각 항목은 {enforce_rules 원문, 스킬 문서에 그대로 존재해야 하는 문장}이다.
// 두 문서가 같은 문장을 쓰는 경우(ddd의 변환 크기 규칙)에는 규칙 원문이 앵커이다.
// 어느 쪽도 생성물이 아니므로 한쪽에서 다른 쪽을 생성하지 않는다.
var methodologySkillAnchors = map[string][][2]string{
	"tdd": {
		{"테스트 없이 코드 작성 금지", "테스트 없이 코드를 작성하지 않는다"},
		{"RED → GREEN → REFACTOR 순서 준수", "RED-GREEN-REFACTOR 사이클"},
		{"테스트 커버리지 85% 이상 유지", "테스트 커버리지 85% 이상"},
	},
	"ddd": {
		{"기존 동작을 보존하면서 개선", "보존하면서 점진적으로 개선"},
		{"특성 테스트로 기존 동작 고정", "기존 동작을 테스트로 고정합니다 (Characterization Tests)"},
		{"최대 변환 크기: small (50줄 미만)", "최대 변환 크기: small (50줄 미만)"},
	},
	"double-diamond": {
		{"수렴 전 충분한 인사이트 수집", "충분한 인사이트 수집"},
		{"사용자 관점에서 문제 정의", "[사용자]는 [맥락]에서 [목표]를 달성하려 하지만"},
	},
}

// methodologyRulesWithoutSkillCarrier는 스킬 문서가 아직 전달하지 않는 규칙이다.
//
// double-diamond의 "발산 단계에서 평가 금지 (아이디어 먼저)"는 스킬 문서에 대응
// 문장이 없다. Discover/Develop 절은 발산 기법(현장 조사, Crazy 8s)만 나열하고
// 발산 중 평가를 미루라는 규율은 한 문장도 쓰지 않는다. 규칙이 스킬에 어울리지
// 않아서가 아니라 스킬이 이 규칙을 누락한 상태이므로, 문안 추가는 콘텐츠 소유자의
// 결정으로 남기고 여기서는 예외로 기록한다. 스킬이 이 규칙을 전달하기 시작하면
// 아래 자기 폐기 검사가 이 예외를 지우라고 실패한다.
var methodologyRulesWithoutSkillCarrier = map[string][]string{
	"double-diamond": {"발산 단계에서 평가 금지 (아이디어 먼저)"},
}

// TestMethodologyEnforceRulesReachTheSkill은 방법론 YAML이 선언한 강제 규칙이
// 에이전트가 읽는 스킬 문서에 실제로 전달되는지 양방향으로 검증한다.
func TestMethodologyEnforceRulesReachTheSkill(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..")
	methodologyDir := filepath.Join(repoRoot, "content", content.MethodologyDirName)

	entries, err := os.ReadDir(methodologyDir)
	require.NoError(t, err)

	checked := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		checked++
		name := strings.TrimSuffix(entry.Name(), ".yaml")

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			methodologyPath := filepath.Join(methodologyDir, entry.Name())
			skillPath := filepath.Join(repoRoot, "content", "skills", name+".md")

			def, err := content.LoadMethodology(methodologyPath)
			require.NoError(t, err)

			body, err := os.ReadFile(skillPath)
			require.NoError(t, err, "%s에 대응하는 스킬 문서가 없습니다: %s", methodologyPath, skillPath)
			skill := string(body)

			anchors := methodologySkillAnchors[name]
			exceptions := methodologyRulesWithoutSkillCarrier[name]

			for _, rule := range def.EnforceRules {
				if slices.Contains(exceptions, rule) {
					if strings.Contains(skill, rule) {
						t.Errorf("규칙 %q는 이제 %s가 전달합니다. methodologyRulesWithoutSkillCarrier[%q]에서 지우고 methodologySkillAnchors에 문장을 등록하세요",
							rule, skillPath, name)
					}
					continue
				}
				anchor, mapped := anchorFor(anchors, rule)
				if !mapped {
					t.Errorf("%s가 선언한 강제 규칙 %q를 전달하는 문장이 등록되지 않았습니다. %s가 이 규칙을 전달하도록 고치고 methodologySkillAnchors에 그 문장을 등록하세요",
						methodologyPath, rule, skillPath)
					continue
				}
				if !strings.Contains(skill, anchor) {
					t.Errorf("%s가 선언한 강제 규칙 %q가 %s에서 사라졌습니다 (찾던 문장: %q)",
						methodologyPath, rule, skillPath, anchor)
				}
			}

			// 반대 방향: YAML에서 규칙이 빠지면 남은 앵커와 예외가 실패한다.
			for _, pair := range anchors {
				if !slices.Contains(def.EnforceRules, pair[0]) {
					t.Errorf("methodologySkillAnchors[%q]에 남은 %q가 %s의 enforce_rules에 없습니다",
						name, pair[0], methodologyPath)
				}
			}
			for _, rule := range exceptions {
				if !slices.Contains(def.EnforceRules, rule) {
					t.Errorf("methodologyRulesWithoutSkillCarrier[%q]에 남은 %q가 %s의 enforce_rules에 없습니다",
						name, rule, methodologyPath)
				}
			}
		})
	}
	require.NotZero(t, checked, "검사한 방법론 정의가 없습니다: %s", methodologyDir)
}

// anchorFor는 규칙에 대응하는 스킬 문장을 찾는다.
func anchorFor(anchors [][2]string, rule string) (string, bool) {
	for _, pair := range anchors {
		if pair[0] == rule {
			return pair[1], true
		}
	}
	return "", false
}
