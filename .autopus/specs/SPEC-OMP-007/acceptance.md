# SPEC-OMP-007 수락 기준

## Oracle Acceptance Notes

- 모든 시나리오는 **Must**다. 숫자는 concrete expected output으로 정확 일치(numeric tolerance 0) 비교한다. `AC-nnn`은 헤더 순서대로 parser가 부여하는 ID와 같다.
- 기준 trajectory(A28, `omp-context-evidence-v0.50.117` report에서 `jq`로 추출): A input = `36045 + 25859*(k-1)` (k=1..10, 합 1,524,105); B input = `[36045,61904,83921,109780,104154,130013,114152,140011,117288,143147]` (합 1,040,415), compaction at session_sequence 3,5,7,9. 두 segment 동일. 기대값: per-pair `[0,0,438,338,2533,2137,4030,3550,5172,4674]`, median 2335, `effective_reduction_bp` 3174 (`ompContextReductionBasisPointsV1` 반올림).
- below-floor B′ = `[36045,61904,87763,113622,106000,131859,157718,183577,209436,235295]` (합 1,323,219): per-pair `[0,0,0,0,2400,2025,1751,1542,1378,1246]`, median 1312, effective 1318.
- late-compaction B‴ = `[36045,61904,87763,113622,139481,165340,55859,81718,107577,133436]` (합 982,745): per-pair `[0,0,0,0,0,0,7078,6235,5571,5035]`, median 0, effective 3552 — 실제로 줄이지만 v1 median은 놓치는 경우.
- maintenance-heavy fixture: B input을 A와 동일하게 두고 segment마다 compaction 1회에 `maintenance_input_tokens: 20000`을 붙이면 `effective_reduction_bp = round_half_away_from_zero((1524105 − (1524105 + 20000))·10000/1524105) = -131`이고(기존 `ompContextReductionBasisPointsV1`처럼 half-offset 뒤 0 방향 절단이므로 -132가 아니다), 진단 문자열에는 `neg131`로 렌더된다.
- effective chain fixture: report `runtime.omp_version`이 `omp/17.2.7`이면 policy `effective_method_order: ["snapcompact"]`, `omp/18.1.13`이면 `["remote","snapcompact"]`이며, `omp/19.0.0`은 검증된 chain row가 없어 런타임과 verifier가 모두 거부한다. fake OMP는 `--version`으로 세 문자열 중 하나를 낸다.
- OMP RPC는 테스트에서 fake 실행 파일(기존 `pipeline_omp_context_active_rpc_process_fixture_test.go`)로 대체한다. 실제 omp와 provider는 AC-010·AC-012에서만 쓴다.
- expected value가 있는 항목만 oracle이다. 파일 존재·exit code만으로 판정하는 시나리오는 없다.

## Test Scenarios

