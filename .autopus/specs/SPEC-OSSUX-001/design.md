# SPEC-OSSUX-001 Technical Design

**SPEC**: SPEC-OSSUX-001
**Status**: draft
**Created**: 2026-04-02

## 1. Data Flow Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        auto init                                │
│                                                                 │
│  ┌──────────────┐    ┌──────────────────┐    ┌───────────────┐ │
│  │ Profile Step │───▶│ Language/Quality  │───▶│ Write YAML    │ │
│  │ (wizard_     │    │ Steps (existing)  │    │ usage_profile │ │
│  │  profile.go) │    │                  │    │ + hints conf  │ │
│  └──────────────┘    └──────────────────┘    └───────┬───────┘ │
└──────────────────────────────────────────────────────┼─────────┘
                                                       │
                                                       ▼
                                                 autopus.yaml
                                                       │
┌──────────────────────────────────────────────────────┼─────────┐
│                        auto go                       │         │
│                                                      ▼         │
│  ┌──────────┐    ┌──────────┐    ┌──────────────────────────┐ │
│  │ Pipeline │───▶│ Phase 4  │───▶│ hint.CheckAndShow()      │ │
│  │ Executor │    │ Complete │    │  ├─ Read config           │ │
│  │          │    │          │    │  ├─ Read state.json       │ │
│  │          │    │          │    │  ├─ Evaluate conditions   │ │
│  │          │    │          │    │  ├─ Display hint (maybe)  │ │
│  │          │    │          │    │  └─ Update state.json     │ │
│  └──────────┘    └──────────┘    └──────────────────────────┘ │
└────────────────────────────────────────────────────────────────┘
                                            │
                                            ▼
                                   ~/.autopus/state.json
```

## 2. autopus.yaml Schema Changes

### 신규 필드

```yaml
# autopus.yaml — 신규 필드 예시
usage_profile: developer    # "developer" | "fullstack" (default: developer)

hints:
  platform: true            # true | false (default: true, omit = true)
```

### HarnessConfig 구조체 변경

```go
// pkg/config/schema.go — HarnessConfig에 추가되는 필드
type HarnessConfig struct {
    // ... existing fields ...

    UsageProfile UsageProfile `yaml:"usage_profile,omitempty"` // developer (default) or fullstack
    Hints        HintsConf    `yaml:"hints,omitempty"`         // platform hint settings
}
```

### 신규 타입 정의 (pkg/config/schema_profile.go)

```go
// UsageProfile represents how the user intends to use ADK.
type UsageProfile string

const (
    ProfileDeveloper UsageProfile = "developer"
    ProfileFullstack UsageProfile = "fullstack"
)

// DefaultUsageProfile returns the default profile for backward compatibility.
func DefaultUsageProfile() UsageProfile {
    return ProfileDeveloper
}

// IsValid checks if the profile value is recognized.
func (p UsageProfile) IsValid() bool {
    return p == ProfileDeveloper || p == ProfileFullstack || p == ""
}

// Effective returns the effective profile, defaulting empty to developer.
func (p UsageProfile) Effective() UsageProfile {
    if p == "" {
        return ProfileDeveloper
    }
    return p
}

// HintsConf holds configuration for non-intrusive platform hints.
type HintsConf struct {
    // Platform controls whether platform upgrade hints are shown.
    // nil (omitted) = enabled (default), explicit false = disabled permanently.
    Platform *bool `yaml:"platform,omitempty"`
}

