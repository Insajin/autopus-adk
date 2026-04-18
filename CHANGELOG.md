# Changelog — autopus-adk

All notable changes to this project will be documented in this file.

## [Unreleased]

### Changed

- **Claude Code Agent Teams + mode 파라미터 동기화** (2026-04-18): Agent Teams 공식 스펙(https://code.claude.com/docs/en/agent-teams)을 반영하고, Agent() 호출 파라미터 이름을 `permissionMode` → `mode` 로 통일. 플랫폼별 `--team` 플래그 동작 명시.
  - `content/skills/agent-pipeline.md`, `content/skills/worktree-isolation.md` — 본문 `Agent(... permissionMode=)` 10건 → `mode=`
  - `templates/codex/skills/agent-pipeline.md.tmpl`, `templates/codex/skills/worktree-isolation.md.tmpl`, `templates/gemini/skills/agent-pipeline/SKILL.md.tmpl`, `templates/gemini/skills/worktree-isolation/SKILL.md.tmpl` — 동일 변경 (각 4-6건)
  - `content/skills/agent-teams.md` — Prerequisites 섹션(v2.1.32+ 버전 요구) + Team Constraints 섹션(nested 금지, leader-only cleanup, 3-5명 권장, 영속 경로) 신설. Team Creation Pattern의 `Teammate()` → `Agent(team_name=..., name=...)` 공식 문법으로 교정
  - `templates/claude/commands/auto-router.md.tmpl` — Route B preflight 2단계(버전 + 환경변수) 추가, 에러 메시지 개선
  - `templates/codex/skills/agent-teams.md.tmpl` — 상단 ⚠️ Platform Note: Claude Code 전용 명시, Codex는 `spawn_agent` fallback
  - `templates/gemini/commands/auto-router.md.tmpl`, `templates/gemini/skills/agent-teams/SKILL.md.tmpl` — Platform Note 배너 + Route B 비활성화 + `--team` 경고 후 Route A fallback, 스테일 "Gemini CLI Agent Teams" 참조 제거
  - **Subagent frontmatter `permissionMode:` 필드는 공식 스펙이므로 그대로 유지** (Agent() 호출 파라미터와 별개 레이어)

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
