# SPEC-OMP-007 리서치

Clarification Ledger unavailable — 이 SPEC은 direct 지시(Facts/Constraints/Contract)로 작성됐고 BS 파일이나 inline ledger는 없다. 지시문의 사실은 아래 표에서 실측/정적 증거로 재확인했다.

## Outcome Lock

- User-visible outcome: 릴리즈 핀이 omp/17.2.7에 영구 고정되지 않고, 유효(provider 청구) 컨텍스트 감축을 실제로 내는 OMP 버전은 evidence로 핀을 올릴 수 있으며, 내지 않는 버전은 여전히 refused된다.
- Mandatory requirements: REQ-MEASURE-001/002, REQ-METHOD-001/002, REQ-ATTEST-001, REQ-COMPAT-001/002, REQ-DIAG-001, REQ-PIN-001, REQ-DOCTOR-001, REQ-PROBE-001, REQ-PRODUCT-001.
- Explicit non-goals: floor 2000 bp 수치 완화, cohort 형태 변경, attestation envelope·키·lineage 변경, upstream 수정, 비용(통화) 계산, sandbox/tools 변경.
- Completion evidence: AC-001–AC-012, probe `probe.jsonl` 요약, 23개 로컬 evidence tag의 변경 전후 동일 historical proof, 선택된 핀의 v2 서명 evidence, runbook·verdict table·doctor 갱신.

## Visual Planning Brief

```mermaid
sequenceDiagram
  participant H as observe-session
  participant B as OMP B (optimized; A는 compaction 없이 동일 순서)
  participant P as provider gateway
  H->>B: compact (reused call, methodOrder chain) → pre-ACK / 거부 텍스트 / post-ACK
  H->>B: get_messages_page → compactionSummary.method, tokensBefore/After (진단만)
  H->>B: get_session_stats(before) · prompt · get_session_stats(after)
  B->>P: request(input = 전체 history) → usage.input/cacheRead/cacheWrite
  Note over H: input_tokens = Δ(input+cacheRead+cacheWrite)(두 버전 동일 청구 단위); v2 effective_reduction_bp = (ΣA − (ΣB + ΣB_maint))·10000/ΣA per segment, gate = min ≥ 2000
```

## 현재 측정 경로 (verified: read)

| 값 | 어디서 어떻게 | 근거 |
|---|---|---|
| `observation.input_tokens` | `receipt.InputTokens = afterStats.Input − beforeStats.Input`; `Input = tokens.input + cacheRead + cacheWrite`(`get_session_stats`), before는 compaction 이후·prompt 직전, after는 terminal `agent_end` 이후 같은 세션; 한 prompt의 여러 turn(tool loop)이 합산되고 evidence는 `PrimaryProviderRequests: 1` 고정(`_lifecycle.go:161-226`) | `internal/cli/pipeline_omp_context_active_usage.go:44-57`, `pipeline_omp_context_active_rpc_execute.go:68,80,104-121`, `workflow_context_runtime_observe_session_run.go:149-153`, `workflow_context_runtime_observe_session_evidence.go:206-209` |
| per-pair reduction | `ompContextReductionBasisPointsV1(full, optimized) = (full−opt)·10000/full` 반올림(half away from zero), `Tokens = InputTokens` | `pkg/promptlayer/omp_context_canary_reduce.go:59,117-123`, `omp_context_promotion_report_cohort.go:90-95` |
| median | 20 pair 정렬 후 짝수면 10·11번째 평균 | `omp_context_canary_reduce.go:125-135` |
| floor | `Policy.MinReductionBasisPoints`는 verifier가 2000 이외를 거부(하드 상수); gate `token-reduction`은 `>= 2000` | `omp_context_promotion_report_verify.go:48-49`, `omp_context_canary_promotion.go:66`, `omp_context_promotion_report_authority.go:90` |
| `compactionCount` | B observation의 `CompactionProviderRequests` 합 = `receipt.CompactionCycles` = `manualCompact`가 true 반환한 횟수(0/1); 거부 3종은 no-op(false); 시도는 B의 `sequence > 0`인 모든 call 앞(segment당 9회, `_rpc.go:120`, `_evaluator.go:71`) | `omp_context_promotion_report_cohort.go:198-205`, `pipeline_omp_context_active_rpc_execute.go:48-58,114`, `pipeline_omp_context_active_lifecycle.go:28-40,93-105` |
| 방법 강제 | 오버레이 `compaction: enabled false, methodOrder: [snapcompact], keepRecentTokens 256, reserveTokens 128`; manual RPC `compact` 경로는 두 버전 모두 `auto_compaction_start/end`를 내지 않아 `action` 검사는 자동 경로에서만 작동 | `workflow_context_runtime_product_overlay.go:143-157`, `pipeline_omp_context_active_process.go:111,129`; binary 17.2.7 line 1121697-1121830, 18.1.13 line 1311127-1311400(`auto_compaction_start` 부재) |
| verifier 엄격성 | `DisallowUnknownFields` + canonical bytes 재직렬화 일치 + `schema_version == v1` | `omp_context_evidence_verify.go:232-243`, `omp_context_promotion_report_verify.go:13-31,35-37` |

