# SPEC-SPECREV-001: Adaptive Review Context + Provider Health Verdict Labeling

**Status**: draft
**Created**: 2026-05-04
**Domain**: SPECREV

## 목적

Multi-provider spec review의 두 가지 신호 손실을 후속 패치로 보완한다.

1. **컨텍스트 손실**: `pkg/config/defaults.go:50`의 `ContextMaxLines: 500` 고정값은 6개 이상 파일·700~1000줄 이상 코드를 인용하는 module-scope SPEC에서 절반 이하만 provider 프롬프트에 도달한다. provider가 timeout 없이 응답해도 검증 근거가 부실해진다.
2. **인프라 장애와 콘텐츠 결함 신호 혼동**: `pkg/spec/merge.go:113`의 `denom := float64(totalProviders)`는 timeout으로 dropped된 provider도 분모에 포함시켜 1 PASS / 2 timeout 시 0.33 < 0.67 threshold 미달로 REVISE를 산출한다. 사용자는 review.md 만으로 "SPEC을 고쳐야 하나?" vs "타임아웃을 늘려야 하나?"를 구분하기 어렵다.

이 SPEC은 (C) Adaptive context_max_lines와 (D) Provider Health 라벨링 + 선택적 denom 보정을 함께 다룬다. (A) effort 완화, (B) per-provider timeout 480, 외부 알림 채널, 다른 orchestra 명령(brainstorm, review, secure)으로의 확장은 본 SPEC의 범위 밖이다.

## 요구사항

### REQ-CTX-1 (Event-driven / Priority: Must)

WHEN spec review가 SPEC의 인용 코드 컨텍스트를 수집하면 (`pkg/spec/context_collect.go:90`의 `CollectContextForSpec` 호출 경로), THE SYSTEM SHALL `ContextMaxLines` 한도를 SPEC가 직접 인용하는 파일 수 (`extractSpecContextTargets`로 산출된 unique resolved 파일 개수) 에 비례하여 동적으로 산출한다.

매핑은 다음과 같다:

- 인용 파일 수 0~2 → 한도 500
- 인용 파일 수 3~5 → 한도 1500
- 인용 파일 수 6 이상 → 한도 3000 (hard cap)

관측 지점: `internal/cli/spec_review.go:109`의 `CollectContextForSpec` 호출 시 적용된 한도와 인용 파일 수가 stderr 또는 review.md `Provider Health` 섹션의 사이드 라인으로 노출된다.

### REQ-CTX-2 (Optional feature / Priority: Should)

WHERE SPEC frontmatter에 `review_context_lines: <int>` 키가 명시되면, THE SYSTEM SHALL 자동 매핑 결과보다 frontmatter 값을 우선 사용한다. frontmatter 값은 양의 정수여야 한다.

관측 지점: `pkg/spec/metadata.go`의 `parseSpecFrontmatter`가 새 키를 인식하고, `runSpecReview`가 적용된 한도를 stderr에 한 줄로 기록한다.

### REQ-CTX-3 (Unwanted behavior / Priority: Must)

IF frontmatter `review_context_lines` 값이 0 이하이거나 10000을 초과하면, THE SYSTEM SHALL 해당 override를 무시하고 자동 매핑 결과로 fallback하며 stderr에 reject 사유를 한 줄 기록한다.

관측 지점: spec review CLI의 stderr 라인 (`경고: review_context_lines 무시 (값=<n>, 사유=<reason>)`).

### REQ-CTX-4 (Optional feature / Priority: Should)

WHERE `autopus.yaml`의 `spec.review_gate.context_max_lines` 가 양의 정수로 명시되어 있으면, THE SYSTEM SHALL 자동 매핑·frontmatter override 결과 모두에 대해 그 값을 hard ceiling으로 적용한다 (자동 매핑 결과 ≤ ceiling, frontmatter override ≤ ceiling).

관측 지점: 적용된 최종 한도가 ceiling보다 작거나 같음을 단위 테스트로 확인한다.

### REQ-VERD-1 (Event-driven / Priority: Must)

WHEN multi-provider spec review가 종료되어 `pkg/spec/review_persist.go`의 `formatReviewMd`가 review.md를 빌드하면, THE SYSTEM SHALL 헤더 영역에 별도 `## Provider Health` 섹션을 추가하고 각 provider의 status (`success` / `timeout` / `error`), 응답 시간(초, 가능한 경우), 비율 라인을 기록한다.

관측 지점: review.md 본문 내 `## Provider Health` heading과 그 직후 표 또는 bullet list. 표는 최소한 `| Provider | Status | Note |` 컬럼을 갖는다.

### REQ-VERD-2 (Event-driven / Priority: Must)

WHILE provider infra failure (timeout 또는 exit-error) 비율이 전체 configured provider 수의 50% 이상이면, THE SYSTEM SHALL `formatReviewMd`가 작성하는 verdict 라인에 `(degraded — N/M providers responded)` 라벨을 부착한다 (N=success provider 수, M=configured provider 수).

