# SPEC-SPECREV-001 수락 기준

본 SPEC의 oracle acceptance는 concrete input과 expected output을 명시한다.
Heading 존재 / 파일 존재 / exit code 만으로는 PASS로 간주하지 않는다.

## 시나리오

### AC-CTX-1: 인용 파일 4개 → 한도 1500 적용 (REQ-CTX-1, Must oracle)

Given `pkg/config/defaults.go`의 `ContextMaxLines` 가 500으로 설정된 상태이다.
And SPEC `SPEC-FAKE-CTX1` 의 `research.md` 가 다음 4개 경로를 인용한다: `pkg/a.go`, `pkg/b.go`, `pkg/c.go`, `pkg/d.go`.
And 각 파일은 200줄 길이의 Go 소스이다.
When `internal/cli/spec_review.go`의 `runSpecReview`가 호출된다.
Then 적용된 한도는 1500이다.
And `spec.CollectContextForSpec` 가 반환한 누적 라인 수는 800줄에서 1500줄 사이이다.
And 4개 파일 헤더 (`--- pkg/a.go ---`, `--- pkg/b.go ---`, `--- pkg/c.go ---`, `--- pkg/d.go ---`) 가 모두 결과 문자열에 포함된다.
And stderr에 `SPEC review context: cited=4 applied=1500` 형태의 한 줄이 기록된다.

### AC-CTX-2: frontmatter override 우선 적용 (REQ-CTX-2, Must oracle)

Given SPEC `SPEC-FAKE-CTX2` 의 `spec.md` frontmatter에 `review_context_lines: 800` 이 명시되어 있다.
And 같은 SPEC의 `research.md` 가 7개 파일을 인용하여 자동 매핑은 3000을 산출한다.
And `autopus.yaml` 의 `spec.review_gate.context_max_lines` 는 미설정 상태이다.
When `runSpecReview`가 호출된다.
Then 적용된 한도는 800이다 (자동 매핑 3000보다 frontmatter 800이 우선).
And stderr 라인은 `SPEC review context: cited=7 applied=800 override=frontmatter` 이다.

### AC-CTX-3: 인용 파일 0개 → 기본 500 유지 (REQ-CTX-1 회귀, Must oracle)

Given SPEC `SPEC-FAKE-CTX3` 의 `research.md` 와 `plan.md` 에 어떤 코드 경로도 인용되지 않는다.
When `runSpecReview`가 호출된다.
Then 적용된 한도는 500이다.
And `spec.CollectContextForSpec` 결과 문자열은 빈 문자열이다 (인용 대상이 없으므로).
And stderr 라인은 `SPEC review context: cited=0 applied=500` 이다.

### AC-CTX-OVR-INVALID: frontmatter 음수 override는 거부 (REQ-CTX-3, Must oracle)

Given SPEC `SPEC-FAKE-CTX4` 의 frontmatter에 `review_context_lines: -1` 가 명시되어 있다.
And 같은 SPEC의 자동 매핑 결과는 1500이다.
When `runSpecReview`가 호출된다.
Then 적용된 한도는 1500이다 (자동 매핑 fallback).
And stderr에 `경고: review_context_lines 무시 (값=-1, 사유=must be >0 and <=10000)` 라인이 기록된다.

### AC-CTX-OVR-OVER: frontmatter 10001 override는 거부 (REQ-CTX-3, Must oracle)

Given SPEC `SPEC-FAKE-CTX5` 의 frontmatter에 `review_context_lines: 10001` 가 명시되어 있다.
And 같은 SPEC의 자동 매핑 결과는 3000이다.
When `runSpecReview`가 호출된다.
Then 적용된 한도는 3000이다.
And stderr에 `경고: review_context_lines 무시 (값=10001, 사유=must be >0 and <=10000)` 라인이 기록된다.

### AC-CTX-CEIL: autopus.yaml ceiling 적용 (REQ-CTX-4, Must oracle)

Given `autopus.yaml`의 `spec.review_gate.context_max_lines: 1200` 이 명시되어 있다.
And SPEC `SPEC-FAKE-CTX6` 의 자동 매핑 결과는 1500이다 (인용 4개).
And 같은 SPEC의 frontmatter에 `review_context_lines: 2500` 이 명시되어 있다.
When `runSpecReview`가 호출된다.
Then 적용된 최종 한도는 1200이다 (frontmatter override 2500과 자동 매핑 1500 모두 ceiling 1200으로 캡됨).
And stderr 라인은 `SPEC review context: cited=4 applied=1200 override=frontmatter ceiling=1200` 이다.

### AC-VERD-1: 1 PASS / 2 timeout → REVISE + degraded 라벨 (REQ-VERD-1/2, Must oracle)

Given configured providers 가 `[claude, gemini, codex]` (총 3개)이다.
And `claude` provider 만 `VERDICT: PASS` 응답을 반환한다.
And `gemini` 와 `codex` 는 timeout으로 응답하지 않는다.
And `autopus.yaml` 의 `spec.review_gate.exclude_failed_from_denom` 는 `false` (기본값) 이다.
When `runSpecReviewLoop`가 결과를 머지하고 `formatReviewMd`가 review.md를 빌드한다.
Then `denom = 3`, `passCount = 1`, `passCount/denom = 0.333...` 이며 0.67 미만이므로 final verdict 는 `REVISE` 이다.
And review.md 의 verdict 라인은 정확히 `**Verdict**: REVISE (degraded — 1/3 providers responded)` 이다.
And review.md 의 `## Provider Health` 섹션은 다음 세 행을 포함한다:
  | Provider | Status | Note |
  | claude | success | - |
  | gemini | timeout | - |
  | codex | timeout | - |

