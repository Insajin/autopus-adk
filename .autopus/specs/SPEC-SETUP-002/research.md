# SPEC-SETUP-002 리서치

## 기존 코드 분석

### 핵심 파일 및 함수

| 파일 | 함수/타입 | 역할 | 라인수 |
|------|----------|------|--------|
| `pkg/setup/scanner.go` | `Scan()` | 프로젝트 스캔 진입점 — 멀티레포 감지 호출 추가 지점 | 573 |
| `pkg/setup/types.go` | `ProjectInfo` | 스캔 결과 보유 — MultiRepo 필드 추가 지점 | 138 |
| `pkg/setup/workspace.go` | `DetectWorkspaces()` | 기존 모노레포 감지 (go.work, npm 등) — 패턴 참고 | 230 |
| `pkg/setup/renderer.go` | `renderArchitecture()`, `renderStructure()` | 문서 렌더링 — 멀티레포 섹션 삽입 지점 | 467 |
| `pkg/setup/scenarios.go` | `generateScenarios()` | 시나리오 생성 — 크로스 컴포넌트 시나리오 추가 지점 | 39 |
| `pkg/setup/engine.go` | `Generate()`, `Update()` | 생성/업데이트 오케스트레이션 — 변경 불필요 | 301 |
| `internal/cli/setup.go` | `newSetupGenerateCmd()` | CLI 진입점 — 변경 불필요 | 214 |
| `internal/cli/arch.go` | `newArchGenerateCmd()` | 아키텍처 CLI — 추후 연동 가능 | 97 |

### 기존 패턴

**워크스페이스 감지 패턴** (`workspace.go`):
- `DetectWorkspaces(dir string) []Workspace` — 매니페스트 파일 기반 감지
- 결과를 `[]Workspace` 슬라이스로 반환, `ProjectInfo.Workspaces`에 할당
- 멀티레포 감지도 동일한 패턴으로 `DetectMultiRepo(dir string) *MultiRepoInfo` 구현 가능

**스캐너 패턴** (`scanner.go`):
- `Scan()`이 여러 detect 함수를 순차 호출 (`detectLanguages`, `detectFrameworks`, `detectBuildFiles` 등)
- 멀티레포 감지는 `Scan()` 마지막에 추가, 결과에 따라 컴포넌트별 재스캔

**렌더러 패턴** (`renderer.go`):
- 각 `renderXxx()` 함수가 `*ProjectInfo`를 받아 `string` 반환
- 조건부 섹션 추가: `if len(info.Workspaces) > 0 { ... }` 패턴 (renderIndex 81행 참고)
- 동일하게 `if info.MultiRepo != nil { ... }` 가드 사용

### 크로스 레포 의존성 실제 데이터

**autopus-bridge/go.mod:**
```
replace github.com/insajin/autopus-agent-protocol => ./third_party/autopus-agent-protocol
replace github.com/insajin/autopus-codex-rpc => ./third_party/autopus-codex-rpc
require github.com/insajin/autopus-agent-protocol v0.9.0
require github.com/insajin/autopus-codex-rpc v0.1.0
```

이로부터 다음 의존성 그래프를 도출:
```
autopus-bridge --> autopus-agent-protocol (replace + require)
autopus-bridge --> autopus-codex-rpc (replace + require)
```

**autopus-adk/go.mod:**
- 외부 의존성만 존재, 다른 autopus 레포 참조 없음 (독립 컴포넌트)

**autopus-agent-protocol/go.mod, autopus-codex-rpc/go.mod:**
- 의존성 없는 리프 모듈

### git remote 정보 추출 방법

각 컴포넌트의 git remote URL은 다음으로 추출:
```go
// .git/config 파싱 또는 exec("git", "-C", repoDir, "remote", "get-url", "origin")
```

외부 명령 의존성을 피하려면 `.git/config` 파일을 직접 파싱하는 것이 바람직. 기존 코드베이스에서 `os/exec`는 사용하지 않는 패턴이므로 파일 파싱 방식 채택.

## 설계 결정

### D1: MultiRepo를 포인터 필드로 추가 (Workspace 슬라이스와 병존)