### S1: AC-001 유효 감축 20% 이상인 버전은 통과하고 compaction-method gate가 projection된다
Given 기준 trajectory로 만든 v2 report fixture(schema `autopus.omp_context_promotion_report.v2`, `runtime.omp_version: omp/17.2.7`, policy `min_effective_reduction_basis_points: 2000`, `compaction_method_order: ["remote","snapcompact"]`, `effective_method_order: ["snapcompact"]`, B의 compaction 4회/segment는 `compaction_method: snapcompact`, `maintenance_input_tokens: 0`, `maintenance_observability: local_render`)와 K3 fixture 서명.
When `VerifyOMPContextPromotionArtifactV2`를 호출한다.
Then 검증이 성공하고 gate 목록은 15행이며 `effective-reduction`은 `{status: passed, observed_value: "3174", required_value: "2000"}`, `compaction-method`는 `{status: passed, observed_value: "remote=0,snapcompact=8", required_value: "chain=snapcompact"}`, `median_reduction_basis_points`는 2335, `token-reduction` gate는 없다.
And 같은 B 입력을 `runtime.omp_version: omp/18.1.13`, `effective_method_order: ["remote","snapcompact"]`, 8회 compaction `compaction_method: remote`, `maintenance_input_tokens: 20000`, `maintenance_output_tokens: 500`, `maintenance_observability: observed_remote`로 바꾼 fixture는 `effective-reduction` observed `"2649"`(= round((1524105−(1040415+80000))·10000/1524105))로 통과하고 `compaction-method`는 `{observed_value: "remote=8,snapcompact=0", required_value: "chain=remote,snapcompact"}`다.
And 같은 18.1.13 fixture에서 8회를 `compaction_method: snapcompact`(chain의 두 번째 방법)로 바꾸면, `maintenance_observability: local_render`를 주장하는 판본은 `OMP context promotion report schema is invalid`로, `unobserved`를 적은 판본은 `OMP context promotion cohort gates failed`(`chain=remote_snapcompact methods=remote/0 snapcompact/8` 포함)로 실패한다.
And gate 목록에서 `compaction-method` 행의 `required_value`를 `"chain=snapcompact"`으로 바꾼 18.1.13 remote fixture는 `OMP context promotion gate projection mismatch`로 실패하고, `VerifiedOMPContextPromotion.EffectiveReductionBasisPoints()`는 3174/2649를 돌려준다.
And `runtime.omp_version: omp/19.0.0`인 fixture는 `effective_method_order` 값과 무관하게 `OMP context promotion report schema is invalid`로 실패한다.

### S2: AC-002 유효 감축 0 bp 또는 floor 미만인 버전은 실패한다
Given B input을 A와 동일하게 둔 v2 fixture(`omp/17.2.7`, compaction 2회/segment는 수행되었지만 감축 0)와 below-floor B′ fixture(`omp/17.2.7`, compaction 1회/segment); 두 fixture의 compaction은 모두 `maintenance_observability: local_render`, `maintenance_input_tokens: 0`이다.
When 각각 `BuildOMPContextPromotionReportV2`와 검증을 호출한다.
Then 두 경우 모두 `OMP context promotion cohort gates failed`로 실패하고 오류 문자열에 각각 `effective_reduction_bp=0/2000`, `effective_reduction_bp=1318/2000`이 있으며 `median_reduction_bp=0`, `median_reduction_bp=1312`가 함께 있다.
And below-floor fixture의 per-pair 값은 정확히 `[0,0,0,0,2400,2025,1751,1542,1378,1246]`이다.
And maintenance-heavy fixture(`omp/18.1.13`, B = A, segment마다 `remote` compaction 1회에 `maintenance_input_tokens: 20000`, `maintenance_observability: observed_remote`)는 `effective_reduction_bp=neg131/2000`을 포함한 `OMP context promotion cohort gates failed`로 실패하고 `median_reduction_bp=0`이 함께 있으며, 어떤 gate row도 projection되지 않는다.

### S3: AC-003 floor는 하드 상수고 median은 gate가 아니다
Given S1 fixture의 policy를 `min_effective_reduction_basis_points: 1500`으로 바꾼 report, 그리고 정상 report에 `token-reduction` gate row를 덧붙인 report.
When 검증한다.
Then 첫 경우는 `OMP context promotion policy is invalid`, 둘째는 `OMP context promotion gate projection mismatch`로 실패한다.
And 늦은 단일 compaction B‴ = `[36045,61904,87763,113622,139481,165340,55859,81718,107577,133436]`(합 982,745; call 7에서만 compaction, `omp/17.2.7`·`maintenance_observability: local_render`) fixture는 per-pair `[0,0,0,0,0,0,7078,6235,5571,5035]`, median 0으로 v1 oracle이라면 실패하지만 v2에서는 `effective-reduction` observed `"3552"`(expected value = round((1524105−982745)·10000/1524105))로 검증이 성공하고 `median_reduction_basis_points`는 0으로 기록된다.

