# SPEC-ADK-EVIDENCE-LOOPS-001: Learning Read Revival and Doctor Evidence Freshness

**Status**: completed
**Created**: 2026-07-17
**Domain**: EVIDENCE-LOOPS
**Module**: autopus-adk

## 목적

세 증거 루프가 "배선만 존재"하고 사용되지 않는다. 핵심 결함은 `pkg/learn/store.go::Store.Read`(store.go:64)의 fail-all unmarshal(store.go:82-84)이다. `json.Unmarshal`이 한 줄이라도 실패하면 전체 읽기가 죽는다. 워크스페이스 루트 `.autopus/learnings/pipeline.jsonl`의 L-003 엔트리 timestamp `2026-06-14T14:31:55+0900`(콜론 없는 offset, 비RFC3339)이 이 죽음을 유발한다 — 라이브 재현: `auto learn query` → `Error: query: unmarshal entry: parsing time "2026-06-14T14:31:55+0900" as "2006-01-02T15:04:05Z07:00": cannot parse "+0900" as "Z07:00"`. `Read`는 `query`·`summary`·`prune`·`NextID`·`UpdateReuseCount` 모두가 호출하므로 학습 루프의 읽기 전체가 죽어 재사용(reuse)이 불가능하다.

부수 결함 두 가지도 함께 닫는다. (1) 스킬 문서(`.claude/skills/autopus/auto-go.md:529`·`auto-fix.md:29`)는 `auto learn query --spec {SPEC_ID}`를 호출하나 CLI에는 `--spec` 플래그가 없다(문서-CLI 불일치; `spec_id` 필드는 `LearningEntry`에 이미 존재). (2) 세 루프(learnings·canary·memindex)의 마지막 활동이 어디에서도 관측되지 않아 조용히 스테일해진다.

이 SPEC은 학습 읽기 경로를 tolerant하게 되살리고(손상 줄은 건너뛰되 흔한 비표준 timestamp는 fallback으로 수용, 쓰기·재작성은 canonical RFC3339 보장), `--spec` 필터로 문서-CLI를 정렬하며, `auto doctor`에 세 루프의 신선도를 **비차단 advisory**로 관측 가능하게 만든다. 자동화 신설·스케줄링·루프 폐기는 하지 않는다.

## Outcome Boundary

- **User-visible outcome**: (a) `auto learn query`가 손상 엔트리가 있어도 죽지 않고 유효 엔트리를 반환하며, 손상 줄은 건너뛰고 skip count+식별자(줄번호)를 경고로 표기한다. `+0900` 같은 흔한 비표준 offset은 fallback 포맷으로 수용해 엔트리를 유지한다. (b) `auto learn record`와 재작성 경로(prune·reuse-count)가 항상 canonical RFC3339 timestamp를 기록한다. (c) `auto learn query --spec SPEC-ID`가 동작한다. (d) `auto doctor`(text+`--json`)가 learnings·canary·memindex의 마지막 활동 age와 30일 임계를 비차단으로 보고하고 초과 시 WARN+재가동 힌트를 표기한다. `overall_ok`는 뒤집지 않는다.
- **Mandatory requirements**: REQ-001~REQ-010.
- **Explicit non-goals**: 학습 자동 주입 강화, 신규 자동화/스케줄링, canary·memindex 기능 자체 개조(빌드 동작·스키마 변경 없음, 읽기만), 루프 폐기/강등 결정(관측 가능해진 뒤 사용자 판단), bare(무필터) `learn query`의 list-all 의미 변경, 워크스페이스의 기존 L-003 데이터 파일 편집(tolerant 파서가 fallback으로 읽으므로 불요이며 이 SPEC 커밋 범위 밖).
- **Completion evidence**: fallback 수용(정상 2+`+0900` 1→3 반환, skip 0), tolerant skip(정상 2+손상 1→2 반환, skip 1+줄번호), canonical 직렬화 정규식, 재작성 시 손상 줄 보존, `--spec` 정확 필터, doctor 신선도 fresh/stale/absent/receipt-없음 oracle + JSON 미러 + 실 워크스페이스 라이브 확인.

## Requirements