// IsPlatformHintEnabled returns whether platform hints are active.
func (h HintsConf) IsPlatformHintEnabled() bool {
    if h.Platform == nil {
        return true
    }
    return *h.Platform
}
```

### Validate() 확장

```go
// pkg/config/schema.go — Validate()에 추가
if !c.UsageProfile.IsValid() {
    return fmt.Errorf("invalid usage_profile %q: must be 'developer' or 'fullstack'", c.UsageProfile)
}
```

## 3. State File Format (~/.autopus/state.json)

### 경로

```
~/.autopus/state.json
```

`os.UserHomeDir()`로 홈 디렉토리를 결정한다. `~/.autopus/` 디렉토리가 없으면 `os.MkdirAll`로 생성한다.

### 포맷

```json
{
  "version": 1,
  "projects": {
    "a1b2c3d4e5f6...": {
      "go_success_count": 3,
      "first_hint_shown": true,
      "second_hint_shown": false
    },
    "f6e5d4c3b2a1...": {
      "go_success_count": 0,
      "first_hint_shown": false,
      "second_hint_shown": false
    }
  }
}
```

### 프로젝트 키 생성

```go
// projectKey computes a stable key from the absolute project path.
func projectKey(projectPath string) string {
    abs, err := filepath.Abs(projectPath)
    if err != nil {
        abs = projectPath
    }
    h := sha256.Sum256([]byte(abs))
    return hex.EncodeToString(h[:16]) // 32-char hex, sufficient uniqueness
}
```

SHA-256의 앞 16바이트(32자 hex)를 사용한다. 전체 해시보다 짧으면서도 충돌 가능성이 무시할 수 있는 수준이다.

### StateStore 인터페이스

```go
// pkg/hint/state.go

type stateFile struct {
    Version  int                     `json:"version"`
    Projects map[string]ProjectState `json:"projects"`
}

type ProjectState struct {
    GoSuccessCount  int  `json:"go_success_count"`
    FirstHintShown  bool `json:"first_hint_shown"`
    SecondHintShown bool `json:"second_hint_shown"`
}

type StateStore struct {
    path string
}

func NewStateStore() (*StateStore, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return nil, err
    }
    dir := filepath.Join(home, ".autopus")
    if err := os.MkdirAll(dir, 0o755); err != nil {
        return nil, err
    }
    return &StateStore{path: filepath.Join(dir, "state.json")}, nil
}

func (s *StateStore) Load(projectPath string) (*ProjectState, error)
func (s *StateStore) Save(projectPath string, state *ProjectState) error
```

## 4. Init Wizard Integration

### 프로파일 스텝 위치

프로파일 스텝은 `buildStepList()`에서 **첫 번째 스텝**으로 삽입된다.

```
기존 순서: Language → Quality → ReviewGate → Methodology
변경 순서: Profile → Language → Quality → ReviewGate → Methodology
```

### InitWizardOpts 변경

```go
type InitWizardOpts struct {
    Quality         string
    NoReviewGate    bool
    Platforms       []string
    Accessible      bool
    Providers       []string
    ExistingProfile string  // NEW: pre-set from existing autopus.yaml
}
```

### InitWizardResult 변경

```go
type InitWizardResult struct {
    CommentsLang string
    CommitsLang  string
    AILang       string
    Quality      string
    ReviewGate   bool
    Methodology  string
    Cancelled    bool
    UsageProfile string  // NEW: "developer" or "fullstack"
}
```

### defaultResult 변경

```go
func defaultResult(opts InitWizardOpts) *InitWizardResult {
    r := &InitWizardResult{
        // ... existing defaults ...
        UsageProfile: "developer", // R10: --yes 모드 기본값
    }
    if opts.ExistingProfile != "" {
        r.UsageProfile = opts.ExistingProfile // R9: pre-select
    }
    return r
}
```

### wizard_profile.go (신규 파일)

```go
package tui

import (
    "fmt"
    "github.com/charmbracelet/huh"
)

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

## 5. Hint Display Trigger Points

### auto go 파이프라인 완료 시점

힌트 표시 로직은 `auto go` 파이프라인의 **성공 완료 직후**에 호출된다.

```
auto go pipeline
  │
  ├── Phase 1: Plan loading / validation
  ├── Phase 2: Task execution (parallel/sequential)
  ├── Phase 3: Merge & verification
  ├── Phase 4: APPROVE / completion
  │
  └── Post-completion hook ← 여기서 hint.CheckAndShow() 호출
```

### 호출 코드 (개념)

