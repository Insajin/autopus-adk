# Changelog — autopus-adk

All notable changes to this project will be documented in this file.

## [Unreleased]

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
