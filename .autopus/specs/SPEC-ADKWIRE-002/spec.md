# SPEC-ADKWIRE-002: Worker 2차 패키지 Wiring — audit, scheduler, parallel, knowledge, workspace

**Status**: completed (rolled up into SPEC-ADKWIRE-003, 2026-04-21 sync)
**Created**: 2026-04-04
**Domain**: ADKWIRE
**Superseded By**: `SPEC-ADKWIRE-003`

**Sync Summary**:
- audit/scheduler/parallel/knowledge wiring 정리는 `SPEC-ADKWIRE-003` 통합 구현과 현재 Worker loop lifecycle에 반영됐다.
- multi-workspace 같은 확장 아이디어는 이 초안을 열린 backlog로 유지하지 않고, 필요 시 새 SPEC로 재정의한다.

## 목적

autopus-adk Worker의 5개 서브패키지(audit, scheduler, parallel, knowledge, workspace)가 완전한 구현체를 갖추고 있지만 Worker 메인 루프에서 한 번도 호출되지 않아 dead code로 남아 있다. SPEC-ADKWIRE-001이 auth/poll/net을 loop_lifecycle.go 패턴으로 연결한 것에 이어, 이 SPEC은 나머지 5개 패키지를 동일한 라이프사이클 패턴에 통합한다.

현재 상태:
- **audit**: RotatingWriter가 구현되어 있으나 Worker 시작 시 인스턴스화되지 않고, 태스크 실행 이벤트가 기록되지 않음
- **scheduler**: Dispatcher가 cron 평가/dedup을 지원하나 Start()가 호출되지 않아 로컬 스케줄이 실행되지 않음
- **parallel**: TaskSemaphore/WorktreeManager가 테스트에서만 사용되고, 메인 루프는 태스크를 순차 처리함
- **knowledge**: KnowledgeSearcher/Syncer/FileWatcher가 존재하고 payload에 KnowledgeCtx 필드가 있으나, 검색 결과가 채워지지 않음
- **workspace**: MultiWorkspace가 다중 워크스페이스 연결을 관리하나, 참조가 0건이고 Worker가 단일 워크스페이스만 지원함

## 요구사항

### REQ-WIRE2-01: Audit Logger Wiring
WHEN the WorkerLoop starts, THE SYSTEM SHALL instantiate an `audit.RotatingWriter` with configurable path/size/age from LoopConfig and start its background cleanup goroutine. WHEN a task execution begins or completes, THE SYSTEM SHALL write a structured JSON Lines event to the audit log including taskID, status, duration, and costUSD.

### REQ-WIRE2-02: Scheduler Dispatcher Wiring
WHEN the WorkerLoop starts AND LoopConfig.WorkspaceID is non-empty, THE SYSTEM SHALL instantiate a `scheduler.Dispatcher` with the backend URL, auth token, workspace ID, and local timezone, providing an `onTrigger` callback that submits triggered schedule payloads to `handleTask`. WHEN the WorkerLoop closes, THE SYSTEM SHALL cancel the dispatcher's context.

### REQ-WIRE2-03: Parallel Task Execution
WHEN the WorkerLoop starts AND LoopConfig.MaxConcurrency > 1, THE SYSTEM SHALL instantiate a `parallel.TaskSemaphore` with the configured concurrency limit. WHEN a task is received via `handleTask`, THE SYSTEM SHALL acquire a semaphore slot before execution and release it after completion (success or failure). WHEN LoopConfig.MaxConcurrency is 0 or 1, THE SYSTEM SHALL retain sequential execution behavior.

### REQ-WIRE2-04: Worktree Isolation for Parallel Tasks
WHEN LoopConfig.MaxConcurrency > 1 AND LoopConfig.WorktreeIsolation is true, THE SYSTEM SHALL create a `parallel.WorktreeManager` and for each concurrent task, create an isolated worktree before execution and remove it after completion. WHEN worktree creation fails, THE SYSTEM SHALL fall back to in-place execution with a warning log.

### REQ-WIRE2-05: Knowledge Syncer and Search Wiring
WHEN the WorkerLoop starts AND LoopConfig.KnowledgeSync is true, THE SYSTEM SHALL instantiate a `knowledge.Syncer` and a `knowledge.FileWatcher` that watches WorkDir for file changes and syncs them to the backend. WHEN a task payload's Description is non-empty, THE SYSTEM SHALL call `knowledge.KnowledgeSearcher.Search()` with the description and populate the `KnowledgeCtx` field in the TaskPayload before building the prompt.

### REQ-WIRE2-06: Multi-Workspace Support
WHEN LoopConfig.Workspaces has more than one entry, THE SYSTEM SHALL instantiate a `workspace.MultiWorkspace`, register all workspace connections, and spawn a per-workspace A2A server goroutine. WHEN a task arrives with a workspace-specific routing target, THE SYSTEM SHALL use `MultiWorkspace.RouteTask()` to determine the correct WorkDir. WHEN LoopConfig.Workspaces has 0 or 1 entry, THE SYSTEM SHALL retain existing single-workspace behavior.

### REQ-WIRE2-07: LoopConfig Extension
THE SYSTEM SHALL extend LoopConfig with the following fields:
- `AuditLogPath string` — audit log file path (default: `{WorkDir}/.autopus/audit.jsonl`)
- `AuditMaxSize int64` — max log size before rotation (default: 10MB)
- `AuditMaxAge time.Duration` — max age of rotated files (default: 7 days)
- `WorkspaceID string` — workspace identifier for scheduler
- `MaxConcurrency int` — max parallel tasks (0 or 1 = sequential)
- `WorktreeIsolation bool` — enable worktree isolation for parallel tasks
- `KnowledgeSync bool` — enable knowledge file sync
- `Workspaces []WorkspaceConfig` — multi-workspace configuration

### REQ-WIRE2-08: Lifecycle Integration
THE SYSTEM SHALL integrate all 5 packages into the existing `loop_lifecycle.go` pattern established by SPEC-ADKWIRE-001. Service startup order: audit → knowledge syncer → scheduler → semaphore (parallel) → workspace goroutines. Shutdown order: reverse of startup. All services MUST gracefully stop within 5 seconds on context cancellation.

## 생성/수정 파일 상세

| 파일 | 역할 |
|------|------|
| `pkg/worker/loop.go` | (MOD) LoopConfig에 7개 필드 추가 (REQ-WIRE2-07) |
| `pkg/worker/loop_lifecycle.go` | (MOD) 5개 서비스 startup/shutdown 추가 |
| `pkg/worker/loop_exec.go` | (MOD) semaphore acquire/release, worktree create/remove, audit event 기록 |
| `pkg/worker/loop_audit.go` | (NEW) audit event 구조체 정의 및 write 헬퍼 |
| `pkg/worker/loop_knowledge.go` | (NEW) knowledge search/populate 헬퍼, watcher callback |
| `pkg/worker/loop_workspace.go` | (NEW) multi-workspace goroutine 스폰 및 라우팅 로직 |
