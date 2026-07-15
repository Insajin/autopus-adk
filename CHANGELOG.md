# Changelog — autopus-adk

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added

- **Desktop 장치 설정 릴리스 handoff (SPEC-DESKTOP-DEVICE-SETUP-001)** (2026-07-14): 결정론적 companion manifest producer, detached Ed25519 서명, Darwin 릴리스 배선, 검증·롤백 계약은 코드로 구현됐으며 관련 집중 테스트가 통과했다. 다만 `T11`과 `AC-019`는 충족되지 않았으며 릴리스도 완료되지 않았다. Production Desktop 릴리스 정책의 `pinnedPublicKeys`와 `priorReleaseKeyIds`는 비어 있고, 실제 이전 릴리스 키 ID와의 중첩이 필요하다. 실제 key ID와 public key는 릴리스 custodian이 제공해야 하며 fixture 값을 사용할 수 없다. 서명·공증된 현재 ADK 아티팩트와 실제 릴리스 증거는 외부 blocker로 남아 있다.

- **Codex Ultra 역할별 worker effort 적용** (2026-07-11): Codex Ultra의 quality-managed supervisor와 orchestra는 Sol+`ultra`를 유지하고, 전략·보안 역할인 `planner`·`architect`·`security-auditor`만 Sol+`max`, 그 밖의 모든 관리형 에이전트와 unknown role은 Sol+`xhigh`를 사용하도록 중앙 profile resolver를 조정했다. `auto quality ultra --apply`와 fresh init이 같은 3개/나머지 역할 분리를 생성하며, 어떤 managed worker도 자동 task delegation이 있는 `ultra` effort를 받지 않는다.

- **Codex 사용자 기본 모델 상속 및 품질 모드 즉시 적용** (2026-07-11): 새 프로젝트는 `quality.supervisor_model_policy: inherit`를 사용해 `.codex/config.toml`에 주 세션 model/effort를 쓰지 않고 사용자의 Codex 기본값을 상속한다. 정책 필드가 없는 기존 프로젝트의 markerless root 설정은 보존 우선으로 이행하며, `auto quality supervisor inherit|quality --apply`로 소유권을 명시할 수 있다. Codex 설정 업데이트는 사용자 소유 키 목록을 기록해 해당 assignment만 반복 업데이트 후에도 보존하고, 비모델 checksum drift가 생성 model/effort를 고정하지 않도록 한다. `auto quality ultra|balanced --apply`는 설정을 원문 보존 방식으로 저장한 뒤 현재 프로젝트의 플랫폼 하네스를 갱신하고, 일부 플랫폼 실패 시 적용 수와 정확한 재시도 명령을 출력한다.

- **디자인 시스템 문서 provider preflight** (2026-07-10): `auto design docs`가 프로젝트의 Astryx, shadcn/Radix/Tailwind, 로컬 디자인 소스를 metadata-only로 감지하고 component/template/token 문서 preflight를 Markdown 또는 JSON으로 출력한다. `auto design pack`에도 같은 provider 보고서와 setup gap을 포함하며, `design.docs_providers`로 탐지 범위를 제한할 수 있다. Frontend executor, reviewer, verifier와 Claude/Codex/Gemini 템플릿은 감지된 provider의 실제 props/import/token 문서를 먼저 확인하고, Astryx가 없는 프로젝트에는 의존성을 추가하지 않도록 동기화했다.

- **GPT-5.6 기반 Codex 품질 프로필 통합 (SPEC-CODEXQUAL-001)** (2026-07-10): Codex supervisor, managed subagent/native multi-agent, orchestra의 모델·reasoning effort 결정을 하나의 quality resolver로 통합했다. Balanced는 supervisor/orchestra와 Opus-tier worker에 `gpt-5.6-sol+xhigh`, Sonnet-tier worker에 `gpt-5.6-terra+role effort`, Haiku-tier worker에 `gpt-5.6-luna+role effort`를 사용한다. Ultra는 depth-0 supervisor/orchestra에 자동 task delegation을 포함한 Sol+`ultra`, 이미 명시적으로 배치된 managed worker에 Sol+`max`를 사용한다. `codex debug models` structured catalog로 같은 모델의 effort downgrade를 먼저 적용하고, 모델 미지원 시 `gpt-5.5`, 호환 모델 부재 시 runtime default로 관측 가능하게 fallback한다. Orchestra provider에는 `model_policy: quality|pinned` 소유권을 도입해 exact historical canonical 설정만 이행하고 custom argv와 기존 사용자 root model/effort는 보존한다. Runtime `--quality`/`--effort`는 일반 orchestra, `orchestra run`, structured SPEC review에 동일하게 적용된다.

- **Worker redline 수정 지침 전달 배선 (SPEC-REDLINE-EDIT-WIRE-001)** (2026-07-09): ADK worker가 A2A payload의 `redline_instructions`를 block ID와 정제된 수정 지침만 포함하는 untrusted JSON section으로 만들어 prompt 끝에 추가한다. 빈 항목과 digest·approval binding 필드는 제외하며, carrier가 없으면 기존 prompt를 그대로 유지한다. 회귀 테스트는 허용 필드만 전달되고 신뢰 경계 문구가 유지되는지 검증한다.

- **라이브 실행 경로 방어 배선 (SPEC-ADK-LIVEPATH-DEFENSE-001, completed 2026-07-08)**: 설계 감사에서 발견된 "구현돼 있지만 라이브 경로가 우회하던" 3개 방어를 실제 실행 경로에 연결했다. 인터랙티브 debate/rebuttal/judge prompt builder가 subprocess 템플릿과 같은 `AUTOPUS_PART_<hex>-BEGIN/END` fence와 SECURITY NOTE를 적용해 forged header/ignore-instruction payload를 데이터 영역 안에 가둔다. wired `pkg/worker/parallel.WorktreeManager.Create`는 `refs.lock`/`packed-refs.lock`류 shared-lock 실패에 base 3s, factor 2, 최대 3회 retry를 수행하고, dead-in-production `pkg/pipeline.WorktreeManager` private retry duplicate는 제거하되 `NewWorktreeManager`/`Create`/`Remove`/`ActiveCount` public API는 보존했다. 신규 `pkg/experiment.Loop`와 `auto experiment run`은 `MaxIterations`, `CircuitBreakerN`, context cancellation, `ExperimentTimeout`을 in-process hard stop으로 강제하며 `stop_reason`/`total_iterations`를 출력한다. 추가 hardening으로 `pkg/worker/taskid`가 worktree task ID를 branch/path 생성 전에 검증하고, metric command validator가 `&` background operator를 거부한다. 검증: `go test ./pkg/orchestra/... ./pkg/worker/... ./pkg/pipeline/... ./pkg/experiment/... ./internal/cli/...` PASS, `auto spec validate .autopus/specs/SPEC-ADK-LIVEPATH-DEFENSE-001 --strict` PASS, review PASS. Completion Verdict: Outcome Lock satisfied, mandatory 12/12, Must acceptance 7/7, Completion Debt none.

- **route_team 안정성을 manual pipeline 수준으로 격상 (SPEC-HARNESS-WORKFLOW-STABILITY-001)** (2026-06-29): claude-code에서 `auto workflow doctor` 통과 시 `/auto go --team`이 서빙하는 결정적 `route_team` substrate의 네 안정성 격차를 닫음 — ①`gate_build_test` 실패가 즉시 abort가 아니라 `MaxRetry=3`로 bounded RALF remediation + no-progress circuit-break(`pkg/workflow/remediation.go::RunGateRemediation`, `AbortReason="circuit_break_no_progress"`), ②결정적 85% coverage gate(`[NEW] pkg/workflow/coverage_gate.go::EvaluateCoverageGate` — exit-code-only `CommandRunner`로는 stdout을 못 얻으므로 `CoverageRunner.RunOutput` stdout seam 신설, `GateResult`/`VerdictSourceExitCode` 재사용), ③security>code-quality review barrier(`RunReviewBarrier`/`ConsolidateReviewVerdict`, security FAIL → `Barrier=true`/`Reason="security_fail"`), ④phantom config key 제거 — `[NEW] pkg/config/schema_workflow.go::WorkflowConf{TeamDefault,CoverageThreshold}`를 `HarnessConfig`에 추가하고 `DefaultFullConfig` 기본 `TeamDefault=true`(현행 동작 보존) + `applyMissingDefaults` Load-path backfill(섹션 부재/부분 시 zero-value false 금지) + `Validate`/`ParseSchema` 0..100 범위 검증(named error). 안정성은 prose가 아니라 **실 dispatcher 재진입점**으로 보장: `deriveTeamWorkflowJS`를 multi-segment(A planning~gate / B annotation~testing / C review / D release_hygiene)로 확장해 coverage gate가 testing↔review 사이, review barrier가 review↔release_hygiene 사이에 결정적으로 끼어듦(`SEGMENT==='C'`/`'D'` guard). 결정 로직은 LLM-free 순수 Go 함수가 source of truth이고 JS는 verdict 없는 boundary marker 유지. dispatcher contract(`content/skills/harness-workflow.md`·`agent-teams.md`·`auto-router.md.tmpl`·gemini mirror) A→B→C→D 정합화, parity gate(`pkg/content/workflow_parity.go`)를 새 schema field로 fail-closed 확장. regression-0: route_a 2-segment·doctor version pin(2.1.154)·non-claude 플랫폼 불변. 검증: build/vet/gofmt clean, `go test -race -cover ./pkg/workflow/... ./pkg/config/... ./pkg/content/...` 전부 ok(89.2%/87.8%/94.4%, 전부 ≥85%), review.md PASS·6 findings(F-001~006) 전부 resolved, 모든 새 `.go` ≤300줄. **잔여(operational residual, Completion Debt 아님)**: 실 claude-code 세션 `/auto go --team` 라이브 클릭스루는 hermetic 증명 불가(로직은 S1–S12 hermetic oracle로 검증됨).

