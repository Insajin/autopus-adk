# SPEC-ORCH-021: Reliable interactive-pane orchestration as the default execution path (subscription-first), with best-effort subprocess fallback

**Status**: completed
**Created**: 2026-05-30
**Domain**: ORCH

## 목적

Provider CLI의 `-p`(headless print/subprocess) 모드는 provider의 **API 경로**로만 동작한다. 그러나 **대부분의 유저는 구독제(Claude Pro/Max, ChatGPT Plus, Gemini 구독 등)로 AI를 사용**하며, 구독 세션은 **로그인된 인터랙티브 CLI 세션**으로만 접근 가능하고 `-p` API 모드로는 쓸 수 없다(별도 API 키/과금 필요). 따라서 인터랙티브 pane은 "fallback 회피용 기본값"이 아니라 **대부분의 유저가 멀티프로바이더를 돌릴 수 있는 유일한 실행 경로**다. `-p` subprocess는 API 키 보유 유저에 한정된 best-effort 경로로 강등된다.

이 작업의 진짜 목표는 단순 default flip이 아니라 **인터랙티브(pane) 모드가 안정적으로 굴러가는 것**이다. 역사적으로 pane 기본값이 버려진 이유(commit `ca7f026`: "cmux pane 모드에서 SignalDetector 타임아웃 → poll 폴백으로 불필요한 지연")가 바로 이 안정성 약점이므로, 그것을 정면으로 닫는다. 이 SPEC은 cmux/tmux를 쓸 수 있는 터미널에서 **인터랙티브 pane 실행을 신뢰성 있는 기본값**으로 만들고, `-p` subprocess 실행을 **best-effort fallback**으로 재배치한다. 대상은 현재도 subprocess를 기본값/hardcode로 쓰는 진입점 세 곳이다: **idea/brainstorm**, **spec review(structured)**, **orchestra run(structured)**.

`plan`/`review`/`secure`(legacy `RunOrchestra` 경로)는 이미 `SubprocessMode`를 넘기지 않아 cmux/tmux에서 pane을 쓰므로 이 SPEC의 변경 대상이 아니다(비목표로 명시).

## Outcome Boundary

- **Outcome Lock**: 구독 세션만 보유한 유저가 cmux/tmux 가능한 터미널에서 `/auto idea`, `/auto spec review`, `auto orchestra run`을 실행하면 **설정된 모든 provider(claude, codex, gemini)가 각자 올바른 호출 형태로** `-p`(API) 없이 **로그인된 인터랙티브 pane 세션으로 안정적으로 멀티프로바이더 실행**된다(세션 ready 후 prompt 전송, bounded 완료 감지, hang 없음). 특히 gemini는 argv 파싱 에러로 즉사하지 않고(현재 `agy --print` 무값 → `flag needs an argument`), codex는 구조화 리뷰에서 누락되지 않는다(현재 `review_gate.providers`에서 빠짐). plain/CI 터미널이면 `-p` subprocess로 실행을 시도한다. pane 실행이 실패하면 `-p` subprocess를 best-effort로 시도하되, `-p`마저 미가용(API 미보유 등)이면 **혼란스러운 raw API 에러가 아니라 actionable 에러**로 끝내고, 어느 경로로 실행됐는지(또는 둘 다 실패했는지) 기록된다.
- **Mandatory requirements**: REQ-001~REQ-008(기존) + REQ-009~REQ-013(pane 신뢰성 그룹) + REQ-014~REQ-016(provider 호출 정합성 그룹: subprocess argv·pane argv·참여 정합성, 아래).
- **Explicit non-goals**: plan/review/secure 경로 변경 / provider-API 백엔드 신설(Evolution) / JSON 스키마·structured 출력 계약 자체 변경 / 새 strategy 추가 / brainstorm SCAMPER·ICE 프롬프트 변경 / 새 완료감지·세션ready·hook IPC 메커니즘 재발명(기존 메커니즘 재사용·경화만) / `auto init` hook 설치 자체를 이 SPEC에서 수행.
- **Completion evidence**: backend-selection truth table 행 검증 oracle(S1~S9), pane 신뢰성 acceptance(S10~S14), 그리고 provider 호출 정합성 oracle(S15~S20: gemini subprocess `--print` 값 형태·무값 `--print` 금지, gemini pane `--print` 미포함, codex subprocess `exec`+`--output-schema`, codex pane `exec` 미시작, review provider 집합에 codex 포함)이 통과하고, S6이 실제 실행 backend 이름(`pane`/`subprocess`)을 단언하며, `go build ./...` 및 `go test ./pkg/orchestra/... ./internal/cli/...`가 통과한다. 운영 검증은 `auto doctor`의 `runProviderTransportSmoke`(provider-smoke)로 보강한다(환경 의존 통합).

