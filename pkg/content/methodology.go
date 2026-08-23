package content

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

// MethodologyDirName은 콘텐츠 트리에서 방법론 정의가 놓이는 디렉터리이다.
const MethodologyDirName = "methodology"

// MethodologyModeNone은 방법론을 사용하지 않는 mode 값이다.
// 빈 mode도 같은 의미로 취급한다.
const MethodologyModeNone = "none"

// MethodologyDef는 방법론 정의이다.
type MethodologyDef struct {
	// Name은 방법론 이름이다.
	Name string `yaml:"name"`
	// Stages는 방법론 단계 목록이다.
	Stages []Stage `yaml:"stages"`
	// EnforceRules는 강제 적용 규칙이다.
	EnforceRules []string `yaml:"enforce_rules"`
	// ReviewGate는 단계별 리뷰 요구 여부이다.
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
func LoadMethodology(filePath string) (*MethodologyDef, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("방법론 파일 읽기 실패 %s: %w", filePath, err)
	}
	return parseMethodology(data)
}

// LoadMethodologyFromFS loads methodology from an embedded filesystem.
func LoadMethodologyFromFS(fsys fs.FS, filePath string) (*MethodologyDef, error) {
	data, err := fs.ReadFile(fsys, filePath)
	if err != nil {
		return nil, fmt.Errorf("방법론 파일 읽기 실패 %s: %w", filePath, err)
	}
	return parseMethodology(data)
}

// parseMethodology는 방법론 YAML 본문을 파싱한다.
func parseMethodology(data []byte) (*MethodologyDef, error) {
	var def MethodologyDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("YAML 파싱 실패: %w", err)
	}
	return &def, nil
}

// NormalizeMethodologyMode는 설정된 mode 값을 정규화한다.
// 빈 값은 MethodologyModeNone으로 취급한다.
func NormalizeMethodologyMode(mode string) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == "" {
		return MethodologyModeNone
	}
	return normalized
}

// MethodologyPath는 mode에 대응하는 정의 파일의 콘텐츠 상대 경로이다.
func MethodologyPath(mode string) string {
	return path.Join(MethodologyDirName, NormalizeMethodologyMode(mode)+".yaml")
}

// ResolveMethodology는 설정된 mode를 로드 가능하고 검증된 방법론으로 해석한다.
// mode가 none(또는 빈 값)이면 (nil, nil)을 반환한다. 그 외의 mode는 정의가
// 존재하고 required_before 체인이 선형 순서를 이룰 때만 성공한다.
// 반환된 정의의 Stages는 required_before 순서로 정렬되어 있다.
func ResolveMethodology(fsys fs.FS, mode string) (*MethodologyDef, error) {
	normalized := NormalizeMethodologyMode(mode)
	if normalized == MethodologyModeNone {
		return nil, nil
	}
	// mode는 사용자 설정 값이므로 경로 탈출을 먼저 거부한다.
	if err := validateName(normalized); err != nil {
		return nil, &MethodologyError{Defect: DefectUnknownMode, Methodology: normalized, Cause: err}
	}

	def, err := LoadMethodologyFromFS(fsys, MethodologyPath(normalized))
	if err != nil {
		return nil, &MethodologyError{Defect: DefectUnknownMode, Methodology: normalized, Cause: err}
	}
	if strings.TrimSpace(def.Name) == "" {
		def.Name = normalized
	}

	ordered, err := OrderStages(def)
	if err != nil {
		return nil, err
	}
	def.Stages = ordered
	return def, nil
}

// ResolveEnforcedMethodology는 methodology 설정 한 쌍(mode, enforce)을 검증한다.
// enforce가 true이면 mode는 반드시 로드 가능하고 검증된 정의로 해석되어야 한다.
// mode가 none인 채로 enforce가 true이면 전달할 규칙이 없으므로 실패한다.
func ResolveEnforcedMethodology(fsys fs.FS, mode string, enforce bool) (*MethodologyDef, error) {
	def, err := ResolveMethodology(fsys, mode)
	if err != nil {
		return nil, err
	}
	if def == nil && enforce {
		return nil, &MethodologyError{Defect: DefectEnforceWithoutMode, Methodology: MethodologyModeNone}
	}
	return def, nil
}

// GenerateInstruction은 방법론 정의를 에이전트에게 전달할 지침 텍스트로 렌더한다.
// 단계는 def.Stages에 담긴 순서 그대로 렌더되며, ResolveMethodology가 반환한
// 정의는 이미 required_before 순서로 정렬되어 있다.
//
// 이 텍스트는 규칙을 전달할 뿐이다. 하네스는 설정(mode와 단계 순서)만 검증하고
// 규칙 준수 여부는 관측하지 않으므로, 관측을 전제한 게이트 문구를 쓰지 않는다.
func GenerateInstruction(def *MethodologyDef) string {
	if def == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s 방법론\n\n", def.Name))
	sb.WriteString(methodologyDeliveryNotice)

	if cycle := stageCycleLine(def.Stages); cycle != "" {
		sb.WriteString(fmt.Sprintf("**사이클**: %s\n\n", cycle))
	}

	if len(def.EnforceRules) > 0 {
		sb.WriteString("## 강제 규칙\n\n")
		for _, rule := range def.EnforceRules {
			sb.WriteString(fmt.Sprintf("- %s\n", rule))
		}
		sb.WriteString("\n")
	}

	if len(def.Stages) > 0 {
		sb.WriteString("## 단계\n\n")
		for i, stage := range def.Stages {
			sb.WriteString(fmt.Sprintf("### Phase %d: %s\n", i+1, stage.Name))
			if stage.Description != "" {
				sb.WriteString(fmt.Sprintf("%s\n\n", stage.Description))
			}
			for _, rule := range stage.Rules {
				sb.WriteString(fmt.Sprintf("- %s\n", rule))
			}
			sb.WriteString("\n")
		}
	}

	if def.ReviewGate {
		sb.WriteString("## 단계별 리뷰\n\n")
		sb.WriteString("이 방법론은 각 단계 종료 시 리뷰를 요구합니다. 하네스는 리뷰 수행을 관측하지 않으므로, ")
		sb.WriteString("에이전트가 단계 종료 시 리뷰 결과를 스스로 보고해야 합니다.\n")
	}

	return sb.String()
}

// methodologyDeliveryNotice는 전달과 관측을 혼동할 수 없게 만드는 고정 문구이다.
const methodologyDeliveryNotice = "아래 규칙은 에이전트가 준수해야 하는 구속력 있는 지침으로 전달됩니다.\n" +
	"하네스가 검증하는 것은 설정(mode 해석 가능성과 단계 순서의 정합성)뿐이며, 규칙 준수 여부는 관측하지 않습니다.\n" +
	"따라서 각 단계의 수행과 보고 책임은 에이전트에게 있습니다.\n\n"

// stageCycleLine은 단계 이름을 대문자 화살표 표기로 잇는다.
func stageCycleLine(stages []Stage) string {
	if len(stages) == 0 {
		return ""
	}
	names := make([]string, 0, len(stages))
	for _, stage := range stages {
		names = append(names, strings.ToUpper(stage.Name))
	}
	return strings.Join(names, " → ")
}