### REQ-001: Tolerant per-entry read
WHEN `Store.Read`(또는 그 tolerant 후속)가 `pipeline.jsonl`을 읽을 때, THE SYSTEM SHALL 각 줄을 독립적으로 파싱하여 유효 엔트리를 반환하고, 파싱 불가한 줄은 건너뛰며 skip count와 식별자(줄번호)를 수집하고, 파싱 실패를 치명적 에러로 전파하지 않아야 한다(I/O 에러만 전파).
- EARS type: Event
- Priority: Must
- 관측 지점: `auto learn query`가 유효 엔트리 반환 + skip 경고 라인(줄번호 포함)

### REQ-002: Non-standard timestamp fallback acceptance
WHEN timestamp가 canonical RFC3339(`2006-01-02T15:04:05Z07:00`)로 파싱되지 않을 때, THE SYSTEM SHALL 콜론 없는 수치 offset 포맷(`2006-01-02T15:04:05-0700`, 소수 초 변형 포함)을 fallback으로 시도하여 `+0900` 같은 흔한 비표준 timestamp를 동일 순간으로 수용하고 해당 엔트리를 skip하지 않아야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: `+0900` 입력 엔트리가 query 결과에 포함되고 skip count에 기여하지 않음

### REQ-003: Canonical serialization on write and rewrite
WHEN 엔트리가 기록되거나(record) 재작성될 때(prune·reuse-count), THE SYSTEM SHALL timestamp를 항상 canonical RFC3339(콜론 포함 offset 또는 `Z`)로 직렬화하여, fallback으로 파싱된 `+0900` 엔트리도 재작성 후 콜론 포함 offset으로 정규화해야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: 산출 파일 라인이 `^[0-9T:.-]+(Z|[+-][0-9]{2}:[0-9]{2})$` offset 형태를 만족하고 콜론 없는 `+0900`이 잔존하지 않음

### REQ-004: Rewrite preserves unparseable lines
WHEN 재작성 경로(`Prune`의 `rewriteStore`, `UpdateReuseCount`의 inline 재작성)가 tolerant read 결과로 파일을 다시 쓸 때, THE SYSTEM SHALL 파싱 불가로 skip된 원본 줄을 변형 없이 그리고 원본 상대 순서를 유지한 채 보존하여, 관측 불가한 엔트리가 재작성으로 조용히 삭제되거나 재배열되지 않도록 해야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: 손상 줄 1개 포함 fixture에서 실제 age-out prune 후 파일에 그 원본 줄이 원본 상대 순서로 그대로 남아 있음

### REQ-005: `--spec` query filter
WHEN `auto learn query --spec SPEC-ID`가 실행될 때, THE SYSTEM SHALL `spec_id` 필드가 정확히 일치하는 엔트리로 후보를 제한하고, `--spec`가 유일한 조건이면 관련도 점수와 무관하게 일치 엔트리를 모두 반환해야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: `--spec SPEC-A`가 SpecID `SPEC-A` 엔트리만 반환(SPEC-B·빈 값 제외)

### REQ-006: Doctor evidence freshness advisory
WHEN `auto doctor`가 실행되고 한 루프가 워크스페이스에 존재할 때, THE SYSTEM SHALL 그 루프의 마지막 활동 시각(learnings=최신 tolerant-parsed 엔트리 timestamp, canary=`.autopus/canary/latest.json`의 `timestamp`, memindex=`DefaultIndexPath` 인덱스 파일 mtime)에서 age를 계산하고 30일 임계 초과 시 WARN+재가동 힌트를 비차단으로 보고해야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: text `Evidence Freshness` 섹션 + JSON `doctor.evidence.{learnings,canary,memindex}` check의 status/age/hint

### REQ-007: JSON mirror and advisory non-blocking
WHEN `auto doctor --json`이 실행될 때, THE SYSTEM SHALL 세 신선도 check를 doctor check 계약(`jsonCheck{ID,Severity,Status,Detail}`)으로 JSON checks 배열에 미러하고, 신선도가 advisory이므로 어떤 결과에서도 `overall_ok`를 true로 유지해야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: JSON `checks[]`의 세 check + `data.overall_ok`