### S4: AC-004 effective chain, 방법·거부·유지비 판독
Given fake OMP(`--version` = `omp/18.1.13`)가 manual `compact` 응답 `data`에 `{summary: "...", preserveData: {openaiRemoteCompaction: {version: "v2", provider: "openai-codex", replacementHistory: [...], usedTokens: 20000, usage: {inputTokens: 20000, outputTokens: 500, totalTokens: 20500}}}}`를 넣고 `get_messages_page`에 `{role: "compactionSummary", method: "remote", tokensBefore: 61904, tokensAfter: 30000, summary: "..."}`를 넣어 돌려주고, 다른 호출에서는 `compact` 응답 `success:false, error:"snapcompact would not reduce context locally."`, `error:"Nothing to compact (session too small)"`, `error:"Nothing to compact (no messages yet)"`, `error:"Already compacted"`를 차례로 돌려주며, 마지막 호출에서는 `method: "handoff"`를 돌려준다.
And 이 성공 fixture는 테스트용 correlated transport receipt로 provider attempt가 정확히 1회임을 증명한다. no-op fixture도 provider attempt가 0회임을 증명한다. 최종 OMP usage만 있는 경우에는 성공 기대값을 적용하지 않는다.
When 각 reused call을 `executeManaged`로 실행한다.
Then receipt는 순서대로 `{CompactionCycles:1, Method:"remote", Refusal:"none", MaintenanceInputTokens:20000, MaintenanceOutputTokens:500, MaintenanceObservability:"observed_remote", CompactionTokensBefore:61904, CompactionTokensAfter:30000}`, `{CompactionCycles:0, Method:"none", Refusal:"would_not_reduce"}`, `{CompactionCycles:0, Method:"none", Refusal:"too_small"}`, `{CompactionCycles:0, Method:"none", Refusal:"no_messages"}`이고, `Already compacted`는 오늘과 글자 같은 `managed active OMP manual compaction response is invalid: id_match=true command="compact" success=false ...`로, `handoff`는 `managed active OMP compaction method is unsupported: handoff`로 실패한다.
And fake OMP가 `--version` = `omp/17.2.7`이고 page의 `compactionSummary`에 `method`·`tokensAfter`가 없으며 이미지 digest 1건을 더하면(17.x 형태; 오버레이는 그대로 `[remote, snapcompact]`) receipt는 `{CompactionCycles:1, Method:"snapcompact", Refusal:"none", MaintenanceInputTokens:0, MaintenanceOutputTokens:0, MaintenanceObservability:"local_render", CompactionImages:1}`이고 그 run의 report policy는 `effective_method_order: ["snapcompact"]`이며, `--version` = `omp/18.1.13`인데 page에 `method`가 없으면 `managed active OMP compaction method is unobservable`로 실패하고, `--version` = `omp/19.0.0`이면 첫 reused call에서 `managed active OMP effective compaction chain is unverified: omp/19.0.0`으로 실패한다.
And `--version` = `omp/18.1.13`, page `method: "remote"`인데 `compact` 응답 `preserveData.openaiRemoteCompaction`에 `usage`가 없으면(V1 remote fallback 형태 `{provider, replacementHistory, compactionItem}`) `managed active OMP remote compaction usage is unobservable`로 실패하고 `CompactionCycles`는 0이다.
And legacy lifecycle fixture(`AUTOPUS_TEST_OMP_ACTIVE_LEGACY_COMPACTION=1`)가 `auto_compaction_start{reason: manual, action: remote}`와 page `method: "remote"`를 내면 수락되고, `action: snapcompact`와 page `method: "remote"`를 내면 `managed active OMP manual compaction start is invalid`로 실패한다.
And fake OMP가 `--version` = `omp/18.1.13`(chain `[remote, snapcompact]`)에서 page `method: "snapcompact"` 완료를 이미지 digest 1건과 함께 돌려주면 — remote 시도가 provider에 닿았는지 여부를 응답이 말하지 않으므로 — `managed active OMP compaction maintenance cost is unobservable`로 실패하고 `CompactionCycles`는 0이며, 같은 fake가 compaction 중 `extension_ui_request{method: "notify", message: "remote compaction failed; trying the next preferred method"}`를 내면 오늘과 글자 같은 `managed active OMP emitted unsupported UI activity`로 실패한다.
And `--version` = `omp/17.2.7`에서 `compactionSummary`가 이미지 digest를 하나도 더하지 않은 완료(provider LLM 요약 경로)는 `managed active OMP compaction maintenance cost is unobservable`로 실패하고 `CompactionCycles`는 0이다.
And remote가 내부에서 stream 오류 후 재시도해 성공하고 최종 usage 20000/500만 돌려주면 `blocked_unobservable`이며 numeric report가 없다. 전체 두 시도 비용이 correlated receipt에 각각 10000/200과 20000/500으로 증명된 별도 fixture는 maintenance input=30000/output=700으로 합산한다. 부분 비용을 20000/500으로 attest하는 구현은 실패다.

