# SPEC-HARNESS-WORKFLOW-TEAM-001 구현 계획

Migration 디렉토리: **N/A** — SQL/DB 스키마 변경 없음(전부 Go + manifest + skill source).

## Tasks

### G1 — pkg/workflow Go 공유 머신러리 확장
- [ ] T1: `pkg/workflow/schema.go` — `PhaseDef`에 `Model`/`Effort`/`VerifyVotes`/`FanOutCap`/`Synthesis` 필드 추가, `rawPhase`에 대응 JSON 키 추가(`DisallowUnknownFields` 파서가 새 키를 거부하지 않도록), `ParseSchema`에서 새 필드 채움. `Schema.{ModelSet,EffortSet,DepthSet}()` accessor 추가. 300줄 한계 근접 시 phase 타입/accessor를 `pkg/workflow/schema_sets.go`로 분리.
- [ ] T2: `[NEW] pkg/workflow/schema_validate.go` — model whitelist(`claude-opus-4-8`/`claude-opus-4-7`/`claude-sonnet-4-6`/`claude-haiku-4-5`)와 effort enum(`low`/`medium`/`high`/`xhigh`/`max`/빈 문자열)을 검증하는 `isSafeAgentModel`/`isSafeEffort`. deterministic gate phase(result_type=`exit_code`)는 빈 model/effort 허용, agent phase는 비-whitelist 값 fail-closed. `ParseSchema`가 phase-id 검증(`isSafePhaseID`)과 같은 경계에서 호출.
- [ ] T3: `[NEW] pkg/workflow/depth.go` — `DepthProfile{VerifyVotes,FanOutCap,Synthesis,Retry}`, cap 상수 `MaxVerifyVotes=3`/`MaxFanOut=5`/`MaxRetry=3`, 순수 함수 `ResolveDepth(quality string) DepthProfile`(balanced→votes=1/synthesis=false, ultra→votes=3/synthesis=true, 둘 다 fan_out<=cap), cap 초과 schema 값 거부 헬퍼 `ClampOrReject`. pkg/workflow는 internal/cli를 import하지 않으므로 depth는 여기 둔다.
- [ ] T4: `pkg/workflow/render.go` — `DryRunReport`에 per-phase `Phases []RenderedPhase{ID,Model,Effort,VerifyVotes,FanOutCap,Synthesis}` 추가, `Render()`가 schema에서 채움. `PromptManifestHash`는 무변경(ephemeral 제외 보존). 300줄 근접 시 render 타입을 `render_types.go`로 분리.

### G2 — manifest SoT + JS 재생성
- [ ] T5: `[NEW] content/workflows/route_team.md` + `[NEW] content/workflows/route_team.schema.json` — 8 phase 순서(planning, test_scaffold, implementation, gate_build_test, annotation, testing, review, release_hygiene), 각 agent phase에 balanced 기본 model/effort(=`ModelForAgent("balanced",role)` + effort=medium), implementation `fan_out_cap`, review `verify_votes`/`synthesis`, gate phase `verdict_source: exit_code`(model/effort 빈값). route_a.md를 mirror해 SoT/parity/regen 문구 포함.
- [ ] T6: `pkg/content/workflow_generate.go` — `generateWorkflowTemplates`/`deriveWorkflowJS`를 route 파라미터화(route_a 하드코딩 상수 `routeASchemaFile`/`routeAMarkdown`/`routeAJSTmplName` 일반화해 route_a + route_team 양쪽 파생). gate_build_test는 `agent.exec(['auto','workflow','gate'])`, release_hygiene는 `agent.exec(['auto','check','--hygiene','--arch','--quiet','--staged'])`(route_a와 동일 bridge; deterministic phase는 model/effort/RT 미방출). route_a.workflow.js.tmpl의 planning/implementation 본문은 `agent({model,effort})`로 보강하되 phase-id/retry/budget/result-type 집합 불변(route_a 구조 회귀 0). route_team 본문의 agent fan-out/security_auditor/`RT.<phase>` override 방출은 T16이 소유. `auto generate-templates`로 양 route 재생성.
- [ ] T7: `pkg/content/workflow_parity.go` — parity 비교를 model/effort/depth(verify_votes/fan_out_cap/synthesis) 집합으로 확장하고 route_team manifest를 등록. 생성 JS는 각 phase 블록 안에 결정적 baseline 마커(`agent('<role>', { model: '<baseline>', effort: '<baseline>', votes: <n>, fanout: <n>, synthesis: <bool> }` 리터럴)를 방출하고, parity는 현재 전역 `strings.Contains` 토큰 검사를 보강해 `phase('<id>'` 경계로 per-phase 블록을 슬라이스한 뒤 그 블록 안에서만 baseline model/effort/depth 토큰을 검사한다. 드리프트 시 phase-scoped 이름(예: `planning.model`)을 방출하고 JS 미기록.
- [ ] T8: `pkg/cost/pricing.go::QualityModeToModels` — team role 추가: `test_scaffold`/`annotator`/`security_auditor` → ultra=`claude-opus-4-8`, balanced=`claude-sonnet-4-6`. `ModelForAgent`가 모든 team phase role에 비-빈 model 반환하도록 보장(빈 model은 whitelist 검증서 거부되므로 필수).

