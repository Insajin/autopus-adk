# SPEC-HARNESS-WORKFLOW-001: harness workflow — `--workflow` opt-in 라우트 기반층

**Status**: completed
**Created**: 2026-06-19
**Domain**: HARNESS-WORKFLOW
**Target module**: autopus-adk (하네스 정본)
**Source**: BS-HARNESS-WORKFLOW-001 (Outcome Lock + Clarification Ledger + Technology Stack Decision 이관)

## Completion Verdict

- **Outcome Lock**: satisfied — manifest 2파일 SoT + 파생 generated JS + parity 게이트 + doctor capability gate + fallback taxonomy + Go 경계 + dry-run 생성기 + 최소 결정적 4-phase 실행(Gate=CommandRunner exit-code) + release hygiene 종단 phase가 Outcome Lock의 user-visible 결과를 닫는다. Feature Coverage Map의 Primary 슬라이스 전부 `covered`.
- **Mandatory requirements**: 12/12 (REQ-001~012 전부 구현).
- **Must acceptance**: 14/14 — S1-S11·S13·S16 hermetic green(pkg/workflow·pkg/content·pkg/adapter·internal/cli `-race` ok), S15는 operational completion evidence(라이브 `/auto go --workflow` 디스패치 + run journal, hermetic Go 단위 불가로 구현 중 1회 실증). Should S12·S14 green.
- **Completion Debt**: none (research.md `## Completion Debt` 근거: JS→Go gate 경계는 `auto workflow gate` CLI bridge로 명세·S16 hermetic CLI 오라클로 커버, unresolved debt 아님).
- **Evolution Ideas**: 14건 surfaced as optional, not scheduled (worktree fan-out·resume·Saga·budget rollover·기본 승격 등 — 사용자 명시 요청 시에만 승격).
- **Sibling**: SPEC-HARNESS-WORKFLOW-GATE-002 (approved, Primary 비의존; 결정적 게이트 엔진 — 본 SPEC 완료에 영향 없음).
- **Sync 검증(go 자가보고 불신→게이트 재실행)**: `go build ./...` exit 0 · `go vet`(workflow/content/adapter/claude) 0 · `gofmt -l` 내 변경 파일 clean · `go test -race` pkg/workflow·pkg/content·pkg/adapter/claude ok + pkg/adapter(parity) ok + internal/cli(workflow) ok · golangci-lint pkg/workflow 0 issues·내 변경 파일 NEW 위반 0 · file-size 내 신규/수정 .go 전부 ≤300(최대 claude_generate.go 166). 작업트리의 codex 테스트 2건 FAIL은 외래 codex 템플릿/스킬 드리프트 산물(SPEC 무관)이며 격리빌드로 자기완결 증명.

## 목적

Autopus-ADK 하네스의 `/auto go` Route A는 메인 세션이 마크다운 절차를 해석해 Agent를 수동 스폰하므로 비결정적이고 단계 누락·표류·재개 불가 문제가 있다. Claude Code의 Dynamic Workflows(GA 기능)는 게이트·재시도·시퀀싱을 코드로 표현해 결정적 제어 흐름을 제공한다. 이 SPEC은 그 위에 안전한 opt-in 라우트 기반층을 만든다: 안정 계약(manifest)을 단일 진실 소스로 두고, 재생성 가능한 얇은 JS 어댑터를 파생하며, 비-claude 플랫폼은 회귀 0으로 Route A에 폴백한다.

핵심 아키텍처 제약(techstack 실사로 정밀화): Workflow JS 저작 API는 공식 문서에 비공개이고 안정성/버전 고정 정책이 없는 내부 프리미티브다. 따라서 고정 JS 아티팩트를 정본으로 박는 전략은 위험하며, 정본은 사람이 편집하는 manifest(`route_a.md` + `route_a.schema.json`)로, JS는 manifest에서 파생되는 generated-surface로 강등한다.

## Outcome Boundary

### Outcome Lock (사용자/운영자 가시 결과)
claude-code 사용자가 `auto workflow render --dry-run`으로 생성된 workflow JS / manifest / schema / prompt-manifest 해시를 실행 없이 검토할 수 있고, `/auto go --workflow`로 claude-code에서 최소 결정적 4-phase 실행(Planning → Implementation → deterministic Gate(빌드/테스트 exit-code 기반) → release hygiene)을 수행할 수 있으며, 비-claude 플랫폼(codex/gemini/opencode)과 doctor capability gate 실패 시 회귀 0으로 기존 Route A에 fail-fast 폴백한다. 정본은 manifest 2파일(md+schema)이며, 여기서 파생된 generated JS와 manifest/schema 간 phase-id·retry·budget·result-type 정합을 parity 게이트로 드리프트 차단한다(JS는 정본이 아니라 generated-surface).