### S5: AC-005 compaction 경계·transcript novelty fail-closed와 probe 무서명
Given fake OMP(`--version` = `omp/17.2.7`)가 compaction 도중 `turn_start`를 내거나, pre-compaction page에는 `type` 토큰 `{text, toolCall, toolResult}`과 `role` 토큰 `{user, assistant, toolResult}`만 있는데 post-compaction page에 `{role:"assistant", content:[{type:"opaque", data:"..."}]}`를 넣거나, `{type:"Opaque-Item!"}`를 넣거나, post-compaction page에 `{role:"compactionSummary", images:[<PNG digest 1건>]}`와 `{type:"toolCall"}`만 더 넣는다.
When reused call을 실행한다.
Then 처음 세 경우는 각각 `managed active OMP provider activity crossed the compaction barrier`, `managed active OMP transcript introduced unclassified content type: opaque`, `managed active OMP transcript introduced unclassified content type: unprintable`로 실패하고 `CompactionCycles`는 증가하지 않으며, 네 번째 경우는 `CompactionCycles:1`·`MaintenanceObservability:"local_render"`로 성공한다.
And `--probe-dir`로 돌린 40-call fake cohort(위의 `opaque` 케이스 1건 포함)는 마지막 frame이 `{type:"error", error_code:"probe_completed", error_stage:"probe"}`이고 `.autopus/runtime/omp-context/promotion-report-v*.json`과 `evidence-v1.json`이 생기지 않으며 `<dir>/probe.jsonl`(mode 0600)은 40개 call record와 18개 compaction record(모두 숫자·bool·enum·digest 필드만)를 가지고, 해당 compaction record의 `content_types`는 `{opaque: 1}`을 포함하며 call은 실패하지 않는다.

### S6: AC-006 과거 v1 evidence는 바이트 단위로 동일하게 검증된다
Given 로컬 tag `omp-context-evidence-v0.50.93`…`v0.50.117` 23개(104·109 결번)의 `omp-context-promotion-report.v1.json`·`omp-context-promotion-attestation.v2.json` blob(변경 전 `git cat-file`로 보관한 SHA256 목록).
When 변경 후 코드로 각 tag에 `ompcontextverify --mode historical`과 `decodeOMPContextPromotionReportV1`을 실행한다.
Then 23건 모두 exit code와 stdout이 변경 전과 동일하고, A28(v0.50.117) report의 `token-reduction` observed는 `"2335"`, v0.50.93은 `"7089"`, v0.50.101은 `"3310"`, `multi-compaction-admission` observed는 각각 `"8/20"`, `"9/20"`, `"8/20"`이며, canonical 재직렬화 bytes가 원본과 `bytes.Equal`이다.
And v1 함수 `decodeOMPContextPromotionReportV1`, `validateOMPContextPromotionCohortV1`, `expectedOMPContextPromotionGatesV1`의 본문 diff는 비어 있다.

