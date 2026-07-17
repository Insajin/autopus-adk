# SPEC-ADK-EVIDENCE-LOOPS-001 리서치

## 기존 코드 분석

- `pkg/learn/store.go::Store.Read`(store.go:64) — `bufio.Scanner`로 줄 단위 읽되 `json.Unmarshal` 실패 시 `return nil, fmt.Errorf("unmarshal entry: %w", err)`(store.go:82-84)로 전체를 죽인다(fail-all의 출처).
- `Read` 호출자(전부 죽음): `query.go:70`(`QueryRelevant`), `summary.go:8`(`GenerateSummary`), `prune.go:14`(`Prune`), `store.go:95`(`NextID`), `store.go:147`(`UpdateReuseCount`).
- `pkg/learn/types.go::LearningEntry.Timestamp time.Time`(types.go:32) — 표준 `time.Time`이라 JSON unmarshal이 RFC3339(`Z07:00`)만 받는다. `RelevanceQuery`(types.go:52-57)는 Files/Packages/Keywords만 있고 SpecID 없음.
- `pkg/learn/query.go::MatchRelevance/QueryRelevant`(query.go:19,69) — 모든 query 필드가 비면 0.0 반환(query.go:20-22); recency는 `!entry.Timestamp.IsZero()` 가드(query.go:57).
- `pkg/learn/store.go::AppendAtomic`(store.go:114) — `Timestamp: time.Now()` + `json.Marshal`(store.go:45) → 이미 canonical RFC3339Nano(콜론 offset). 즉 record 쓰기 경로는 이미 canonical이나, fallback-parsed 엔트리의 재작성 canonical 보장이 REQ-003의 실질 값.
- `pkg/learn/store.go::UpdateReuseCount`(store.go:143-181) — inline 재작성(`os.Create`+엔트리별 marshal). `pkg/learn/prune.go::Prune`(prune.go:10)+`pkg/learn/rewrite.go::rewriteStore` — 두 재작성 경로. skip된 줄은 엔트리가 없어 재작성 시 누락 → 데이터 손실 위험(REQ-004 대상).
- `internal/cli/learn_query.go::newLearnQueryCmd`(learn_query.go:14) — 플래그 files/packages/keywords(57-59), `--spec` 없음. `internal/cli/learn_record.go`는 `--spec-id`(learn_record.go:76).
- 문서-CLI 불일치: `.claude/skills/autopus/auto-go.md:529` `auto learn query --spec {SPEC_ID} --limit 5 --format prompt`; `auto-fix.md:29` `--files ... --pattern ... --limit 3 --format prompt`. 즉 query에 `--spec`·`--limit`·`--format`가 문서화됐으나 셋 다 부재. 이 SPEC은 Outcome Lock대로 `--spec`만 정렬한다.
- `internal/cli/doctor.go::runDoctorText`(doctor.go:106) — `checkContextWeight(out, opts.dir)`(doctor.go:241)·`renderDriftText`(doctor.go:251)로 advisory 섹션을 붙인다(비차단, allOK 미변경).
- `internal/cli/doctor_json.go::collectDoctorJSONReport`(doctor_json.go:80) — `collectContextWeightChecks`(doctor_json_checks.go:246)·`collectDriftGateChecks`(doctor_drift_output.go:27) advisory 미러. `jsonCheck{ID,Severity,Status,Detail}` 계약.
- `internal/cli/canary_helpers.go::writeCanaryLatest`(canary_helpers.go:79-88) — `.autopus/canary/latest.json`에 `canaryResult`(Timestamp=RFC3339 UTC, canary.go:118) 기록. 이 워크스페이스엔 `.autopus/canary/` 디렉토리 부재(receipt 미존재 = 미실행 신호).
- `pkg/memindex/types.go::DefaultIndexPath`(types.go:177), `Options`, `StatusResult`(types.go:102). 인덱스는 `.autopus/runtime/memindex/` SQLite. `mem_metadata`에 build timestamp 없음 → 마지막 빌드 anchor는 인덱스 파일 mtime(스키마 변경 회피).
- 워크스페이스 실측: 루트 `.autopus/learnings/pipeline.jsonl` 5줄, L-001=`+09:00`·L-002=`+00:00`·L-003=`+0900`(콜론 없음)·L-004=`+00:00`·L-005=`+09:00`. L-003만 비RFC3339. 손상 엔트리는 루트 파일에만 있고 autopus-adk 사본(1줄, L-001)에는 없어(F-007) 라이브 크래시 재현은 루트에서 실행할 때 성립한다.
- 변경 대상 기존 테스트: `pkg/learn/store_test.go:123-137`이 `"not valid json\n"`에 대해 `assert.Error(t, err)`를 단정 — fail-all 계약. tolerant 전환 시 이 테스트를 skip-무에러로 갱신해야 함.