### G3 — `/auto go` skill source fragments (정본만, 생성된 `.claude/` 미편집)
- [ ] T9: `templates/claude/commands/auto-router.md.tmpl` — Route B 섹션을 "claude-code + doctor pass → team workflow substrate, 아니면 기존 Agent Teams/폴백"으로 확장, Quality Mode Step 2.1에 (model, effort, depth) 동시 구동 명시, flag table에 `--no-workflow`(disable escape hatch) 추가, fallback 로그 라인 형식 보존(`[workflow] fallback-class=fail-fast reason={doctor_fail|non_claude_platform}`), `--team --multi` 직교 명시(provider review는 review phase에서 risk tier 기준).
- [ ] T10: `content/skills/agent-teams.md`(claude workflow 치환 노트) + `content/skills/harness-workflow.md`(team phase 집합 문서화) 확장. 두 스킬 모두 claude-scoped 유지(비-claude 미설치). 비-claude 어댑터 템플릿(codex/gemini)에는 team workflow 토큰 미추가.

### G4 — 테스트 + parity + 비-claude 회귀
- [ ] T11: `pkg/workflow/schema_test.go`(model/effort/depth parse), `[NEW] pkg/workflow/schema_validate_test.go`(whitelist 거부), `[NEW] pkg/workflow/depth_test.go`(ResolveDepth + cap 초과 거부), `pkg/workflow/render_test.go`(dry-run per-phase model/effort/depth + prompt-manifest 해시 ephemeral 제외).
- [ ] T12: `pkg/content/workflow_parity_test.go` + `pkg/content/workflow_generate_test.go` — model/effort/depth 드리프트 fail-closed, route_team generate 바이트 결정성, route_a 구조 회귀 0(기존 골든 불변).
- [ ] T13: `internal/cli/workflow_test.go`(또는 신규 dispatch 테스트) — 품질→effort/model 해석이 `ResolveEffort`/`ModelForAgent` 출력과 일치(재사용 증명), doctor-fail→fail-fast Route A, disable→Agent Teams, non-claude→불변.
- [ ] T14: 비-claude 회귀 스위트 — codex/gemini/opencode 어댑터 Generate 산출물에 `route_team`/`workflow` `.js` 0건, 기존 `--team` 표면 보존.

