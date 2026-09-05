# SPEC-OMP-006: OMP RPC 리뷰 backend와 판정 단계

---
id: SPEC-OMP-006
title: OMP RPC 리뷰 backend와 판정 단계
version: 1.0.0
status: completed
priority: HIGH
created: 2026-09-05
domain: OMP
---

## Purpose

`auto spec review`와 `auto orchestra`의 provider 실행은 `pkg/orchestra.ExecutionBackend`(structured 경로)와 `runProvider`(직접 subprocess 경로)로 나뉘고, 둘 다 외부 CLI(`claude --print`, `codex exec`, `agy --print`)만 실행한다. 2026-09-05 SPEC-OMP-005 게이트 실측에서 codex CLI 사용량 한도, `agy` headless 권한 거부(`empty_output`), gemini 25분 타임아웃이 연달아 나와 3-provider 정족수를 채우지 못했다. OMP는 세 프로바이더의 인증·라우팅을 이미 갖고 있고, SPEC-OMP-004가 `internal/cli`에 OMP RPC 세션 클라이언트를 두었다.

또한 SPEC 리뷰 러너는 `NoJudge: true`로 고정돼 `spec.review_gate.judge` 설정과 달리 판정을 하지 않고 독립 리뷰를 supermajority로 합친다. 리뷰어가 `--no-tools`라 프롬프트 밖 코드를 확인할 수 없어 참조 검증 류 checklist를 추측으로 답한다.

이 SPEC은 (1) provider 단위 opt-in `backend: omp`로 리뷰어를 OMP RPC 세션(read/grep/glob만, LSP·MCP·web search 비활성)에서 실행하고 — structured SPEC 리뷰 경로와 `auto orchestra`의 직접 실행 경로 모두 — (2) SPEC 리뷰에 typed judge 단계를 켠다. 리비전 루프·checklist·receipt·status 승격은 그대로 둔다.

## Outcome Boundary

- **Outcome Lock**: `orchestra.providers.<name>.backend: omp` + `model`인 provider는 어느 실행 경로(structured 리뷰 fan-out, `auto orchestra`의 `RunOrchestra` 직접 경로, judge)에서든 외부 CLI 없이 프로젝트 cwd의 OMP RPC 세션에서 프롬프트를 실행하고, `spec.review_gate.judge`가 설정된 SPEC 리뷰는 typed judge를 실행해 receipt에 provider별 `executed_backend`와 judge 블록을 남긴다.
- **Mandatory requirements**: provider 스키마와 fail-closed 검증, 요청 단위 라우팅(structured·직접·judge 경로), read-only OMP 세션(argv·도구 allowlist·hardening overlay·격리 런타임), 모델·thinking·family, judge 설정 해석·프롬프트·출력 검증·판정 우선순위·verify 모드 ID 보존·fallback, receipt/review.md 관측면, dogfood.
- **Explicit non-goals**: pane backend 변경, `auto orchestra` 전략(라운드 구조) 변경, OMP를 provider로 자동 등록, 리뷰어에 write/edit/bash/MCP/LSP 도구 부여, 읽기 경로 sandbox(OMP `read`가 접근 가능한 경로를 파일시스템 수준에서 제한하는 것 — Residual Risk 참조), Agent Hub task 패널, SPEC-OMP-003/005 모델 프로젝션 변경.
- **Completion evidence**: S1-S15 Must oracle 통과, `go test ./...` 통과, autopus-adk에서 `backend: omp` 3-provider + judge로 `auto spec review SPEC-OMP-006 --subprocess --auto`가 완료되고 `review-receipt.json`의 `providers[].executed_backend`가 세 provider 모두 `omp`, `judge.status`가 `ok`이며 실행 로그에 `OMP 리뷰 세션: provider=<name> model=<selector> tools=glob,grep,read`가 세 번 나온다.

## Requirements

### Ubiquitous

