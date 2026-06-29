# SPEC-HARNESS-WORKFLOW-STABILITY-001 수락 기준

모든 Must 시나리오는 oracle-first다 — 파일 존재, heading, exit code 0, non-empty output만으로는
닫지 않는다. coverage 로직은 stdout을 반환하는 `[NEW] CoverageRunner` seam으로, RALF/review-barrier/
security-priority 로직은 `pkg/workflow`의 순수 결정 함수(`RunGateRemediation`/`RunReviewBarrier`/
`ConsolidateReviewVerdict`)로, config 로직은 단위 테스트로 hermetic하게 검증한다.

## Test Scenarios

### S1: RunGateRemediation은 fail→progress→pass 시퀀스에서 fixer를 정확히 2회 세고 segment B를 launch
Given gate retry budget이 2이고 `MaxRetry`는 3이다.
And gate 평가 시퀀스가 `[]GateSignature{{BuildExit:1,TestExit:0},{BuildExit:0,TestExit:1},{BuildExit:0,TestExit:0}}`이다 (initial fail → 다른 signature fail → pass).
When `RunGateRemediation(2, evals)`를 호출한다.
Then 반환된 `GateRemediationDecision.FixerAttempts`는 정확히 2이다.
And `SegmentBLaunched`는 true이고 마지막 평가가 pass된 뒤에만 true가 된다.
And `Aborted`는 false이고 FixerAttempts는 retry budget 2를 초과하지 않는다.

### S2: RunGateRemediation은 연속 동일 signature에서 예산 잔존에도 circuit-break abort
Given gate retry budget이 3이다.
And gate 평가 시퀀스가 `[]GateSignature{{BuildExit:2,TestExit:0},{BuildExit:2,TestExit:0}}`이다 (초기 fail 후 동일 exit signature 재발).
When `RunGateRemediation(3, evals)`를 호출한다.
Then 반환된 `Aborted`는 true이고 `AbortReason`은 `circuit_break_no_progress`이다.
And `FixerAttempts`는 정확히 1이다 (초기 fail 뒤 fixer 1회 spawn → 재실행이 동일 signature → 두 번째 fixer 전에 circuit-break, retry budget 3이 남아도).
And `SegmentBLaunched`는 false이다.

### S3: coverage 84%는 gate FAIL, 85%는 gate PASS (oracle 경계)
Given coverage threshold가 85로 선언되어 있다.
And `[NEW] CoverageRunner.RunOutput`이 coverage 명령 stdout으로 `total: (statements) 84.0%`를 반환한다 (exit-code-only `CommandRunner`는 stdout이 없어 이 seam을 별도로 쓴다).
When `EvaluateCoverageGate(ctx, runner, coverageCmd, 85)`를 호출한다.
Then 반환 GateResult.Verdict는 `fail`이고 VerdictSource는 `exit_code`이다.
And 같은 호출을 stdout `total: (statements) 85.0%`로 반복하면 Verdict는 `pass`이다.
And stdout `total: (statements) 85.0001%`도 Verdict `pass`를 반환한다 (>= 경계).

### S4: coverage 출력 파싱 불가 시 fail-closed (LLM 개입 없음)
Given coverage threshold가 85로 선언되어 있다.
And `[NEW] CoverageRunner.RunOutput`이 coverage 명령 stdout으로 백분율 토큰이 없는 빈 문자열을 반환한다.
When `EvaluateCoverageGate(ctx, runner, coverageCmd, 85)`를 호출한다.
Then 반환 GateResult.Verdict는 `fail`이다 (파싱 실패는 pass로 둔갑하지 않는다).
And verdict는 오직 파싱된 숫자(또는 그 부재)에서만 도출되며 LLM verdict를 사용하지 않는다.

### S5: RunReviewBarrier는 반복 barrier에서 fixer를 예산만큼 쓰고 abort
Given review retry budget이 2이다 (`MaxRetry`=3 이내).
And consolidated review 라운드가 `[]ConsolidatedVerdict{{Barrier:true,Reason:"request_changes"},{Barrier:true,Reason:"request_changes"},{Barrier:true,Reason:"request_changes"}}`이다.
When `RunReviewBarrier(2, rounds)`를 호출한다.
Then 반환된 `FixerAttempts`는 정확히 2이다 (budget=2).
And `Aborted`는 true이고 `AbortReason`은 `review_budget_exhausted`이다.
And `ReleaseHygieneReached`는 false이다 (release_hygiene marker로 진행하지 않는다).