- **Autopus 기본 minimality discipline (SPEC-ADK-MINIMALITY-DISCIPLINE-001)** (2026-06-27): `@auto plan`, `@auto go`, `@auto fix`, `@auto review` guidance에 "필요한 만큼만 구현" 결정을 기본 discipline으로 배선. plan/spec-writer는 `Minimality Decision Matrix`와 신규 dependency/abstraction justification을 기록하고, go/agent-pipeline은 existing code/helper/pattern 우선 탐색과 minimum sufficient verification receipt를 요구하며, fix/debugger는 caller/shared root-cause 확인 없이는 symptom-only patch를 `revise-target`으로 남긴다. review/reviewer/shared orchestra reviewer는 `Correctness/Security Findings`와 `Complexity Findings`를 분리하고 complexity tag(`delete`, `stdlib`, `native`, `yagni`, `shrink`, `existing-helper`, `existing-dependency`)를 고정했다. `qualityloop`는 minimality reason code 반복 신호를 inactive skill/playbook candidate로 라우팅하고, `skillevolve` path policy는 generated/runtime/plugin-cache/root artifact 변형을 fail-closed로 막는다. 검증: `go test ./templates ./pkg/adapter/codex ./pkg/adapter/opencode ./pkg/adapter/gemini ./pkg/qualityloop ./pkg/skillevolve`, `go run ./cmd/auto spec validate .autopus/specs/SPEC-ADK-MINIMALITY-DISCIPLINE-001 --strict`.

- **Homebrew tap 자동 배포 활성화 (goreleaser `brews`)** (2026-06-22): 릴리즈 시 `auto` CLI formula를 `Insajin/homebrew-autopus` tap repo로 자동 publish하도록 배선. tap repo 신규 생성(public), `.goreleaser.yaml`의 `brews` 블록 활성화(`name: auto`·`repository: Insajin/homebrew-autopus@main`·`directory: Formula`·`install bin.install "auto"`·`test auto version`), `release.yaml`가 cross-repo 푸시 토큰 `HOMEBREW_TAP_TOKEN`을 goreleaser에 전달. `goreleaser check` 통과(brews는 향후 goreleaser v3에서 `homebrew_casks`로 마이그 필요 — deprecation 경고만, v2에서 동작). ⚠️**운영 선행조건**: tap repo에 contents:write 권한을 가진 PAT/fine-grained 토큰을 autopus-adk 저장소 시크릿 `HOMEBREW_TAP_TOKEN`으로 등록해야 다음 릴리즈에서 formula가 publish됨(기본 `GITHUB_TOKEN`은 동일 repo만 접근). 등록 후 `brew install Insajin/autopus/auto` 사용 가능.

- **route_team executor file-ownership 하드 강제 — `auto workflow merge --ownership` (SPEC-HARNESS-WORKFLOW-FIDELITY-001 improvement)** (2026-06-22): executor coordination을 확률적 prompt 유도에서 merge-time 하드 보장으로 격상. 생성된 segment A가 planner 산출(`plan`)을 **return**하고(생성기: top-level `let plan`+`return { plan }`), 디스패처가 이를 임시 JSON으로 persist해 `auto workflow merge --run <id> --ownership <plan.json>`로 전달한다. merge는 각 worktree를 수행한 task에 **1:1 global best-overlap 배정**(`assignWorktreesToTasks`: 최고 overlap pair부터 greedy 점유 → 두 worktree가 한 task를 공유하지 않음, 어긋난 executor도 실제 수행 task로 강제됨)하고 **그 task 소유 파일만** 머지한다. 소유 밖 파일(executor가 다른 task 파일로 overreach)은 `skipped_out_of_scope`로 보고하고 복사하지 않아 executor 오버랩 충돌을 원천 제거. planner 경로 정규화(absolute/prefixed LLM 경로 ↔ repo-relative suffix 매칭, `ownsFile`). `--ownership` 미제공 시 기존 conflict-skip 폴백 유지. `pkg/workflow/merge_ownership.go`(`TaskOwnership`·`ParsePlanOwnership`{tasks}/{plan.tasks}·`ownsFile`·`assignWorktreesToTasks`) + merge.go split(discovery → `merge_discover.go`로 300줄 한계 유지). 회귀 테스트 `merge_ownership_test.go`(parse·suffix-match·강제 overlap 시 stray 드롭+owner 콘텐츠 우선). ⭐**실 런타임 검증**: greeting(impl+test) SPEC로 segment A → planner가 **1 task로 그룹화**(coordination fix 실증, 설명에 "must never be split across worktrees") → segment A가 `{plan}` return(seam 실증) → 디스패처가 plan.json 기록 → `merge --ownership`이 owned 파일 둘 다 머지(exit 0). 게이트 green: build/vet/gofmt·`-race`(pkg/workflow)·golangci 0 issues·file-size≤300.

- **`auto workflow merge` — route_team executor worktree consolidation (SPEC-HARNESS-WORKFLOW-FIDELITY-001 live e2e 후속)** (2026-06-22): FIDELITY-001 executor 종단 e2e가 substrate-level blocker를 드러냄 — route_team 병렬 executor는 Workflow 런타임 `isolation:'worktree'`로 각자 격리 worktree(`.claude/worktrees/wf_<runid>-<N>`)에서 작업하고 **uncommitted 변경을 남기는데**(commit 아님, 실 런타임 실측), 이를 merge하는 단계가 없어 executor 산출이 orphan → `auto workflow gate`가 무변경 main tree를 vacuous pass → route_team 종단 비기능이었다. **신규 결정적 Go 단계** `auto workflow merge --run <runid> [--working-dir]`: runID에 속한 worktree를 `git worktree list --porcelain`으로 열거(`<workingDir>/.claude/worktrees/` 봉쇄), 각 worktree의 `git status --porcelain` 변경 파일을 workingDir로 복사(file-ownership 충돌은 감지·skip·report), `git add` 스테이징 후 worktree+브랜치 정리. `pkg/workflow/merge.go`(+`merge_copy.go`·`merge_links_{unix,other}.go`, GitOutputRunner stdout seam, `pkg/workflow`→`internal/cli` 미import 경계 유지) + `internal/cli/workflow_merge.go`. 디스패처 계약(`content/skills/harness-workflow.md ### Segmented Dispatch Contract`)을 **5-step**으로 갱신: launch A → **merge** → gate → launch B → hygiene. ⭐**실 런타임 live 통합 검증**: 2 executor가 격리 worktree에 disjoint 파일 생성 → merge가 둘 다 consolidate+stage+worktree 제거(exit 0). ⭐⭐**security-auditor 3-라운드 적대 감사로 실 취약점 4건 발견·수정·재실측 PASS**: H-2(과다선택 데이터손실 — `.claude/worktrees/` EvalSymlinks 봉쇄로 main/외부 worktree 제외), M-1(심링크 dst 탈출/비원자 쓰기 — `ensureWithin` EvalSymlinks 조상 해석 + temp+rename), H-1(심링크 src exfil — Lstat 거부 + `O_NOFOLLOW`), H-1-RESIDUAL(**하드링크 src exfil**, 실측 비밀 유출 확인 → `Nlink>1` 수집스킵 + fd `in.Stat()` 백스톱 이중방어; build-tagged unix/non-unix). 회귀 테스트 `pkg/workflow/merge_security_test.go`(symlink/hardlink/containment/deleted) + `merge_test.go`(disjoint/conflict/traversal/unrelated/empty). 최종 감사 Critical/High 0. 게이트 green: build/vet/gofmt·`-race`(pkg/workflow·internal/cli)·file-size≤300. 잔여 L-2(특수문자 파일명 무성 누락, 비차단). 이로써 FIDELITY-001 worktree-merge 종단 blocker 해소(남은 것은 단일 실 SPEC chained run).

### Fixed

- **기존 Codex 프로젝트의 모델 상속 이행** (2026-07-11): `auto update`가 Autopus에서 생성한 뒤 수정되지 않은 과거 `.codex/config.toml`의 `gpt-5.5+xhigh` 설정을 감지하면 `quality.supervisor_model_policy: inherit`를 명시하고 프로젝트의 model/effort override를 제거한다. 생성 헤더, manifest merge 정책, 전체 파일 checksum, 과거 관리 tuple이 모두 일치할 때만 자동 이행하며, 사용자 marker, checksum drift, custom tuple은 그대로 보존한다. 같은 릴리스에서 잘못 `pinned`로 기록한 정확한 v0.50.66 Codex orchestra provider는 `quality`로 복구하되 near-match와 명시적 최신 정책은 변경하지 않는다. `auto update --plan`은 쓰기 없이 예정된 이행을 표시한다. Codex 갱신이 실패하면 supervisor policy를 원복하고, workspace 후속 target이 실패하면 앞선 target과 현재 target의 generated transaction 및 원래 `autopus.yaml`을 함께 복원한다. `auto doctor`는 실제 merge와 같은 키 단위 ownership 규칙으로 legacy shadowing, 소유권이 모호한 설정, 적용되지 않은 explicit `inherit`를 읽기 전용 경고로 보고한다.

