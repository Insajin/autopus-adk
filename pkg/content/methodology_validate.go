package content

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ValidateMethodologyContent는 콘텐츠 디렉터리의 방법론 정의 전부를 검증한다.
// 생성 파이프라인이 검증되지 않은 정의를 템플릿으로 흘려보내지 않게 하는
// fail-closed 관문이다. 방법론 디렉터리가 없으면 검증할 정의도 없으므로
// 통과시킨다.
func ValidateMethodologyContent(contentDir string) error {
	dir := filepath.Join(contentDir, MethodologyDirName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("방법론 디렉터리 읽기 실패 %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		stem := strings.TrimSuffix(entry.Name(), ".yaml")

		def, err := LoadMethodology(filepath.Join(dir, entry.Name()))
		if err != nil {
			return err
		}
		// mode는 파일 이름으로 해석되므로, 정의 이름이 다르면 게이트가 보고하는
		// 이름과 전달되는 지침의 제목이 어긋난다.
		if name := strings.TrimSpace(def.Name); name != "" && name != stem {
			return fmt.Errorf("방법론 %s: name %q가 파일 이름과 일치하지 않습니다", entry.Name(), name)
		}
		if def.Name == "" {
			def.Name = stem
		}
		if _, err := OrderStages(def); err != nil {
			return fmt.Errorf("방법론 %s: %w", entry.Name(), err)
		}
	}
	return nil
}

// MethodologyDefect는 방법론 설정·정의 결함의 종류이다.
// 게이트 메시지는 이 값과 원인 단계 이름을 함께 보고한다.
type MethodologyDefect string

const (
	// DefectUnknownMode는 mode가 로드 가능한 정의로 해석되지 않는 경우이다.
	DefectUnknownMode MethodologyDefect = "unknown_mode"
	// DefectEnforceWithoutMode는 enforce: true인데 mode가 none인 경우이다.
	DefectEnforceWithoutMode MethodologyDefect = "enforce_without_mode"
	// DefectNoStages는 단계가 하나도 정의되지 않은 경우이다.
	DefectNoStages MethodologyDefect = "no_stages"
	// DefectEmptyStageName은 이름이 빈 단계가 있는 경우이다.
	DefectEmptyStageName MethodologyDefect = "empty_stage_name"
	// DefectDuplicateStage는 단계 이름이 중복된 경우이다.
	DefectDuplicateStage MethodologyDefect = "duplicate_stage"
	// DefectDanglingRequirement는 required_before가 존재하지 않는 단계를 가리키는 경우이다.
	DefectDanglingRequirement MethodologyDefect = "dangling_required_before"
	// DefectCycle은 required_before 체인에 순환이 있는 경우이다.
	DefectCycle MethodologyDefect = "cycle"
	// DefectMultipleRoots는 선행 단계가 없는 시작 단계가 둘 이상인 경우이다.
	DefectMultipleRoots MethodologyDefect = "multiple_roots"
	// DefectAmbiguousOrder는 같은 선행 단계를 요구하는 단계가 둘 이상이어서
	// 선형 순서가 결정되지 않는 경우이다.
	DefectAmbiguousOrder MethodologyDefect = "ambiguous_order"
)

// MethodologyError는 방법론 해석·검증 실패이다.
// Defect가 실패 종류, Stage가 원인 단계, Requires가 문제된 선행 단계 참조이다.
type MethodologyError struct {
	// Defect는 실패 종류이다.
	Defect MethodologyDefect
	// Methodology는 대상 방법론 이름(또는 설정된 mode)이다.
	Methodology string
	// Stage는 결함의 원인이 된 단계 이름이다.
	Stage string
	// Conflicts는 Stage와 순서가 충돌하는 단계 이름이다.
	Conflicts string
	// Requires는 문제된 required_before 참조 값이다.
	Requires string
	// Cause는 하위 원인 오류이다.
	Cause error
}

// Error는 결함 종류와 원인 단계를 명시한 메시지를 반환한다.
func (e *MethodologyError) Error() string {
	subject := e.Methodology
	if subject == "" {
		subject = "(unnamed)"
	}
	switch e.Defect {
	case DefectUnknownMode:
		return fmt.Sprintf("methodology[%s] %s: 로드 가능한 정의가 없습니다 (content/methodology/%s.yaml)",
			e.Defect, subject, subject)
	case DefectEnforceWithoutMode:
		return fmt.Sprintf("methodology[%s]: enforce: true이지만 mode가 %q이므로 전달할 규칙이 없습니다",
			e.Defect, MethodologyModeNone)
	case DefectNoStages:
		return fmt.Sprintf("methodology[%s] %s: 단계가 정의되지 않았습니다", e.Defect, subject)
	case DefectEmptyStageName:
		return fmt.Sprintf("methodology[%s] %s: 이름이 빈 단계가 있습니다", e.Defect, subject)
	case DefectDuplicateStage:
		return fmt.Sprintf("methodology[%s] %s: 단계 이름 %q가 중복됩니다", e.Defect, subject, e.Stage)
	case DefectDanglingRequirement:
		return fmt.Sprintf("methodology[%s] %s: 단계 %q의 required_before %q가 존재하지 않습니다",
			e.Defect, subject, e.Stage, e.Requires)
	case DefectCycle:
		return fmt.Sprintf("methodology[%s] %s: 단계 %q가 required_before 순환에 포함됩니다",
			e.Defect, subject, e.Stage)
	case DefectMultipleRoots:
		return fmt.Sprintf("methodology[%s] %s: 시작 단계가 둘 이상입니다 (%q가 두 번째 시작 단계)",
			e.Defect, subject, e.Stage)
	case DefectAmbiguousOrder:
		return fmt.Sprintf("methodology[%s] %s: 단계 %q와 %q가 같은 선행 단계 %q를 요구해 순서가 결정되지 않습니다",
			e.Defect, subject, e.Conflicts, e.Stage, e.Requires)
	default:
		return fmt.Sprintf("methodology[%s] %s: 정의 검증 실패", e.Defect, subject)
	}
}

// Unwrap은 하위 원인 오류를 반환한다.
func (e *MethodologyError) Unwrap() error { return e.Cause }

// OrderStages는 required_before 체인을 선형 순서로 해석한다.
// 중복 이름, 존재하지 않는 선행 단계, 순환, 복수 시작 단계, 순서가 결정되지 않는
// 분기를 각각 원인 단계 이름과 함께 거부한다.
func OrderStages(def *MethodologyDef) ([]Stage, error) {
	if def == nil || len(def.Stages) == 0 {
		name := ""
		if def != nil {
			name = def.Name
		}
		return nil, &MethodologyError{Defect: DefectNoStages, Methodology: name}
	}

	index := make(map[string]int, len(def.Stages))
	for i := range def.Stages {
		name := strings.TrimSpace(def.Stages[i].Name)
		if name == "" {
			return nil, &MethodologyError{Defect: DefectEmptyStageName, Methodology: def.Name}
		}
		if _, duplicate := index[name]; duplicate {
			return nil, &MethodologyError{Defect: DefectDuplicateStage, Methodology: def.Name, Stage: name}
		}
		index[name] = i
	}

	// 각 단계는 선행 단계를 최대 하나 가리키므로, 체인은 successor 맵 한 개로
	// 표현된다. 같은 선행 단계를 두 단계가 요구하면 순서가 결정되지 않는다.
	root := -1
	successor := make(map[string]int, len(def.Stages))
	for i := range def.Stages {
		name := strings.TrimSpace(def.Stages[i].Name)
		requires := strings.TrimSpace(def.Stages[i].RequiredBefore)
		if requires == "" {
			if root >= 0 {
				return nil, &MethodologyError{Defect: DefectMultipleRoots, Methodology: def.Name, Stage: name}
			}
			root = i
			continue
		}
		if _, known := index[requires]; !known {
			return nil, &MethodologyError{
				Defect: DefectDanglingRequirement, Methodology: def.Name, Stage: name, Requires: requires,
			}
		}
		if prior, taken := successor[requires]; taken {
			return nil, &MethodologyError{
				Defect:      DefectAmbiguousOrder,
				Methodology: def.Name,
				Stage:       name,
				Conflicts:   strings.TrimSpace(def.Stages[prior].Name),
				Requires:    requires,
			}
		}
		successor[requires] = i
	}
	if root < 0 {
		return nil, &MethodologyError{
			Defect: DefectCycle, Methodology: def.Name, Stage: strings.TrimSpace(def.Stages[0].Name),
		}
	}

	// root는 어떤 successor 값도 아니고 각 단계는 최대 한 번만 successor 값이
	// 되므로, 이 순회는 최대 len(Stages) 단계에서 종료된다.
	ordered := make([]Stage, 0, len(def.Stages))
	reached := make(map[string]bool, len(def.Stages))
	for cursor := root; ; {
		name := strings.TrimSpace(def.Stages[cursor].Name)
		ordered = append(ordered, def.Stages[cursor])
		reached[name] = true
		next, ok := successor[name]
		if !ok {
			break
		}
		cursor = next
	}
	if len(ordered) != len(def.Stages) {
		// 시작 단계에서 도달할 수 없는 단계들은 서로를 선행 단계로 요구한다.
		return nil, &MethodologyError{
			Defect: DefectCycle, Methodology: def.Name, Stage: firstUnreachedStage(def.Stages, reached),
		}
	}
	return ordered, nil
}

// firstUnreachedStage는 선언 순서상 처음으로 도달되지 않은 단계 이름을 반환한다.
func firstUnreachedStage(stages []Stage, reached map[string]bool) string {
	for i := range stages {
		name := strings.TrimSpace(stages[i].Name)
		if !reached[name] {
			return name
		}
	}
	return ""
}
