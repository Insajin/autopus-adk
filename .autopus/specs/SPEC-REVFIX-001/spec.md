# SPEC-REVFIX-001: SPEC 리뷰 수렴성 재구축

**Status**: completed
**Created**: 2026-04-18
**Domain**: REVFIX
**Related**: SPEC-REVCONV-001 (연장 — 설계는 유지, 프로덕션 통합 완성)

## 목적

`auto spec review --multi` 명령이 거의 모든 SPEC에서 PASS에 도달하지 못하고 REVISE 루프를 소진한 뒤 circuit breaker로 종료되는 구조적 결함을 제거한다.

메인 세션이 `pkg/spec/` 및 `internal/cli/spec_review.go`를 직접 조사한 결과, 원인은 단일 버그가 아니라 7개의 복합 결함이다:

1. `MergeVerdicts`는 1명이라도 REVISE면 최종 REVISE — **unanimous PASS**를 요구하므로 3-provider 환경에서 PASS 도달이 통계적으로 어려움
2. revision 루프가 시작 전 1회만 `spec.Load`를 호출 — 외부 수정이 반영되지 않아 **같은 findings 반복 → circuit breaker 즉시 발동**
3. `BuildReviewPrompt`가 spec.md RawContent만 주입 — plan.md/research.md/acceptance.md 본문이 빠져 provider가 **맥락 부족 상태로 지적**(SPEC-REVCONV-001 review.md의 "acceptance.md의 Finding ID 형식이 모순" 지적이 직접 증거)
4. 프롬프트에 PASS 판정 기준이 명문화되지 않음 — provider 주관으로 verdict가 갈라짐
5. structured FINDING few-shot 예시 부재 + `RawContent == ""` guard 부재 — SPEC-HARN-PIPE-001에서 3 provider 전원이 "SPEC 본문이 없다" critical 지적을 낸 것이 증거
6. `DeduplicateFindings`, `MergeSupermajority`, `MergeFindingStatuses`는 SPEC-REVCONV-001이 구현했으나 **프로덕션 경로에서 호출되지 않는 dead code** — `spec_review.go:130-138`은 단순 `append`로 3 provider의 findings를 누적
7. `parseDiscoverFindings`가 provider별로 `seq := 1`로 ID를 재시작 — 3 provider가 모두 `F-001, F-002`를 발급 → merge 시 **ID 충돌**, `ApplyScopeLock`이 잘못된 finding을 "이미 알려진" 것으로 간주

본 SPEC은 **SPEC-REVCONV-001의 설계를 유지**하고, 누락된 **프로덕션 통합**과 **프롬프트 품질 강화**, **verdict 임계값 정책**을 완성한다.

## 요구사항

### REQ-01: Verdict merge supermajority threshold
- WHEN `MergeVerdicts`가 다수의 `ReviewResult`를 병합할 때, THE SYSTEM SHALL `spec.review_gate.verdict_threshold`(기본 0.67) 기준 supermajority 로직을 적용해야 한다.
- WHERE totalProviders 중 threshold 이상이 동일 verdict를 반환했을 때만 해당 verdict를 최종 판정으로 채택한다.
- WHEN supermajority가 성립하지 않을 때, THE SYSTEM SHALL PASS 쪽으로 편향된 기본값(`VerdictRevise`보다 `VerdictPass`)을 반환하되, 1명이라도 `VerdictReject`면 즉시 REJECT로 종료한다(security 게이트는 유지).

### REQ-02: Revision 루프 내 spec.md 재로드
- WHILE revision 루프가 `revision > 0`에서 반복될 때, THE SYSTEM SHALL 각 iteration 시작 시 `spec.Load(specDir)`를 재호출하여 외부에서 수정된 spec.md/plan.md/acceptance.md를 반영해야 한다.
- WHEN `spec.Load`가 재호출 실패 시, THE SYSTEM SHALL 직전 revision의 doc을 유지하되 경고 로그를 출력한다.

### REQ-03: plan.md / research.md / acceptance.md 본문 주입
- WHEN `BuildReviewPrompt`가 호출될 때, THE SYSTEM SHALL spec.md RawContent 외에 plan.md, research.md, acceptance.md의 raw 본문을 각각 별도 섹션으로 프롬프트에 포함해야 한다.
- WHERE 각 파일이 `spec.review_gate.doc_context_max_lines`(기본 200)를 초과할 때, THE SYSTEM SHALL 앞부분 기준으로 trim하고 말미에 `... (trimmed {N} more lines)` 표기를 남긴다.
- WHERE 해당 파일이 존재하지 않을 때, THE SYSTEM SHALL 해당 섹션을 생략한다(에러 아님).