### S6: ConsolidateReviewVerdict는 security FAIL을 reviewer APPROVE보다 우선
Given 같은 review 라운드에서 reviewer가 APPROVE를, security-auditor가 FAIL을 반환한다 (reviewerApprove=true, securityFail=true).
When `ConsolidateReviewVerdict(true, true)`를 호출한다.
Then 반환된 `ConsolidatedVerdict.Barrier`는 true이다 (reviewer APPROVE에도 불구하고).
And `Reason`은 `security_fail`이다 (reviewer-only APPROVE는 통과시키지 않으며 security가 code-quality보다 우선).

### S7: autopus.yaml의 workflow.team_default=false가 실제 Go field로 unmarshal됨
Given autopus.yaml이 최소 유효 설정과 함께 다음을 포함한다:
```
workflow:
  team_default: false
```
When `config.Load(dir)`로 설정을 로드한다.
Then 로드된 `cfg.Workflow.TeamDefault`는 `false`이다 (무시되지 않고 실제로 파싱됨).
And 같은 키를 `team_default: true`로 두면 로드된 값은 `true`이다.

### S8: team_default 미설정 시 기본값 true (현행 동작 보존)
Given autopus.yaml에 `workflow:` 섹션이 전혀 없다.
When `DefaultFullConfig("proj")`로 기본 설정을 생성한다.
Then `cfg.Workflow.TeamDefault`는 `true`이다.
And `cfg.Workflow.CoverageThreshold`는 85이다.

### S9: 범위를 벗어난 coverage threshold는 named error로 거부됨
Given HarnessConfig의 `Workflow.CoverageThreshold`가 150으로 설정되어 있다.
When `cfg.Validate()`를 호출한다.
Then non-nil error가 반환되고 메시지에 `workflow` 와 `coverage_threshold`가 포함된다.
And 같은 값을 85로 두면 `Validate()`는 nil을 반환한다.

### S10: substrate-selection prose가 실 field 의미를 참조
Given `content/skills/harness-workflow.md`, `content/skills/agent-teams.md`,
`templates/claude/commands/auto-router.md.tmpl`, `templates/gemini/skills/agent-teams/SKILL.md.tmpl`.
When 네 파일에서 `team_default` 토큰을 검색한다.
Then 각 파일은 `workflow.team_default=false`를 실 `WorkflowConf.TeamDefault` opt-out으로 서술하고,
`--no-workflow` 플래그와 별개의 유효 opt-out 경로로 명시한다.
And 어떤 파일도 `team_default`를 미구현 phantom key로 남겨두지 않는다.

### S11: 새 schema field가 derived JS에 없으면 parity gate가 fail-closed
Given `route_team.schema.json`이 `testing` phase에 `coverage_threshold: 85`를 선언한다.
And generator가 그 token을 derived JS block에 emit하도록 수정되어 있다.
When `route_team.schema.json`의 `coverage_threshold`를 90으로 바꾸되 derived JS는 85로 둔 채
parity check를 실행한다.
Then parity check는 non-nil error를 반환하고 diverging element(phase + coverage_threshold)를 명명한다.
And schema와 derived JS가 다시 일치하면 parity check는 nil을 반환한다.

### S12: route_a와 non-claude 플랫폼은 regression-0
Given route_a.schema.json은 `coverage_threshold`와 gate `retry` budget을 선언하지 않는다.
When 전체 parity gate와 `auto workflow render --route a`를 실행한다.
Then route_a 4-phase 순서·token 검사 결과는 이 SPEC 이전과 동일하다 (coverage/gate-retry token
검사가 발화하지 않음).
And codex/antigravity-cli/opencode는 route_team.workflow.js를 emit하지 않아 영향받지 않는다.