50% 임계값은 `>= 0.5` 로 inclusive 처리하여 "정확히 절반 실패" (예: 2/4)도 degraded로 라벨링한다.

관측 지점: review.md `**Verdict**: <verdict> (degraded — N/M providers responded)` 라인.

### REQ-VERD-3 (Optional feature / Priority: Should)

WHERE `autopus.yaml`의 `spec.review_gate.exclude_failed_from_denom: true` 가 명시되면, THE SYSTEM SHALL `MergeVerdicts`의 denom 계산에서 infra failure로 dropped된 provider를 제외하고 살아남은 provider 수를 분모로 사용한다.

기본값은 `false`이며, 미설정 시 기존 `denom = totalProviders` 동작을 유지한다 (backward-compat).

옵션이 `true`이고 살아남은 provider가 0인 경우, THE SYSTEM SHALL VerdictReject가 아닌 한 VerdictRevise를 반환하고 review.md에 `(degraded — 0/M providers responded)` 라벨을 부착한다.

관측 지점: 단위 테스트와 review.md verdict 라인.

### REQ-VERD-4 (Unwanted behavior / Priority: Must)

IF `exclude_failed_from_denom: true` 모드에서 살아남은 provider만으로 VerdictPass가 결정되면, THE SYSTEM SHALL verdict 라인에 항상 degraded 라벨을 함께 부착한다 (사용자가 명시적으로 인프라 장애를 "성공한 provider만으로 판정" 하기로 선택한 사실을 review.md에서 확인할 수 있어야 함).

관측 지점: review.md verdict 라인. degraded 라벨 누락 시 단위 테스트 FAIL.

## 생성/수정 파일 상세

### 수정

- `pkg/config/schema_spec.go` — `ReviewGateConf`에 `ExcludeFailedFromDenom bool` 필드 추가 (yaml: `exclude_failed_from_denom,omitempty`).
- `pkg/spec/context_collect.go` — `CollectContextForSpec` 시그니처는 유지하되 호출자가 사전에 한도를 산출하도록 보조 함수 `[NEW] AdaptiveContextLimit(citedFileCount int, ceiling int) int` 를 같은 패키지에 추가.
- `pkg/spec/metadata.go` — `parseSpecFrontmatter` 결과에 `review_context_lines` 키를 노출하기 위해 `[NEW] ParseReviewContextOverride(content string) (int, bool, error)` 헬퍼 추가.
- `pkg/spec/types.go` — `ReviewResult`에 `[NEW] ProviderStatuses []ProviderStatus` 필드 추가 (review.md Provider Health 섹션용). `[NEW] type ProviderStatus struct { Provider string; Status string; Note string }`.
- `pkg/spec/merge.go` — `MergeVerdicts` 시그니처는 유지하고, `[NEW] MergeVerdictsWithDenomMode(results, threshold, totalProviders, excludeFailed bool, failedCount int) ReviewVerdict` 추가. 기존 `MergeVerdicts`는 `excludeFailed=false`로 위임하여 backward-compat 보장.
- `pkg/spec/review_persist.go` — `formatReviewMd`가 `ReviewResult.ProviderStatuses`를 읽어 Provider Health 섹션을 추가하고, degraded 라벨이 필요하면 verdict 라인에 부착.
- `internal/cli/spec_review.go` — `runSpecReview`에서 인용 파일 수 계산, frontmatter override 파싱, ceiling 적용, ProviderStatuses 수집 오케스트레이션. spec.CollectContextForSpec 호출 직전에 한도 산출 로직을 삽입.

### 신규

- `pkg/spec/[NEW] context_limit.go` — `AdaptiveContextLimit` 와 ceiling 적용 함수.
- `pkg/spec/[NEW] context_limit_test.go` — REQ-CTX-1 ~ REQ-CTX-4 단위 테스트.
- `pkg/spec/[NEW] provider_health.go` — Provider Health 표 렌더링 + degraded 라벨 결정 로직.
- `pkg/spec/[NEW] provider_health_test.go` — REQ-VERD-1 ~ REQ-VERD-4 단위 테스트.
- `internal/cli/[NEW] spec_review_context_test.go` — frontmatter override, ceiling, provider failure 시나리오 통합 테스트.

## Related SPECs

None. 이 SPEC은 단일 SPEC로 GitHub 이슈 Insajin/autopus-adk#55 의 (C)+(D) 슬라이스를 닫는다. (A)/(B) 즉시 패치, 외부 알림 채널, 다른 orchestra 명령으로의 확장은 본 SPEC의 `Out of Scope` 이며 후속 작업이 필요할 때 별도 SPEC ID로 발행한다.

## Out of Scope

- (A) effort 완화 (max → high) — 별도 패치
- (B) claude per-provider timeout 480 — 별도 패치
- (D)의 추가 알림 (Slack / PagerDuty 등 외부 채널)
- 다른 orchestra 명령 (brainstorm, review, secure) 에 동일한 Provider Health 처리 적용 — 후속 SPEC
