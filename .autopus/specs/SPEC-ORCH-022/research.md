# SPEC-ORCH-022 리서치

## 기존 코드 분석 (전부 file:line 직접 확인됨)

### 막는 지점 (gap)
- `internal/cli/orchestra_terminal.go:43-47` `paneInteractiveContext(claudeCode, ci, stdinTTY, stdoutTTY)` — `claudeCode != "" || ci != ""`이면 즉시 false. `detectStructuredTerminal()`(:26-36)가 이때 `terminal.PlainAdapter{}`를 반환한다. 이 함수는 `internal/cli/orchestra_run.go:136`(`Terminal: detectStructuredTerminal()`)과 `internal/cli/spec_review_loop.go:67`에서 호출된다.
- `pkg/orchestra/pane_backend.go:122` `var hookSession *HookSession` — nil 하드코딩. 같은 파일 :123 `waitForCompletion(ctx, b.cfg, pi, ..., hookSession, req.Round)`에 nil이 전달되어, `resolveCompletionDetector`(`cc21_monitor.go:16`)가 hook 경로를 못 타고 `ScreenPollDetector`(`cc21_monitor.go:33`)로 떨어진다.
- `pkg/content/hooks.go:46-87` `generateCLIHooks` — `PreToolUse`/`PostToolUse`만 등록하고 `Stop`/`AfterAgent` 이벤트를 등록하지 않는다. 따라서 `auto init`이 생성하는 `.claude/settings.json`에 `Stop` 키가 없다.
- `internal/cli/orchestra_helpers.go:237-251` `isHookModeAvailable()` — `~/.claude/settings.json`(user-global only)에 `"autopus"` + `"Stop"`이 동시 존재하면 true. `auto init`은 project-local `.claude/settings.json`에 쓰므로(아래) user-global을 보면 mismatch가 날 수 있다.

### 이미 동작하는 수신측 (재사용 대상, 재발명 금지)
- `pkg/orchestra/hook_signal.go:36-48` `NewHookSession` — `/tmp/autopus/{sanitize(session-id)}` 디렉토리를 `0o700`으로 생성. `:53-75` `WaitForDone`은 `{provider}-done` 파일을 200ms 폴링.
- `pkg/orchestra/hook_watcher.go:13-44` `WaitAndCollectHookResults` — provider별 goroutine으로 done-file 수집, fallback placeholder까지 포함. **headless 수집의 핵심 재사용 함수.**
- `pkg/orchestra/completion_file_ipc.go:23-48` `FileIPCDetector.WaitForCompletion` — context deadline 기반 bounded timeout(`fileIPCTimeout`:52-62, deadline 없으면 `defaultFileIPCTimeout=10m`). REQ-007의 결정적 실패가 이미 구현됨.
- `pkg/orchestra/completion_detector.go:29-37` `NewCompletionDetectorWithConfig` — 우선순위 `SignalDetector > FileIPCDetector(hookMode && session!=nil) > ScreenPollDetector`. REQ-006이 session을 비-nil로 만들면 자동으로 FileIPCDetector가 선택된다.
- `content/hooks/hook-claude-stop.sh` — `AUTOPUS_SESSION_ID`/`AUTOPUS_ROUND` env를 읽고, session-id를 `*[!a-zA-Z0-9_-]*` 패턴으로 traversal 검증(:14-16), result를 `chmod 600`(:46), stdin으로 hook JSON 수신(argv 주입 회피). `hook-gemini-afteragent.sh`, `hook-codex-stop.sh`도 동일 구조 확인.