## 바이너리 정적 증거 (`~/.cache/autopus/release/omp-v{17.2.7,18.1.13}-darwin-arm64`)

명령: `strings -n 6 <bin> | grep -c -F <s>`, `LC_ALL=C grep -a -n -F <s> <bin>`, `sed -n 'A,Bp' <bin>`(번들 JS는 줄 단위로 읽힘), `<bin> --version`.

| 항목 | 17.2.7 | 18.1.13 | 의미 |
|---|---|---|---|
| `--version` | `omp/17.2.7` | `omp/18.1.13` | 자체 보고 일치 |
| `snapcompact would not reduce context locally.` | 0 | 1 (throw, line 1311296; `would not reduce` 6줄) | 18.x 전용 거부 |
| 거부 조건 | 없음 | `ue = #P(C,G,{excludeEncryptedReasoning})` ≥ `pe = #R(C)` → throw (line 1311284-1311296); `#P`=nonMessage + 이미지 summary 메시지 + recent 추정(line 1312224-1312234), `#R`=현재 텍스트 history 추정(line 1311713-1311720) | 로컬 tokenizer 추정치 비교, provider 값 아님 |
| 방법 선택 | `compaction.strategy` 기본 `snapcompact`, `remoteEnabled` 기본 true(LLM 요약 경로에만 영향) | `compaction.methodOrder` 기본 `[remote, snapcompact, handoff, shake, soft]`(line 1038206-1038212); `compact()`는 methodOrder 순서로 remote(`canUseRemoteCompaction`) → snapcompact(image 모델) → soft 선택(line 1311157-1311183); 실패 시 다음 방법으로 재귀(line 1311384-1311388) | 오버레이 `[snapcompact]`면 remote는 절대 선택되지 않음 |
| remote 대상 | — | `provider === "openai" \|\| "openai-codex"` → true(line 986204-986212) | codex는 remote 가능 |
| `get_session_stats` | assistant `usage.input/cacheRead/cacheWrite` 합 + `contextUsage`(line 1123806-1123850) | 동일 합산 + branch 항목 usage 추가(`XOa`, line 1313914) + `contextUsage`(line 1313859-1313945) | delta 의미 불변 |
| Responses usage 변환 | `input_tokens`, `cached_tokens`, orchestration 차감(line 652010-652050) | 동일(line 918934-918975) | 청구 단위 불변 |
| `compactionSummary` 메시지 | `summary, shortSummary, tokensBefore, providerPayload, blocks, images, warning`(line 712773-712785; `tokensAfter`·`method` 없음) | `role: compactionSummary, tokensBefore, tokensAfter, method, blocks`(line 984877-984893); `get_messages_page`는 `e.messages`를 페이지로 반환(line 1442308-1442318) | v2 `compaction_method` 측정원 존재(18.x) |
| `turn_end.message` / `agent_end.messages` | 세션 이벤트 그대로 전달 | `e.subscribe((O) => a(O))`(line 1441996), `a = encodeFrames`(line 1441846) | 프레임의 `usage`는 probe 측정원으로 사용 가능 |
| `snapcompact` 프레임 예산 | — | `#A`: contextWindow, `keepRecent`, shape capacity로 maxFrames(line 1312194-1312213) | 18.x 이미지 예산이 다를 수 있음(H3) |

## 실측 표

