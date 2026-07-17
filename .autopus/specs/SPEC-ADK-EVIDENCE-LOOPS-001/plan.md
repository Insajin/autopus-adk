# SPEC-ADK-EVIDENCE-LOOPS-001 구현 계획

## Tasks

- [x] T1: `pkg/learn/types.go` — `LearningEntry.Timestamp`를 `time.Time`으로 유지한 채 `LearningEntry`에 커스텀 `UnmarshalJSON`/`MarshalJSON`을 추가해 tolerant-unmarshal + canonical-marshal을 구현한다(F-004: `time.Time` 임베드 래퍼는 `time.Since(entry.Timestamp)`·`Timestamp: time.Now()`에서 `.Time` 접근이 필요해 ripple이 크므로 지양). Unmarshal은 `time.RFC3339`/`RFC3339Nano` 우선, 실패 시 `2006-01-02T15:04:05-0700`과 소수초 변형을 fallback으로 시도한다. Marshal은 항상 콜론 offset RFC3339로 직렬화한다. `RelevanceQuery`에 `SpecID string` 필드를 추가한다. `Timestamp`를 `time.Time`으로 유지하므로 `query.go`의 `entry.Timestamp.IsZero()`/`time.Since(...)`, `store.go`의 `Timestamp: time.Now()`, `prune.go`의 `Before/Equal` 호출은 수정 없이 그대로 동작한다(REQ-002·003·005).
- [x] T2: `pkg/learn/store.go` — `Read`를 per-line tolerant로 바꾼다. `json.Unmarshal` 실패 시 그 줄을 `SkipRecord{Line int, Raw string, Reason string}`에 담고 계속 진행하며, I/O 에러만 반환한다. 유효 엔트리 슬라이스와 skip 목록을 함께 제공한다(예: `ReadTolerant() ([]LearningEntry, []SkipRecord, error)`; 기존 `Read`는 유효 엔트리만 반환하도록 얇게 래핑). `UpdateReuseCount`의 inline 재작성이 skip된 원본 줄을 보존하도록 skip 목록을 넘긴다(REQ-001·004).
- [x] T3: `pkg/learn/query.go` + `internal/cli/learn_query.go` — `QueryRelevant`가 `query.SpecID != ""`이면 `entry.SpecID == query.SpecID`로 후보를 하드 필터하고, SpecID가 유일 조건이면 점수와 무관하게 일치 엔트리를 모두 반환한다. CLI에 `--spec` 플래그(`StringVar`)를 추가하고, skip count>0이면 줄번호를 포함한 경고를 stderr로 출력한다(테이블은 stdout 유지)(REQ-005·001).
- [x] T4: `[NEW] internal/cli/doctor_evidence_freshness.go` — 세 anchor를 측정한다. learnings=`ReadTolerant`로 얻은 최신 엔트리 timestamp, canary=`.autopus/canary/latest.json`의 `timestamp` 필드(RFC3339), memindex=`memindex.DefaultIndexPath(dir)` 파일 mtime. 존재 신호(learnings 저장 파일 / `.autopus/project/canary.md` / memindex 인덱스)로 게이팅하고, `evidenceFreshnessMaxAge = 30*24*time.Hour` 초과 시 WARN. canary.md 존재+receipt 부재는 "미실행" WARN. `renderEvidenceFreshnessText`(text)와 `collectEvidenceFreshnessChecks`(JSON `jsonCheck`)를 `collectContextWeightChecks`/`collectDriftGateChecks` advisory 패턴대로 작성하되 `r.status`를 건드리지 않는다. 힌트는 플랫폼 중립(REQ-006·007·008·009).
- [x] T5: `internal/cli/doctor.go::runDoctorText`(context-weight/drift 뒤)와 `internal/cli/doctor_json.go::collectDoctorJSONReport`(`collectDriftGateChecks` 뒤)에 신선도 검사를 배선한다. cfg 로드 실패 경로에서도 안전 스킵(REQ-006·007).
- [x] T6: source 규칙 파일 `content/rules/doc-storage.md`의 Weight Guard/Drift Guard와 나란히 Evidence Freshness 가드를 1~2줄로 언급하되 `freshness`/`신선도`+`auto doctor`와 `--spec` query 필터를 함께 적고(REQ-010의 두 문서화 대상), `generate-templates`로 4플랫폼 설치본을 재생성한다. 설치본 `.claude/rules/autopus/doc-storage.md` 등 generated 복사본은 직접 편집하지 않는다(REQ-010).
- [x] T7: `[NEW] pkg/learn/store_tolerant_test.go` + `pkg/learn/store_test.go`(수정) — S1(정상2+`+0900`1→3 반환, skip 0), S2(정상2+잘린JSON1→2 반환, skip 1, 줄번호 3), S3(`--max-age 30`으로 60일 엔트리 실제 age-out→생존 `+0900`이 `+09:00` 정규화, 손상 줄 상대 순서 보존), S4(손상 줄 prune 후 보존) oracle. 기존 store_test.go:123-137의 "invalid json→assert.Error" 계약을 tolerant(skip, 무에러)로 전환한다.
- [x] T8: `[NEW] internal/cli/learn_query_spec_test.go` — S5(`--spec SPEC-A` 정확히 2행, SPEC-B·빈값 제외) oracle.
- [x] T9: `[NEW] internal/cli/doctor_evidence_freshness_test.go` — S6(fresh 세 pass, overall_ok true), S7(stale 세 warn+각 힌트, overall_ok true), S8(부재 세 check 부재), S9(canary.md 존재+receipt 부재→canary warn), S10(규칙 언급 라인) oracle + 실 워크스페이스 `auto doctor --json` 라이브 확인.