## 요구사항

### REQ-001 — brainstorm가 capable 터미널에서 pane 기본값

Ubiquitous / Priority: Must

WHERE the terminal is cmux or tmux capable (`terminal.DetectTerminal()` returns non-nil and `Name() != "plain"`), THE SYSTEM SHALL execute the `orchestra brainstorm` providers via the interactive pane path by default, without requiring the user to pass `--subprocess=false`. Observability: the brainstorm `--subprocess` flag default flips to `false`, and a run on a cmux/tmux terminal does not append `-p` to provider argv.

### REQ-002 — spec review structured 경로가 pane-first

Ubiquitous / Priority: Must

WHERE the shared pane-capability predicate is true (terminal non-nil, `Name()` not `"plain"`, and `SubprocessMode` not forced), THE SYSTEM SHALL select the interactive-pane execution backend for the structured spec review path (`runStructuredSpecReviewOrchestra`). WHERE the predicate is false because the terminal is nil OR its `Name()` equals `"plain"` OR `SubprocessMode` is forced, THE SYSTEM SHALL select the subprocess backend. Observability: the backend chosen is the value returned by `SelectBackend` (which consumes the shared predicate) and is recorded per provider in the review result.

### REQ-003 — orchestra run structured 경로가 pane-first

Ubiquitous / Priority: Must

WHERE the shared pane-capability predicate is true (terminal non-nil, `Name()` not `"plain"`, and `--subprocess` not set), THE SYSTEM SHALL execute the `orchestra run` structured pipeline via the interactive-pane execution backend, and WHERE the predicate is false because the terminal is nil OR its `Name()` equals `"plain"` OR `--subprocess` is set, THE SYSTEM SHALL execute it via the subprocess backend. Observability: `SubprocessPipelineConfig.Backend.Name()` equals `pane` or `subprocess` according to the shared predicate result.

### REQ-004 — structured 명령을 위한 실 인터랙티브 pane 실행 경로 존재

Ubiquitous / Priority: Must

THE SYSTEM SHALL provide an interactive-pane `ExecutionBackend` implementation whose `Execute(ctx, req)` drives one interactive provider pane per request (launch session, send prompt, detect completion, read and sanitize the screen) and returns a `ProviderResponse` whose `Output` is parseable by `orchestra.OutputParser`. The parseable-output invariant is asserted by oracle acceptance for the roles that have a validating parser entry point and a corresponding oracle scenario: `reviewer` (`ParseReviewer`), `debater_r1` (`ParseDebaterR1`), and `judge` (`ParseJudge`). The `debater_r2` role shares the same `Execute` and sanitize path but is not separately oracle-asserted here because `ParseDebaterR2` performs no role-specific validation beyond JSON unmarshal; it is covered transitively by the same sanitize+`extractJSON` path. Observability: the backend's `Name()` returns `pane`, and `ParseReviewer`/`ParseDebaterR1`/`ParseJudge` succeed on its output in oracle tests with embedded JSON.