### S7: AC-007 schema 혼합·누락·미지 schema·중복 키는 거부된다
Given (a) `schema_version: autopus.omp_context_promotion_report.v3`, (b) v2 선언에 `compaction_method` 필드가 없는 observation, (c) v1 선언 body에 `compaction_method` 필드 추가, (d) v2인데 `min_reduction_basis_points`만 있고 `min_effective_reduction_basis_points`가 없음, (e) `schema_version` 키가 두 번(v1 다음 v2) 나오는 body, (f) v2인데 `runtime.omp_version: omp/17.2.7`이고 `effective_method_order: ["remote","snapcompact"]`.
When 검증한다.
Then (a)(b)(d)(f)는 `OMP context promotion report schema is invalid`를 포함한 오류로, (c)는 변경 전과 글자 같은 v1 오류(`decode OMP context evidence: json: unknown field "compaction_method"`)로, (e)는 변경 전 v1과 글자 같은 `decode OMP context promotion report: OMP context evidence contains invalid or duplicate key`로 실패하고 어느 경우도 다른 decoder로 재시도하지 않으며 (e)는 decoder 선택 전에 실패한다.

### S8: AC-008 gate verdict는 새 metric을 이름하고 body-free다
Given below-floor B′ cohort를 돌린 observe-session 실패(`omp/17.2.7`, chain `[snapcompact]`, compaction 2회 모두 `local_render`, refusal too_small 16), 그리고 maintenance-heavy cohort를 돌린 실패(`omp/18.1.13`, compaction 2회 모두 `remote`·`observed_remote`, 관측 유지비 segment당 20000, refusal would_not_reduce 16).
When 각 error frame과 `canary_failure_receipt final 1 <jsonl>`를 본다.
Then 첫 `gate_diagnostic`는 정확히 `pairs=20/20 compactions=2/2 effective_reduction_bp=1318/2000 maint_input_tokens=0 median_reduction_bp=1312 chain=snapcompact methods=remote/0 snapcompact/2 refusals=too_small/16 no_messages/0 would_not_reduce/0`이고 둘째는 `pairs=20/20 compactions=2/2 effective_reduction_bp=neg131/2000 maint_input_tokens=40000 median_reduction_bp=0 chain=remote_snapcompact methods=remote/2 snapcompact/0 refusals=too_small/0 no_messages/0 would_not_reduce/16`이며, 두 문자열 모두 `^[a-z_=/0-9[:space:]]*$`에 맞고 400바이트 이하다.
And receipt는 `final production canary gate verdict: ` 뒤에 각 문자열을 그대로 출력하고(음수 값도 필터에 걸려 사라지지 않는다), `gate_diagnostic`에 `assistant said: DROP TABLE`이 들어오면 verdict 줄을 출력하지 않는다.
And maintenance를 segment별 10000/30000으로 바꾸면 cohort 합 `maint_input_tokens=40000`은 유지하고 effective 값은 최솟값 `neg197`로 바뀐다.