- **플랫폼별 스킬 제외 출력 명확화** (2026-07-11): Codex와 Gemini 하네스 업데이트에서 Claude 전용 스킬을 오류처럼 보이는 `incompatible`로 표시하던 문구를 `platform-skipped`로 바꾸고, 제외된 스킬 이름을 함께 표시한다. 플랫폼별 스킬 생성 동작은 그대로 유지한다.

- **Go 표준 라이브러리 TLS 취약점 대응** (2026-07-10): toolchain과 Security workflow를 Go `1.26.5`로 올려 `crypto/tls`의 Encrypted Client Hello privacy leak인 `GO-2026-5856`을 해소했다. Security Scan이 patch version을 명시적으로 설치하므로 runner의 `1.26` 해석이나 캐시 상태와 관계없이 수정된 표준 라이브러리로 `govulncheck`와 릴리즈 gate를 실행한다.

- **route_team executor coordination — planner가 상호의존 파일을 분리해 발생하던 merge conflict 예방 (SPEC-HARNESS-WORKFLOW-FIDELITY-001 chained run 발견)** (2026-06-22): chained run 관측 — planner가 impl(`greeting.go`)과 그 test(`greeting_test.go`)를 별도 task로 분리하면, test task의 executor가 격리 worktree에서 컴파일을 위해 impl을 재생성→두 executor가 같은 파일 소유→merge가 conflict로 skip→build 불가(fail-fast, 안전하나 run 재시도 필요). 근본 원인은 isolated 병렬 실행에 맞지 않는 task 분해. **수정**(`pkg/content/workflow_generate_team.go`): planner 프롬프트를 "병렬 isolated-worktree 실행용 disjoint task 분해 + 컴파일 상호의존 파일(impl+test, type+소비자)은 한 task로 그룹화·절대 분리 금지" 제약으로 enrich + executor 프롬프트에 "배정된 files만 소유/생성, 그 외 파일은 병렬 executor 소유라 손대지 말 것" 가드. **planner-only probe 실증**: 동일 impl+test SPEC→taskCount=1, greeting.go+greeting_test.go가 같은 task(grouped), planner가 "상호의존→단일 executor 소유 필수" 명시 추론. merge 무변경(conflict 정책 그대로 안전망).

- **`auto workflow merge`가 새 디렉터리 내 untracked 파일을 누락 (SPEC-HARNESS-WORKFLOW-FIDELITY-001 chained e2e 발견)** (2026-06-22): 단일 실 SPEC chained run(segment A→merge→gate→segment B) 중 발견. `changedFiles`가 `git status --porcelain`(기본)을 써서 **새 untracked 디렉터리를 `?? dir/` 한 줄로 collapse** → 코드가 이를 디렉터리(non-regular)로 보고 skip → executor가 **새 패키지 디렉터리**(흔한 경우: `pkg/greeting/`)에 만든 파일이 전부 merge에서 누락되고 worktree는 그대로 제거되어 산출 유실. **수정**: `git status --porcelain --untracked-files=all`로 중첩 파일을 개별 열거. 실-git 회귀 테스트 `pkg/workflow/merge_realgit_test.go::TestMerge_RealGit_NewDirectoryEnumerated`(temp repo에 새 디렉터리+중첩 파일 생성→개별 열거 단언; fake-runner 테스트는 CLI 플래그 누락을 못 잡으므로 실 git 필요)로 잠금. flat-file 단위/통합 테스트가 가렸던 버그 — 실 SPEC chained run(새 패키지 생성)이 적발.

- **생성 워크플로 JS의 `args` 전파 버그 — 실 런타임은 args를 JSON 문자열로 전달 (SPEC-HARNESS-WORKFLOW-FIDELITY-001 live e2e / SPEC-HARNESS-WORKFLOW-RUNTIME-001 후속)** (2026-06-22): FIDELITY-001 live e2e(실 Claude Code Workflow 런타임 v2.1.174 디스패치) 중 발견. **실 런타임은 `args` 글로벌을 parsed object가 아니라 JSON STRING으로 전달한다**(`typeof args === 'string'` 실측). 따라서 생성 JS의 `const SEGMENT = (args && args.segment) || 'A'`는 항상 'A'로 폴백(→ **segment B 영영 미실행 = segmented dispatch 무력화**)하고 `const ctx = args`는 `.spec` 부재로 빈 컨텍스트가 됐다. RUNTIME-001의 "launch PROVEN"은 args 없이 돌린 0-agent 실행이라 이 버그가 잠복(launch는 됐으나 args 실전 전달은 0회). **수정**: 두 생성기(`pkg/content/workflow_generate.go::deriveWorkflowJS`(route_a)·`workflow_generate_team.go::deriveTeamWorkflowJS`(route_team)) 공유 preamble에 `const ARGV = (typeof args === 'string') ? (args ? JSON.parse(args) : {}) : (args || {})` 정규화(공유 상수 `workflowArgvNormalizeJS`)를 추가하고 `ctx`/`RT`/`SEGMENT`를 ARGV에서 읽도록 변경(string·object·undefined 모두 수용). **재검증**: 수정 전 sentinel `segment:'Z'`는 planner를 잘못 spawn(SEGMENT 버그 'A')했으나 수정 후 동일 sentinel은 0 agents(SEGMENT 정상 'Z') → fix 실증. 추가로 실 planner agent(agentType:'planner')에 `schema: PLAN_SCHEMA` 디스패치 → 검증된 `{tasks:[{id,description,files}]}` 2-task 반환(file ownership 비충돌) 실증. `launch-contract` 오라클에 ARGV 정규화 단언 추가, route_a/route_team `.tmpl` 재생성(route_a golden 동반 갱신). 게이트 green: build/vet/gofmt·`-race`(content·workflow). 잔여: executor parallel/worktree 실 코드-write fan-out + 디스패처 segment-gate 종단 루프(파괴적·operational Completion Debt).

### Added

- **route_team 생성 JS를 충실한 전문 에이전트 팀 디스패치로 격상 (SPEC-HARNESS-WORKFLOW-FIDELITY-001, status: implemented)** (2026-06-22): `SPEC-HARNESS-WORKFLOW-RUNTIME-001`(launch 정합)의 후속. 생성 `route_team.workflow.js`는 launch는 되지만 디스패치가 thin skeleton(`agent(\`Execute <role> agent for spec ...\`, {model,effort})` — agentType 부재·얇은 프롬프트·index-only fan-out)이라 Route A 서브에이전트 파이프라인보다 충실도가 낮아 default-on 기본값으로 부적합했다. **격상**(`pkg/content/workflow_generate_team.go::deriveTeamWorkflowJS`): (1) phase별 등록된 `agentType` emit(planning→`planner`, test_scaffold→`tester`, implementation→`executor`, annotation→`annotator`, testing→`tester`, review→`reviewer`+`security-auditor`) → generic Workflow 에이전트 대신 전문 subagent 시스템 프롬프트(TRUST-5/TDD/OWASP/@AX) 적용; (2) planning이 inline `PLAN_SCHEMA`(id/description/files)로 structured-output 캡처(`const plan = await agent(..., {agentType:'planner', schema: PLAN_SCHEMA, ...})`); (3) implementation fan-out이 `plan.tasks`를 task별로 thread(`min(plan.tasks.length, fan_out_cap≤5)`, task id/description/file-ownership 프롬프트)하여 `parallel(executors)` + `isolation:'worktree'`로 실행; (4) task-focused 프롬프트 enrichment(bare "Execute <role> agent" skeleton 제거). ⭐**실 Workflow 런타임 API 정합(correctness)**: baseline의 `parallel(...executors)`(이미 호출된 promise를 spread)는 실 계약 `parallel(thunks: Array<() => Promise>)`과 어긋나 런타임 crash했을 형태 → `executors.push(() => agent(...))` thunk + `parallel(executors)` 배열로 수정. **0-서브에이전트 probe를 실 Workflow 런타임에 디스패치해 경험적 확인**(`parallel([()=>...])`→결과 배열·`parallel([])`→clean no-op·0 agents·25ms·에러 0). ⭐**F-001 fail-fast(feasibility)**: 생성 JS가 `parallel`/`isolation`을 hard-require하므로 `pkg/workflow/doctor.go`에서 둘을 AdvisoryPrimitives→**RequiredPrimitives 승격** — 미지원 런타임은 doctor fail → 안전한 Route A 폴백(launch 후 mid-crash 방지). liveProber는 primitive별 동일 `present` 반환이라 실 경로 불변. ⭐**F-002 degenerate floor(completeness)**: 빈/실패 `plan.tasks`의 zero-executor silent no-op → 전체-SPEC 단일 fallback executor floor. ⭐**F-004**: review synthesis/vote 호출 모두 `agentType:'reviewer'`(count==2)·audit는 `security-auditor`(count==1) 일관성 잠금. 신규 hermetic 오라클 `pkg/content/workflow_fidelity_contract_test.go`(`agentType`/`schema: PLAN_SCHEMA`/`parallel(`/`push(() => agent(`/`isolation:'worktree'`/`Math.min`+`plan.tasks`/floor 단언 + skeleton-prompt 음성 단언) + `workflow_generate_team_test.go`/`workflow_launch_contract_test.go` faithful 단언 갱신. **trust boundary**: planner task description(untrusted LLM 산출)은 런타임 `plan.tasks[i]` 데이터로만 소비(생성 텍스트 미보간), JS-injection whitelist(phase-id/model/effort/result_type) 불변, agentType는 고정 Go 리터럴. **회귀 0**: route_a 생성 표면 byte-unchanged(`TestS19_RouteARegressionGolden`)·비-claude `--team` 불변. 게이트 green: build/vet/gofmt/`-race`(content·workflow·cli)·file-size≤300·`.tmpl` 재생성 byte-stable·idempotent. SPEC review judge(claude) PASS(critical 0/security 0/major 1=F-001) + 후속 reviewer APPROVE(0 blocker) + security-auditor PASS(0 Critical/High/Medium). **Completion Debt(잔여)**: route_team 실 multi-agent real-LLM 종단 실행(specialized agentType spawn + task-threaded parallel/worktree 실 honor)은 hermetic 불가한 operational 잔여 → `implemented` 유지(운영 검증 후 completed 승격).