### S13: route_team generator는 multi-segment라 testing/review/release_hygiene이 서로 다른 segment guard에 들어간다 (interposition oracle)
Given route_team.schema.json은 8 phase를 선언하고 `testing`은 `coverage_threshold > 0`, `review`는 `verify_votes > 0`이다.
When `deriveTeamWorkflowJS(realSchema)`로 workflow JS를 생성한다.
Then 생성된 JS는 `if (SEGMENT === 'C')` guard와 `if (SEGMENT === 'D')` guard를 포함한다 (총 ≥4 segment guard).
And `phase('testing')`이 속한 segment guard의 인덱스 < `phase('review')`가 속한 guard 인덱스 < `phase('release_hygiene')`가 속한 guard 인덱스이다 (셋이 같은 guard에 묶이지 않는다).
And segment A의 마지막 `phase('...')`는 `gate_build_test`이고, route_a를 `deriveWorkflowJS`로 생성하면 여전히 `SEGMENT === 'A'`/`'B'` 2개 guard만 가진다 (route_a regression-0).

### S14: workflow 섹션이 없는 autopus.yaml은 Load 경로에서 team_default가 true로 backfill된다
Given autopus.yaml이 최소 유효 설정을 담되 `workflow:` 섹션을 전혀 포함하지 않는다.
When `config.Load(dir)`로 설정을 로드한다 (`applyMissingDefaults`가 누락 섹션을 채운다).
Then 로드된 `cfg.Workflow.TeamDefault`는 `true`이다 (zero-value false가 아니라 backfill된 기본값).
And 로드된 `cfg.Workflow.CoverageThreshold`는 85이다.
And 같은 파일에 `workflow:\n  team_default: false`를 두면 로드된 `cfg.Workflow.TeamDefault`는 `false`로 보존된다 (explicit이 backfill을 이긴다).

### S15: ParseSchema는 범위를 벗어난 coverage_threshold를 named error로 거부한다 (schema-level)
Given route-team 모양의 schema에서 한 phase가 `coverage_threshold: 150`을 선언한다.
When 그 schema 바이트로 `ParseSchema`를 호출한다.
Then non-nil error가 반환되고 메시지에 `coverage_threshold`가 포함된다.
And 같은 phase의 `coverage_threshold`를 85로 두면 `ParseSchema`는 error 없이 파싱된다.

### S16: workflow 섹션은 있으나 team_default가 생략되면 Load 경로에서 team_default가 true로 backfill된다
Given autopus.yaml의 `workflow:` 섹션이 존재하되 `team_default`는 생략하고 `coverage_threshold: 90`만 둔다.
When `config.Load(dir)`로 설정을 로드한다 (`applyMissingDefaults`가 present-section의 누락 필드를 채운다).
Then 로드된 `cfg.Workflow.TeamDefault`는 `true`이다 (생략된 bool이 zero-value false로 substrate를 silent 비활성화하지 않는다).
And 명시한 `cfg.Workflow.CoverageThreshold`는 90으로 보존된다 (필드-단위 backfill이지 섹션 전체 리셋이 아니다).

## Oracle Acceptance Notes

이 수락 기준의 Must 시나리오는 구조적 신호(파일 존재, heading, exit code 0)만으로 닫지 않고,
concrete expected output과 explicit tolerance를 가진 oracle로 검증한다. S1/S2/S5/S6는
`pkg/workflow`의 순수 결정 함수에 대한 단위 테스트이고, S3/S4는 stdout 반환 `[NEW] CoverageRunner`
seam을 주입한 `EvaluateCoverageGate` 단위 테스트다. S13은 생성된 route_team JS의 segment-guard 구조에
대한 launch-contract oracle, S14는 `config.Load`의 absent-section backfill oracle, S15는 `ParseSchema`의
schema-level coverage range oracle, S16은 present-section에서 team_default만 생략된 partial-omit
backfill oracle이다.

- **S1 RALF retry count (expected value)**: `RunGateRemediation(2, evals)`의 `FixerAttempts` 예상
  값은 정확히 2, `SegmentBLaunched=true`(마지막 pass 이후에만), `Aborted=false`. attempts는 budget 2를
  초과하지 않는다.
