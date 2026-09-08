# SPEC-OMP-007 수락 기준

## Oracle Acceptance Notes

- 모든 시나리오는 **Must**다. 숫자는 concrete expected output으로 정확 일치(numeric tolerance 0) 비교한다. `AC-nnn`은 헤더 순서대로 parser가 부여하는 ID와 같다.
- 기준 trajectory(A28, `omp-context-evidence-v0.50.117` report에서 `jq`로 추출): A input = `36045 + 25859*(k-1)` (k=1..10, 합 1,524,105); B input = `[36045,61904,83921,109780,104154,130013,114152,140011,117288,143147]` (합 1,040,415), compaction at session_sequence 3,5,7,9. 두 segment 동일. 기대값: per-pair `[0,0,438,338,2533,2137,4030,3550,5172,4674]`, median 2335, `effective_reduction_bp` 3174 (`ompContextReductionBasisPointsV1` 반올림).
- below-floor B′ = `[36045,61904,87763,113622,106000,131859,157718,183577,209436,235295]` (합 1,323,219): per-pair `[0,0,0,0,2400,2025,1751,1542,1378,1246]`, median 1312, effective 1318.
- late-compaction B‴ = `[36045,61904,87763,113622,139481,165340,55859,81718,107577,133436]` (합 982,745): per-pair `[0,0,0,0,0,0,7078,6235,5571,5035]`, median 0, effective 3552 — 실제로 줄이지만 v1 median은 놓치는 경우.
- OMP RPC는 테스트에서 fake 실행 파일(기존 `pipeline_omp_context_active_rpc_process_fixture_test.go`)로 대체한다. 실제 omp와 provider는 AC-010·AC-012에서만 쓴다.
- expected value가 있는 항목만 oracle이다. 파일 존재·exit code만으로 판정하는 시나리오는 없다.

## Test Scenarios

### S1: AC-001 유효 감축 20% 이상인 버전은 통과한다
Given 기준 trajectory로 만든 v2 report fixture(schema `autopus.omp_context_promotion_report.v2`, policy `min_effective_reduction_basis_points: 2000`, `compaction_method_order: ["remote","snapcompact"]`, B의 compaction 4회/segment는 `compaction_method: snapcompact`, `maintenance_input_tokens: 0`)와 K3 fixture 서명.
When `VerifyOMPContextPromotionArtifactV2`를 호출한다.
Then 검증이 성공하고 gate `effective-reduction`은 `{status: passed, observed_value: "3174", required_value: "2000"}`, gate `compaction-method`는 `observed_value: "remote=0,snapcompact=8"`, `median_reduction_basis_points`는 2335, `token-reduction` gate는 없다.
And 같은 B 입력에서 8회 compaction을 `compaction_method: remote`, `maintenance_input_tokens: 20000`으로 바꾼 fixture는 `effective-reduction` observed `"2649"`(= round((1524105−(1040415+80000))·10000/1524105))로 통과하고 `compaction-method` observed는 `"remote=8,snapcompact=0"`이다.
And `VerifiedOMPContextPromotion.EffectiveReductionBasisPoints()`는 3174/2649를 돌려준다.

### S2: AC-002 유효 감축 0 bp 또는 floor 미만인 버전은 실패한다
Given B input을 A와 동일하게 둔 v2 fixture(compaction 2회/segment는 수행되었지만 감축 0)와 below-floor B′ fixture(compaction 1회/segment).
When 각각 `BuildOMPContextPromotionReportV2`와 검증을 호출한다.
Then 두 경우 모두 `OMP context promotion cohort gates failed`로 실패하고 오류 문자열에 각각 `effective_reduction_bp=0/2000`, `effective_reduction_bp=1318/2000`이 있으며 `median_reduction_bp=0`, `median_reduction_bp=1312`가 함께 있다.
And below-floor fixture의 per-pair 값은 정확히 `[0,0,0,0,2400,2025,1751,1542,1378,1246]`이다.

### S3: AC-003 floor는 하드 상수고 median은 gate가 아니다
Given S1 fixture의 policy를 `min_effective_reduction_basis_points: 1500`으로 바꾼 report, 그리고 정상 report에 `token-reduction` gate row를 덧붙인 report.
When 검증한다.
Then 첫 경우는 `OMP context promotion policy is invalid`, 둘째는 `OMP context promotion gate projection mismatch`로 실패한다.
And 늦은 단일 compaction B‴ = `[36045,61904,87763,113622,139481,165340,55859,81718,107577,133436]`(합 982,745; call 7에서만 compaction) fixture는 per-pair `[0,0,0,0,0,0,7078,6235,5571,5035]`, median 0으로 v1 oracle이라면 실패하지만 v2에서는 `effective-reduction` observed `"3552"`(expected value = round((1524105−982745)·10000/1524105))로 검증이 성공하고 `median_reduction_basis_points`는 0으로 기록된다.

