# SPEC-OSSUX-001 Implementation Plan

**SPEC**: SPEC-OSSUX-001
**Status**: draft
**Created**: 2026-04-02

## Task Overview

| Task ID | Description | Agent | Mode | Dependencies | File Ownership |
|---------|-------------|-------|------|--------------|----------------|
| T1 | autopus.yaml 스키마 확장 — UsageProfile, HintsConf 필드 추가 | executor | parallel | — | pkg/config/schema.go, pkg/config/schema_profile.go |
| T2 | 상태 파일 관리 — ~/.autopus/state.json 읽기/쓰기 | executor | parallel | — | pkg/hint/state.go, pkg/hint/state_test.go |
| T3 | Init Wizard 프로파일 스텝 추가 — Developer/Fullstack 선택 | executor | sequential | T1 | internal/cli/tui/wizard_steps.go, internal/cli/tui/wizard_profile.go |
| T4 | 힌트 표시 로직 — auto go 완료 후 조건부 힌트 | executor | sequential | T1, T2 | pkg/hint/hint.go, pkg/hint/hint_test.go |
| T5 | auto config set hints.platform 지원 | executor | sequential | T1 | internal/cli/config_cmd.go (또는 기존 config 커맨드 파일) |

## Execution Order

```
Phase 1 (parallel):
  ├── T1: Schema 확장
  └── T2: State 파일 관리

Phase 2 (sequential, after Phase 1):
  ├── T3: Init Wizard 프로파일 스텝 (depends on T1)
  ├── T4: 힌트 표시 로직 (depends on T1, T2)
  └── T5: config set hints.platform (depends on T1)
```

## Task Details

### T1: autopus.yaml 스키마 확장

**목표**: HarnessConfig에 `UsageProfile` 필드와 `Hints` 설정 구조체를 추가한다.

**변경 파일**:
- `pkg/config/schema.go` — HarnessConfig에 UsageProfile, Hints 필드 추가
- `pkg/config/schema_profile.go` — UsageProfile 타입 정의, HintsConf 구조체, 기본값 함수

**구현 상세**:
```go
// pkg/config/schema_profile.go (신규)

// UsageProfile represents the user's intended usage of ADK.
type UsageProfile string

const (
    ProfileDeveloper UsageProfile = "developer"
    ProfileFullstack UsageProfile = "fullstack"
)

// HintsConf holds platform hint configuration.
type HintsConf struct {
    Platform *bool `yaml:"platform,omitempty"` // nil = enabled (default), false = disabled
}

// IsPlatformHintEnabled returns whether platform hints are enabled.
func (h HintsConf) IsPlatformHintEnabled() bool {
    if h.Platform == nil {
        return true // default: enabled
    }
    return *h.Platform
}
```

```go
// pkg/config/schema.go — HarnessConfig에 추가
UsageProfile UsageProfile `yaml:"usage_profile,omitempty"` // developer (default) or fullstack
Hints        HintsConf    `yaml:"hints,omitempty"`
```

**Validate() 확장**:
- `usage_profile`이 비어있으면 기본값 `developer` 적용
- `usage_profile`이 `developer` 또는 `fullstack`이 아니면 에러

**예상 라인 수**: schema_profile.go ~50줄, schema.go 변경 +10줄

---

### T2: 상태 파일 관리

**목표**: `~/.autopus/state.json`에 프로젝트별 힌트 상태를 저장/로드하는 패키지를 생성한다.

**변경 파일**:
- `pkg/hint/state.go` — 상태 읽기/쓰기 로직
- `pkg/hint/state_test.go` — 단위 테스트

**구현 상세**:
```go
// pkg/hint/state.go

// ProjectState holds hint tracking state for a single project.
type ProjectState struct {
    GoSuccessCount  int  `json:"go_success_count"`
    FirstHintShown  bool `json:"first_hint_shown"`
    SecondHintShown bool `json:"second_hint_shown"`
}

// StateStore manages ~/.autopus/state.json.
type StateStore struct {
    path string
}

// NewStateStore creates a store using ~/.autopus/state.json.
func NewStateStore() (*StateStore, error)

// Load reads the project state for the given project path.
func (s *StateStore) Load(projectPath string) (*ProjectState, error)

// Save writes the project state for the given project path.
func (s *StateStore) Save(projectPath string, state *ProjectState) error

// projectKey returns SHA-256 hash of the absolute project path.
func projectKey(projectPath string) string
```

**에러 처리**: 파일 없음/읽기 실패 → 기본 상태 반환 (zero values). 쓰기 실패 → 에러 로그 후 무시.

**예상 라인 수**: state.go ~120줄, state_test.go ~150줄

---

### T3: Init Wizard 프로파일 스텝 추가

**목표**: Init Wizard의 첫 번째 스텝으로 Developer/Fullstack 프로파일 선택을 추가한다.

**변경 파일**:
- `internal/cli/tui/wizard_steps.go` — InitWizardOpts/Result에 프로파일 필드 추가, buildStepList 수정
- `internal/cli/tui/wizard_profile.go` — buildProfileStep 함수 (신규)

