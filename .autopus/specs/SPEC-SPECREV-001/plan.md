# SPEC-SPECREV-001 구현 계획

## 태스크 목록

### Slice 1 — Adaptive context_max_lines (REQ-CTX-1 ~ REQ-CTX-4)

- [ ] T1: `pkg/spec/[NEW] context_limit.go` 작성
  - `AdaptiveContextLimit(citedFileCount, ceiling int) int` 구현
  - 매핑: `0~2 → 500`, `3~5 → 1500`, `6+ → 3000`
  - `ceiling > 0` 일 때 `min(mapped, ceiling)` 적용
  - 파일 길이 100줄 이하

- [ ] T2: `pkg/spec/[NEW] context_limit_test.go` 작성
  - 인용 파일 수 0/1/2/3/5/6/10 케이스의 한도 결정 테스트
  - ceiling이 매핑 결과보다 작을 때, 클 때, 음수일 때
  - 회귀 방지: 인용 파일 0개 → 500 유지

- [ ] T3: `pkg/spec/metadata.go`에 `[NEW] ParseReviewContextOverride(content string) (value int, present bool, err error)` 추가
  - frontmatter `review_context_lines: <int>` 파싱
  - `value <= 0` 또는 `value > 10000` → `err` 반환 (REQ-CTX-3)
  - 미존재 시 `present=false`

- [ ] T4: `pkg/spec/metadata_test.go`에 REQ-CTX-2/REQ-CTX-3 케이스 추가
  - frontmatter 정상 값
  - frontmatter 미존재
  - frontmatter 값이 음수 / 0 / 10001
  - frontmatter 값이 비정수 문자열

- [ ] T5: `internal/cli/spec_review.go`에서 한도 결정 통합
  - `gate.AutoCollectContext` 분기 직전에 인용 파일 수 산출 (`spec.[NEW] CountSpecContextTargets(specDir)` 도입 또는 기존 `extractSpecContextTargets` 노출)
  - frontmatter override 파싱
  - ceiling은 `gate.ContextMaxLines > 0` 일 때 적용
  - 최종 한도와 인용 파일 수, override 적용 여부를 stderr 한 줄로 기록 (`SPEC review context: cited=<n> applied=<lines> override=<flag>`)

- [ ] T6: `internal/cli/[NEW] spec_review_context_test.go` 작성
  - AC-CTX-1, AC-CTX-2, AC-CTX-3, AC-CTX-OVR-INVALID, AC-CTX-CEIL 시나리오를 fake spec dir 기반으로 검증

### Slice 2 — Provider Health labeling + denom mode (REQ-VERD-1 ~ REQ-VERD-4)

- [ ] T7: `pkg/spec/types.go`에 `[NEW] type ProviderStatus struct { Provider, Status, Note string }` 추가, `ReviewResult`에 `ProviderStatuses []ProviderStatus` 필드 추가
  - JSON/YAML 직렬화 영향 검토 (현재 `ReviewResult`는 직접 직렬화되지 않음, 본 필드도 `omitempty` 의도로 추가)
  - 기존 호출자가 깨지지 않도록 zero-value 호환 보장

- [ ] T8: `pkg/spec/[NEW] provider_health.go` 작성
  - `ClassifyProviderStatuses(responses []orchestra.ProviderResponse) []ProviderStatus` 또는 등가 분류 함수
  - `ShouldLabelDegraded(statuses []ProviderStatus, totalConfigured int) bool` — failure 비율 ≥ 0.5 검사
  - `RenderProviderHealthSection(statuses []ProviderStatus, totalConfigured int) string`
  - 파일 길이 200줄 이하

- [ ] T9: `pkg/spec/[NEW] provider_health_test.go` 작성
  - AC-VERD-1, AC-VERD-2, AC-VERD-3, AC-VERD-4 시나리오 단위 테스트
  - boundary: 정확히 50% failure (예: 2/4)는 degraded
  - 49.9% failure (예: 1/3 ≈ 33%)는 not degraded

- [ ] T10: `pkg/spec/merge.go`에 `[NEW] MergeVerdictsWithDenomMode(results, threshold, totalProviders, excludeFailed bool, failedCount int) ReviewVerdict` 추가
  - `excludeFailed=true` 일 때 `denom = totalProviders - failedCount`
  - `denom <= 0` 일 때 (모든 provider 실패) → `VerdictRevise` 반환 (REJECT는 별도 처리)
  - 기존 `MergeVerdicts`는 새 함수에 `excludeFailed=false, failedCount=0`로 위임

- [ ] T11: `pkg/spec/merge_test.go`에 새 함수 케이스 추가
  - `exclude=false` 일 때 기존 동작 보존 (regression guard)
  - AC-VERD-3: 1 PASS / 2 timeout, exclude=true → VerdictPass
  - boundary: 0 살아남음 → VerdictRevise

