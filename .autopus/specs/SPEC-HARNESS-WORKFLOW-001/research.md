# SPEC-HARNESS-WORKFLOW-001 리서치

## 기존 코드 분석

검증된 기존 심볼(rg/Read 확인):

- `pkg/content/generate.go::GenerateAllTemplates(contentDir, templateDir)` — agent/skill 템플릿을 content 소스에서 파생하는 진입점. `validateName`이 path separator/traversal을 거부한다(보안 재사용 가능). workflow 파생을 한 단계로 추가할 지점.
- `cmd/generate-templates/main.go` — `content.GenerateAllTemplates("content", "templates")`를 호출하는 드라이버. workflow 파생을 별도 배선 없이 트리거.
- `content/embed.go` — `//go:embed skills/*.md agents/*.md hooks/*.sh hooks/*.md methodology/*.yaml rules/*.md statusline.sh profiles/executor/*.md`. **`workflows/*` 미포함** → glob 추가 필요([NEW] 라인).
- `templates/embed.go` — `//go:embed claude/commands/*.tmpl claude/skills/*.tmpl claude/*.tmpl claude/rules/*.tmpl codex/... gemini/... hooks/*.tmpl shared/*.tmpl`. **`claude/workflows/*.tmpl` 미포함** → glob 추가 필요([NEW] 라인).
- `pkg/adapter/claude/claude_generate.go::Generate` — `.claude/{rules,skills,commands,agents}` 디렉터리 생성·`copyContentFiles`·`renderRouterCommand`·`ManifestFromFiles`/`m.Save`. `.claude/workflows` 디렉터리·쓰기·매니페스트 등록 추가 지점. 현재 `.claude/workflows` 미생성.
- `pkg/adapter/claude/claude.go` — `adapterName = "claude-code"`, `type Adapter struct`, `NewWithRoot(root string)`.
- `pkg/adapter/parity_test.go` — `TestParity_CrossPlatformFeatures`, `classifyFile`, `countFeatures`, `parityPct`(codex agent/rules ≥95% P0 게이트). 이는 **크로스 플랫폼 feature parity**이며 workflow phase-id parity와는 별개 개념. 회귀 테스트(S3) 패턴 참조용.
- `pkg/pipeline/runner.go` — `RunConfig.WorktreeSlotCap`(기본 5, `effectiveWorktreeSlotCap()`), `ScheduleWorktreeTasksWithCap(phaseTaskIDs(phases), slotCap)`, `SequentialRunner`/`ParallelRunner`, `PhaseBackend`(`Execute(ctx, PhaseRequest)`), `EvaluateGate`. PhaseBackend는 phase-순서 stub 용도(선택)이며 build/test exit-code를 노출하지 않으므로, S8의 deterministic Gate verdict 검증은 `[NEW] pkg/workflow/gate.go`의 fake `CommandRunner`를 사용한다.
- `pkg/pipeline/worktree.go` — `type WorktreeManager`, `NewWorktreeManager()`, `Create(ctx, branch)`, `Remove(ctx, path)`, `ActiveCount()`, max-limit(기본 5). **실재 확인(rev1 correctness 보정: 이전 '부재' 주장은 오류)**. Go 경계의 repo 변형 소유자.
- `pkg/worker/parallel/worktree.go` — `type WorktreeManager`, `NewWorktreeManager(baseDir)`, `Create(taskID)`/`Remove(path,force)`/`IsClean`/`RemoveIfClean`/`List`. worker 측 task 격리 worktree 관리자(별개). Go 경계.
- `pkg/experiment/circuit.go` — `defaultCircuitBreakerThreshold = 10`, `CircuitBreaker.Record(improved bool)`(단일 bool), `IsTripped()`, `ConsecutiveNoProgress()`, `Reset()`. "3회·progress vector·다차원 AND"는 전부 [NEW] 설계 변경 → 이 SPEC 범위 아님(sibling GATE-002).
- `pkg/promptlayer/layer.go`, `pkg/promptlayer/context_scan.go` — prompt 레이어 매니페스트/컨텍스트 스캔. prompt-manifest 해시(REQ-010, stable+snapshot 해싱, ephemeral 제외) 재사용 기반.
- `pkg/orchestra/` — `backend.go`, `consensus.go`, `completion_detector.go` 등 멀티 프로바이더 오케스트레이션. 비-claude 폴백·spec review 하네스로 유지(Go 경계).
- `internal/cli/root.go` — `root.AddCommand(newDoctorCmd())`/`newSpecCmd()`/`newExperimentCmd()`/`newOrchestraCmd()` 등록 패턴. `newWorkflowCmd()` 추가 지점. **`newGoCmd()` 부재** → `/auto go`는 markdown 라우터(`auto-router.md.tmpl`)이며 Go cobra 커맨드가 아니다(feasibility 핵심).
- `internal/cli/doctor.go` 외 doctor 패밀리 — `auto workflow doctor`의 커맨드 패턴 참조.
- `pkg/techstack/policy.go::InferMode(projectDir, description)` — greenfield/brownfield 분류. 이 작업은 brownfield(+신규 외부 의존).
- `templates/claude/commands/auto-router.md.tmpl` — Route A(default, line 1053), Route B `--team`(line 1070, `TEAMS_MODE`), 멤버 불완전 시 Route A 폴백(line 1182). `--workflow` 라우트 추가 지점.
- `content/skills/agent-teams.md` — `--team` opt-in("Fallback: Run without n"). 폴백 패턴 레퍼런스.
- `content/rules/worktree-safety.md` — 동시 worktree 최대 5, GC 억제, 락 재시도. Go 경계 정합.