### REQ-005 — pane 실패 시 best-effort subprocess fallback, 실패 시 actionable 에러, 실행 경로 기록

Event-driven / Priority: Must

WHEN interactive-pane execution for a structured provider request fails OR the terminal is not pane-capable, THE SYSTEM SHALL attempt `-p`/stdin subprocess execution as a best-effort fallback and record which backend actually produced the response. WHEN that subprocess fallback is itself unavailable or fails (e.g. the user has only a subscription session and no API access, so `-p` cannot authenticate), THE SYSTEM SHALL surface a single actionable error that names both the pane failure and the unavailable `-p` fallback and tells the user how to recover (ensure a logged-in cmux/tmux CLI session), instead of terminating with a raw provider/API error. Observability: for a recoverable fallback the recorded executed-backend identifier equals `subprocess` and the response is parseable structured output; for the both-failed case the error message contains an actionable recovery instruction and the recorded path indicates neither backend succeeded.

### REQ-006 — spec review 경로에 terminal 감지 주입

Ubiquitous / Priority: Must

WHERE the structured spec review config is constructed (`spec_review.go`/`spec_review_loop.go`), THE SYSTEM SHALL populate `OrchestraConfig.Terminal` via terminal detection so that backend selection can distinguish pane-capable from plain/CI terminals. Observability: after construction, `OrchestraConfig.Terminal` is non-nil on a cmux/tmux terminal and the structured review path no longer hardcodes the subprocess backend.

### REQ-007 — 단일 backend 선택 규칙

Ubiquitous / Priority: Must

THE SYSTEM SHALL route every backend-selection decision through one shared pane-capability predicate `paneCapable(term, subprocessMode)` keyed on (terminal nil-ness, `Name()=="plain"`, `SubprocessMode`), and both the legacy `RunOrchestra` guard (`runner.go`, used by brainstorm) and `SelectBackend` (used by the structured spec review and orchestra run paths) SHALL consume this single predicate. The two paths run different execution models — brainstorm uses the multi-round `RunPaneOrchestra` pane path, while the structured paths use the per-provider pane `ExecutionBackend` — but their selection INPUT is structurally single, so they cannot diverge on which mode (pane vs subprocess) a given (terminal, `SubprocessMode`) implies. Observability: for identical (terminal, `SubprocessMode`) inputs, the shared predicate returns one boolean, and oracle tests assert the legacy guard and `SelectBackend` agree on that predicate result (pane-mode vs subprocess-mode) across all three entry points.

### REQ-008 — pane 화면 출력 정제 후 사용

Ubiquitous / Priority: Must

WHERE the pane execution backend reads provider output from the terminal screen, THE SYSTEM SHALL sanitize that output with the existing screen sanitizer (`SanitizeScreenOutput` / `CleanScreenForCrossPollination`) before returning it, so ANSI escapes, status bars, and CLI banners do not corrupt downstream JSON parsing or leak terminal control sequences. Observability: backend output for a screen containing ANSI/banner noise yields sanitized, parseable text in oracle tests.

### REQ-009 — 세션 ready 전 prompt 미전송 (입력 유실 방지)

Event-driven / Priority: Must

WHEN the pane execution backend launches an interactive provider session, THE SYSTEM SHALL detect session readiness with the existing `isSessionReady`/`SessionReadyPatterns` mechanism (bounded by `startupTimeoutFor(provider)`) BEFORE sending the prompt, and SHALL NOT send the prompt while the session is not yet ready. WHEN the startup timeout elapses before readiness, THE SYSTEM SHALL record the provider as failed rather than send into an unready session. Observability: in oracle tests, no prompt-send occurs before a session-ready pattern matches; when readiness never matches within `startupTimeoutFor`, the provider response is marked failed/timed-out and no prompt was sent.

### REQ-010 — 완료 감지는 hook/monitor 우선, poll fallback, bounded

Event-driven / Priority: Must