- **생성 워크플로 JS를 실제 Workflow 런타임 API에 정합 + segmented dispatch (SPEC-HARNESS-WORKFLOW-RUNTIME-001, status: implemented)** (2026-06-22): `SPEC-HARNESS-WORKFLOW-001`(route_a)·`SPEC-HARNESS-WORKFLOW-TEAM-001`(route_team)이 생성한 `.claude/workflows/route_{a,team}.workflow.js`는 실제 Claude Code Workflow 런타임에서 **`SyntaxError: Unexpected keyword 'export'`로 launch 불가**였다(실 디스패치로 실증). 근본 원인: 생성 JS가 런타임 미지원 API(`export default async function run()` 엔트리 + `env()`·`agent.exec()`·role-only `agent('executor')`·3-인자 `phase()`)를 가정. **수정**: 두 생성기(`pkg/content/workflow_generate.go::deriveWorkflowJS`·`workflow_generate_team.go::deriveTeamWorkflowJS`)가 실 API(단일 `export const meta` + top-level 본문 + `agent(prompt, opts)`(prompt=task 문자열·`${ctx.spec}` 보간) + `args` 글로벌, `export default`/`env(`/`agent.exec(` 0)를 방출하도록 재작성. **Segmented dispatch**(결정적 게이트를 실 barrier로): 단일 `workflow({scriptPath}, args)` launch가 전 phase를 무조건 실행하므로 외부 Go 게이트가 mid-launch를 못 막는다 → 생성 JS에 `const SEGMENT = (args && args.segment) || 'A'` 가드(A = …→`gate_build_test` 마커, B = `annotation`→…→`release_hygiene` 마커)를 두고, 디스패처가 segment A launch → `auto workflow gate`(exit-code) hard barrier → verdict=pass일 때만 segment B launch → `auto check --hygiene --arch --quiet --staged`. 게이트 phase는 `phase(id)+log()` 경계 마커이며 실행은 JS 밖 Go(`verdict_source: exit_code` 보존, "JS는 sequencing만" 경계 유지). 품질 binding 전달 채널 `env`→`args.quality`. 신규 `pkg/content/workflow_launch_contract_test.go`(S1/S2/S3·S11) anti-theater 오라클: 첫 토큰 `export const meta`·`export` 1개·`export default`/`env(`/`agent.exec(` 0·SEGMENT preamble·segment A 마지막 phase=`gate_build_test`·segment B 첫 phase=`annotation`. ⭐**보안(security-auditor V-1, High proven JS-injection)**: `verdict_source`(`PhaseDef.ResultType`)가 model/effort/phase-id와 달리 parse 경계 미검증 → newline이 생성 JS `//` 주석을 종료시켜 실행문 emit·parity 통과 → `pkg/workflow/schema_validate.go::isSafeResultType`(whitelist `""|"exit_code"`)를 `ParseSchema`에서 강제 + 회귀 `TestParseSchema_RejectsUnsafeResultType`. ⭐⭐**operational launch PROVEN**: 재생성 route_a를 실 Workflow 런타임 디스패치 → 완료(0 agents·SyntaxError 0); route_team은 구조동치 + hermetic contract test. 디스패처 계약 docs(`content/skills/harness-workflow.md`·`agent-teams.md`·`templates/claude/commands/auto-router.md.tmpl`)에 2-segment + args `{spec, workingDir, quality, segment}` 명세. 게이트 green: build/vet/gofmt/`-race`(content·workflow·cli)·file-size≤300. reviewer APPROVE(blocking 봉합)·security PASS(V-1 수정 후). 잔여: route_team 실 멀티에이전트 end-to-end 1회(operational, 본 commit 범위 밖).

- **`--team`을 claude-code 결정적 Workflow 기반층으로 대체 (SPEC-HARNESS-WORKFLOW-TEAM-001, status: implemented)** (2026-06-21): 부모 `SPEC-HARNESS-WORKFLOW-001`(route_a, 비-team `--workflow`)에 HARD 의존하며, claude-code 플랫폼에 한해 `/auto go --team`의 "parallel multi-agent" 의도를 **결정적 team Workflow 실행 기반층**(`route_team`)으로 해소한다. 정본(SoT) = `content/workflows/route_team.{md,schema.json}` 2파일(route_a를 재정의하지 않는 별도 8-phase 집합: planning → test_scaffold → implementation(병렬/worktree) → gate_build_test(Gate 2, exit-code) → annotation → testing → review(reviewer+security_auditor) → release_hygiene), JS는 `templates/claude/workflows/route_team.workflow.js.tmpl` → 설치 시 `.claude/workflows/route_team.workflow.js`(편집금지 generated-surface, manifest 파생). 품질 모드는 **모델 tier(기존 `pkg/cost/pricing.go::ModelForAgent`)·effort(기존 `internal/cli/effort_resolve.go::ResolveEffort`)·오케스트레이션 깊이(`pkg/workflow/depth.go::ResolveDepth`, bounded: MaxVerifyVotes=3/MaxFanOut=5/MaxRetry=3)** 세 축을 동시 구동한다. 아키텍처 경계 보존(`pkg/workflow`는 `internal/cli` 미import): 품질→(model,effort) 해석은 CLI 디스패치(`internal/cli/workflow_quality_binding.go`)가 수행하고 결과를 `pkg/workflow/binding.go::QualityBinding` 데이터로 주입하며, 생성 JS는 `RT = JSON.parse(env('AUTOPUS_WORKFLOW_QUALITY'))` override seam을 baseline 리터럴 fallback과 함께 읽는다(런타임 우선). model/effort 문자열은 생성 JS에 보간되므로 `pkg/workflow/schema_validate.go` whitelist/enum으로 parse 경계 fail-closed(JS-injection 방어). parity 게이트(`pkg/content/workflow_parity.go`)는 model/effort/depth 필드까지 확장·fail-closed. `auto workflow render --route team --quality <mode>`가 route 선택+overlay를 노출, claude adapter(`pkg/adapter/claude/claude_workflow.go`)가 route_a와 route_team JS를 함께 설치. routing/fallback taxonomy 1:1 보존(disable=`--no-workflow`/config `workflow.team_default=false` → 기존 Agent Teams; doctor fail → fail-fast Route A; 비-claude → `--team` 불변 = **회귀 0**). `--multi`는 직교(substrate 비결합). SPEC review PASS 69/69(claude·codex·gemini, F-001/F-002 suggestion resolved). sync 시점 게이트 재실행 green: `go build`/`vet`/`gofmt`/`-race`(pkg/workflow·pkg/content·internal/cli·pkg/adapter/claude·pkg/cost), file-size 전 신규 `.go` ≤144, `auto workflow doctor` overall=pass(v2.1.174), team render가 8-phase·ultra overlay(implementation model=claude-opus-4-8/effort=max·review votes=3/synthesis=true)·fan-out·env seam 노출(S16/S18/S19/S20). **Completion Debt NOT none**: live end-to-end `/auto go --team --quality ultra` 실행(claude-code Workflow 런타임이 설치된 route_team.workflow.js를 실제 실행, 실 LLM 트래픽으로 executor×N+reviewer+security_auditor 구동)은 결정적 hermetic 오라클로 불가한 operational 잔여이며 SPEC 설계상 **sync completion을 차단**한다 → status `completed` 미승격, `implemented` 유지(운영 검증 후 재-sync 필요). 부모 `SPEC-HARNESS-WORKFLOW-001`(route_a, 비-team `--workflow`)에 HARD 의존하며, claude-code 플랫폼에 한해 `/auto go --team`의 "parallel multi-agent" 의도를 **결정적 team Workflow 실행 기반층**(`route_team`)으로 해소한다. 정본(SoT) = `content/workflows/route_team.{md,schema.json}` 2파일(route_a를 재정의하지 않는 별도 8-phase 집합: planning → test_scaffold → implementation(병렬/worktree) → gate_build_test(Gate 2, exit-code) → annotation → testing → review(reviewer+security_auditor) → release_hygiene), JS는 `templates/claude/workflows/route_team.workflow.js.tmpl` → 설치 시 `.claude/workflows/route_team.workflow.js`(편집금지 generated-surface, manifest 파생). 품질 모드는 **모델 tier(기존 `pkg/cost/pricing.go::ModelForAgent`)·effort(기존 `internal/cli/effort_resolve.go::ResolveEffort`)·오케스트레이션 깊이(`pkg/workflow/depth.go::ResolveDepth`, bounded: MaxVerifyVotes=3/MaxFanOut=5/MaxRetry=3)** 세 축을 동시 구동한다. 아키텍처 경계 보존(`pkg/workflow`는 `internal/cli` 미import): 품질→(model,effort) 해석은 CLI 디스패치(`internal/cli/workflow_quality_binding.go`)가 수행하고 결과를 `pkg/workflow/binding.go::QualityBinding` 데이터로 주입하며, 생성 JS는 `RT = JSON.parse(env('AUTOPUS_WORKFLOW_QUALITY'))` override seam을 baseline 리터럴 fallback과 함께 읽는다(런타임 우선). model/effort 문자열은 생성 JS에 보간되므로 `pkg/workflow/schema_validate.go` whitelist/enum으로 parse 경계 fail-closed(JS-injection 방어). parity 게이트(`pkg/content/workflow_parity.go`)는 model/effort/depth 필드까지 확장·fail-closed. `auto workflow render --route team --quality <mode>`가 route 선택+overlay를 노출, claude adapter(`pkg/adapter/claude/claude_workflow.go`)가 route_a와 route_team JS를 함께 설치. routing/fallback taxonomy 1:1 보존(disable=`--no-workflow`/config `workflow.team_default=false` → 기존 Agent Teams; doctor fail → fail-fast Route A; 비-claude → `--team` 불변 = **회귀 0**). `--multi`는 직교(substrate 비결합). SPEC review PASS 69/69(claude·codex·gemini, F-001/F-002 suggestion resolved). sync 시점 게이트 재실행 green: `go build`/`vet`/`gofmt`/`-race`(pkg/workflow·pkg/content·internal/cli·pkg/adapter/claude·pkg/cost), file-size 전 신규 `.go` ≤144, `auto workflow doctor` overall=pass(v2.1.174), team render가 8-phase·ultra overlay(implementation model=claude-opus-4-8/effort=max·review votes=3/synthesis=true)·fan-out·env seam 노출(S16/S18/S19/S20). **Completion Debt NOT none**: live end-to-end `/auto go --team --quality ultra` 실행(claude-code Workflow 런타임이 설치된 route_team.workflow.js를 실제 실행, 실 LLM 트래픽으로 executor×N+reviewer+security_auditor 구동)은 결정적 hermetic 오라클로 불가한 operational 잔여이며 SPEC 설계상 **sync completion을 차단**한다 → status `completed` 미승격, `implemented` 유지(운영 검증 후 재-sync 필요).