## Outcome Lock

- **User-visible outcome**: claude-code 사용자가 `auto workflow render --dry-run`으로 생성 workflow JS / manifest / schema / prompt-manifest 해시를 실행 없이 검토하고, `/auto go --workflow`로 최소 결정적 4-phase(Planning → Implementation → deterministic Gate(exit-code) → release hygiene)를 실행하며, 비-claude 플랫폼과 doctor 실패 시 회귀 0으로 Route A에 fail-fast 폴백한다.
- **Mandatory requirements (Primary)**: SoT = manifest 2파일(md+schema), JS는 파생 generated-surface + generate parity 게이트(드리프트 차단), `auto workflow doctor` capability gate(Primary 프리미티브만 hard-gate, 후속은 advisory), fallback taxonomy(silent 금지), Go 런타임 경계 보존, `--dry-run` 생성기, 최소 결정적 4-phase 실행(deterministic Gate=`CommandRunner` exit-code), release hygiene 종단 phase(drift+Lore+300).
- **Explicit non-goals**: 게이트 엔진(progress vector/verdict committee, T3→sibling), worktree fan-out(T5), resume 무효화(T4), Saga(AP1), budget rollover(AP2), 기본 `/auto go` 승격(K13), 비-claude를 Workflow로 끌어올리기, `--team` 재구현, 백엔드 `durable_workflow` 변경, 출력 결정성 약속.
- **Completion evidence**: 동일 manifest→동일 phase 순서(dry-run 골든 S7 + fake-`CommandRunner` replay S8), parity fail-closed(S2), 비-claude 회귀0(S3), doctor fail-fast→폴백(S4/S5)·advisory 비게이팅(S14), drift+Lore/300 차단(S6/S13), prompt-manifest 해시 결정성(S11).

## Visual Planning Brief

작업 성격이 CLI/하네스이므로 wireframe 대신 command-flow + 생성 파이프라인 다이어그램을 사용한다(상세 다이어그램은 `plan.md`의 `## Visual Planning Brief` 참조). 요지:

**정본 2파일(md+schema)→generate.go가 JS 파생(generated-surface)→parity 게이트로 드리프트 차단(fail-closed)→go:embed→claude 어댑터가 `.claude/workflows/route_a.workflow.js`(generated) 설치→doctor가 claude 가용성·Primary 프리미티브 hard-gate(미지원=Route A fail-fast 폴백)→Workflow가 4-phase를 결정적 제어 흐름으로 실행(Gate=CommandRunner exit-code)→release hygiene가 drift gate+Lore/300+sync로 종단.** dry-run render는 이 경로를 실행 없이 검토하는 우회로다.

## 설계 결정

- **왜 manifest=SoT, JS=generated인가**: Workflow 저작 API가 무계약 내부 프리미티브(아래 Technology Stack Decision)이므로 정확한 JS 시그니처에 핀하면 CC 업그레이드 시 전체 무력화 위험(CR1). 안정 계약을 우리가 소유하는 manifest로 올리고 JS는 폐기·재생성 가능한 얇은 어댑터로 강등. 대안(고정 JS 정본)은 항구성 리스크로 기각.
- **왜 doctor는 프로버 seam인가**: 버전 계약이 없어 문서 버전 체크로 안정성을 대체할 수 없다. 런타임 가용성을 프로브하되, 테스트는 fake prober로 hermetic하게(주입). 라이브 프로브는 운영 경로. doctor 구현 ≠ 게이트 통과(BS techstack §3).
- **왜 fake-`CommandRunner` replay인가**: JS는 Go에서 실행 불가. deterministic Gate verdict의 결정적 증거를 `[NEW] pkg/workflow/gate.go`의 injectable `CommandRunner`(build/test 명령→exit-code) stub으로 확보(build exit=1→verdict=fail). 기존 `pkg/pipeline.PhaseBackend`는 build/test exit-code를 노출하지 않아 gate 검증에 부적합하므로 사용하지 않는다(phase 순서는 manifest 파생으로 별도 검증). scaffold-only 회피의 핵심 hermetic 오라클이며 새 JS 런타임 의존을 만들지 않는다.
- **왜 deterministic Gate=exit-code(CommandRunner seam)인가**: LLM verdict 단독 pass/fail은 CR2(의미오류가 타입 경계 탈출). Primary는 `CommandRunner`가 반환한 실측 build/test exit-code만으로 게이트를 닫고(`verdict = build_exit==0 && test_exit==0`, `verdict_source=exit_code`), LLM verdict 합성(verdict committee, K14)은 sibling GATE-002로 분리. 마케팅 정직성: "제어 흐름 결정성 + 관측 증거"만 약속.
- **왜 Go 경계 보존인가**: 위험 repo 변형(branch 명명, slot cap, reclaim)은 worktree-safety 규칙과 결합된 Go 책임. JS가 이를 가지면 폴백·테스트 하네스가 깨진다. 기존 `WorktreeSlotCap`/`ScheduleWorktreeTasksWithCap` 재사용으로 Ease↑.

## Semantic Invariant Inventory

