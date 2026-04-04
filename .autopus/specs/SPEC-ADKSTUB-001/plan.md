# SPEC-ADKSTUB-001 구현 계획

## 태스크 목록

### Area A: agent_run 실행 연결
- [ ] T1: `internal/cli/agent_run_exec.go` 생성 — adapter registry 조회, subprocess 실행, stream 파싱, 결과 추출 함수
- [ ] T2: `internal/cli/agent_run.go` 수정 — placeholder 제거, T1의 실행 함수 호출로 대체
- [ ] T3: `internal/cli/agent_run.go`에 에러 핸들링 — subprocess 실패 시 `status: "failed"` 기록

### Area B: TUI 키 핸들러 연결
- [ ] T4: `pkg/worker/tui/model.go`에 `OnPauseToggle func()`, `OnCancelTask func(taskID string)` 콜백 필드 추가
- [ ] T5: `p` 키 핸들러에 `paused` 상태 토글 + `OnPauseToggle` 콜백 호출 연결
- [ ] T6: `c` 키 핸들러에 `OnCancelTask` 콜백 호출 연결 (기존 `m.currentTask = nil` 전에 신호 전송)
- [ ] T7: `paused` 상태일 때 header에 "PAUSED" 표시 렌더링

### Area C: Config Schema 정리
- [ ] T8: `TelemetryConf` struct 및 `HarnessConfig.Telemetry` 필드 제거, defaults.go 업데이트
- [ ] T9: `ConstraintConf` 로딩 — config에서 path 읽어 `constraint.Check()`에 전달하는 헬퍼 함수 작성
- [ ] T10: `IssueReport.AutoSubmit` 연결 — pipeline error handler에서 config 확인 후 자동 report 트리거
- [ ] T11: `Hints.Platform` 연결 — `CheckAndShow()` 시그니처에서 `hintsEnabled bool` 제거, config 직접 참조

### Area D: 테스트
- [ ] T12: `agent_run_exec.go` 단위 테스트 — mock adapter로 실행/실패 시나리오 검증
- [ ] T13: TUI 키 핸들러 테스트 — pause/cancel 콜백 호출 검증
- [ ] T14: config 정리 테스트 — Telemetry 필드 제거 후 YAML 파싱, Constraints 로딩 테스트

## 구현 전략

### Area A 접근 방법
기존 `pkg/worker/adapter/` 패키지의 `ProviderAdapter` 인터페이스와 `pkg/worker/stream/Parser`가 이미 완성되어 있다. `agent_run.go`의 placeholder(L67-68)를 adapter 기반 실행으로 교체한다.

핵심 흐름:
1. `taskContext.Description`에서 provider 이름 결정 (또는 default provider 사용)
2. `adapter.Registry`에서 adapter 조회
3. `adapter.BuildCommand()` → `exec.Cmd` 생성
4. `stream.NewParser(cmd.Stdout)` → 이벤트 루프
5. `result` 이벤트 수신 시 `adapter.ExtractResult()` → `taskResult` 기록

실행 로직은 300줄 제한을 고려해 `agent_run_exec.go`로 분리한다.

### Area B 접근 방법
bubbletea 모델에 콜백 패턴 적용. `OnApprovalDecision`, `OnViewDiff`가 이미 같은 패턴으로 구현되어 있으므로 동일하게 `OnPauseToggle`, `OnCancelTask` 콜백을 추가한다. `paused` bool 필드를 `WorkerModel`에 추가하고 View에서 조건 렌더링한다.

### Area C 접근 방법
- **Telemetry**: 기존 `pkg/telemetry/` 패키지는 자체 파일 기반 reader를 사용하며 `HarnessConfig.Telemetry`를 참조하지 않는다. config struct에서 제거해도 telemetry CLI 동작에 영향 없다.
- **Constraints**: `init_constraints.go`에서 `constraint.GenerateDefaultFile()`만 호출하고 있다. config의 `Constraints.Path`를 읽어 커스텀 경로 지원을 추가한다.
- **IssueReport.AutoSubmit**: `internal/cli/issue.go`의 `runIssueReport()`가 이미 autoSubmit 파라미터를 받는다. pipeline 실패 hook에서 config를 읽어 자동 호출하는 wrapper를 작성한다.
- **Hints.Platform**: `hint.CheckAndShow()`의 `hintsEnabled bool` 파라미터를 `HintsConf`로 교체하여 config와 직접 연결한다.

### 변경 범위
- 수정 파일: 5-6개
- 신규 파일: 2-3개 (exec 분리, auto issue trigger, 테스트)
- 삭제 코드: `TelemetryConf` struct (~6줄), placeholder 코드 (~5줄)