### hook 스크립트 설치 경로 (T1 활성화의 절반은 이미 됨)
- `content/embed.go:9` `//go:embed ... hooks/*.sh ...` — `hook-claude-stop.sh` 등은 바이너리에 임베드됨.
- `pkg/adapter/claude/claude_prepare_files.go:56` `prepareContentFiles("hooks", ".claude/hooks/autopus")` — `content/hooks/*.sh`(claudeRootHookFiles 제외)를 `.claude/hooks/autopus/`로 복사. `isClaudeRootHookFile`(`claude_files.go:186`)는 `task-created-validate.sh`/`README.md`만 제외하므로 **`hook-claude-stop.sh`는 이미 `.claude/hooks/autopus/`에 설치된다.**
- 결론: T1이 추가할 것은 스크립트 복사가 아니라 **settings.json `hooks.Stop`에 명령 경로 등록**뿐이다. `claude_settings.go:38-136` `InstallHooks`는 이미 임의 이벤트(`h.Event`)를 nested schema로 쓰므로(:79-95), `generateCLIHooks`가 `Event:"Stop"` HookConfig를 반환하기만 하면 settings.json에 자동 기록된다.

## Outcome Lock

- **User-visible outcome**: cmux/tmux + 설치된 완료 hook 환경에서 `auto spec review` / `auto orchestra brainstorm`을 Claude Code 안(CLAUDECODE)에서 실행하면 provider들이 done-file IPC로 완료 수집되어 0/N timeout 없이 결과 반환. hook/세션 미가용이면 subprocess `-p` graceful degrade, 둘 다 불가면 actionable 에러.
- **Mandatory requirements**: REQ-001~REQ-010 (spec.md). 핵심은 T1(Stop hook 등록), T2(gemini AfterAgent + codex Stop), T3(isHookModeAvailable project-local 인식), T4(headless hook 수집), T5(가드 완화), T6(PaneBackend 실 HookSession).
- **Explicit non-goals**: provider-API 백엔드 신설 / JSON 스키마 변경 / 새 strategy / SCAMPER·ICE 프롬프트 변경 / plan·review·secure legacy 경로 변경 / 화면 스크래핑 재설계 / plain·CI subprocess floor 파괴 / `creack/pty` 직접 의존성 승격.
- **Completion evidence**: spec.md Completion evidence (1)~(6) oracle 수락.

## Visual Planning Brief

작업 성격: CLI/backend 실행 경로 분기 + 완료 신호 IPC. command-flow + sequence 다이어그램이 적합.

### 백엔드 선택 분기 (현재 vs 목표)
```
detectStructuredTerminal()  [현재]
  CLAUDECODE!="" ─────────────────────────► PlainAdapter ──► SelectBackend ──► subprocess -p (구독 불가)

detectStructuredTerminal()  [목표 — REQ-005]
  CLAUDECODE!="" 이지만:
    cmux 설치 AND isHookModeAvailable() ──► CmuxAdapter ──► hook-IPC pane 경로 (구독 세션 유지)
    그 외 ───────────────────────────────► PlainAdapter ──► subprocess -p floor (REQ-008 보존)
    cmux 없음 AND API 키 없음 ───────────► actionable error (REQ-009)
```

### headless hook 수집 sequence (REQ-004/006/007)
```
Orchestrator                cmux daemon            provider CLI (구독)        /tmp/autopus/{sid}/
  | NewHookSession(sid) ----------------------------------------------------> mkdir 0o700
  | setenv AUTOPUS_SESSION_ID=sid, AUTOPUS_ROUND=N
  | cmux new-split -----------> surface 생성
  | SendLongText(launch+prompt) -----------> claude 인터랙티브 세션 시작
  |                                            ...응답 생성...
  |                                            Stop hook 실행 ---------------> claude-result.json (0o600)
  |                                                                            claude-done (빈 파일)
  | FileIPCDetector.WaitForDone(timeout) <폴링 200ms>...detect ------------- claude-done
  | ReadResult(claude) <---------------------------------------------------- claude-result.json
  | [timeout 시] bounded deadline 경과 → false 반환 (REQ-007, 무한 hang 금지)
```

## 설계 결정 — 핵심 feasibility 질문: 구독 세션을 headless로 어떻게 구동하는가

검토한 세 옵션과 코드 근거:

