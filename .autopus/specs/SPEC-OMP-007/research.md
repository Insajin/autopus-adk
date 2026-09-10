# SPEC-OMP-007 리서치

Clarification Ledger unavailable — 이 SPEC은 direct 지시(Facts/Constraints/Contract)로 작성됐고 BS 파일이나 inline ledger는 없다. 지시문의 사실은 아래 표에서 실측/정적 증거로 재확인했고, 리뷰 rev 0의 finding은 spec.md `## Review Revision 1`에, 그 뒤 남아 있던 정직성·근거 문제(P-1–P-9)는 `## Review Revision 2`에 반영했다.

## Outcome Lock

- User-visible outcome: 릴리즈 핀이 evidence 오라클의 위치 취약성 때문에 정체되지 않고, 유효(provider 청구, 관측된 유지비 net) 컨텍스트 감축을 실제로 내는 OMP 버전은 evidence로 핀을 올릴 수 있으며, 내지 않는 버전은 여전히 refused되고, 현재 핀 17.2.7은 v2 오라클에서도 evidence를 **생성**할 수 있다 — 그 cohort가 floor를 넘는지는 미측정이고, 넘지 못하면 결과는 `blocked`로 기록되어 사용자 결정으로 escalate된다.
- Mandatory requirements: REQ-MEASURE-001/002, REQ-METHOD-001/002, REQ-ATTEST-001, REQ-COMPAT-001/002, REQ-DIAG-001, REQ-PIN-001, REQ-DOCTOR-001, REQ-PROBE-001, REQ-PRODUCT-001, REQ-CHECKPOINT-001.
- Explicit non-goals: floor 2000 bp 수치 완화, cohort 형태 변경, attestation envelope·키·lineage 변경, upstream 수정, `Already compacted` no-op 승격, 비용(통화) 계산, sandbox/tools 변경.
- Completion evidence: AC-001–AC-013, retained probe 요약, 23개 historical v1 동일 검증, 선택된 핀의 `passed` 또는 `blocked_below_floor`/`blocked_unobservable` 결과. 중단에는 수치를 만들지 않는다. checkpoint 개정안은 별도 current-input review 통과 후 구현하며, runbook·상태표·doctor를 동일한 결과로 갱신한다.

## Visual Planning Brief

```mermaid
sequenceDiagram
  participant H as observe-session
  participant B as OMP B (optimized; A는 compaction 없이 동일 순서)
  participant P as provider gateway
  H->>B: compact (reused call, overlay methodOrder [remote, snapcompact]) → pre-ACK / 거부 텍스트 / post-ACK
  B-->>P: (remote일 때만) /responses/compact → usage.input_tokens/output_tokens
  B->>H: response.data.preserveData.openaiRemoteCompaction.usage → maintenance_input/output_tokens
  H->>B: get_messages_page → compactionSummary.method (18.x) / 부재면 effective chain에서 유도 (17.x), tokensBefore/After (진단만)
  H->>B: get_session_stats(before) · prompt · get_session_stats(after)
  B->>P: request(input = 전체 history) → usage.input/cacheRead/cacheWrite
  Note over H: input_tokens = Δ(input+cacheRead+cacheWrite); effective_reduction_bp = (ΣA − (ΣB + ΣB_maint))·10000/ΣA per segment(부호 유지), gate = min ≥ 2000
```

## 현재 측정 경로 (verified: read)