### Mandatory requirements (Primary 슬라이스 한정)
- SoT = manifest 2파일(md + schema.json). JS는 manifest에서 파생되는 generated-surface이며 정본 아님. generate.go parity 게이트(md↔schema↔generated-js 집합 정합, 드리프트 fail-closed).
- `auto workflow doctor` capability gate(Primary가 쓰는 프리미티브만 hard-gate, 후속/Evolution 프리미티브는 advisory 프로브).
- Fallback taxonomy(fail-fast / fail-closed / resumable / explicit, silent 금지) + 비-claude Route A 폴백 회귀0.
- Go 런타임 경계 보존(repo 변형=Go `pkg/pipeline`, 시퀀싱=JS).
- `auto workflow render --dry-run` 생성기(JS+manifest+schema+prompt-manifest 해시 검토).
- 최소 결정적 4-phase 실행(deterministic Gate=exit-code, LLM committee 제외) + release hygiene 종단 phase(generated-surface drift gate + Lore/300 + sync).

### Explicit non-goals (이번에 하지 않음)
- 결정적 게이트 엔진(progress vector 서킷브레이커 + review 2-phase + verdict committee, T3) → sibling SPEC-HARNESS-WORKFLOW-GATE-002.
- worktree fan-out 실행(T5), resume 무효화 입자도(T4), Saga 2-tier 보상(AP1), budget rollover+reservation(AP2), 기본 `/auto go` Route A 승격(K13, harness-bench 증거 전제).
- codex/gemini/opencode를 Workflow로 끌어올리기(폴백 유지, polyfill 거부), `--team`의 Workflow 재구현(공존만), 백엔드 `durable_workflow`/Symphony 변경.
- 출력(output) 결정성 약속(제어 흐름 결정성 + 관측 증거만 약속).

### Completion evidence
동일 manifest → 동일 phase 실행 순서(dry-run 골든 + fake-`PhaseBackend` replay), parity 게이트 fail-closed 테스트, 비-claude 폴백 회귀0 회귀 테스트, doctor fail-fast→폴백 테스트, drift gate 차단 테스트, prompt-manifest 해시 결정성 골든.

## Requirements

EARS 형식. 각 요구사항의 정규 문장은 단독 라인이며, Priority(Must/Should/Nice)는 EARS type과 별도 축이다.

### REQ-001 — manifest를 workflow 정본으로 고정
THE SYSTEM SHALL treat `[NEW] content/workflows/route_a.schema.json` as the machine-authoritative source for phase-id, retry, budget, and result-type sets, and `[NEW] content/workflows/route_a.md` as the human contract documenting those phase-ids, together forming the workflow source of truth.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: harness workflow 정의가 변경될 때 이 두 정본 파일만 편집 대상이다(phase-id 집합 권위 = schema.json JSON, md는 사람 문서로 별도 markdown 문법/파서 불요).
- Observability: `content/workflows/`에 두 파일이 존재하고 generate 산출물이 schema.json에서 파생됨을 generate 로그와 parity 게이트(schema↔js 집합 + md phase-id presence)로 확인한다.

### REQ-002 — manifest에서 JS 어댑터 파생 및 임베드
WHEN `generate-templates` runs, THEN THE SYSTEM SHALL derive `[NEW] templates/claude/workflows/route_a.workflow.js.tmpl` from the SoT manifest and embed it via the `claude/workflows/*.tmpl` glob.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: `pkg/content.GenerateAllTemplates` 실행 시점.
- Observability: 파생된 `.js.tmpl`이 templates 트리에 존재하고 embed FS에 포함됨을 generate 출력과 `go build`로 확인한다.

### REQ-003 — parity 게이트 드리프트 fail-closed
IF the derived JS and `route_a.schema.json` diverge on phase-id, retry, budget, or result-type sets, OR any schema phase-id is absent as a string token in `route_a.md`, THEN THE SYSTEM SHALL fail generation closed with a non-zero exit code, name the diverging element, and skip writing the JS.
- EARS type: Unwanted behavior
- Priority: Must
- Trigger/Condition: parity 게이트가 세 산출물의 집합 불일치를 탐지할 때.
- Observability: 종료 코드와 stderr의 diverging phase 이름으로 확인한다.

### REQ-004 — 생성된 workflow는 직접편집 금지 generated-surface
WHEN the claude adapter generates, THEN THE SYSTEM SHALL write `[NEW] .claude/workflows/route_a.workflow.js` as a generated, edit-forbidden surface and register it in the manifest.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: claude 어댑터 Generate 시점.
- Observability: 생성물 파일 + 매니페스트 엔트리 + 파일 헤더의 generated 경고 문구로 확인한다.