## Implementation Strategy

- **재사용 우선, 신규 의존성 0**: `encoding/json`, `time`(RFC3339 layouts), `bufio`(기존 `Read` scanner), `os.Stat`(mtime), `strings`를 재사용한다. doctor 측은 `jsonCheck` 계약, `tui.SectionHeader/OK/Warn`, `collectContextWeightChecks`/`collectDriftGateChecks` advisory 선례, `memindex.DefaultIndexPath`, `canaryResult.Timestamp`(기존 `writeCanaryLatest` 산출)를 그대로 읽는다. 새 라이브러리·새 추상화 없음.
- **하위호환**: `LearningEntry` JSON 필드명은 불변(`timestamp` 등). canonical marshal은 Go 기본 `time.Time` 직렬화와 동일 형태라 기존 정상 엔트리에 무해. `--spec-id`(record)는 그대로 두고 query에만 `--spec`를 추가한다(문서 `auto-go.md`/`auto-fix.md`가 query에 쓰는 이름과 정렬).
- **비파괴 읽기/파괴 쓰기 분리**: 읽기(query/summary/doctor)는 tolerant 결과만 소비한다. 쓰기(prune/reuse-count)는 tolerant 결과 + 보존 대상 원본 줄을 함께 재작성해 skip이 데이터 손실로 번지지 않게 한다(REQ-004). 이것이 reuse-count 부활과 무결성을 동시에 만족시키는 핵심.
- **anchor는 기존 증거만 읽음**: memindex는 스키마·빌드 동작을 바꾸지 않고 인덱스 파일 mtime만 관측한다(non-goal 준수). canary는 기존 `latest.json` receipt만 읽는다. learnings는 tolerant read의 최신 timestamp를 쓴다.
- **비차단**: 세 신선도 check는 advisory. `r.status`/`overall_ok` 불변(context-weight·drift 선례). 스테일 워크스페이스도 doctor 실패로 보고되지 않는다.
- **라인 예산**: `doctor_evidence_freshness.go`는 측정+렌더+JSON을 한 파일에 담되 ≤300줄 목표(측정 helper·anchor 구조체·render·jsonCheck 4파트). 테스트는 파일별 분리.

## Visual Planning Brief (command/data-flow)

```mermaid
flowchart TD
  Q[auto learn query --spec/--packages] --> R[Store.ReadTolerant]
  R --> R1{line json.Unmarshal ok?}
  R1 -- ok --> R2[timestamp: RFC3339 else -0700 fallback]
  R1 -- fail --> R3[SkipRecord{line,raw}: skip + count]
  R2 --> R4[valid entries]
  R3 --> R4
  R4 --> QF{--spec set?}
  QF -- yes --> QF1[filter SpecID == spec]
  QF -- no --> QF2[relevance score >= threshold]
  QF1 --> QO[stdout table + stderr skip warning]
  QF2 --> QO
  R4 --> W[prune / reuse-count rewrite]
  R3 --> W
  W --> WC[canonical RFC3339 marshal + preserve skipped raw lines]
  D[auto doctor / --json] --> D1[measure 3 anchors]
  D1 --> DA[learnings: newest tolerant ts]
  D1 --> DB[canary: latest.json timestamp]
  D1 --> DC[memindex: DefaultIndexPath mtime]
  DA --> DG{present signal?}
  DB --> DG
  DC --> DG
  DG -- absent --> DS[silent skip; canary.md+no receipt=never-run warn]
  DG -- present --> DT{age > 30d?}
  DT -- yes --> DW[warn + platform-neutral revive hint]
  DT -- no --> DP[pass]
  DW --> DJ[jsonCheck advisory; overall_ok unchanged]
  DP --> DJ
```

## Feature Completion Scope

Primary SPEC이 Outcome Lock을 단독으로 닫는다. (a) tolerant read+fallback=T1/T2/T7, (b) canonical 직렬화=T1/T7, (c) 재작성 무결성=T2/T7, (d) `--spec` 정렬=T1/T3/T8, (e) doctor 신선도 advisory+미러+스킵+힌트=T4/T5/T9, 문서=T6. 승인된 sibling 의존성 없음. Completion Debt 없음(모든 mandatory requirement가 태스크로 커버됨). 워크스페이스의 기존 L-003 데이터 파일은 tolerant 파서가 fallback으로 읽으므로 편집 불요이며 이 SPEC 커밋 범위 밖이다.