- [ ] T12: `pkg/spec/review_persist.go`의 `formatReviewMd` 수정
  - `r.ProviderStatuses` 가 비어 있지 않으면 헤더 직후에 `## Provider Health` 섹션 렌더
  - degraded 조건 만족 시 verdict 라인에 ` (degraded — N/M providers responded)` 부착
  - REQ-VERD-4: `excludeFailed` 모드로 PASS 가 결정된 경우 항상 degraded 라벨 부착 (호출 측이 `ProviderStatuses`에 failure를 채워 보냈다면 자동 충족)
  - 파일 길이 150줄 이하 유지

- [ ] T13: `pkg/spec/review_persist_test.go` (없으면 신규) 에 verdict 라벨 / Provider Health 섹션 렌더링 테스트 추가

- [ ] T14: `internal/cli/spec_review_loop.go`의 `runSpecReviewLoop`에서 orchestra 응답 → `[]ProviderStatus` 변환 후 `merged.ProviderStatuses` 채우기
  - `result.Responses` 외에 timeout/error 메타데이터를 노출하는 orchestra 결과 형식 확인 후 매핑 (필요 시 `pkg/orchestra` 응답 구조 조사 후 plan T14a로 분기)
  - configured provider 수 (`len(p.providers)`) 와 실제 응답 provider 수의 차이를 timeout으로 분류

- [ ] T15: `internal/cli/spec_review_loop.go` 에서 `MergeVerdicts` 호출을 `MergeVerdictsWithDenomMode`로 교체, `gate.ExcludeFailedFromDenom` 전달

- [ ] T16: `pkg/config/schema_spec.go`에 `ExcludeFailedFromDenom bool` 필드 추가 (yaml: `exclude_failed_from_denom,omitempty`)

### Slice 3 — Documentation & 회귀

- [ ] T17: `CHANGELOG.md` (`autopus-adk/CHANGELOG.md`) 에 SPEC-SPECREV-001 항목 추가
- [ ] T18: 기존 spec review 통합 테스트가 새 한도 / Provider Health 출력으로 깨지지 않는지 회귀 확인. `TestSpecReview*` 계열 모두 실행
- [ ] T19: `auto check --lore` / 빌드가 새 yaml 키, 신규 파일에 대해 통과하는지 확인

## 구현 전략

### Slice 1 (REQ-CTX-*) 우선

- 인용 파일 수 산출은 `pkg/spec/context_collect.go`의 기존 `extractSpecContextTargets`/`resolveSpecTargetPath`를 재사용한다. 새 함수 `CountSpecContextTargets(specDir, projectRoot string) int` 를 같은 파일에 노출하거나, `context_limit.go`에서 호출하도록 한다.
- frontmatter override는 `pkg/spec/metadata.go`의 `parseSpecFrontmatter`가 이미 모든 키-값을 흡수하므로, 별도 헬퍼만 추가하면 된다.
- ceiling 적용은 `runSpecReview`에서 한도 결정의 마지막 단계에 위치한다.

### Slice 2 (REQ-VERD-*) 분리 작업

- 파일은 `provider_health.go`/`provider_health_test.go`로 분리하여 `review_persist.go` 300줄 한계를 침해하지 않는다.
- `MergeVerdicts`의 시그니처는 그대로 유지하고 신규 함수로 위임하여 outside caller breakage를 막는다.
- review.md 출력 포맷은 기존 `## Findings`, `## Provider Responses` 사이의 위치를 침해하지 않도록 헤더 직후 (Verdict 라인과 Findings 사이)에 삽입한다.

### Feature Completion Scope

이 SPEC은 단일 SPEC로 (C)+(D) 슬라이스를 닫는다.

- Slice 1과 Slice 2는 서로 다른 코드 경로(`context_collect.go` vs `merge.go` + `review_persist.go`)를 다루지만, 동일 트리거(이슈 #55의 multi-provider review 안정화)와 동일한 검증 흐름(`spec review` 명령 한 번으로 둘 다 관측)을 공유한다.
- Out of scope으로 분리한 (A)/(B) 즉시 패치는 본 SPEC와 무관하게 별도 PR로 진행되며, 본 SPEC의 acceptance를 막지 않는다.
- 외부 알림 채널과 다른 orchestra 명령 확장은 후속 SPEC을 별도로 발행한다 (현재 placeholder 없음, 필요할 때 신규 ID 부여).

### 검증 흐름

- 단위 테스트: T2, T4, T6, T9, T11, T13
- 통합 테스트: T6, T18 (`TestSpecReview*`)
- 수동 검증: 실제 `autopus spec review SPEC-WORKSPACE-OPS-007` 류 명령으로 review.md 의 Provider Health 섹션과 verdict 라벨을 한 번씩 눈으로 확인 (Slice 2 완료 후)