**결정**: `ProjectInfo.MultiRepo *MultiRepoInfo` — nil이면 단일 레포/모노레포
**이유**: 기존 `Workspaces []Workspace`는 go.work/npm 워크스페이스용으로 유지하고, 멀티레포는 근본적으로 다른 개념이므로 별도 필드로 분리. 포인터를 사용하여 nil 체크로 멀티레포 여부를 빠르게 판단.

### D2: 새 파일 3개로 분리 (multirepo.go, multirepo_types.go, multirepo_render.go)

**결정**: 감지/타입/렌더링을 각각 독립 파일로 분리
**이유**: scanner.go(573줄)와 renderer.go(467줄)가 이미 300줄 제한을 초과하고 있어, 추가 로직을 인라인으로 넣을 수 없음. 멀티레포 관련 코드를 독립 파일로 분리하면 기존 파일 수정을 최소화하고 파일 크기 제한도 준수.

### D3: go.mod 파싱으로 의존성 매핑 (AST 불필요)

**결정**: 텍스트 기반 go.mod 파싱
**이유**: 기존 `scanner.go`의 `detectLanguages()`가 이미 go.mod를 텍스트로 파싱하는 패턴을 사용중. `replace`와 `require` 디렉티브는 단순 텍스트 매칭으로 충분히 추출 가능. golang.org/x/mod 의존성을 추가하는 것은 과도.

### D4: git config 파일 직접 파싱 (exec 미사용)

**결정**: `.git/config` 파일을 텍스트로 파싱하여 remote URL 추출
**이유**: 기존 코드베이스가 외부 명령 실행(os/exec)을 사용하지 않는 패턴. 파일 파싱이 이식성과 테스트 용이성 모두 우수.
**대안**: `os/exec`로 `git remote get-url origin` 실행 — git 바이너리 의존성 추가되므로 기각.

### D5: 멀티레포 감지 조건 — 루트에 .git 없고 서브디렉토리에 2개 이상 .git 존재

**결정**: `.git` 없는 루트 + 서브디렉토리 2개 이상에 `.git` 존재 시 멀티레포로 분류
**이유**: 서브디렉토리 1개만 .git이면 단순히 git 서브모듈이거나 우연한 구조일 수 있음. 2개 이상이어야 의미 있는 멀티레포 워크스페이스.
**대안**: 1개부터 감지 — 오탐 위험이 있으므로 기각.

## Implementation Outcome (2026-04-21)

### 최종 파일 구성

| 파일 | 역할 | 라인수 |
|------|------|--------|
| `pkg/setup/multirepo.go` | root + immediate child repo 감지, component metadata 추출 | 225 |
| `pkg/setup/multirepo_deps.go` | Go `require`/`replace`, npm `package`/`file:` edge 파싱 | 208 |
| `pkg/setup/multirepo_types.go` | `MultiRepoInfo`, `RepoComponent`, `RepoDependency` | 29 |
| `pkg/setup/multirepo_render.go` | Workspace / Development Workflow / Repository Boundaries 렌더링 | 95 |
| `pkg/setup/scanner.go` | single-repo vs multi-repo 오케스트레이션, aggregate merge | 180 |
| `pkg/setup/scenarios.go` | cross-repo synthetic scenario + language-specific verification command | 102 |

### 구현 중 확정된 사항

- `DetectWorkspaces()` 는 그대로 두고, `DetectMultiRepo()` 를 별도 축으로 추가했다.
- multi-repo discovery 범위는 SPEC `R1`에 맞춰 **immediate child repositories only** 로 고정했다.
- `MapCrossRepoDeps()` 는 300줄 제한 준수를 위해 `multirepo_deps.go` 로 분리했다.
- cross-repo scenario는 repo 이름이 아니라 `RepoComponent.Path` 를 사용하고, `Go` / `JavaScript|TypeScript` / `Rust` / `Python` 별 검증 명령을 분기한다.

### 최종 검증

- `go test ./pkg/setup/...`
- `go test -cover ./pkg/setup/...` → 86.9%
- `go vet ./pkg/setup/...`