### 옵션 A — creack/pty로 PTY attach
- 근거: `go.sum:42-43`에 `github.com/creack/pty v1.1.24`가 존재하나, `go mod why`는 `charmbracelet/x/xpty`(huh 테스트 의존)를 통한 **transitive**임을 보여준다. autopus-adk 소스는 `creack/pty`를 import하지 않는다.
- 평가: 직접 의존성 승격이 필요하고, PTY lifecycle·signal·resize·escape 처리를 새로 구현해야 한다(고복잡도, 300줄 한도 압박). 구독 세션은 PTY가 있으면 유지되지만, **cmux가 이미 그 추상을 제공**하므로 중복이다. **기각.**

### 옵션 B — cmux daemon detached surface (선택) ✅
- 근거: `pkg/terminal/cmux.go` 전체가 `execCommand("cmux", "new-split"|"send"|"read-screen"|...)` subprocess 호출로 surface를 구동한다(:25, :62, :86, :109, :202). 즉 **오케스트레이터 프로세스 자신은 TTY에 attach될 필요가 없고**, cmux daemon이 surface(PTY)를 호스팅한다. cmux daemon이 살아 있으면 비-TTY(Claude Code Bash) 컨텍스트에서도 `cmux new-split`로 surface를 만들고 거기서 구독 CLI를 인터랙티브로 띄울 수 있다.
- 완료 감지는 화면 스크래핑이 아니라 hook 파일 시그널(`FileIPCDetector`)로 하므로, 비-TTY에서 ReadScreen이 신뢰 불가하다는 문제(가드가 들어간 이유)를 우회한다.
- 평가: PTY 라이브러리 불필요, 기존 `CmuxAdapter` + `WaitAndCollectHookResults` 재사용. 제약: cmux daemon이 실행 중이어야 한다. daemon이 없거나 `cmux new-split`가 실패하면 REQ-008 degrade로 subprocess floor. **선택.** 신뢰도: 높음 (cmux subprocess-구동 구조를 코드로 확인).
- tmux도 동일 원리(`pkg/terminal/tmux.go`는 detached session에 `tmux new-window` 가능)지만, 1차 타깃은 cmux. tmux 지원은 동일 코드 경로로 자동 커버되며 별도 작업 아님.

### 옵션 C — headless에서 `-p`로 폴백 + hook으로 완료만 결정론화
- 평가: 구독 문제를 못 푼다(`-p`는 API 키 필요). 완료 신뢰성만 해결하는 부분 해법. REQ-008(degrade floor)로 이미 포함되므로 **주 경로로는 기각, fallback floor로만 유지.**

**결론**: 옵션 B. PTY 라이브러리 없이 cmux daemon이 호스팅하는 detached surface 위에서 구독 CLI를 인터랙티브로 띄우고, Stop/AfterAgent hook의 done-file을 `FileIPCDetector`로 감시한다. cmux daemon 미가용 시 subprocess floor로 graceful degrade(REQ-008), hook·cmux·API 키 모두 불가 시 actionable 에러(REQ-009).

## Semantic Invariant Inventory