| 값 | 어디서 어떻게 | 근거 |
|---|---|---|
| `observation.input_tokens` | `receipt.InputTokens = afterStats.Input − beforeStats.Input`; `Input = tokens.input + cacheRead + cacheWrite`(`get_session_stats`), before는 compaction 이후·prompt 직전, after는 terminal `agent_end` 이후 같은 세션; evidence는 `PrimaryProviderRequests: 1` 고정 | `internal/cli/pipeline_omp_context_active_usage.go:44-57`, `pipeline_omp_context_active_rpc_execute.go:48-58,68,80,104-121`, `workflow_context_runtime_observe_session_evidence.go:206-209` |
| compaction 자체의 usage | 현재 미측정. `manualCompact`는 `compact` 응답 `frame.Data`를 `nativeResult`로 받지만 `summary`만 검사; `get_session_stats`는 rewritten history 합이라 compaction 전후 delta는 compaction 요청 usage가 아니다(주석) → v2는 `frame.Data.preserveData.openaiRemoteCompaction.usage`를 읽고([STATIC], 아래 바이너리 표), 유지비 `0`은 `local_render`(chain 첫 방법으로 완료 + transcript walk가 돌려주는 새 이미지 digest ≥1) 관측에만 붙이며 usage 부재·chain 뒤 방법으로 완료·이미지 0건 완료는 `unobserved`로 fail-closed한다 | `pipeline_omp_context_active_lifecycle.go:107-128,140-150,251-256`, `_rpc_execute.go:57-58`, `_transcript.go:67-71,94-158` |
| per-pair reduction / median / floor | `ompContextReductionBasisPointsV1(full, optimized)` 반올림; 20 pair 정렬 후 10·11번째 평균; `Policy.MinReductionBasisPoints`는 2000 이외 거부, gate `token-reduction >= 2000`; gate 목록은 `expectedOMPContextPromotionGatesV1` projection과 `reflect.DeepEqual` | `pkg/promptlayer/omp_context_canary_reduce.go:59,117-135`, `omp_context_promotion_report_verify.go:48-49`, `omp_context_promotion_report_cohort.go:26-47`, `omp_context_promotion_report_authority.go:78-95` |
| `compactionCount` | B observation의 `CompactionProviderRequests` 합 = `receipt.CompactionCycles` = `manualCompact`가 true 반환한 횟수(0/1); no-op 3종은 false; 시도는 B의 `sequence > 0`인 모든 call 앞(segment당 9회) | `omp_context_promotion_report_cohort.go:198-205`, `_rpc.go:120`, `_evaluator.go:71`, `_lifecycle.go:28-40` |
| 거부 text | `pipelineOMPActiveCompactionNoopMessages` = `Nothing to compact (session too small)`, `Nothing to compact (no messages yet)`, `snapcompact would not reduce context locally.`; 그 외 text(예: `Already compacted`)는 `manual compaction response is invalid`로 실패 | `_lifecycle.go:28-40,107-123` |
| 방법 강제·lifecycle frame | 오버레이 `compaction: enabled false, methodOrder: [snapcompact], keepRecentTokens 256, reserveTokens 128`; 현 두 바이너리는 manual RPC `compact`에서 `auto_compaction_start/end`를 내지 않지만(응답 후 `!started`면 종료), 하네스는 legacy lifecycle을 받아 `auto_compaction_start{reason: manual}`의 `Action != "snapcompact"`와 `auto_compaction_end`의 `Action != "snapcompact"`을 실패시키고 fixture legacy 모드가 이 프레임을 emit한다; barrier 목록에 `turn_start`는 없다. OMP notice는 RPC 모드에서 UI host가 `extension_ui_request{method: notify}`로 내보내며(18.1.13 line 1441896-1441902), manual compaction 중에는 `validatePipelineOMPActiveBridgeFrame`(line 266-272)이 `confirm` 외 method를 `managed active OMP emitted unsupported UI activity`로 이미 fail-closed한다 | `workflow_context_runtime_product_overlay.go:143-157`, `_lifecycle.go:68-73,126-138,259-272`, `_rpc_process_fixture_test.go:183-197`; binary 17.2.7 line 1121697-1121830, 18.1.13 line 1311127-1311400, 1441896-1441902 |
| transcript 검사 | `validatePipelineOMPActiveMessageValue`는 kind 분류기가 아니라 nil/bool/number/string/array/map을 모두 허용하는 구조 walk다: 모든 string과 key는 sanitizer exact-pass, `type == image` map은 PNG 3-4키 + provenance, `image` 포함 key는 `compactionSummary.images`만 | `pipeline_omp_context_active_transcript.go:94-158` |
| verifier 엄격성 | size → UTF-8 → `rejectDuplicateOMPContextEvidenceKeysV1` → `DisallowUnknownFields` → canonical bytes 일치 → `schema_version == v1`; 중복 키 오류 text `OMP context evidence contains invalid or duplicate key` | `omp_context_evidence_verify.go:232-285`, `omp_context_promotion_report_verify.go:13-37` |
| OMP 버전 인지 | `omp --version` probe(`installedOMPVersionPattern` `^omp/N.N.N$`) → `setup.ompVersion` → report `runtime.omp_version`; 핀 라벨은 `pipelineOMPActivePolicyIdentity` | `workflow_context_runtime_canary.go:10`, `workflow_context_runtime_observe_session_setup.go:179-201`, `_evidence.go:164`, `pipeline_omp_context_active_process.go:21` |

## 바이너리 정적 증거 (`~/.cache/autopus/release/omp-v{17.2.7,18.1.13}-darwin-arm64`)

명령: `LC_ALL=C grep -a -n -F <s> <bin>`, `sed -n 'A,Bp' <bin>`(번들 JS는 줄 단위로 읽힘), `<bin> --version`. 프로바이더 호출 없음.