- **결정적 `--workflow` opt-in 라우트 기반층 (SPEC-HARNESS-WORKFLOW-001)** (2026-06-19): `/auto go` Route A를 Claude Code의 Dynamic Workflows 위에서 결정적으로 실행하는 claude-code 전용 opt-in 라우트의 안전 기반층을 추가한다. **정본(SoT) = manifest 2파일**(`content/workflows/route_a.md` 사람 계약 + `route_a.schema.json` phase-id/retry/budget/result-type 권위)이며, JS(`templates/claude/workflows/route_a.workflow.js.tmpl` → 설치 시 `.claude/workflows/route_a.workflow.js`)는 manifest에서 파생되는 **편집금지 generated-surface**다(Workflow 저작 API가 무계약 내부 프리미티브라 고정 JS 정본 핀을 기각). `pkg/content/workflow_generate.go`가 정본에서 JS를 파생하고 `workflow_parity.go` parity 게이트가 md↔schema↔generated-js의 phase-id/retry/budget/result-type 집합 불일치를 **fail-closed**(exit≠0 + diverging 원소 보고, JS 미기록)로 차단한다. 신규 `pkg/workflow` 패키지: `doctor.go`(capability gate — Primary 프리미티브 agent/schema/phase만 hard-gate, parallel/isolation/budget/model-override는 advisory 비게이팅) + `doctor_version.go`(claude-code >= 2.1.154 핀) + `gate.go`(deterministic Gate — injectable `CommandRunner` seam이 build/test exit-code로 `{verdict, verdict_source:"exit_code", build_exit, test_exit}` 판정, LLM·PhaseBackend 비의존) + `render.go`(dry-run 렌더 + `pkg/promptlayer` 재사용 prompt-manifest 해시) + `fallback.go`(fallback taxonomy 전수 분류 — fail-fast/fail-closed/resumable/explicit, silent 금지) + `drift_gate.go`(release hygiene 종단 — generated-surface drift + `auto check --lore --message` + `auto check --arch --staged` 300줄 차단). `internal/cli/workflow.go`(+`workflow_gate.go`/`workflow_render.go`)가 `auto workflow doctor`/`render`/`gate` 커맨드를 제공하며 `gate`는 workflow JS→Go exit-code bridge다. 4-phase 결정적 실행: Planning → Implementation(worktree 변형은 기존 `pkg/pipeline.WorktreeManager`/`WorktreeSlotCap`=Go 소유, JS는 시퀀싱만) → deterministic Gate(exit-code) → release hygiene. 비-claude(codex/gemini/opencode)는 workflow JS·`--workflow` 라우트·harness-workflow 스킬 **0건으로 회귀**(skill_catalog_policy `claudeOnlySkillSet`으로 claude-scoped 설치) 후 Route A fail-fast 폴백. Outcome Lock satisfied · mandatory 12/12(REQ-001~012) · Must acceptance 14/14(S1-S11·S13·S16 hermetic green, S15 operational evidence) · Completion Debt none. sibling `SPEC-HARNESS-WORKFLOW-GATE-002`(결정적 게이트 엔진)는 Primary 비의존 approved. `go build`/`vet`/`gofmt`/`-race`(pkg/workflow·pkg/content·pkg/adapter·internal/cli) green, golangci-lint 0 issues, file-size 전 .go ≤300.

- **QAMESH-first QA policy and Codex QuestionGate transport** (2026-06-18): `auto qa full` now emits an explicit `qa_policy` payload and text summary that frames QAMESH as the project QA orchestration layer while treating Playwright/browser runners as Journey Pack adapters, not competing QA modes. Generated QA starters and Codex/Claude/Gemini testing guidance now ask for project, execution, environment/origin, credentials, mobile/cloud, or canary authority instead of asking users to choose between QAMESH and Playwright. Codex router, idea/plan/PRD clarification guidance now treats `request_user_input` as the preferred interactive transport whenever it is present in the active tool list, maps the same contract to Codex App Server `tool/requestUserInput`, and falls back to concise plain text only when no Codex question tool is exposed.

- **하네스 업데이트 트랜잭션 롤백 보강** (2026-06-16): `auto update --workspace`가 대상 저장소를 쓰기 전에 설정/플랫폼 preview를 선검증하고, Codex·Claude Code·Antigravity CLI·OpenCode 어댑터의 Update 경로를 공통 트랜잭션 journal 기반으로 묶어 중간 쓰기 실패 시 생성·수정·삭제된 managed surface와 manifest를 원상 복구한다. Claude/Gemini settings 후처리와 OpenCode stale surface prune도 트랜잭션 범위에 포함해 부분 업데이트와 workspace 다중 타깃 실패 전파를 막는다.

- **릴리스타임 크로스서피스 Journey 재생성 + diff 승인 게이트 (SPEC-QAMESH-011)** (2026-06-15): 사용자 호출 단일 명령 `auto qa release-readiness`를 추가해 현재 멀티서피스 코드베이스(web/desktop/mobile)를 분석→starter 템플릿 기반으로 `.autopus/qa/journeys/**` Journey Pack을 재합성→기존 팩과 구조적 필드 단위(added/changed/removed) 비교→정제된 결정적 diff(정확한 카운트·안정 정렬) 출력→**단일 승인 게이트에서 정지**(승인 전 영속/실행 0건, decline은 no-op)→승인 후에만 기존 `runCommand` exec 경로(`qarun.Execute`)로 크로스서피스 실행(present mobile 서피스에 `mobile-scripted` 레인 포함)→exit-code 파생 결정적 verdict + 정제된 `qamesh.evidence.v2` 매니페스트를 발행한다. CI/push/PR 자동트리거가 아닌 릴리스타임 user-invoked 전용(init·scheduler·hook·cron 등록 없음). 신규 패키지 `pkg/qa/regen`(analyze/synthesize/diff/redact/apply/ai-authority)·`pkg/qa/releasereadiness`(orchestrate/dispatch/execute) + CLI `internal/cli/qa_release_readiness.go`. ⭐핵심 설계: 공유 `release.ReleaseLanes()` 카탈로그를 **불변 유지**하고 release-readiness가 자체 레인셋(mobile-scripted 포함)을 합성(REQ-REG-01)·`qarun.Execute` 재사용으로 중복 실행엔진 없음. ⭐`journey.Validate`는 모바일 어댑터 정책에서만 `pass_fail_authority=="ai"`를 거부하므로 web/desktop을 커버하는 **surface-agnostic AI-authority guard**(`qa_regen_ai_authority_forbidden`)를 신설해 어떤 서피스도 AI 판정 권한을 못 갖게 한다(authoring·execution 양쪽 AI 권한 0). ⭐fail-closed: 미존재 도구는 `surface_tool_unavailable`, 부재 서피스는 `surface_absent`로 거짓 통과 금지(`exec.LookPath` 프로빙, GNU `timeout` 래퍼 금지) — reason code는 `pkg/qa/adapter` 와 dispatch 간 published 계약 상수. ⭐보안: diff·재합성 팩·증거 전부 `RedactDiff`+`AssertSafeText`로 정제 후 표시/영속, `apply.go`의 `pack.ID` 경로 traversal 하드닝(reviewer+security 수렴, reject 가드). hermetic 픽스처로 전 invariant 검증(실 디바이스/브라우저 불요). Must AC 15/15(AC-QAMESH11-001..010·012·013·014·015·016) + Should AC-011, 커버리지 89.7/93.4%, 라이브 `--approve` smoke=phase executed·verdict blocked(fail-closed 실증). GUI/mobile 행위 추출(code→flow)은 구조적 필드 diff 범위 밖 Evolution Idea, Appium 탐색(SPEC-QAMESH-009)·클라우드 디바이스랩(SPEC-QAMESH-010)은 reserved sibling. Completion Debt none.