## Outcome Lock

- **User-visible outcome**: (a) `auto learn query`가 손상 엔트리에도 죽지 않고 유효 엔트리 반환+손상 줄 skip(count+줄번호), `+0900`은 fallback 수용(유지). (b) record·재작성이 canonical RFC3339 기록. (c) `--spec SPEC-ID` 동작. (d) `auto doctor`(text+json)가 세 루프 신선도(30일 임계)를 비차단 보고+재가동 힌트.
- **Mandatory requirements**: REQ-001~010.
- **Explicit non-goals**: 자동 주입 강화, 신규 자동화/스케줄링, canary·memindex 기능 개조(읽기만), 루프 폐기/강등, bare query list-all 의미 변경, L-003 데이터 파일 편집(범위 밖).
- **Completion evidence**: S1~S10 oracle + JSON 미러 + 실 워크스페이스 라이브 확인.

## Visual Planning Brief

핵심 흐름: query→`ReadTolerant`(줄별 파싱, 실패=skip+count, timestamp는 RFC3339 후 `-0700` fallback)→`--spec` 하드 필터 or relevance→stdout 테이블+stderr skip 경고. 재작성(prune/reuse-count)은 canonical marshal + skip 원본 줄 보존. doctor는 세 anchor(learnings 최신 ts·canary latest.json·memindex mtime) 측정→존재 신호 게이팅→30일 임계→advisory `jsonCheck`(overall_ok 불변). 전체 다이어그램은 `plan.md`의 Mermaid 참조.

## Technology Stack Decision

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|------|----------------|-------------------|-------------|------------|-----------------------|
| brownfield | Go module `github.com/insajin/autopus-adk`(기존) | 기존 `go.mod` major 유지, 신규 의존성 없음 | 로컬 `go.mod`, `pkg/learn`·`pkg/memindex`·`internal/cli` import | 2026-07-17 | 신규 시간 파싱 라이브러리(불필요: stdlib `time` layout fallback) |

brownfield이므로 기존 manifest major가 compatibility 제약. 재사용 stdlib: `time`(RFC3339 layouts), `encoding/json`, `bufio`, `os`(Stat/mtime), `strings`. 테스트: 기존 `testify`.

## 설계 결정