WHEN the pane execution backend waits for a provider to finish, THE SYSTEM SHALL reuse the existing layered completion detection (`waitForCompletion`): hook/monitor event-driven detection first, then fall back to `ScreenPollDetector` polling, bounded by `MonitorTimeout` and the overall per-provider timeout. THE SYSTEM SHALL NOT introduce a new completion-detection mechanism. Observability: in oracle tests with `MonitorEnabled`, completion is detected via the event-driven detector when it fires within `MonitorTimeout`, and otherwise the system transitions to polling before the overall timeout.

### REQ-011 — 완료 timeout 시 결정적 실패 결과 (무한 hang 금지)

Event-driven / Priority: Must

WHEN neither hook/monitor nor poll detects completion within the bounded per-provider timeout, THE SYSTEM SHALL return a deterministic `ProviderResponse` marked timed-out/failed (with whatever partial sanitized screen output was captured) and SHALL NOT block indefinitely or return garbage. Observability: in an oracle test where completion never occurs, `Execute` returns within the bounded timeout with `TimedOut`/failed set, and the call does not hang.

### REQ-012 — hook 미설치 시 graceful degrade

Event-driven / Priority: Must

WHERE `auto init` completion hooks are not installed (no `HookSession`/`HookMode`), THE SYSTEM SHALL degrade gracefully to monitor→poll completion detection without erroring, since hook IPC (`FileIPCDetector`/`HookSession`, the most reliable signal) depends on hook installation (SPEC-ORCH-007 R5/R6). Observability: in oracle tests with `HookMode` disabled, the resolved completion detector is the non-hook path (monitor or `ScreenPollDetector`) and execution still completes.

### REQ-013 — pane + subprocess 동시 실패 시 actionable 에러 (구독 현실 비은폐)

Event-driven / Priority: Must

WHEN both pane execution and the best-effort `-p` subprocess fallback are unavailable or fail for a run, THE SYSTEM SHALL present an actionable, user-facing error that explicitly states the subscription-vs-API reality (interactive pane unavailable AND `-p` subprocess fallback unavailable) and the concrete recovery step, rather than hiding the subscription-only limitation behind a raw API error. Observability: the surfaced error string contains both failure causes and a recovery instruction (e.g. "ensure a cmux/tmux session and that the provider CLI is logged in") in an oracle/behavioral test.

### REQ-014 — provider 호출 정합성: subprocess 모드 argv가 프롬프트를 올바른 형태로 전달

Ubiquitous / Priority: Must

WHERE a provider is executed in subprocess mode, THE SYSTEM SHALL construct an argv that delivers the prompt in the form the provider CLI actually accepts, and SHALL NOT construct a form that fails argument parsing before the prompt is read. Specifically: (a) for gemini (`agy`), the prompt SHALL be passed as the value of `--print`/`-p` (e.g. `agy --print "<prompt>"`), never as a bare `agy --print` with the prompt only on stdin — because `agy`'s `--print` requires an argument value and a valueless `agy --print` exits with `flag needs an argument: -print` (live-confirmed); (b) for codex, the subprocess argv SHALL be `codex exec` with the prompt via argument or stdin plus `--output-schema <FILE>` for structured roles. Observability: a unit test asserts the constructed gemini subprocess argv contains the prompt immediately after `--print`/`-p` (or in the `--print` value slot) and never produces a valueless `--print`; a unit test asserts the codex subprocess argv begins with `exec` and contains `--output-schema` when a schema path is supplied.

### REQ-015 — provider 호출 정합성: pane 모드 argv가 대화형 세션 형태

Ubiquitous / Priority: Must