- **모바일 로컬 실행엔진 — Maestro 스크립트 회귀 (SPEC-QAMESH-008)** (2026-06-14): planning-only였던 `mobile-readiness`를 실행 가능한 `mobile-scripted` 레인으로 종결한다(SPEC-QAMESH-006이 "future SPEC"으로 연기했던 실행 절반). `mobile.Assess`가 `ready`이면 `maestro-scripted` 팩을 `selected_adapters`/`selected_journeys`에 유지하고, 주입 가능한 `MobileDeviceRunner` seam(`mobile_lane`/`mobile_exec`/`mobile_runner`/`mobile_device`/`mobile_oracle`/`mobile_artifacts`)을 통해 기존 `runCommand` 단일 엔진으로 프로젝트-로컬 Maestro 플로우를 실행한다(중복 실행엔진 없음). 불투명 `device_ref`→런타임 핸들 해소는 프로세스 env로만 전달하고 published `device_ref`는 불투명 유지, 해소 실패는 `device_ref_unresolved`로 fail-closed; opt-in 관리형 모드는 `exec.LookPath`로 `maestro`/`adb`/`xcrun` 부재 감지(GNU `timeout` 래퍼 금지)·context 타임아웃 경계 부팅/설치를 수행하되 컴퓨티드 sha256이 `app_artifact_digest`와 일치할 때만 설치(불일치=`app_artifact_digest_mismatch`, 설치 0회). 오라클은 exit+선언 assertion으로 결정적 판정하고 `pass_fail_authority=="ai"`를 `validateMaestroPolicy`에서 거부한다(AI 권한 0). 증거는 `qamesh.evidence.v2`(surface `mobile`, 디바이스 메타, app digest, sanitized 로그, screenshot/video quarantine refs)로 발행하되 raw 미디어/서명 URL/raw 디바이스 id/비정제 경로는 `unsafe_mobile_artifact`로 최종 매니페스트 쓰기 전 차단한다. ⭐보안 HIGH 적출·봉합: 디바이스 핸들이 published `sanitized_log` stdout 본문으로 누출(maestro/adb가 `MAESTRO_DEVICE`를 echo, 게이트 regex가 `emulator-5554`·대시 UUID 미포착)→known-value `redactMobileHandle`(패턴 아닌 정확 치환, `WriteFinalManifest` 전)로 4포맷 독립 재검증. `HasAndroidSignals`/`HasIOSSignals` 감지와 리뷰 가능한 `maestro-scripted` 스타터 스캐폴드 추가. 전부 hermetic `fakeMobileDeviceRunner`로 검증(실 디바이스 불요), 실 device-exec seam은 의도적 미커버. AC-QAMESH8-001..010 + edge 012/013(Must 12/12), AC-011(Should). Appium 탐색(SPEC-QAMESH-009)·클라우드 디바이스랩(SPEC-QAMESH-010)은 sequenced sibling 로드맵.

- **SPEC 리뷰 파이프라인 정합성 보강 (SPEC-SPECREV-002)** (2026-06-12): `pkg/spec` provider 체크리스트 파서가 `N/A` 상태를 1급으로 파싱하고(`reChecklist` PASS|FAIL|N/A), self-verify가 빈 reason N/A를 fail-closed로 거부하며, review.md 렌더가 빈 reason N/A를 `reason missing` 마커로 구분 표기한다. inert였던 글로벌 `--loop` 플래그가 spec review 반복 한도 floor(5)로 실효 배선되고(`resolveSpecReviewMaxRevisions`), EARS 미인식 SHALL 라인이 `auto spec validate` warning으로 표면화되며, SPEC Load 실패가 "본문이 비어있습니다" 오진단 대신 원인 보존 메시지로 보고된다. S1~S13 oracle 회귀 잠금.

- **오케스트라·learn·worker 런타임 견고성 하드닝 (SPEC-ORCH-023)** (2026-06-12): cc21 완료 감지의 detector 에러 3중 묵살을 관측 가능(provider 로그 + ctx취소/I/O 구분 + completed=false 강제)으로 전환, learn 스토어 `UpdateReuseCount`/`Prune`의 무잠금 truncate-rewrite를 뮤텍스로 직렬화해 동시 Append 유실 race 봉인(-race oracle), 프로바이더 fast-fail/hook/prompt 패턴을 `ProviderConfig` 오버라이드로 선언화(기본값=기존 하드코딩, default-equivalence oracle), 디베이트/judge 프롬프트 참가자 출력을 라운드별 랜덤 sentinel(`AUTOPUS_PART_<hex>`) 펜스로 감싸 위조 구조 헤더 주입 무력화, reliability 영수증 영속화 실패 store당 1회 경고, unsigned 제어평면 진입 프로세스당 1회 경고(fail-open 정책 불변), surface tracker를 `~/.autopus/surfaces`(uid/0700 검증 + ref 형식 검증 + legacy read-only reap)로 하드닝.

- **플랫폼 어댑터 패리티 보강 (SPEC-PARITY-002)** (2026-06-12): Gemini/Antigravity 생성 규칙에 누락됐던 `deferred-tools`/`project-identity`/`spec-quality` 3종을 추가해 content/rules 14종 패리티를 달성하고, Gemini extended skill의 `.claude/skills/autopus/` 정규 참조를 네이티브 경로로 해소(generate/update 양 경로). `platform:` frontmatter 값을 어댑터 식별자로 정규화(`shell-portability` gemini→antigravity-cli)하고, source−exclusion 양방향 패리티 커버리지 게이트 테스트(`runCoverageGate` + synthetic probe)로 플랫폼 누락의 구조적 재발을 차단.

- **Hook-IPC headless multiprovider completion + pane orphan reaping (SPEC-ORCH-022)** (2026-06-11): Inside Claude Code (CLAUDECODE), `auto spec review` / `orchestra` now collect provider completion through the Stop-hook done-file IPC with no 0/N screen-scrape timeout, finishing SPEC-ORCH-022. The completion detector no longer gates `FileIPCDetector` behind the CC21 monitor feature flag (`resolveCompletionDetector`, `pkg/orchestra/cc21_monitor.go`): when a hook session is active it is selected first as a full-budget completion floor, so `Execute` blocks until the done file appears instead of letting a screen-poll fallback return early and race the deferred session-dir cleanup against the provider's Stop hook (the root cause of the done file landing in an already-removed directory and never being collected). `content/hooks/hook-claude-stop.sh` now writes the done signal unconditionally and guards `chmod`, so an empty assistant message can never suppress completion. Verified end-to-end in a trusted-ancestor cmux e2e (done file collected, session dir alive at Stop time, no screen-scrape fallback). Also adds killed-process pane orphan reaping (`pkg/orchestra/surface_tracker.go`): created cmux/tmux surfaces are tracked per owning orchestrator PID and reaped on the next run only when their owner process is no longer alive, so a SIGKILL/crash leak is cleaned up without ever closing a live concurrent run's panes (PID-liveness gated, with a conservative reap-later degrade on PID reuse).

- **QAMESH one-command full QA entrypoint and coverage/profile diagnostics** (2026-06-01): `auto qa full` now acts as the simple default for full project QA planning, with `--bootstrap` for safe starter generation and `--run` for explicit full gate execution. Meta workspace roots now return scored project candidates instead of writing root QA artifacts. New `auto qa coverage` summarizes latest run/release lane, journey, manifest, setup-gap, and domain-readiness coverage, and `auto qa profile check` compares Journey Pack capability requirements against `standalone/local/ci/prod` test profiles before execution. Domain-readiness starter catalogs now expand from project signals such as browser, auth, desktop, and build surfaces.

- **Interactive-pane orchestration as the default execution path, subscription-first (SPEC-ORCH-021)** (2026-05-30): The three structured orchestra entry points — `orchestra brainstorm`, structured `spec review`, and structured `orchestra run` — now default to the interactive cmux/tmux pane backend, with `-p` headless subprocess demoted to a best-effort fallback (most users run subscription sessions that only authenticate through a logged-in interactive CLI, not the `-p` API path). A real per-provider pane `ExecutionBackend` (`pkg/orchestra/pane_backend.go`, `pane_backend_collect.go`, `pane_fallback.go`) reuses the existing session-ready / `waitForCompletion` (monitor→poll, bounded) / screen-sanitize mechanisms without inventing a new detector, and degrades gracefully when completion hooks are absent. Backend selection is unified behind one shared `paneCapable(term, subprocessMode)` predicate (`pane_capable.go`) consumed by both the legacy `RunOrchestra` guard and `SelectBackend`, fixing F-001 where a non-nil `"plain"` terminal returned a fake pane backend. On pane failure the system attempts `-p` best-effort, records the executed backend in `ProviderResponse.ExecutedBackend`, and surfaces an actionable both-failed error (naming the subscription-vs-API reality and a recovery step) instead of a raw API error. Provider argv correctness: gemini subprocess delivers the prompt as the `--print` value (eliminating the `flag needs an argument: -print` crash) and drops `--print` in interactive pane mode; codex standardizes on `exec --sandbox workspace-write … --output-schema` (no deprecated `--full-auto`); and `review_gate.providers` now includes codex so it is no longer silently dropped from structured review. 16 requirements (REQ-001~016) / 22 Must acceptance scenarios (S1~S20). Verified with `go build ./...`, `go vet`, and `go test ./pkg/orchestra/... ./internal/cli/... ./pkg/config/...`.