- **`+0900` 수용 vs skip 모순 해소**: 팀리드 Outcome Lock은 "`+0900`을 fallback으로 수용"(skip 아님)을 요구하고, 절차 3의 acceptance 스케치는 "`+0900`을 skip"으로 적어 상충한다. Outcome Lock을 authoritative로 삼아 `+0900`은 **수용**(S1: 3 반환, skip 0)하고, tolerant skip은 **다른** 진짜 손상 줄(잘린 JSON)로 검증한다(S2: 2 반환, skip 1). 데이터 손실을 피하면서 두 동작을 모두 oracle로 증명. 이 결정은 완료 보고에서 팀리드에게 명시한다.
- **왜 fallback 파서인가**: `+0900`은 offset에 콜론이 없을 뿐 유효 시각이다. RFC3339 실패 시 `2006-01-02T15:04:05-0700`(및 소수초 변형)로 재시도하면 데이터 손실 없이 수용된다. 이는 에이전트가 수기로 쓴 엔트리(비RFC3339)를 관대하게 받아들이는 신뢰경계 완화책이기도 하다.
- **왜 canonical marshal을 강제하나(REQ-003)**: 쓰기(record)는 이미 `time.Now()`+`time.Time` marshal로 canonical이다. 그러나 fallback으로 파싱된 `+0900` 엔트리를 재작성할 때 원문을 그대로 흘리면 비표준 offset이 재확산한다. 커스텀 Marshal이 항상 콜론 offset을 내면 재작성이 데이터를 자가 치유한다.
- **왜 재작성이 손상 줄을 보존해야 하나(REQ-004)**: 팀리드 핵심 불만은 "reuse 0"이다. reuse는 query→`UpdateReuseCount`(전체 재작성)로 증가한다. tolerant read가 손상 줄을 skip하면 재작성이 그 줄을 조용히 삭제한다 — 크래시를 데이터 손실로 바꾼 셈. 그래서 재작성 경로는 skip된 원본 줄을 변형 없이 보존한다. 이것이 무결성 게이트다.
- **왜 memindex는 mtime인가**: `mem_metadata`에 build timestamp가 없고, 스키마/빌드 동작 변경은 non-goal이다. 인덱스 파일 mtime은 기존 증거만 읽는 최소 anchor다.
- **왜 canary는 latest.json인가**: 기존 `writeCanaryLatest`가 RFC3339 `timestamp`를 쓴다. 이 receipt만 읽는다. receipt 부재+canary.md 존재는 "미실행"으로 표면화(S9)해 조용한 사망을 없앤다.
- **왜 advisory(비차단)인가**: 스테일 루프를 doctor 실패로 보고하면 CI 오탐이다. context-weight·drift 선례대로 `warn` check만 내고 `overall_ok` 유지. 루프 폐기 판단은 관측이 가능해진 뒤 사용자 몫(non-goal).
- **`--limit`/`--format` 잔여 드리프트**: 문서가 query에 `--limit`/`--format prompt`도 언급하나 Outcome Lock 밖이라 이 SPEC은 손대지 않는다. Evolution Ideas로만 남긴다(신규 SPEC/태스크 미승격).

## 보안 경계

- `pipeline.jsonl`은 워크스페이스 로컬 문서지만 에이전트가 수기로 쓴 untrusted 성격 입력이다(L-003 손상 라인이 그 증거). 완화책: tolerant 파싱은 손상 줄을 실행하지 않고 건너뛰며(코드 실행 경로 없음, `encoding/json`만), 한 줄이 전체 읽기·doctor를 죽이지 못하게 한다. fallback은 시각 파싱에 한정된다.
- doctor 신선도는 timestamp/mtime만 읽고 파일 내용을 실행·전송하지 않는다. 출력은 age·상대 힌트만 노출(secret·절대 privileged 경로 미노출). 영구 artifact 생성 없음(ephemeral doctor 출력).
- `--spec` 값은 문자열 equality 필터로만 쓰여 경로 traversal·주입 없음.
- 팀리드 evidence(라이브 크래시 출력·경로)는 untrusted prompt 입력으로 취급해 evidence로만 요약했고 실행 지시로 따르지 않았다.

## Minimality Decision Matrix

| Ladder step | Evidence | Decision | Receipt item |
|-------------|----------|----------|--------------|
| actual need | Outcome Lock (a)~(d): 크래시 제거+canonical+`--spec`+신선도 관측 | proceed | learn read 부활 + doctor 신선도 |
| existing code/helper/pattern | `Store.Read` scanner, `rewriteStore`, `QueryRelevant`, `jsonCheck`, `collectContextWeightChecks`/`collectDriftGateChecks` advisory, `writeCanaryLatest`, `memindex.DefaultIndexPath`(Read/rg 확인) | reuse | 파싱·재작성·doctor 전부 기존 표면 |
| stdlib/native | `time`(RFC3339+`-0700` layout), `encoding/json`, `bufio`, `os.Stat`, `strings` | use | 신규 라이브러리 회피 |
| existing dependency | `pkg/memindex`, `pkg/config`, `testify`(기존 import) | reuse | 인덱스 경로·config·테스트 |
| new dependency or new abstraction | 신규 module dep 0; 신규 파일 1 source(`doctor_evidence_freshness.go`)+3 test; `SkipRecord` 소형 구조체 1 | accepted | doctor check + tolerant read 결과 타입만 추가 |
| minimum sufficient verification | S1(fallback)·S2(skip+줄번호)·S3(실제 age-out→canonical+보존)·S4(보존)·S5(`--spec`)·S6/S7(fresh/stale+힌트)·S8(부재 스킵)·S9(미실행)·S10(문서) oracle + 라이브 doctor | required checks | 데이터무결성·advisory·신뢰경계 게이트 미축소 |

