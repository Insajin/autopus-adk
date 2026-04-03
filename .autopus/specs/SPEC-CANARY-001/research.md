# SPEC-CANARY-001 리서치

## 기존 코드 분석

### E2E 인프라 (`pkg/e2e/`)

- **`runner.go`**: `RunnerOptions` (ProjectDir, AutoBuild, BuildCommand, Builds, Timeout) 구조체와 `RunnerResult` (ExitCode, Stdout, Stderr, Pass, FailureDetails) 제공. canary의 E2E 실행 단계에서 직접 활용 가능.
- **`scenario.go`**: scenarios.md 파싱. 시나리오별 Command, Precondition, Verify 필드 추출.
- **`build.go`**: 빌드 커맨드 실행 로직. canary Step 1 (빌드 검증)에서 재사용.
- **`verify.go`**: `exit_code`, `stdout_contains`, `file_exists` 등 검증 프리미티브.

### 브라우저 자동화 (`pkg/browse/`)

- **`agent.go`**: Agent Browser 연동. 페이지 접근, DOM 조회 가능.
- **`cmux.go`**: cmux 기반 브라우저 자동화. headed 모드 기본 (feedback_browser_headed 메모리).
- **`backend.go`**: 브라우저 백엔드 추상화.

### 스킬 라우터 (`templates/claude/commands/auto-router.md.tmpl`)

- Subcommand Routing 테이블이 카테고리별로 정리되어 있음 (Development Workflow, Quality & Exploration, Operational).
- canary는 Operational 카테고리에 sync/dev 다음으로 추가 적합.
- 라우터에 이미 `test` (E2E 시나리오 실행)가 존재하므로, canary와 test의 차이 명확화 필요: test는 개발 중 검증, canary는 배포 후 검증.

### 시나리오 정의 (`scenarios.md`)

- `Build` 필드에 빌드 커맨드 정의됨 (`go build -o bin/auto ./cmd/auto`).
- 시나리오마다 `Status: active` 필드로 활성 여부 판단 가능.

### Lore (`pkg/lore/`)

- `LoreEntry` 구조체에 커밋 기반 의사결정 메타데이터 저장. canary 결과의 히스토리 추적에 간접 활용 가능하나, canary 전용 저장소(`.autopus/canary/`)가 더 적합.

## 설계 결정

### D1: 스킬 레벨 구현 (Go 코드 최소 변경)

canary는 기존 `pkg/e2e`와 `pkg/browse`를 조합하는 오케스트레이션이므로, Go 바이너리에 새 패키지를 추가하기보다 스킬 템플릿에서 기존 도구를 순차 호출하는 방식이 적절하다.

**근거**: canary의 핵심 로직은 "빌드 -> E2E -> 브라우저 -> 판정"이라는 순서 제어이며, 각 단계는 이미 Go 코드로 구현되어 있다. 스킬 레벨에서 이를 파이프라인으로 엮는 것이 변경 범위를 최소화한다.

**대안 검토**: `pkg/canary/` 패키지 신설 — 모니터링 주기 관리(--watch)나 결과 저장 로직이 복잡해지면 향후 Go 패키지로 승격 가능. v0.15에서는 스킬 레벨로 시작.

### D2: 결과 저장 형식 — JSON

`.autopus/canary/latest.json`에 최근 결과, `{commit-hash}.json`에 커밋별 스냅샷 저장. JSON 형식은 프로그래매틱 비교(--compare)에 적합하고, 기존 프로젝트의 `.autopus/` 컨벤션과 일치.

### D3: test vs canary 구분

- `/auto test`: 개발 중 E2E 검증. scenarios.md 기반 시나리오 실행.
- `/auto canary`: 배포 후 건강 검진. test + 브라우저 검진 + 이전 결과 비교 + 판정.

canary는 test의 상위 집합이지만, 용도가 다르므로 별도 서브커맨드로 분리.
