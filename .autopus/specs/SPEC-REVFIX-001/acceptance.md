# SPEC-REVFIX-001 수락 기준

각 수락 기준(AC)은 대응 요구사항(ReqID), 검증 방법(unit/integration/manual), 검증 위치(파일·함수)를 포함한다.

## AC1: Supermajority verdict — PASS 가결

- **ReqID**: REQ-01
- **검증**: unit test
- **위치**: `pkg/spec/reviewer_test.go::TestMergeVerdictsSupermajorityPass`
- **Given**: 3개 `ReviewResult` — `[VerdictPass, VerdictPass, VerdictRevise]`, threshold=0.67, totalProviders=3
- **When**: `MergeVerdicts(results, 0.67, 3)` 호출
- **Then**: 반환값 = `VerdictPass` (2/3 ≥ 0.67 이므로 PASS 다수 성립)

## AC2: Supermajority verdict — REVISE 가결 + REJECT 우선

- **ReqID**: REQ-01
- **검증**: unit test
- **위치**: `pkg/spec/reviewer_test.go::TestMergeVerdictsSupermajorityRevise`, `TestMergeVerdictsRejectOverrides`
- **Given**:
  - case A: `[VerdictPass, VerdictRevise, VerdictRevise]`, threshold=0.67
  - case B: `[VerdictPass, VerdictPass, VerdictReject]`, threshold=0.67
- **When**: `MergeVerdicts` 각각 호출
- **Then**:
  - case A → `VerdictRevise` (2/3 REVISE)
  - case B → `VerdictReject` (1 REJECT만으로 overriding — security gate 유지)

## AC3: Findings supermajority merge + dedup 통합

- **ReqID**: REQ-06, REQ-07
- **검증**: integration test
- **위치**: `internal/cli/spec_review_test.go::TestRunSpecReviewMergesDuplicateFindingsAcrossProviders`
- **Given**:
  - 3-provider mock orchestra 결과, 각 provider가 동일한 finding을 발급:
    - ScopeRef: `pkg/spec/reviewer.go:54`
    - Category: `correctness`
    - Description: `seq restarts at 1 per provider`
  - `gate.VerdictThreshold = 0.67`, `totalProviders = 3`
- **When**: `runSpecReview` 호출
- **Then**:
  - `merged.Findings`의 길이 = 1 (3개가 하나로 dedup)
  - 해당 finding의 ID = `F-001` (global sequential)
  - 저장된 `review-findings.json`의 모든 finding.ID가 전역 유니크, 충돌 없음

## AC4: Empty RawContent guard

- **ReqID**: REQ-05 (b)
- **검증**: integration test
- **위치**: `internal/cli/spec_review_test.go::TestRunSpecReviewRejectsEmptySpecContent`
- **Given**: spec.md 파일이 존재하지만 RawContent가 빈 문자열인 SPEC 디렉토리
- **When**: `runSpecReview` 호출
- **Then**:
  - 반환 error != nil
  - 에러 메시지에 `SPEC 본문이 비어있습니다` 포함
  - orchestra 호출이 1회도 발생하지 않음 (mock의 CallCount == 0)

## AC5: Revision 루프 내 spec.md 재로드

- **ReqID**: REQ-02
- **검증**: integration test
- **위치**: `internal/cli/spec_review_test.go::TestRunSpecReviewReloadsSpecBetweenRevisions`
- **Given**:
  - 초기 spec.md에 요구사항 `REQ-A` 1개 포함
  - 1차 revision 완료 후 test hook에서 spec.md를 수정하여 `REQ-B` 요구사항 추가
  - mock provider는 revision 0에서 REVISE, revision 1에서 PASS 반환하도록 설정
- **When**: `runSpecReview`가 2회 iteration 실행
- **Then**:
  - revision 1에서 생성된 프롬프트에 `REQ-B` 문자열이 포함됨(새로 로드된 내용 반영 증거)
  - 최종 verdict = PASS
  - circuit breaker는 발동하지 않음

## AC6: 다중 문서 주입 — plan/research/acceptance.md

