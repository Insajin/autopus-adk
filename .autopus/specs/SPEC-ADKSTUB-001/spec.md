# SPEC-ADKSTUB-001: CLI/TUI Stub 해소 및 Config Schema 정리

**Status**: draft
**Created**: 2026-04-04
**Domain**: ADKSTUB

## 목적

autopus-adk 코드베이스에 남아 있는 세 가지 미완성 영역을 해소한다:
1. `auto agent run <task-id>` 커맨드가 context를 파싱만 하고 실제 실행 없이 더미 결과를 기록하는 문제
2. Worker TUI의 `p`(pause), `c`(cancel) 키가 UI 상으로만 존재하고 실제 동작이 없는 문제
3. `HarnessConfig`에 정의되었으나 런타임에서 참조되지 않는 설정 필드 4개의 사용처 불일치

이 세 영역은 독립적이지만, 모두 "선언만 있고 동작이 없는 코드"라는 공통 패턴을 가진다.

## 요구사항

### R1: agent_run 실행 연결

WHEN `auto agent run <task-id>` is executed with a valid context.yaml,
THE SYSTEM SHALL resolve the appropriate provider adapter from the registry,
build a subprocess command via `ProviderAdapter.BuildCommand()`,
stream the subprocess output through `stream.Parser`,
and write the real `TaskResult` (including cost, duration, session ID) to `result.yaml`.

### R2: agent_run 에러 전파

WHEN the provider subprocess exits with a non-zero code or the stream contains an error event,
THE SYSTEM SHALL write a `result.yaml` with `status: "failed"` and include the error detail,
rather than always writing `status: "success"`.

### R3: TUI pause 키 연결

WHEN the user presses `p` in the Worker TUI dashboard,
THE SYSTEM SHALL toggle a `paused` state that suspends the task poller from accepting new tasks,
and THE SYSTEM SHALL display "PAUSED" in the header status area.

### R4: TUI cancel 키 연결

WHEN the user presses `c` in the Worker TUI dashboard while a task is running,
THE SYSTEM SHALL send a cancellation signal (context cancel) to the running subprocess,
and THE SYSTEM SHALL update the current task status to "cancelled" before clearing it.

### R5: Telemetry 설정 필드 제거

WHERE the `TelemetryConf` struct fields (`Enabled`, `RetentionDays`, `CostTracking`) are defined in `HarnessConfig.Telemetry` but never read at runtime (telemetry CLI uses its own file-based reader),
THE SYSTEM SHALL remove the `Telemetry` field from `HarnessConfig` and the `TelemetryConf` struct.

### R6: Constraints 설정 연결

WHEN `HarnessConfig.Constraints.Enabled` is true and `Constraints.Path` is set,
THE SYSTEM SHALL load constraints from the configured path at pipeline startup,
rather than relying solely on the hardcoded default file generation in `init_constraints.go`.

### R7: IssueReport.AutoSubmit 연결

WHEN `HarnessConfig.IssueReport.AutoSubmit` is true and a pipeline run fails,
THE SYSTEM SHALL automatically invoke the issue report flow (collect context, format, submit)
without requiring the user to manually run `auto issue report`.

### R8: Hints.Platform 런타임 연결

WHEN `CheckAndShow()` is called in `pkg/hint/hint.go`,
THE SYSTEM SHALL consult `HarnessConfig.Hints.IsPlatformHintEnabled()` to decide whether to display hints,
rather than receiving `hintsEnabled` as a separate boolean parameter.

## 생성 파일 상세

| 파일/모듈 | 역할 |
|-----------|------|
| `internal/cli/agent_run.go` | provider adapter 호출, stream parsing, 실제 결과 기록 |
| `internal/cli/agent_run_exec.go` (신규) | 실행 로직 분리 — adapter 선택, subprocess 구동, 결과 추출 |
| `pkg/worker/tui/model.go` | `p`, `c` 키 핸들러에 실제 callback 연결 |
| `pkg/worker/tui/pause.go` (신규) | pause 상태 관리 및 UI 렌더링 |
| `pkg/config/schema.go` | `TelemetryConf` 제거, `Constraints` 연결 강화 |
| `pkg/hint/hint.go` | `HintsConf` 기반 판단 로직으로 전환 |
| `internal/cli/issue_auto.go` (신규) | pipeline 실패 시 자동 issue report 트리거 |