| 항목 | 17.2.7 | 18.1.13 | 의미 |
|---|---|---|---|
| `--version` | `omp/17.2.7` | `omp/18.1.13` | 자체 보고 일치 |
| 방법 선택·유지비 경로 | `compact()`(line 1121697-1121760): `compaction.strategy === "snapcompact"`로 로컬 snapcompact, `methodOrder` 없음(오버레이 키는 `config get`에만 echo); 렌더는 `N5A()`(1121795) 로컬 호출이고 vision 불가·예산 초과면 provider LLM 요약(`el6()` 1121838; notice 1121762)으로 내려간다 | `compact()`(line 1311127-1311183): `methodOrder` 순서로 remote(`fve`) → snapcompact(vision 모델) → soft; `#F()`(1312161-1312193)는 `{kind: needsLlm}`만 만들고 snapcompact 분기는 `uYe()`(1311265) 로컬 렌더; 실패 시 notice를 emit하고 다음 방법으로 재귀(1311387-1311388), 후보 소진 시 throw(1311185); 기본 `[remote, snapcompact, handoff, shake, soft]`(line 1038206) | effective chain: 17.x `[snapcompact]`, 18.x `[remote, snapcompact]` — 읽은 major만 표에 넣는다. chain 첫 방법 완료(+이미지)는 provider 요청 없음(유지비 0의 근거); 뒤 방법으로 끝난 완료는 앞 시도가 청구되었을 수 있고 응답에 usage가 없어 fail-closed 대상 |
| 거부 throw | `Already compacted`(1121727), `Nothing to compact (session too small)`(1121729); `no messages yet`는 UI `showWarning`(936502)만 | 같은 둘(1311200, 1311202) + `snapcompact would not reduce context locally.`(1311296, `ue >= pe` 로컬 추정 비교); `no messages yet`는 UI(1187427)만 | class 매핑 `too_small`/`no_messages`/`would_not_reduce`; `Already compacted`는 미변경 fail-closed |
| RPC `compact` 응답 | — | `case "compact"`(1442247)가 `e.compact()` 반환값 `{summary, shortSummary, firstKeptEntryId, tokensBefore, details, preserveData: oB(v)}`(1311364-1311373)를 data로; `oB`는 `snapcompact` 키만 제거(982799-982804); `method`는 응답에 없고 세션 entry에만 | `preserveData.openaiRemoteCompaction`이 응답에 남는다 |
| V2 remote usage | — | `Toe()`(985563) → `plo()`(985764-985782) → `{compactionItem, replacementHistory, usedTokens: usage.inputTokens, usage}`; `JJr()`(985823-985845)가 `response.completed.response.usage.input_tokens/output_tokens/total_tokens/cached_tokens` 파싱; `woe()`(985980-985990)가 `preserveData.openaiRemoteCompaction = {version: "v2", provider, replacementHistory, usedTokens, usage, retainedImageCount}` | v2 `maintenance_*_tokens` 측정원 |
| V1 remote fallback | — | V2 실패 시 `dSe()`(986583-986690) → `{provider, replacementHistory, compactionItem}`(usage 없음) → `xoe()`(986291) | remote인데 usage 부재 → fail-closed 사유 |
| `get_session_stats` | assistant `usage.input/cacheRead/cacheWrite` 합(1123806-1123850) | 동일 합산 + `XOa()`(1313829-1313842)가 마지막 compaction 이후 `model_usage` entry만 더함(1313914); `appendModelUsage`는 `purpose: "auto-thinking"`에서만 호출(1308337, 1155422) | compaction 요청 usage는 stats에 없다 |
| Responses usage 변환 | `input_tokens`, `cached_tokens`(652010-652050) | `Hke()`(918934-918975) → `R6e()`(916993-917006): `input = input_tokens − cached − cacheWrite`, `cacheRead = cached` → `input+cacheRead+cacheWrite = input_tokens` | primary와 maintenance가 같은 단위 |
| `compactionSummary` 메시지 | `summary, shortSummary, tokensBefore, providerPayload, blocks, images, warning`(712773-712785; `tokensAfter`·`method` 없음) | `Hm()`(984877-984893): `role: compactionSummary, tokensBefore, tokensAfter, method, providerPayload, blocks`; messages view(1452229-1452236)는 `method: d.method`, `providerPayload: ime(d)` = `{type: "openaiResponsesHistory", provider, items: replacementHistory}`(1452055-1452068); `get_messages_page`는 `e.messages` 페이징(1442308-1442318) | v2 `compaction_method` 측정원(18.x); remote 결과는 `type` 토큰 `openaiResponsesHistory`·`compaction`(985807-985812)을 transcript에 남긴다 [INFERENCE, probe 확인] |
| 프레임 usage / snapcompact 예산 | 세션 이벤트 그대로 전달 | `turn_end.message`/`agent_end.messages`는 `e.subscribe((O) => a(O))`(1441996), `a = encodeFrames`(1441846); `#A`: contextWindow·`keepRecent`·shape capacity로 maxFrames(1312194-1312213) | 프레임 `usage`는 probe 진단원; 18.x 이미지 예산이 다를 수 있음(H3) |

## 실측 표

| | omp/17.2.7 (A28, `omp-context-evidence-v0.50.117`) | omp/18.1.5 (2026-09-03) | omp/18.1.13 (A29, 2026-09-07) |
|---|---|---|---|
| calls / pairs | 40 / 20 (10:10) | 40 / 20 | 42/42 records |
| compaction cycles | **8** (B sseq 3,5,7,9 ×2 segment) | 2 | 2 |
| median reduction | **2335 bp** (A23 2321, A24 2309, A25 2309, A27 2335) | 0 bp | 895 bp |
| segment 누적 ΣA / ΣB | 1,524,105 / 1,040,415 → **3174 bp** (A23 3131, A24 3160) | 미보존 — v2 metric으로는 미측정(0으로 가정하지 않는다) | 미보존 — v2 metric으로는 미측정; median 895는 누적값을 함의하지 않는다 |
| 나머지 gate | pass | pass | pass |

A28 trajectory(`git cat-file blob <report> \| jq`): A input = 36045 + 25859·(k−1); B input = 36045, 61904, 83921(c), 109780, 104154(c), 130013, 114152(c), 140011, 117288(c), 143147. 첫 compaction은 history를 6.2%만 줄이고 이후 28.7%, 32%, 34.7%; per-pair `[0,0,438,338,2533,2137,4030,3550,5172,4674]×2`, median은 5·6번째 pair가 결정하고 8/20 pair는 구조적으로 ≤438 bp다. 같은 omp/17.2.7로 만든 23개 tag의 `token-reduction` observed: v0.50.93–97 **7084–7305**(impl `dd7be49d`, prompt ≈18.6–37.9K/call), v0.50.98–100 **2556**(impl `eb94cbb8`부터), v0.50.101–108 **3308–3310**(≈65K/call), v0.50.110–117 **2309–2335**(≈34–36K/call) — median은 (OMP × harness × workload)의 함수고 버전 비교는 같은 cohort manifest에서만 유효하다.

## 가설 (label: hypothesis)