| ID | source clause | invariant type | affected outputs | acceptance IDs |
|----|---------------|----------------|------------------|----------------|
| INV-001 | "auto init이 claude .claude/settings.json의 hooks.Stop에 hook-claude-stop.sh를 자동 등록" | parser/config row | settings.json `hooks.Stop[].hooks[].command` | S1 |
| INV-002 | "gemini AfterAgent + codex Stop 등록" | parser/config row | gemini hooks.json `autopus.AfterAgent`, codex Stop entry | S2 |
| INV-003 | "isHookModeAvailable() true 전이" (autopus+Stop 동시 존재) | boolean predicate | `isHookModeAvailable()` 반환값 | S3 |
| INV-004 | "done-file IPC로 완료 수집되어 화면 스크래핑 비의존" | detector selection | 선택 detector 타입 = FileIPCDetector | S4 |
| INV-005 | "CLAUDECODE+hook가용 시 backend/경로가 hook-수집 경로 선택(subprocess -p 아님)" | conditional routing | `detectStructuredTerminal()` 터미널 이름 | S5, S5b |
| INV-006 | "PaneBackend가 HookMode=true여도 hook IPC 미사용 → 실 HookSession 사용" | non-nil session | Execute의 hookSession, 선택 detector | S6 |
| INV-007 | "done-file 미수신 시 bounded timeout 결정적 실패(무한 hang 금지)" | bounded timeout | WaitForCompletion 반환 false + TimedOut | S7 |
| INV-008 | "hook 미설치 시 subprocess degrade 유지" | fallback routing | SelectBackend=subprocess, detectStructured=plain | S8 |
| INV-009 | "both-failed actionable 에러" | error contract | 에러 메시지 텍스트(복구 지침 포함) | S9 |
| INV-010 | "done-file 디렉토리 0o700, session-id traversal 방지, result 0o600" | security invariant | MkdirAll 모드, chmod, session-id 검증 분기 | S10 |

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| Stop hook 자동 등록 (claude) | Primary SPEC T1/REQ-001 | covered |
| gemini AfterAgent + codex Stop 등록 | Primary SPEC T2/REQ-002 | covered |
| hook 가용성 게이트 전이 | Primary SPEC T3/REQ-003 | covered |
| headless hook-IPC 수집 | Primary SPEC T4/REQ-004 | covered |
| CLAUDECODE 가드 완화 (hook-IPC 진입) | Primary SPEC T5/REQ-005 | covered |
| PaneBackend 실 HookSession | Primary SPEC T6/REQ-006 | covered |
| bounded timeout 결정적 실패 | Primary SPEC (기존 FileIPCDetector 재사용) | covered |
| subprocess degrade 보존 | Primary SPEC T5/T7/REQ-008 | covered |
| both-failed actionable 에러 | Primary SPEC T7/REQ-009 | covered |
| 보안 경계 | Primary SPEC T1/T4/REQ-010 (기존 NewHookSession 0o700 재사용) | covered |

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| hook-IPC done-file 수집이 실 환경에서 엔게이지하는지 미확인 | Outcome Lock ("done-file IPC로 완료 수집") | applyHookMode/RunInteractivePaneOrchestra를 instrument하여 spec review 실행 중 `/tmp/autopus/{sid}` 세션 디렉토리 + done-file 출현을 확인. 미출현 원인(HookMode set 여부, isHookModeAvailable cwd, RunInteractivePaneOrchestra 진입, NewHookSession 생성) 규명 후 닫는다. |

go 단계 oracle(라우팅 truth-table, env-injection 안전성, hookCollectionEligible, detector 선택, bounded timeout)은 모두 통과한다. 그러나 2026-06-10 실 e2e(임시 워크스페이스 + 단일 claude provider + `/tmp/autopus` 75s 폴링)에서 **spec review는 CLAUDECODE에서 0/N 실패 없이 동작(실 claude 리뷰 수집, REVISE 판정)했으나 hook 세션 디렉토리가 출현하지 않아 done-file IPC 수집이 demonstrable하게 확인되지 않았다.** 수집은 screen-scrape pane 또는 subprocess `-p`로 이루어진 것으로 추정된다. routing(T5)·env 주입(T4)·HookMode 배선(T8)·features.Monitor 디커플링은 구현·단위 검증되었으나, end-to-end hook-IPC done-file 경로의 실 환경 엔게이지는 BLOCKING Completion Debt로 남는다. 따라서 이 SPEC은 `implemented`이며 `completed`로 승급해서는 안 된다(false-complete 금지).

## Accepted Assumptions

These assumptions gate the Outcome Lock. They are accepted to proceed, but each carries a
validation experiment and an `If Wrong` consequence. They are not Completion Debt because the
REQ-007 bounded-timeout and REQ-008 subprocess floor make a wrong assumption fail safe (degrade
or deterministic timeout), not hang.