| | omp/17.2.7 (A28, `omp-context-evidence-v0.50.117`) | omp/18.1.5 (2026-09-03) | omp/18.1.13 (A29, 2026-09-07) |
|---|---|---|---|
| calls / pairs | 40 / 20 (10:10) | 40 / 20 | 42/42 records |
| compaction cycles | **8** (B sseq 3,5,7,9 ×2 segment) | 2 | 2 |
| median reduction | **2335 bp** (A23 2321, A24 2309, A25 2309, A27 2335) | 0 bp | 895 bp |
| segment 누적 ΣA / ΣB | 1,524,105 / 1,040,415 → **3174 bp** (A23 3131, A24 3160) | 미보존(가정: ≈0) | 미보존 ([INFERENCE] ≤ ~1300 bp: compaction 1회/segment 가정 시) |
| 나머지 gate | pass | pass | pass |

A28 trajectory(`git cat-file blob <report> \| jq`): A input = 36045 + 25859·(k−1) (k=1..10, 268,776까지 선형); B input = 36045, 61904, 83921(c), 109780, 104154(c), 130013, 114152(c), 140011, 117288(c), 143147. compaction 직후 history 추정 = 58062, 78295, 88293, 91429 → 첫 compaction은 history를 6.2%만 줄이고(이미지가 텍스트만큼 비쌈) 이후 28.7%, 32%, 34.7%. per-pair `[0,0,438,338,2533,2137,4030,3550,5172,4674]×2`. 즉 median은 5·6번째 pair(2137/2533)가 결정하고 8/20 pair는 구조적으로 ≤438 bp다.

같은 omp/17.2.7로 만든 23개 tag의 `token-reduction` observed(`git tag -l | git cat-file | jq`): v0.50.93–97 **7084–7305**(pipeline impl `dd7be49d`, prompt ≈18.6–37.9K/call, compaction 9/20), v0.50.98–100 **2556**(impl `eb94cbb8`부터), v0.50.101–108 **3308–3310**(prompt ≈65K/call), v0.50.110–117 **2309–2335**(prompt ≈34–36K/call). OMP는 같았고 harness 구현·workload prompt 크기만 바뀌었는데 metric은 70%→25%→33%→23%로 움직였다. 즉 median 값은 (OMP × harness × workload)의 함수이고, 버전 비교는 같은 workload(cohort manifest)에서만 유효하다 — 17.2.7 vs 18.1.x 비교는 같은 plan generator였으므로 유효하지만, floor 여유가 3점대로 준 것은 OMP가 아니라 harness·workload 변화의 결과다.

## 가설 (label: hypothesis)

| ID | 가설 | 찬성 증거 | 반대 증거 | 판정 수단 |
|---|---|---|---|---|
| H1 | 18.1.x가 감축 일부를 provider-native remote compaction에 넘겨 local 측정이 못 본다 | 18.0.0 `methodOrder` 기본 remote 우선; 18.1.6/18.1.8/18.1.10 remote 관련 노트; `dB()`가 openai-codex를 remote 대상으로 봄 | (a) 측정원은 provider 청구 `input_tokens` delta라 B에서 remote 감축이 일어났다면 보였다; (b) 오버레이 `[snapcompact]`와 `compact()`의 methodOrder 루프 때문에 evidence 경로에서 remote는 실행되지 않는다; (c) codex Responses 요청 body는 `store: false`라 A/B 어느 쪽도 provider-side 대화 상태를 쓰지 않는다(line 985615-985620) | **정적으로 반증**(측정 불가 가설). 남는 형태 H1′: "하네스가 remote를 금지해서 18.x의 선호 경로를 평가하지 못한다" → probe(chain `[remote, snapcompact]`)로 판정 |
| H2 | 18.1.x는 설계상 덜 자주 compaction한다(임계값) | 18.0.9 PR #10024 projection 거부; A29 2회 vs 8회 | 오버레이 임계값은 두 버전 동일(`thresholdTokens` 100000은 manual에 무관, `keepRecentTokens` 256 동일); 거부는 임계값이 아니라 projection 비교 | probe의 refusal class 분포(`too_small` vs `would_not_reduce`)가 결정 |
| H3 | 18.1.x snapcompact 출력이 같은 history에 대해 더 크다 | 18.1.13 `#A` 프레임 예산·모델별 tokenizer("dynamically scoped to each specific model tokenizer", 18.0.0 노트)·`snapcompact.shape`; 18.1.5→18.1.13 0→895 bp는 archived history 크기 계산 변화(18.1.8) | 수행된 2회의 compaction의 전후 청구값이 evidence에 남지 않아 판정 불가 | probe의 compaction 직후 B input vs 직전 B input, `tokensBefore/After`(진단) |
| H4 | 거부는 projection 아티팩트다 — 실제로 compaction했다면 청구량은 줄었을 것(17.2.7이 같은 history에서 ≥20% 냈음) | 17.2.7은 gate 없이 같은 history를 compaction해 첫 회 6.2%, 이후 28–35% 감축; 18.x 거부는 로컬 추정치 비교만으로 결정 | 18.1.8이 추정 정확도를 고친 뒤에도 18회 중 2회만 수행(16회 거부, 클래스 미상) → 추정치가 실제와 계속 다르다는 직접 증거는 없음 | probe에서 `would_not_reduce` 거부 직후 pair의 A/B 청구값과 `tokensBefore`, `#R` 기준값의 크기 관계 |