### AC-VERD-2: 2 PASS / 1 timeout (33% failure) → degraded 라벨 미부착 (REQ-VERD-2 boundary, Must oracle)

Given configured providers 가 `[claude, gemini, codex]` 이다.
And `claude` 와 `gemini` 가 `VERDICT: PASS` 를 반환한다.
And `codex` 가 timeout 한다.
And `exclude_failed_from_denom` 는 `false` 이다.
When `runSpecReviewLoop`가 결과를 머지하고 `formatReviewMd`가 review.md를 빌드한다.
Then `passCount/denom = 2/3 = 0.6667` 이고 tolerance 0.005를 더하면 0.6717 ≥ 0.67 이므로 final verdict 는 `PASS` 이다.
And failure 비율 1/3 ≈ 33.3% 는 50% 미만이므로 degraded 라벨은 부착되지 않는다.
And review.md 의 verdict 라인은 정확히 `**Verdict**: PASS` 이다 (괄호 라벨 없음).
And `## Provider Health` 섹션은 여전히 렌더되며 `codex` 행의 Status 가 `timeout` 이다.

### AC-VERD-3: exclude_failed_from_denom=true 일 때 1 PASS / 2 timeout → PASS + degraded (REQ-VERD-3/4, Must oracle)

Given configured providers 가 `[claude, gemini, codex]` 이다.
And `claude` 만 `VERDICT: PASS` 를 반환한다.
And `gemini`, `codex` 는 timeout 한다.
And `autopus.yaml` 의 `spec.review_gate.exclude_failed_from_denom: true` 가 명시되어 있다.
When `runSpecReviewLoop`가 결과를 머지한다.
Then `MergeVerdictsWithDenomMode` 호출 시 `excludeFailed=true, failedCount=2` 가 전달된다.
And `denom = 3 - 2 = 1` 이다.
And `passCount/denom = 1/1 = 1.0 ≥ 0.67` 이므로 final verdict 는 `PASS` 이다.
And review.md 의 verdict 라인은 정확히 `**Verdict**: PASS (degraded — 1/3 providers responded)` 이다 (PASS여도 REQ-VERD-4에 따라 degraded 라벨 부착).
And `## Provider Health` 섹션은 3개 행을 포함하고 timeout 2건을 명시한다.

### AC-VERD-4: 50% boundary (2/4 failure) → degraded 부착 (REQ-VERD-2 boundary, Must oracle)

Given configured providers 가 `[claude, gemini, codex, opus2]` (총 4개) 이다.
And `claude` 와 `gemini` 가 `VERDICT: PASS` 를 반환한다.
And `codex` 와 `opus2` 가 timeout 한다.
And `exclude_failed_from_denom` 는 `false` 이다.
When `runSpecReviewLoop`가 결과를 머지한다.
Then `passCount/denom = 2/4 = 0.5` 이고 tolerance 0.005를 더해도 0.505 < 0.67 이므로 final verdict 는 `REVISE` 이다.
And failure 비율 2/4 = 0.5 가 inclusive 50% 임계값을 만족하므로 degraded 라벨이 부착된다.
And review.md 의 verdict 라인은 정확히 `**Verdict**: REVISE (degraded — 2/4 providers responded)` 이다.

### AC-VERD-BACKCOMPAT: exclude_failed_from_denom 미지정 시 기존 동작 유지 (REQ-VERD-3 backward-compat, Must oracle)

Given configured providers 가 `[claude, gemini, codex]` 이다.
And `claude` 와 `gemini` 가 `VERDICT: PASS` 를 반환하고 `codex` 가 정상 응답으로 `VERDICT: REVISE` 를 반환한다.
And `autopus.yaml` 에 `exclude_failed_from_denom` 키가 없다.
When `runSpecReviewLoop`가 결과를 머지한다.
Then `MergeVerdictsWithDenomMode` 호출 시 `excludeFailed=false, failedCount=0` 으로 위임된다.
And final verdict 는 `REVISE` 이다 (1 REVISE 가 있으면 reviseCount > 0 으로 REVISE).
And review.md 의 verdict 라인은 정확히 `**Verdict**: REVISE` 이다 (degraded 라벨 없음, 모든 provider가 응답함).

### AC-VERD-EMPTY: exclude_failed_from_denom=true + 모든 provider 실패 → REVISE + 0/M 라벨 (REQ-VERD-3 edge, Must oracle)

Given configured providers 가 `[claude, gemini, codex]` 이다.
And 3개 모두 timeout 한다.
And `exclude_failed_from_denom: true` 이다.
When `runSpecReviewLoop`가 결과를 머지한다.
Then `denom = 0` 이며 `MergeVerdictsWithDenomMode` 가 `VerdictRevise` 를 반환한다.
And review.md 의 verdict 라인은 정확히 `**Verdict**: REVISE (degraded — 0/3 providers responded)` 이다.
