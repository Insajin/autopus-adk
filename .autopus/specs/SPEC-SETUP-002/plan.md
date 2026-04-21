# SPEC-SETUP-002 구현 계획

## 태스크 목록

- [x] T1: `multirepo_types.go` — MultiRepoInfo, RepoComponent, RepoDependency 타입 정의
- [x] T2: `multirepo.go`, `multirepo_deps.go` — DetectMultiRepo, ScanRepoComponent, MapCrossRepoDeps 구현
- [x] T3: `types.go` 수정 — ProjectInfo에 MultiRepo 필드 추가
- [x] T4: `scanner.go` 수정 — Scan()에서 멀티레포 감지 및 컴포넌트별 집계 로직 추가
- [x] T5: `multirepo_render.go` — 워크스페이스/워크플로우/바운더리 렌더링 헬퍼 구현
- [x] T6: `renderer_arch.go`, `renderer_docs.go` 수정 — architecture/index/structure에서 멀티레포 섹션 삽입
- [x] T7: `scenarios.go` 수정 — 번호 offset, repo path 기반 subshell, language-specific 크로스 컴포넌트 시나리오 생성
- [x] T8: 단위 테스트 — `multirepo_test.go`, `multirepo_render_test.go`, `multirepo_scenarios_test.go`
- [x] T9: fixture 기반 통합 검증 — temp workspace topology로 end-to-end acceptance 검증

## 구현 전략

### 접근 방법

기존 `workspace.go`의 DetectWorkspaces 패턴을 참고하되, 근본적으로 다른 감지 메커니즘을 구현한다. 기존 워크스페이스 감지는 go.work, package.json 등 매니페스트 기반이지만, 멀티레포 감지는 `.git` 디렉토리 존재 여부 기반이다.

### 기존 코드 활용

- `workspace.go`의 `DetectWorkspaces()` — 패턴 참고 (호출 구조, 타입 네이밍)
- `scanner.go`의 `detectLanguages()`, `detectBuildFiles()` — 컴포넌트별 재활용
- `renderer_arch.go`, `renderer_docs.go` — 삽입 지점
- `types.go`의 `Workspace` — 기존 타입과 공존, 충돌 없이 확장

### 변경 범위

- 신규 production 파일 4개: `multirepo.go`, `multirepo_deps.go`, `multirepo_types.go`, `multirepo_render.go`
- 수정 production 파일 5개: `types.go`, `scanner.go`, `renderer_arch.go`, `renderer_docs.go`, `scenarios.go`
- 신규 테스트 파일 3개: `multirepo_test.go`, `multirepo_render_test.go`, `multirepo_scenarios_test.go`

### 위험 요소

- `scanner.go`는 오케스트레이션 역할만 유지하고, 멀티레포 감지/의존성 파싱은 `multirepo*.go` 계열로 분리
- dependency parsing은 `multirepo.go` 단일 파일이 300줄을 넘지 않도록 `multirepo_deps.go`로 추가 분리
- 단일 레포 사용자에게 영향 없도록 모든 멀티레포 코드 경로는 `MultiRepo != nil` 가드로 보호

## Sync Result (2026-04-21)

- `pkg/setup` 멀티레포 계층이 구현되었고, 문서/시나리오 렌더링까지 연결됐다.
- 실제 topology E2E는 로컬 workspace 편차가 커서 live `autopus-co` 의존 대신 temp fixture 기반 acceptance 검증으로 고정했다.
- 검증:
  - `go test ./pkg/setup/...`
  - `go test -cover ./pkg/setup/...` → 86.9%
  - `go vet ./pkg/setup/...`