WHERE a provider is executed in the true interactive pane mode (SPEC-ORCH-021 pane path), THE SYSTEM SHALL construct a pane launch argv for an interactive session, not a non-interactive print/exec form. Specifically: (a) for gemini, the pane argv SHALL NOT contain `--print` (non-interactive); it SHALL launch an interactive `agy` session (bare `agy` or `-i`/`--prompt-interactive`) so prompts are sent via keystrokes — the current `PaneArgs:["--print"]` defeats the pane path and SHALL be corrected; (b) for codex, the pane argv SHALL launch the interactive `codex` TUI (without the `exec` non-interactive subcommand) for sendkeys input. Observability: a unit test asserts the gemini pane argv does not contain `--print`, and the codex pane argv does not begin with `exec`.

### REQ-016 — provider 참여 정합성: 구조화 리뷰 provider 집합 일관성

Ubiquitous / Priority: Must

WHERE the structured spec review provider set is resolved (`review_gate.providers`), THE SYSTEM SHALL ensure the set is consistent with the orchestra command provider set (`[claude, codex, gemini]`) so that no configured provider is silently dropped from structured review, OR the deliberate subset SHALL be documented with its rationale. The default configuration SHALL include codex in the structured review provider set, and codex SHALL respond to structured review through its `--output-schema` JSON contract. Observability: a unit test asserts codex is present in the resolved review provider set for the default config, and that codex's structured argv includes `--output-schema`.

## 생성 파일 상세

