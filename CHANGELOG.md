# Changelog — autopus-adk

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added

- **Adaptive SPEC review context limit + Provider Health labeling** (2026-05-04, [SPEC-SPECREV-001](.autopus/specs/SPEC-SPECREV-001/spec.md), issue [#55](https://github.com/Insajin/autopus-adk/issues/55)): multi-provider spec review now scales the citation context budget per SPEC and surfaces provider infrastructure failures as a structured verdict label so operators can distinguish content concerns from timeouts.
  - `pkg/spec/context_limit.go` — new `AdaptiveContextLimit(citedFileCount, ceiling)` mapping (`0~2 → 500`, `3~5 → 1500`, `6+ → 3000`); honors optional `autopus.yaml` ceiling (REQ-CTX-1, REQ-CTX-4)
  - `pkg/spec/metadata.go` — new `ParseReviewContextOverride` reads optional `review_context_lines` SPEC frontmatter override; rejects values ≤0 or >10000 with explicit error (REQ-CTX-2, REQ-CTX-3)
  - `internal/cli/spec_review_context.go` — new `resolveSpecReviewContextLimit` orchestrates cited count → adaptive map → frontmatter override → ceiling cap, emitting `SPEC review context: cited=N applied=M [override=frontmatter] [ceiling=K]` to stderr
  - `pkg/spec/provider_health.go` — new `BuildProviderStatuses`, `RenderProviderHealthSection`, `DegradedLabel`, `ShouldLabelDegraded`; classifies orchestra responses into success/timeout/error and renders `## Provider Health` table (REQ-VERD-1, REQ-VERD-2). Provider Note column is sanitized (control chars stripped, length capped at 200) so committed review.md never embeds raw provider stderr
  - `pkg/spec/merge.go` — new `MergeVerdictsWithDenomMode` adds optional `excludeFailed` denom mode plus AC-VERD-1 fix (dropped providers without supermajority → REVISE not silent PASS); existing `MergeVerdicts` delegates with `excludeFailed=false`
  - `pkg/spec/review_persist.go` — `formatReviewMd` now renders `## Provider Health` after the verdict line and appends `(degraded — N/M providers responded)` when failure ratio ≥ 50% (REQ-VERD-2/4)
  - `pkg/config/schema_spec.go` — new `ExcludeFailedFromDenom bool` yaml field (default false, backward-compatible) (REQ-VERD-3)
  - `internal/cli/spec_review_loop.go` — wires orchestra responses into `BuildProviderStatuses` and switches to denom-mode merge
  - **Behavior change**: `MergeVerdicts` now treats any single REVISE vote as REVISE even when the supermajority math would otherwise pass (AC-VERD-BACKCOMPAT). Existing `TestMergeVerdictsSupermajorityPass` was renamed to `TestMergeVerdicts_AnyReviseWins` to reflect this. External tooling that grepped `**Verdict**: PASS` should be updated to handle the new optional `(degraded — N/M …)` suffix.
  - **Follow-up hardening (2026-05-04)**:
    - `pkg/spec/provider_health.go::sanitizeNote` now uses rune-aware truncation (200 runes + ellipsis) instead of byte slicing, so multi-byte UTF-8 in provider stderr never lands as malformed runes in committed `review.md`.
    - `pkg/spec/metadata.go` split into 3 files (`metadata.go`, `metadata_status.go`, `metadata_frontmatter.go`) — each ≤100 lines, fully out of the 200-line warning band.
    - `internal/cli/spec_review_loop.go` now skips ParseVerdict for failed providers (`TimedOut || ExitCode != 0 || Error != ""`). A failed provider's partial stdout containing `VERDICT: REJECT` no longer triggers the REJECT short-circuit (S-005 hardening).
    - `pkg/orchestra/output_parser.go::ParseReviewer` accepts `PASS | FAIL | N/A` checklist statuses (was `PASS | FAIL`).
- **Checklist Summary section in review.md** (2026-05-04, SPEC-SPECREV-001 follow-up): `formatReviewMd` now renders a `## Checklist Summary` section between `## Provider Health` and `## Findings` whenever `ReviewResult.ProviderStatuses` carries checklist outcomes. The section follows the same column-aligned table pattern as Provider Health.
  - Section structure: heading `## Checklist Summary`, columns `| ID | Status | Provider | Reason |`, terminal totals line `Total: N (PASS: P, FAIL: F, N/A: A)`.
  - `pkg/spec/types.go` — new `ChecklistStatusNA ChecklistStatus = "N/A"` constant; `ChecklistOutcome.Reason` is now required for FAIL **and** N/A (see `content/rules/spec-quality.md` § "N/A Status Guidance" for usage).
  - `pkg/spec/checklist_render.go` [NEW] — `CountChecklistStatuses` (per-status totals) and `RenderChecklistSection` (markdown table, reason sanitization via shared `sanitizeNote`).
  - `internal/cli/spec_review_output.go::printChecklistSummary` now prints `체크리스트 결과: N건 (PASS: P, FAIL: F, N/A: A)` — the N/A count is a new field. Tooling that grepped the previous 2-tuple format must be updated.
  - `internal/cli/spec_self_verify.go` `auto spec self-verify --status` flag now accepts `PASS | FAIL | N/A` (was `PASS | FAIL`); error string is `expected PASS, FAIL, or N/A`.
  - `pkg/spec/selfverify.go::AppendSelfVerifyEntry` accepts `N/A` and writes it verbatim to `.self-verify.log` JSONL entries.
  - **External grep contract**: tools that consume `review.md` should expect either `## Provider Health` immediately followed by `## Findings`, or with `## Checklist Summary` interposed when checklist data is present. Section order is: verdict → Provider Health → Checklist Summary → Findings → Provider Responses.

### Changed

- **Spec review claude provider defaults relaxed for stability** (2026-05-04, issue [#55](https://github.com/Insajin/autopus-adk/issues/55)): default claude orchestra entry now uses `--effort high` (was `max`) and a per-provider subprocess timeout of 480s, exceeding the 240s global timeout to prevent the 4-minute cutoff observed on opus reasoning during multi-provider spec review.
  - `pkg/config/defaults.go` — new `ClaudeOrchestraTimeoutSeconds = 480` constant; claude provider entry sets `Subprocess.Timeout` and switches `--effort` to `high`
  - `pkg/config/defaults_test.go` — regression coverage for claude provider timeout and effort defaults
  - Existing installs are not auto-migrated — run `auto update` or edit `autopus.yaml` to adopt the new defaults

## [v0.43.0] — 2026-05-01

### Changed

- **UX skills now include platform-neutral design-system reasoning** (2026-05-01): `frontend-skill` now performs a compact UX Intelligence pass before UI implementation, and `frontend-verify` / UX agents use the same matrix for visual verification across Claude, Codex, Gemini, and OpenCode surfaces.
  - `content/skills/{frontend-skill,frontend-verify}.md`, `content/agents/{frontend-specialist,ux-validator}.md` — design discovery matrix, UX Intelligence synthesis, viewport matrix, state/accessibility checks, and pattern/style mismatch detection
  - `templates/{codex,gemini}/**/{frontend-skill,frontend-verify,frontend-specialist,ux-validator}*` — regenerated Codex/Gemini surfaces from canonical content
  - `pkg/content/ux_skill_parity_test.go` — regression coverage that the UX Intelligence sections transform for Claude, Codex, Gemini, and OpenCode

- **DESIGN.md starter now participates in init/update** (2026-04-30): `auto init` creates a non-destructive starter `DESIGN.md`, and `auto update` backfills missing `design:` config plus the starter file for older harness installs.
  - `internal/cli/{init.go,update.go,design.go,update_preview.go}` — starter creation/preservation, update backfill, and `--plan` preview visibility
  - `pkg/config/loader.go` — top-level config key detection for safe migration decisions
  - `internal/cli/{init_test.go,update_test.go,update_preview_test.go}`, `pkg/config/defaults_design_test.go` — regression coverage for init, update, disabled design, and dry-run behavior

## [v0.42.1] — 2026-04-30

### Fixed

- **Orchestra degraded run diagnostics are now persisted** (2026-04-30): `auto orchestra brainstorm` and related successful-but-degraded runs now preserve structured failed-provider diagnostics in Markdown artifacts, terminal summaries, and sidecar JSON reports.
  - `internal/cli/{orchestra.go,orchestra_output.go,orchestra_failure_output.go}` — degraded success artifacts now include provider failure class, stderr/stdout previews, timeout provenance, remediation hints, and `degraded-*.json` sidecar reports
  - `pkg/orchestra/{runner.go,pipeline.go,pipeline_execute.go}` — partial provider failures now mark results as degraded and pass through shared failed-provider classification
  - `internal/cli/orchestra_timeout_test.go`, `pkg/orchestra/pipeline_execute_test.go` — regression coverage for degraded Markdown/JSON diagnostics and subprocess pipeline failure preservation

## [v0.42.0] — 2026-04-29

### Added

- **Semantic invariant acceptance gate hardening (SPEC-ACCGATE-002)** (2026-04-29): SPEC generation and implementation guidance now preserve original task semantic invariants through research inventory, oracle acceptance, behavioral tests, validator coverage, and observable subagent pipeline evidence.
  - `content/rules/spec-quality.md`, `content/agents/{spec-writer,tester,validator}.md` — `Q-COMP-05`, `Semantic Invariant Inventory`, oracle acceptance, and structural-only test rejection guidance
  - `content/skills/agent-pipeline.md`, `templates/{claude,codex,gemini}/**`, `pkg/adapter/opencode/opencode_test.go` — `subagent_dispatch_count`, dispatched-role evidence, degraded-mode blocker language, and cross-platform regression coverage
  - `templates/template_test.go` — source-of-truth template assertions for semantic-invariant and workflow-authenticity contracts

- **Project-local DESIGN.md context support (SPEC-DESIGN-001)** (2026-04-29): UI-sensitive ADK workflows can now discover safe local design context, inject compact `## Design Context` evidence into verify/review surfaces, and import external design references only through explicit sanitized generated artifacts.
  - `pkg/design/**`, `internal/cli/design.go` — safe path policy, source-of-truth frontmatter selection, deterministic summary trimming, UI file detection, public-HTTPS URL fetch guard, sanitizer, import artifact writer, and `auto design init/context/import`
  - `internal/cli/{verify.go,orchestra_helpers.go}`, `pkg/adapter/opencode/opencode_workflow_custom.go` — shared UI detector and design-context reporting/injection for `auto verify`, `auto orchestra review`, and OpenCode verify surfaces
  - `content/**`, `templates/**`, `README.md`, `docs/README.ko.md` — platform prompt parity and user docs for optional DESIGN.md, non-blocking skip semantics, read-only review checks, and generated-surface ownership

### Docs

- **Desktop runtime ownership boundary synced to desktop repo (SPEC-DESKTOP-014)** (2026-04-23): packaged `autopus-desktop-runtime` 의 source/build/release provenance 가 `autopus-desktop/runtime-helper/` 로 이동했음을 문서에 반영하고, ADK의 `connect` / `desktop` / `worker` 표면을 harness 또는 compatibility 범위로 재정의
  - `README.md`, `docs/README.ko.md` — desktop runtime source-of-truth 와 ADK compatibility boundary 안내 추가

## [v0.40.51] — 2026-04-25

### Changed

- **Plan workflow now requires complete feature coverage or sibling SPEC decomposition** (2026-04-25): `auto plan` 이 단일 스캐폴드 SPEC으로 멈추지 않도록 completion outcome, Feature Coverage Map, sibling SPEC 세트 분해 계약을 Codex/Claude/Gemini plan surface와 spec-writer/planner agent 지침에 반영
  - `content/agents/{planner.md,spec-writer.md}` — 사용자 요청의 최종 기능 결과를 먼저 정의하고 단일 SPEC 충분성 또는 sibling SPEC 세트를 판단하도록 기획/작성 절차 보강
  - `content/rules/spec-quality.md` — `Q-COMP-04` / `Q-COH-03` 품질 게이트를 추가해 스캐폴드-only SPEC과 vague future work를 self-verify/review 실패로 분류
  - `templates/{codex,gemini,claude}/...` — plan workflow prompt/router/skill surface에 primary/sibling SPEC 추출, Feature Coverage Map, 필수 follow-on SPEC 교차 참조 계약 추가

## [v0.40.45] — 2026-04-23

### Fixed

- **Orchestra multi-provider timeout semantics and config-backed provider resolution hardened** (2026-04-23): pane startup timeout과 실제 실행 timeout을 분리하고, `spec review --multi` 및 subprocess `orchestra run` 경로가 config/CLI timeout 우선순위를 일관되게 사용하도록 정리
  - `internal/cli/{orchestra.go,orchestra_brainstorm.go,orchestra_config.go,orchestra_file_cmds.go,orchestra_helpers.go,spec_review.go,spec_review_runtime.go,orchestra_run.go,orchestra_run_runtime.go}` — command timeout precedence, config-backed provider resolution, subprocess run timeout wiring 추가
  - `pkg/orchestra/{types.go,runner.go,pipeline.go,runner_timeout_config_test.go,pipeline_subprocess_test.go}` — `ExecutionTimeout` 분리, subprocess debater/judge request timeout 전달, 회귀 테스트 보강
  - `internal/cli/{orchestra_provider_timeout_test.go,spec_review_test.go,spec_review_result_ready_test.go,orchestra_run_test.go}` — CLI/config timeout precedence와 review/run wiring regression 추가

- **Debate prompt growth and pane round-2 readiness failures no longer silently drop providers** (2026-04-23): Round 2 rebuttal과 judge prompt에 공통 budget cap을 적용하고, prompt-ready가 되지 않은 pane은 명시적으로 skip/timed-out 처리해 긴 3-provider debate에서 Gemini 등 일부 provider가 조용히 탈락하는 경로를 줄임
  - `pkg/orchestra/{prompt_budget.go,debate.go,crosspolinate.go,interactive_debate_round.go}` — rebuttal/judge prompt budget cap, anonymized subprocess prompt cap, Round 2 prompt-ready guard 추가
  - `pkg/orchestra/{debate_test.go,crosspolinate_test.go,interactive_debate_test.go}` — long-output truncation, judge cap, prompt-ready skip 회귀 테스트 추가

## [v0.40.44] — 2026-04-23

### Added

- **Worker execution lane advertisement surfaced in runtime metadata** (2026-04-23): worker 런타임이 제공 가능한 execution lane 정보를 status/setup 경로에서 기계적으로 노출해 desktop / orchestration consumer가 lane-safe routing 가능 여부를 사전 판정할 수 있도록 확장
  - `pkg/worker/{loop.go,setup/status.go}`, `pkg/worker/a2a/{types.go,server_runtime.go}` — worker config/runtime payload에 `execution_lanes` metadata를 연결하고 server runtime surface에 반영
  - `pkg/worker/{setup/status_test.go,a2a/server_runtime_test.go}` — lane advertisement 회귀 테스트 추가

### Fixed

- **Provider capability fixtures and orchestra timeout expectations aligned with current runtime contracts** (2026-04-23): 최근 orchestration/runtime contract 변경 이후 흔들리던 테스트 기대값을 실제 provider capability / startup timeout 규칙에 맞춰 재정렬
  - `internal/cli/{doctor_json_platforms_test.go,orchestra_provider_timeout_test.go}` — installed CLI capability surface와 provider timeout 회귀 기대값 보정

- **Codex hooks empty categories now serialize as arrays instead of null** (2026-04-23): `.codex/hooks.json` 의 `SessionStart` / `Stop` 빈 카테고리가 `null`로 직렬화되어 Codex CLI가 `invalid type: null, expected a sequence`로 실패하던 문제를 복구
  - `pkg/adapter/codex/{codex_hooks.go,codex_internal_test.go}` — empty hook slice를 `[]`로 내보내는 marshal contract와 회귀 테스트 추가

## [v0.40.43] — 2026-04-23

### Added

- **Claude statusLine 선택 UX** (2026-04-23): 설치/업데이트 시 statusLine 동작을 명시적으로 선택할 수 있도록 CLI surface와 adapter wiring을 확장
  - `internal/cli/{init.go,statusline_mode.go,update.go,update_preview.go,update_preview_test.go,update_statusline_test.go}` — statusLine mode 선택, preview, 회귀 테스트 추가
  - `pkg/adapter/claude/{claude.go,claude_generate.go,claude_settings.go,claude_statusline.go,claude_hooks_test.go}` — 선택된 mode를 실제 Claude settings/statusline surface에 반영
  - `pkg/config/{runtime.go,schema.go}` — runtime 설정 스키마와 adapter 전달 경로 보강

### Fixed

- **기존 사용자 관리 Claude `statusLine` 설정 보존** (2026-04-23): workspace가 이미 사용자 정의 `statusLine`을 가지고 있을 때 하네스 업데이트가 이를 덮어쓰지 않고, Autopus statusline을 쓰는 경우에만 안전하게 갱신하도록 정리
  - `pkg/adapter/claude/{claude.go,claude_files.go,claude_prepare_files.go,claude_settings.go,claude_statusline.go}` — 기존 `statusLine` 감지/보존과 Autopus-managed 갱신 경계 추가
  - `pkg/adapter/claude/claude_hooks_test.go`, `internal/cli/update_statusline_test.go` — preserve/update 분기 회귀 테스트 추가

### Changed

- **Self-hosted generated/runtime artifact ignore 정리** (2026-04-23): self-hosting 과정에서 생기는 backup/context/docs/telemetry, split-mode `.opencode/skills`, demo/internal CLI 하위 `.autopus` 산출물이 작업트리를 오염시키지 않도록 ignore 규칙을 보강
  - `.gitignore` — self-host generated/runtime 경로를 release 이전 기본 ignore set에 포함

## [v0.40.42] — 2026-04-22

### Fixed

- **Spec review non-interactive verdict completion no longer waits for lingering provider processes** (2026-04-22): provider가 `VERDICT:`를 출력한 뒤 tail output 때문에 subprocess가 더 살아 있어도, review flow가 의미 있는 결과를 idle grace 이후 성공으로 수집하고 정리하도록 수정
  - `pkg/orchestra/{types.go,provider_runner.go,provider_result_ready.go,runner_timeout_test.go}` — semantic result-ready pattern/grace contract, non-interactive terminate monitor, regression test 추가
  - `internal/cli/{spec_review.go,spec_review_test.go}` — spec review provider에 `VERDICT:` completion hint를 주입하고 orchestration config 회귀 테스트를 보강

## [v0.40.41] — 2026-04-22

### Added

- **Skill registry + split surface compiler contract (SPEC-SKILLSURFACE-001)** (2026-04-22): 100+ skill / mixed Codex+OpenCode workspace 를 giant shared surface 없이 수용할 수 있도록 canonical catalog, split compiler mode, manifest diff/prune contract 를 도입
  - `pkg/content/{skill_catalog.go,skill_catalog_distribution.go,skill_catalog_policy.go,skill_catalog_test.go,skill_transformer_refs.go}` — canonical skill metadata, bundle/visibility/compile target, dependency extraction, `registered / compiled / visible` state 분리, registry-driven reference rewrite 추가
  - `pkg/config/{schema.go,schema_skill_compiler.go}` — `skills.compiler.mode`, explicit skill, OpenCode/Codex long-tail target validation 추가
  - `pkg/adapter/{manifest_diff.go,manifest_prune.go}`, `internal/cli/update_preview.go`, `internal/cli/update_preview_test.go` — emit/retain/prune preview, checksum diff, stale artifact prune contract 추가
  - `pkg/adapter/codex/*`, `pkg/adapter/opencode/*`, `README.md`, `docs/README.ko.md` — shared/core vs platform-local long-tail ownership split 과 사용자 문서를 split compiler model 에 맞게 정렬

## [v0.40.40] — 2026-04-21

### Added

- **Desktop sidecar contract metadata surfaced for supervision preflight (SPEC-DESKTOP-005)** (2026-04-21): desktop가 retained ADK source of truth를 strict parsing으로 소비할 수 있도록 runtime contract / sidecar protocol metadata를 worker status/session과 shared contract package에 고정
  - `pkg/worker/{setup/status.go,setup/desktop_session.go,sidecarcontract/contract.go}` — `runtime_contract_*`, `sidecar_protocol_*` metadata를 machine-readable bootstrap/session surface에 추가
  - `pkg/worker/host/sidecar.go` — same contract metadata를 sidecar runtime stream에 맞춰 정렬

### Changed

- **Desktop supervision approval correlation and launch parity (SPEC-DESKTOP-005)** (2026-04-21): `auto worker sidecar` 가 desktop launch nonce 플래그를 수용하고, approval request/response 경로가 `approval_id` / `trace_id` correlation metadata를 A2A → worker loop → sidecar NDJSON까지 유지하도록 정리
  - `internal/cli/worker_sidecar.go` — `--desktop-launch-nonce` 플래그를 sidecar entrypoint에 추가해 desktop supervision launch command parity를 맞춤
  - `pkg/worker/a2a/{types.go,server_approval.go,server_approval_test.go}` — approval payload/request-response에 correlation metadata를 추가하고 A2A round-trip 회귀 테스트를 보강
  - `pkg/worker/{loop.go,loop_runtime.go,loop_task.go,loop_approval_state.go,loop_approval_test.go,host_observer.go}` — pending approval state를 task별로 보존하고 response/resolution/task cleanup 시 correlation metadata를 유지
  - `pkg/worker/host/{sidecar.go,resolve_test.go}` — sidecar NDJSON approval payload에 `approval_id` / `trace_id`를 노출하고 unknown host event를 explicit degraded signal로 처리

### Fixed

- **Codex auto skill duplicate surface cleanup** (2026-04-21): generated plugin/local skill surface가 동시에 남을 때 중복 라우팅 흔적과 README drift가 발생하던 문제를 정리
  - `pkg/adapter/codex/{codex.go,codex_standard_skills.go,codex_surface_cleanup.go,codex_surface_test.go,codex_update_test.go}` — duplicate skill cleanup 경로와 회귀 테스트를 추가
  - `pkg/adapter/integration_test.go`, `README.md`, `docs/README.ko.md` — surface cleanup 동작과 사용자 문서를 현재 Codex contract에 맞춤

### Docs

- **SPEC-SETUP-003 planning/status sync** (2026-04-21): preview-first setup/connect truth-sync 이후 SPEC 문서를 구현 상태 기준으로 갱신
  - `.autopus/specs/SPEC-SETUP-003/{spec,plan,acceptance}.md` — 구현/검증 상태와 follow-up 범위를 실제 완료 기준에 맞춰 정리

## [v0.40.39] — 2026-04-21

### Added

- **Preview-first bootstrap planning and connect truth-sync (SPEC-SETUP-003)** (2026-04-21): `auto update` 와 `auto setup generate/update` 가 no-write preview를 먼저 계산하고, `auto connect` 는 deterministic verify surface와 실제 구현 기준 안내 문구를 제공하도록 정리
  - `internal/cli/{setup.go,preview_output.go,setup_preview.go,setup_preview_test.go,update.go,update_preview.go,update_config_preview.go,update_preview_test.go}` — `--plan`/`--preview`/`--dry-run` preview 출력, tracked/generated/runtime/config 분류, no-write regression test 추가
  - `pkg/config/loader.go`, `pkg/setup/{engine.go,engine_docs.go,meta.go,scenarios.go,sigmap_integration.go,types.go,change_plan.go,change_apply.go,change_plan_test.go,workspace_hints.go,sigmap_helpers_test.go}` — reusable change-plan 모델, stale preview revalidation, repo-aware workspace hint, preview/apply shared helpers 추가
  - `internal/cli/{connect.go,connect_status.go,connect_truth_sync_test.go}`, `README.md`, `docs/README.ko.md` — `auto connect status` surface와 onboarding wording truth-sync, README/help drift regression test 추가

- **Stable machine-readable CLI JSON envelopes (SPEC-CLIJSON-001)** (2026-04-21): phase-1 상태/진단 명령과 기존 JSON surface를 공통 envelope로 정렬해 CI, desktop, agent chaining이 text scraping 없이 재사용할 수 있도록 정리
  - `internal/cli/{output_json.go,doctor_json.go,doctor_json_platforms.go,doctor_json_checks.go,status_json.go,setup_json.go,telemetry_json.go,test_json.go,worker_status_json.go}` — shared envelope writer, redaction/home-path masking, command별 payload/check helper 추가
  - `internal/cli/{doctor.go,status.go,setup.go,telemetry.go,permission.go,test.go,worker_commands.go,root.go}` — `--json`/`--format json` rollout, warn/error payload contract, fatal JSON path cleanup 반영
  - `pkg/connect/headless_event.go`, `internal/cli/json_contract_test.go` — `connect --headless` NDJSON compatibility metadata와 contract/redaction/fatal-path regression test 추가

- **Multi-repo workspace detection and cross-repo setup rendering (SPEC-SETUP-002)** (2026-04-21): `auto setup` / `auto arch` 가 root+nested repo topology를 1급 모델로 인식하고 repo boundary/workflow/scenario 문서를 생성하도록 확장
  - `pkg/setup/{multirepo.go,multirepo_deps.go,multirepo_types.go,multirepo_render.go,scanner.go,types.go}` — `MultiRepoInfo` 모델, immediate-child repo discovery, Go/NPM cross-repo dependency mapping, aggregate scan wiring 추가
  - `pkg/setup/{renderer_arch.go,renderer_docs.go,scenarios.go}` — Workspace / Development Workflow / Repository Boundaries 섹션과 path-aware language-specific cross-repo scenario 생성 추가
  - `pkg/setup/{multirepo_test.go,multirepo_render_test.go,multirepo_scenarios_test.go}` — topology, rendering, scenario synthesis acceptance 회귀 테스트 추가

- **Desktop bootstrap session surface for the approval-only shell (SPEC-DESKTOP-004)** (2026-04-21): desktop handoff/session restore가 ADK source of truth를 재사용하도록 `auto worker session` 과 status readiness contract를 추가
  - `internal/cli/{worker_commands.go,worker_session.go}` — `worker session` command 등록, desktop-oriented machine-readable help/command boundary 정리
  - `pkg/worker/setup/{status.go,desktop_session.go}` — `credential_backend`, `secure_storage_ready`, `desktop_session_ready` 를 `worker status --json` 에 노출하고 fail-closed desktop session payload 구현
  - `pkg/worker/setup/desktop_session_test.go` — desktop bootstrap readiness/reason contract 회귀 테스트 추가

- **Orchestra reliability receipts, failure bundles, and run correlation (SPEC-ORCH-020)** (2026-04-21): pane/hook/detach orchestration에 provider preflight, prompt transport, collection receipt와 compact failure bundle contract를 추가
  - `pkg/orchestra/reliability_{receipt,preflight,bundle}.go`, `pkg/orchestra/{types.go,detach.go,job.go}` — schema v1, `run_id`, fallback mode, sanitized artifact, runtime artifact root/retention wiring 추가
  - `pkg/orchestra/{interactive_debate.go,interactive_debate_helpers.go,interactive_debate_round.go,interactive_collect.go}` — hook timeout structured event, partial collection receipt, degraded summary, remediation hint 연결
  - `internal/cli/{orchestra.go,orchestra_output.go}` — degraded 상태, run id, artifact dir를 CLI 결과물에 표면화
  - `pkg/orchestra/reliability_{core,collection}_test.go` — secret redaction, preflight receipt, retention, timeout bundle 회귀 테스트 추가

### Fixed

- **Worker status/session credential source mismatch** (2026-04-21): secure storage backend와 auth validity 판정이 command마다 달라질 수 있던 문제를 단일 credential snapshot 경로로 정리
  - `pkg/worker/setup/{credential_snapshot.go,credentials_store.go}` — keychain/encrypted/plaintext credential payload를 하나의 snapshot loader로 통합
  - `pkg/worker/setup/{auth_test.go,status_coverage_test.go,desktop_session_test.go}` — status/session이 같은 credential backend와 readiness를 반환하는지 회귀 검증 추가

- **pkg/orchestra full-suite timeout regression** (2026-04-21): reliability work 이후에도 `go test -timeout 120s ./pkg/orchestra`가 다시 통과하도록 interactive polling/backoff와 fixture sequencing을 결정적으로 정리
  - `pkg/orchestra/{completion_poll.go,interactive.go,interactive_collect.go,interactive_surface.go,surface_manager.go,interactive_debate_round.go}` — polling interval, retry/backoff, submit/empty-output wait를 짧고 결정적으로 조정
  - `pkg/orchestra/{pane_mock_test.go,interactive_pane_debate_test.go,interactive_surface_test.go,interactive_surface_round_test.go,interactive_edge_test.go,surface_manager_test.go,warm_pool_test.go,cc21_monitor_test.go}` — pane-aware mock sequencing과 stale/idle recovery fixture를 정리하고 runtime expectation을 현재 detector contract에 맞춤

## [v0.40.38] — 2026-04-21

### Added

- **Worker shared host assembly and machine-readable sidecar entrypoint (SPEC-DESKTOP-003)** (2026-04-20): desktop supervision이 launch logic를 fork하지 않도록 shared host runtime과 NDJSON sidecar surface를 추가
  - `internal/cli/worker_sidecar.go`, `internal/cli/worker_commands.go` — `auto worker sidecar` command 등록 및 machine-oriented help surface 추가
  - `pkg/worker/host/{errors.go,resolve.go,runtime.go,sidecar.go,resolve_test.go}` — typed host input, resolved runtime config, structured host errors, sidecar protocol/event contract 구현
  - `pkg/worker/host_observer.go`, `pkg/worker/{loop.go,loop_runtime.go,loop_task.go,loop_subprocess.go,loop_lifecycle.go,loop_approval_test.go}` — runtime/task/approval observer bridge와 degraded/progress/completion signal wiring 추가

### Changed

- **Legacy worker start path now reuses the shared host runtime** (2026-04-20): `auto worker start`가 duplicated assembly를 버리고 compatibility shim으로 축소되고, explicit credentials path override가 desktop sidecar용 실제 auth source로 동작
  - `internal/cli/worker_start.go`, `internal/cli/worker_start_test.go` — start command를 shared runtime shim으로 정리하고 기존 local resolver 테스트를 host package로 이동
  - `pkg/worker/setup/{apikey.go,status.go,credentials_override.go,apikey_coverage_test.go}` — `LoadAPIKeyFromPath`, `LoadAuthTokenFromPath`, path-backed CredentialStore, custom credentials path coverage 추가

### Fixed

- **Worker setup device auth now honors deadline boundaries** (2026-04-21): Windows에서 `auto worker setup` 승인 직후 polling deadline 경계에 걸리면 stale token 요청이 한 번 더 나가 backend의 `expired_token`을 그대로 surfacing하던 문제를 수정
  - `pkg/worker/setup/auth.go` — poll interval 대기를 context-aware `select`로 바꾸고 token exchange HTTP request에 context를 전달해 deadline 이후 추가 poll과 hanging request를 차단
  - `pkg/worker/setup/auth_device_test.go`, `pkg/worker/setup/auth_deadline_test.go` — 새 context-aware exchange signature 반영 및 deadline 경계 회귀 테스트 2건 추가

## [v0.40.37] — 2026-04-19

### Changed

- **Residual golangci-lint cleanup sweep across ADK** (2026-04-19): 남아 있던 `staticcheck`/`ineffassign`/test-style 경고를 일괄 정리해 현재 `golangci-lint run --max-issues-per-linter=0 --max-same-issues=0` 기준 0 issue 상태로 수렴
  - `.golangci.yml`, `internal/cli/**`, `pkg/orchestra/**`, `pkg/setup/**`, `pkg/worker/**` — 빈 에러 브랜치, 비효율 할당, 루프/append 패턴, 테스트 fixture/헬퍼 표현을 정리
  - `pkg/adapter/opencode/opencode_router_contract.go`, `pkg/content/agent_transformer_condense.go`, `internal/cli/issue_auto.go` — 더 이상 쓰이지 않는 보조 경로와 dead code를 제거
  - 광범위한 테스트/헬퍼 파일에서 lint 친화적 표현으로 정렬해 release gate를 통과하도록 회귀 범위를 동기화

## [v0.40.36] — 2026-04-19

### Fixed

- **Install bootstrap now separates install from init** (2026-04-19): installer가 `auto init`/`auto update`를 자동 실행하지 않고, 필수 도구만 점검한 뒤 `auto init`, `auto update --self`, `auto update`의 역할을 명시적으로 안내하도록 정리
  - `install.sh`, `install.ps1` — post-install 단계에서 required dependency만 자동 설치하고, 자동 project init/update 분기 제거
  - `internal/cli/doctor.go`, `internal/cli/doctor_fix.go` — `--required-only` 플래그와 required dependency filter 추가
  - `pkg/detect/detect.go` — `gh`를 필수 도구로 승격하고 Gemini CLI npm 패키지를 `@google/gemini-cli`로 정정
  - `README.md`, `docs/README.ko.md`, `internal/cli/doctor_fix_runtime_test.go`, `internal/cli/doctor_fix_test.go`, `pkg/detect/fullmode_deps_test.go` — 설치 가이드/회귀 테스트 동기화 및 테스트 파일 분할로 300-line limit 유지

- **E2E scenario runner backend submodule path correction** (2026-04-19): Backend build 시나리오가 `Autopus/`를 cwd로 잡아 존재하지 않는 `cmd/server` 경로를 참조하던 문제를 실제 backend 소스 경로인 `Autopus/backend/`로 정렬
  - `pkg/e2e/build.go`, `pkg/e2e/build_test.go` — default submodule map을 canary H2/H3 build cwd와 일치시키고 회귀 테스트 추가

- **Permission detection tests now use injected process-tree stubs** (2026-04-19): `--dangerously-skip-permissions`가 걸린 세션에서 `pkg/detect` 테스트가 실제 부모 프로세스 트리에 오염되던 문제를 제거
  - `pkg/detect/permission.go`, `pkg/detect/permission_test.go` — `checkProcessTreeFn` 주입 지점과 결정적 stub helper 추가

- **CC21 monitor runtime flake removed via Claude version injection hook** (2026-04-19): `claude --version` subprocess timeout으로 인해 `TestResolveCC21MonitorRuntime_Enabled`가 간헐적으로 실패하던 문제를 테스트 전용 version injector로 제거
  - `pkg/platform/claude.go`, `internal/cli/orchestra_cc21_test.go` — `claudeVersionFn`/`SetClaudeVersionForTest` 추가 및 monitor runtime 회귀 테스트 보강

## [v0.40.35] — 2026-04-19

### Fixed

- **Release workflow bootstrap ordering** (2026-04-19): `goreleaser-action@v7`가 `cosign`이 PATH에 있을 때 GoReleaser 다운로드 자체의 sigstore bundle을 추가 검증하는데, upstream bundle 검증 실패로 `v0.40.34` release workflow가 즉시 중단되던 문제를 우회
  - `.github/workflows/release.yaml` — action을 `install-only`로 먼저 실행해 checksum 검증만 수행하고, 이후 `cosign` 설치와 `goreleaser release --clean` 직접 실행으로 실제 checksum signing 단계만 유지하도록 순서 조정

## [v0.40.34] — 2026-04-19

### Added

- **Test Profile 기반 시나리오 요구조건 스킵** (2026-04-19): `auto test run`에 `--profile` capability 집합을 도입해 시나리오의 `Requires` 조건이 충족되지 않으면 FAIL 대신 SKIP으로 처리
  - `internal/cli/test.go`, `internal/cli/test_profile_test.go` — `--profile` 플래그, SKIP 집계, JSON 출력 회귀 테스트 추가
  - `pkg/config/test_profiles.go`, `pkg/config/test_profiles_test.go`, `pkg/config/schema.go` — profile별 capability 기본값 및 `autopus.yaml` 확장
  - `pkg/e2e/requires.go`, `pkg/e2e/scenario.go`, `pkg/e2e/scenario_requires_test.go` — `Requires` 파싱 및 capability mismatch 계산 로직 추가
  - `templates/shared/scenarios-*.md.tmpl` — 시나리오 템플릿에 `Requires` 필드 추가

### Fixed

- **SPEC review finding status breakdown summary** (2026-04-19): `auto spec review` 최종 요약이 단순 unique count 대신 `open/resolved/out_of_scope` 상태별 집계를 함께 출력하도록 개선해 운영자가 `review-findings.json`을 별도로 집계하지 않아도 열린 finding 수를 바로 확인 가능
  - `pkg/spec/findings_summary.go`, `pkg/spec/findings_test.go` — `ReviewFinding` slice를 상태별로 집계하는 `SummarizeFindings` / `FindingsSummary.Format` 로직과 회귀 테스트 추가
  - `internal/cli/spec_review.go` — 최종 CLI 요약을 status breakdown 표면으로 교체

- **Pipeline worktree remove canonical path fallback** (2026-04-19): macOS의 `/tmp` → `/private/tmp`, `/var` → `/private/var` symlink 환경에서 `git worktree remove`가 symlink path를 실제 worktree로 인식하지 못해 release gate의 `pkg/pipeline` 테스트가 실패하던 문제를 수정
  - `pkg/pipeline/worktree.go` — remove 시 원본 path와 canonical path를 순차 재시도하고, 실제 git worktree가 아닌 fallback 디렉터리는 안전하게 `os.RemoveAll`로 정리하도록 보강
  - `pkg/pipeline/worktree_internal_test.go` — symlink alias로 생성한 실제 worktree를 remove 하는 회귀 테스트 추가

- **SPEC 리뷰 체크리스트 런타임 주입 및 self-verify 기록 경로 복구 (SPEC-SPECWR-002)** (2026-04-19): `auto spec review`가 `content/rules/spec-quality.md`를 실제 런타임 프롬프트에 주입하고, `CHECKLIST:` 응답을 구조화 파싱하며, `auto spec self-verify`로 결정적 JSONL 기록을 남길 수 있도록 동기화.
  - `pkg/spec/checklist.go`, `pkg/spec/prompt.go` — embed 우선 + 디스크 fallback 체크리스트 로더, `## Quality Checklist` 주입, checklist response examples 추가
  - `pkg/spec/types.go`, `pkg/spec/reviewer.go`, `internal/cli/spec_review_loop.go`, `internal/cli/spec_review.go` — `ChecklistOutcome` 타입, `CHECKLIST:` 파싱, provider outcome 집계, 최종 요약 출력 연결
  - `pkg/spec/selfverify.go`, `internal/cli/spec.go`, `internal/cli/spec_self_verify.go`, `.gitignore` — `auto spec self-verify` 서브커맨드, 100라인 retention, `.self-verify.log` ignore 규칙 추가
  - `pkg/spec/checklist_test.go`, `pkg/spec/reviewer_checklist_test.go`, `pkg/spec/selfverify_test.go`, `internal/cli/spec_review_checklist_test.go`, `internal/cli/spec_self_verify_test.go` — checklist injection/parser/CLI/self-verify 회귀 테스트 추가

- **SPEC 리뷰 수렴성 재구축 (SPEC-REVFIX-001)** (2026-04-19): `auto spec review --multi`가 대부분의 SPEC에서 PASS에 도달하지 못하고 REVISE 루프를 소진한 뒤 circuit breaker로 종료되던 7개 복합 결함 제거.
  - **REQ-01 Supermajority verdict**: `MergeVerdicts`가 `spec.review_gate.verdict_threshold`(기본 0.67) 기준 supermajority를 적용. 1 REJECT 단독 override는 유지(security gate). `pkg/spec/reviewer.go`
  - **REQ-02 Revision 루프 내 재로드**: `runSpecReview`가 iteration마다 `spec.Load(specDir)` 재호출. 외부 수정이 다음 round에 반영됨. `internal/cli/spec_review_loop.go`
  - **REQ-03 다중 문서 주입**: `BuildReviewPrompt`가 plan.md / research.md / acceptance.md 본문을 별도 섹션으로 주입. `doc_context_max_lines`(기본 200)로 trim. `pkg/spec/prompt.go`
  - **REQ-04 Verdict 판정 기준 명문화**: 프롬프트에 `critical==0 && security==0 && major<=2 → PASS` 규칙 포함. `pass_criteria` override 지원.
  - **REQ-05 FINDING 포맷 강제 + empty RawContent guard**: structured FINDING few-shot(positive 2 + negative 1), `doc.RawContent == ""` 시 early error.
  - **REQ-06 DeduplicateFindings / MergeSupermajority 프로덕션 통합**: REVCONV-001이 구현했으나 호출되지 않던 dead code를 `runSpecReview` 경로에 연결. critical/security는 supermajority 우회.
  - **REQ-07 Finding ID 전역 유니크**: `parseDiscoverFindings`가 ID 비어있게 두고 `DeduplicateFindings`가 global `F-001..` 재발급. `ApplyScopeLock` 오동작 해결.
  - 신규: `pkg/spec/merge.go`, `pkg/config/schema_spec.go`, `internal/cli/spec_review_loop.go`, `pkg/spec/prompt_test.go`, `pkg/spec/reviewer_supermajority_test.go`, `internal/cli/spec_review_scaffold_test.go`
  - `autopus.yaml` 샘플에 `verdict_threshold`, `pass_criteria`, `doc_context_max_lines` 주석 예시 추가

### Changed

- **Claude Code 2.1 CC21 경로 연결 및 precedence 정렬 (SPEC-CC21-001)** (2026-04-19): effort frontmatter, TaskCreated hook, initial prompt 검사, monitor 기반 완료 감지를 source-of-truth와 CLI/runtime 경로에 연결
  - `internal/cli/effort*.go`, `internal/cli/check_initial_prompt*.go`, `internal/cli/orchestra_cc21.go`, `internal/cli/check_cc21.go`, `internal/cli/cc21_runtime.go` — CC21 전역 플래그, runtime precedence, check 명령, orchestra wiring 추가
  - `pkg/orchestra/cc21_monitor.go`, `pkg/platform/claude.go`, `pkg/platform/claude_test*.go` — Claude Code 2.1 capability 감지와 monitor contract 연결
  - `content/hooks/task-created-validate.sh`, `content/hooks/README.md`, `pkg/content/hooks.go`, `pkg/adapter/claude/claude_task_created_test.go` — TaskCreated generated default와 runtime override precedence 정렬
  - `content/skills/monitor-patterns.md`, `content/embed.go`, `content/skills/adaptive-quality.md`, `content/skills/idea.md`, `content/skills/agent-pipeline.md` — CC21 monitor/effort 규칙과 문서 표면 동기화
  - `pkg/adapter/claude/claude_generate.go`, `pkg/adapter/claude/claude_prepare_files.go`, `pkg/adapter/claude/claude_update.go` — Claude adapter 파일 생성/업데이트 경로를 300줄 제한에 맞게 분리 정리

- **Claude deferred-tools 선로딩 규칙 추가** (2026-04-18): Claude Code의 지연 로드 도구(`AskUserQuestion`, `TaskCreate`, `TeamCreate` 등)가 스키마 미로드 상태로 호출될 때 생기던 평문 downgrade / validation error를 줄이기 위해 전역 규칙을 추가
  - `content/rules/deferred-tools.md` — `/auto triage`, Gate 1 승인, `--team` 진입 시 `ToolSearch`로 스키마를 먼저 로드하도록 trigger point 규칙 추가

- **Claude Code Agent Teams + mode 파라미터 동기화** (2026-04-18): Agent Teams 공식 스펙(https://code.claude.com/docs/en/agent-teams)을 반영하고, Agent() 호출 파라미터 이름을 `permissionMode` → `mode` 로 통일. 플랫폼별 `--team` 플래그 동작 명시.
  - `content/skills/agent-pipeline.md`, `content/skills/worktree-isolation.md` — 본문 `Agent(... permissionMode=)` 10건 → `mode=`
  - `templates/codex/skills/agent-pipeline.md.tmpl`, `templates/codex/skills/worktree-isolation.md.tmpl`, `templates/gemini/skills/agent-pipeline/SKILL.md.tmpl`, `templates/gemini/skills/worktree-isolation/SKILL.md.tmpl` — 동일 변경 (각 4-6건)
  - `content/skills/agent-teams.md` — Prerequisites 섹션(v2.1.32+ 버전 요구) + Team Constraints 섹션(nested 금지, leader-only cleanup, 3-5명 권장, 영속 경로) 신설. Team Creation Pattern의 `Teammate()` → `Agent(team_name=..., name=...)` 공식 문법으로 교정
  - `templates/claude/commands/auto-router.md.tmpl` — Route B preflight 2단계(버전 + 환경변수) 추가, 에러 메시지 개선
  - `templates/codex/skills/agent-teams.md.tmpl` — 상단 ⚠️ Platform Note: Claude Code 전용 명시, Codex는 `spawn_agent` fallback
  - `templates/gemini/commands/auto-router.md.tmpl`, `templates/gemini/skills/agent-teams/SKILL.md.tmpl` — Platform Note 배너 + Route B 비활성화 + `--team` 경고 후 Route A fallback, 스테일 "Gemini CLI Agent Teams" 참조 제거
  - **Subagent frontmatter `permissionMode:` 필드는 공식 스펙이므로 그대로 유지** (Agent() 호출 파라미터와 별개 레이어)

### Docs

- **spec-writer 자체 품질 체크리스트 도입 문서 동기화 (SPEC-SPECWR-001)** (2026-04-19): `content/rules/spec-quality.md` 신규 체크리스트, `content/skills/spec-review.md`의 pre-review self-check, `content/agents/spec-writer.md`의 자체 검증 루프를 실제 산출물 기준으로 정렬하고 SPEC 문서를 completed 상태로 동기화
  - `content/rules/spec-quality.md`, `content/skills/spec-review.md`, `content/agents/spec-writer.md` — 체크리스트, pre-review self-check, 자체 검증 루프 source-of-truth 반영
  - `.autopus/specs/SPEC-SPECWR-001/{spec,plan,acceptance,research}.md` — completed 상태 동기화, validator/review 기준 정렬
  - 후속 보강: `research.md`의 `Self-Verify Summary` 관측 지점과 구조화된 `Open Issues` 스키마를 문서 규약으로 추가해 reviewer가 retry 경로를 문서 안에서 추적 가능하도록 보강

- **`/auto go --team` Route B 실행 절차 공백 수정** (2026-04-18): `--team` 플래그로 실행해도 core 4명 중 lead 1명만 spawn되어 멀티에이전트 협업이 작동하지 않던 문제를 수정. 실측 증거: `~/.claude/teams/spec-waitux-001/config.json` 의 members 배열에 team-lead 1명만 등록. 근본 원인: Route B 문서가 TeamCreate 호출 주체·시점, ToolSearch 선행 의존성, 4명 병렬 spawn 규칙, members 검증 게이트, phase별 SendMessage 디스패치를 명시하지 않음
  - `templates/claude/commands/auto-router.md.tmpl` — Route B에 **Team Orchestration Procedure (B1~B5)** 신설: ToolSearch → TeamCreate → 4명 병렬 Agent() spawn → `.members | length == 4` HARD GATE → SendMessage 오케스트레이션
  - `content/skills/agent-teams.md` — Lead 책임에서 "Creates the team" 문구 제거(teammates MUST NOT call TeamCreate), Team Creation Pattern을 top-level session 주체 + ToolSearch 선행 + verification gate 구조로 재작성
  - `templates/codex/skills/agent-teams.md.tmpl`, `templates/gemini/skills/agent-teams/SKILL.md.tmpl` — 플랫폼 비지원 명시를 유지한 채 Lead 문구와 코드 주석 정정

- **Route B 실측 smoke-test 기반 절차 정정** (2026-04-18): 1차 패치의 Route B 절차를 실제 `TeamCreate` + 3명 `Agent()` 호출로 smoke-test 한 결과, 공식 Claude Code Agent Teams API와 어긋난 4가지 세부 사항을 확인하고 정정. 실측 증거: `~/.claude/teams/team-probe-001/config.json` members=4 (team-lead + builder-1 + tester + guardian) 정상 생성 후 `SendMessage({type:"shutdown_request"})` ×3 + `TeamDelete()` 사이클 E2E 통과
  - **TeamCreate 파라미터명 정정**: `TeamCreate(name=...)` → `TeamCreate(team_name=..., agent_type="planner")` — 공식 스키마 파라미터는 `team_name` (기존 `name`은 오타)
  - **Lead 자동 등록 명시**: `TeamCreate`는 호출 시점에 메인 세션을 자동으로 `name: "team-lead"`, `agentType: <agent_type>`로 등록한다. Step B3은 **lead 제외 3명만 spawn**(builder-1 / tester / guardian)으로 축소 — lead Agent() 중복 spawn 방지
  - **SendMessage 주소 교정**: phase 오케스트레이션 매핑 표의 `to="lead"` → `to="team-lead"`. Phase 1 Planning은 메인 세션이 직접 담당하므로 SendMessage 불필요
  - **Step B6: Teardown 신설**: 구조화된 `{type:"shutdown_request"}`는 **per-teammate** 발송 필수 (broadcast `to:"*"`는 plain text 전용, structured payload rejected). `TeamDelete()`는 active members 남아 있으면 실패하므로 shutdown_request 후 `sleep 8` 대기 필수
  - 수정 파일: `templates/claude/commands/auto-router.md.tmpl`, `content/skills/agent-teams.md`, `templates/codex/skills/agent-teams.md.tmpl`, `templates/gemini/skills/agent-teams/SKILL.md.tmpl`

### Chore

- **SPEC review 산출물 ignore 정리** (2026-04-19): review 실행이 생성하는 `review.md`, `review-findings.json`을 runtime artifact로 간주하고 git 추적 대상에서 제외
  - `.gitignore` — `**/.autopus/specs/**/review.md`, `**/.autopus/specs/**/review-findings.json` 패턴 추가

## [v0.40.32] — 2026-04-17

### Changed

- **Claude Opus 4.7 Alignment**: 2026-04-16 Anthropic Opus 4.7 공식 출시에 맞춰 하네스 모델 ID/가격을 전면 동기화. 기존 cost estimator가 Opus 가격을 $15/$75로 과대 산정하던 오류도 함께 보정
  - `pkg/cost/pricing.go` — 모델 ID를 `claude-opus-4-7` / `claude-sonnet-4-6` / `claude-haiku-4-5`로 버전 명시, Opus 입력/출력 가격을 공식가 $5/$25로, Haiku를 $1/$5로 정정 (이전 $15/$75, $0.80/$4)
  - `pkg/cost/pricing_test.go`, `pkg/cost/estimator_test.go`, `pkg/cost/estimator_extra_test.go` — 모델명 assertion과 실제 달러 기대값(ultra/executor 4k 토큰 시 $0.04 등) 재계산
  - `pkg/worker/routing/config.go`, `pkg/worker/routing/{config,router}_test.go`, `pkg/worker/routing_integration_test.go` — Complex tier를 `claude-opus-4-7`로 승격
  - `pkg/config/defaults.go`, `autopus.yaml`, `configs/autopus.yaml` — Full 모드 기본 router tier `premium` / `ultra` 를 Opus 4.7로 갱신
  - `demo/simulate-claude.sh` — welcome banner 모델 표기를 `claude-opus-4-7`로 교체

### Docs

- **using-autopus Router Tier 예시 동기화**: `auto init` 이 생성하는 `configs/autopus.yaml` 기본값이 이미 `claude-opus-4-7` / `claude-sonnet-4-6` 버전 명시형인데, 가이드 문서의 예시 블록은 unversioned alias 로 남아 있어 사용자 혼란을 유발하던 불일치 제거
  - `content/skills/using-autopus.md`, `templates/codex/skills/using-autopus.md.tmpl`, `templates/gemini/skills/using-autopus/SKILL.md.tmpl` — router.tiers 예시 블록 통일

## [v0.40.29] — 2026-04-16

### Fixed

- **Codex Auto-Go Completion Handoff Gate Recovery**: Codex `@auto go ... --auto --loop` 가 구현/검증 요약만 남기고 종료하지 않도록 completion handoff contract를 source-of-truth와 회귀 테스트에 고정
  - `templates/codex/skills/auto-go.md.tmpl`, `templates/codex/prompts/auto-go.md.tmpl` — `Completion Handoff Gates` 와 `Final Output Contract` 를 추가해 `current_gate`, `phase_4_review_verdict`, `next_required_step`, `next_command`, `auto_progression_state` 가 비면 success-style completion summary로 닫지 못하게 보강
  - `pkg/adapter/codex/codex_surface_test.go`, `pkg/adapter/codex/codex_prompts_test.go` — generated Codex skill/prompt surface가 workflow lifecycle 뒤에 next-step handoff contract를 유지하는지 회귀 테스트 추가

## [v0.40.28] — 2026-04-16

### Fixed

- **Legacy SPEC Status Sync Recovery**: `auto spec review` 가 PASS 후 `approved` 상태를 새 scaffold SPEC뿐 아니라 기존 legacy SPEC 형식에도 안전하게 반영하도록 메타데이터 파서와 상태 갱신 경로를 복구
  - `pkg/spec/metadata.go` — `# SPEC: ...` + `**SPEC-ID**:` / `**Status**:` legacy metadata를 읽도록 보강하고, frontmatter 탐지를 문서 상단으로 제한해 본문 `---` 구분선을 잘못된 frontmatter로 오인하지 않도록 수정
  - `pkg/spec/metadata_test.go` — legacy ID/status 파싱, legacy status rewrite, 본문 separator 보호 회귀 테스트 추가

- **Status Dashboard Legacy Title Recovery**: `status` 대시보드가 legacy `# SPEC: ...` 헤더를 쓰는 SPEC에서도 ID, 상태, 제목을 다시 함께 표시하도록 회귀를 보강
  - `internal/cli/status_legacy_test.go` — `# SPEC: ...` + `**SPEC-ID**:` 형식의 legacy SPEC가 대시보드에서 제목과 상태를 유지하는지 검증

## [v0.40.27] — 2026-04-16

### Fixed

- **Auto Sync Completion Gate Recovery**: Codex `auto sync` 가 더 이상 컨텍스트/주석/커밋 게이트를 빠뜨린 채 완료를 선언하지 않도록 completion discipline을 source-of-truth와 테스트에 고정
  - `templates/codex/skills/auto-sync.md.tmpl`, `templates/codex/prompts/auto-sync.md.tmpl` — `Context Load`, `SPEC Path Resolution`, `@AX Lifecycle Management`, `Lore commit hash 또는 blocked reason`, `2-Phase Commit decision` 을 `Completion Gates` 로 승격하고, 암묵적 subagent 제한 시 사용자 opt-in 또는 `--solo` 확인을 먼저 요구하도록 보강
  - `pkg/adapter/codex/codex_prompts_test.go`, `pkg/adapter/codex/codex_surface_test.go` — generated Codex prompt/skill surface가 `@AX: no-op`, `commit hash`, completion gate 문구를 유지하는지 회귀 테스트 추가

- **OpenCode Runtime Wording Parity**: OpenCode generated `auto sync` skill에 Codex 전용 런타임 문구가 새지 않도록 변환기와 회귀 테스트를 보강
  - `pkg/adapter/opencode/opencode_util.go` — `task(...)` 문맥에서 `Codex 런타임 정책` 잔여 문구를 `OpenCode 런타임 정책` 으로 정규화
  - `pkg/adapter/opencode/opencode_test.go`, `pkg/adapter/opencode/opencode_sync_gate_test.go` — shared `.agents/skills/auto-sync/SKILL.md` 에 completion gate와 OpenCode wording parity가 유지되는지 검증

## [v0.40.26] — 2026-04-16

### Fixed

- **Workspace Policy Context Propagation**: `auto setup` 이 루트 저장소 역할과 nested repo 경계, generated/runtime 추적 정책을 별도 `workspace.md` 문서로 기록하고 이후 라우터가 공통 컨텍스트로 다시 읽도록 정렬
  - `templates/codex/skills/auto-setup.md.tmpl`, `templates/codex/prompts/auto-setup.md.tmpl` — `workspace.md` 를 `.autopus/project/` 핵심 산출물로 승격하고 meta workspace / source-of-truth / generated-runtime 경로 기록 규약 추가
  - `templates/codex/skills/auto-go.md.tmpl`, `templates/codex/prompts/auto-go.md.tmpl`, `templates/codex/skills/auto-sync.md.tmpl`, `templates/codex/prompts/auto-sync.md.tmpl` — 구현/동기화 단계가 `.autopus/project/workspace.md` 를 공통 프로젝트 컨텍스트로 로드하도록 보강
  - `pkg/adapter/codex/codex_context_docs.go`, `pkg/adapter/codex/codex_skill_render.go`, `pkg/adapter/opencode/opencode_router_contract.go`, `pkg/adapter/opencode/opencode_util.go`, `templates/claude/commands/auto-router.md.tmpl` — Codex prompt/plugin router, OpenCode shared router/alias command, Claude router가 모두 동일한 workspace policy context load 및 canonical router hand-off 계약을 따르도록 정렬
  - `pkg/adapter/codex/codex_workspace_context_test.go`, `pkg/adapter/opencode/opencode_workspace_context_test.go`, `pkg/adapter/claude/claude_workspace_context_test.go` — `workspace.md` 전파 회귀 테스트를 추가해 플랫폼별 contract drift를 다시 통과하지 못하게 보강

## [v0.40.25] — 2026-04-16

### Fixed

- **Codex Router Prompt Contract Recovery**: Codex `@auto` 메인 prompt surface가 workflow skill 쪽에만 있던 브랜딩/실행 계약을 prompt에도 동일하게 주입하고, 대형 프로젝트 문서가 잘리지 않도록 기본 project doc budget을 상향
  - `pkg/adapter/codex/codex_prompts.go`, `pkg/adapter/codex/codex_skill_render.go` — generated `.codex/prompts/auto*.md` 에 canonical branding block과 `Router Execution Contract` 를 주입
  - `templates/codex/config.toml.tmpl`, `pkg/adapter/codex/codex_lifecycle.go` — `project_doc_max_bytes` 기본값을 `262144` 로 상향하고, router prompt / config drift를 `validate` 에서 탐지하도록 보강
  - `pkg/adapter/codex/codex_*_test.go` — branding, router contract, Context7 rule, doc budget 회귀 테스트 추가

- **Context7 Web Fallback Contract Recovery**: 외부 라이브러리 문서 조회 규칙이 이제 `Context7 MCP 우선 → 실패 시 web search fallback` 계약을 공통 rule, pipeline skill, Codex/OpenCode generated surface 전반에서 일관되게 유지
  - `content/rules/context7-docs.md`, `content/skills/agent-pipeline.md`, `pkg/adapter/codex/codex_extended_skill_rewrites_agents.go` — Context7 실패 시 official docs / release notes / API reference 중심 web fallback 절차를 문서화
  - `pkg/content/skill_transformer_replace.go` — non-Claude platform surface에서 `mcp__context7__*` references를 단순 `WebSearch` 치환이 아니라 Context7-first / web-fallback 의미가 보존되는 안내로 변환
  - `pkg/adapter/opencode/opencode_lifecycle.go`, `pkg/adapter/opencode/opencode_test.go`, `pkg/content/*test.go` — OpenCode/Codex validate와 content transformer 회귀 테스트로 fallback 계약 누락을 다시 통과하지 못하게 보강

## [v0.40.24] — 2026-04-16

### Fixed

- **Acceptance Gate Lifecycle Recovery**: `spec validate` 와 pipeline validate/review 경로가 더 이상 `acceptance.md` 를 무시하지 않고, scaffold 기본 시나리오 형식도 실제 Gherkin 파서와 일치하도록 복구
  - `pkg/spec/template.go`, `pkg/spec/gherkin_parser.go` — `spec.Load()` 가 `acceptance.md` 를 함께 로드해 `AcceptanceCriteria` 를 채우고, `### Scenario 1:` / `### Edge Case 1:` scaffold 헤더를 파싱하도록 정렬
  - `pkg/pipeline/phase_prompt.go`, `pkg/spec/template_test.go`, `pkg/pipeline/phase_prompt_test.go`, `internal/cli/cli_extra_test.go` — `test_scaffold` / `implement` / `validate` / `review` 프롬프트에 acceptance context를 주입하고, scaffolded SPEC validate 회귀를 추가

- **Codex Shared Skill Branding Recovery**: Codex 에서 `@auto` 브랜드 배너가 간헐적으로 사라지던 문제를, 실제 우선 선택되던 shared `.agents/skills/` 경로에도 canonical branding block을 주입하도록 보강
  - `pkg/adapter/opencode/opencode_util.go`, `pkg/adapter/opencode/opencode_skills.go`, `pkg/adapter/opencode/opencode_workflow_custom.go` — OpenCode가 소유하는 shared skill surface에도 `## Autopus Branding` 과 canonical banner injection을 적용
  - `pkg/adapter/opencode/opencode_test.go` — generated `.agents/skills/auto*.md` 가 branding header를 유지하는지 회귀 테스트 추가

## [v0.40.20] — 2026-04-15

### Fixed

- **OpenCode Router SPEC Path Resolution Contract Recovery**: OpenCode `auto` command/skill 생성물이 shared router contract의 `SPEC Path Resolution` 섹션을 다시 포함하고, OpenCode 표면에 Codex 전용 wording이 새지 않도록 정렬
  - `pkg/adapter/opencode/opencode_router_contract.go`, `pkg/adapter/opencode/opencode_commands.go`, `pkg/adapter/opencode/opencode_skills.go` — Claude canonical router에서 SPEC path resolution block을 추출해 OpenCode `auto` surfaces에 재주입하고, `TARGET_MODULE` / `WORKING_DIR` / `Available SPECs` 계약을 복원
  - `pkg/adapter/opencode/opencode_test.go` — 생성된 `.opencode/commands/auto.md` 와 `.agents/skills/auto/SKILL.md` 가 `SPEC Path Resolution` 을 유지하고 Codex wording leak이 없는지 회귀 테스트 추가

- **Workspace-Root Submodule SPEC Resolution Regression Coverage**: workspace root에서 실행되는 OpenCode SPEC 워크플로우가 `Autopus/.autopus/specs/...` 같은 실제 서브모듈 SPEC를 놓치지 않도록 회귀 케이스를 보강
  - `pkg/spec/resolve_test.go` — `SPEC-OPCOCK-001` 이 workspace root 기준으로 `Autopus` 서브모듈에서 정확히 resolve 되는지 검증

## [v0.40.18] — 2026-04-14

### Fixed

- **Codex `@auto` Branding Injection**: Codex local plugin skill surface가 router/prompt에는 있던 문어 배너 지시를 실제 `@auto` plugin workflow skill에도 동일하게 주입하도록 정렬
  - `pkg/adapter/codex/codex_skill_render.go`, `pkg/adapter/codex/codex_workflow_custom.go` — router skill과 workflow/custom workflow skill 생성 경로 모두에 canonical Autopus branding block을 삽입
  - `pkg/adapter/codex/codex_surface_test.go` — `.agents` / `.autopus/plugins` Codex skill surfaces가 branding header를 유지하는지 회귀 테스트 추가

## [v0.40.17] — 2026-04-14

### Added

- **OpenCode Strategic Skill Canonical Sources**: OpenCode가 더 이상 Claude 전용 산출물에 의존하지 않도록 `product-discovery`, `competitive-analysis`, `metrics`를 canonical `content/skills/`에 추가
  - `content/skills/product-discovery.md`, `content/skills/competitive-analysis.md`, `content/skills/metrics.md` — platform-agnostic source로 승격하여 OpenCode `.agents/skills/`에도 동일하게 배포되도록 정렬

### Fixed

- **Codex Workflow and Rule Parity Recovery**: Codex 하네스가 Claude Code 기준 workflow surface와 규칙 패키징을 다시 충족하도록 정렬
  - `pkg/adapter/codex/codex_workflow_specs.go`, `pkg/adapter/codex/codex_workflow_custom.go`, `pkg/adapter/codex/codex_prompts.go`, `templates/codex/prompts/auto.md.tmpl` — `@auto` router와 workflow generation이 `status`, `map`, `why`, `verify`, `secure`, `test`, `dev`, `doctor`를 포함한 전체 helper flow surface를 생성하도록 복구
  - `pkg/adapter/codex/codex_rules.go`, `pkg/adapter/codex/codex_skill_render.go`, `pkg/adapter/codex/codex_skill_template_mappings.go`, `pkg/adapter/codex/codex_standard_skills.go` — Codex rule/skill rendering이 stub `@import` 대신 canonical content와 Codex-native semantics를 사용하고 `branding`, `project-identity` rule parity를 회복
  - `pkg/adapter/codex/codex_*_test.go`, `pkg/adapter/parity_test.go`, `pkg/adapter/integration_test.go` — prompt/rule count와 cross-platform parity 회귀 테스트를 추가해 workflow 누락과 규칙 드리프트를 다시 통과하지 못하게 보강

- **OpenCode Helper Flow Surface Recovery**: OpenCode router와 command surface가 `setup` 외 helper flow도 노출하고, Codex prompt 단일 의존 없이 OpenCode 전용 contract를 사용하도록 정리
  - `pkg/adapter/opencode/opencode_specs.go`, `pkg/adapter/opencode/opencode_router_contract.go`, `pkg/adapter/opencode/opencode_workflow_custom.go` — `status`, `map`, `why`, `verify`, `secure`, `test`, `dev`, `doctor` helper flow inventory와 custom skill/command body 추가
  - `pkg/adapter/opencode/opencode_commands.go`, `pkg/adapter/opencode/opencode_skills.go` — router/command generation이 OpenCode-native helper semantics와 상세 스킬 목록을 사용하도록 갱신

- **OpenCode Plugin Wiring Diagnostics**: hook plugin이 파일만 생성되고 `opencode.json`에는 연결되지 않던 결손을 수정하고, registration 누락을 validation에서 탐지하도록 보강
  - `pkg/adapter/opencode/opencode_config.go`, `pkg/adapter/opencode/opencode.go`, `pkg/adapter/opencode/opencode_lifecycle.go`, `pkg/adapter/opencode/opencode_util.go` — managed plugin 경로를 기본 등록하고 plugin array parsing/validation을 보강
  - `pkg/adapter/opencode/opencode_runtime_test.go`, `pkg/adapter/opencode/opencode_test.go` — helper flow surface, plugin registration, strategic skill generation 회귀 테스트 추가

- **Queued Task Deadline Guard**: 이미 만료된 worker task가 semaphore 슬롯을 선점하거나 subprocess를 시작하지 않도록 acquire 단계의 cancellation 우선순위를 보강
  - `pkg/worker/parallel/semaphore.go`, `pkg/worker/loop_runtime_fix_test.go` — 만료된 context는 즉시 거절하고 queued-task expiry 회귀 테스트 기대를 다시 만족하도록 정렬
  - `pkg/adapter/integration_test.go` — Codex prompt surface 확장에 맞춰 E2E prompt count 기대치를 갱신

- **Worker MCP Startup Compatibility**: Codex가 worker MCP 서버를 startup 단계에서 타입 오류 없이 수용하도록 초기 lifecycle, tool schema, resource 응답 형식을 최신 MCP 계약에 가깝게 정렬
  - `pkg/worker/mcpserver/server.go`, `pkg/worker/mcpserver/server_test.go` — `initialize` protocol negotiation, `tools/list` schema metadata, `tools/call` structured result envelope, `resources/templates/list`, `resources/read` contents wrapper 추가
  - `pkg/worker/mcpserver/resources.go`, `pkg/worker/mcpserver/resources_test.go` — resource title/template metadata를 추가해 execution URI template discovery를 노출
  - `templates/codex/config.toml.tmpl` — Codex generated config가 `autopus` MCP를 다시 기본 등록해도 startup validation을 통과하도록 정렬

## [v0.40.13] — 2026-04-14

### Fixed

- **OpenCode Workflow Surface Alignment**: OpenCode가 `auto` workflow를 얇은 prompt entrypoint가 아니라 실제 skill 템플릿과 맞는 표면으로 생성하도록 정렬
  - `pkg/adapter/opencode/opencode_specs.go`, `pkg/adapter/opencode/opencode_skills.go` — workflow별 prompt와 skill source를 분리하고, `auto`는 thin router / 하위 workflow는 실제 skill 템플릿으로 생성되도록 조정
  - `pkg/adapter/opencode/opencode_util.go` — OpenCode `task(...)` / command entrypoint semantics에 맞는 body normalization과 예제 치환 보강
  - `pkg/adapter/opencode/opencode_test.go` — workflow skill / command surface 회귀 테스트 추가

- **Codex Router Thin-Skill Stabilization**: Codex router skill이 더 이상 Claude router rewrite에 의존하지 않고 Codex thin router semantics로 생성되도록 정리
  - `pkg/adapter/codex/codex_standard_skills.go`, `pkg/adapter/codex/codex_skill_render.go`, `pkg/adapter/codex/codex_plugin_manifest.go` — router rendering과 plugin metadata를 분리하고 300-line limit를 만족하도록 파일 분할
  - `pkg/adapter/codex/codex_test.go` — `.agents/.autopus/.codex` 전 surface 회귀 테스트 추가

- **Gemini Canary Workflow Parity**: Gemini `canary` command가 참조하던 `auto-canary` skill 누락을 보완해 command-skill 정합성을 복구
  - `templates/gemini/skills/auto-canary/SKILL.md.tmpl` — Gemini 전용 `auto-canary` skill 추가
  - `pkg/adapter/gemini/gemini_test.go` — workflow command와 대응 skill 생성 정합성 회귀 테스트 추가

## [v0.40.12] — 2026-04-14

### Fixed

- **`auto update` New Platform Detection**: 바이너리 업데이트 후 새로 설치한 OpenCode 같은 supported CLI가 기존 프로젝트의 `auto update` 경로에서 자동 반영되지 않던 문제 수정
  - `internal/cli/update.go`, `internal/cli/init_helpers.go` — `update`가 현재 설치된 supported platform을 다시 감지해 `autopus.yaml`에 누락된 플랫폼을 추가하고, 같은 실행에서 해당 하네스를 생성하도록 정렬
  - `internal/cli/update_test.go` — 기존 `claude-code` 프로젝트에서 `opencode` 설치 후 `auto update`가 `opencode.json`과 `.opencode/` 하네스를 생성하는 회귀 테스트 추가

## [v0.40.11] — 2026-04-14

### Fixed

- **Worker Queue Timeout Separation**: worker 실행 대기와 provider 세마포어 대기를 분리해, 혼잡 상황에서도 queue starvation과 잘못된 타임아웃 해석이 줄어들도록 정리
  - `pkg/worker/loop.go`, `pkg/worker/loop_exec.go`, `pkg/worker/loop_test.go` — worker loop가 queue wait / execution timeout을 구분해 처리하고 직렬화 경로를 더 명확히 검증하도록 보강
  - `internal/cli/worker_start.go`, `internal/cli/worker_start_test.go` — worker start 경로가 새 timeout semantics와 직렬화 보강을 반영하도록 조정

- **Codex Worker Concurrency Stabilization**: Codex worker 동시 실행 시 output artifact와 setup 경로가 더 안정적으로 유지되도록 보강
  - `internal/cli/worker_setup_wizard.go`, `internal/cli/worker_setup_wizard_test.go` — setup wizard가 최신 worker concurrency 흐름과 일치하도록 조정

## [v0.40.10] — 2026-04-14

### Added

- **OpenCode Native Harness Generation**: `auto init/update`가 이제 OpenCode를 정식 하네스 설치 플랫폼으로 지원하여 `.opencode/` 네이티브 산출물과 `.agents/skills/` 표준 스킬을 함께 생성
  - `pkg/adapter/opencode/*` — OpenCode 어댑터를 stub에서 실제 generate/update/validate/clean 구현으로 확장하고 `AGENTS.md`, `opencode.json`, `.opencode/rules/`, `.opencode/agents/`, `.opencode/commands/`, `.opencode/plugins/`를 생성
  - `internal/cli/init_helpers.go`, `internal/cli/update.go`, `internal/cli/doctor.go`, `internal/cli/platform.go`, `internal/cli/init.go` — OpenCode를 init/update/doctor/platform add-remove 및 gitignore 경로에 연결
  - `pkg/adapter/opencode/opencode_test.go`, `pkg/content/opencode_transform_test.go` — OpenCode 산출물 생성, 설정 병합, CLI 연결, 변환 규칙 회귀 테스트 추가

### Fixed

- **OpenCode Content Mapping**: Claude 중심 helper 문서와 agent source가 OpenCode native surface에 맞게 치환되도록 정렬
  - `pkg/content/skill_transformer.go`, `pkg/content/skill_transformer_replace.go`, `pkg/content/agent_transformer_opencode.go` — `.claude/*` 경로를 `.opencode/*` / `.agents/skills/*`로 치환하고, subagent/tool references를 OpenCode `task`, `question`, `todowrite` 중심 semantics로 재해석

### Fixed

- **JWT-Only Worker / No-Bridge Cleanup**: worker setup, connect wizard, runtime lifecycle가 더 이상 bridge source provisioning이나 bridge-based file sync를 전제로 하지 않도록 정리
  - `internal/cli/worker_setup_wizard.go`, `internal/cli/connect.go`, `internal/cli/worker_start.go` — setup/connect가 JWT-only auth 및 authenticated provider 우선 선택으로 정렬되고 bridge source 자동 생성 제거
  - `pkg/worker/loop.go`, `pkg/worker/loop_lifecycle.go`, `pkg/worker/setup/config.go` — runtime이 legacy bridge sync source를 더 이상 사용하지 않고 local knowledge search만 유지하도록 조정
  - `pkg/e2e/build.go`, `README.md` — user-facing build/docs 표면에서 deprecated bridge target 설명 제거

## [v0.40.5] — 2026-04-13

### Fixed

- **Worker Launch Readiness Alignment**: worker setup이 knowledge source provisioning, worktree isolation, runtime launch 경로를 실제 실행 계약과 맞추도록 정리
  - `internal/cli/worker_setup_wizard.go`, `internal/cli/worker_start.go`, `pkg/worker/loop_lifecycle.go` — setup wizard에서 받은 knowledge/worktree 설정이 런칭 직전 lifecycle과 source provisioning에 실제 연결되도록 보강
  - `pkg/worker/setup/config.go`, `pkg/worker/setup/config_test.go` — worker config가 knowledge source 및 isolation 필드를 안정적으로 유지하도록 회귀 보강

- **Knowledge Sync / MCP Path Contract Repair**: knowledge sync와 MCP 검색 경로가 현재 서버 계약 및 테스트 기대와 다시 일치
  - `pkg/worker/knowledge/syncer.go`, `pkg/worker/knowledge/syncer_test.go` — knowledge sync 입력/출력 경로와 에러 처리 흐름을 서버 계약 기준으로 복구
  - `pkg/worker/mcpserver/tools.go`, `pkg/worker/mcpserver/tools_test.go` — MCP search tooling이 sync된 knowledge location을 기준으로 검색하도록 정렬

- **Claude Worker Session Resume Recovery**: Claude worker 재개 경로가 현재 런타임/테스트 기대와 맞게 복구
  - `pkg/worker/adapter/claude.go` — resumed Claude worker session wiring을 현재 adapter contract에 맞게 조정

## [v0.40.4] — 2026-04-13

### Fixed

- **Codex Team Mode Semantics**: Codex `--team` 문서와 생성 스킬이 이제 Claude Team API가 아니라 하네스가 생성한 `.codex/agents/*` 역할 정의를 사용하는 멀티에이전트 오케스트레이션으로 정렬
  - `pkg/adapter/codex/codex_extended_skill_rewrites.go` — `agent-teams` / `agent-pipeline` Codex rewrite가 harness-defined agents와 `spawn_agent(...)` coordination을 기준으로 설명되도록 갱신
  - `templates/codex/skills/agent-teams.md.tmpl`, `templates/codex/skills/auto-go.md.tmpl`, `templates/codex/prompts/auto-go.md.tmpl` — generated Codex docs now explain `--team` as `.codex/agents/` role orchestration and `--multi` as extra review/orchestra reinforcement

- **`--multi` Runtime Activation**: 루트 전역 플래그 `--multi`가 더 이상 단순 노출에 그치지 않고 SPEC review / pipeline run에서 실제 멀티 프로바이더 리뷰 흐름을 확장
  - `internal/cli/spec_review.go` — `--multi` 시 review provider set을 review gate + orchestra config + default providers로 확장하고, 설치된 provider가 2개 미만이면 명확히 실패
  - `internal/cli/pipeline_run.go` — `auto pipeline run --multi` 완료 후 실제 `runSpecReview(...)`를 호출해 다중 프로바이더 검증을 수행
  - `internal/cli/spec_review_test.go`, `internal/cli/pipeline_run_test.go`, `pkg/adapter/codex/codex_coverage_test.go` — provider expansion 및 Codex multi/team semantics regression coverage 추가

## [v0.40.3] — 2026-04-13

### Fixed

- **Codex Harness Hook Drift**: Codex 훅 생성이 더 이상 깨진 템플릿 명령에 의존하지 않고, 실제 훅 생성 로직과 같은 소스에서 `.codex/hooks.json`을 만들도록 정리
  - `pkg/adapter/codex/codex_hooks.go` — Codex hook rendering now marshals `pkg/content/hooks.go` output directly, so `PreToolUse`/`PostToolUse` stay aligned with real CLI support
  - `pkg/adapter/codex/codex_internal_test.go`, `pkg/adapter/codex/codex_coverage_test.go` — invalid `SessionStart`/`Stop` expectations 제거, unsupported `auto check --status`, `auto session save`, `auto check --lore --quiet` 회귀 방지

- **Lore Guidance Alignment**: Lore 문서와 생성 스킬이 현재 프로토콜과 실제 검사 범위를 기준으로 정리
  - `content/rules/lore-commit.md`, `content/skills/lore-commit.md` — legacy `Why/Decision/Alternatives` 중심 설명을 `Constraint` 계열 프로토콜과 `auto check --lore` / `auto lore validate` 실제 역할 기준으로 갱신
  - `templates/codex/skills/lore-commit.md.tmpl`, `templates/gemini/skills/lore-commit/SKILL.md.tmpl` — 생성되는 Codex/Gemini Lore 스킬도 동일한 프로토콜로 정렬

## [v0.40.2] — 2026-04-13

### Fixed

- **Release Workflow Action Drift**: GitHub Release workflow의 deprecated Node 20 / floating version 경고를 줄이기 위해 action 버전과 GoReleaser 버전 범위를 최신 기준으로 정리
  - `.github/workflows/release.yaml` — `actions/checkout@v6`, `actions/setup-go@v6`, `goreleaser/goreleaser-action@v7` 로 갱신
  - `.github/workflows/release.yaml` — GoReleaser 실행 버전을 `latest` 대신 `~> v2`로 고정해 릴리즈 시 경고를 제거
  - `.github/workflows/release.yaml` — 더 이상 필요 없는 `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24` 환경 변수 제거

## [v0.40.1] — 2026-04-13

### Fixed

- **Codex Harness Flag Parity**: Codex `@auto` router와 하위 스킬이 Claude 전용 가정을 덜어내고 Codex 실행 모델에 맞게 정규화됨
  - `pkg/adapter/codex/codex_standard_skills.go` — `AskUserQuestion`, `TeamCreate`, `SendMessage`, legacy `/auto` 예시를 Codex의 `spawn_agent(...)`, `send_input(...)`, plain-text 확인 흐름으로 재해석
  - `templates/codex/skills/auto-*.md.tmpl`, `templates/codex/prompts/auto-*.md.tmpl` — `--team`, `--loop`, `--auto`, `--quality`, `--continue` 등 핵심 플래그 의미와 `@auto ...` 표기를 보강
  - `templates/codex/skills/auto-canary.md.tmpl` — `auto-canary`를 prompt fallback이 아닌 전용 skill 템플릿 기반으로 생성

- **Codex Helper Skill Rewrite Layer**: 깊은 helper 문서가 더 이상 Claude Code Team/permission/worktree 전제를 직접 요구하지 않도록 Codex 전용 body rewrite 추가
  - `pkg/adapter/codex/codex_extended_skill_rewrites.go` — `agent-teams`, `agent-pipeline`, `worktree-isolation`, `subagent-dev`, `prd` 문서를 Codex orchestration semantics로 재작성
  - `pkg/adapter/codex/codex_extended_skills.go`, `codex_skills.go`, `codex_prompts.go`, `codex_agents.go` — helper path 및 invocation 정규화를 생성 파이프라인 전반에 적용
  - `pkg/adapter/codex/codex_coverage_test.go` — Codex 전용 rewrite 회귀 테스트 추가

## [v0.40.0] — 2026-04-13

### Added

- **Codex Standard Skills + Local Plugin Bootstrap**: Codex 최신 표준에 맞춰 repo skill 및 local plugin 진입점을 자동 생성
  - `pkg/adapter/codex/codex_standard_skills.go` — `.agents/skills/*` 표준 스킬과 `.autopus/plugins/auto` 로컬 플러그인 번들 생성
  - `pkg/adapter/codex/codex.go` — Codex generate/update 시 `.agents/skills`, `.agents/plugins`, `.autopus/plugins/auto` 출력 경로 생성
  - `pkg/adapter/codex/codex_lifecycle.go` — validate/clean이 `.agents/skills/*`, `.agents/plugins/marketplace.json`, `.autopus/plugins/auto`를 인식하도록 확장
  - `pkg/adapter/codex/codex_skills.go` — AGENTS.md에 Agent Skills / Plugin Marketplace 경로 노출
  - `internal/cli/init.go` — Codex 다음 단계 안내를 `$auto ...` / `@auto ...` 기준으로 갱신하고 `.agents/plugins/`를 gitignore에 추가
  - `pkg/adapter/codex/codex_test.go`, `pkg/adapter/integration_test.go`, `pkg/adapter/parity_test.go`, `internal/cli/*_test.go` — 표준 스킬/플러그인 생성 회귀 테스트 추가

- **Codex Invocation Normalization**: Codex generated skill examples and chaining messages now prefer `@auto plan`, `@auto go`, `@auto idea` syntax while preserving `$auto ...` fallback
  - generated Codex skills normalize legacy `/auto` and `@auto-foo` references into Codex-compatible `@auto foo` forms

- **Codex Brainstorm / Multi-Provider Parity**: `auto idea` workflow is now exposed through Codex standard entrypoints without dropping multi-provider discussion or flag-based chaining
  - generated `auto-idea` Codex skills preserve `--strategy`, `--providers`, `--auto` and `@auto plan --from-idea ...` chaining semantics

### Added

- **Gemini CLI Harness Parity**: Gemini CLI 어댑터에 Claude Code 및 Codex 수준의 기능 패리티 구현
  - `/auto` 라우터 명령어 지원 (`auto-router.md.tmpl`)
  - 상태 업데이트를 위한 `statusline.sh` 복사 로직 추가
  - 테스트 코드에 Gemini 템플릿 포함 및 검증 추가

### Fixed

- **macOS Self-Update Crash (zsh: killed)**: `auto update --self` 실행 시 macOS 커널 보호(SIGKILL) 및 Linux ETXTBSY 에러 우회
  - 실행 중인 바이너리를 덮어쓰지 않고 `.old`로 이동(Rename) 후 새 바이너리로 교체하도록 `replacer.go` 수정
  - Cross-device 링크 시 fallback (io.Copy) 로직 추가


- **Init Platform Auto-Detection**: `auto init` without `--platforms` now scans PATH for supported installed coding CLIs and installs all detected supported platforms
  - `internal/cli/init.go` — default platform selection now delegates to PATH-based detection when `--platforms` is omitted
  - `internal/cli/init_helpers.go` — `detectDefaultPlatforms()` filters detected CLIs to ADK-supported init targets (`claude-code`, `codex`, `gemini-cli`) with Claude fallback
  - `internal/cli/init_test.go` — auto-detect and no-CLI fallback regression tests
  - `pkg/detect/detect.go` — orchestra provider detection now tracks `codex` instead of stale `opencode`
  - `pkg/detect/detect_test.go` — provider detection expectations updated to Codex
  - `README.md`, `docs/README.ko.md` — docs aligned to 3 auto-generated platforms and supported-CLI wording

- **Worker 프로세스 안정화** (SPEC-WKPROC-001):
  - `pkg/worker/pidlock/` — PID lock 패키지 (advisory flock, stale detection, auto-reclaim)
  - `pkg/worker/reaper/` — Zombie 프로세스 reaper (30초 주기, Unix Wait4, build-tag 분리)
  - `pkg/worker/mcpserver/sse.go` — MCP SSE transport (/mcp/sse 엔드포인트)
  - `pkg/worker/mcpserver/config.go` — MCP config 구조체 + JSON 검증
  - `pkg/worker/mcpserver/server.go` — NewMCPServerFromConfig, StartSSE 메서드
  - `pkg/worker/loop.go` — Start/Close에 PID lock 획득/해제 통합
  - `pkg/worker/loop_lifecycle.go` — startServices에 reaper goroutine 추가
  - `pkg/worker/daemon/launchd.go` — ProcessType=Background, ThrottleInterval=10
  - `pkg/worker/daemon/systemd.go` — StandardOutput/StandardError 로그 경로
  - `internal/cli/worker_commands.go` — worker status에 PID 표시

## [v0.37.0] — 2026-04-07

### Added

- **Pipeline-Learn Auto Wiring** (SPEC-LEARNWIRE-002): 파이프라인 gate 실패 시 자동 학습 기록
  - `pkg/learn/store.go` — AppendAtomic 동시성 안전 메서드 (sync.Mutex)
  - `pkg/pipeline/learn_hook.go` — nil-safe hook wrapper 4개 (gate fail, coverage gap, review issue, executor error) + 출력 파싱
  - `pkg/pipeline/runner.go` — SequentialRunner/ParallelRunner에 learn hook 와이어링 (R2-R6, R9)
  - `pkg/pipeline/phase.go` — DefaultPhases()에 GateValidation/GateReview 할당 (R10)
  - `pkg/pipeline/engine.go` — EngineConfig.RunConfig 필드 추가
  - `internal/cli/pipeline_run.go` — .autopus/learnings/ 조건부 Store 초기화 (D4)

- **SPEC Review Convergence** (SPEC-REVCONV-001): 2-Phase Scoped Review로 REVISE 루프 수렴성 보장
  - `pkg/spec/types.go` — FindingStatus, FindingCategory, ReviewMode 타입, ReviewFinding 확장 (ID/Status/Category/ScopeRef/EscapeHatch)
  - `pkg/spec/prompt.go` — Mode-aware BuildReviewPrompt (discover: open-ended, verify: checklist + FINDING_STATUS 스키마)
  - `pkg/spec/reviewer.go` — ParseVerdict 확장 (priorFindings 기반 scope filtering), ShouldTripCircuitBreaker, MergeFindingStatuses (supermajority merge)
  - `pkg/spec/review_persist.go` — PersistReview 분리 (reviewer.go 300줄 리밋 준수)
  - `pkg/spec/findings.go` — review-findings.json 영속화, ScopeRef 정규화, ApplyScopeLock, DeduplicateFindings
  - `pkg/spec/static_analysis.go` — golangci-lint JSON 파싱, RunStaticAnalysis graceful skip, MergeStaticWithLLMFindings dedup
  - `internal/cli/spec_review.go` — REVISE 루프 (discover→verify 전환, max_revisions, circuit breaker, static analysis 통합)
  - 테스트 커버리지 93.7% (convergence_test, findings_test, static_analysis_test, coverage_gap_test, coverage_merge_test)

- **resolvePlatform Unit Tests** (SPEC-AXQUAL-001): PATH 의존 플랫폼 감지 로직 단위 테스트 추가
  - `internal/cli/pipeline_run_test.go` — `TestResolvePlatform` table-driven 테스트 (explicit platform, PATH 탐색 우선순위, 빈 PATH 폴백)
  - `internal/cli/pipeline_run.go` — `@AX:TODO` 태그 제거, `@AX:NOTE` 추가
  - `internal/cli/agent_create.go`, `skill_create.go` — 템플릿 TODO 마커에 `@AX:EXCLUDE` 문서화

- **ADK Worker Approval Flow** (SPEC-ADKWA-001): Backend MCP → A2A WebSocket → Worker TUI 승인 플로우 구현
  - `pkg/worker/a2a/types.go` — `MethodApproval`, `MethodApprovalResponse` 상수, `ApprovalRequestParams`, `ApprovalResponseParams` 타입 정의
  - `pkg/worker/a2a/server.go` — `ApprovalCallback` 콜백 필드, `handleApproval` 핸들러 (input-required 상태 전환)
  - `pkg/worker/a2a/server_approval.go` — `SendApprovalResponse` (tasks/approvalResponse JSON-RPC 전송, working 상태 복원)
  - `pkg/worker/tui/model.go` — `OnApprovalDecision` / `OnViewDiff` 콜백, a/d/s/v 키 바인딩
  - `pkg/worker/loop.go` — WorkerLoop A2A 콜백 → TUI program 브릿지 와이어링

- **Multi-Platform Harness Integration** (SPEC-MULTIPLATFORM-001): Codex/Gemini 어댑터를 Claude Code 수준 하네스 패리티로 확장
  - Codex: 커스텀 프롬프트 (`codex_prompts.go`), 에이전트 정의 (`codex_agents.go`), 훅 설정 (`codex_hooks.go`), MCP/권한 설정 (`codex_settings.go`), 규칙 인라인 (`codex_rules.go`), 전체 스킬 변환 (`codex_skills.go`), 라이프사이클/마커 관리 (`codex_lifecycle.go`, `codex_marker.go`)
  - Gemini: 커스텀 커맨드 (`gemini_commands.go`), 에이전트 정의 (`gemini_agents.go`), 훅/설정 통합 (`gemini_hooks.go`, `gemini_settings.go`), 규칙+@import (`gemini_rules.go`), 전체 스킬 변환 (`gemini_skills.go`), 라이프사이클/마커 관리 (`gemini_lifecycle.go`, `gemini_marker.go`)
  - Shared: 크로스 플랫폼 템플릿 헬퍼 (`pkg/template/helpers.go` — TruncateToBytes, MapPermission, SkillList), 공유 테스트 유틸 (`pkg/adapter/testutil_test.go`)
  - Templates: `templates/codex/` (agents, prompts, skills, hooks.json.tmpl, config.toml.tmpl), `templates/gemini/` (commands, rules, settings, skills)

- **Permission Detect** (SPEC-PERM-001): `auto permission detect` 서브커맨드 및 agent-pipeline 동적 권한 상승
  - `pkg/detect/permission.go` — DetectPermissionMode: 부모 프로세스 트리에서 `--dangerously-skip-permissions` 감지, 환경변수 오버라이드, fail-safe 반환
  - `pkg/detect/permission_test.go` — 환경변수 오버라이드, invalid 값 폴백, 프로세스 검사 실패 시 safe 반환 테스트
  - `internal/cli/permission.go` — `auto permission detect` Cobra 서브커맨드, `--json` 출력 모드 지원
  - `content/skills/agent-pipeline.md` — Permission Mode Detection 섹션 추가, 동적 mode 할당 규칙
  - `templates/claude/commands/auto-router.md.tmpl` — Step 0.5 Permission Detect 및 조건부 mode 파라미터

- **Brainstorm Multi-Turn Debate Protocol** (SPEC-ORCH-009): brainstorm 커맨드에서 멀티턴 debate 활성화 및 ReadScreen 출력 정제 강화
  - `internal/cli/orchestra_brainstorm.go` — `resolveRounds()` 호출 추가로 brainstorm debate 기본 2라운드 적용, `--rounds N` 플래그 추가
  - `pkg/orchestra/screen_sanitizer.go` — SanitizeScreenOutput: ANSI/CSI/OSC/DCS 이스케이프, 상태바, trailing whitespace 제거하는 순수 함수
  - `pkg/orchestra/interactive_detect.go` — cleanScreenOutput()에서 SanitizeScreenOutput() 호출로 rebuttal 프롬프트 품질 개선

- **Interactive Multi-Turn Debate** (SPEC-ORCH-008): interactive pane에서 N라운드 핑퐁 토론 실행
  - `pkg/orchestra/interactive_debate.go` — runInteractiveDebate: 멀티턴 debate 루프 (Round1 독립응답 → Round2..N 교차 반박)
  - `pkg/orchestra/interactive_debate_helpers.go` — collectRoundHookResults, runJudgeRound, consensusReached, buildDebateResult
  - `pkg/orchestra/round_signal.go` — RoundSignalName: 라운드 스코프 시그널 파일명, CleanRoundSignals, SendRoundEnvToPane
  - `pkg/orchestra/hook_signal.go` — WaitForDoneRound/ReadResultRound: 라운드별 hook 결과 수집 (하위 호환)
  - `internal/cli/orchestra.go` — `--rounds N` 플래그 (1-10, debate 전략 전용, 기본값 2)
  - `content/hooks/` — AUTOPUS_ROUND 환경변수 인식 (라운드 스코프 파일명 분기, 정수 검증)
  - 조기 합의 감지 (MergeConsensus 66% 임계값), Judge 라운드 interactive 실행
  - hook-opencode-complete.ts sessId path traversal 검증 추가 (보안 수정)

- **Orchestra Hook-Based Result Collection** (SPEC-ORCH-007): 프로바이더 CLI의 hook/plugin 시스템을 활용하여 구조화된 JSON 파일 시그널로 결과 수집
  - `pkg/orchestra/hook_signal.go` — HookSession: 세션 디렉토리 관리, done 파일 200ms 폴링 감시, result.json 파싱, 0o700/0o600 보안 권한
  - `pkg/orchestra/hook_watcher.go` — Hook 모드 waitForCompletion: 프로바이더별 hook/ReadScreen 혼합 분기, 타임아웃 graceful degradation
  - `content/hooks/hook-claude-stop.sh` — Claude Code Stop hook: `last_assistant_message` 추출 → result.json 저장
  - `content/hooks/hook-gemini-afteragent.sh` — Gemini CLI AfterAgent hook: `prompt_response` 추출 → result.json 저장
  - `content/hooks/hook-opencode-complete.ts` — opencode plugin: `text` 필드 추출 → result.json 저장
  - `pkg/adapter/opencode/opencode.go` — opencode PlatformAdapter: plugin 자동 주입, opencode.json 생성/머지
  - `pkg/adapter/claude/claude_settings.go` — Stop hook 자동 주입 (기존 사용자 hook 보존)
  - `pkg/adapter/gemini/gemini_hooks.go` — AfterAgent hook 자동 주입 (기존 사용자 hook 보존)
  - `pkg/config/migrate.go` — codex → opencode 자동 마이그레이션
  - hook 미설정 프로바이더는 기존 SPEC-ORCH-006 ReadScreen + idle 감지로 자동 fallback (R8)
  - debate/relay/consensus 전략이 hook 결과의 `response` 필드를 직접 활용 (R11-R13)

### Fixed

- **Issue Reporter / React Hook Reliability**:
  - `internal/cli/issue.go` — `auto issue report/list/search` now prefer `autopus.yaml` repo config and default autopus issue target for `auto ...` command failures instead of accidentally following the current workspace remote
  - `internal/cli/react.go` — `auto react check --quiet` now skips cleanly when the repo has no configured remote, avoiding repeated Claude hook noise
  - `pkg/content/hooks.go`, `templates/codex/hooks.json.tmpl`, `content/hooks/react-*.sh` — all generated reaction hooks now use the supported `auto react check --quiet` command and deduplicate duplicate `PostToolUse` entries
  - `pkg/spec/resolve_test.go` — added nested submodule regression coverage for depth-2 SPEC resolution

- **SPEC Review Context + Parent Harness Isolation**:
  - `pkg/spec/prompt.go`, `internal/cli/spec_review.go` — `auto spec review` now collects code context only from files explicitly referenced by SPEC `plan.md` / `research.md`, instead of recursively sweeping the whole repo
  - `pkg/spec/reviewer_test.go` — regression coverage for target-file-only collection and module-relative path resolution
  - `pkg/detect/detect.go`, `internal/cli/prompts.go` — parent Autopus rule directories are now treated as real inherited conflicts, and non-interactive init/update automatically set `isolate_rules: true`
  - `pkg/detect/detect_test.go`, `internal/cli/prompts_test.go`, `pkg/adapter/claude/claude_markers.go` — tests and Claude isolation guidance updated for nested harness scenarios

- **Installer PATH Visibility**: installers now expose the actual CLI location and make post-install shell behavior explicit, so `auto`/`autopus` are discoverable after one-line installs
  - `install.sh` — creates an `autopus` alias alongside `auto`, prints concrete PATH export instructions when the install dir is not visible to the current shell, and defers platform auto-detection to `auto init`
  - `install.ps1` — creates `autopus.exe` alongside `auto.exe`, persists PATH updates without duplicate entries, warns Git Bash users to reopen the shell or export the printed path, and defers platform auto-detection to `auto init`
  - `README.md`, `docs/README.ko.md` — install docs now state the `autopus` alias and the Git Bash PATH refresh caveat

- **E2E Scenario Runner Monorepo Build Path** (SPEC-E2EFIX-001): 모노레포 루트에서 `auto test run`할 때 서브모듈별 빌드 커맨드와 작업 디렉토리를 올바르게 해석하도록 수정
  - `pkg/e2e/build.go` (신규) — `BuildEntry` 구조체, `ParseBuildLine()` 멀티 빌드 파서, `ResolveBuildDir()` 서브모듈 경로 매핑, `MatchBuild()` 시나리오별 빌드 선택
  - `pkg/e2e/scenario.go` — `ScenarioSet.Builds []BuildEntry` 필드 추가, `ParseScenarios()` 멀티 빌드 위임
  - `pkg/e2e/runner.go` — 빌드 엔트리별 `sync.Once` 맵, 시나리오 섹션 기반 빌드 선택 및 서브모듈 WorkDir 적용
  - `internal/cli/test.go` — `set.Builds`를 `RunnerOptions`에 전달, 단일 빌드 폴백 유지

### Added

- **Orchestra Interactive Pane Mode** (SPEC-ORCH-006): cmux/tmux에서 프로바이더 CLI를 인터랙티브 세션으로 직접 실행하고 결과 자동 수집
  - `pkg/terminal/terminal.go` — Terminal 인터페이스에 `ReadScreen`, `PipePaneStart`, `PipePaneStop` 메서드 추가
  - `pkg/terminal/cmux.go` — CmuxAdapter: `cmux read-screen`, `cmux pipe-pane` 명령 래핑
  - `pkg/terminal/tmux.go` — TmuxAdapter: `tmux capture-pane`, `tmux pipe-pane` 명령 래핑
  - `pkg/terminal/plain.go` — PlainAdapter no-op 구현
  - `pkg/orchestra/interactive.go` — 인터랙티브 pane 실행 플로우 (pipe capture, session launch, prompt send, ReadScreen 폴링 완료 감지, 결과 수집)
  - `pkg/orchestra/interactive_detect.go` — 프로바이더별 프롬프트 패턴 매칭, idle 감지, ANSI 이스케이프 제거
  - `pane_runner.go`에 `OrchestraConfig.Interactive` 플래그 기반 인터랙티브 모드 분기
  - plain 터미널 또는 인터랙티브 실패 시 기존 sentinel 모드로 자동 fallback (R8)
  - 부분 타임아웃 시 `ReadScreen`으로 수집된 부분 결과를 `TimedOut: true`와 함께 기록 (R9)
  - ANSI 이스케이프 시퀀스, CLI 프롬프트 장식 자동 제거로 깨끗한 결과 전달 (R10)

- **Browser Automation Terminal Adapter** (SPEC-BROWSE-001): 터미널 환경별 브라우저 백엔드 자동 선택
  - `pkg/browse/backend.go` — BrowserBackend 인터페이스 + NewBackend 팩토리 (cmux → CmuxBrowserBackend, 그 외 → AgentBrowserBackend)
  - `pkg/browse/cmux.go` — CmuxBrowserBackend: `cmux browser` CLI 래핑, surface ref 관리, shell escape
  - `pkg/browse/agent.go` — AgentBrowserBackend: `agent-browser` CLI 래핑
  - cmux 실패 시 AgentBrowserBackend로 자동 fallback (R6)
  - 세션 종료 시 브라우저 surface/프로세스 자동 정리 (R7)

- **Orchestra Relay Pane Mode** (SPEC-ORCH-005): relay 전략에서 cmux/tmux pane 기반 인터랙티브 실행 지원
  - `pkg/orchestra/relay_pane.go` — 순차 pane relay 실행 엔진: SplitPane → 인터랙티브 실행 → sentinel 완료 감지 → 결과 수집 → 맥락 주입
  - `-p` 플래그 없이 프로바이더 CLI를 실행하여 전체 TUI/인터랙티브 기능 활용 가능
  - 이전 프로바이더 결과를 heredoc으로 다음 pane에 프롬프트 주입
  - 프로바이더 실패 시 skip-continue 처리 (SPEC-ORCH-004 REQ-3a 패턴 재사용)
  - `runner.go` relay pane fallback 경고 제거 — relay도 `RunPaneOrchestra`로 통합 라우팅
  - pane 라이프사이클 관리: 완료 후 defer로 모든 pane 및 임시 파일 정리
  - plain 터미널 환경에서는 기존 standard relay 실행으로 자동 fallback

- **Agent Teams Terminal Pane Visualization** (SPEC-TEAMPANE-001): `--team` 모드에서 팀원별 cmux/tmux 패널 분할 및 실시간 로그 스트리밍
  - `pkg/pipeline/team_monitor.go` — TeamMonitorSession: PipelineMonitor 인터페이스 구현, plain 터미널 graceful degradation
  - `pkg/pipeline/team_layout.go` — LayoutPlan: 순차적 Vertical split 전략, 3~5인 팀 지원
  - `pkg/pipeline/team_pane.go` — 팀원별 패널 생성/정리, tail -f 로그 스트리밍, shell-escape 보안
  - `pkg/pipeline/team_dashboard.go` — 폭 인식(width-aware) 대시보드 렌더링, compact 모드(< 38자)
  - `pkg/pipeline/monitor.go` — PipelineMonitor 인터페이스 추가 (MonitorSession + TeamMonitorSession 공통 계약)
  - SplitPane 실패 시 자동 cleanup 및 plain 터미널 폴백
  - tmux 지원 (개별 패널 닫기 미지원 제한사항 문서화)

- **Orchestra Agentic Relay Mode** (SPEC-ORCH-004): 프로바이더를 agentic one-shot 모드로 순차 실행하는 relay 전략
  - `pkg/orchestra/relay.go` — 릴레이 실행 로직, 프롬프트 주입, 결과 포맷팅
  - 프로바이더별 agentic 플래그 자동 매핑 (claude: `--allowedTools`, codex: `--approval-mode full-auto`)
  - 이전 프로바이더 분석 결과를 `## Previous Analysis by {provider}` 섹션으로 다음 프로바이더에 주입
  - 부분 실패 시 skip-continue 처리 (REQ-3a)
  - `--keep-relay-output` 플래그로 결과 파일 보존 옵션
  - `/tmp/autopus-relay-{jobID}/` 임시 디렉토리 관리

- **Orchestra Detach Mode** (SPEC-ORCH-003): pane 터미널(cmux/tmux) 감지 시 auto-detach 비동기 실행
  - `pkg/orchestra/job.go` — Job persistence model, status tracking, stale job GC
  - `pkg/orchestra/detach.go` — ShouldDetach() 판정, RunPaneOrchestraDetached() 진입점
  - `internal/cli/orchestra_job.go` — `auto orchestra status/wait/result` CLI 서브커맨드
  - `--no-detach` 플래그로 blocking 실행 강제 가능
  - REQ-11: 1시간 이상 된 abandoned job 자동 정리 (opportunistic GC)