**REQ-BACKEND-001 — provider 단위 backend 선언**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL accept `orchestra.providers.<name>.backend` with values `""`(기본, pane/subprocess/CLI) or `omp`, SHALL require `model` as `<provider>/<model-id>[:<thinking>]` when `backend` is `omp`, SHALL accept `tools` only from `{read, grep, glob}` and SHALL expand an omitted or empty `tools` list to all three, SHALL normalize `tools` by sorting and de-duplicating (`EffectiveTools`), and SHALL reject any other backend value, a missing model, or a tool outside the allowlist with `provider_backend_invalid: <name>`, `provider_model_required: <name>`, or `provider_tools_invalid: <name>`.
Observability: `config.Validate` error text; `orchestra.ProviderConfig{Backend, Model, Tools}`.

**REQ-BACKEND-002 — 요청 단위 라우팅**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL execute a provider whose `Backend == "omp"` through the OMP backend on every execution path — the structured SPEC review fan-out (`specReviewBackendFactory` → `NewRoutedBackend`), the SPEC review judge, and every `RunOrchestra` strategy (`runParallel` consensus, debate round 1/rebuttal/judge, `runFastest`, `runPipeline`, `runRelay`, `recheckTransport`) through `OrchestraConfig.ProviderBackends` and `runConfiguredProvider` — SHALL execute every other provider exactly as before, SHALL skip the `Binary` installed-preflight for `omp` providers, and SHALL fail with `provider <name> backend "omp" is not available in this execution path` instead of spawning `Binary` when a path has no registered OMP backend.
Observability: `ProviderResponse.ExecutedBackend == "omp"`; routed backend `Name() == "subprocess+omp"` (or `pane+omp`); receipt `providers[].executed_backend`.