- **ReqID**: REQ-03
- **검증**: unit test
- **위치**: `pkg/spec/prompt_test.go::TestBuildReviewPromptIncludesAuxiliaryDocs`
- **Given**:
  - SpecDocument (spec.md RawContent 포함)
  - specDir에 `plan.md` (50줄), `research.md` (250줄), `acceptance.md` (80줄) 존재
  - `doc_context_max_lines = 200`
- **When**: `BuildReviewPrompt(doc, codeContext, opts, specDir)` 호출
- **Then**:
  - 결과 문자열에 "## plan.md" 섹션 존재, 50줄 전부 포함
  - "## research.md" 섹션에 200줄까지 포함, 끝에 "... (trimmed 50 more lines)" 표기 존재
  - "## acceptance.md" 섹션 존재, 80줄 전부 포함
  - 파일이 없는 경우(예: research.md 삭제 후 재실행) 해당 섹션은 생략되고 에러는 발생하지 않음

## AC7: Critical/security escape hatch 유지

- **ReqID**: REQ-06 (기존 `findings.go:148-151` 동작 유지)
- **검증**: unit test
- **위치**: `pkg/spec/findings_test.go::TestMergeSupermajorityBypassesThresholdForSecurity` (기존 테스트 회귀 확인)
- **Given**: 3-provider 중 1 provider만 발급한 finding — severity=`critical` OR category=`security`
- **When**: `MergeSupermajority(findings, 3, 0.67)` 호출
- **Then**:
  - 해당 finding이 결과에 포함됨 (1/3 < 0.67 임에도 bypass)
  - non-critical/non-security finding은 supermajority 미달 시 제외됨

## AC8: Verdict decision rules 프롬프트 포함

- **ReqID**: REQ-04
- **검증**: unit test
- **위치**: `pkg/spec/prompt_test.go::TestBuildReviewPromptIncludesVerdictRules`
- **Given**: 기본 `ReviewGateConfig` (pass_criteria는 빈 문자열)
- **When**: `BuildReviewPrompt` 호출
- **Then**:
  - 프롬프트에 "### Verdict Decision Rules" 섹션 존재
  - 규칙 문자열에 `critical == 0`, `security == 0`, `major <= 2`, `PASS`, `REJECT`, `REVISE` 키워드 모두 포함
- **추가 case**: `PassCriteria = "custom override text"` 설정 시 해당 문자열이 프롬프트에 override되어 삽입됨

## AC9: Finding format few-shot 포함

- **ReqID**: REQ-05 (a)
- **검증**: unit test
- **위치**: `pkg/spec/prompt_test.go::TestBuildReviewPromptIncludesFewShotExamples`
- **Given**: 기본 discover mode opts
- **When**: `BuildReviewPrompt` 호출
- **Then**:
  - 프롬프트에 "### Finding Format Examples" 섹션 존재
  - positive 예시 2개 (structured format `FINDING: [major] [correctness] pkg/foo/bar.go:42 description`)
  - negative 예시 1개 (legacy format `FINDING: [major] description` — "AVOID" 라벨 포함)

## AC10: End-to-end regression smoke

- **ReqID**: 모든 REQ 통합
- **검증**: manual (CI는 선택)
- **위치**: 수동 실행 — `go test ./...` + `auto spec review --multi SPEC-HARN-PIPE-001`
- **Given**: 현재 REVISE 루프 소진으로 종료되던 SPEC (예: SPEC-HARN-PIPE-001)
- **When**: `auto spec review --multi <SPEC-ID>` 실행
- **Then**:
  - 서킷 브레이커가 revision 0에서 즉시 발동하지 않음
  - 동일 findings가 revision 간 무한 반복되지 않음
  - 최종 verdict가 PASS 또는 근거 있는 REVISE/REJECT (verdict 이유가 프롬프트 판정 기준에 매칭됨)
  - review-findings.json의 모든 finding.ID 전역 유니크

## 검증 커버리지 매트릭스

| ReqID | AC |
|-------|-----|
| REQ-01 | AC1, AC2 |
| REQ-02 | AC5 |
| REQ-03 | AC6 |
| REQ-04 | AC8 |
| REQ-05 | AC4, AC9 |
| REQ-06 | AC3, AC7 |
| REQ-07 | AC3 |
| 전체 | AC10 |