| ID | Accepted assumption | Confidence | Validation experiment | If Wrong |
|----|---------------------|------------|-----------------------|----------|
| A-001 | claude/codex/gemini가 **비-TTY 구독 인터랙티브 세션**(cmux surface 위)에서도 Stop/AfterAgent hook을 발화하고 `{provider}-done` + `{provider}-result.json` envelope를 신뢰성 있게 기록한다. (hook 발화 자체는 provider CLI lifecycle 소유이며 본 SPEC이 강제할 수 없음) | medium | go 단계 통합 실험: 실제 cmux daemon + Claude Code Bash에서 `auto spec review`를 1회 구동해 `/tmp/autopus/{sid}/`에 각 provider done-file이 생성되는지, S4/S7 oracle이 실 envelope로도 성립하는지 관측. claude 우선 확인 후 codex/gemini 순차. | REQ-007 bounded timeout이 해당 provider를 결정적으로 timeout 처리하고 REQ-008로 subprocess `-p` floor degrade. Outcome Lock의 0/N 부재 보장은 해당 provider에 한해 미달성 → go 단계에서 envelope 미기록이 관측되면 그 provider는 subprocess floor로만 수집하도록 명시하고 hook 발화 실패를 actionable 에러/로그로 노출(REQ-009 경로 재사용). |
| A-002 | cmux daemon이 살아 있으면 비-TTY 컨텍스트에서 `cmux new-split`로 detached surface를 만들고 거기서 구독 CLI를 인터랙티브로 띄울 수 있다. | high | `pkg/terminal/cmux.go` execCommand 구조 코드 확인(완료) + go 단계 `cmux new-split` 1회 실측. | `cmux new-split` 실패 시 즉시 subprocess degrade(REQ-008, Edge Case 1). |

A-001이 본 SPEC의 핵심 feasibility 리스크다. 본 SPEC은 envelope **수신·감시**(FileIPCDetector)와 **송신측 등록**(settings.json hook 명령)만 결정론적으로 보장하며, provider CLI가 그 hook을 실제로 실행해 envelope를 쓰는 행위는 외부 lifecycle에 의존한다. 따라서 oracle 테스트(S1~S10)는 등록·detector 선택·timeout·degrade·권한을 닫지만, "실 provider가 구독 세션에서 envelope를 쓴다"는 명제 자체는 go 단계 통합 실험(A-001 validation experiment)으로만 닫힌다.

## Evolution Ideas

These are optional improvements and do not block sync completion.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| opencode complete plugin을 동일 경로로 활성화 | 1차 타깃은 claude/gemini/codex 구독 세션 | 사용자가 opencode 구독 수집을 명시 요청 |
| cmux daemon 부재 시 자동 daemon 기동 | degrade floor로 충분, 자동 기동은 부작용 위험 | 사용자가 daemon 자동 관리를 요청 |
| done-file 수신 진행률 실시간 표시 | 완료 결정론성과 무관한 UX 개선 | 사용자가 진행률 UI 요청 |

## Sibling SPEC Decision

| Decision | Reason | Sibling SPEC IDs |
|----------|--------|------------------|
| none | 단일 cohesive 활성화 작업(끊긴 hook 활성화 경로 잇기). 독립 배포/보안 경계/migration sequencing 사유 없음. 태스크 7개·소스 파일 ~8개로 25/40 임계값 미만. | None |

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| `internal/cli/orchestra_terminal.go:43-47` `paneInteractiveContext` | existing | Read 확인 |
| `pkg/orchestra/pane_backend.go:122` `var hookSession *HookSession` | existing | Read 확인 (nil 하드코딩) |
| `pkg/content/hooks.go:46` `generateCLIHooks` | existing | Read 확인 (Stop 미등록) |
| `internal/cli/orchestra_helpers.go:237-251` `isHookModeAvailable` | existing | Read 확인 |
| `pkg/orchestra/hook_watcher.go:13` `WaitAndCollectHookResults` | existing | Read 확인 |
| `pkg/orchestra/hook_signal.go:36` `NewHookSession` (0o700) | existing | Read 확인 |
| `pkg/orchestra/completion_file_ipc.go:23` `FileIPCDetector` | existing | Read 확인 |
| `pkg/orchestra/completion_detector.go:29` `NewCompletionDetectorWithConfig` | existing | Read 확인 |
| `pkg/terminal/cmux.go` (execCommand 기반 surface 구동) | existing | Read 확인 |
| `pkg/adapter/claude/claude_prepare_files.go:56` `prepareContentFiles("hooks",...)` | existing | Read 확인 (스크립트 이미 설치) |
| `pkg/adapter/claude/claude_settings.go:38` `InstallHooks` (임의 이벤트 기록) | existing | Read 확인 |
| `content/hooks/hook-claude-stop.sh` / `hook-gemini-afteragent.sh` / `hook-codex-stop.sh` | existing | Read 확인 |
| `github.com/creack/pty v1.1.24` | existing (transitive) | `go mod why` 확인 (직접 import 없음) |
| `[NEW] pkg/orchestra/headless_hook_runner.go` | planned addition | 미존재 |
| `[NEW] pkg/orchestra/headless_hook_runner_test.go` | planned addition | 미존재 |
| `[NEW] pkg/content/hooks_stop_test.go::TestGenerateCLIHooks_StopEvent` | planned addition | 미존재 |
| `[NEW] internal/cli/orchestra_terminal_test.go::TestPaneInteractiveContext_HookIPC` | planned addition | 기존 테스트에 케이스 추가 |

