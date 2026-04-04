# SPEC-ADKWIRE-001: ADK Stub Wiring — Learn CLI, Worker 핵심 패키지, Pipeline Dashboard

**Status**: draft
**Created**: 2026-04-04
**Domain**: ADKWIRE

## 목적

autopus-adk에 구현체가 존재하지만 호출 경로가 없는 3개 영역(Learn CLI, Worker auth/poll/net, Pipeline Dashboard)을 실제 코드 경로에 연결한다. Skills 문서에서 `auto learn query` 등을 참조하지만 Cobra 커맨드가 없어 "command not found"가 발생하고, Worker의 TokenRefresher/TaskPoller/NetMonitor는 Start()가 호출되지 않으며, Pipeline Dashboard는 모든 Phase를 PhasePending으로 하드코딩한다.

## 요구사항

### REQ-WIRE-01: Learn CLI 커맨드 등록
WHEN the user runs `auto learn <subcommand>`, THE SYSTEM SHALL route to the corresponding `pkg/learn/` function:
- `auto learn query --files <paths> --packages <pkgs> --keywords <kws>` → `learn.QueryRelevant()`
- `auto learn record --type <type> --pattern <pattern> [--phase --spec-id --files --packages --resolution --severity]` → `learn.Record*()`
- `auto learn prune --days <N>` → `learn.Prune()`
- `auto learn summary [--top <N>]` → `learn.GenerateSummary()`

### REQ-WIRE-02: TokenRefresher Wiring
WHEN the WorkerLoop starts AND LoopConfig.AuthToken is non-empty, THE SYSTEM SHALL instantiate a `auth.TokenRefresher` and start its background loop, passing an `onTokenRefresh` callback that updates the A2A Server's auth token.

### REQ-WIRE-03: TaskPoller Wiring (A2A Fallback)
WHEN the A2A WebSocket reconnect fails (all retries exhausted), THE SYSTEM SHALL activate `poll.TaskPoller` as a fallback mechanism to continue receiving tasks via REST polling.

### REQ-WIRE-04: NetMonitor Wiring
WHEN the WorkerLoop starts, THE SYSTEM SHALL start `net.NetMonitor` with an `onChange` callback that triggers `a2a.Transport.Reconnect()` and an `onValidate` callback that performs a WebSocket health check.

### REQ-WIRE-05: Pipeline Dashboard 실제 데이터 연동
WHEN the user runs `auto pipeline dashboard <spec-id>`, THE SYSTEM SHALL load `{cwd}/.autopus-checkpoint.yaml` via `pipeline.Load()` and map `CheckpointStatus` → `PhaseStatus` for rendering, falling back to all-pending only when the checkpoint file does not exist.

## 생성/수정 파일 상세

| 파일 | 역할 |
|------|------|
| `internal/cli/learn.go` | (NEW) `auto learn` Cobra 커맨드 그룹 + 4 subcommands |
| `internal/cli/root.go` | (MOD) `root.AddCommand(newLearnCmd())` 추가 |
| `pkg/worker/loop.go` | (MOD) `LoopConfig`에 credentials path 추가, `NewWorkerLoop`에서 auth/net 초기화 |
| `pkg/worker/loop_lifecycle.go` | (NEW) Start/Close에서 auth.TokenRefresher, net.NetMonitor, poll.TaskPoller 라이프사이클 관리 |
| `internal/cli/pipeline_dashboard.go` | (MOD) stub 제거, checkpoint 로드 + status 매핑 |
| `pkg/pipeline/status_map.go` | (NEW) CheckpointStatus → PhaseStatus 매핑 함수 |