### REQ-008: Graceful skip when a loop is absent
WHEN 한 루프의 존재 신호가 없을 때(learnings 저장 파일 부재, `.autopus/project/canary.md` 부재, memindex 인덱스 부재), THE SYSTEM SHALL 해당 신선도 check를 경고 없이 조용히 스킵해야 한다. 단 canary 구성 문서는 있으나 실행 receipt가 없으면 "미실행" WARN+힌트를 보고해야 한다.
- EARS type: Event
- Priority: Must
- 관측 지점: 부재 루프의 check 부재; canary.md 존재+receipt 부재 시 `doctor.evidence.canary` WARN

### REQ-009: Platform-neutral revive hints
THE SYSTEM SHALL 재가동 힌트를 claude-code·codex·antigravity-cli·opencode 사용자 모두에게 유효한 플랫폼 중립 명령(`auto learn record`, `auto canary`, `auto mem rebuild`)으로 제시해야 한다.
- EARS type: Ubiquitous
- Priority: Must
- 관측 지점: WARN check detail의 플랫폼 중립 힌트 문자열

### REQ-010: Rule documentation mention
THE SYSTEM SHALL 신선도 advisory와 `--spec` query 필터의 존재를 doctor advisory 가드가 문서화된 content 규칙 참조 한 곳에 1~2줄로 기록해야 한다.
- EARS type: Ubiquitous
- Priority: Should
- 관측 지점: 규칙 파일 언급 라인에 `freshness`(또는 `신선도`)와 `auto doctor` 포함

## 생성 파일 상세

- `pkg/learn/types.go`(수정) — tolerant unmarshal + canonical marshal timestamp 타입(또는 `LearningEntry` 커스텀 (Un)MarshalJSON); `RelevanceQuery`에 `SpecID` 추가(REQ-002·003·005).
- `pkg/learn/store.go`(수정) — `Read`를 per-line tolerant로: skip된 원본 줄을 `SkipRecord{Line,Raw,Reason}`로 수집; `UpdateReuseCount` inline 재작성이 skip된 원본 줄 보존(REQ-001·004).
- `pkg/learn/rewrite.go`(수정) — `rewriteStore`가 보존 대상 원본 줄을 재작성 산출에 포함(REQ-004).
- `pkg/learn/query.go`(수정) — `QueryRelevant`가 `SpecID` 하드 필터 적용(REQ-005).
- `internal/cli/learn_query.go`(수정) — `--spec` 플래그 + skip 경고 출력(REQ-005·001).
- `[NEW] internal/cli/doctor_evidence_freshness.go` — 세 anchor 측정·존재 신호 게이팅·30일 임계·text 렌더·JSON check(REQ-006~009).
- `internal/cli/doctor.go`·`doctor_json.go`(수정) — 신선도 검사 배선(REQ-006·007).
- `content/rules/doc-storage.md`(source 수정) — 신선도 가드와 `--spec` 필터를 1~2줄로 언급 + `generate-templates`로 4플랫폼 설치본 재생성(REQ-010). 설치본 `.claude/rules/autopus/doc-storage.md` 등 generated 복사본은 직접 편집하지 않는다.
- `[NEW]` 대응 `_test.go` 3종 + `pkg/learn/store_test.go`(수정, fail-all 계약 전환).

## Related SPECs

None (Primary SPEC이 Outcome Lock을 단독으로 닫는다). SPEC-LEARN-001(학습 store 계약)·SPEC-LEARNWIRE-002(주입 배선)·SPEC-CANARY-001의 기존 파일 포맷·플래그와 하위호환을 유지하며 의존 관계는 아니다.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-001 (Must) | T2, T7 | S2 | INV-001 |
| REQ-002 (Must) | T1, T7 | S1 | INV-002 |
| REQ-003 (Must) | T1, T7 | S3 | INV-003 |
| REQ-004 (Must) | T2, T7 | S3, S4 | INV-004 |
| REQ-005 (Must) | T1, T3, T8 | S5 | INV-005 |
| REQ-006 (Must) | T4, T5, T9 | S6, S7, S9 | INV-006 |
| REQ-007 (Must) | T4, T5, T9 | S6, S7 | INV-007 |
| REQ-008 (Must) | T4, T9 | S8, S9 | INV-006 |
| REQ-009 (Must) | T4, T9 | S7 | INV-007 |
| REQ-010 (Should) | T6 | S10 | INV-007 |