**REQ-SESSION-001 — read-only OMP 리뷰 세션**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL run each `omp` request in a fresh private OMP RPC process whose argv is exactly `--mode rpc --no-session --no-extensions --session-dir <base>/pipeline-task-*/sessions --no-skills --no-lsp --config <base>/review-hardening.yml --tools <sorted csv> --approval-mode yolo --max-time <seconds>` (`<base>` = the call's private 0700 review runtime, `<seconds>` = provider timeout, default 1800), cwd = project directory, canonical environment, executable identity pinned as in SPEC-OMP-004, SHALL negotiate protocol v2 and disable auto retry/compaction, SHALL issue `set_model{provider, modelId}` and, only when the selector carries a thinking level, `set_thinking_level{level}`, SHALL accept both the omp 17.x prompt acknowledgement (`data.agentInvoked=true`) and the omp 18.1.x bare acknowledgement (no `data`) and settle the turn on the `agent_start`/terminal `agent_end` lifecycle, SHALL return `get_last_assistant_text`, and SHALL remove `<base>` (sessions, materialized executable, and overlay) after the call.
Observability: log `OMP 리뷰 세션: provider=<name> model=<selector> tools=<csv>`; fixture-recorded argv; absence of the runtime directory after completion.

**REQ-SESSION-002 — family와 실패 분류**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL derive `ProviderResponse.ModelFamily` from the selector's provider token (`anthropic`→`anthropic`; `openai`, `openai-codex`→`openai`; `google`, `google-antigravity`, `google-vertex`→`google`; otherwise the token itself), SHALL report context deadline expiry as `TimedOut=true`, an RPC failure as `ExitCode=1` with the error text, and an empty assistant text after a normally stopped turn as `EmptyOutput=true`; SHALL read the terminal `agent_end.messages` and, when the last assistant message has `stopReason=error`, SHALL report `ExitCode=1` with `provider error status <errorStatus>: <errorMessage ≤240 chars>` (never `EmptyOutput`), and when it has `stopReason=aborted`, an aborted-turn error; SHALL retry a turn whose `errorStatus` ∈ {408, 409, 425, 429, 500, 502, 503, 504, 529} in a fresh private process on the same pinned model after `attempt × 5s`, at most 3 attempts in total and never past the request deadline; and SHALL fail closed with `executed model mismatch: want <provider>/<model> got <provider>/<model>` when the assistant message's `provider`/`model` differ from the pinned selector (an absent identity is accepted for older runtimes).
Observability: `ProviderResponse` fields; structured review failure classes `timeout`, `execution_error`, `empty_output`, `provider_error`, `provider_model_error`; log `OMP 리뷰 세션 재시도: provider=<name> attempt=<n>/3 reason=<error>`.

**REQ-SESSION-003 — 도구 allowlist와 런타임 검증**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL pass to the OMP session only the built-in tools in the provider's effective allowlist (a subset of `{read, grep, glob}`), SHALL disable LSP tools (`--no-lsp`), extension discovery (`--no-extensions`), skills (`--no-skills`), and — through the hardening overlay — project MCP servers, web search, memory, and LSP settings, and, because `--tools` validates built-in names only while MCP, custom, and extension tools arrive through discovery, SHALL read the live session's `get_state.dumpTools` after protocol negotiation and SHALL refuse to send the prompt (`ExitCode=1`, `Error` naming the extra tools) when any reported tool is outside the allowlist or when the session does not report its tool set.
Observability: argv `--tools`; overlay content; RPC command order (`get_state` before `set_model`); error `OMP review session exposed tools outside the allowlist: <names>`.

**REQ-SESSION-004 — hardening overlay**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL write, before the process starts, `<base>/review-hardening.yml` (mode 0600 inside the 0700 `<base>` runtime) containing exactly `lsp.enabled: false`, `mcp.enableProjectConfig: false`, `web_search.enabled: false`, `secrets.enabled: true`, `memory.backend: off`, `tools.approvalMode: yolo`, SHALL pass it with `--config`, and SHALL delete it with the runtime.
Observability: fixture-recorded overlay content; overlay absence after completion.

**REQ-JUDGE-003 — judge 출력 유효성**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL treat a judge output as valid only when it is a `review_judge` JSON object whose `verdict` ∈ {PASS, REVISE, REJECT}, whose `findings` array is present (empty only with `PASS`), whose every finding has `severity` ∈ {critical, major, minor, suggestion}, `decision` ∈ {accept, reject, merge}, a `reason` when `decision != accept`, `merge_into` naming an `accept` finding's `id` when `decision == merge`, `sources` drawn only from the anonymized reviewer aliases, and a unique `id` when supplied (uniqueness holds across every decision), and whose `REVISE`/`REJECT` verdict is backed by at least one `accept` finding; SHALL classify anything else as `invalid` with a reason.
Observability: `ReviewResult.Judge.Status`, `Reason`.

### Event-driven

**REQ-JUDGE-001 — SPEC 리뷰 판정 단계**
Type: Event-driven | Priority: Must
WHEN `spec.review_gate.judge` names a provider and at least one reviewer output parsed, THEN THE SYSTEM SHALL resolve the judge `ProviderConfig` from the harness `orchestra.providers` entry of that name (so a judge outside the reviewer set keeps its `backend`/`model`/`tools`), SHALL build the judge prompt from the SPEC id, review mode, reviewer outputs relabeled `Reviewer A/B/C…` in the prompt headers (reviewer body text is passed verbatim and the template instructs the judge to ignore self-identification inside it), prior findings in verify mode, and the `review_judge` schema, SHALL execute it through the routed backend, SHALL parse it under REQ-JUDGE-003, and SHALL append the response as `<judge> (judge)` with `Role: "judge"`.
Observability: log `SPEC 리뷰 judge 완료: <judge> (verdict=…, accepted=…, rejected=…, merged=…)` / `SPEC 리뷰 judge 실패: <judge> (<class>): <reason>`; receipt `judge` block.

**REQ-JUDGE-002 — 판정 우선순위**
Type: Event-driven | Priority: Must
WHEN a valid judge output exists, THEN THE SYSTEM SHALL apply, in this order: (1) replace the supermajority verdict and findings with the judge verdict and the `accept` findings (`merge` findings fold their `sources` into the accepted finding named by `merge_into` and do not open a new finding; `reject` findings are dropped and counted), (2) downgrade `PASS` to `REVISE` only when an accepted finding that is still active (`open` or `regressed`) is `critical` or `major` — prior findings the judge resolved never veto convergence, (3) in verify mode keep each accepted finding's prior `ID`/`FirstSeenRev`, union the prior attribution with the judge sources, set `regressed` when the prior was `resolved` or `regressed` and `open` otherwise, mark prior findings absent from the accept set `resolved`, and number new findings after the highest prior ID (the existing verify scope lock then classifies such new findings `out_of_scope` rather than `open`, exactly as it does for reviewer-added findings), (4) apply the existing verify scope lock, deterministic static findings, and `effectiveReviewVerdict` (a reviewer checklist `FAIL` sustains `REVISE` but never lifts `PASS`; a judge `PASS` with zero accepted findings therefore stays `PASS`, matching the legacy rule).
Observability: `ReviewResult.Judge{Provider, Family, Status, Verdict, Accepted, Rejected, Merged, AcceptedIDs, Rationale, Reason}`; `review.md` `## Judge`; receipt `judge`.

### Unwanted

**REQ-JUDGE-004 — judge fallback**
Type: Unwanted | Priority: Must
IF the judge fails, times out, or returns an invalid output, THEN THE SYSTEM SHALL keep the supermajority verdict and findings unchanged, SHALL record `Judge.Status` as `failed` or `invalid` with the reason, SHALL add a `FailedProvider{Role: "judge"}` entry, and SHALL still write the receipt `judge` block.
Observability: receipt `judge.status`, `judge.reason`; `review.md` `## Judge`.

**REQ-OBS-001 — receipt 관측면**
Type: Unwanted | Priority: Must
IF a review completes, THEN THE SYSTEM SHALL record in `review-receipt.json` a `providers[]` row per configured provider with `status` and `executed_backend`, and a `judge` block `{provider, family, status, verdict, accepted, rejected, merged, accepted_ids, rationale(≤500 chars), reason}`; IF the judge did not run, THEN the `judge` block SHALL be absent.
Observability: receipt JSON.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-BACKEND-001 | T1 | S1 | INV-001, INV-007 |
| REQ-BACKEND-002 | T1, T3, T7 | S2, S12 | INV-001 |
| REQ-SESSION-001 | T2, T8 | S3, S4 | INV-002, INV-006 |
| REQ-SESSION-002 | T2, T10 | S4, S5, S15 | INV-003, INV-006, INV-010 |
| REQ-SESSION-003 | T1, T2, T8 | S1, S3, S13, S14 | INV-002, INV-007 |
| REQ-SESSION-004 | T8 | S13 | INV-002, INV-009 |
| REQ-JUDGE-003 | T9 | S11 | INV-004, INV-008 |
| REQ-JUDGE-001 | T4, T5, T9 | S6, S7, S10 | INV-004 |
| REQ-JUDGE-002 | T5, T9 | S7, S10 | INV-004, INV-005 |
| REQ-JUDGE-004 | T5 | S8 | INV-004 |
| REQ-OBS-001 | T9 | S8, S9 | INV-004 |
| Completion evidence | T6 | S9 | INV-001, INV-004 |

## Residual Risk

- OMP `read`는 프로세스 사용자가 읽을 수 있는 모든 경로와 URL을 읽을 수 있다. 이 SPEC은 write/shell/MCP/LSP/web search를 막고 `secrets.enabled: true`로 알려진 토큰 패턴을 난독화하지만, 저장소 파일에 심긴 지시로 리뷰어가 홈 디렉터리 파일을 읽어 리뷰 출력에 실을 가능성은 남는다. 리뷰 출력은 `review.md`에 보존되므로 이 위험은 "정보 노출"로 한정되며, 파일시스템 sandbox는 non-goal이다.
- judge 프롬프트에 리뷰어 본문이 그대로 들어가므로 리뷰어 출력 안의 지시문이 judge에 영향을 줄 수 있다. 템플릿이 리뷰어 본문을 비신뢰 데이터로 다루도록 지시하고, judge 출력은 REQ-JUDGE-003으로 형식·의미 검증된다.

## Out of Scope

Outcome Boundary의 explicit non-goals와 동일하다.