### G5 — route_team 설치·render route 선택·런타임 품질 바인딩 seam
- [ ] T15: `[NEW] pkg/workflow/binding.go` — `QualityBinding{Phases map[string]PhaseBinding}`, `PhaseBinding{Model,Effort string; VerifyVotes,FanOutCap int; Synthesis bool}`, 그리고 schema baseline에 binding을 덮어쓰는 순수 overlay 헬퍼 `OverlayPhases(schema, *QualityBinding) []RenderedPhase`(binding nil이면 baseline 그대로). pkg/workflow는 internal/cli·pkg/cost를 import하지 않으므로 데이터 타입과 overlay만 소유하고, 값 계산은 디스패치(T18)가 주입한다. 300줄 한계: binding 타입/overlay를 이 신규 파일로 분리.
- [ ] T16: `[NEW] pkg/content/workflow_generate_team.go` — route_team 본문 emitter: 각 agent phase가 `agent('<role>', { model: (RT.<phase> && RT.<phase>.model) || '<schema baseline>', effort: ... })` 형태로 schema baseline 리터럴(parity 도메인)과 `RT.<phase>` 런타임 override(ephemeral seam)를 함께 방출. 본문 상단에 `const RT = JSON.parse(env('AUTOPUS_WORKFLOW_QUALITY') || '{}')` seam을 emit. implementation은 fan_out_cap 한정 executor fan-out 루프(`for (let i=0;i<Math.min(n,FANOUT_CAP);i++) await agent('executor', …)`), review는 `agent('reviewer', …)`와 `agent('security_auditor', …)` 두 호출 + synthesis 조건부 multi-vote. gate_build_test/release_hygiene는 T6의 deterministic agent.exec 그대로(model/effort/RT 미방출). 300줄 한계: route_team 본문 emitter를 generate.go가 아닌 이 신규 파일로 분리.
- [ ] T17: `pkg/adapter/claude/claude_workflow.go` — `workflowFiles`를 확장해 route_a FileMapping에 더해 `route_team.workflow.js.tmpl`을 읽어 `.claude/workflows/route_team.workflow.js` FileMapping을 추가 방출(OverwriteAlways, generated-warning 첫 줄 보존). 상수 `workflowTemplatePath`/`workflowTargetPath`를 route별 쌍으로 일반화. 비-claude 어댑터는 무변경(회귀 0).
- [ ] T18: `internal/cli/workflow_render.go` + `[NEW] internal/cli/workflow_quality_binding.go` — render에 `--route`(기본 `route_a`)와 `--quality` 플래그 추가: route가 embed 상수(`workflowSchemaEmbedPath`/`workflowContractEmbedPath`/`workflowJSEmbedPath`)를 route_a/route_team 쌍에서 선택하고, `resolveTeamQualityBinding(quality, complexity)`가 phase-id↔role map(planning→planner, test_scaffold→test_scaffold, implementation→executor, annotation→annotator, testing→tester, review→reviewer+security_auditor; gate_build_test/release_hygiene=deterministic, role 없음)으로 `ResolveEffort`/`ModelForAgent`/`workflow.ResolveDepth`를 호출해 `workflow.QualityBinding`을 만든다. render는 binding을 `OverlayPhases`로 `DryRunReport.Phases`에 overlay하고, 디스패치 경로는 binding을 `AUTOPUS_WORKFLOW_QUALITY` env JSON으로 직렬화한다. 300줄 한계: binding 해석/role map/env 직렬화를 신규 파일로 분리.
- [ ] T19: 테스트 — `[NEW] internal/cli/workflow_quality_binding_test.go`(resolveTeamQualityBinding 값 + env JSON 직렬화: ultra implementation=claude-opus-4-8/max, review votes=3/synthesis=true → S16/S20), `pkg/content/workflow_generate_test.go`(route_team JS의 executor fan-out 루프 + fan_out_cap 참조 + `agent('reviewer'`/`agent('security_auditor'` + `RT.<phase>` override 참조 + schema baseline 리터럴 → S19; route_a 골든 불변 = 회귀 0), `pkg/adapter/claude/claude_workflow_test.go`(route_a·route_team `.js` 둘 다 설치 → S17), `internal/cli/workflow_render_test.go`(`--route team`=8-phase 선택, 기본=route_a 4-phase, `--quality ultra` overlay → S18/S16).

## Implementation Strategy

- **공유 머신러리 재사용**: doctor.go/gate.go/fallback.go/drift_gate.go/doctor_version.go는 무변경 재사용. parity·generate·schema·render만 확장. 이로써 fallback taxonomy(5 kind 1:1)·doctor MinVersion(2.1.154)·exit-code gate가 자동 보존된다.
- **아키텍처 경계**: 품질→effort/model은 `internal/cli`(ResolveEffort/ModelForAgent)에서 해석하고 `pkg/workflow`에 데이터로 주입. `pkg/workflow`는 `internal/cli`/`pkg/content` 미import(schema.go:5 경계). depth만 pkg/workflow의 순수 함수.
- **런타임 바인딩 seam(baseline vs override)**: schema가 per-phase balanced baseline(model/effort/verify_votes/fan_out_cap)을 선언하고 generate가 이를 agent() 리터럴로 렌더(parity 도메인). 디스패치는 per-run override를 `QualityBinding`으로 계산해 `AUTOPUS_WORKFLOW_QUALITY` env JSON으로 직렬화하고, 생성 JS의 agent() opts는 `RT.<phase>` override를 baseline 리터럴 fallback과 함께 읽는다(런타임 우선). `render --route team --quality <mode>`는 동일 binding을 `OverlayPhases`로 overlay해 override가 agent() opts에 도달함을 hermetic하게 노출(S16). depth도 동일 이중성(schema=baseline/parity, ResolveDepth=ephemeral override/런타임 우선).
- **route_a 회귀 0**: route_team을 신설해 route_a의 phase 집합(phase-id/retry/budget/result-type)을 건드리지 않는다. route_a JS 본문의 agent() 보강은 구조 골든을 바꾸지 않는 본문-only 변경으로 한정한다.
- **300줄 한계**: schema.go(현 141줄)에 필드/검증/accessor를 추가하면 한계 근접 → schema_validate.go·depth.go·render_types.go로 분리.