- **Claude Opus 4.8 model adoption + ultra effort mapping fix** (2026-05-29): Default premium/ultra tier, cost pricing table, worker routing, and skill docs/templates now use `claude-opus-4-8` (pricing verified at $5/$25 input/output per MTok against official docs). Opus 4.7 pricing is retained as a still-available legacy model. Fixed `resolveUltraMode` hardcoding `opus-4-7` as the only `max`-effort branch, which silently demoted Opus 4.8 to `high` in ultra mode; 4.8 is now in the max branch with a regression test (TC3b). The harness remains `opus`/`sonnet` alias-based, so only model-ID literals changed; `probeOpus47`/`HasOpus47` are preserved as legacy 4.7-availability probes.

- **CodeOps ADK supervised delivery local contract (SPEC-CODEOPS-ADK-001)** (2026-05-25): `auto delivery` now exposes source-owned dry-run planning and strict phase-result envelope validation for PM-owned CodeOps delivery.
  - `pkg/delivery/**` — canonical workflow mode, provider modes, phase order, generated/runtime deny-list checks, dry-run plan schema, and `codeops.phase_result.v1` validation.
  - `internal/cli/delivery.go` + `internal/cli/root.go` — public `auto delivery plan|validate` namespace with JSON output and fail-closed validation errors.
  - Tests passed with `go test ./pkg/delivery ./internal/cli -run 'TestDelivery'`.

- **Codex `/goal` wrapper and native team profile support** (2026-05-25): Codex harness generation now enables `goals` alongside `multi_agent`, exposes `@auto goal` / `$auto goal` as a thin wrapper over Codex `get_goal` / `create_goal` / `update_goal` thread state, and treats `--team` as a native Codex multi-agent Lead/Builder/Guardian profile rather than a Claude Team API compatibility shim. Generated Codex/OpenCode router surfaces, AGENTS guidance, workflow skills, and regression tests now preserve goal handoff semantics without creating separate ADK-persisted goal state.

- **Workspace folder profile policy in memindex/setup guidance (SPEC-WORKSPACE-FOLDER-PROFILE-001)** (2026-05-21): `pkg/memindex` now mirrors workspace folder policy includes/excludes so human-managed project/spec docs remain indexable, `.autopus/inbox/**` remains candidate-only, sanitized learning rows remain projection-only, and generated/runtime/harness paths such as `.autopus/runtime/**`, `.autopus/qa/**`, `.autopus/context/signatures.md`, `.autopus/*-manifest.json`, `.autopus/plugins/**`, `.codex/**`, `.claude/**`, `.gemini/**`, `.opencode/**`, `.agents/plugins/**`, and `config.toml` are skipped with reason codes. Source-owned setup guidance/templates now describe ADK as an execution harness inside a Platform/Desktop-owned workspace folder profile, not the folder identity owner.

- **Visual Brief guidance for `auto idea` and `auto plan`** (2026-05-17): idea and planning guidance now ask agents to explain outcomes with an appropriate visual aid: Mermaid flowcharts for workflows/state, low-fi wireframes for UI/UX, or sequence/data-flow/command-flow sketches for CLI/API/backend work. Planner/spec-writer guidance preserves these visuals as explanation and planning context without promoting visual-only elements into requirements unless they map to the Outcome Lock or Must acceptance.

- **QAMESH readiness projection and repair handoff (SPEC-QAMESH-007)** (2026-05-16): `auto qa readiness` now validates QAMESH run/release indexes and redacted evidence manifests, rejects unsafe raw refs before rendering or prompt handoff, emits a generic `qamesh.readiness_projection.v1` model with lane status taxonomy, setup gaps, audit/evidence/feedback refs, and safe repair actions, and includes a non-Autopus fixture proving the projection is not product-bound.

- **Deep Interview clarification gate for `auto idea` (SPEC-ADK-IDEA-CLARIFY-001)** (2026-05-15): `auto idea` source guidance and Claude/Codex/Gemini/OpenCode templates now require a five-row `Clarification Ledger` before orchestra fan-out, select the single highest-gain unresolved question in interactive mode, keep `--auto` non-blocking with `assumed`/`deferred` rows, record external `deep-interview` provenance as untrusted evidence, and teach `auto plan --from-idea` / spec-writer guidance to map ledger rows into requirements, non-goals, risks, acceptance seeds, open questions, and reviewer focus.

- **QAMESH Journey Pack init flow** (2026-05-15): `auto qa init` now creates a project-local `.autopus/qa/journeys/desktop-gui-explore.yaml` starter when desktop GUI signals are detected, preserves existing packs, validates the generated pack before returning, and documents that generated Journey Packs require human review before execution.

### Changed

- **릴리스 게이트에 보안 워크플로 게이팅** (2026-06-12): `security.yml`(gitleaks+govulncheck)이 release 경로에 게이팅되지 않아 태그 푸시가 보안 검사를 우회할 수 있던 구멍을 `workflow_call` + `needs: [ci, security]`로 봉인.
- **300줄 경계 파일 선제 분할** (2026-06-12): `pkg/worker/compress/tool_pairs.go`(300)→types/prune 2파일, `pkg/qa/evidence/manifest.go`(300)→types 분리, `pkg/adapter/codex/codex_workflow_custom.go`(300)→bodies 분리. 전부 동작 불변(body byte-identical 검증).
- **테스트 커버리지 보강 80.7%→83.9% + CI 임계값 80→83 ratchet** (2026-06-12): 7개 레인 헤르메틱 테스트 보강(+약 1,250 covered 문장, 신규 테스트 70+파일·~7,900줄)으로 domainreadiness 51.7→96.0%, internal/cli 69.2→73.0%, content 84.8→94.8%, setup 85.6→89.7%, orchestra 86.4→88.2% 등 달성. CI 게이트는 실측(83.9%) 아래 0.9%p 여유의 83으로 ratchet. 85 목표 잔여 갭은 hermetic 한계(실 프로세스/PTY/임베디드 FS 에러 경로) — 인터페이스 주입 리팩토링이 선행돼야 하며 별도 작업으로 기록.

- **Project-scoped SQL migration numbering guidance** (2026-05-26): Source-owned database, executor, validator, pipeline, router, and worktree guidance now treats each owning repo's migration directory as a serialized numbering lane. New paired SQL migrations must use 6-digit zero-padded numbers, compute `max(existing)+1` inside the target directory only, keep same-stem up/down pairs, avoid parallel number reservation, and validate affected directories before deploy.

- **QA 대상 리포 자동 해석 및 workspace 문서화** (2026-05-23): `auto qa init`이 기본 실행에서 meta workspace를 감지하면 nested git repo의 Go/Node/Python/Rust/Playwright/desktop 신호를 점수화해 Journey Pack을 제품 리포에 생성합니다. `auto setup`/`auto sync` source guidance와 multi-repo 렌더링은 QA/Journey Pack 대상 리포, `auto qa init --project-dir <repo>` 명령, root `.autopus/qa/**` runtime/generated 경계를 명시하도록 갱신되었습니다.

- **`auto go` QAMESH scope budget** (2026-05-15): go-stage guidance now limits QAMESH execution to affected/fast/smoke lanes and defers full GUI/native/release matrices to explicit `auto qa ...` or `auto canary` runs.
- **QAMESH harness/project-local Journey Pack contract** (2026-05-15): `auto qa plan` and `auto qa explore --dry-run` now expose a `harness_contract` declaring ADK as the harness and project-local `.autopus/qa/journeys/**` as the owner of concrete Journey Packs. Desktop GUI signals now produce non-blocking `project_hints` on fast plans and explicit `setup_gaps` on `gui-explore` requests when the target project has not declared a GUI Journey Pack, while `gui-explore` no longer falls back to generic detected Node/Vitest/Playwright adapters.

### Fixed

- **Windows self-update GitHub API 403 진단 보강** (2026-06-18): `auto update --self`의 릴리스 확인 요청이 명시적인 GitHub REST API 헤더(`User-Agent`, `Accept`, API version)를 보내도록 수정하고, `AUTOPUS_GITHUB_TOKEN`/`GITHUB_TOKEN`/`GH_TOKEN` 기반 인증 요청을 지원한다. GitHub API가 403을 반환하면 응답 본문과 rate-limit reset 정보를 포함해 토큰 설정 후 재시도할 수 있도록 안내한다.

- **worktree gitfile 대상 update 실패** (2026-06-16): 연결 worktree처럼 `.git`이 디렉터리가 아니라 `gitdir:` 파일인 저장소에서 Codex/OpenCode fallback git hook 경로 `.git/hooks/*`를 transaction write/prune 대상으로 잡아 `lstat ... .git/hooks/pre-commit: not a directory`로 `auto update --workspace`가 실패하던 문제를 수정했다. native hook surface(`.codex/hooks.json`, OpenCode plugin)는 계속 갱신하되 root-local fallback git hook은 해당 형태에서 건너뛴다.