### REQ-005 — 비-claude 플랫폼 회귀 0
WHEN the codex, gemini, or opencode adapter generates, THEN THE SYSTEM SHALL emit no workflow JS, no `--workflow` route, and no `--workflow`-bearing harness-workflow skill (the harness-workflow skill is claude-scoped) while preserving the existing Route A command.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: 비-claude 어댑터 Generate 시점.
- Observability: 비-claude 산출물에 `*workflow*.js` 0건, `--workflow` 토큰 0건, Route A 존재로 확인한다.

### REQ-006 — workflow doctor capability gate
WHEN `[NEW] auto workflow doctor` runs, THEN THE SYSTEM SHALL hard-gate only the primitives this Primary route uses (claude-code availability, minimum version >= 2.1.154, agent, schema, phase) and exit non-zero when any of those is unavailable, while probing follow-on primitives (parallel, isolation, budget, agent-model-override) as advisory non-gating capabilities reported but not failing the gate.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: doctor 명령 실행 시점, required 프리미티브 프로브 결과에 따라(advisory 프리미티브는 verdict에 영향 없음).
- Observability: 구조화 리포트(JSON)의 per-primitive status(required는 게이팅, advisory는 비게이팅으로 구분 표기)와 종료 코드로 확인한다.

### REQ-007 — `--workflow` 라우트 게이팅 및 fail-fast 폴백
WHEN the `/auto go --workflow` route is selected and the doctor capability gate fails or the platform is non-claude, THEN THE SYSTEM SHALL fall back to Route A without executing any workflow.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: 라우트 진입 시 doctor verdict=fail 또는 platform!=claude.
- Observability: 라우터가 방출하는 fallback-class 로그 라인과 Route A 진입으로 확인한다. opt-in은 `--workflow` 플래그(skill/command 경로)로만 트리거되며 사용자 키워드 타이핑에 의존하지 않는다.

### REQ-008 — fallback taxonomy 분류, silent 금지
THE SYSTEM SHALL classify every workflow route failure into exactly one of fail-fast, fail-closed, resumable, or explicit, and SHALL NOT silently opt out.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: 임의의 workflow 실패 경로 발생 시.
- Observability: 분류기(`[NEW] pkg/workflow` fallback classifier)의 반환 클래스와 로그 라인으로 확인한다.

### REQ-009 — Go 런타임 경계 보존
THE SYSTEM SHALL keep repository-mutating operations (worktree create/remove via the existing `pkg/pipeline.WorktreeManager`, branch naming, the default worktree slot cap of 5, worktree reclaim) in the Go runtime, with the workflow JS owning sequencing only.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: 병렬 구현 phase가 worktree 작업을 스케줄링할 때.
- Observability: 기존 `pkg/pipeline.WorktreeManager`(`Create`/`Remove`/`ActiveCount`, max-limit)와 `RunConfig.WorktreeSlotCap`(기본 5)·`ScheduleWorktreeTasksWithCap`가 동시성 상한·repo 변형 소유를 강제함을 `pkg/pipeline` 테스트로 확인한다.

### REQ-010 — dry-run 생성기
WHEN `[NEW] auto workflow render --dry-run` runs against the canonical manifest, THEN THE SYSTEM SHALL emit the generated workflow JS, manifest, schema, and a deterministic prompt-manifest hash for inspection without executing any agent.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: render 명령 실행 시점.
- Observability: stdout 산출물과 골든 스냅샷 비교로 확인한다(prompt-manifest 해시는 `pkg/promptlayer` 재사용).

### REQ-011 — 최소 결정적 4-phase 실행
THE SYSTEM SHALL define the deterministic workflow as the ordered phases Planning, Implementation, deterministic Gate, and release hygiene, where the deterministic Gate verdict derives from build and test command exit codes rather than an LLM verdict. WHEN the workflow JS reaches the deterministic Gate phase, it SHALL invoke the `[NEW] auto workflow gate` CLI subcommand, which runs build/test through the `[NEW] CommandRunner` seam in `[NEW] pkg/workflow/gate.go` and emits a structured `{verdict, verdict_source: "exit_code", build_exit, test_exit}` JSON that the JS reads to branch; the gate SHALL NOT depend on the existing `pkg/pipeline.PhaseBackend` for exit-code data. This `auto workflow gate` CLI is the JS→Go execution bridge.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: workflow JS의 deterministic Gate phase 도달 시 `auto workflow gate` 호출.
- Observability: dry-run render의 phase 순서 출력(manifest 파생)과 gate phase의 `verdict_source: exit_code` 선언, fake `CommandRunner`(build exit=1 주입)로 gate verdict=fail을 재현하는 replay 테스트(S8), 그리고 `auto workflow gate`가 build exit=1에 대해 `{verdict:"fail", verdict_source:"exit_code"}` JSON을 방출함을 CLI 오라클(S16)로 확인한다.

