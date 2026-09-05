# SPEC-OMP-003 수락 기준
## Oracle Acceptance Notes

`C1`은 hermetic fake OMP `17.1.8` catalog다. credential 값은 없고 `auth_enabled` boolean만 있다.

| Exact selector | Family | Capabilities | Thinking | Auth |
|----------------|--------|--------------|----------|------|
| `anthropic/alpha-reasoner` | `anthropic` | `deep_reasoning,independent_dissent` | `high,xhigh` | true |
| `openai/beta-coder` | `openai` | `coding_tool_use,fast_validation,deterministic_transform,independent_dissent` | `medium,high` | true |
| `google/gamma-vision` | `google` | `vision_design` | `high` | true |
| `openai/unauthorized-coder` | `openai` | `coding_tool_use` | `high` | false |

Profile `P1`은 deep_reasoning=`anthropic/alpha-reasoner:xhigh`, coding_tool_use의 ordered candidates=
`openai/unauthorized-coder:high`, `openai/beta-coder:high`, independent_dissent의 ordered candidates=
`openai/beta-coder:high`, `anthropic/alpha-reasoner:high`, vision_design=`google/gamma-vision:high`,
fast_validation과 deterministic_transform=`openai/beta-coder:high`를 선언한다. `C2`는 `C1`에서
independent_dissent auth-enabled 후보를 `openai/beta-coder` 하나만 남긴 catalog다.