| ID | 가설 | 찬성 | 반대 | 판정 수단 |
|---|---|---|---|---|
| H1 | 18.1.x가 감축 일부를 remote compaction에 넘겨 local 측정이 못 본다 | 18.0.0 `methodOrder` 기본 remote 우선; `dB()`가 openai-codex를 remote 대상으로 봄 | 측정원은 provider 청구 delta라 보였을 것; 오버레이 `[snapcompact]`로 remote 미실행; codex 요청은 `store: false`(985615-985620) | **정적으로 반증**. 남는 H1′: "하네스가 remote를 금지해 18.x의 선호 경로를 평가하지 못한다"([HYPOTHESIS]; "snapcompact 강제가 median 저하의 원인"이라는 인과 주장도 미확립) → probe(chain `[remote, snapcompact]`, `maintenance_usage_present`, `ui_request_methods`) |
| H2 | 18.1.x는 설계상 덜 자주 compaction한다 | 18.0.9 projection 거부; A29 2회 vs 8회 | 오버레이 임계값은 두 버전 동일; 거부는 projection 비교 | probe refusal class 분포 |
| H3 | 18.1.x snapcompact 출력이 같은 history에 대해 더 크다 | `#A` 프레임 예산·모델별 tokenizer 차이 | 수행된 2회의 전후 청구값이 evidence에 없고, 18.1.5(median 0)→18.1.13(median 895) 변화의 원인은 미검증 — 릴리즈 노트 v18.1.6·18.1.8·18.1.10 중 무엇이 기여했는지 증거 없음 [HYPOTHESIS] | probe의 compaction 직후 B input vs 직전, `tokensBefore/After` |
| H4 | 거부는 projection 아티팩트다 | 17.2.7은 gate 없이 같은 history를 감축(6.2%–34.7%) | 18.1.13에서도 18회 중 2회만 수행; 직접 증거 없음 | `would_not_reduce` 직후 pair의 A/B 청구값과 `tokensBefore` |

Upstream 릴리즈 노트(`gh api repos/can1357/oh-my-pi/releases`, 2026-09-08): v18.0.0 `compaction.strategy`/`remoteEnabled` → `compaction.methodOrder`(`[remote, snap]`); v18.0.9 PR #10024 "skip inflating snapcompact results"(거부 클래스의 기원); v18.1.6 provider-native compaction은 Responses Lite; v18.1.8 opaque reasoning 크기 오판 수정; v18.1.10 Codex V2 remote compaction prefix/prompt-cache(#10786); v18.1.13 compaction 노트 없음. 이 목록은 시점만 말한다 — 어떤 항목이 median 0→895 변화를 냈는지, 또는 냈는지 자체가 미검증이다.

## Backward compatibility 분석

