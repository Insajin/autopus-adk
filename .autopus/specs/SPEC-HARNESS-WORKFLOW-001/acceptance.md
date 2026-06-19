# SPEC-HARNESS-WORKFLOW-001 수락 기준

모든 Must 시나리오는 oracle-first다. 구조적 신호(단순 존재 확인)만으로 Must를 닫지 않으며, 구체 기대 출력 또는 명시 허용오차를 포함한다. Priority(Must/Should)는 EARS type과 별도 축이다.

## Test Scenarios

### S1: manifest에서 결정적 생성
Priority: Must
Given content/workflows/route_a.md와 route_a.schema.json이 정본 4 phase 집합 planning, implementation, gate_build_test, release_hygiene를 선언한다.
When generate-templates를 동일 입력으로 두 번 실행한다.
Then 두 실행의 templates/claude/workflows/route_a.workflow.js.tmpl 산출물이 바이트 동일하다.
And 생성된 js.tmpl과 schema와 md에서 추출한 phase-id 집합이 정확히 planning, implementation, gate_build_test, release_hygiene로 서로 동일하다.
When 이어서 claude 어댑터 Generate를 임시 디렉터리에 실행한다.
Then .claude/workflows/route_a.workflow.js 첫 줄에 generated·직접편집 금지 경고 문구가 있다.

### S2: parity 게이트 fail-closed
Priority: Must
Given route_a.schema.json에서 phase gate_build_test를 제거해 manifest와 schema가 어긋난 상태다.
When generate-templates를 실행한다.
Then 종료 상태가 0이 아니다.
And stderr가 어긋난 phase 이름 gate_build_test를 보고한다.
And templates/claude/workflows/route_a.workflow.js.tmpl이 기록되거나 갱신되지 않는다.

### S3: 비-claude 플랫폼 회귀 0
Priority: Must
Given codex, gemini, opencode 어댑터를 각각 임시 디렉터리에 생성한다.
When 각 어댑터의 Generate를 실행한다.
Then 세 플랫폼 산출물에서 이름이 workflow를 포함하는 .js 파일 개수가 정확히 0이다.
And 세 플랫폼 산출물 텍스트에서 --workflow 토큰 개수가 정확히 0이다.
And 비-claude 산출물에 harness-workflow 스킬의 --workflow 언급이 없다(claude-scoped 설치).
And 각 플랫폼에 기존 Route A 커맨드 표면이 존재한다.

### S4: doctor가 누락된 required 프리미티브를 fail-fast로 보고
Priority: Must
Given capability 프로버가 required 프리미티브 schema를 unavailable로 보고하도록 주입된다.
When auto workflow doctor를 실행한다.
Then 종료 상태가 0이 아니다.
And 구조화 리포트의 schema status 값이 unavailable이고 required로 표시된다.
And 리포트의 overall verdict 값이 fail이다.

### S5: doctor 실패 시 라우트가 Route A로 폴백
Priority: Must
Given doctor overall verdict 값이 fail이다.
When /auto go --workflow 라우트가 해석된다.
Then 라우트가 어떤 workflow도 실행하지 않고 Route A로 진입한다.
And 폴백 클래스가 fail-fast로 분류된 로그 라인이 방출된다.

### S6: drift gate가 SoT 변경 없는 generated 표면 staging을 차단
Priority: Must
Given staged 경로 목록에 .claude/workflows/route_a.workflow.js가 있고 content/workflows/ 아래에는 변경이 없다.
When release hygiene drift gate를 실행한다.
Then 종료 상태가 0이 아니다.
And 차단 메시지가 staged generated 표면 .claude/workflows/route_a.workflow.js를 목록으로 보고한다.

### S7: dry-run 렌더의 결정적 phase 순서
Priority: Must
Given 정본 manifest가 주어진다.
When auto workflow render --dry-run을 실행한다.
Then 표준 출력이 phase를 planning, implementation, gate_build_test, release_hygiene 순서로 나열한다.
And gate_build_test phase가 verdict_source 값으로 exit_code를 선언한다.
And 출력에 manifest, schema, prompt-manifest 해시가 포함된다.