### REQ-04: PASS 판정 기준 프롬프트 명문화
- WHEN `buildDiscoverInstructions` 또는 `buildVerifyInstructions`가 호출될 때, THE SYSTEM SHALL verdict 판정 규칙을 프롬프트에 명시적으로 포함해야 한다.
- 기본 규칙: `critical == 0 AND security == 0 AND major ≤ 2 → PASS`; `critical > 0 OR security > 0 → REJECT`; 그 외 → `REVISE`
- WHERE `spec.review_gate.pass_criteria`가 설정되어 있을 때, THE SYSTEM SHALL 해당 문자열 템플릿을 override로 사용한다.

### REQ-05: FINDING 포맷 강제 + empty RawContent guard
- WHEN 프롬프트를 구성할 때, THE SYSTEM SHALL structured FINDING 포맷에 대한 positive few-shot 예시 2개와 legacy format negative 예시 1개를 포함해야 한다.
- WHEN `runSpecReview`가 호출되고 `doc.RawContent == ""`일 때, THE SYSTEM SHALL 루프 진입 전에 명시적 에러(`SPEC 본문이 비어있습니다: {specID}`)를 반환해야 한다.

### REQ-06: DeduplicateFindings + MergeSupermajority 프로덕션 통합
- WHEN `runSpecReview`가 provider들의 findings를 병합할 때, THE SYSTEM SHALL 다음 순서로 처리해야 한다:
  1. 모든 provider findings를 flatten
  2. `MergeSupermajority(findings, totalProviders, threshold)` 호출 — 2+ provider 동의한 finding만 활성화
  3. `DeduplicateFindings(findings)` 호출 — ScopeRef + Category + Description 정규화 기준 중복 제거 및 global sequential ID 재발급
- WHERE finding의 severity가 `critical`이거나 category가 `security`일 때, THE SYSTEM SHALL supermajority 임계값을 우회하여 1 provider만 제기해도 포함해야 한다(`findings.go:148-151` 기존 동작 유지).

### REQ-07: Provider 간 Finding ID 전역 유니크화
- WHEN `parseDiscoverFindings`가 findings를 생성할 때, THE SYSTEM SHALL finding의 `ID` 필드를 빈 문자열("")로 두고 sequential 번호를 부여하지 않아야 한다.
- WHEN `DeduplicateFindings`가 호출될 때, THE SYSTEM SHALL 결과 findings에 global sequential ID(`F-001`, `F-002`, ...)를 재발급해야 한다(기존 `findings.go:71-74` 로직 유지).
- 선택된 옵션: **옵션 B** (빈 ID → merge 단계에서 global 재발급). 근거는 research.md D2 참조.

## 생성/수정 파일 상세

| 파일 | 역할 | 예상 변경 |
|------|------|-----------|
| `pkg/spec/prompt.go` | 프롬프트 빌더 | REQ-03 (다중 문서 주입), REQ-04 (판정 기준), REQ-05a (few-shot) |
| `pkg/spec/reviewer.go` | 파싱 + merge | REQ-01 (MergeVerdicts threshold), REQ-07 (parseDiscoverFindings ID 제거) |
| `pkg/spec/findings.go` | dedup/merge (기존) | 시그니처 변경 없음 — 호출 시점만 변경 |
| `internal/cli/spec_review.go` | 루프 (현재 282줄) | REQ-02 (재로드), REQ-05b (guard), REQ-06 (통합). 300줄 초과 방지 위해 루프 로직 분리 검토 |
| `internal/cli/spec_review_loop.go` (신규) | 루프 본체 분리 | 파일 크기 제한 회피 |
| `pkg/config/spec.go` (또는 해당 파일) | 신규 config key | `verdict_threshold`, `pass_criteria`, `doc_context_max_lines` |
| `autopus.yaml` | 샘플 주석 업데이트 | 신규 key 예시 |
| `pkg/spec/prompt_test.go` | 테스트 | REQ-03, REQ-04, REQ-05a 검증 |
| `pkg/spec/reviewer_test.go` | 테스트 | REQ-01, REQ-07 검증 |
| `internal/cli/spec_review_test.go` | 테스트 | REQ-02, REQ-05b, REQ-06 통합 검증 |

## 설계 결정

- **D1**: REVCONV-001 함수 시그니처 동결. `DeduplicateFindings`, `MergeSupermajority`, `ApplyScopeLock`, `ShouldTripCircuitBreaker`, `MergeFindingStatuses`의 공개 API는 변경하지 않는다. 호출 시점과 인자 source만 바꾼다. 근거: 기존 단위 테스트가 regression safety net 역할을 수행해야 함.
- **D2**: `MergeSupermajority` → `DeduplicateFindings` 순서(역순 아님). 이유는 research.md §트레이드오프 참조.
- **D3**: `verdict_threshold` 기본값 0.67. 3-provider 환경에서 2/3 동의 시 PASS 성립. 대안 0.5(과반)는 research.md에서 비교.
- **D4**: Config key는 선택적. 기본값만으로 기존 autopus.yaml 호환.
- **D5**: 루프 로직은 필요 시 `spec_review_loop.go`로 분리하여 300줄 제한 준수.