### S9: AC-009 Verdict State Table이 script와 doctor를 함께 결정한다
Given 오라클 v2가 적용되고 초기 행(17.2.7 `in_use` oracle=v1 median 2335 bp; 18.1.2·18.1.5·18.1.13 `unmeasured`, v1 median 0·0·895 bp)이 들어 있는 저장소, 그리고 `18.1.13 refused oracle=v2 effective_reduction_bp=1300/2000`을 더한 fixture 표.
When `advance-omp-pin.sh 17.2.7`, `18.1.5`, `18.1.13 --dry-run`, `18.1.13 --measure`, `18.2.0`, `18.2.0 --measure`를 초기 표에서, `18.1.13 --measure`를 fixture 표에서 실행한다.
Then 17.2.7은 `already at omp/17.2.7`로 exit 0, 18.1.5는 `refused under oracle v1 median_reduction_bp=0/2000`과 `unmeasured under oracle v2 effective_reduction_bp`를 포함한 refusal로 실패, `18.1.13 --dry-run`은 `refused under oracle v1 median_reduction_bp=895/2000; unmeasured under oracle v2 effective_reduction_bp` 안내와 launch contract만 검사, `18.1.13 --measure`는 이동 허용, `18.2.0`은 `effective_reduction_bp`를 이름한 unmeasured refusal, `18.2.0 --measure`는 이동 허용이며, fixture 표의 `18.1.13 --measure`는 `effective_reduction_bp=1300/2000 oracle=v2`를 포함한 refusal로 실패한다.
And Go 표 테스트는 `advance-omp-pin.sh` case arm의 버전 집합과 Go 표의 버전 집합이 같음을 확인하고, `ompContextReductionReason`은 `omp/17.2.7 → measured_reduction_verified`(detail `oracle=v1 median_reduction_bp=2335/2000 compactions=8`), `omp/18.1.13 → reduction_unmeasured`, `omp/18.1.6 → reduction_unmeasured`, `omp/18.2.0 → reduction_unmeasured`, `"" → version_unavailable`을 돌려주며, fixture 표에서는 `omp/18.1.13 → measured_reduction_below_floor`(detail `oracle=v2 effective_reduction_bp=1300/2000`)를 돌려주고, doctor projection의 reason allowlist는 `measured_reduction_below_floor`를 수락하고 은퇴한 `measured_zero_reduction`은 어떤 경로에서도 나오지 않는다.
And pinned 17.2.7에 historical v1=2335와 complete v2=1300이 함께 있으면 script/doctor는 `refused`/`measured_reduction_below_floor`이며, historical 2335는 별도 필드에 남는다. 동일 핀의 v2 시도가 관측 불가로 중단된 fixture는 `unmeasured`/`reduction_unmeasured`, `last_attempt_reason`을 갖고 v2 숫자는 없으며 `in_use`로 덮어쓰지 않는다.

### S10: AC-010 probe cohort 1회가 가설을 판정한다 (live)
Given `[NEW] advance-omp-pin.sh 18.1.13 --probe`로 핀을 옮기고 `[NEW] OMP_CONTEXT_PROBE_DIR`을 설정한 뒤 `release-prep.sh --apply`를 한 번 실행한다.
When 실행이 끝난다.
Then full run은 `probe_completed`, call record 40개·compaction record 18개를 남기고 tag/report/evidence/측정 verdict는 없다. canary UID의 모든 프로세스가 종료된 뒤 동일 descriptor 검증을 거친 레코드만 runner 소유 0600 retained 파일로 발행된다. histogram과 usage key의 `openaiResponsesHistory`, `compactionSummary`, `toolCall`, `toolResult`, `inputTokens`, `outputTokens`, `totalTokens`는 대소문자를 유지해 서로 구별되고 `probe_records_rejected=0`이다.
And source 또는 상위 디렉터리를 symlink로 바꾸거나, protected sentinel 파일의 hardlink로 바꾸거나, 검사와 읽기 사이의 경로를 교체하는 fixture는 unverified bytes를 발행하지 않는다. sentinel 내용은 retained 파일과 로그 어디에도 없고, source fstat 대상과 read 대상은 같은 descriptor이며 destination은 배타적으로 생성된다. 잔존 canary UID 프로세스가 있으면 export 자체가 실행되지 않는다.
And partial run은 `probe_completed`를 반환하지 않고 완료 record 수와 body-free abort reason만 보존한다. identifier 범위 밖 토큰은 rejected count에 포함되어 allowlist 후보로 쓰이지 않는다.
And 각 call record의 `usage_identity_delta`는 기록되고 그 분포는 research.md에 적힐 뿐 통과 조건이 아니다.
And `method=remote`와 final usage 존재만으로 H1′ 또는 C2를 확정하지 않는다. 전 provider attempt 비용까지 관측되면 segment 순감축을 계산하고, 아니면 `maintenance_observability=unobserved`와 누락 관측원을 기록한다. runtime histogram은 T1의 semantic review 자료이며 그 자체로 allowlist를 승인하지 않는다.

