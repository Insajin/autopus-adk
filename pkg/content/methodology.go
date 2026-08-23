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

// deliveryPath는 방법론 교리가 에이전트에게 전달되는 스킬 파일의 콘텐츠
// 상대 경로이다. 정의(YAML)는 기계가 읽는 골격이고, 전달은 이 스킬이 한다.
func deliveryPath(mode string) string {
	return path.Join("skills", NormalizeMethodologyMode(mode)+".md")
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

// ResolveEnforcedMethodology는 methodology 설정이 온전한지 검증한다.
//
// ResolveMethodology가 "정의가 정합한가"(구조)를 보는 반면, 이 함수는 "설정이
// 무언가를 실제로 의미하는가"를 본다. 둘을 나눈 이유는 구조 검증이 콘텐츠 트리
// 전체를 전제하지 않아야 하기 때문이다 - 합성 FS로 체인만 시험하는 테스트가
// 스킬 파일까지 들고 있을 필요는 없다.
//
// mode가 none인 채로 enforce가 true이면 전달할 규칙이 없으므로 실패한다.
// 그리고 mode가 정해졌다면 enforce 여부와 무관하게 교리가 에이전트에게 닿아야
// 한다 - 전달은 같은 이름의 스킬이 담당하며, 그 파일이 없으면 설정은 아무도
// 읽지 않는 규칙을 선언하는 셈이다. 렌더러를 따로 두는 대신 이미 살아 있는
// 표면의 존재를 요구한다. 표면을 새로 만들면 그것이 곧 두 번째 출처가 된다.
func ResolveEnforcedMethodology(fsys fs.FS, mode string, enforce bool) (*MethodologyDef, error) {
	def, err := ResolveMethodology(fsys, mode)
	if err != nil {
		return nil, err
	}
	if def == nil {
		if enforce {
			return nil, &MethodologyError{Defect: DefectEnforceWithoutMode, Methodology: MethodologyModeNone}
		}
		return nil, nil
	}
	normalized := NormalizeMethodologyMode(mode)
	if _, statErr := fs.Stat(fsys, deliveryPath(normalized)); statErr != nil {
		return nil, &MethodologyError{
			Defect:      DefectMissingDelivery,
			Methodology: normalized,
			Cause:       statErr,
		}
	}
	return def, nil
}