### S8: fake CommandRunner replay의 exit-code 게이트
Priority: Must
Given gate phase에서 빌드 명령 종료 상태를 1로 반환하는 fake CommandRunner가 pkg/workflow/gate.go seam에 주입된다.
When deterministic Gate를 평가한다.
Then Gate verdict가 agent 텍스트가 아니라 빌드 종료 상태에서 파생되어 fail이다.
And gate phase의 verdict_source 값이 exit_code다.
And manifest에서 파생된 phase 실행 순서가 planning, implementation, gate_build_test, release_hygiene와 일치한다.

### S9: fallback taxonomy totality
Priority: Must
Given 유도된 실패 입력 집합 비-claude 플랫폼, doctor 실패, parity 드리프트, 실행 중단, API 부재가 주어진다.
When fallback 분류기로 각 실패를 분류한다.
Then 각 실패가 fail-fast, fail-closed, resumable, explicit 중 정확히 하나의 클래스에 매핑된다.
And 어떤 실패도 미분류이거나 silent로 남지 않는다.

### S10: Go 경계 — worktree 슬롯 cap
Priority: Must
Given 기본 RunConfig로 8개의 implementation worktree 태스크가 주어진다.
When ParallelRunner가 태스크를 스케줄링한다.
Then 동시에 실행되는 worktree 태스크가 최대 5개다.
And branch 명명과 worktree reclaim이 Go 런타임에서 수행되고 workflow JS에서 수행되지 않는다.

### S11: prompt-manifest 해시 결정성
Priority: Must
Given 동일한 stable+snapshot 컨텍스트가 주어진다.
When auto workflow render --dry-run을 두 번 실행한다.
Then 두 실행의 prompt-manifest 해시가 동일하다.
And stable 레이어 파일을 변경하면 해시가 달라진다.
And ephemeral 컨텍스트만 변경하면 해시가 변하지 않는다.

### S12: doctor 버전 핀
Priority: Should
Given claude-code 버전이 2.1.140으로 최소 핀 2.1.154보다 낮게 프로브된다.
When auto workflow doctor를 실행한다.
Then 리포트가 버전이 최소 핀 미만임을 보고한다.
And overall verdict 값이 fail이다.

### S13: release hygiene가 Lore/300줄 위반을 차단
Priority: Must
Given staged 변경에 301줄짜리 신규 .go 소스 파일이 포함되고 커밋 메시지가 Lore 형식이 아니다.
When release hygiene 종단 phase의 enforcement를 실행한다.
Then 종료 상태가 0이 아니다.
And 차단 리포트가 300줄 초과 파일 경로를 보고한다.
And 차단 리포트가 Lore 형식 위반을 보고한다.

### S14: doctor advisory 프리미티브는 게이트를 막지 않는다
Priority: Should
Given capability 프로버가 advisory 프리미티브 isolation을 unavailable로, required 프리미티브 agent와 schema와 phase를 available로 보고한다.
When auto workflow doctor를 실행한다.
Then 종료 상태가 0이다.
And 리포트의 isolation status 값이 unavailable이고 advisory로 표시된다.
And 리포트의 overall verdict 값이 pass다.

### S15: `/auto go --workflow` 라이브 디스패치 실행 (operational)
Priority: Must
Given claude-code 환경에서 workflow capability가 available하고 fixture SPEC이 주어진다.
When `/auto go --workflow`를 실행한다.
Then 세션 workflows 디렉터리에 workflow run journal이 생성된다.
And journal이 phase planning, implementation, gate_build_test, release_hygiene를 순서대로 기록한다.
And gate_build_test phase가 auto workflow gate CLI를 호출하고 verdict_source exit_code를 기록한다.
And journal에 terminal verdict가 기록된다.
Note 이는 hermetic Go 단위 테스트가 아니라 operational/integration 오라클이다(markdown 라우터 + claude-code 전용 Workflow 툴이라 hermetic 불가). BS techstack 라이브 스모크(run journal 생성 실증)와 동류이며 구현 중 1회 실제 실행으로 검증한다.

