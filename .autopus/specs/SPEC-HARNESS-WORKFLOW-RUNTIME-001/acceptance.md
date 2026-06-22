# SPEC-HARNESS-WORKFLOW-RUNTIME-001 수락 기준

모든 Must 시나리오(S1~S9, S11, S12)는 oracle-first다. 구조적 신호(섹션 heading·파일 존재·exit success·non-empty 출력)만으로 Must를 닫지 않으며, 구체 기대/금지 substring 또는 명시 값을 포함한다. S10만 Should다. Priority(Must/Should)는 EARS type과 별도 축이다. 기대 값은 이번 세션의 경험적 Workflow 런타임 계약과 실제 코드(`pkg/content/workflow_generate*.go`, `pkg/content/workflow_parity.go`, `internal/cli/workflow_quality_binding.go`, `pkg/workflow/depth.go`)에서 도출했다.

S1·S2·S3가 launch-contract 오라클(anti-theater)이고, S11이 segment 경계/디스패처 barrier 순서 오라클(INV-007), S12가 runtime quality caps 오라클(INV-008)이다. render/parity-only 신호로는 Must를 닫지 않는다.

## Test Scenarios

### S1: route_a 생성 JS가 실제 Workflow API launch 계약을 만족
Priority: Must
Given content/workflows/route_a.schema.json이 phase 집합 planning, implementation, gate_build_test, release_hygiene를 선언한다.
When deriveWorkflowJS로 route_a JS를 생성한다.
Then 생성 문자열의 첫 비주석 코드 토큰이 export const meta로 시작한다.
And 부분문자열 export의 출현 횟수가 정확히 1이고 export default의 출현 횟수가 0이다.
And 부분문자열 function run(의 출현 횟수가 0이다.
And 부분문자열 env(의 출현 횟수가 0이고 agent.exec(의 출현 횟수가 0이다.
And gate_build_test phase 블록과 release_hygiene phase 블록 각각에 agent.exec(의 출현 횟수가 0이고 phase('gate_build_test')와 phase('release_hygiene') 마커와 log( 호출이 존재한다.
And phase('planning') 호출의 인자 개수가 1이다.

### S2: route_team 생성 JS가 실제 Workflow API launch 계약을 만족
Priority: Must
Given content/workflows/route_team.schema.json이 8개 phase planning, test_scaffold, implementation, gate_build_test, annotation, testing, review, release_hygiene를 선언한다.
When deriveTeamWorkflowJS로 route_team JS를 생성한다.
Then 생성 문자열의 첫 비주석 코드 토큰이 export const meta로 시작한다.
And 부분문자열 export의 출현 횟수가 정확히 1이고 export default의 출현 횟수가 0이다.
And 부분문자열 function run(의 출현 횟수가 0이다.
And 부분문자열 env(의 출현 횟수가 0이고 agent.exec(의 출현 횟수가 0이다.
And 부분문자열 JSON.parse(env의 출현 횟수가 0이고 AUTOPUS_WORKFLOW_QUALITY의 출현 횟수가 0이다.
And gate_build_test phase 블록과 release_hygiene phase 블록 각각에 agent.exec(의 출현 횟수가 0이다.

### S3: agent 호출이 args에서 만든 비자명 task prompt 문자열을 첫 인자로 사용
Priority: Must
Given route_team JS의 agent-driven phase 집합 planning, test_scaffold, implementation, annotation, testing, review가 있다.
When deriveTeamWorkflowJS로 생성한 JS에서 각 agent-driven phase의 첫 agent( 호출 첫 인자를 검사한다.
Then 첫 인자가 따옴표로 감싼 단일 role identifier 문자열(예: 'planner' 또는 'executor' 같은 bare role literal)이 아니다.
And 첫 인자가 role과 per-run 컨텍스트를 args/ctx 토큰으로 보간한 비자명 task template literal(예: `Execute executor agent for spec ${ctx.spec || ''} in ${ctx.workingDir || ''}`)이고 두 번째 인자가 model 키를 포함하는 opts 객체다.
And route_team JS 본문에 const ctx = args 또는 args.quality를 읽는 preamble이 존재한다.

### S4: quality binding이 env가 아니라 args.quality로 전달
Priority: Must
Given resolveTeamQualityBinding(ultra, "")가 phase별 PhaseBinding 맵을 계산한다.
When ultra 모드로 binding을 계산하고 직렬화한다.
Then implementation phase의 fan_out_cap가 5이고 review phase의 verify_votes가 3이며 synthesis가 true다.
And planning phase의 model이 claude-opus-4-8이고 implementation phase의 model이 claude-sonnet-4-6이다.
And 생성된 route_team JS가 이 binding을 args.quality에서 읽고 env('AUTOPUS_WORKFLOW_QUALITY') 또는 JSON.parse(env( 형태로 읽지 않는다.

### S5: 단일 인자 phase에도 parity 토큰이 보존되어 게이트 green
Priority: Must
Given route_team schema가 implementation phase에 retry=2, budget=120000, model=claude-sonnet-4-6, effort=medium, fan_out_cap=5를 선언한다.
When deriveTeamWorkflowJS 출력으로 checkWorkflowParity를 실행한다.
Then phase('implementation') 호출이 단일 인자다.
And implementation phase 블록 안에 retry 토큰 값 2, budget 토큰 값 120000, model=claude-sonnet-4-6, effort=medium, fan_out_cap=5 토큰이 모두 존재한다.
And checkWorkflowParity가 nil 에러를 반환한다.
When implementation phase의 model 토큰을 JS에서 claude-haiku-4-5로 변조한 픽스처로 checkWorkflowParity를 실행한다.
Then 반환 에러 메시지가 implementation.model을 diverging element로 명시한다.

### S6: 결정적 게이트가 JS 밖 Go 브리지로 실행됨이 문서·JS에 반영
Priority: Must
Given 정정된 content/skills/harness-workflow.md와 templates/claude/commands/auto-router.md.tmpl가 있다.
When 두 문서에서 게이트 서술과 Workflow API globals 서술을 검사한다.
Then 부분문자열 agent.exec의 출현 횟수가 0이고 "Workflow API globals: agent, phase, log, env" 문구의 출현 횟수가 0이다.
And gate_build_test가 auto workflow gate를 JS 밖에서 실행하는 Go-runtime 단계로 서술되고 verdict_source: exit_code가 보존된다.
And quality 전달이 AUTOPUS_WORKFLOW_QUALITY env가 아니라 Workflow args 입력으로 서술된다.
And 생성된 route_a·route_team JS의 gate_build_test/release_hygiene phase 본문에 agent.exec(의 출현 횟수가 0이다.

### S7: 비-claude 어댑터 회귀 0
Priority: Must
Given codex, gemini, opencode 어댑터의 Generate 산출물이 있다.
When 각 어댑터의 생성 파일 목록을 검사한다.
Then 이름에 route_a 또는 route_team 또는 workflow를 포함하는 .js 파일의 개수가 0이다.
And 산출물에 harness-workflow 또는 --workflow 토큰을 포함하는 surface의 개수가 0이고 기존 Route A 라우터 표면이 존재한다.

### S8: parse·depth·whitelist invariant 보존
Priority: Must
Given route_team schema parse 경계가 있다.
When fan_out_cap=6을 가진 phase로 ParseSchema를 호출한다.
Then 반환 에러가 fan_out_cap 6 exceeds cap 5를 명시한다.
When model=claude-opus-4-8");evil((를 가진 phase로 ParseSchema를 호출한다.
Then 반환 에러가 unsafe model을 명시하고 JS가 생성되지 않는다.
When 정상 route_team schema로 ParseSchema를 호출한다.
Then 에러가 nil이고 doctor MinVersion 상수가 2.1.154로 유지된다.

### S9: 양 route 실 런타임 launch(operational 오라클)
Priority: Must
Given /auto go --workflow와 /auto go --team이 claude-code에서 doctor 통과로 디스패치된다.
When 메인 세션이 생성된 .claude/workflows/route_a.workflow.js와 .claude/workflows/route_team.workflow.js를 각각 Workflow 툴로 args={spec, workingDir, quality, segment}와 함께 launch한다.
Then route_a launch와 route_team launch 모두 "SyntaxError: Unexpected keyword 'export'"를 방출하지 않고 워크플로우가 launch된다.
And 각 route에서 해당 segment의 선언 phase가 순서대로 시작되어 run journal/로그에 phase 라벨이 기록된다.
And 이 시나리오는 hermetic Go 단위가 아니라 operational 오라클이다(subagent는 Workflow 툴 호출 불가): hermetic half S1/S2/S3가 양 route(route_a AND route_team)의 계약 conformance를 결정적으로 닫고, S9는 메인 세션이 두 route를 실 런타임에서 launch + phase 진입을 실증하는 operational 절반이다.

### S11: 결정적 게이트가 segment 경계 + 디스패처 barrier로 순서를 강제
Priority: Must
Given deriveTeamWorkflowJS로 생성한 route_team JS와 정정된 content/skills/agent-teams.md·templates/claude/commands/auto-router.md.tmpl 디스패처 계약이 있다.
When 생성 JS의 segment 가드 블록과 디스패처 문서의 segment-launch 순서를 검사한다.
Then 생성 JS에 const SEGMENT = (args && args.segment) || 'A' preamble이 존재하고 segment A 가드 블록과 segment B 가드 블록이 분리되어 있다.
And segment A 가드 블록의 마지막 phase( 호출이 phase('gate_build_test')이고 segment B 가드 블록의 첫 phase( 호출이 phase('annotation')이다.
And 디스패처 문서가 segment A launch 다음 auto workflow gate(verdict_source: exit_code) barrier를 두고 verdict=pass일 때만 segment B를 launch한 뒤 auto check --hygiene --arch --quiet --staged barrier를 실행한다고 명세한다.
And 게이트 verdict가 pass가 아니면 디스패처가 segment B를 launch하지 않고 중단한다고 문서가 명시한다.

### S12: 런타임 quality binding이 bounded caps를 args.quality로 전달
Priority: Must
Given resolveTeamQualityBinding과 workflow.ResolveDepth가 quality tier에서 per-phase binding을 계산한다.
When ultra와 balanced(baseline) tier로 binding을 계산하고 직렬화한다.
Then ultra binding의 implementation.fan_out_cap가 5이고 review.verify_votes가 3이며 review.synthesis가 true다.
And baseline binding의 review.verify_votes가 1이고 review.synthesis가 false다.
And depth cap 상수 MaxFanOut가 5, MaxVerifyVotes가 3, MaxRetry가 3이며 fan_out_cap=6 입력이 fail-closed로 거부된다.
And 생성된 route_team JS가 이 caps를 args.quality(RT.<phase>.fan_out_cap/verify_votes/synthesis)에서 읽고 env( 또는 JSON.parse(env( 형태로 읽지 않는다.

### S10: dry-run 렌더가 갱신 후에도 per-phase 표면 노출(회귀)
Priority: Should
Given 갱신된 route_team 생성기와 manifest가 있다.
When auto workflow render --route team --quality ultra를 실행한다.
Then DryRunReport phase_order가 8개 phase를 schema 순서로 출력한다.
And review phase 행의 verify_votes가 3이고 synthesis가 true이며 implementation phase 행의 fan_out_cap가 5다.
And prompt-manifest 해시가 quality 오버레이 적용 전후로 동일하다.

## Oracle Acceptance Notes

Must 시나리오 S1~S9, S11, S12는 oracle acceptance다. 구조적 신호(섹션 heading, 파일 존재, exit success, non-empty output)만으로는 Must를 닫지 않으며, 각 시나리오는 concrete expected output 또는 explicit tolerance를 포함한다.

- **S1/S2 (launch-contract 오라클)**: 예상 출력은 substring 출현 횟수의 정확한 값이다 — `export`=1, `export default`=0, `function run(`=0, `env(`=0, `agent.exec(`=0. 이는 file-existence나 render 성공이 아니라 실제 Workflow API 계약 conformance다.
- **S3**: 예상 값은 agent 첫 인자가 role-only 문자열이 아니라 args/ctx 참조 template literal이라는 구체 형태 단언이다.
- **S4**: 예상 값은 resolveTeamQualityBinding(ultra) 출력 — fan_out_cap=5, verify_votes=3, synthesis=true, planning.model=claude-opus-4-8, implementation.model=claude-sonnet-4-6 — 와 args.quality 전달 채널이다.
- **S5**: 예상 출력은 retry=2/budget=120000/model=claude-sonnet-4-6/effort=medium/fan_out_cap=5 토큰 존재 + checkWorkflowParity nil, 그리고 변조 시 diverging element=implementation.model 메시지다.
- **S6**: 예상 값은 문서 substring 횟수 `agent.exec`=0, "Workflow API globals: agent, phase, log, env"=0 + 게이트가 JS 밖 Go 단계로 서술됨이다.
- **S8**: 예상 출력은 parse 에러 메시지(fan_out_cap 6 exceeds cap 5, unsafe model) + MinVersion=2.1.154다.
- **S9 (operational 오라클, 양 route)**: 예상 출력은 route_a와 route_team 두 설치 JS가 각각 실 런타임에서 "SyntaxError: Unexpected keyword 'export'"를 방출하지 않고 launch + phase 라벨 기록이다. numeric tolerance는 적용되지 않으며(이진 launch 성공/실패), hermetic이 아닌 operational 오라클이다. hermetic half S1/S2/S3가 양 route 계약을 닫는다.
- **S11 (segment/게이트 순서 오라클)**: 예상 값은 생성 JS의 segment A 가드 마지막 phase가 `phase('gate_build_test')`, segment B 가드 첫 phase가 `phase('annotation')`이고, 디스패처 문서가 segment A launch → `auto workflow gate`(verdict_source: exit_code) barrier → pass 시 segment B launch → `auto check` barrier 순서와 게이트 실패 시 segment B 미launch를 명세함이다. 단일 launch가 전 phase를 실행한다는 false barrier 모델을 닫는다.
- **S12 (runtime caps 오라클)**: 예상 출력은 ultra fan_out_cap=5/verify_votes=3/synthesis=true, baseline verify_votes=1/synthesis=false, 그리고 depth cap 상수 MaxFanOut=5/MaxVerifyVotes=3/MaxRetry=3 + fan_out_cap=6 거부이며, 생성 JS가 이를 args.quality에서 읽고 env(/JSON.parse(env( 형태가 아님이다.

structural-only 신호로 Must를 닫지 않는다. 각 expected value/expected output은 실제 코드·계약에서 도출했고 임의 placeholder가 아니다.