### S11: AC-011 product 경로 동등성
Given `workflowContextProductOverlayBody(true/false)`와 managed product fixture(`--version` = `omp/18.1.13`).
When overlay body와 product validator 테스트를 돌린다.
Then 두 body 모두 `methodOrder: [remote, snapcompact]`를 포함하고 `strategy:`·`remoteEnabled`는 없으며 readback wants도 `[]any{"remote","snapcompact"}`이고, product validator는 `auto_compaction_start{reason: threshold, action: remote}`와 `action: snapcompact`를 수락하고 `action: soft`는 `managed OMP native compaction start is invalid`로 거부하며, 같은 fixture가 `--version` = `omp/17.2.7`이면 `action: remote`도 같은 오류로 거부하고, `--version` = `omp/19.0.0`이면 `action: snapcompact`·`action: remote` 둘 다 같은 오류로 거부한다.
And `pipelineOMPActivePolicyIdentity`는 `compaction-method-chain=remote,snapcompact;compaction-oracle=effective-reduction-v2;omp-pin=omp-v17.2.7`를 포함하고 `snapcompact-image-schema=`는 없으며 `TestOMPPinAgreesEverywhere`는 그 라벨에서 핀을 읽는다.

### S12: AC-012 측정 cohort와 문서 (completion evidence)
Given T0의 allowlist가 확정되고 T1–T6가 끝난 저장소에서 `advance-omp-pin.sh <version> --measure` 후 `release-prep.sh --apply`를 실행한다.
When cohort가 끝난다.
Then `passed`는 완전한 cohort의 net effective 값 ≥2000과 그 runtime의 effective chain에서 유도한 required_value를 갖는 서명 v2 evidence만 의미한다: 17.2.7은 `chain=snapcompact`, 18.1.13은 `chain=remote,snapcompact`다. `blocked_below_floor`는 완전한 cohort의 실제 숫자·oracle·cohort digest를 `refused`로 기록한다. `blocked_unobservable`은 조기 중단 이유·partial count를 `unmeasured`로 기록하며 감축률 숫자나 v2 report가 없다. 두 blocked 분기 모두 실제 release pin을 복원하고 그 핀에서 한 번 실제 cohort를 실행해 같은 세 분기로 기록한다. 그 핀도 실패하면 v2 발행은 blocked로 남는다. 17.2.7 historical v1=2335가 있어도 v2=1300은 `refused`이고 unobservable v2는 `unmeasured`이며, 어느 것도 failed v2 값을 `in_use` 값으로 기록하지 않는다. 서명 v1 이력은 별도로 보존하고 floor/C2/C4는 변경하지 않는다.
And 23개 과거 tag의 historical 검증 결과는 S6과 동일하다.

### S13: AC-013 method fallback checkpoints preserve ordering and replay safety
Given a stateful fake OMP using the measured 18.1.13 probe profile and `[remote,snapcompact]`, sending authenticated pre ID p1, waiting for its acknowledgement, sending pre ID p2, answering the intervening get_messages_page with the unchanged initial transcript, then sending one post ID q and the correlated compact success response.
When the probe executes the compact transaction.
Then it acknowledges p1 and p2 once each, performs canonical re-admission once at q, records pre_checkpoints=2/post_checkpoints=1 and attempt_coverage=unknown, and produces no report, attestation or tag.
And variants with p2=p1, a third pre, changed pre-transcript, post-before-pre, duplicate post, pre-after-post, mismatched binding or nonce, or a primary provider frame before completion fail closed and retain the appropriate body-free abort reason; no rejected checkpoint is acknowledged.
And the same repeated-pre exchange in a normal v1 producer, A session, unverified executable version/digest or different chain fails under the original single-pre rule; no production admission is widened by setting --probe-dir alone.
And a declined compaction after a bounded repeated pre is only accepted with the original unchanged-transcript and idle-session proof; its checkpoint counts never imply a successful compaction or known maintenance cost.
And while the second-pre proof query is waiting, injected agent_start, turn_start, turn_end, agent_end, prompt_result, a third pre, post, early compact response, extension_error or a response with another ID/command aborts the transaction before p2 is acknowledged. Authenticated injected checkpoints are counted but never ACKed. The fixture proves the strict reader does not silently consume a frame and then accept the later page response.