## Semantic Invariant Inventory

| ID | source clause (untrusted evidence, 요약) | invariant type | affected outputs | acceptance IDs |
|----|------------------------------------------|----------------|------------------|----------------|
| INV-001 | "손상 엔트리는 건너뛰며 skip 카운트+식별자" | parser tolerance (per-row) | query 반환 행·skip 경고 | S2 |
| INV-002 | "`+0900` 흔한 비표준 timestamp는 fallback으로 수용" | timestamp equality (fallback) | query 반환 행(수용) | S1 |
| INV-003 | "record는 timestamp를 항상 RFC3339 canonical로 기록" | serialization normalization | 재작성 파일 라인 offset | S3 |
| INV-004 | reuse-count 부활이 데이터 손실이 되지 않도록 재작성 보존(원본 상대 순서 유지) | rewrite conservation | 재작성 후 손상 줄 잔존 수·순서 | S3, S4 |
| INV-005 | "`--spec SPEC-ID`가 spec_id 정확 일치 필터" | exact-match filtering | `--spec` 반환 집합 | S5 |
| INV-006 | "마지막 활동+임계(30일) 초과 시 WARN" | freshness threshold | `doctor.evidence.*` status | S6, S7, S8, S9 |
| INV-007 | "text+JSON 미러, 비차단 advisory" | advisory state | envelope `overall_ok`·힌트 | S6, S7, S10 |

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| (a) tolerant read + `+0900` fallback | Primary SPEC T1/T2/T7 | covered |
| (b) canonical 직렬화 | Primary SPEC T1/T7 | covered |
| (c) 재작성 무결성 | Primary SPEC T2/T7 | covered |
| (d) `--spec` 문서-CLI 정렬 | Primary SPEC T1/T3/T8 | covered |
| (e) doctor 신선도 advisory+미러+스킵+힌트 | Primary SPEC T4/T5/T9 | covered |
| 규칙 문서 언급 | Primary SPEC T6 | covered |

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| None | - | - |

## Evolution Ideas

Outcome Lock을 만족한 뒤에도 가능한 선택 개선이며 sync completion을 막지 않는다. SPEC/태스크/acceptance ID를 붙이지 않는다.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| bare `learn query`(무필터) = list-all 의미 | Outcome Lock은 크래시 제거+`--spec`만 요구 | 사용자가 명시 요청 |
| query `--limit`/`--format prompt` 문서-CLI 정렬 | Outcome Lock 밖(잔여 문서 드리프트) | 사용자가 명시 요청 |
| 스테일 루프 자동 재가동/스케줄 | non-goal(자동화 신설) | 사용자가 명시 요청 |
| 루프 폐기/강등 결정 | 관측 가능해진 뒤 사용자 판단(non-goal) | 사용자가 명시 요청 |

## Sibling SPEC Decision

| Decision | Reason | Sibling SPEC IDs |
|----------|--------|------------------|
| none | Primary SPEC이 한 모듈(autopus-adk) 내 cohesive read-revival+doctor 변경으로 Outcome Lock을 닫음 | None |

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| `pkg/learn/store.go`(Read/UpdateReuseCount), `types.go`, `query.go`, `prune.go`, `rewrite.go`, `summary.go`, `record.go` | existing | Read로 확인 |
| `internal/cli/learn_query.go`·`learn_record.go`(`--spec-id`), `doctor.go`·`doctor_json.go`·`doctor_json_checks.go`·`doctor_drift_output.go` | existing | Read로 확인 |
| `internal/cli/canary.go`·`canary_helpers.go::writeCanaryLatest`, `pkg/memindex/types.go::DefaultIndexPath`·`store.go`·`mem.go` | existing | Read로 확인 |
| `.claude/skills/autopus/auto-go.md:529`·`auto-fix.md:29`(`--spec` 문서화), `pkg/learn/store_test.go:123-137`(fail-all 계약) | existing | rg/Read로 확인 |
| 루트 `.autopus/learnings/pipeline.jsonl`(L-003 `+0900`; adk 사본은 L-001만) | existing | cat로 확인 |
| `internal/cli/doctor_evidence_freshness.go` + `_test.go`, `SkipRecord`, `evidenceFreshnessMaxAge`, check id `doctor.evidence.{learnings,canary,memindex}` | [NEW] planned addition | 미존재, 정합 검증 제외 |
| `RelevanceQuery.SpecID`, `ReadTolerant`, `--spec` 플래그 | [NEW] planned addition | 신규 |
| `content/rules/doc-storage.md` 신선도+`--spec` 언급 라인(source of truth) | [NEW] planned addition | source 수정; 설치본 `.claude/rules/autopus/doc-storage.md` 등은 generated 복사본이라 미편집 |