### S4: AC-004 방법과 거부 판독
Given fake OMP가 manual `compact` 후 `get_messages_page`에 `{role: "compactionSummary", method: "remote", tokensBefore: 61904, tokensAfter: 30000, summary: "..."}`를 넣어 돌려주고, 다른 호출에서는 `compact` 응답 `success:false, error:"snapcompact would not reduce context locally."`, 또 다른 호출에서는 `error:"Nothing to compact (session too small)"`, 마지막 호출에서는 `method: "handoff"`를 돌려준다.
When 각 reused call을 `executeManaged`로 실행한다.
Then receipt는 순서대로 `{CompactionCycles:1, Method:"remote", Refusal:"none", CompactionTokensBefore:61904, CompactionTokensAfter:30000}`, `{CompactionCycles:0, Method:"none", Refusal:"would_not_reduce"}`, `{CompactionCycles:0, Method:"none", Refusal:"too_small"}`이고 마지막은 `managed active OMP compaction method is unsupported: handoff`로 실패한다.
And 오버레이 `methodOrder`가 `[snapcompact]` 하나뿐이고 page에 `method`가 없으면(17.x 형태) `Method:"snapcompact"`으로 추론되며, `[remote, snapcompact]`인데 `method`가 없으면 `managed active OMP compaction method is unobservable`로 실패한다.

### S5: AC-005 compaction 경계 fail-closed와 probe 무서명
Given fake OMP가 compaction 도중 `turn_start`를 내거나, post-compaction page에 `{role:"assistant", content:[{type:"opaque", data:"..."}]}`를 넣는다.
When reused call을 실행한다.
Then 각각 `managed active OMP provider activity crossed the compaction barrier`, `managed active OMP transcript contains unclassified content: opaque`로 실패하고 `CompactionCycles`는 증가하지 않는다.
And `--probe-dir`로 돌린 40-call fake cohort는 마지막 frame이 `{type:"error", error_code:"probe_completed", error_stage:"probe"}`이고 `.autopus/runtime/omp-context/promotion-report-v*.json`과 `evidence-v1.json`이 생기지 않으며 `probe.jsonl`은 40개 call record와 18개 compaction record(모두 숫자·enum·digest 필드만)를 가진다.

### S6: AC-006 과거 v1 evidence는 바이트 단위로 동일하게 검증된다
Given 로컬 tag `omp-context-evidence-v0.50.93`…`v0.50.117` 23개(104·109 결번)의 `omp-context-promotion-report.v1.json`·`omp-context-promotion-attestation.v2.json` blob(변경 전 `git cat-file`로 보관한 SHA256 목록).
When 변경 후 코드로 각 tag에 `ompcontextverify --mode historical`과 `decodeOMPContextPromotionReportV1`을 실행한다.
Then 23건 모두 exit code와 stdout이 변경 전과 동일하고, A28(v0.50.117) report의 `token-reduction` observed는 `"2335"`, v0.50.93은 `"7089"`, v0.50.101은 `"3310"`, `multi-compaction-admission` observed는 각각 `"8/20"`, `"9/20"`, `"8/20"`이며, canonical 재직렬화 bytes가 원본과 `bytes.Equal`이다.
And v1 함수 `decodeOMPContextPromotionReportV1`, `validateOMPContextPromotionCohortV1`, `expectedOMPContextPromotionGatesV1`의 본문 diff는 비어 있다.

### S7: AC-007 schema 혼합·누락·미지 schema는 거부된다
Given (a) `schema_version: autopus.omp_context_promotion_report.v3`, (b) v2 선언에 `compaction_method` 필드가 없는 observation, (c) v1 선언 body에 `compaction_method` 필드 추가, (d) v2인데 `min_reduction_basis_points`만 있고 `min_effective_reduction_basis_points`가 없음.
When 검증한다.
Then (a)(b)(d)는 `OMP context promotion report schema is invalid`를 포함한 오류로, (c)는 변경 전과 글자 같은 v1 오류(`decode OMP context evidence: json: unknown field "compaction_method"`)로 실패하고 어느 경우도 다른 decoder로 재시도하지 않는다.

### S8: AC-008 gate verdict는 새 metric을 이름하고 body-free다
Given below-floor B′ cohort를 돌린 observe-session 실패(compaction 2회, refusal too_small 4·would_not_reduce 12, method snapcompact 2).
When error frame과 `canary_failure_receipt final 1 <jsonl>`를 본다.
Then `gate_diagnostic`는 정확히 `pairs=20/20 compactions=2/2 effective_reduction_bp=1318/2000 median_reduction_bp=1312 methods=remote/0 snapcompact/2 refusals=too_small/4 would_not_reduce/12`이고 `^[a-z_=/0-9[:space:]]*$`에 맞으며 400바이트 이하다.
And receipt는 `final production canary gate verdict: ` 뒤에 같은 문자열을 그대로 출력하고, `gate_diagnostic`에 `assistant said: DROP TABLE`이 들어오면 verdict 줄을 출력하지 않는다.