구조 존재, heading, exit code 0, non-empty output만으로 Must scenario를 PASS 처리하지 않는다.
각 시나리오의 표와 Then 절에 명시한 예상 값, exact JSON field, byte/mode, ordered row를 oracle로 비교한다.
## Test Scenarios
### S1: opt-in 부재와 platform 경계
Given SPEC-OMP-002 baseline config와 OMP profile이 없는 `HarnessConfig`가 있다.
When OMP Generate, Update, Preview, doctor를 실행한다.
Then generated OMP file bytes와 manifest-owned path 집합은 baseline과 같아야 한다.
And `PlatformToProvider("omp")`는 빈 문자열이며 `orchestra.providers.omp`는 없어야 한다.
### S2: paired 역할 행렬 projection
Given `P1`과 `C1`이 있고 legacy override는 없다.
When 역할 정책을 컴파일한다.
Then `plan=anthropic/alpha-reasoner:xhigh`, `task=openai/beta-coder:high`,
`advisor=anthropic/alpha-reasoner:high`, `vision=google/gamma-vision:high`이어야 한다.
And planner/executor/reviewer/ux-validator agent selector는 각각 `@plan`, `@task`, `@advisor`,
`@vision`이며 receipt effective selector와 동일해야 한다.
And source agent 16개는 `smol=[annotator,explorer]`, `slow=[architect]`,
`plan=[planner,spec-writer]`, `vision=[ux-validator]`, `designer=[frontend-specialist]`,
`tiny=[validator]`, `task=[debugger,deep-worker,devops,executor,perf-engineer,tester]`,
`advisor=[reviewer,security-auditor]`와 exact set equality여야 한다.
And mapping에 없는 source agent fixture는 `agent_role_unmapped`, capability와 맞지 않는 explicit role은
`role_capability_mismatch`로 실패해야 한다.
### S3: legacy tier compatibility
Given balanced legacy preset이 planner=`opus`, executor=`sonnet`, validator=`haiku`를 제공하고
명시적 agent override는 없다.
When v1 compatibility table을 적용한다.
Then requested capability는 각각 `deep_reasoning`, `coding_tool_use`,
`deterministic_transform`이고 native role은 `plan`, `task`, `tiny`여야 한다.
And `legacy_source`는 각각 `opus`, `sonnet`, `haiku`여야 한다.
### S4: exact selector와 thinking oracle
Given `C1`과 `anthropic/alpha-reasoner:xhigh` 요청이 있다.
When selector를 해석한다.
Then effective provider/model/thinking은 `anthropic`, `alpha-reasoner`, `xhigh`여야 한다.
And `anthropic/alpha-reason` 같은 fuzzy near-match는 `model_unknown`이어야 한다.
And `google/gamma-vision:xhigh`는 `thinking_unsupported`여야 한다.
And unknown native role은 `role_unknown`, unknown provider는 `provider_unknown`, exact provider 아래
없는 model은 `model_unknown`이어야 한다.
### S5: catalog missing/invalid fail-closed
Given empty, malformed, oversized, timeout catalog fixture와 required planner role이 있다.
When 각 fixture를 독립적으로 resolve한다.
Then status는 모두 `blocked`이고 reason은 각각 `catalog_empty`, `catalog_invalid`,
`catalog_oversized`, `catalog_timeout`이어야 한다.
And model/provider field를 runtime default라고 추정해 채우지 않아야 한다.
### S6: declared fallback order
Given executor candidates가 unauthorized-coder, beta-coder 순서이고 `C1`을 사용한다.
When executor를 resolve한다.
Then 첫 attempt는 `unauthorized`, 두 번째는 `selected`이며 effective selector는
`openai/beta-coder:high`여야 한다.
And fallback chain과 receipt attempt 배열은 선언 순서와 byte-for-byte 같아야 한다.
And `evidence_class`는 `availability`여야 한다.
### S7: fallback 소진과 explicit degraded
Given 모든 candidate가 missing 또는 unauthorized인 required reviewer fixture A와, 같은 입력에
`degraded_action=runtime_default`를 명시한 fixture B가 있다.
When 두 fixture를 독립적으로 resolve한다.
Then A는 status=`blocked`, reason=`no_compatible_candidate`, effective selector가 비어 있어야 한다.
And B는 status=`degraded`, degraded_reason=`explicit_runtime_default`여야 한다.
### S8: reviewer/security family diversity 성공
Given executor가 `openai/beta-coder`이고 reviewer/security 후보가
`openai/beta-coder`, `anthropic/alpha-reasoner` 순서이며 둘 다 `independent_dissent` compatible하다.
When paired diversity constraint를 적용한다.
Then reviewer와 security의 effective family는 `anthropic`이어야 한다.
And `family_diversity`는 `{status:"satisfied",executor:"openai",reviewer:"anthropic"}`여야 한다.
### S9: family diversity 부족
Given `C2`와 executor `openai/beta-coder`가 있어 auth-enabled independent_dissent 후보가 같은
`openai` family에만 있다.
When reviewer를 resolve한다.
Then 실행 가능 selector는 유지하되 diversity status=`degraded`, reason=`same_family_only`여야 한다.
And independent-provider/quorum/consensus evidence는 false여야 한다.
### S10: overlay byte와 mode 보존
Given SPEC-OMP-002가 이미 생성한 valid skills marker, comments, `${MODEL_ID}`, unknown keys를 가진
`.omp/config.yml` baseline과 mode `0640`, projection/readback을 구현하는 hermetic fake OMP runner가 있다.
When `config_mode=overlay` profile을 생성·갱신하고 fake runner에 exact overlay를 전달한다.
Then `.omp/config.yml`의 sha256와 mode는 전후 동일해야 한다.
And overlay는 mode `0600`이며 modelRoles/fallbackChains만 canonical projection해야 한다.
And fake runner가 기록한 `omp --config <overlay> config get modelRoles`와 `config get retry.fallbackChains` readback은 compiled projection과 정확히 같고 model request count는 `0`이어야 한다.
And overlay를 생략하거나 다른 overlay를 전달한 fixture는 activation mismatch로 실패하며 receipt의 invocation argv/config hash와 readback hash가 compiled projection을 가리켜야 한다.
### S11: project-managed 충돌과 array replace
Given user-owned key와 fingerprint가 없거나 다른 conflict fixture A, complete
`disabledProviders=["openai","groq"]` ownership과 matching fingerprint가 있는 fixture B가 있다.
When 두 fixture에 project-managed projection을 독립 실행한다.
Then A는 `managed_key_conflict` 또는 `array_ownership_required`로 실패하고 byte-identical이어야 한다.
And B의 effective array는 정확히 두 항목이며 append/merge된 제3 항목이 없어야 한다.
### S12: byte/mode/env/credential 보존
Given unmanaged YAML bytes, mode `0600`, env placeholder, secret sentinel이 있는 config가 있다.
When Generate, Update, Preview, failed transaction, Clean을 각각 수행한다.
Then unmanaged bytes·mode·placeholder는 각 동작 전후 동일해야 한다.
And generated files, logs, receipt, doctor JSON 어디에도 secret sentinel이 없어야 한다.
### S13: opt-in safety profile
Given user effective approval/isolation 값과 safety ownership이 없는 fixture A, supported version에서
explicit safety ownership으로 approval=`write`, isolation=`auto`를 가진 fixture B가 있다.
When 두 fixture에 routing을 독립 적용한다.
Then A는 두 값을 보존하고 receipt source=`user_effective`여야 한다.
And B의 effective tuple은 정확히 `write/auto`, source=`autopus_profile`이어야 한다.
### S14: receipt exact schema와 doctor row
Given `P1`과 `C1` resolution이 성공했다.
When receipt를 쓰고 doctor JSON을 생성한다.
Then schema_version=`autopus.omp-model-resolution.v1`, omp_version=`omp/17.1.8`, catalog fingerprint는
`sha256:`+64 lowercase hex여야 한다.
And `generated_at`은 RFC3339이고 `resolution_digest`는 이를 제외한 canonical body의 sha256여야 한다.
And 각 role row는 profile, config_source, requested/effective role, capability, provider, model,
selector, thinking, fallback_attempts, degraded_reason, family_diversity, safety_source를 가져야 한다.
And secret/auth value field는 없어야 한다.
And `git check-ignore .autopus/omp-model-resolution-v1.json`은 exact ignore rule을 반환하고
`git ls-files --error-unmatch`는 nonzero이며 receipt 생성 전후 `git status --short --untracked-files=all`은 같아야 한다.
### S15: doctor 독립 freshness 검증
Given receipt의 OMP version, catalog fingerprint 또는 projected selector 중 하나를 변조했다.
When doctor가 runtime을 다시 probe한다.
Then 각각 reason=`version_stale`, `catalog_stale`, `projection_mismatch`인 non-green role row가 있어야 한다.
And receipt에 적힌 성공 상태만 신뢰해 PASS하면 안 된다.
### S16: fallback/quorum 및 context 회귀
Given fallback과 advisor가 사용된 OMP run이 있다.
When orchestra evidence와 prompt context profile 회귀를 검사한다.
Then provider quorum 수는 증가하지 않고 judge/consensus evidence는 생성되지 않아야 한다.
And 기존 `pkg/promptlayer`와 완료된 context-engineering SPEC이 소유한 stable/snapshot/ephemeral classification과 delivery authority는 변경되지 않아야 하며, SPEC-OMP-004가 이를 대체하는 owner로 기록되지 않아야 한다.
### S17: 결정성과 canonical ordering
Given 같은 profile, normalized catalog, version, effective config를 map insertion order만 바꿔 100회 입력한다.
When compiler와 receipt writer를 실행한다.
Then modelRoles, fallbackChains, agent files, `resolution_digest`는 100회 모두 동일해야 한다.
### S18: hermetic verification과 coverage
Given fake OMP runner와 S1~S17 fixtures가 있다.
When focused/race/fuzz 후보와 full test, vet, build를 network/auth 없이 실행한다.
Then 모든 oracle이 통과하고 T1의 exact pre-change package coverage보다 감소하지 않아야 한다.
And 하나의 coverprofile에서 새로 추가되거나 수정된 routing-owned Go production file의 aggregate
statement coverage가 `85.0%` 이상이어야 한다.
And default source의 새 concrete provider/model literal은 0개여야 한다.
## Operator-Attested Must Scenarios