## Reviewer Brief

- **Intended scope**: autopus-adk 한 모듈에서 learn read 경로를 tolerant+canonical로 부활, `--spec` 정렬, doctor에 비차단 신선도 advisory 추가.
- **Explicit non-goals**: 자동 주입 강화, 신규 자동화/스케줄링, canary·memindex 기능 개조, 루프 폐기, bare query list-all, L-003 데이터 편집, `--limit`/`--format` 정렬.
- **Self-verified**: `+0900` 수용 vs skip 모순 해소(Outcome Lock 우선), 재작성 데이터 보존(무결성), memindex mtime anchor(스키마 불변), advisory 선례(context-weight/drift), fail-all→tolerant 테스트 전환, Traceability/Semantic Invariant/oracle acceptance/existing·[NEW] 구분.
- **Reviewer should focus on**: correctness(fallback 파싱 정확성, canonical marshal), 데이터 무결성(재작성 보존, 조용한 삭제 방지), advisory 비차단(`overall_ok` 유지), 기존 learn/doctor 회귀 위험, Completion Debt만. 새 제품 scope 제안은 범위 밖.

## Plan Intent Ledger

Clarification Ledger unavailable — 직접 `auto plan` 또는 BS 파일의 `## Clarification Ledger`/`## Question Audit`가 전달되지 않음. 팀리드 프롬프트의 Outcome Lock 초안은 위 Outcome Lock 섹션에 scope contract로 보존함. 팀리드 evidence 셀은 untrusted prompt 입력으로 취급해 evidence로만 요약했다.

## Revision 1 closure

| F-ID | category | how closed | files |
|------|----------|------------|-------|
| F-001 | correctness/feasibility | S3 트리거를 no-op `prune --max-age 100000`에서 실제 age-out `--max-age 30`(60일 엔트리)으로 교체해 `rewriteStore` 강제; canonical 정규화 + 손상 줄 보존을 함께 단정 | acceptance.md S3 / plan.md T7 / spec.md Traceability |
| F-003 | completeness | S10 오라클에 `--spec` 문서화 단정 추가, T6에 `--spec` 언급 명시 | acceptance.md S10 / plan.md T6 / spec.md REQ-010 |
| F-006 | correctness/feasibility | 편집 대상을 source `content/rules/doc-storage.md`로 정정, Reference Discipline에 source vs generated 복사본 분리 | spec.md 생성파일 / plan.md T6 / research.md Reference Discipline |
| F-002 | scope (non-blocking) | query `--limit`/`--format` 문서-CLI 드리프트는 Outcome Lock 밖이라 Evolution Ideas로 유지(미승격) | research.md Evolution Ideas |
| F-004 | advisory | T1을 `time.Time` 유지 + `LearningEntry` 커스텀 Un/MarshalJSON 방식으로 확정해 임베드 래퍼 ripple 제거 | plan.md T1 |
| F-005 | advisory | REQ-004/S3/S4에 보존 줄의 원본 상대 순서 유지 계약 추가 | spec.md REQ-004 / acceptance.md S3·S4 |
| F-007 | advisory | 손상 엔트리는 루트 파일에만 있고 adk 사본(L-001만)엔 없음을 정정, 라이브 재현은 루트 한정 | research.md 기존 코드 분석 |

## Self-Verify Summary