### S9: AC-009 verdict table 의미와 doctor 정렬
Given 오라클 v2가 적용된 저장소.
When `advance-omp-pin.sh 17.2.7`, `18.1.5`, `18.1.13 --dry-run`, `18.1.13 --measure`, `18.2.0`, `18.2.0 --measure`를 각각 실행한다.
Then 17.2.7은 `already at`로 종료, 18.1.5는 `oracle=v1`, 측정 cohort manifest digest, `effective_reduction_bp`를 포함한 refusal로 실패, `18.1.13 --dry-run`은 `refused under oracle v1; unmeasured under oracle v2` 안내와 launch contract만 검사, `18.1.13 --measure`는 이동 허용, `18.2.0`은 unmeasured refusal, `18.2.0 --measure`는 이동 허용이며 상태는 세 가지(in use / refused / unmeasured)뿐이다.
And Go 표 테스트는 `advance-omp-pin.sh` case arm의 버전 집합과 doctor 표의 버전 집합이 같음을 확인하고, `ompContextReductionReason`은 `omp/17.2.7 → measured_reduction_verified`, `omp/18.1.13 → measured_zero_reduction`(v2 재측정 전), `omp/18.2.0 → reduction_unmeasured`, `"" → version_unavailable`을 돌려준다.

### S10: AC-010 probe cohort 1회가 가설을 판정한다 (live)
Given `advance-omp-pin.sh 18.1.13 --probe`로 핀을 옮기고 `OMP_CONTEXT_PROBE_DIR`을 설정한 뒤 `release-prep.sh --apply`를 한 번 실행한다.
When 실행이 끝난다.
Then 마지막 frame은 `error_code=probe_completed`이고 tag·evidence store·report·verdict row는 생기지 않으며, `probe.jsonl`은 call record 40개와 compaction record 18개를 가지고 모든 call record에서 `stats_after.input − stats_before.input == Σ turn_usages(input+cacheRead+cacheWrite) + branch_usage_delta`(tolerance 0; `branch_usage_delta`는 18.x `getSessionStats`가 더하는 branch 항목 usage의 변화량으로 record에 기록되며 기대값 0)이 성립하며 각 compaction record의 `method`와 `refusal`은 enum 값이다.
And 판정 규칙: `method=remote` 완료가 ≥1건이고 그 다음 B call의 input delta가 직전 B call보다 작으면 H1′ 확인, remote 완료 0건이면 H1′ 반증; `refusals` 분포가 H2를, `would_not_reduce` 직후 pair의 A/B 청구값과 `tokensBefore/After`가 H3/H4를 결정하며, 요약 숫자(compactions, methods, refusals, segment ΣA/ΣB, effective bp)가 research.md 실측 표에 추가된다.

### S11: AC-011 product 경로 동등성
Given `workflowContextProductOverlayBody(true/false)`와 managed product fixture.
When overlay body와 product validator 테스트를 돌린다.
Then 두 body 모두 `methodOrder: [remote, snapcompact]`를 포함하고 `strategy:`·`remoteEnabled`는 없으며 readback wants도 `[]any{"remote","snapcompact"}`이고, product validator는 `auto_compaction_start{reason: threshold, action: remote}`와 `action: snapcompact`를 수락하고 `action: soft`는 `managed OMP native compaction start is invalid`로 거부한다.
And `pipelineOMPActivePolicyIdentity`는 `compaction-method-chain=remote,snapcompact;compaction-oracle=effective-reduction-v2;omp-pin=omp-v17.2.7`를 포함하고 `snapcompact-image-schema=`는 없으며 `TestOMPPinAgreesEverywhere`는 그 라벨에서 핀을 읽는다.

### S12: AC-012 측정 cohort와 문서 (completion evidence)
Given Decision Needed가 확정되고 T1–T6가 끝난 저장소에서 `advance-omp-pin.sh <version> --measure` 후 `release-prep.sh --apply`를 실행한다.
When cohort가 끝난다.
Then 통과 시 서명 `omp-context-promotion-report.v2.json`이 `VerifyOMPContextPromotionArtifactV2`를 통과하고 gate `effective-reduction` observed ≥ 2000과 `compaction-method` observed가 기록되며 verdict table·doctor 표·`docs/runbooks/omp-pin-advance.md`·`CHANGELOG.md`에 같은 숫자가 적힐다; 실패 시 `gate_diagnostic`의 `effective_reduction_bp=<n>/2000`이 refused row(oracle=v2)로 기록되고 핀은 17.2.7에 남는다.
And 23개 과거 tag의 historical 검증 결과는 S6과 동일하다.
