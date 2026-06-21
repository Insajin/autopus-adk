# SPEC-HARNESS-WORKFLOW-TEAM-001 수락 기준

모든 Must 시나리오는 oracle-first다. 구조적 신호(섹션 heading·파일 존재·exit success·non-empty 출력)만으로 Must를 닫지 않으며, 구체 기대 출력 또는 명시 허용오차를 포함한다. Priority(Must/Should)는 EARS type과 별도 축이다. 기대 값은 검증된 기존 resolver(`internal/cli/effort_resolve.go`, `pkg/cost/pricing.go`)와 공유 머신러리(`pkg/workflow`)의 실제 동작에서 도출했다.

## Test Scenarios

### S1: 전체 팀 phase 순서와 agent() 오케스트레이션
Priority: Must
Given content/workflows/route_team.md와 route_team.schema.json이 team phase 집합 planning, test_scaffold, implementation, gate_build_test, annotation, testing, review, release_hygiene를 이 순서로 선언한다.
When auto workflow render --dry-run을 team manifest 대상으로 실행한다.
Then DryRunReport의 phase_order가 정확히 planning, test_scaffold, implementation, gate_build_test, annotation, testing, review, release_hygiene 8개 순서다.
And 생성된 route_team.workflow.js에서 planning, test_scaffold, implementation, annotation, testing, review phase 본문은 agent( 호출을 포함한다.
And gate_build_test phase 본문은 agent.exec(['auto','workflow','gate'])를 포함하고 release_hygiene phase 본문은 agent.exec(['auto','check','--hygiene','--arch','--quiet','--staged'])를 포함한다.

### S2: 품질→effort가 기존 resolver 출력과 일치
Priority: Must
Given ResolveEffort에 FlagQuality와 Model과 FlagComplexity를 주입한다.
When FlagQuality=ultra, Model=claude-opus-4-8로 ResolveEffort를 호출한다.
Then 반환 Effort 값이 max이고 Source 값이 quality_mode다.
When FlagQuality=ultra, Model=claude-sonnet-4-6로 호출한다.
Then 반환 Effort 값이 high다.
When FlagQuality=balanced, FlagComplexity=high로 호출한다.
Then 반환 Effort 값이 high다.
When FlagQuality=balanced, FlagComplexity=medium으로 호출한다.
Then 반환 Effort 값이 medium이다.
When FlagQuality=ultra, Model=claude-haiku-4-5로 호출한다.
Then 반환 Effort 값이 빈 문자열(EffortStripped)이고 Source 값이 quality_mode다.

### S3: 품질→model이 ModelForAgent 출력과 일치
Priority: Must
Given QualityModeToModels가 team role을 포함하도록 확장된 상태다.
When ModelForAgent(ultra, executor)를 호출한다.
Then 반환 model 값이 claude-opus-4-8이다.
When ModelForAgent(balanced, executor)를 호출한다.
Then 반환 model 값이 claude-sonnet-4-6이다.
When ModelForAgent(balanced, planner)를 호출한다.
Then 반환 model 값이 claude-opus-4-8이다.
When ModelForAgent(ultra, annotator)와 ModelForAgent(ultra, security_auditor)와 ModelForAgent(ultra, test_scaffold)를 호출한다.
Then 세 반환 model 값이 모두 claude-opus-4-8이고 빈 문자열이 아니다.

### S4: 품질→bounded depth와 cap 강제
Priority: Must
Given pkg/workflow.ResolveDepth와 cap 상수 MaxVerifyVotes=3, MaxFanOut=5, MaxRetry=3이 존재한다.
When ResolveDepth(balanced)를 호출한다.
Then 반환 VerifyVotes 값이 1이고 Synthesis 값이 false이며 FanOutCap 값이 5 이하다.
When ResolveDepth(ultra)를 호출한다.
Then 반환 VerifyVotes 값이 3이고 Synthesis 값이 true이며 FanOutCap 값이 5 이하다.
When verify_votes=4를 선언한 route_team.schema.json을 ParseSchema한다.
Then parse가 cap 초과(MaxVerifyVotes=3)로 fail-closed 에러를 반환하고 schema가 거부된다.

### S5: parity 게이트가 model/effort 드리프트를 fail-closed
Priority: Must
Given route_team.schema.json이 phase planning에 model=claude-opus-4-8을 선언하는데 파생 route_team.workflow.js의 planning agent() 호출은 model=claude-sonnet-4-6을 방출하도록 어긋난 상태다.
When generate-templates를 실행한다.
Then 종료 상태가 0이 아니다.
And stderr가 어긋난 element 이름 planning.model을 보고한다.
And templates/claude/workflows/route_team.workflow.js.tmpl이 기록되거나 갱신되지 않는다.

### S6: model JS-injection 시도가 parse 경계에서 거부
Priority: Must
Given route_team.schema.json의 phase planning이 model 값으로 비-whitelist 문자열 claude-opus-4-8");evil((를 선언한다.
When ParseSchema를 실행한다.
Then parse가 unsafe model 값을 명시하는 fail-closed 에러를 반환한다.
And 후속 generate가 route_team.workflow.js를 생성하지 않는다.

### S7: doctor가 MinVersion 미만이면 fail-fast
Priority: Must
Given capability 프로버가 version 2.1.100을 보고하고 required 프리미티브는 전부 available을 보고하도록 주입된다.
When EvaluateCapabilities를 실행한다.
Then 반환 리포트의 VersionOK 값이 false이고 overall verdict 값이 fail이다.
And 라우터가 FailureDoctorFail을 Classify하여 fail-fast 클래스를 얻고 Route A subagent pipeline으로 폴백한다.

### S8: fallback taxonomy 1:1 보존
Priority: Must
Given pkg/workflow.Classify와 KnownFailureKinds가 존재한다.
When non_claude_platform, doctor_fail, parity_drift, execution_abort, api_unavailable를 각각 Classify한다.
Then 결과 클래스가 정확히 fail-fast, fail-fast, fail-closed, resumable, explicit이다.
When 알 수 없는 failure kind를 Classify한다.
Then ok 반환 값이 false다.

### S9: dry-run 렌더가 per-phase model/effort/depth 노출
Priority: Must
Given route_team.schema.json이 planning에 model=claude-opus-4-8, effort=medium을, implementation에 fan_out_cap을, review에 verify_votes와 synthesis를 선언한다.
When auto workflow render --dry-run을 team manifest 대상으로 실행한다.
Then DryRunReport의 planning phase 항목이 model=claude-opus-4-8과 effort=medium을 포함한다.
And implementation phase 항목이 fan_out_cap 값을 포함한다.
And review phase 항목이 verify_votes와 synthesis 값을 포함한다.

### S10: 비-claude `--team` 회귀 0
Priority: Must
Given codex, gemini, opencode 어댑터를 각각 임시 디렉터리에 생성한다.
When 각 어댑터의 Generate를 실행한다.
Then 세 플랫폼 산출물에서 이름이 route_team 또는 workflow를 포함하는 .js 파일 개수가 정확히 0이다.
And 각 플랫폼에 기존 --team 표면이 존재하고 그 의미가 변경되지 않는다.
And 비-claude 산출물에 team workflow substrate 라우팅 토큰이 없다.

### S11: 결정적 exit-code 게이트 보존
Priority: Must
Given fake CommandRunner가 build 명령에 exit code 1을, test 명령에 exit code 0을 반환하도록 주입된다.
When EvaluateGate를 build와 test 명령으로 실행한다.
Then GateResult의 Verdict 값이 fail이고 VerdictSource 값이 exit_code이며 BuildExit 값이 1이다.
And team workflow의 gate_build_test phase verdict가 LLM 판정이 아니라 이 exit-code 결과에서 파생된다.

### S12: disable escape hatch가 기존 Agent Teams 동작 보존
Priority: Should
Given platform=claude-code이고 사용자가 /auto go --team --no-workflow를 호출한다.
When 라우터가 substrate를 결정한다.
Then substrate가 agent-teams로 선택되고 team workflow JS가 디스패치되지 않는다.
And 이 opt-out이 fallback taxonomy의 failure kind로 분류되지 않는다.

### S13: RALF 재시도 서킷브레이크 상한
Priority: Should
Given route_team.schema.json의 implementation phase가 retry=5를 선언한다.
When ParseSchema를 실행한다.
Then parse가 retry cap(MaxRetry=3) 초과로 fail-closed 에러를 반환한다.
When implementation phase가 retry=2를 선언한다.
Then parse가 성공하고 RetrySet의 implementation 값이 2로 보존된다.

### S14: `--multi`는 실행 기반층과 직교
Priority: Should
Given platform=claude-code이고 doctor가 pass를 보고한다.
When /auto go --team --multi를 호출한다.
Then substrate 선택이 --team 단독일 때와 동일하게 team workflow로 유지된다.
And provider review가 review phase에서만 risk tier가 high 또는 critical일 때 활성화된다.
When /auto go --team을 --multi 없이 호출한다.
Then review phase에서 provider review가 활성화되지 않는다.

### S15: prompt-manifest 해시가 ephemeral을 제외
Priority: Must
Given team workflow의 prompt 레이어가 stable 구조 레이어와 ephemeral per-run 레이어로 분류된다.
When per-run quality override만 balanced에서 ultra로 바꾸고 PromptManifestHash를 계산한다.
Then 해시 값이 변경 전과 동일하다.
When schema의 planning phase model 기본값을 바꾸고(stable 레이어) PromptManifestHash를 계산한다.
Then 해시 값이 변경 전과 다르다.

### S16: 품질 override가 render overlay로 per-phase agent() opts에 도달
Priority: Must
Given resolveTeamQualityBinding가 phase-id↔role map(implementation→executor, review→reviewer, planning→planner)으로 ResolveEffort와 ModelForAgent와 ResolveDepth를 호출해 QualityBinding을 만들고 OverlayPhases가 schema baseline에 override를 덮어쓴다.
When auto workflow render --route team --quality ultra를 실행한다.
Then DryRunReport의 implementation phase 항목이 model=claude-opus-4-8과 effort=max를 보인다.
And review phase 항목이 verify_votes=3과 synthesis=true를 보인다.
When auto workflow render --route team --quality balanced를 실행한다.
Then implementation phase 항목이 model=claude-sonnet-4-6과 effort=medium을 보인다.
And review phase 항목이 verify_votes=1과 synthesis=false를 보인다.

### S17: claude adapter가 route_a와 route_team JS를 모두 설치
Priority: Must
Given claude 어댑터를 임시 디렉터리에 생성한다.
When 어댑터의 Generate를 실행한다.
Then 산출물에 .claude/workflows/route_a.workflow.js가 존재한다.
And 산출물에 .claude/workflows/route_team.workflow.js가 존재한다.
And 두 파일의 첫 줄이 모두 GENERATED와 DO NOT EDIT 경고를 포함한다.

### S18: render route 선택이 route_a와 route_team을 분리
Priority: Must
Given route_a manifest는 4 phase를, route_team manifest는 8 phase를 선언한다.
When auto workflow render --route team --dry-run을 실행한다.
Then phase_order가 정확히 planning, test_scaffold, implementation, gate_build_test, annotation, testing, review, release_hygiene 8개다.
When auto workflow render --dry-run을 --route 플래그 없이 실행한다.
Then phase_order가 정확히 planning, implementation, gate_build_test, release_hygiene 4개이고 route_a manifest가 선택된다.

### S19: 생성 route_team JS의 fan-out과 security_auditor 구조
Priority: Must
Given route_team.schema.json이 implementation에 fan_out_cap을, review에 verify_votes와 synthesis를 선언한다.
When generate-templates로 route_team.workflow.js.tmpl을 파생한다.
Then implementation phase 블록이 for 키워드와 agent('executor' 호출을 포함하는 executor fan-out 루프를 가지고 fan_out_cap 값을 참조한다.
And review phase 블록이 agent('reviewer' 호출과 agent('security_auditor' 호출을 모두 포함한다.
And implementation phase 블록이 RT.implementation runtime binding 참조와 schema baseline model 리터럴을 함께 포함한다.
And route_a.workflow.js.tmpl 골든이 변경되지 않는다.

### S20: 디스패치가 binding을 AUTOPUS_WORKFLOW_QUALITY env로 직렬화
Priority: Must
Given resolveTeamQualityBinding가 quality=ultra와 complexity로 QualityBinding을 만든다.
When 디스패치가 binding을 AUTOPUS_WORKFLOW_QUALITY 환경값으로 직렬화한다.
Then 직렬화된 JSON의 implementation 항목이 model=claude-opus-4-8과 effort=max를 포함한다.
And review 항목이 verify_votes=3과 synthesis=true를 포함한다.
And 이 env 키 이름이 생성 route_team JS가 읽는 RT seam 키 AUTOPUS_WORKFLOW_QUALITY와 일치한다.

## Oracle Acceptance Notes

각 Must 시나리오는 concrete expected output 또는 explicit tolerance를 oracle로 사용하며, structural-only 신호(파일 존재·heading·exit code 단독)로 Must를 닫지 않는다.

- S1: expected value = 정확한 8개 phase_order 시퀀스(planning..release_hygiene)와 phase별 `agent(`/`agent.exec(` 호출 종류. 단순 파일 존재가 아니라 생성 JS 본문의 호출 형태를 검증한다.
- S2: expected value = effort 리터럴(ultra+opus-4-8→`max`, ultra+sonnet→`high`, balanced+high→`high`, balanced+medium→`medium`, ultra+haiku→빈 문자열). 기존 `ResolveEffort` 출력과 byte 단위 일치.
- S3: expected value = model 리터럴(`claude-opus-4-8`/`claude-sonnet-4-6`). 기존 `ModelForAgent` 출력과 일치.
- S4: expected value = depth(balanced votes=1/synthesis=false, ultra votes=3/synthesis=true) + cap 초과(verify_votes=4) explicit 거부.
- S5: expected output = stderr의 diverging element 이름 `planning.model` + JS 미기록(no numeric tolerance; exact 문자열 oracle).
- S6: expected output = unsafe model 값에 대한 parse 에러 + JS 미생성(fail-closed oracle).
- S7: expected value = VersionOK=false, overall=`fail`, fallback class=`fail-fast`.
- S8: expected value = 5 FailureKind→class 정확 매핑 + unknown→ok=false.
- S9: expected value = DryRunReport phase 항목의 model/effort/verify_votes/synthesis 필드 존재 및 값 일치.
- S11: expected value = GateResult Verdict=`fail`, VerdictSource=`exit_code`, BuildExit=1.
- S13: expected output = retry=5 거부(cap=3) + retry=2 보존(RetrySet 값=2).
- S15: expected value = ephemeral-only 변경 시 PromptManifestHash 불변, stable 구조 변경 시 해시 변동.

- S16: expected value = render --route team --quality overlay의 per-phase 값(ultra implementation model=claude-opus-4-8/effort=max·review votes=3/synthesis=true; balanced implementation model=claude-sonnet-4-6/effort=medium·review votes=1/synthesis=false). resolver 반환이 아니라 RENDER 출력(DryRunReport per-phase 필드)을 검증한다.
- S17: expected value = `.claude/workflows/route_a.workflow.js`와 `route_team.workflow.js` 둘 다 존재 + 양쪽 generated-warning 첫 줄. 단일 route 설치가 아니라 두 산출물 동시 설치.
- S18: expected value = team route phase_order 8개(planning..release_hygiene) vs 기본 route_a phase_order 4개(route 선택 분기 oracle).
- S19: expected output = route_team JS implementation 블록의 fan-out 루프(`for` + `agent('executor'` + fan_out_cap) + review 블록의 `agent('reviewer'`·`agent('security_auditor'` 동시 존재 + implementation 블록의 `RT.implementation` override 참조와 baseline 리터럴 + route_a 골든 불변. 구조-only "agent( 존재"가 아니라 역할별 호출·fan-out·override seam을 검증.
- S20: expected output = `AUTOPUS_WORKFLOW_QUALITY` env JSON이 implementation model=claude-opus-4-8/effort=max, review votes=3/synthesis=true를 담고 키 이름이 JS seam과 일치.

Should 시나리오(S10/S12/S14)는 substrate 라우팅·회귀 0 동작을 expected output(예: `route_team`/`workflow` `.js` count=0, substrate=agent-teams)으로 검증한다.