## Reviewer Brief

- **Intended scope**: SPEC-ORCH-007이 구현한 hook 파일 시그널 수집을 SPEC-ORCH-021의 인터랙티브 pane 경로와 결합해 Claude Code(CLAUDECODE) 안에서 활성화한다. 신규 백엔드·스키마·strategy 없음.
- **Explicit non-goals**: provider-API 백엔드 / JSON 스키마 변경 / 새 strategy / brainstorm 프롬프트 변경 / legacy plan·review·secure 경로 / 화면 스크래핑 재설계 / plain·CI subprocess floor 파괴 / creack/pty 직접 의존성. 리뷰어는 이 항목들로 scope를 확장하지 말 것.
- **Self-verified**: Traceability Matrix(REQ↔Task↔Scenario↔INV), Semantic Invariant Inventory(10개 oracle 매핑), oracle acceptance(S1·S3·S5·S7·S10 concrete 값), existing/[NEW] reference discipline, 옵션 A/B/C feasibility 코드 근거.
- **Reviewer should focus on**: correctness(가드 완화가 plain/CI floor를 깨지 않는가), convergence safety(degrade·both-failed 경로가 결정적인가), regression risk(기존 RunInteractivePaneOrchestra hook 분기와 충돌 없는가), Completion Debt only. 새 제품 scope 제안 금지.

## Technology Stack Decision

brownfield. 본 SPEC은 기존 autopus-adk 모듈(Go 1.26, `go.mod` toolchain go1.26.2)을 변경하며 새 런타임/프레임워크/package manager를 도입하지 않는다. cmux는 외부 CLI 바이너리(설치 전제)이고 의존성 manifest 항목이 아니다. `creack/pty`는 transitive로만 존재하며 본 SPEC은 직접 의존성으로 승격하지 않는다(옵션 B 선택). 새 라이브러리 version 결정이 없으므로 greenfield freshness 표는 N/A.