**구현 상세**:

wizard_steps.go 변경:
```go
// InitWizardOpts에 추가
ExistingProfile string // pre-set from existing autopus.yaml

// InitWizardResult에 추가
UsageProfile string // "developer" or "fullstack"
```

buildStepList 변경:
- 프로파일 스텝을 steps 배열의 맨 앞에 삽입
- `--yes` 모드(Accessible)에서는 기본값 `developer` 적용

wizard_profile.go (신규):
```go
// buildProfileStep creates the usage profile selection step.
func buildProfileStep(num, total int, r *InitWizardResult) *huh.Form {
    title := fmt.Sprintf("[%d/%d] Usage Profile", num, total)
    return huh.NewForm(
        huh.NewGroup(
            huh.NewSelect[string]().
                Title(title).
                Description("How will you use ADK?").
                Options(
                    huh.NewOption("Developer — plan/go/sync 개발 도구", "developer"),
                    huh.NewOption("Fullstack — + Worker/Platform 연동", "fullstack"),
                ).
                Value(&r.UsageProfile),
        ),
    ).WithTheme(AutopusTheme()).WithWidth(bannerWidth + 10)
}
```

pre-select 처리 (R9):
- `ExistingProfile`이 있으면 `r.UsageProfile`을 해당 값으로 초기화

**예상 라인 수**: wizard_profile.go ~40줄, wizard_steps.go 변경 +20줄

---

### T4: 힌트 표시 로직

**목표**: `auto go` 파이프라인 완료 후 조건을 평가하여 힌트를 표시한다.

**변경 파일**:
- `pkg/hint/hint.go` — 힌트 조건 평가 및 표시 로직
- `pkg/hint/hint_test.go` — 단위 테스트

**구현 상세**:
```go
// pkg/hint/hint.go

const (
    Hint1 = "💡 이 작업을 AI 에이전트 팀이 자동화할 수 있습니다 → autopus.co"
    Hint2 = "💡 AI Worker가 이 SPEC을 자율 구현할 수 있습니다 → autopus.co/worker"
)

// CheckAndShow evaluates hint conditions and displays if appropriate.
// Returns true if a hint was shown.
func CheckAndShow(projectPath string, profile UsageProfile, hintsEnabled bool, w io.Writer) bool

// RecordSuccess records a successful auto go completion.
func RecordSuccess(projectPath string) error
```

**힌트 조건 로직**:
1. `profile != developer` → return false
2. `hintsEnabled == false` → return false
3. state 로드
4. `go_success_count == 1 && !first_hint_shown` → Hint1 표시, first_hint_shown = true
5. `go_success_count >= 3 && !second_hint_shown` → Hint2 표시, second_hint_shown = true
6. 그 외 → return false

**호출 위치**: `auto go` 파이프라인 완료 핸들러 (성공 시) — 기존 코드에 1줄 호출 추가

**예상 라인 수**: hint.go ~80줄, hint_test.go ~150줄

---

### T5: auto config set 커맨드 신규 생성 + hints.platform 지원

**목표**: `auto config set <key> <value>` 커맨드를 새로 구현하고, 첫 번째 지원 키로 `hints.platform`을 추가한다.

**NOTE**: `auto config` 커맨드는 현재 코드베이스에 존재하지 않는 신규 Cobra 커맨드이다.

**변경 파일**:
- `internal/cli/config_cmd.go` — 신규 파일: `auto config` 커맨드 트리 (config, config set, config get)
- `internal/cli/config_cmd_test.go` — 신규 파일: config set 단위 테스트
- `internal/cli/root.go` (또는 commands.go) — config 서브커맨드 등록

**구현 상세**:
- `auto config set <key> <value>`: autopus.yaml을 로드 → 키에 해당하는 필드 수정 → 파일에 재직렬화
- 지원 키 (최초): `hints.platform` (bool), `usage_profile` (string)
- dot-notation 키를 구조체 필드에 매핑하는 간단한 switch-case (범용 리플렉션 아님)
- 값이 `false`이면 `HintsConf.Platform`을 `*bool(false)`로 설정
- 변경 후 autopus.yaml 파일 저장 (`yaml.Marshal` + `os.WriteFile`)
- `auto config get <key>`: 현재 값 표시 (P1, 선택적)

**예상 라인 수**: config_cmd.go ~120줄, config_cmd_test.go ~80줄

## Total Estimated Changes

| Category | Lines |
|----------|-------|
| New files | ~440줄 (source) + ~300줄 (test) |
| Modified files | ~50줄 |
| Total | ~790줄 |

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| wizard_steps.go가 300줄 초과할 수 있음 | wizard_profile.go로 프로파일 스텝 분리 |
| state.json 동시 접근 | 프로젝트별 키로 격리, 최악의 경우 힌트 1회 추가 표시 |
| 기존 autopus.yaml 호환성 | 모든 새 필드에 omitempty + 기본값 처리 |