`R1`은 identity/version과 required settings는 유효하지만 strict normalization이 정확히 `catalog_metadata_insufficient`를 반환하는 bounded native raw catalog다. canonical selectors는 `anthropic/alpha-reasoner`, `openai/beta-coder`, `raw/undeclared`이고 auth/keyless semantic metadata는 없다. Explicit stored profile `P2`는 `catalog_trust=operator-attested`와 함께 alpha를 family=`anthropic`, capability=`deep_reasoning,independent_dissent`, thinking=`xhigh/high`로, beta를 family=`openai`, capability=`coding_tool_use`, thinking=`high`로 선언하고 `profile/missing`도 후보로 선언한다.
### S20: explicit opt-in 성공과 selector 교집합
Given selected name `p2`가 `role_model_policy.profiles.p2`에 `P2`로 실제 저장되어 있고 `R1` probe가 있다.
When OMP routing을 컴파일한다.
Then status/reason은 `ready/catalog_ready`여야 한다.
And projected selector set은 정확히 `[anthropic/alpha-reasoner,openai/beta-coder]`여야 하며 `raw/undeclared`, `profile/missing`, fuzzy selector는 없어야 한다.
And plan과 task effective tuple은 각각 `anthropic/alpha-reasoner:xhigh`, `openai/beta-coder:high`여야 한다.
And projected synthetic model은 `operator_attested=true`, `auth_enabled=false`, `keyless=false`여야 하며 role row는 `evidence_class=operator_attested`와 선언 family의 `effective_family`를 가져야 한다.
### S21: strict/default/built-in/automatic init 회귀
Given `R1`과 (A) empty 또는 `strict` trust의 explicit profile, (B) quality-derived built-in profile, (C) 저장 profile이 없는 automatic init을 각각 독립 준비한다.
When 각 경로가 catalog를 probe하거나 proposal/apply를 시도한다.
Then A와 B는 status/reason=`blocked/catalog_metadata_insufficient`여야 한다.
And C는 `autopus.yaml`을 byte-identical로 유지하고 `operator-attested`를 생성·상속하지 않으며 `catalog_metadata_insufficient`로 실패해야 한다.
And REQ-005/006 strict catalog, selector, auth-enabled 판정 결과는 기존 S4/S5와 같아야 한다.
### S22: non-metadata 실패는 attestation으로 우회하지 않음
Given explicit `P2`에 identity_unverified, required setting unsupported, catalog_invalid, catalog_empty, catalog_timeout, catalog_oversized fixture를 각각 주입한다.
When 각 fixture를 독립적으로 probe한다.
Then 모두 status=`blocked`이고 reason은 각각 `identity_unverified`, `model_setting_unsupported`, `catalog_invalid`, `catalog_empty`, `catalog_timeout`, `catalog_oversized`여야 한다.
And attested catalog models, projection, receipt는 생성되지 않아야 한다.
### S23: family/route validation과 evidence authority
Given (A) candidate family가 빈 profile, (B) 같은 exact selector를 상충하는 family로 선언한 profile, (C) role/capability pair가 불일치한 profile, (D) valid `P2`에서 executor=openai와 reviewer=anthropic인 resolution이 있다.
When validation과 paired diversity를 수행한다.
Then A와 B는 invalid operator-attested profile로, C는 기존 `role_capability_mismatch`로 projection 전에 실패해야 하며 오류는 해당 profile/capability 또는 agent 경로를 식별해야 한다.
And D는 `evidence_class=operator_attested`, `effective_family=anthropic`, `independent_provider_evidence=false`, `quorum_evidence=false`, `consensus_evidence=false`여야 한다.
### S24: receipt/doctor trust binding
Given S20 resolution receipt와 동일 runtime/profile을 doctor가 독립 reprobe한다.
When receipt와 doctor JSON을 비교한다.
Then receipt top-level은 `catalog_trust=operator-attested`와 catalog fingerprint를, 각 role은 `evidence_class=operator_attested`와 선언된 `effective_family`를 가져야 하며 doctor의 independent reprobe와 같아야 한다.
And synthetic model의 `operator_attested=true`, `auth_enabled=false`, `keyless=false` 조합 때문에 doctor는 auth-enabled 또는 keyless를 observed=true로 주장하지 않아야 한다.
And receipt의 catalog fingerprint 변조는 `catalog_stale`, catalog trust·role evidence class·effective family 변조는 기존 `projection_mismatch`인 non-green status여야 한다.
And secret/auth value field는 receipt와 doctor 어디에도 없어야 한다.
## Explicitly Approved Live Scenario
### S19: 최대 1-call live canary
Given 사용자가 exact selector, 예상 비용, redaction 범위를 명시적으로 승인했고 hermetic gate가 green이다.
When installed OMP로 고정 prompt를 호출한다.
Then provider 호출은 정확히 1회 이하여야 한다.
And actual `--config <overlay>` modelRoles/fallbackChains readback은 compiled projection과 같아야 한다.
And observed effective provider/model/thinking은 doctor resolution과 같아야 한다.
And receipt에는 token/latency/selector와 pass/fail만 남고 prompt body, response body, credential은 없어야 한다.