- 서명·검증: attestation v2는 report bytes의 sha256과 trust lane만 서명(`omp_context_promotion_attestation_v2.go:14-36`, `_verify_v2.go:44-80`). v1 struct에 필드를 더하면 canonical 재직렬화가 깨지므로 v2는 별도 struct·decoder이고 dispatcher는 v1 pre-check(size→UTF-8→duplicate key) 뒤에 `schema_version`을 peek한다.
- v1 report struct 소비자(모두 T3/T6 목록): `VerifiedOMPContextPromotion.CanaryRows()`(`pipeline_omp_context_active_evidence.go:32,53-54`), `EvaluateOMPContextHistoryPromotionV1`(`omp_context_canary_promotion.go:37-66`), `pipeline_omp_context_cohort.go:132-148`, `omp_context_evidence_store.go:101`, `BuildOMPContextPromotionStaticPolicyV3`(`omp_context_promotion_static_policy.go:11`, lane은 `companion_omp_context_static_policy.go:73`로 호출), `matchesOMPContextPromotionStaticPolicyV3`(`omp_context_promotion_runtime_v3.go:233`), `computeOMPContextPromotionEvidenceIDV1`(`omp_context_promotion_evidence_id.go:15`), authority digest(`omp_context_promotion_report_authority.go:10,23`), `pipeline_omp_context_observation.go:115`(`MinReductionBasisPoints != 2000`).
- 파일명: release `omp-context-promotion-report.v1.json` 15개소(scripts/companion-release/*, release.yaml, companionmanifest 테스트) + runtime `promotion-report-v1.json` 4개소(`workflow_context_runtime_observe_session_command.go:21`, `omp_context_promotion_artifact_loader_v2.go:6-8`, `prepare-release-runtime-lib.sh:236`, `prepare-release.sh:240`) + `validate_canary` `(.gates | length) == 14`(`prepare-release-runtime-lib.sh:245`). 과거 tag는 `.v1.json` 그대로.
- 핀 표·doctor·diagnostic: `advance-omp-pin.sh:75-113`의 세 상태(refused는 `--measure`에도 거부), `doctor_omp_context_reduction.go:18-49`는 `omp/18.1.` prefix 전체를 `measured_zero_reduction`으로 묶음 → Verdict State Table + reason `measured_reduction_below_floor`로 대체(reason 열거 지점 3곳: `doctor_omp_context_reduction.go:37-49`, `doctor_omp_context_projection.go:122-124` allowlist, `docs/runbooks/omp-pin-advance.md:290-293`); `gate_diagnostic`은 `prepare-release-runtime-lib.sh:124` 정규식 `^[a-z_=/0-9[:space:]]*$`, ≤400(쉼표·`-` 불가 → chain 토큰은 `remote_snapcompact`, 음수 bp는 `neg<digits>`).
- canary 격리: `run_canary`(`prepare-release-runtime-lib.sh:147-231`)는 root 소유 isolated root 아래 `project/home/tmp`를 canary UID로 chown하고 `TMPDIR=$isolated_tmp`로 실행; 실패 시 `canary_failure_receipt`(213-214) 후 `set -e`로 `cleanup`(16-58)이 isolation root와 `temp_dir`(`final-output.jsonl` 포함, `prepare-release.sh:236`)를 삭제 → probe 파일은 receipt 직전에 `.autopus/runtime/`(gitignored) 아래로 반출해야 남는다.

## 설계 결정 / 대안 검토

- **측정원 유지(provider 청구 delta) + 유지비는 `compact` 응답 usage**: 두 값은 같은 `input_tokens` 단위(`R6e`). 기각: `get_session_stats` 전후 delta로 유지비 추정(rewritten history 감소와 섞임), `turn_end` 프레임(바리어 안에서는 위반), OMP 로컬 추정 attest. "유지비 0"은 가정이 아니라 `local_render` 관측(chain 첫 방법 완료 + 이미지 digest ≥1)에만 붙이고, remote usage 부재·chain 뒤 방법으로 완료·이미지 0건 완료는 `unobserved`로 fail-closed다 — C2가 조용히 C1으로 퇴화하지 않는다. remote usage 필드 자체는 [STATIC]이며 런타임 존재는 T0 probe가 확인한다.
- **effective chain을 검증된 major 표에서 유도**: 17.x는 `methodOrder`를 모르고 `method`를 쓰지 않으므로 오버레이 키만으로는 방법을 알 수 없다. 표에는 바이너리를 읽은 major(17, 18)만 넣고 그 외 major는 fail-closed다 — `major >= 18`을 규칙으로 쓰면 읽지 않은 미래 바이너리의 동작을 가정하게 된다. 대안 기각: (a) 17.x에 `[snapcompact]` 오버레이를 따로 쓰기 — product/evidence body가 버전별로 갈려 `production_path_equivalent` 주장이 약해짐; (b) legacy `auto_compaction_start.action` — 현 바이너리는 manual 경로에서 이 프레임을 내지 않음. 버전 major는 이미 서명 report에 있어 verifier가 교차 검사할 수 있다.
- **transcript 규칙 = 기존 구조 walk + novelty**: kind 분류기는 없으므로 "미분류"는 pre-compaction 대비 신규 `role`/`type` 토큰으로 정의하고 allowlist는 probe histogram 후 고정. 기각: 고정 allowlist 선언(정적 추론만으로는 remote item shape를 모름).
- **누적 집계(C4 segment min)**: median의 위치 취약성 때문. B‴(합 982,745)는 median 0으로 v1 실패지만 effective 3552 bp. 전체 20 pair 누적은 한 segment의 실패를 가릴 수 있어 기각.
- **Verdict State Table**: state→script→doctor 1:1. `in_use`의 verified는 "최신 release evidence가 자기 oracle의 gate 통과"로 정의해 v2 적용 직후에도 17.2.7이 정직하게 verified(oracle=v1 명시)다. `refused`의 doctor reason은 `measured_reduction_below_floor`이고 detail에 측정값·oracle을 적어 895/1300 bp를 "zero"라 부르지 않는다. 기각: refused@v1을 doctor에서 유지(script의 `unmeasured under v2`와 불일치), `measured_zero_reduction` 토큰 유지(측정값과 불일치).
- **`compaction-method`는 gate**: observed만 남기면 "모든 compaction이 chain 방법"이라는 주장의 전제를 서명하지 못한다. required `chain=…`와 pass 규칙을 projection에 넣는다.

## Minimality Decision Matrix

| Ladder step | Evidence | Decision | Receipt item |
|-------------|----------|----------|--------------|
| actual need | 핀이 evidence 오라클 때문에 정체; 5 cohort의 floor 여유 309–335 bp; 18.x는 강제된 방법으로 18회 시도 중 2회만 수행 | proceed | Outcome Lock |
| existing code/helper/pattern | `sessionStats` delta, `ReduceOMPContextCanaryPairsV1`, `manualCompact`(이미 `frame.Data` 보유), 구조 walk, `omp --version` probe, verdict table, `gate_diagnostic` 경로를 모두 재사용 | reuse | 측정원·프레임·페이지 API 불변 |
| stdlib/native | 누적 합·반올림은 `ompContextReductionBasisPointsV1` 산식 재사용; strict decode는 `encoding/json` 기존 helper | use | 새 라이브러리 없음 |
| existing dependency | ed25519 attestation, cobra flag, jq/shasum(scripts) 기존 것만 | reuse | — |
| new dependency or abstraction | 새 의존성 없음. 새 abstraction은 `ReportV2` struct + `ReduceOMPContextCanarySegmentsV2` + Verdict State Table(v1 byte-identical·단일 진실원을 위해 불가피) | accepted | schema dispatch 1개, 표 1개 |
| minimum sufficient verification | AC-001–AC-012: v2 verify fixture(두 chain, ≥20% pass / 0 bp fail), 23 tag historical byte-identical, diagnostic regex, pin script dry-run, product overlay 테스트, probe 1회, 측정 cohort 1회 | required checks | security(REQ-METHOD-002)·data-loss(evidence 미생성, probe 반출)·deterministic oracle 유지 |

## Semantic Invariant Inventory

| ID | source clause | invariant type | affected outputs | acceptance IDs |
|----|---------------|----------------|------------------|----------------|
| INV-001 | "measure compaction benefit … name the exact measurement source in the RPC/usage frames" — primary는 `get_session_stats` delta(provider 청구) | parser / data source | `input_tokens`, probe `stats_before/after` | AC-001, AC-010 |
| INV-002 | "a version that genuinely reduces context passes and one that does not still fails" — `effective_reduction_bp = round((ΣA − (ΣB + ΣB_maint))·10000/ΣA)` per segment(부호 유지, clamp 없음), gate = min ≥ 2000 | numeric formula / ordering | gate `effective-reduction`, `gate_diagnostic`(음수는 `neg<digits>`) | AC-001, AC-002, AC-003, AC-008 |
| INV-003 | "must NOT propose lowering the 2000 bp floor" — verifier는 policy floor 2000 이외 거부, median은 진단값 | bound | verifier error, `median_reduction_basis_points` | AC-002, AC-003, AC-007 |
| INV-004 | effective chain·방법·거부: `effective_method_order` = `pipelineOMPActiveEffectiveChains[major]`(17·18만 검증, 그 외 fail-closed), `compaction_method ∈ {none,remote,snapcompact}`(`compactionSummary.method` 또는 단일 chain 유도, 두 항목 chain에서 부재 시 fail), `compaction_refusal ∈ {none,too_small,no_messages,would_not_reduce}` text 매핑, gate `compaction-method` required `chain=…` + pass 규칙(방법 ⊆ chain, 관측성 ∈ {observed_remote, local_render}) | parser / state | observation 필드, policy, gate row | AC-001, AC-004, AC-010, AC-011 |
| INV-005 | "backward compatibility with A0..A28" — v1 struct·decoder 불변, 23 tag 바이트 동일 검증 | state / parser | historical proof 결과 | AC-006, AC-012 |
| INV-006 | schema dispatch는 v1 pre-check 후 `schema_version` 정확 일치, 혼합/누락/기타/중복 키 거부 | parser | verifier error text | AC-007 |
| INV-007 | body-free diagnostic는 signed bp를 `neg<digits>`로 렌더하고 maintenance는 두 segment의 cohort 전체 합으로 보고 | parser / report row | error frame, receipt | AC-008 |
| INV-008 | 현재 oracle의 완전한 실패 → 중단 → 역사적 서명 성공 순으로 state 결정; 핀/과거 서명값은 별도 metadata | state transition | script, doctor, runbook | AC-009, AC-012 |
| INV-009 | probe는 UID process-free 확인 뒤 no-follow descriptor 기반으로 검증·재직렬화해 retained 경로에 배타적으로 발행; 원본 raw 복사 없음 | state / security | retained probe, abort reason | AC-005, AC-010 |
| INV-010 | remote 최종 usage는 전 시도 비용의 증거가 아님. 실패·내부 재시도 포함 complete attempt coverage 없으면 `unobserved`, C2 숫자나 서명 evidence 없음 | parser / data source | maintenance, partial abort | AC-001, AC-004, AC-012 |
| INV-011 | transcript novelty: post-compaction `role`/`type` 토큰 ⊆ pre-compaction ∪ `{compactionSummary}` ∪ allowlist; 위반은 body-free 오류(토큰은 `^[a-z_]{1,32}$`일 때만 출력); barrier에 `turn_start` 포함 | state / security | error text, probe histogram | AC-005 |
| INV-012 | probe metadata 식별자는 `^[A-Za-z_][A-Za-z0-9_]{0,63}$` 범위에서 대소문자 그대로 보존; 진단 문자열 redaction과 분리 | encoding / security | role/type/usage key histogram | AC-010 |
| INV-013 | only verified 18.1.13 B probes permit a second distinct authenticated pre, with unchanged transcript before ACK; all replay/order/authority barriers and ordinary single-pre behavior remain | protocol / security | checkpoint counts, abort reason | AC-013 |

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| 가설 판정(H1′–H4)·allowlist histogram probe 1회 + retained 반출 | T0 | covered |
| effective chain·방법·거부·유지비 관측·barrier/transcript fail-closed·product 동등성·v2 report·누적 gate·`compaction-method` projection·diagnostic·v1 byte-identical·schema dispatch | T1, T2, T3, T4 | covered |
| Verdict State Table·doctor·release lane 파일명/gate 수·측정 cohort·runbook | T5, T6, T7 | covered |
| 자동(threshold) compaction 임계값, upstream projection 수정, `Already compacted` no-op | 없음 | non-goal |

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| probe cohort(T0) 실행과 `pipelineOMPActiveCompactionContentTypes` allowlist 확정 | T1 이후 전부 | `advance-omp-pin.sh 18.1.13 --probe` + `release-prep.sh --apply` 1회; retained 파일의 숫자와 histogram을 실측 표에 추가 |
| 측정 cohort(T7)와 선택된 핀의 재측정 | Outcome Lock | `passed`, `blocked_below_floor`, `blocked_unobservable`을 구분하고 숫자는 완전한 cohort에만 부여한다. 후보가 막히면 원래 핀도 한 번 실행하며 그 핀의 실패를 과거 verified 값으로 덮지 않는다. 승인 전 live probe와 oracle 구현은 미실행이다. |

## Evolution Ideas

These are optional improvements and do not block sync completion.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| workload를 두 단계(짧은/긴 history)로 나눠 첫 4 pair의 구조적 0 bp를 제거 | 누적 metric이 위치 취약성을 이미 해소 | 누적 metric에서도 segment 간 편차가 큼 |
| 23 tag의 report/attestation SHA256과 gate projection을 testdata로 vendoring해 tag 없는 CI에서도 historical 검증 실행(리뷰 deferred 항목) | 로컬 tag 검증이 Outcome Lock을 닫고 CI는 release lane이 아님 | 사용자 요청 / CI에서 v1 회귀가 놀친 사례 |

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| `pkg/promptlayer/{omp_context_canary_reduce.go, omp_context_promotion_report.go, omp_context_promotion_report_cohort.go, omp_context_promotion_report_verify.go, omp_context_promotion_report_authority.go, omp_context_canary_promotion.go, omp_context_promotion_attestation_v2.go, omp_context_promotion_attestation_verify_v2.go, omp_context_evidence_verify.go, omp_context_promotion_static_policy.go, omp_context_promotion_runtime_v3.go, omp_context_promotion_evidence_id.go, omp_context_promotion_artifact_loader_v2.go}`; `internal/cli/{pipeline_omp_context_active_usage.go, _rpc_execute.go, _lifecycle.go, _rpc.go, _evaluator.go, _process.go, _transcript.go, _policy.go, _rpc_process_fixture_test.go, pipeline_backend_omp_protocol.go, pipeline_omp_context_observation.go, workflow_context_runtime_observe_session_{run,evidence,command,types,setup}.go, workflow_context_runtime_canary.go, workflow_context_runtime_product_overlay.go, workflow_context_runtime_managed_rpc_{run,boundary,completion,product,preflight}.go, doctor_omp_context_reduction.go, doctor_omp_context_projection.go, companion_omp_context_static_policy.go, pipeline_omp_context_cohort.go, pipeline_omp_context_active_evidence.go}` | existing | read/grep 확인(line 인용, rev 2에서 재확인) |
| `scripts/release-tools/advance-omp-pin.sh`, `scripts/companion-release/{prepare-release.sh, prepare-release-runtime-lib.sh, prepare-release-local-lib.sh, publish-release-coordinates.sh, verify-omp-context-evidence-tag.sh, ompcontextverify/main.go}`, `.github/workflows/release.yaml`, `internal/companionmanifest/{omp_pin_agreement_test.go, release_contract_test.go}`, `.gitignore`(`.autopus/runtime/`) | existing | read/grep 확인 |
| `docs/runbooks/omp-pin-advance.md`, `CHANGELOG.md`(A26–A28), 로컬 evidence tags `omp-context-evidence-v0.50.93…v0.50.117`(23개, 104·109 결번); `~/.cache/autopus/release/omp-v{17.2.7,18.1.13}-darwin-arm64` | existing | read; `git tag -l`, `git cat-file`; `--version`, `grep -a -n`, `sed -n` |
| `[NEW] pkg/promptlayer/omp_context_promotion_report_v2.go`(`OMPContextPromotionReportV2`, `OMPContextPromotionObservationV2`, `decodeOMPContextPromotionReportV2`, `expectedOMPContextPromotionGatesV2`, `computeOMPContextPromotionEvidenceIDV2`), `[NEW] omp_context_canary_reduce_v2.go`(`ReduceOMPContextCanarySegmentsV2`), `[NEW] omp_context_promotion_report_dispatch.go`(pre-check 후 schema peek) | [NEW] planned addition | T3 |
| `[NEW] internal/cli/workflow_context_runtime_observe_session_probe.go`(`--probe-dir`, `probe.jsonl`), `[NEW] pipeline_omp_context_active_compaction_method.go`(`pipelineOMPActiveEffectiveChains` verified-major 표, method, maintenance 관측성), `[NEW] pipelineOMPActiveCompactionContentTypes`, `[NEW] doctor_omp_context_reduction_table.go` + `_table_test.go`, `[NEW] scripts/companion-release/prepare-release-runtime-lib.sh::export_probe_records`, `[NEW] OMP_CONTEXT_PROBE_DIR`, `[NEW] advance-omp-pin.sh --probe`, `[NEW] scripts/release-tools/tests/advance-omp-pin-verdict-test.sh`, `[NEW] pkg/promptlayer/omp_context_promotion_historical_tags_test.go` | [NEW] planned addition | T0, T1, T3, T5 |

## Reviewer Brief

- Intended scope: promotion evidence의 측정 주장을 유효 chain의 유효(provider 청구, 관측된 유지비 net) 감축 + segment 누적 집계로 바꾸고(v2), 검증된 major(17·18)에서 evidence를 만들며, v1 검증 불변, probe 선행, Verdict State Table로 pin/doctor/diagnostic 정렬.
- Explicit non-goals: floor 완화, cohort 형태, attestation envelope/키, upstream 수정, threshold 자동 compaction, `Already compacted` no-op 승격, 비용 계산.
- Reviewer focus (revision 3): (1) final remote usage가 내부 재시도 비용을 포함한다는 가정을 제거했는지, (2) descriptor 기반 probe export가 검사-사용 경쟁을 막는지, (3) camelCase histogram이 allowlist 근거를 보존하는지, (4) 현재 oracle 실패/중단과 historical 성공의 우선순위가 일관적인지, (5) 세 execution outcome이 17.2.7 및 18.x 모두에서 거짓 수치 없이 닫히는지. 이전 self-verify PASS는 재리뷰가 발견한 문제를 대신하지 않는다.

## Security Boundary

| 위협 | 통제 | 잔여 |
|---|---|---|
| remote compaction이 history를 provider에 보냄; 결과의 `compaction` item(encrypted reasoning 등)이 transcript에 남음 | 같은 provider·gateway·credential locator로 primary turn이 이미 전체 history를 보냄(새 경계 없음); 구조 walk의 sanitizer exact-pass + novelty 규칙 fail-closed(REQ-METHOD-002) | allowlist는 probe histogram 후 확정 |
| probe source/ancestor 교체로 root가 검사하지 않은 파일을 반출 | canary UID 전 프로세스 종료 확인; trusted dirfd/no-follow traversal·open/fstat·동일 fd 읽기; nlink=1·종류·UID·크기 검사; 검증된 레코드만 새 0600 runner 파일에 생성; raw install/overwrite 금지 | 구현 전이며 AC-010 공격 fixture로 검증해야 함 |

## Self-Verify Summary

- Revision 3은 2026-09-09 review의 열린 finding만 다룬다. 이전 revision의 PASS 주장은 해당 입력에 대한 자체 검토 기록이며 정식 승인 근거가 아니다.
- Q-CORR-01: remote 내부 retry 및 final usage 누락 가능성은 Codex 재리뷰의 `Toe`/`moe=2` 정적 분석에 근거한다. 런타임에서 전체 attempt coverage는 아직 관측되지 않았다.
- Q-COMP-03: `maint_input_tokens`는 전체 cohort 합으로 정의했다(20000+20000=40000, 10000+30000=40000); segment min은 별도다.
- Q-COMP-05 | status: PASS | attempt: 5 | files: acceptance.md, research.md | reason: 회귀 검증과 실제 partial probe 결과를 구분하여 기록했으며 AC-010 전체 성공이나 v2 감축률을 주장하지 않는다.
- Q-CORR-02 / Q-CORR-04: descriptor exporter·probe flags·attempt coverage는 모두 계획이며 기존 기능으로 주장하지 않는다.
- Q-COMP-06 | status: PASS | attempt: 4 | files: spec.md, research.md | reason: requirement·plan·AC·invariant의 추적표와 revision 3 Reviewer Brief를 연결했다.
- Q-COMP-07 | status: PASS | attempt: 4 | files: research.md | reason: probe·allowlist·측정은 Completion Debt로, historical fixture vendoring은 기존 deferred advisory로 구분했다.
- 독립 리뷰 승인: 사용자 승인에 따른 Codex·Gemini 리뷰/Codex judge 실행 `orch-ed56c3aba701c8c12ee8721e2b5f1730`, 48/48 PASS, gate passed, current_status approved, override_applied false. 이 승인은 문서 계약만 대상으로 하며 runtime probe 성공을 뜻하지 않는다.

## T0 attempt 1 — retained partial observation (2026-09-09)

- Source `a36b8d4790a12408dc2083cd85ff78fa38208544`, OMP 18.1.13, model gpt-5.6-sol: five primary calls completed, sequence 6 stopped; six call records (one failed), two compaction attempts, rejected=0, complete=false, mode 0600, no publication. Artifact `.autopus/runtime/omp007/retained/probe-00b0f30b2c4114520557e9a5eb579b51.jsonl`, SHA256 `b87a661b0a0bc01106eee0d4fa40652f5a5f3d7829542e4272e0238c1a4ec357`.
- Attempt at sequence 3 was refused as `too_small`. Attempt at sequence 6 recorded `ack_out_of_order`, `method=none`, `attempt_coverage=unknown`; neither remote completion nor net maintenance was observed. This is not a zero-reduction result.
- Instrumentation gaps found: pre/post ordering shared one reason token; compaction and failed-call durations were left at zero. The next diagnostic-only revision separates `pre_ack_out_of_order`/`post_ack_out_of_order` and records actual durations. It preserves the ACK rejection conditions.
- [STATIC] [OMP 18.1.13 session-maintenance.ts](https://github.com/can1357/oh-my-pi/blob/v18.1.13/packages/coding-agent/src/session/session-maintenance.ts) emits `session_before_compact` for an attempt (around lines 860–871) and can recursively enter the next method after failure (around 1115–1130). Repeated pre-checkpoints are a plausible cause, not proven by the retained generic reason. No ordering tolerance is introduced without identifying the observed sequence.

## T0 attempt 2 — checkpoint boundary identified (2026-09-10)

- Source `81a5361e12bad684dc46c4bd08fb4c35a23c2836`, OMP 18.1.13, model gpt-5.6-sol: five primary calls completed, sequence 6 stopped; six call records (one failed), two compaction attempts, rejected=0, complete=false, mode 0600. Artifact `.autopus/runtime/omp007/retained/probe-c046b270bbac69ffb1ec736409a138da.jsonl`, SHA256 `e27902849b1f79a25f8d3ac6d9ce0fbfab8710099f76c84ae6e9731200a79753`.
- Sequence 6 compaction recorded `pre_ack_out_of_order`, elapsed 61ms; failed call elapsed 111ms. `method=none` and `attempt_coverage=unknown` remain observations of missing evidence, not a successful remote compaction or zero maintenance claim.
- In `manualCompact`, this error is raised for an authenticated pre-checkpoint after pre/post/response state was already established. A repeated pre-checkpoint within the current compact transaction is therefore established; the first method's internal failure is not. The pinned upstream recursive fallback is a compatible explanation, not proof of that internal failure.
- This is a protocol compatibility blocker, not a token-reduction verdict. After two failures the live probe stops; restore the release pin to 17.2.7 and revise/review a bounded per-method checkpoint contract before any further live retry. No report, attestation, release tag or evidence tag was produced.
- T0-C amendment: repeated pre motivates a stateful no-provider reproduction, not a remote/billing success claim. F-013 requires a strict page-response reader that retains the barrier during proof queries; F-014 updates native-end symbols; F-015 aligns the EARS type to Event-driven. Current-input Codex/Gemini review with Codex judge must approve this amendment before admission changes; ordinary v1 producer behavior remains unchanged.