## Self-Verify Summary
- Q-CORR-01 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: 모든 비-[NEW] 경로/심볼(orchestra_terminal.go:43, pane_backend.go:122, hooks.go:46, hook_watcher.go:13, cmux.go, claude_prepare_files.go:56 등)을 Read로 직접 확인함.
- Q-CORR-02 | status: PASS | attempt: 1 | files: spec.md, research.md, plan.md | reason: 신규 파일/테스트는 모두 [NEW] 마커로 표기하고 정합성 PASS 근거에서 제외함.
- Q-CORR-03 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: EARS type(Ubiquitous/Event-driven/Unwanted/Where) + Priority(Must) 분리, acceptance는 bare Given/When/Then 사용.
- Q-CORR-04 | status: PASS | attempt: 1 | files: research.md | reason: Reference Discipline 표가 existing(rg/Read 확인) vs [NEW] planned를 분리하고 creack/pty의 transitive 성격을 go mod why로 구분함.
- Q-COMP-01 | status: PASS | attempt: 1 | files: spec.md, plan.md, acceptance.md, research.md | reason: 4파일이 목적·계획·검증·근거로 상호 보완.
- Q-COMP-02 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: Traceability Matrix가 REQ-001~010을 Task·Scenario·INV에 모두 연결, 누락 없음.
- Q-COMP-03 | status: PASS | attempt: 1 | files: spec.md | reason: 각 REQ에 EARS type·조건·기대결과·관측 지점을 명시.
- Q-COMP-04 | status: PASS | attempt: 2 | files: research.md | reason: Outcome Lock의 mandatory가 Primary SPEC requirements/plan/Must acceptance로 닫히고 Completion Debt=None. 단, provider hook 발화는 외부 lifecycle 의존이라 A-001 Accepted Assumption으로 명시(Completion Debt 아님: 가정이 틀려도 REQ-007/008로 fail-safe).
- Q-COMP-05 | status: PASS | attempt: 2 | files: spec.md, plan.md, acceptance.md, research.md | reason: 1차에서 INV-001/INV-005/INV-010만 oracle이고 나머지가 structural에 가까웠음 → S1·S3·S5·S7·S10에 concrete 값(settings.json command 문자열, FileIPCDetector 타입, 터미널 이름, timeout 경계, 0o700/0o600 모드) 추가해 oracle 강화.
- Q-COMP-06 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: Traceability Matrix + Reviewer Brief(scope/non-goals/self-verified/focus) 존재.
- Q-COMP-07 | status: PASS | attempt: 1 | files: research.md | reason: Completion Debt(None)와 Evolution Ideas(opencode/daemon/진행률 — 모두 Outcome Lock 밖)를 분리, ID 미부여.
- Q-FEAS-01 | status: PASS | attempt: 2 | files: research.md, plan.md | reason: 런타임 Go 코드 변경 + config 생성기 변경으로 layer 정확. 핵심 feasibility 리스크(구독 인터랙티브 세션에서 provider가 envelope를 신뢰성 있게 기록하는지)를 1차에서 명시 가정으로 분리하지 않았음 → Accepted Assumptions A-001로 캡처하고 validation experiment(go 단계 실 cmux+Claude Code Bash 통합 관측)와 If Wrong(REQ-007 timeout/REQ-008 floor degrade)을 기록.
- Q-FEAS-02 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: 대상 경로가 모두 autopus-adk module 내부, content/(source) vs .claude/hooks/autopus(generated install) 구분.
- Q-FEAS-03 | status: PASS | attempt: 1 | files: plan.md, acceptance.md | reason: 검증이 go test + auto init 후 settings.json 검사로 실제 수행 가능.
- Q-STYLE-01 | status: PASS | attempt: 1 | files: spec.md | reason: REQ description에 should/might/could 등 모호어 없음.
- Q-STYLE-02 | status: PASS | attempt: 1 | files: spec.md | reason: Priority(Must)와 EARS type을 별도 축으로 분리.
- Q-STYLE-03 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: 문장 완결, acceptance는 bare Given/When/Then/And.
- Q-SEC-01 | status: PASS | attempt: 1 | files: spec.md, research.md, acceptance.md | reason: provider done-file 출력을 untrusted 입력으로 취급(merge sanitize 의존) 명시, prompt injection 경계 기술.
- Q-SEC-02 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: session-id traversal 방지(영숫자/-/_ 검증), 0o700/0o600, headless --dangerously-skip-permissions 위험 문서화.
- Q-SEC-03 | status: PASS | attempt: 1 | files: research.md | reason: /tmp/autopus/{sid}는 Cleanup으로 제거되는 ephemeral artifact, secret 미기록.
- Q-COH-01 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: 하나의 명확한 문제(hook 활성화 경로 잇기)와 밀접한 변경 대상으로 수렴.
- Q-COH-02 | status: PASS | attempt: 1 | files: research.md | reason: Outcome Lock 필수 작업이 Primary SPEC에 포함, optional은 Evolution Ideas로만.
- Q-COH-03 | status: N/A | attempt: 1 | files: research.md | reason: sibling SPEC 없음(Sibling SPEC Decision=none). 적용 대상 아님.