## Visual Planning Brief

`--team` 라우팅·실행 흐름(command/data-flow):

```
/auto go --team (claude-code)
        |
        v
 [pre-route check] --no-workflow / workflow.team_default=false ? --yes--> 기존 Agent Teams (Route B, 회귀 0)
        | no
        v
 platform == claude-code ? --no--> 비-claude: 기존 --team 불변 (Agent Teams는 claude-only → 폴백)
        | yes
        v
 auto workflow doctor  --(overall=fail: version<2.1.154 or required primitive 누락)--> Classify(doctor_fail)=fail-fast
        | overall=pass                                                                       |
        v                                                                                    v
 internal/cli dispatch: ResolveEffort(quality,model,complexity) + ModelForAgent(quality,role)   Route A subagent pipeline
        + workflow.ResolveDepth(quality)  [bounded: votes<=3, fanout<=5, retry<=3]
        |  (resolved model/effort/depth = ephemeral, 주입)
        v
 team workflow JS (route_team.workflow.js) phase DAG:
   planning      agent({model,effort})
   test_scaffold agent({model,effort})
   implementation agent({model,effort}) ×N  (fan_out_cap<=5, worktree; RALF retry<=3)
   gate_build_test  agent.exec(['auto','workflow','gate'])  --> {verdict, verdict_source: exit_code}
        | verdict=fail --> RALF retry (<=cap) --> circuit-break --> fallback(execution_abort=resumable)
        v verdict=pass
   annotation    agent({model,effort})
   testing       agent({model,effort})
   review        agent({model,effort}) (+ security-auditor; verify_votes, synthesis; --multi=provider review @ high/critical risk only)
   release_hygiene agent.exec(['auto','check','--hygiene','--arch','--quiet','--staged'])
        |
        v
   sync 진입
```

**런타임 바인딩 seam(다이어그램 보강)**: 위 흐름의 "resolved model/effort/depth = ephemeral, 주입" 단계는 디스패치가 `QualityBinding`을 `AUTOPUS_WORKFLOW_QUALITY` env JSON으로 직렬화하는 것으로 구현된다. 생성 route_team JS 본문 상단의 `const RT = JSON.parse(env('AUTOPUS_WORKFLOW_QUALITY') || '{}')`가 이를 읽고, 각 `agent('<role>', { model: RT.<phase>?.model || '<baseline>', … })`가 override를 baseline 리터럴 fallback과 함께 적용한다. `auto workflow render --route team --quality <mode>`는 동일 binding을 `OverlayPhases`로 overlay해 override가 agent() opts에 도달함을 hermetic하게 노출한다(결정 로직 검증). live claude-code Workflow 런타임이 이 env를 agent()에 실제 상속시켜 실 LLM으로 8-phase를 구동하는 부분만 operational 잔여(Completion Debt)다.

## Feature Completion Scope

- Primary SPEC가 Outcome Lock을 닫는다: 전체 8-phase agent() 오케스트레이션(REQ-002), 품질→(model,effort,depth) 배선(REQ-003/004), 품질 override→agent() opts 런타임 바인딩(REQ-015), route_team 설치 + render route 선택(REQ-016), multi-agent executor fan-out + security_auditor(REQ-017), claude 전용 doctor-gated 활성화(REQ-001/007), 결정적 게이트·RALF 보존(REQ-005/006), parity 확장(REQ-010), JS-injection 방어(REQ-011), dry-run 노출(REQ-012), 비-claude 회귀 0(REQ-009), `--multi` 직교(REQ-013), prompt-layer 계약(REQ-014).
- 승인된 sibling 의존성: **none**.
- 남은 Completion Debt: **1건(live end-to-end 실행)** — research.md `## Completion Debt` 참조. 결정 로직(품질 binding 해석·render overlay·route 설치/선택·fan-out/security_auditor 구조·env 직렬화)은 전부 hermetic Go/parity/adapter 오라클(S16-S20)로 닫는다. 그러나 claude-code Workflow 런타임이 설치된 route_team.workflow.js를 실제 실행해 agent()가 `AUTOPUS_WORKFLOW_QUALITY`를 상속하고 실 LLM 트래픽으로 executor×N + reviewer + security_auditor를 구동하는 live 실행은 결정적 hermetic 오라클로 만들 수 없는 operational 잔여이므로 REAL Completion Debt로 정직하게 기록하고 sync completion 전 운영 검증을 요구한다(none으로 선언하지 않음).