승인이 없으면 S19는 `not-run: explicit approval required`로 보고하며 S1~S18의 deterministic release
gate를 실패시키지 않는다. 승인 없는 실제 호출을 성공으로 간주해서도 안 된다.

**S18 결과 (2026-09-05)**: hermetic full test/vet/build green; routing-owned changed production file 41개 aggregate `89.91%`(≥85); routing-owned 파일의 package별 aggregate(`internal/cli 83.44`, `pkg/adapter/omp 88.95`, `pkg/config 97.02`, `pkg/content 98.75`)가 T1 base `b86fab06` package coverage(`81.3 / 88.8 / 89.6 / 95.2`) 이상; default source의 새 concrete provider/model literal 0개(기존 테스트 고정). 판정: PASS.

**S19 결과 (2026-09-05, 사용자 승인: "다 완주하자")**: autopus-workspace(`.omp/config.yml` ultra 프로필, doctor `model-routing.receipt supported/fresh/valid`)에서 role `autopus_validator`(doctor: agent=validator capability=deterministic_transform status=supported reason=selected)로 `omp --mode rpc --no-session --no-extensions --no-skills --no-lsp --tools read --model @autopus_validator` 세션을 열고 `get_state` readback 뒤 고정 prompt 1회를 보냈다. 컴파일된 projection `anthropic/claude-opus-5:xhigh` = readback(`provider=anthropic model=claude-opus-5 thinkingLevel=xhigh`) = 관측 assistant(`provider=anthropic model=claude-opus-5 stopReason=stop`). provider 호출 정확히 1회(messageCount 2). token input 4 / output 4 / cacheRead 12772 / cacheWrite 0, cost $0.0065, latency 1730ms(ttft 1204ms). receipt에는 위 수치·selector·pass만 기록하고 prompt/response body와 credential은 없다. 판정: PASS.
## Verification Commands

```bash
go run ./cmd/auto spec validate .autopus/specs/SPEC-OMP-003 --strict
go test ./pkg/config ./pkg/adapter/omp ./pkg/content ./internal/cli -count=1
go test ./pkg/config ./pkg/adapter/omp ./pkg/content ./internal/cli -cover
go test ./... -count=1
go vet ./...
go build ./...
```