- `[NEW] pkg/orchestra/pane_backend.go`: interactive-pane `ExecutionBackend` 구현(`Execute`/`Name`). 단일 provider pane을 띄워 prompt 전송→완료 감지→screen read→sanitize→`ProviderResponse` 반환. subprocess fallback 위임 포함. 300줄 미만 유지(초과 시 `pane_backend_collect.go`로 분할).
- `[NEW] internal/cli/orchestra_terminal.go`: structured 경로(spec review, orchestra run)에서 `terminal.DetectTerminal()`을 주입하고 공유 선택 규칙을 호출하는 작은 헬퍼. `orchestra_run.go`/`spec_review_structured.go`가 300줄에 근접하므로 별도 파일로 분리.
- `pkg/orchestra/backend.go` (existing): `SelectBackend`가 (a) `[NEW] paneCapable(term, subprocessMode)` 공유 술어를 소비하도록 하고 (b) non-nil `"plain"` 터미널을 subprocess로 처리하도록 수정한다. 현재 `backend.go:50`은 `cfg.SubprocessMode || cfg.Terminal == nil`만 검사하여 non-nil `"plain"`에서 가짜 pane(`NewPaneBackend`→child-process `runProvider`)을 반환하는 결함이 있다(F-001). pane-capable일 때는 새 pane backend를 반환한다.
- `[NEW] pkg/orchestra/pane_capable.go` (또는 `backend.go` 내 신규 함수 `paneCapable`): 공유 pane-capability 술어. `runner.go:25` 가드와 `SelectBackend`가 둘 다 이 술어를 소비하여 선택 입력을 구조적으로 단일화한다(F-002/F-006).
- `internal/cli/orchestra_brainstorm.go` (existing): `--subprocess` 기본값 `true`→`false`.
- `internal/cli/spec_review_runtime.go` / `internal/cli/orchestra_run_runtime.go` (existing): hardcode된 `specReviewBackendFactory`/`orchestraRunBackendFactory`를 선택 규칙 경유로 교체(또는 선택 결과 주입).
- `internal/cli/spec_review_loop.go` / `internal/cli/spec_review.go` (existing): `OrchestraConfig.Terminal` 주입.
- `internal/cli/orchestra_run.go` (existing): 폐기된 `_ = SelectBackend(cfg)` 자리에 실제 선택 결과를 파이프라인 backend로 사용 + `cfg.Terminal` 주입.
- `[NEW] pkg/orchestra/pane_backend_test.go`, `[NEW] internal/cli/orchestra_terminal_test.go`: truth table oracle 테스트.
- `pkg/orchestra/pane_backend.go` (위 항목, 신뢰성 경화): `Execute`가 (1) `waitForSessionReady`/`isSessionReady`+`startupTimeoutFor`로 세션 ready 후에만 prompt 전송, (2) `waitForCompletion`(hook/monitor→poll, `MonitorTimeout` bounded) 재사용, (3) 완료 timeout 시 결정적 timed-out `ProviderResponse` 반환, (4) hook 미설치 시 `resolveCompletionDetector`의 비-hook 경로로 degrade를 따른다. 기존 메커니즘 재사용 — 새 감지기 작성 금지.
- `[NEW] pkg/orchestra/pane_fallback.go` (또는 backend 파일 내 함수): pane→best-effort `-p`→both-failed 처리와 actionable 에러 메시지 구성(REQ-005/REQ-013). pane_backend.go가 300줄에 근접하면 fallback/에러 구성과 collect를 별도 파일로 분할.
- `[NEW] pkg/orchestra/pane_backend_reliability_test.go`: 신뢰성 behavioral/oracle 테스트(S10~S14: 세션 ready 후 전송, bounded 완료, timeout 시 no-hang 결정적 실패, hook 미설치 degrade, both-failed actionable 에러).
- `autopus.yaml` (existing, module-local source of truth `autopus-adk/autopus.yaml` `orchestra.providers`): gemini를 subprocess에서 프롬프트가 `--print` 값으로 가도록 정정(예: `args: [--print, ""]` placeholder 패턴 — `buildSubprocessArgs`/`injectPromptArg`가 빈 슬롯을 프롬프트로 치환), pane은 `pane_args`에서 `--print` 제거(대화형). codex `args`의 deprecated `--full-auto`를 `--sandbox workspace-write`로 정정(`codex exec --full-auto`는 `warning: --full-auto is deprecated; use --sandbox workspace-write`만 내고 동작하나 표준화). `review_gate.providers`에 codex 추가(`[claude, codex, gemini]`).
- `internal/cli/orchestra_helpers.go:145-147` (existing, config 부재 시 fallback registry): gemini 기본값 `Args:["--print"], PromptViaArgs:false`(stdin)를 모드 정합 형태로 정정 — subprocess는 프롬프트를 `--print` 값으로(예: `Args:["--print",""], PromptViaArgs:true` 또는 placeholder), pane은 `PaneArgs`에서 `--print` 제거. codex `Args` `exec --sandbox workspace-write` 유지(이미 올바름), `PaneArgs`는 `exec` 없이 대화형 유지 확인. SchemaFlag `--output-schema` 유지.
- `configs/autopus.yaml` (existing, 배포 템플릿): 위 정정과 동일하게 동기화(설치 시 기본값이 정합하도록).
- `[NEW] pkg/orchestra/provider_argv_test.go` (또는 `internal/cli/orchestra_helpers_test.go`에 추가): provider argv 정합성 oracle 단위 테스트(REQ-014/REQ-015/REQ-016) — gemini subprocess `--print` 값 포함 + 무값 `--print` 금지, gemini pane `--print` 미포함, codex subprocess `exec`+`--output-schema`, codex pane `exec` 미시작, review provider 집합에 codex 포함.
- `internal/cli/doctor_provider_smoke.go:83/112/116` (existing, 운영 검증 경로): `runProviderTransportSmoke`/`providerSmokePrompt`/`classifyProviderSmokeResult`는 실 provider 응답 스모크 검증으로 plan/research에 명시(argv 단위 테스트가 1차 oracle, doctor provider-smoke가 환경 의존 통합 검증).

## Related SPECs