## Upstream 릴리즈 노트 (`gh api repos/can1357/oh-my-pi/releases`, 2026-09-08 조회)

- v18.0.0 (embedded CHANGELOG line 1578704, 1578767): `compaction.strategy`/`remoteEnabled` → `compaction.methodOrder`(`[remote, snap]`은 OpenAI 등 지원 provider에서 remote). v18.0.9 (2026-08-28): "Fixed Snapcompact so it skips or falls back when compaction would not reduce context size" — PR #10024 "skip inflating snapcompact results", 거부 클래스의 기원.
- v18.1.6 (2026-09-03): "Codex GPT-5.6 requests now use full Responses by default, enabling independent tool calls to run in parallel; provider-native compaction continues to use catalog-selected Responses Lite."
- v18.1.8 (2026-09-03): "Fixed context compaction incorrectly accepting archived history that was larger because of opaque reasoning data, allowing the next compaction strategy to run instead." (+ Codex 라우팅에 model/service tier 전달)
- v18.1.10 (2026-09-04): Codex V2 remote compaction prefix/prompt-cache 수정(#10786). v18.1.13 (2026-09-07): compaction 관련 노트 없음.

## Backward compatibility 분석

- 서명·검증: attestation v2는 report bytes의 sha256과 trust lane만 서명한다(`omp_context_promotion_attestation_v2.go:14-36`, `_verify_v2.go:44-80`) — envelope·키(K3)·lineage 불변. v1 struct에 필드를 더하면 canonical 재직렬화가 깨져 과거 evidence가 실패하므로(`_verify.go:22-25`) v2는 별도 struct·decoder이고 `schema_version` 선행 peek로 dispatch하며 v1 함수는 손대지 않는다.
- 소비자: `VerifiedOMPContextPromotion.CanaryRows()`(runtime lease, `pipeline_omp_context_active_evidence.go:32,53-54`), `EvaluateOMPContextHistoryPromotionV1`(runtime 판정, `omp_context_canary_promotion.go:37-66`), `pipeline_omp_context_cohort.go:132-148`(shadow receipt), `omp_context_evidence_store.go:101`(local store). v2 rows에는 v2 aggregate(누적)가 필요하고 v1 rows는 v1 aggregate를 유지한다.
- 파일명: `omp-context-promotion-report.v1.json`은 release lane 15개소(scripts/companion-release/*, .github/workflows/release.yaml, internal/companionmanifest/*_test.go)에 박혀 있다. `.v2.json`으로 옮기며 `release_contract_test.go`가 누락을 잡는다. 과거 tag는 `.v1.json` 그대로.
- 핀 표·diagnostic: `advance-omp-pin.sh:75-113` 세 상태, `doctor_omp_context_reduction.go:18-49`는 `omp/18.1.` prefix 전체를 `measured_zero_reduction`으로 묶는다(895 bp도 zero로 표기 — 이름은 유지하되 표를 한 곳으로); `gate_diagnostic`은 `prepare-release-runtime-lib.sh:124` 정규식 `^[a-z_=/0-9[:space:]]*$`, ≤400이라 새 필드명은 밑줄만(`effective_reduction_bp`).

## 설계 결정 / 대안 검토

- **측정원 유지(provider 청구 delta)**: 두 버전에서 동일하게 verified; OMP 로컬 추정(`contextUsage.tokens`, `tokensBefore/After`)은 tokenizer가 버전마다 달라 attest 대상에서 제외(기각: local 추정치 attest). `turn_end.message.usage`는 같은 값의 per-turn 분해라 probe 진단원으로만 쓴다. 기각: floor 수치 완화(0 bp 버전 admit), 이벤트 궤적만 gate(B안, 주장 약화).
- **누적 집계(C4 segment min)**: median의 위치 취약성(위 실측) 때문. 예: call 7에서만 강하게 compaction하는 B‴(합 982,745)는 per-pair `[0×6,7078,6235,5571,5035]`, median 0으로 v1 실패지만 effective 3552 bp — 실제로 줄이는 버전을 v1은 놓친다. 전체 20 pair 누적은 한 segment의 실패를 다른 segment가 가릴 수 있어 기각.
- **net of maintenance(C2)**: remote compaction은 provider 요청을 쓰므로 그 input을 B에 더해야 "operator가 지불하는 컨텍스트"가 된다. snapcompact는 0이라 17.2.7 값은 변하지 않는다.
- **chain `[remote, snapcompact]`와 probe 우선**: 18.x 기본 순서의 앞 두 항목만(`handoff/shake/soft`는 LLM 요약·hook 동작이 출력 digest 오라클과 충돌 가능). 정적 미검증 항목 셋(gateway 통과, barrier 프레임, opaque item)이 있어 oracle 코드를 먼저 바꾸면 cohort 한 번을 헛되이 쓸 수 있으므로 probe가 먼저다.

## Minimality Decision Matrix

| Ladder step | Evidence | Decision | Receipt item |
|-------------|----------|----------|--------------|
| actual need | 핀이 evidence 오라클 때문에 정체; 5 cohort의 floor 여유 309–335 bp; 18.x는 강제된 방법으로 18회 시도 중 2회만 수행 | proceed | Outcome Lock |
| existing code/helper/pattern | `sessionStats` delta, `ReduceOMPContextCanaryPairsV1`, `manualCompact` 거부 매칭, `validatePipelineOMPActiveTranscript` 페이지 읽기, verdict table, `gate_diagnostic` 경로를 모두 재사용 | reuse | 측정원·프레임·페이지 API 불변 |
| stdlib/native | 누적 합·반올림은 기존 `ompContextReductionBasisPointsV1` 산식 재사용; JSON strict decode는 `encoding/json` 기존 helper | use | 새 라이브러리 없음 |
| existing dependency | ed25519 attestation, cobra flag, jq/shasum(scripts) 기존 것만 | reuse | — |
| new dependency or abstraction | 새 의존성 없음. 새 abstraction은 `ReportV2` struct + `ReduceOMPContextCanarySegmentsV2` 하나(v1 byte-identical을 위해 불가피) | accepted | schema dispatch 1개 |
| minimum sufficient verification | AC-001–AC-012: v2 verify fixture(≥20% pass / 0 bp fail), 23 tag historical byte-identical, diagnostic regex, pin script dry-run, product overlay 테스트, probe 1회, 측정 cohort 1회 | required checks | security(REQ-METHOD-002)·data-loss(evidence 미생성)·deterministic oracle 유지 |

## Semantic Invariant Inventory

| ID | source clause | invariant type | affected outputs | acceptance IDs |
|----|---------------|----------------|------------------|----------------|
| INV-001 | "measure compaction benefit … name the exact measurement source in the RPC/usage frames" — 측정원은 `get_session_stats` delta(provider 청구) 하나 | parser / data source | `input_tokens`, probe `stats_before/after` | AC-001, AC-010 |
| INV-002 | "a version that genuinely reduces context passes and one that does not still fails" — `effective_reduction_bp = round((ΣA − (ΣB + ΣB_maint))·10000/ΣA)` per segment, gate = min ≥ 2000 | numeric formula / ordering | gate `effective-reduction`, `gate_diagnostic` | AC-001, AC-002, AC-003 |
| INV-003 | "must NOT propose lowering the 2000 bp floor" — verifier는 policy floor 2000 이외 거부, median은 진단값 | bound | verifier error, `median_reduction_basis_points` | AC-002, AC-003, AC-007 |
| INV-004 | 방법·거부 관측: `compaction_method ∈ {none,remote,snapcompact}`, `compaction_refusal ∈ {none,too_small,would_not_reduce,already_compacted}`, 미지 값 fail-closed; 방법은 `compactionSummary.method` 또는 단일 methodOrder에서 결정 | parser / state | observation 필드, gate `compaction-method` | AC-004, AC-005, AC-010, AC-011 |
| INV-005 | "backward compatibility with A0..A28" — v1 struct·decoder 불변, 23 tag 바이트 동일 검증 | state / parser | historical proof 결과 | AC-006, AC-012 |
| INV-006 | schema dispatch는 `schema_version` 정확 일치, 혼합/누락/기타 거부 | parser | verifier error text | AC-007 |
| INV-007 | "body-free diagnostics" — `gate_diagnostic` 문법 `pairs=… compactions=… effective_reduction_bp=… median_reduction_bp=… methods=… refusals=…`, charset/길이 | parser / report row | error frame, receipt line | AC-008 |
| INV-008 | "`advance-omp-pin.sh` table semantics unchanged" — 세 상태, oracle version별 1회 `--measure`, doctor 표와 동일 | state transition | script 출력, doctor reason | AC-009 |
| INV-009 | probe는 서명 evidence를 만들지 않고(`probe_completed`) 금지 문자열을 쓰지 않는다 | state / security | `probe.jsonl`, 마지막 frame | AC-005, AC-010 |

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| 가설 판정(H1′–H4) probe 1회 | T0 | covered |
| B/product 세션 chain·방법·거부 관측·barrier fail-closed | T1, T2 | covered |
| v2 report·누적 gate·diagnostic·v1 byte-identical·schema dispatch | T2, T3, T4 | covered |
| verdict table·doctor 정렬·release lane 파일명·측정 cohort·runbook | T5, T6, T7 | covered |
| 자동(threshold) compaction 임계값, upstream projection 수정 | 없음 | non-goal |

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| Decision Needed 확정(A/B/C/D, C2·C4) | T1–T7 착수 | 사용자 결정; 기본값은 C(C2·C4) |
| probe cohort(T0) → 측정 cohort(T7) 실행 결과 | T1 이후 전부 / Outcome Lock 완료 판정 | `advance-omp-pin.sh 18.1.13 --probe` + `release-prep.sh --apply` 1회로 probe 숫자를 실측 표에 추가; 선택된 핀에서 v2 evidence가 `effective-reduction` 통과 또는 refused row 기록 |

## Evolution Ideas

These are optional improvements and do not block sync completion.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| workload를 두 단계(짧은/긴 history)로 나눠 첫 4 pair의 구조적 0 bp를 제거 | 누적 metric이 위치 취약성을 이미 해소 | 누적 metric에서도 segment 간 편차가 큼 |
| `handoff`/`soft` 방법까지 chain에 허용; 실패한 cohort의 report를 unsigned artifact로 보존 | 출력 digest 오라클과 충돌 가능; `gate_diagnostic`이 이미 판정 숫자를 남김 | 사용자 요청 / 진단 숫자만으로 원인 분석이 부족한 사례 |
| `measured_zero_reduction` 이름을 `measured_below_floor`로 개명 | doctor reason allowlist·테스트 변경 비용 대비 이득 작음 | 사용자 요청 |

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| `pkg/promptlayer/{omp_context_canary_reduce.go, omp_context_promotion_report.go, omp_context_promotion_report_cohort.go, omp_context_promotion_report_verify.go, omp_context_promotion_report_authority.go, omp_context_canary_promotion.go, omp_context_promotion_attestation_v2.go, omp_context_promotion_attestation_verify_v2.go, omp_context_evidence_verify.go}`; `internal/cli/{pipeline_omp_context_active_usage.go, _rpc_execute.go, _lifecycle.go, _rpc.go, _evaluator.go, _process.go, _transcript.go, pipeline_backend_omp_protocol.go, workflow_context_runtime_observe_session_{run,evidence,command,types}.go, workflow_context_runtime_product_overlay.go, workflow_context_runtime_managed_rpc_{run,boundary,completion,product}.go, doctor_omp_context_{reduction,capabilities,projection}.go, pipeline_omp_context_cohort.go, pipeline_omp_context_active_evidence.go}` | existing | read/grep 확인(line 인용) |
| `scripts/release-tools/advance-omp-pin.sh`, `scripts/companion-release/{prepare-release.sh, prepare-release-runtime-lib.sh, prepare-release-local-lib.sh, publish-release-coordinates.sh, verify-omp-context-evidence-tag.sh, ompcontextverify/main.go}`, `.github/workflows/release.yaml`, `internal/companionmanifest/{omp_pin_agreement_test.go, release_contract_test.go}` | existing | read/grep 확인 |
| `docs/runbooks/omp-pin-advance.md`, `CHANGELOG.md`(A26–A28), 로컬 evidence tags `omp-context-evidence-v0.50.93…v0.50.117`(23개, 104·109 결번); `~/.cache/autopus/release/omp-v{17.2.7,18.1.13}-darwin-arm64` | existing | read; `git tag -l`, `git cat-file`; `--version`, `strings`, `grep -a -n`, `sed -n` |
| `[NEW] pkg/promptlayer/omp_context_promotion_report_v2.go`(`OMPContextPromotionReportV2`, `OMPContextPromotionObservationV2`, `decodeOMPContextPromotionReportV2`, `expectedOMPContextPromotionGatesV2`), `[NEW] omp_context_canary_reduce_v2.go`(`ReduceOMPContextCanarySegmentsV2`), `[NEW] omp_context_promotion_report_dispatch.go`(schema peek) | [NEW] planned addition | T3 |
| `[NEW] internal/cli/workflow_context_runtime_observe_session_probe.go`(`--probe-dir`, `probe.jsonl`), `[NEW] pipeline_omp_context_active_compaction_method.go`, `[NEW] doctor_omp_context_reduction_table.go` + `_table_test.go`, `[NEW] scripts/release-tools/tests/advance-omp-pin-verdict-test.sh`, `[NEW] pkg/promptlayer/omp_context_promotion_historical_tags_test.go` | [NEW] planned addition | T0, T1, T3, T5 |

## Reviewer Brief

- Intended scope: promotion evidence의 측정 주장을 method-agnostic 유효(provider 청구) 감축 + segment 누적 집계로 바꾸고(v2), v1 검증 불변, probe 선행, pin/doctor/diagnostic 정렬.
- Explicit non-goals: floor 완화, cohort 형태, attestation envelope/키, upstream 수정, threshold 자동 compaction, 비용 계산.
- Self-verified: Traceability Matrix, INV-001–009 ↔ AC-001–012, existing/[NEW] 분리, 바이너리 line 인용은 실제 `sed -n` 출력, evidence 숫자는 로컬 tag에서 jq로 재계산.
- Reviewer should focus on: (1) 누적 metric 정의(C2·C4)가 "줄이지 않는 버전은 실패"를 지키는지, (2) v1 byte-identical 경로에 우회가 없는지, (3) remote compaction 경계의 fail-closed 규칙, (4) probe가 서명 evidence를 절대 만들지 않는지. 새 scope 제안은 Evolution Ideas로.

## Security Boundary

| 위협 | 통제 | 잔여 |
|---|---|---|
| remote compaction이 history를 provider에 보냄; 결과의 opaque item(encrypted reasoning 등)이 transcript에 남음 | 같은 provider·gateway·credential locator로 primary turn이 이미 전체 history를 보냄(새 경계 없음); REQ-METHOD-002가 미분류 content를 fail-closed | opaque item 허용 규칙은 probe 후 확정 |
| probe 파일에 prompt/응답/credential이 새어 들어가거나 probe가 서명 evidence를 만들어 태그됨; 문서의 시크릿·절대경로 노출 | evidence writer와 같은 forbidden 문자열 스캔, 숫자·enum·digest만, 0600; `error_code=probe_completed`로 lane fail-closed, report/store 미기록; 이 문서는 바이너리 캐시 경로를 `~/.cache/...`로만 인용, credential 값 없음 | 없음 |

## Self-Verify Summary

- Q-CORR-01 | status: PASS | attempt: 1 | files: research.md, spec.md, plan.md | reason: 인용한 경로·심볼·line은 read/grep/sed 출력에서 옮겼고 evidence 숫자는 로컬 tag에서 jq로 재계산
- Q-CORR-02 | status: PASS | attempt: 1 | files: plan.md, research.md | reason: 신규 파일·함수·flag·테스트에 [NEW] 마커
- Q-CORR-03 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: REQ는 `SHALL`/`WHEN…THEN`/`IF…THEN` 단일 패턴, AC는 `### Sn:` 헤더와 bare Given/When/Then
- Q-CORR-04 | status: PASS | attempt: 1 | files: research.md | reason: Reference Discipline이 existing(read/grep/binary 명령)과 [NEW]를 분리
- Q-COMP-01 | status: PASS | attempt: 1 | files: spec.md, plan.md, acceptance.md, research.md | reason: 문제·요구·결정·계획·오라클·리서치가 각 문서에 분담
- Q-COMP-02 | status: PASS | attempt: 1 | files: spec.md, plan.md, acceptance.md | reason: 12 REQ ↔ T0–T7 ↔ AC-001–012 Traceability Matrix
- Q-COMP-03 | status: PASS | attempt: 1 | files: spec.md | reason: 모든 REQ에 Type/Priority/Observability와 관측 필드명
- Q-COMP-04 | status: PASS | attempt: 1 | files: research.md, plan.md | reason: Outcome Lock의 미해결 항목(결정·probe·측정 cohort)은 Completion Debt로 sync를 막음
- Q-COMP-05 | status: PASS | attempt: 1 | files: research.md, acceptance.md | reason: INV-001–009가 concrete 숫자(3174/2335/0 bp, A28 trajectory)와 정규식을 가진 Must oracle에 매핑
- Q-COMP-06 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: Traceability Matrix와 Reviewer Brief가 검토 범위를 4개 focus로 제한
- Q-COMP-07 | status: PASS | attempt: 1 | files: research.md | reason: Completion Debt(결정·probe·cohort)와 Evolution Ideas(ID 없음) 분리
- Q-FEAS-01 | status: PASS | attempt: 1 | files: plan.md | reason: 런타임 코드·스크립트·문서 변경 대상이 실제 파일이며 probe→oracle 순서가 실행 가능
- Q-FEAS-02 | status: PASS | attempt: 1 | files: plan.md | reason: 변경 경로가 autopus-adk 모듈 안이고 generated surface 없음
- Q-FEAS-03 | status: PASS | attempt: 1 | files: acceptance.md | reason: 검증 명령(go test, ompcontextverify, script dry-run, jq)이 현재 저장소에서 실행 가능
- Q-STYLE-01 | status: PASS | attempt: 1 | files: spec.md | reason: REQ 본문에 should/might/could/possibly/maybe/perhaps 없음
- Q-STYLE-02 | status: PASS | attempt: 1 | files: spec.md | reason: Priority는 Must만, Type과 분리
- Q-STYLE-03 | status: PASS | attempt: 1 | files: acceptance.md | reason: 완결 문장, bare Gherkin step
- Q-SEC-01 | status: PASS | attempt: 1 | files: research.md, spec.md | reason: remote compaction·opaque item·probe 파일의 경계와 fail-closed 규칙 명시
- Q-SEC-02 | status: PASS | attempt: 1 | files: research.md | reason: credential 값·절대 사용자 경로 미기재, probe 0600·forbidden 스캔
- Q-SEC-03 | status: PASS | attempt: 1 | files: research.md, spec.md | reason: probe 산출물은 숫자·enum만, 서명 evidence 미생성, diagnostic 문법 고정
- Q-COH-01 | status: PASS | attempt: 1 | files: spec.md | reason: 하나의 문제(오라클 의미)와 밀접한 변경 대상
- Q-COH-02 | status: PASS | attempt: 1 | files: research.md | reason: 필수 후속(probe·cohort)은 Completion Debt
- Q-COH-03 | status: PASS | attempt: 1 | files: research.md, plan.md | reason: sibling 없음(Primary SPEC 단독, plan Feature Completion Scope에 명시)