### REQ-012 — release hygiene 종단 phase
WHEN the release hygiene terminal phase runs, THEN THE SYSTEM SHALL block the run when generated surfaces (.claude/.codex/.gemini/.opencode/.autopus/orchestra) are staged without a corresponding source-of-truth change, enforce the pending commit message Lore format via `auto check --lore --message <msgfile>` (the pending message file, not the last commit), and enforce the 300-line source limit via `auto check --arch --staged`, before sync.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: 종단 phase의 drift gate가 SoT 변경 없는 generated 표면 staging을 탐지할 때.
- Observability: drift gate의 차단 종료 코드와 차단된 staged 경로 목록으로 확인한다.

## 생성 파일 상세

| 파일/심볼 | 역할 | 상태 |
|-----------|------|------|
| `content/workflows/route_a.md` | 사람 계약 정본(phase/gate 서술) | [NEW] |
| `content/workflows/route_a.schema.json` | phase/result 타입 정본(parity 비교 기준) | [NEW] |
| `templates/claude/workflows/route_a.workflow.js.tmpl` | manifest 파생 JS 어댑터 템플릿 | [NEW] generated |
| `.claude/workflows/route_a.workflow.js` | claude 설치 산출물(직접편집 금지) | [NEW] generated |
| `pkg/content/workflow_generate.go` | 정본에서 JS 파생 로직 | [NEW] |
| `pkg/content/workflow_parity.go` | phase-id/retry/budget/result-type parity 게이트 | [NEW] |
| `pkg/adapter/claude/claude_workflow.go` | claude 어댑터 workflow 쓰기 | [NEW] |
| `pkg/workflow/doctor.go` | capability 리포트 + 프로버 seam | [NEW] |
| `pkg/workflow/fallback.go` | fallback taxonomy 분류기 | [NEW] |
| `pkg/workflow/drift_gate.go` | generated-surface drift gate | [NEW] |
| `pkg/workflow/render.go` | dry-run 렌더 + prompt-manifest 해시 | [NEW] |
| `pkg/workflow/gate.go` | deterministic Gate — injectable `CommandRunner` seam(build/test exit-code) | [NEW] |
| `internal/cli/workflow.go` | `auto workflow doctor`/`render`/`gate` 커맨드(`gate`=JS→Go exit-code bridge) | [NEW] |
| `content/skills/harness-workflow.md` | opt-in 사용/doctor/fallback 스킬 문서(claude-scoped, 비-claude 미설치) | [NEW] |
| `pkg/content/generate.go` | 정본 파생 호출 추가 | 기존(확장) |
| `content/embed.go` / `templates/embed.go` | workflows glob 추가 | 기존(확장) |
| `pkg/adapter/claude/claude_generate.go` | `.claude/workflows` 쓰기 배선 | 기존(확장) |
| `templates/claude/commands/auto-router.md.tmpl` | `--workflow` 라우트 + 폴백 추가 | 기존(확장) |
| `internal/cli/root.go` | `newWorkflowCmd()` 등록 | 기존(확장) |

## Related SPECs

- Sibling 후보 (이 SPEC에서 구현하지 않음): SPEC-HARNESS-WORKFLOW-GATE-002 — 결정적 게이트 엔진(progress vector 서킷브레이커 + review 2-phase + verdict committee, T3). research.md의 `## Sibling SPEC Decision` 참조. verdict committee 스파이크(CR2)가 착수 전제이며, sibling의 sibling은 생성하지 않는다.
- 후속(자동 생성 금지): worktree fan-out(T5) → resume(T4) → Saga 2-tier(AP1) → budget rollover+reservation(AP2) → 기본 `/auto go` 승격(K13). research.md의 `## Evolution Ideas` 참조.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-001 manifest 정본 | T1 | S1 | INV-001 |
| REQ-002 JS 파생/임베드 | T2 | S1 | INV-001 |
| REQ-003 parity fail-closed | T3 | S2 | INV-002 |
| REQ-004 generated-surface | T4 | S1, S6 | INV-001, INV-005 |
| REQ-005 비-claude 회귀0 | T5 | S3 | INV-003 |
| REQ-006 doctor capability | T6 | S4, S12, S14 | INV-004 |
| REQ-007 라우트 fail-fast 폴백 | T8 | S5 | INV-004 |
| REQ-008 fallback taxonomy | T11 | S9 | INV-007 |
| REQ-009 Go 경계 | T7 | S10 | INV-008 |
| REQ-010 dry-run 생성기 | T6, T10 | S7, S11 | INV-006, INV-009 |
| REQ-011 4-phase 결정적 실행 | T1, T6, T7 | S7, S8, S15, S16 | INV-006 |
| REQ-012 release hygiene | T9 | S6, S13 | INV-005 |