| ID | source clause | invariant type | affected outputs | acceptance IDs |
|----|---------------|----------------|------------------|----------------|
| INV-001 | "SoT 3분할 + generate.go parity 게이트(드리프트 차단)" (BS Outcome Lock) | generate determinism / set equality | generate-templates 출력, .js.tmpl, .claude/workflows/*.js | S1 |
| INV-002 | "phase id·retry·budget·schema 불일치 시 fail-closed" (BS Visual Brief) | parser/report set comparison, fail-closed | parity 게이트 종료 코드 + diverging 원소 | S2 |
| INV-003 | "비-claude 플랫폼은 fail-fast로 기존 Route A에 회귀 0으로 폴백" (BS Outcome Lock) | regression invariant (count=0) | codex/gemini/opencode 어댑터 산출물 | S3 |
| INV-004 | "auto workflow doctor capability gate(silent 금지); Primary 프리미티브(agent/schema/phase)만 hard-gate, 후속/Evolution(parallel/isolation/budget/model-override)은 advisory 비게이팅" (BS Mandatory; Q-COMP-07) | fail-safe gating + required/advisory 분리 | doctor 리포트 required/advisory status, 라우터 결정 | S4, S5, S12, S14 |
| INV-005 | "release hygiene = generated-surface drift gate(.claude/.codex/.gemini/.opencode/.autopus/orchestra 우발 staging 차단) + 300줄 초과 신규 소스 차단 + Lore 형식 위반 차단" (prompt AP3; REQ-012) | block-list membership + file-size limit + Lore format | drift/300/Lore 차단 목록 + 종료 코드 | S6, S13 |
| INV-006 | "최소 결정적 4-phase 실행 ... deterministic Gate verdict = CommandRunner build/test exit-code; workflow JS가 `auto workflow gate` CLI를 호출하는 JS→Go bridge로 verdict 전달; 라이브 디스패치가 4-phase 실제 실행" (prompt Q-COMP-04; REQ-011) | ordered sequence + exit-code verdict source + JS→Go bridge + live dispatch | dry-run phase 순서, fake-CommandRunner replay(S8), `auto workflow gate` CLI JSON(S16), 라이브 run journal(S15 operational) | S7, S8, S15, S16 |
| INV-007 | "fallback taxonomy(fail-fast/fail-closed/resumable/explicit, silent 금지)" (BS Mandatory) | classification totality | fallback 분류기 반환 클래스 | S9 |
| INV-008 | "Go 경계 보존(repo 변형=Go ... WorktreeSlotCap 기본5)" (prompt 아키텍처 제약3) | concurrency bound + ownership | ParallelRunner 동시성, branch/reclaim 위치 | S10 |
| INV-009 | "resume = prompt_manifest_hash(stable+snapshot 해싱, ephemeral 제외)" (BS K2; Primary는 해시 결정성만) | hash determinism | render dry-run prompt-manifest 해시 | S11 |

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| Happy path: dry-run 검토 + 4-phase 결정적 실행 + 라이브 디스패치 | Primary (REQ-010, REQ-011 / T6,T7,T10 / S7,S8,S11,S15) | covered |
| Error/recovery: 비-claude/doctor 실패 폴백 | Primary (REQ-005,REQ-007 / T5,T8 / S3,S4,S5) | covered |
| Error/recovery: parity 드리프트 fail-closed | Primary (REQ-003 / T3 / S2) | covered |
| Error/recovery: drift gate 차단 | Primary (REQ-012 / T9 / S6) | covered |
| 생성 파이프라인 경계 (SoT→JS→embed→설치) | Primary (REQ-001,REQ-002,REQ-004 / T1,T2,T4 / S1) | covered |
| CLI 표면 (--workflow / --dry-run / auto workflow doctor) | Primary (REQ-006,REQ-007,REQ-010 / T6,T8 / S4,S5,S7) | covered |
| 검증 (fallback totality, Go 경계, 해시 결정성) | Primary (REQ-008,REQ-009 / T11,T7,T10 / S9,S10,S11) | covered |
| docs/ops (opt-in 사용 가이드) | Primary (T12) | covered |
| 결정적 게이트 엔진 (progress vector/verdict committee) | sibling SPEC-HARNESS-WORKFLOW-GATE-002 (T3) | approved-sibling |
| worktree fan-out 실행 (T5) | follow-up (자동 생성 금지) | evolution-idea |
| resume 무효화 입자도 (T4) | follow-up (자동 생성 금지) | evolution-idea |
| 기본 /auto go 승격 (K13) | explicit non-goal | out-of-scope |

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| None | - | - |

근거: Outcome Lock은 단일-패스 결정적 실행 + dry-run 검토 + 폴백 회귀0 + parity + doctor + drift hygiene로 닫힌다. deterministic gate의 JS→Go 실행 경계는 `auto workflow gate` CLI bridge로 명세되어(S16 hermetic CLI 오라클 + S8 CommandRunner unit) unresolved debt가 아니다. happy-path 라이브 실행(`/auto go --workflow`→Workflow 디스패치→4-phase)은 markdown 라우터 + claude-code 전용 툴이라 hermetic Go 단위 테스트가 불가하며, S15 operational 오라클(구현 중 1회 실제 실행 + run journal 생성)로 검증한다 — 이는 Completion Debt가 아니라 operational completion evidence이고, hermetic 스위트(S1-S11, S13, S14)가 나머지를 커버한다(BS techstack 라이브 스모크가 Workflow 툴의 journal 생성 능력을 이미 실증). resume(T4)는 단일-패스 실행을 막지 않으므로 후속 Evolution이다. 기본 `/auto go` 승격(K13)은 명시적 non-goal로 sync completion을 막지 않는다.

## Evolution Ideas

These are optional improvements and do not block sync completion. SPEC ID / task ID / acceptance ID를 자동 부여하지 않는다(Q-COMP-07). 사용자 명시 요청 시에만 승격한다.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| worktree fan-out 실행 (T5) | Outcome Lock의 단일-패스 실행을 막지 않음 | 사용자가 명시적으로 요청 |
| resume 무효화 입자도 (T4) | 단일-패스 실행에 불필요, CR5 입자도 튜닝 난제 | 사용자가 명시적으로 요청 |
| Saga 2-tier 보상 (AP1) | 머지-후 보상은 별도 안전 설계 필요 | 사용자가 명시적으로 요청 |
| budget rollover + reservation (AP2) | 횡단 요구, Primary 실행에 불필요 | 사용자가 명시적으로 요청 |
| 기본 /auto go Route A 승격 (K13) | harness-bench 증거 의존(명시적 non-goal) | 벤치 증거 + 사용자 요청 |
| 모델 다운그레이드 라우터 (U1) | per-call 모델 오버라이드 미검증, net-loss 위험 | 벤치 증거 + API 지원 확인 |
| --team --workflow-preview 브리지 (U2) | 마이그 편의, Outcome Lock 밖 | 사용자 요청 |
| learning handoff→pkg/learn (U3) | scope creep | 사용자 요청 |
| /auto plan --workflow (U4) | /auto go 코어 밖 | 사용자 요청 |
| spec-review 디베이트를 Workflow로 (U5) | 동일 엔진 재사용, 코어 밖 | 사용자 요청 |
| 마이그/감사 스윕 (U6) | 재사용 가치, 코어 밖 | 사용자 요청 |
| 비주얼 로우코드 편집기 (U7) | 가장 먼 표면, 고위험 | 사용자 요청 |
| SQLite Decision/Quality FTS (U8) | 발안자 R2서 드롭 | 사용자 요청 |
| validation-first Route A (U9) | TDD 재배열, 미통합 | 사용자 요청 |
| security auditor preflight (U10) | 저위험·고가치이나 Outcome Lock 밖 | plan 시 옵션 편입 검토 |
| hard/soft gate 세분 (U11) | 게이트 강도 모델 | plan 시 편입 검토 |

## Sibling SPEC Decision

| Decision | Reason | Sibling SPEC IDs |
|----------|--------|------------------|
| approved (이 SPEC에서는 구현하지 않음) | 결정적 게이트 엔진(progress vector 서킷브레이커 + review 2-phase + verdict committee, T3)은 fake agent() golden fixture로 라이브 배선 없이 독립 개발·검증 가능 → Primary와 병렬 정당(Q-COH-03). | SPEC-HARNESS-WORKFLOW-GATE-002 |

경계: sibling 최대 1개, sibling의 sibling 금지. hard prerequisite = verdict committee 실험(CR2 의미 정확성 스파이크: 깨진 빌드에 LLM이 pass를 줘도 실측 exit-code가 override함을 입증)이 착수 전제. 이 sibling은 Primary 완료에 의존하지 않으며, Primary 또한 이 sibling 완료에 의존하지 않는다(Primary의 deterministic Gate는 exit-code 단독으로 닫힘).

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| `pkg/content/generate.go::GenerateAllTemplates`, `validateName` | existing | Read 확인 |
| `cmd/generate-templates/main.go` | existing | rg 확인(`GenerateAllTemplates("content","templates")`) |
| `content/embed.go` glob (workflows 미포함) | existing | rg 확인 |
| `templates/embed.go` glob (claude/workflows 미포함) | existing | rg 확인 |
| `pkg/adapter/claude/claude_generate.go::Generate`, `ManifestFromFiles` | existing | Read 확인 |
| `pkg/adapter/claude/claude.go::NewWithRoot`, `adapterName="claude-code"` | existing | rg 확인 |
| `pkg/adapter/parity_test.go::TestParity_CrossPlatformFeatures`, `parityPct` | existing | Read 확인(별개 개념) |
| `pkg/pipeline/runner.go::RunConfig.WorktreeSlotCap`, `ScheduleWorktreeTasksWithCap`, `PhaseBackend`, `ParallelRunner` | existing | Read 확인(기본 5) |
| `pkg/pipeline/worktree.go::WorktreeManager`(Create/Remove/ActiveCount), `pkg/worker/parallel/worktree.go::WorktreeManager` | existing | rg 확인(rev1 보정: 실재, 이전 '부재' 주장 철회) |
| `pkg/experiment/circuit.go::CircuitBreaker.Record(bool)`, `defaultCircuitBreakerThreshold=10` | existing | Read 확인(스켈레톤만, 본 SPEC 미사용) |
| `pkg/promptlayer/layer.go`, `context_scan.go` | existing | ls 확인 |
| `pkg/orchestra/` (backend.go, consensus.go) | existing | ls 확인 |
| `internal/cli/root.go::AddCommand(newDoctorCmd())` 패턴 (newGoCmd 부재) | existing | rg 확인 |
| `internal/cli/doctor.go` 패밀리 | existing | ls 확인 |
| `pkg/techstack/policy.go::InferMode` | existing | rg 확인 |
| `templates/claude/commands/auto-router.md.tmpl` Route B `--team`(line 1070) | existing | rg 확인 |
| `content/skills/agent-teams.md` `--team` opt-in/fallback | existing | rg 확인 |
| `content/workflows/route_a.md`, `route_a.schema.json` | [NEW] planned addition | content/workflows/ 디렉터리 부재 확인 |
| `templates/claude/workflows/route_a.workflow.js.tmpl` | [NEW] planned addition (generated) | 부재 확인 |
| `.claude/workflows/route_a.workflow.js` | [NEW] planned addition (generated surface) | 부재 확인 |
| `pkg/content/workflow_generate.go`, `workflow_parity.go` | [NEW] planned addition | rg `--workflow`/workflow gen 0건 |
| `pkg/adapter/claude/claude_workflow.go` | [NEW] planned addition | 부재 확인 |
| `pkg/workflow/doctor.go`, `fallback.go`, `drift_gate.go`, `render.go`, `gate.go`(`CommandRunner` seam) | [NEW] planned addition | pkg/workflow 부재 |
| `internal/cli/workflow.go::newWorkflowCmd` | [NEW] planned addition | rg 확인(미등록) |
| `content/skills/harness-workflow.md` | [NEW] planned addition | ls 확인(부재) |
| `auto workflow doctor`, `auto workflow render --dry-run`, `auto workflow gate`(JS→Go bridge), `--workflow` flag, `resumeFromRunId` | [NEW] planned addition | 워크스페이스 전체 0건(rg) |

## Reviewer Brief

- **Intended scope**: opt-in `--workflow` 라우트 기반층 — SoT = manifest 2파일(md+schema, JS는 파생 generated) + parity 게이트 + doctor capability gate(Primary 프리미티브만 hard-gate) + fallback taxonomy(비-claude 회귀0) + Go 경계 + dry-run 생성기 + 최소 결정적 4-phase 실행(Gate=CommandRunner exit-code) + release hygiene 종단 phase(drift+Lore+300). 이것이 Outcome Lock을 닫는다.
- **Explicit non-goals (리뷰어가 새 scope로 확장하지 말 것)**: 게이트 엔진(progress vector/verdict committee→sibling GATE-002), worktree fan-out(T5), resume 무효화(T4), Saga(AP1), budget rollover(AP2), 기본 `/auto go` 승격(K13). 이들은 Evolution Ideas/sibling/non-goal이며 REQUEST_CHANGES 근거가 아니다.
- **Self-verified**: Reference Discipline(existing 심볼 rg/Read 확인, [NEW] 분리), Traceability Matrix(REQ↔Task↔AC↔INV), Semantic Invariant Inventory(9건), oracle acceptance(S1-S11+S13 Must, S12/S14 Should, 구체 기대 출력), Technology Stack Decision(BS 실사 이관, CONDITIONAL PASS), Revision 1 closure(멀티프로바이더 REVISE 9건 봉합).
- **Reviewer should focus on**: correctness(generate/parity 정합), feasibility(doctor 프로버 seam·fake-`CommandRunner` replay의 hermetic 성립·`/auto go`가 markdown 라우터인 점), drift 안전(generated-surface block-list + Lore/300), 회귀 위험(비-claude 회귀0), scaffold-only 회피(REQ-011/S7/S8 실제 실행 + REQ-012/S13 release hygiene enforcement 포함), Completion Debt only.

## Technology Stack Decision

BS-HARNESS-WORKFLOW-001 §Technology Stack Decision 실사(2026-06-19 라이브 스모크+resume 포함)를 이관한다.

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|------|----------------|-------------------|-------------|------------|-----------------------|
| brownfield(+신규 외부 의존) | Claude Code Dynamic Workflows (`Workflow` 툴) | feature GA v2.1.154+; 설치 ground-truth 2.1.174; opt-in 키워드 `ultracode` v2.1.160+ | code.claude.com/docs/en/workflows.md; changelog v2.1.154~181; 현 세션 라이브 툴 스키마; 라이브 스모크/resume 실행 증거 | `--team` Agent Teams(experimental·취약·gemini 비활성); 순수 Go `pkg/pipeline` 확장(결정성은 얻으나 저널/resume 미보유) |

### 게이트 판정: CONDITIONAL PASS — 현 버전 feasibility 경험적 확정, 아키텍처 제약 부과

**확인됨**: Dynamic Workflows = 정식(GA), 실험 플래그 불요(비활성만 `CLAUDE_CODE_DISABLE_WORKFLOWS=1`). 설치 2.1.174 ≥ 2.1.154. 동시성 16/총 1000 에이전트, 세션 내 resume 캐싱(공식 문서).

**경험적 검증 (라이브 스모크 + resume, 2026-06-19) — 프리미티브 검증 매트릭스**:

| 프리미티브 | 검증 방법 | 결과 |
|---|---|---|
| `agent({schema})` JSON Schema 강제 | parallel 2-probe, additionalProperties:false | PASS `schema_enforced:true` |
| `parallel()` 팬아웃 | 2 thunk 동시 실행 | PASS `parallel_fanout:true`(6.4s) |
| `phase()`/`log()` | Probe/Verify 2 phase | PASS |
| `budget` 전역 | `budget.spent()`/`budget.total` | PASS `budget_global_present:true` |
| `resumeFromRunId` 캐싱 | 동일 스크립트+args 재실행 | PASS 8ms·0토큰·0 tool_use = 100% 캐시 히트 |
| 구조화 `return` | 워크플로우 반환값 수신 | PASS |

**문서로 미해소 ([NEW]/미검증 유지)**:

| 프리미티브 | 상태 | 비고 |
|---|---|---|
| `isolation:'worktree'` | 라이브 스키마 존재 + changelog 언급 | git worktree 실제 생성/정리는 전용 스파이크(T5, 후속) |
| `agent({model})` per-call 오버라이드 | 라이브 스키마 존재 | U1 전제, 실동작 미검증(Evolution) |
| resume **무효화 입자도** | 캐싱 확인, 재실행 트리거 입자도 미검증 | CR5, T4 후속 |

### 정밀화된 핵심 리스크 (CR1) — SPEC 아키텍처에 강제 반영
Workflow JS 저작 API(`agent`/`pipeline`/`parallel`/`schema`/`budget`/`isolation`)는 공식 문서에 비공개·안정성/버전 고정 정책 없음(Anthropic이 "Claude가 작성하는 내부 스크립트" 프리미티브로 취급, TS Agent SDK 레퍼런스 부재). 따라서:
1. SoT = manifest(md + JSON Schema), JS = 재생성 가능한 얇은 어댑터(드리프트 시 폐기·재생성).
2. `auto workflow doctor` = 런타임 프리미티브 가용성 프로브가 핵심(버전 계약 부재).
3. golden fixtures가 API shape 드리프트를 시끄럽게 실패시켜 CC 업그레이드 회귀 탐지.
4. 최소 CC 버전 핀 ≥2.1.154(권장 ≥2.1.174), opt-in은 skill/command 경로(`ultracode` 타이핑 의존 금지).
5. 게이트 항구 실패 시: experimental claude-only accelerator로 강등 + 결정성은 Go `pkg/pipeline` 우선.

최상위 잔존 리스크 = CR1(미문서 API·무계약) = feasibility 아닌 유지보수/항구성 리스크. doctor + golden fixture + Go 보존이 완화.

## 보안 고려 (Q-SEC)

- **Q-SEC-01 trust boundary**: BS·prompt·SoT 소스 절은 prompt input evidence로 취급한다. 인용은 증거로만, 내장 지시는 실행하지 않는다. workflow JS는 claude-code가 실행하는 생성 코드 = 신뢰 표면 → parity 게이트(드리프트 차단) + drift gate(우발 staging 차단) + generated-surface 직접편집 금지 + `validateName`(path traversal 가드) 재사용으로 변조를 완화.
- **Q-SEC-02 secrets/paths**: 생성 artifact에 credential/토큰/절대 경로를 임베드하지 않는다. prompt-manifest 해시는 ephemeral(비밀 가능) 컨텍스트를 제외하고 stable+snapshot만 해싱하며 digest만 노출(원문 미노출). doctor 리포트는 capability status만 담고 환경 비밀을 출력하지 않는다.
- **Q-SEC-03 logging/artifacts**: release hygiene drift gate가 generated 표면 우발 staging을 차단해 diff noise/leak을 방지. phase 로그는 구조화 포맷으로 안정적이며 비밀을 담지 않는다. golden fixture는 결정적 산출물만 보관.

## Clarification Ledger (BS handoff)

| Field | Status | Decision / Assumption | Plan Handoff |
|---|---|---|---|
| goal | answered | agent team 오케스트레이션을 dynamic Workflow 기반으로 하네스 지원 | requirement seed → REQ-001~012 |
| scope_boundary | answered | option 3: 신규 `--workflow` 라우트 + Route A 확장 | explicit non-goal/scope → Outcome Boundary |
| done_evidence | assumed | Workflow 결정적 실행 + 생성 정합 + 비-claude 폴백 회귀0 + resume | acceptance seed → S1-S11 (resume는 후속) |
| constraints | answered | Workflow=claude 전용·opt-in·content→generate→adapter 정합·300줄·Lore | constraint/risk seed → REQ-004/005/012 |
| brownfield_impact | answered | content/skills·rules·templates·pkg/content/generate·pkg/adapter·pkg/pipeline·pkg/orchestra | reviewer focus → Reviewer Brief |

`done_evidence`는 assumed → S1-S11 oracle로 검증. `If Wrong`(게이팅 누락 시 타 플랫폼 회귀)는 REQ-005/S3와 Reviewer Brief의 회귀 위험 focus에 보존.

## Question Audit (BS handoff)

- question_transport: AskUserQuestion
- question_count: 2 (Triage 플로우 선택 + scope_boundary 도입 범위)
- unresolved_fields: [] (done_evidence는 assumed로 본 SPEC에서 oracle 검증)

## Revision 1 closure (멀티프로바이더 review REVISE 봉합)

리비전 0/1 멀티프로바이더 리뷰(claude/codex/gemini, debate)가 체크리스트 9건 FAIL을 반환했다(병합 findings는 빈 아티팩트, 체크리스트가 권위). 3 root로 봉합:

| Q-* | category | 봉합 내용 | 위치 |
|-----|----------|-----------|------|
| Q-CORR-04 | correctness | "정본 3파일(md+schema+js)"→"SoT manifest 2파일 + 파생 generated JS" 용어 정정, generated-surface 경계 명확화 | spec.md L18/L21·research Outcome Lock/Visual/Reviewer Brief |
| Q-COMP-02 | completeness | REQ-012 Lore/300 enforcement에 S13 oracle 추가; REQ-011 exit-code gate에 `CommandRunner` replay 계약(S8) 부여 | acceptance S13/S8·spec REQ-011·plan T9 |
| Q-COMP-04 | completeness | deterministic exit-code gate(CommandRunner seam) + release hygiene(drift/Lore/300) 완전 명세로 Outcome Lock 닫음 | spec REQ-011/012·plan T7/T9·acceptance S8/S13 |
| Q-COMP-05 | completeness | exit-code gate oracle을 fake CommandRunner로 runnable화(S8); INV-005를 Lore/300까지 확장(S13) | research INV-005/INV-006·acceptance S8/S13 |
| Q-COMP-07 | completeness | doctor hard-gate를 Primary 프리미티브(agent/schema/phase)로 한정, parallel/isolation/budget/model-override는 advisory 비게이팅 | spec REQ-006·research INV-004·acceptance S4/S14 |
| Q-FEAS-01 | feasibility | exit-code를 PhaseBackend가 아닌 `[NEW] pkg/workflow/gate.go` CommandRunner seam으로 노출(replay 가능) | spec REQ-011·plan T7·research 기존코드분석/설계결정 |
| Q-FEAS-02 | feasibility | doctor hard gate에서 optional 프리미티브 제거(advisory)로 SoT/generated 경계와 정합 | spec REQ-006·research INV-004 |
| Q-FEAS-03 | feasibility | S1을 generate-templates + claude 어댑터 Generate 2단계로 분리(runnable); S8을 fake CommandRunner로 runnable화 | acceptance S1/S8 |
| Q-COH-02 | cohesion | exit-code gate seam + release hygiene enforcement를 Primary에 완전 명세(implicit debt 제거), Completion Debt None 유지가 정직 | spec REQ-011/012·research Completion Debt |

## Revision 2 closure (멀티프로바이더 review 재실행 REVISE 봉합)

rev1 봉합 후 재리뷰가 새 체크리스트 9건 FAIL 반환(이전 9건 해소 = 수렴 진행). 4 root로 봉합:

| Q-* | category | 봉합 내용 | 위치 |
|-----|----------|-----------|------|
| Q-CORR-01 | correctness | **사실 정정**: `WorktreeManager` 실재(pkg/pipeline/worktree.go + pkg/worker/parallel/worktree.go) rg 확인, 이전 '부재' 주장(BS judge 오류 전파) 철회 | research 기존코드분석·Reference Discipline·spec REQ-009·plan T7 |
| Q-CORR-03 | correctness | manifest 파서 계약 정의: phase-id 권위 = schema.json(JSON), md는 presence만(markdown grammar 불요) | spec REQ-001/003·research 설계결정/INV-001 |
| Q-CORR-04 | correctness | Reference Discipline의 stale WorktreeManager 주장 정정(실재 등록) | research Reference Discipline |
| Q-COMP-04 | completeness | happy-path 라이브 실행 Must 오라클 S15(operational, router→workflow 디스패치+run journal) 추가 | acceptance S15·spec REQ-011·research Completion Debt |
| Q-COMP-05 | completeness | INV-006에 라이브 dispatch 차원 + S15, manifest 파서 오라클(schema 권위) 명세 | research INV-006·acceptance S1/S15 |
| Q-FEAS-01 | feasibility | 실행 검증이 dry-run/fake-replay에 멈추지 않도록 S15 operational 실행 오라클 추가 | acceptance S15·plan T7 |
| Q-FEAS-02 | feasibility | Go 경계 분석에 실재 WorktreeManager 반영(module surface 정합) | research 기존코드분석·spec REQ-009 |
| Q-FEAS-03 | feasibility | Lore=`auto check --lore --message <msgfile>`(pending), file-size=`auto check --arch --staged`로 runnable화 | spec REQ-012·plan T9·acceptance S13 |
| Q-COH-02 | cohesion | happy-path 실행을 S15 operational evidence로 명시(hermetic 불가 정직 기록), Completion Debt None 근거 보강 | research Completion Debt·acceptance S15 |

## Revision 3 closure (멀티프로바이더 review 3차 REVISE 봉합 — 수렴)

rev2 봉합 후 재리뷰가 7건 FAIL 반환(체크리스트 9→9→7, 4 root→2 root로 수렴). 봉합:

| Q-* | category | 봉합 내용 | 위치 |
|-----|----------|-----------|------|
| Q-COMP-04·Q-COMP-05·Q-FEAS-01·Q-COH-02·Q-COMP-07 | completeness/feasibility/cohesion | **JS→Go gate bridge 정의**: workflow JS의 gate phase가 `[NEW] auto workflow gate` CLI 호출→Go `CommandRunner`가 build/test 실행→`{verdict, verdict_source:exit_code, build_exit, test_exit}` JSON 반환→JS 분기. gate.go(Go)와 workflow JS(실행 계층)의 호출 경계 명세. S16 hermetic CLI 오라클 추가. Completion Debt None 정직(bridge 명세됨) | spec REQ-011·plan T6/T7·acceptance S15/S16·research INV-006/Completion Debt |
| Q-FEAS-02 | feasibility | harness-workflow.md를 claude-scoped로(비-claude 어댑터 미설치 또는 `--workflow` 미언급 변형) → 비-claude 0-token 회귀 경계(S3) 보존 | spec REQ-005·plan T12·acceptance S3 |

CLI merge-path는 매 회 REVISE+empty-findings+서킷브레이커 아티팩트를 반복하나, per-provider 체크리스트는 9→9→7로 수렴하고 각 회차 이전 findings는 해소된다. 잔여는 thorough-reviewer 점증 제안 성격이며 concrete blocker는 본 closure로 해소.

## Self-Verify Summary

- Q-CORR-01 | status: PASS | attempt: 3 | files: research.md, plan.md, spec.md | reason: rev2 사실 정정 — WorktreeManager 실재(pkg/pipeline/worktree.go + pkg/worker/parallel)를 rg로 확인하고 '부재' 주장 철회. 그 외 기존 심볼 Read·rg 확인 유지.
- Q-CORR-02 | status: PASS | attempt: 1 | files: spec.md, plan.md, research.md | reason: 신규 파일/심볼/플래그(content/workflows, pkg/workflow, workflow.go, --workflow, resumeFromRunId)를 모두 [NEW]로 표기, 정합성 PASS 근거에서 제외.
- Q-CORR-03 | status: PASS | attempt: 3 | files: spec.md, research.md, acceptance.md | reason: rev2서 manifest 파서 계약 정의(phase-id 권위=schema.json JSON, md presence만, markdown grammar 불요). acceptance bare Gherkin·EARS 분리 유지.
- Q-CORR-04 | status: PASS | attempt: 2 | files: spec.md, research.md | reason: rev1서 '정본 3파일' 용어를 'SoT manifest 2파일 + 파생 generated JS'로 정정(Revision 1 closure). Reference Discipline existing vs [NEW] 분리 유지.
- Q-COMP-01 | status: PASS | attempt: 1 | files: spec.md, plan.md, acceptance.md, research.md | reason: 4파일 각 역할 보유, 상호 보완(요구/계획/오라클/근거).
- Q-COMP-02 | status: PASS | attempt: 2 | files: spec.md, acceptance.md, plan.md | reason: rev1서 REQ-012 Lore/300에 S13, REQ-011 exit-code gate에 CommandRunner replay(S8) 추가로 acceptance 추적 보강.
- Q-COMP-03 | status: PASS | attempt: 1 | files: spec.md | reason: 각 REQ에 EARS type/Priority/trigger/observability 명시.
- Q-COMP-04 | status: PASS | attempt: 2 | files: spec.md, research.md, acceptance.md, plan.md | reason: rev1서 exit-code gate(CommandRunner) + release hygiene(drift/Lore/300)를 완전 명세해 Outcome Lock 닫음(scaffold-only 회피).
- Q-COMP-05 | status: PASS | attempt: 2 | files: research.md, spec.md, acceptance.md | reason: rev1서 INV-006을 CommandRunner exit-code로, INV-005를 Lore/300까지 확장하고 S8/S13 runnable oracle로 추적.
- Q-COMP-06 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: Traceability Matrix + Reviewer Brief(scope/non-goals/self-verified/focus)로 review 범위 제한.
- Q-COMP-07 | status: PASS | attempt: 2 | files: spec.md, research.md, acceptance.md | reason: rev1서 doctor hard-gate를 Primary 프리미티브로 한정, optional 프리미티브는 advisory(REQ-006/INV-004/S14). Completion Debt/Evolution 분리 유지.
- Q-FEAS-01 | status: PASS | attempt: 4 | files: spec.md, plan.md, acceptance.md, research.md | reason: rev3서 JS→Go 호출 경계 명세 — workflow JS의 gate phase가 `auto workflow gate` CLI를 호출해 Go CommandRunner verdict를 받음(boundary 정합, S16). exit-code seam(rev1)·markdown 라우터 반영 유지.
- Q-FEAS-02 | status: PASS | attempt: 4 | files: spec.md, plan.md, acceptance.md, research.md | reason: rev2 WorktreeManager 실재 반영 + rev3 harness-workflow.md claude-scoped(비-claude 미설치)로 0-token 회귀 경계(S3) 보존. doctor advisory 분리(rev1) 유지.
- Q-FEAS-03 | status: PASS | attempt: 2 | files: acceptance.md, plan.md | reason: rev1서 S1을 generate+claude 어댑터 Generate 2단계로 분리(runnable), S8을 fake CommandRunner로 runnable화(PhaseBackend exit-code 미노출 문제 해소).
- Q-STYLE-01 | status: PASS | attempt: 1 | files: spec.md | reason: REQ description에 모호어(should/might/could) 미사용, 단정형.
- Q-STYLE-02 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: Priority(Must/Should)와 EARS type을 별도 축으로 분리.
- Q-STYLE-03 | status: PASS | attempt: 1 | files: acceptance.md | reason: 완결 문장 + bare Gherkin 키워드.
- Q-SEC-01 | status: PASS | attempt: 1 | files: research.md | reason: prompt input/생성 코드 신뢰 표면과 완화(parity/drift/validateName) 명시.
- Q-SEC-02 | status: PASS | attempt: 1 | files: research.md | reason: 비밀/절대경로 미임베드, 해시는 ephemeral 제외·digest만.
- Q-SEC-03 | status: PASS | attempt: 1 | files: research.md | reason: drift gate가 staging leak/diff noise 차단, 로그 구조화·비밀 미포함.
- Q-COH-01 | status: PASS | attempt: 1 | files: spec.md | reason: 하나의 응집 변경 스토리(opt-in workflow 기반층)로 수렴.
- Q-COH-02 | status: PASS | attempt: 2 | files: spec.md, research.md | reason: rev1서 exit-code gate seam + release hygiene enforcement를 Primary에 완전 명세(implicit debt 제거), Completion Debt None이 정직.
- Q-COH-03 | status: PASS | attempt: 1 | files: research.md | reason: Sibling Decision이 1개·독립 검증 가능·sibling의 sibling 금지 명시.