- **S2 circuit-breaker (expected value)**: `RunGateRemediation(3, evals)`에서 동일 exit signature
  2회면 예상 값은 `Aborted=true`, `AbortReason="circuit_break_no_progress"`, `FixerAttempts=1`,
  `SegmentBLaunched=false` (budget 3 잔존에도).
- **S3 coverage 경계 (expected value, numeric tolerance)**: 84.0% → `verdict=fail`, 85.0% →
  `verdict=pass`, 85.0001% → `verdict=pass`. tolerance는 `>= threshold` (포함 경계, epsilon 없음).
  예상 값은 `GateResult.Verdict` 문자열과 `VerdictSource="exit_code"`이며 stdout은 `[NEW]
  CoverageRunner.RunOutput`이 공급한다.
- **S4 파싱 실패 fail-closed (expected value)**: 백분율 토큰 부재 시 예상 출력은 `verdict=fail`
  (pass로 둔갑 금지).
- **S5 review barrier (expected value)**: `RunReviewBarrier(2, rounds)`에서 barrier 반복 시
  `FixerAttempts=2`, `Aborted=true`, `AbortReason="review_budget_exhausted"`,
  `ReleaseHygieneReached=false`.
- **S6 security 우선순위 (expected value)**: `ConsolidateReviewVerdict(true, true)`의 예상 값은
  `Barrier=true`, `Reason="security_fail"` (reviewer-only APPROVE 통과 금지).
- **S7/S8 config (expected value)**: `cfg.Workflow.TeamDefault`의 예상 값은 yaml에 따라 false/true,
  미설정 시 기본 예상 값 true, `CoverageThreshold` 기본 예상 값 85.
- **S9 validation (expected value)**: threshold=150의 예상 출력은 `workflow`/`coverage_threshold`를
  포함한 non-nil error, threshold=85는 nil.
- **S11 parity (expected value)**: schema 90 vs JS 85의 예상 출력은 diverging element를 명명한
  non-nil parity error, 일치 시 nil.
- **S13 multi-segment interposition (expected value)**: `deriveTeamWorkflowJS(realSchema)`의 예상
  출력 JS는 `if (SEGMENT === 'C')`와 `if (SEGMENT === 'D')` guard를 모두 포함하고(8-phase route_team에서
  ≥4 guard), `testing`의 segment 인덱스 < `review`의 인덱스 < `release_hygiene`의 인덱스이다. 이는
  파일 존재/heading이 아니라 생성된 JS 문자열의 구조적 oracle이다 — `testing`/`review`/`release_hygiene`이
  서로 다른 guard에 있어야 dispatcher가 coverage gate·review barrier를 interpose할 수 있다.
  route_a는 `deriveWorkflowJS`로 생성 시 예상 출력이 `SEGMENT === 'A'`/`'B'` 2개 guard뿐이다.
- **S14 absent-section Load default (expected value)**: `workflow:` 섹션 없는 autopus.yaml을
  `config.Load`로 읽으면 예상 값은 `cfg.Workflow.TeamDefault == true`, `cfg.Workflow.CoverageThreshold == 85`
  (`applyMissingDefaults` backfill). explicit `team_default: false`를 주면 예상 값은 `false`(보존).
  이 oracle은 S8(DefaultFullConfig)이 덮지 못하는 Load 경로를 검증한다.
- **S15 schema-level validation (expected value)**: `coverage_threshold: 150`을 담은 schema의
  `ParseSchema` 예상 출력은 `coverage_threshold`를 명명한 non-nil error, `85`는 error 없는 파싱.
  이는 config-level S9와 별개의 schema-level 경계다.
- **S16 partial-section Load default (expected value)**: `workflow:` 섹션이 present이고 `team_default`만
  생략(`coverage_threshold: 90`만 둠)된 autopus.yaml을 `config.Load`로 읽으면 예상 값은
  `cfg.Workflow.TeamDefault == true`(생략된 bool → 기본 true), `cfg.Workflow.CoverageThreshold == 90`
  (explicit 값 보존). 이는 S14(섹션 전체 부재)가 덮지 못하는 partial-section edge를 검증해, 부분 설정이
  substrate를 silent 비활성화하지 않고 backfill이 섹션 리셋이 아니라 필드-단위임을 보장한다.
