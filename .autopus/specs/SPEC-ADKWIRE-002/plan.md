# SPEC-ADKWIRE-002 구현 계획

## 태스크 목록

- [ ] T1: `pkg/worker/loop.go` — LoopConfig 필드 7개 추가 + WorkspaceConfig 타입 정의
- [ ] T2: `pkg/worker/loop_audit.go` — audit event 구조체 및 JSON write 헬퍼 생성
- [ ] T3: `pkg/worker/loop_knowledge.go` — knowledge search/populate 및 watcher callback 생성
- [ ] T4: `pkg/worker/loop_workspace.go` — multi-workspace goroutine 스폰 및 라우팅 로직 생성
- [ ] T5: `pkg/worker/loop_lifecycle.go` — 5개 서비스 startup/shutdown 추가 (ADKWIRE-001 확장)
- [ ] T6: `pkg/worker/loop_exec.go` — semaphore wiring, worktree isolation, audit event 호출 추가
- [ ] T7: 단위 테스트 — audit write, knowledge populate, parallel semaphore 통합, workspace routing

## 에이전트 배정

| Task | Agent | 의존성 | 예상 라인 |
|------|-------|--------|-----------|
| T1 | executor-1 | 없음 | ~25 |
| T2 | executor-1 | 없음 | ~70 |
| T3 | executor-2 | 없음 | ~80 |
| T4 | executor-2 | 없음 | ~90 |
| T5 | executor-3 | T1-T4 | ~80 |
| T6 | executor-3 | T1, T2, T5 | ~40 |
| T7 | tester | T1-T6 | ~150 |

## 구현 전략

### T1: LoopConfig 확장
- `LoopConfig`에 REQ-WIRE2-07의 7개 필드 추가
- `WorkspaceConfig` struct 정의: `WorkspaceID`, `ProjectDir`, `BackendURL`, `AuthToken`
- 기존 필드와 충돌 없음, 순수 additive 변경

### T2: Audit Event 헬퍼 (loop_audit.go)
- `AuditEvent` struct: `TaskID`, `Event` (started/completed/failed), `Timestamp`, `DurationMS`, `CostUSD`
- `writeAuditEvent(w *audit.RotatingWriter, evt AuditEvent)` — JSON marshal + newline + write
- WorkerLoop에 `auditWriter *audit.RotatingWriter` 필드 추가를 위한 인터페이스 설계
- 파일 크기 목표: 70줄 이내

### T3: Knowledge 통합 (loop_knowledge.go)
- `populateKnowledge(ctx, searcher *knowledge.KnowledgeSearcher, description string) string`
  - description을 query로 사용, 결과를 포맷하여 반환
  - 검색 실패 시 빈 문자열 반환 (non-blocking)
- `startKnowledgeWatcher(ctx, syncer *knowledge.Syncer, watcher *knowledge.FileWatcher)`
  - watcher의 onChange 콜백에서 syncer.SyncFile 호출
- WorkerLoop에 `knowledgeSearcher`, `knowledgeSyncer`, `knowledgeWatcher` 필드 추가

### T4: Multi-Workspace (loop_workspace.go)
- `startWorkspaceGoroutines(ctx, workspaces []WorkspaceConfig, handler TaskHandler) *workspace.MultiWorkspace`
  - 각 워크스페이스별 A2A Server 생성 및 goroutine 스폰
  - errgroup으로 에러 전파
- `routeWorkDir(mw *workspace.MultiWorkspace, workspaceID string, defaultDir string) string`
  - MultiWorkspace가 nil이면 defaultDir 반환

### T5: Lifecycle 확장 (loop_lifecycle.go)
- SPEC-ADKWIRE-001이 만든 `startServices()`/`stopServices()` 패턴에 5개 서비스 추가
- 시작 순서: audit → knowledge(syncer+watcher) → scheduler → (semaphore는 stateless, 시작 불필요) → workspace goroutines
- 종료 순서: workspace → scheduler → knowledge → audit
- 각 서비스는 context.Context로 제어, 5초 graceful shutdown timeout

### T6: Execution Path 수정 (loop_exec.go)
- `handleTask` 진입점에서:
  1. Knowledge search → KnowledgeCtx 채우기 (T3의 populateKnowledge 호출)
  2. Audit "started" 이벤트 기록
- `executeSubprocess` 래핑:
  1. Semaphore acquire (MaxConcurrency > 1일 때만)
  2. Worktree create (WorktreeIsolation && MaxConcurrency > 1일 때만)
  3. 실행
  4. Worktree remove
  5. Semaphore release (defer)
  6. Audit "completed"/"failed" 이벤트 기록

### 기존 코드 변경 최소화 원칙
- loop.go는 LoopConfig 필드 추가만 (구조체 정의)
- loop_exec.go는 handleTask/executeSubprocess에 호출 삽입 (각 5-10줄)
- loop_lifecycle.go는 기존 ADKWIRE-001 패턴에 서비스 추가
- 신규 로직은 모두 새 파일(loop_audit.go, loop_knowledge.go, loop_workspace.go)에 분리