- None (Primary SPEC가 Outcome Lock을 단독으로 닫는다). Sibling SPEC 없음 — `research.md`의 `## Sibling SPEC Decision` 참조.
- 선행 컨텍스트(역전 대상): SPEC-ORCH-019(structured backend + brainstorm subprocess 기본값), commit `ca7f026`. SPEC-ORCH-006(cmux/tmux pane auto-enable). 이들은 변경하지 않고 기본값 정책만 역전한다.
- 의존(변경 안 함): SPEC-ORCH-007 R5/R6(`auto init` 완료 hook 설치). hook IPC(`FileIPCDetector`/`HookSession`)는 가장 안정적인 완료 신호지만 hook 설치에 의존한다. 이 SPEC은 hook 미설치 시 monitor→poll로 graceful degrade하며(REQ-012), hook 설치 자체는 수행하지 않는다.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-001 brainstorm pane default | T1, T6 | S1, S7 | INV-001, INV-007 |
| REQ-002 spec review pane-first | T2, T3 | S2, S3, S6 | INV-001, INV-002, INV-005 |
| REQ-003 orchestra run pane-first | T4 | S4, S5 | INV-001, INV-002 |
| REQ-004 real pane ExecutionBackend | T5 | S8 (reviewer + debater_r1 + judge) | INV-003, INV-006 |
| REQ-005 best-effort fallback + actionable error + record path | T5, T6, T9 | S6, S14 | INV-004, INV-005, INV-012 |
| REQ-006 terminal detection injected | T3 | S2, S3 | INV-002 |
| REQ-007 shared pane-capability predicate | T2, T2b, T6 | S1, S2, S4, S9 | INV-001 |
| REQ-008 sanitize pane screen output | T5 | S8 | INV-006 |
| REQ-009 session-ready before prompt | T8a | S10 | INV-008 |
| REQ-010 layered completion (monitor→poll, bounded) | T8b | S11 | INV-009 |
| REQ-011 deterministic failure on timeout (no hang) | T8b | S12 | INV-010 |
| REQ-012 graceful degrade when hooks absent | T8b | S13 | INV-011 |
| REQ-013 actionable error when pane + -p both fail | T9 | S14 | INV-012 |
| REQ-014 subprocess argv prompt-form correctness (gemini/codex) | T11, T12 | S15, S16, S18 | INV-013, INV-015 |
| REQ-015 pane argv interactive-form (gemini/codex) | T11, T12 | S17, S19 | INV-014 |
| REQ-016 review provider parity + codex --output-schema | T13 | S18, S20 | INV-016 |

## Completion Verdict

> Recorded by `/auto sync SPEC-ORCH-021` (2026-05-30).

- **Outcome Lock**: satisfied — interactive pane is the default execution path for the three structured entry points (brainstorm, spec review structured, orchestra run structured), with a real per-provider pane `ExecutionBackend`, session-ready/bounded-completion reliability hardening, best-effort `-p` subprocess fallback, actionable both-failed errors, executed-backend path recording, and provider argv correctness (gemini `--print` value form, codex `exec --output-schema`, codex present in `review_gate.providers`).
- **Mandatory requirements**: 16/16 (REQ-001~REQ-016).
- **Must acceptance**: 22/22 (S1~S20, including S8a/S8b/S8c). Verified by `go build ./...`, `go vet`, and `go test ./pkg/orchestra/... ./internal/cli/... ./pkg/config/...`. The reliability suite (S10~S14) uses real-time timeout bounds; tests are deterministic in isolation but the 20s-bounded pre-existing timing tests can flake under concurrent multi-package execution (see verification note).
- **Completion Debt**: none.
- **Evolution Ideas** (optional, unscheduled): provider-API execution backend; pane completion-latency benchmark + `MonitorTimeout`/`startupTimeoutFor` tuning; full skill-prose backend rewrite. C0-byte strip on screen sanitize (security Low F-1).
- **Verification note**: timing-bound tests in `failure_diagnostics_test.go`, `interactive_debate_helpers_test.go`, `pipeline_subprocess_test.go`, `runner_timeout_config_test.go`, and `safety_evidence_test.go` can flake under parallel package execution; they pass deterministically when `pkg/orchestra` runs in isolation. Hardening their timing assumptions is an optional follow-up, not a blocker.
- **Review evidence**: Phase 4 reviewer APPROVE + security-auditor PASS (go phase).