- Q-CORR-01 | status: FAIL | attempt: 1 | files: research.md, spec.md, plan.md | reason: F-006 — `content/rules/autopus/doc-storage.md` 기존 참조가 실재하지 않음(생성 표면 네임스페이스)
- Q-CORR-01 | status: PASS | attempt: 2 | files: research.md, spec.md, plan.md | reason: source 경로를 `content/rules/doc-storage.md`로 정정, 나머지 기존 참조는 Read/rg 확인 유지
- Q-CORR-02 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: 신규 파일·SkipRecord·check id·`--spec`를 `[NEW]`로 표기
- Q-CORR-03 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: EARS는 비불릿 THE SYSTEM SHALL, acceptance는 bare Given/When/Then
- Q-CORR-04 | status: FAIL | attempt: 1 | files: research.md | reason: F-006 — generated 표면 `autopus/` 네임스페이스를 source 경로로 오접합
- Q-CORR-04 | status: PASS | attempt: 2 | files: research.md | reason: Reference Discipline에서 source(`content/rules/doc-storage.md`) vs generated 복사본을 명시 분리
- Q-COMP-01 | status: PASS | attempt: 1 | files: all | reason: 4파일 상호 보완
- Q-COMP-02 | status: FAIL | attempt: 1 | files: spec.md, acceptance.md | reason: F-003 — REQ-010의 `--spec` 문서화 절이 어떤 acceptance에도 미추적
- Q-COMP-02 | status: PASS | attempt: 2 | files: spec.md, acceptance.md | reason: S10에 `--spec` 단정 추가, 10 REQ 전부 Traceability 매핑
- Q-COMP-04 | status: PASS | attempt: 1 | files: research.md | reason: Outcome Lock을 Primary SPEC이 단독으로 닫음
- Q-COMP-05 | status: FAIL | attempt: 1 | files: acceptance.md | reason: F-001 — INV-003(S3) 오라클의 `prune --max-age 100000`이 `pruned==0` short-circuit로 rewriteStore 미호출→canonical 관측 불가
- Q-COMP-05 | status: PASS | attempt: 2 | files: acceptance.md, spec.md, research.md | reason: S3를 실제 age-out(`--max-age 30`) 트리거로 교체, INV-001~005가 S1~S5 concrete oracle에 매핑
- Q-COMP-06 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: Traceability Matrix + Reviewer Brief로 리뷰 범위 제한
- Q-COMP-07 | status: PASS | attempt: 1 | files: research.md | reason: Completion Debt(None)와 Evolution Ideas(list-all/`--limit`/자동화/폐기) 분리
- Q-FEAS-01 | status: PASS | attempt: 1 | files: plan.md, research.md | reason: 런타임 코드 변경으로 layer 일치, 문서-CLI 정렬은 CLI 플래그 추가로 실현
- Q-FEAS-02 | status: FAIL | attempt: 1 | files: spec.md, plan.md, research.md | reason: F-006 — 편집 대상 `content/rules/autopus/doc-storage.md`가 소스 트리에 부재
- Q-FEAS-02 | status: PASS | attempt: 2 | files: spec.md, plan.md, research.md | reason: source `content/rules/doc-storage.md`로 정정, generated 복사본 미편집 명시
- Q-FEAS-03 | status: FAIL | attempt: 1 | files: acceptance.md | reason: F-001 — S3의 no-op prune이 실제 재작성을 일으키지 않아 오라클 수행 불가
- Q-FEAS-03 | status: PASS | attempt: 2 | files: acceptance.md | reason: S3를 실제 age-out prune으로 교체해 rewriteStore가 호출되어 오라클 수행 가능
- Q-STYLE-01 | status: PASS | attempt: 1 | files: spec.md | reason: REQ에 모호어 없음
- Q-STYLE-02 | status: PASS | attempt: 1 | files: spec.md | reason: Priority(Must/Should)와 EARS type 분리
- Q-SEC-01 | status: PASS | attempt: 1 | files: research.md | reason: pipeline.jsonl untrusted 성격·tolerant skip 완화·prompt evidence 신뢰경계 명시
- Q-SEC-02 | status: PASS | attempt: 1 | files: research.md | reason: timestamp/mtime만 읽고 secret·절대 privileged 경로 미노출, `--spec` equality 필터