```go
// auto go 완료 핸들러에 추가 (1줄)
if pipelineSuccess {
    hint.RecordSuccess(projectPath)
    hint.CheckAndShow(projectPath, cfg.UsageProfile.Effective(), cfg.Hints.IsPlatformHintEnabled(), os.Stdout)
}
```

### hint.CheckAndShow 로직

```go
func CheckAndShow(projectPath string, profile config.UsageProfile, hintsEnabled bool, w io.Writer) bool {
    // Guard: only for developer profile with hints enabled
    if profile != config.ProfileDeveloper || !hintsEnabled {
        return false
    }

    store, err := NewStateStore()
    if err != nil {
        return false // graceful degradation
    }

    state, err := store.Load(projectPath)
    if err != nil {
        return false
    }

    var shown bool

    if state.GoSuccessCount >= 1 && !state.FirstHintShown {
        fmt.Fprintln(w, Hint1)
        state.FirstHintShown = true
        shown = true
    } else if state.GoSuccessCount >= 3 && !state.SecondHintShown {
        fmt.Fprintln(w, Hint2)
        state.SecondHintShown = true
        shown = true
    }

    if shown {
        _ = store.Save(projectPath, state) // best-effort
    }

    return shown
}
```

### hint.RecordSuccess 로직

```go
func RecordSuccess(projectPath string) error {
    store, err := NewStateStore()
    if err != nil {
        return err
    }

    state, err := store.Load(projectPath)
    if err != nil {
        state = &ProjectState{}
    }

    state.GoSuccessCount++
    return store.Save(projectPath, state)
}
```

## 6. auto config set hints.platform 지원

### 명령어 형태

```bash
auto config set hints.platform false   # 힌트 영구 비활성화
auto config set hints.platform true    # 힌트 재활성화
```

### 구현 방식

기존 `auto config set` 커맨드의 키 매핑에 `hints.platform` 경로를 추가한다.

```go
// config set 핸들러 내부
case "hints.platform":
    val := args[1] == "true"
    cfg.Hints.Platform = &val
```

변경 후 autopus.yaml을 다시 직렬화하여 저장한다.

## 7. 파일 구조 요약

```
autopus-adk/
├── pkg/
│   ├── config/
│   │   ├── schema.go              # 기존 (HarnessConfig에 2 필드 추가)
│   │   └── schema_profile.go      # 신규 (UsageProfile, HintsConf 타입)
│   └── hint/
│       ├── state.go               # 신규 (StateStore, ProjectState)
│       ├── state_test.go          # 신규 (StateStore 단위 테스트)
│       ├── hint.go                # 신규 (CheckAndShow, RecordSuccess)
│       └── hint_test.go           # 신규 (힌트 로직 단위 테스트)
├── internal/
│   └── cli/
│       ├── tui/
│       │   ├── wizard_steps.go    # 기존 (Opts/Result 확장, buildStepList 수정)
│       │   └── wizard_profile.go  # 신규 (buildProfileStep)
│       └── config_cmd.go          # 기존 또는 신규 (hints.platform set 지원)
```

## 8. Backward Compatibility

| Scenario | Behavior |
|----------|----------|
| 기존 autopus.yaml에 `usage_profile` 없음 | `developer`로 기본 동작 |
| 기존 autopus.yaml에 `hints` 없음 | 힌트 활성화(기본값) |
| `~/.autopus/state.json` 없음 | 자동 생성, 초기 상태 사용 |
| `~/.autopus/state.json` 읽기 실패 | 힌트 건너뜀, 에러 없음 |
| `~/.autopus/` 디렉토리 생성 실패 | 힌트 건너뜀, 에러 없음 |

## 9. 보안 고려사항

- `state.json`은 사용자 홈 디렉토리에 저장되며 프로젝트 코드에 포함되지 않음
- 프로젝트 경로는 SHA-256 해시로 저장되어 경로 노출 없음
- `state.json`에 민감한 정보(토큰, 인증 정보)를 저장하지 않음
- 파일 권한은 `0o644`(기본), 디렉토리는 `0o755`