### S16: auto workflow gate CLI이 exit-code에서 verdict JSON을 방출
Priority: Must
Given gate에 주입된 fake CommandRunner가 build 명령 종료 상태 1과 test 명령 종료 상태 0을 반환한다.
When auto workflow gate를 실행한다.
Then 표준 출력 JSON의 verdict 값이 fail이다.
And verdict_source 값이 exit_code다.
And build_exit 값이 1이고 test_exit 값이 0이다.

## Oracle Acceptance Notes

각 Must 시나리오는 structural-only 신호가 아니라 concrete expected output 또는 explicit tolerance를 가진다. S15는 operational 오라클(hermetic 아님)로 명시한다.

- S1: expected value — route_a.schema.json의 phase-id 집합 = 생성 js.tmpl 집합 = 정확히 {planning, implementation, gate_build_test, release_hygiene}, 각 phase-id가 route_a.md에 문자열로 존재; 두 generate 실행의 js.tmpl이 byte-identical; 이어 실행한 claude 어댑터 Generate 산출물 `.claude/workflows/route_a.workflow.js` 첫 줄에 generated 경고. (exact match + presence)
- S2: expected stdout/stderr — diverging phase 이름 `gate_build_test`가 stderr에 출력되고 종료 상태 != 0, js 미기록.
- S3: expected value — workflow JS 개수 = 0, `--workflow` 토큰 개수 = 0 (정확히 0, tolerance 없음), Route A 존재.
- S4/S12: expected json — capability 리포트의 `schema.status` = `unavailable`(S4, required) 또는 version < 2.1.154(S12), `overall` = `fail`, 종료 상태 != 0.
- S5: expected stdout — 폴백 클래스 = `fail-fast` 로그 라인, workflow 실행 0회.
- S6: expected stdout — 차단 목록에 `.claude/workflows/route_a.workflow.js` 포함, 종료 상태 != 0.
- S7: expected stdout — phase 순서 = [planning, implementation, gate_build_test, release_hygiene], `gate_build_test.verdict_source` = `exit_code`.
- S8: expected value — Gate verdict = `fail`(fake CommandRunner build exit=1에서 파생), `verdict_source` = `exit_code`, phase 순서가 S7과 동일.
- S9: expected value — 5개 실패 입력 각각이 4개 클래스 중 정확히 1개로 매핑(미분류 0건).
- S10: expected value — 최대 동시 worktree = 5 (WorktreeSlotCap 기본값).
- S11: expected value — 동일 입력 해시 동일, stable 변경 시 해시 상이, ephemeral 변경 시 해시 불변.
- S13: expected stdout — 차단 리포트에 300줄 초과 파일 경로(`auto check --arch --staged`) + 대기 메시지 Lore 위반(`auto check --lore --message`)이 둘 다 포함, 종료 상태 != 0.
- S14: expected json — `isolation.status` = `unavailable`(advisory), `overall` = `pass`, 종료 상태 = 0.
- S15: expected artifact (operational) — `/auto go --workflow` 실행 후 workflow run journal에 4 phase(planning/implementation/gate_build_test/release_hygiene) 순서 + gate phase의 `auto workflow gate` 호출 + terminal verdict 기록. hermetic 아닌 operational 오라클.
- S16: expected json — `auto workflow gate`(fake CommandRunner build exit=1, test exit=0) stdout JSON: `verdict`=`fail`, `verdict_source`=`exit_code`, `build_exit`=1, `test_exit`=0.

위 oracle은 예상 출력과 예상 값을 명시하므로 단순 구조 확인을 넘어선다.