- **Claude Code Stop/SessionStart 훅 상대경로 실패** (2026-06-12): 설치된 settings.json의 훅 명령이 상대경로(`.claude/hooks/autopus/hook-claude-stop.sh`)라 훅 spawn cwd가 settings 루트와 다른 서브에이전트/하위 디렉토리 세션에서 매 Stop마다 "No such file or directory"로 실패했다(하루 27회 실측). 생성기(`pkg/content/hooks_completion.go`)가 `"${CLAUDE_PROJECT_DIR:-.}"/` 프리픽스로 앵커하도록 수정하고 회귀 테스트로 고정. 설치된 워크스페이스 복사본 7개는 동일 값으로 로컬 패치됨(다음 `auto update` 재생성과 일치).

- **Gemini SPEC review subprocess timeout backfill** (2026-06-04): Default `agy`/Gemini orchestra provider config now declares a 480s subprocess execution timeout, matching the structured SPEC review budget used by Claude, and in-memory orchestra config migration backfills existing `autopus.yaml` entries where `orchestra.providers.gemini.subprocess.timeout` was missing. This prevents Gemini review runs from falling back to the 240s global `orchestra.timeout_seconds` and failing at exactly 4 minutes.

- **OpenCode shared workflow skill metadata regression** (2026-05-23): `auto update` no longer lets extended `content/skills` entries overwrite `.agents/skills/auto-*` workflow skills, preventing empty `description` frontmatter such as `.agents/skills/auto-setup/SKILL.md` from being emitted and skipped by Codex/OpenCode skill loaders.

- **SPEC review issue #55 migration gap** (2026-05-20): `auto spec review` now applies orchestra provider migrations in-memory before building review providers, so existing configs with legacy Claude `--effort max` adopt `--effort high` and the 480s per-provider timeout during review. Legacy generated `context_max_lines: 500` is treated as unset for review execution so the adaptive 500/1500/3000-line context budget is not accidentally capped back to 500.

- **QAMESH profile capability resolution** (2026-05-16): `auto qa run` / `auto qa explore` now resolve required Journey Pack capabilities from the effective test profile, including project-local `autopus.yaml` profile additions. The local profile also advertises `auth-state`, and QAMESH runtime/cache/gui/feedback artifacts are ignored as generated local evidence.


## [v0.50.10] — 2026-05-20

### Added

- **Official Antigravity CLI harness surface** (2026-05-20): `antigravity-cli` now generates an Antigravity plugin-compatible `.agents/plugins/autopus/` surface with `plugin.json`, skills, rules, agents, and command mappings, while preserving the legacy `.gemini/**` compatibility surface. Generated `GEMINI.md` imports rules from the Antigravity plugin surface, and `.agents/hooks.json` uses official `PreToolUse`/`PostToolUse` hook structure with `run_command` matchers and JSON stdout wrappers.

### Fixed

- **Antigravity AGY worker invocation** (2026-05-20): worker and orchestra defaults now invoke local `agy` through supported non-interactive `--print` mode instead of obsolete Gemini CLI flags such as `--output-format`, `--resume`, and `--model`. Plain-text `agy --print` output is parsed as task results and multiline output is preserved.

## [v0.47.5] — 2026-05-12

### Added

- **Provider transport smoke diagnostics** (2026-05-12): `auto doctor --provider-smoke` now runs a bounded text transport smoke against configured spec-review providers and reports per-provider pass/warn/fail status in both terminal and JSON doctor output. The default `auto doctor` path keeps this probe skipped so routine diagnostics remain non-blocking.

- **Orphaned orchestra provider process detection** (2026-05-12): `auto doctor` now identifies orphaned headless provider commands from orchestra/spec-review runs and includes them in stale runtime process warnings and `--fix` cleanup.

- **Codex bundled browser plugin enablement** (2026-05-12): generated `.codex/config.toml` now enables `browser-use@openai-bundled` by default so frontend verification sessions can load the in-app Browser plugin without manual project setup. Codex validation now warns when that bundled browser plugin toggle is missing.

- **Executable canary CLI baseline (SPEC-CANARY-001)** (2026-05-10): `auto canary` is now a real Cobra subcommand in addition to generated workflow guidance.
  - `internal/cli/{canary,canary_helpers,canary_browser}.go` — dry-run JSON planning, root workspace build targets, `auto test run --scenario version`, `auto doctor`, URL endpoint/page checks, local frontend Playwright smoke, latest-result persistence, and PASS/WARN/FAIL summary output
  - `internal/cli/root.go` — public command registration
  - `internal/cli/canary_test.go` — dry-run JSON contract and fail-closed persistence error regression
  - `--watch` and `--compare` are accepted and reported in result metadata; active loop and commit snapshot diff remain follow-up hardening

- **Delegation Safety Rails (SPEC-ADK-SAFE-RAILS-001)** (2026-05-06): ADK-managed delegation, worktree, provider-timeout, reclaim, and hard-interrupt paths now emit bounded safety evidence instead of silently continuing.
  - `pkg/pipeline/{safety,runtime_safety,worktree_scheduler,reclaim,interrupt}.go` — shared `DegradedEvidence`, `DelegationContext`, depth-cap checks, workflow authenticity preflight, FIFO worktree slot scheduling, reclaim terminal states, and hard-interrupt evidence contracts
  - `pkg/pipeline/{engine,runner}.go` and `internal/cli/pipeline_run.go` — default subagent pipeline authenticity, delegation depth metadata, worktree slot cap decisions, and safety event collection are wired into pipeline execution
  - `pkg/orchestra/{failure_result,pipeline_execute,runner,types}.go` — failed-provider diagnostics now include timeout source, configured/elapsed duration, role, continuation status, failure class, remediation, and redacted previews
  - `pkg/worker/{worktree_safety,loop_audit,loop_exec,loop_runtime,loop_subprocess,pipeline,pipeline_phase}.go`, `pkg/worker/host/resolve.go`, `pkg/worker/parallel/semaphore.go`, and `pkg/worker/security/emergency.go` — required worktree isolation fails closed unless an explicit fallback override reason is present, worktree reclaim emits terminal audit states, and emergency stop records SIGTERM/SIGKILL evidence
  - `content/skills/{agent-pipeline,worktree-isolation}.md` and `templates/**` — source-owned Claude/Codex/Gemini/OpenCode guidance now requires `subagent_dispatch_count`, `degraded_mode`, delegation-depth metadata, worktree slot caps, and reclaim evidence
  - Acceptance coverage exercises depth-cap blocking, workflow authenticity blockers, FIFO slot scheduling, provider timeout evidence, worktree fallback refusal, reclaim sanitization, emergency stop evidence, and source-template safety wording

- **Structured Context Compression (SPEC-CONTEXT-COMPRESS-001)** (2026-05-06): phase handoff compression now preserves long-running agent context as a replayable compaction contract instead of a lossy short summary.
  - `pkg/worker/compress/{summarizer,compressor,events,pruner,tool_pairs,tool_payload}.go` — seven-section summaries (`Goal`, `Constraints`, `Progress`, `Decisions`, `Relevant Files`, `Next Steps`, `Critical Context`), summary continuity metadata, redacted-derived index eligibility, pair-aware tool call/result pruning, safe provider-payload omission, source-ref extraction, and fail-closed context-budget blockers
  - `pkg/pipeline/{engine,events}.go` and `pkg/worker/pipeline.go` — compaction events are recorded before the next phase/model handoff, and context-budget blockers abort instead of silently dropping constraints or decisions
  - `pkg/orchestra/context_compaction.go` — orchestra-side context summarization now reuses the structured compressor contract
  - Acceptance coverage now exercises schema preservation, tool-pair integrity across XML/fenced/JSON-style traces, repeated compaction continuity, redaction of secrets/local paths/provider payloads, event source-ref safety, and pipeline/worker blocker handling

- **FTS5 Decision/Quality Index (SPEC-AUTO-MEM-001)** (2026-05-06): `auto mem` now provides a local, rebuildable quality recall projection over human-managed project docs, SPEC docs, learning JSONL entries, and redacted QAMESH summaries.
  - `pkg/memindex/**` — SQLite FTS5 projection schema, source scanner, deterministic source hashes, redaction/source-root admission guards, QAMESH and learning importers, top-k search, stale/corrupt fail-closed handling, status output, and bounded prompt context rendering
  - `pkg/memindex/driver` — `modernc.org/sqlite` backed FTS5 startup probe before projection writes
  - `internal/cli/mem.go` and `internal/cli/root.go` — public `auto mem rebuild|search|context|status` command namespace with JSON envelopes
  - `internal/cli/init.go` — generated gitignore patterns now include `.autopus/runtime/` so projection files stay runtime-only
  - `pkg/worker/a2a/{heartbeat.go,heartbeat_test.go}` — resolved the stale `@AX:TODO` heartbeat branch-test note by adding non-ok response coverage and removing the annotation during sync lifecycle management

### Fixed

- **Spec review and orchestra provider failure convergence** (2026-05-12): provider-only review failures now stop the spec-review loop without burning all revision retries, empty provider output is classified separately from generic execution errors, pane stdin commands use valid pipeline grouping before `tee`, and provider timeout evidence now reports execution timeout provenance instead of pane startup timeout.

## [v0.44.0] — 2026-05-05

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
  - Existing installs are migrated in-memory when `auto spec review` resolves providers; `auto update` can still rewrite `autopus.yaml` to persist the new defaults.

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
