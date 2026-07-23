# SPEC-FABLE5-001 Acceptance: Claude Fable 5 및 세션 effort 지원

## Test Scenarios

### S1: Fable model literal 허용
Priority: Must
Given generated workflow model trust boundary가 활성화되어 있다
When `claude-fable-5`, `fable`, `best`를 각각 검증한다
Then expected value `isSafeAgentModel`은 세 입력 모두 `true`이다
And 기존 `claude-opus-4-8`과 `claude-sonnet-5`도 `true`를 유지한다

### S2: Fable whitelist의 injection fail-closed
Priority: Must
Given generated workflow model 값이 JavaScript에 interpolation된다
When `claude-fable-5");evil((`와 `fable\nawait evil()`을 검증한다
Then expected value `isSafeAgentModel`은 두 입력 모두 `false`이다
And `ParseSchema`는 non-nil error를 반환한다

### S3: Fable full ID 가격
Priority: Must
Given canonical pricing table을 생성한다
When key `claude-fable-5`를 조회한다
Then expected value `InputPricePerMillion`은 정확히 `10.0`이다
And expected value `OutputPricePerMillion`은 정확히 `50.0`이다
And `fable`과 `best` key는 존재하지 않는다

### S4: Ultra Fable effort
Priority: Must
Given quality mode가 `ultra`이다
When model을 `claude-fable-5`, `fable`, `best`로 각각 ResolveEffort한다
Then expected JSON value `effort`는 모든 경우 `max`이다
And expected JSON value `source`는 모든 경우 `quality_mode`이다

### S5: 기존 quality 기본값 보존
Priority: Must
Given canonical quality model mapping을 생성한다
When Ultra와 Balanced role mapping을 조회한다
Then expected value Ultra executor는 `claude-opus-4-8`이다
And expected value Balanced executor는 `claude-sonnet-5`이다
And Fable full ID 또는 alias가 default mapping value로 나타나는 횟수는 `0`이다

### S6: Claude worker effort argv 전달
Priority: Must
Given Claude worker TaskConfig에 Fable model과 공식 CLI session effort가 있다
When `low`, `medium`, `high`, `xhigh`, `max`, `ultracode` 각각으로 BuildCommand를 실행한다
Then expected argv에는 `--model fable`이 순서쌍으로 한 번 존재한다
And expected argv에는 `--effort <requested>`가 순서쌍으로 정확히 한 번 존재한다

### S7: Claude worker invalid effort 생략
Priority: Must
Given Claude worker TaskConfig의 Effort가 `""`, `"auto"`, `"ultra"`, `"future-value"` 중 하나이다
When BuildCommand를 실행한다
Then expected argv에서 `--effort`의 발생 횟수는 `0`이다
And model, resume, MCP argv는 기존 값과 동일하다

### S8: Configured Claude split flag 교체
Priority: Must
Given Claude provider Args가 `--print --model fable --effort high --verbose`이고 PaneArgs도 effort를 가진다
When global `--effort ultracode` runtime override를 적용한다
Then expected Args subsequence는 `--model fable --effort ultracode`이다
And expected PaneArgs effort 값도 `ultracode`이다
And expected value model은 `fable`로 보존된다

### S9: Configured Claude effort upsert와 equals syntax
Priority: Must
Given 한 provider argv에는 effort flag가 없고 다른 argv에는 `--effort=medium`이 있다
When global `--effort max` runtime override를 적용한다
Then expected value 첫 argv의 마지막 두 항목은 `--effort`, `max`이다
And expected value 둘째 argv에는 `--effort=max`가 정확히 한 번 있다
And unrelated argv의 순서는 변하지 않는다

### S10: Fallback Claude effort
Priority: Must
Given config를 사용할 수 없어 hardcoded fallback provider registry를 사용한다
When runtime quality `balanced`와 explicit effort `xhigh`로 Claude provider를 만든다
Then expected Args에는 `--effort xhigh`가 정확히 한 번 있다
And expected PaneArgs에도 `--effort xhigh`가 정확히 한 번 있다
And expected model 값은 `opus`이다

### S11: Route-team 다섯 effort override
Priority: Must
Given Balanced route-team quality binding과 explicit model effort가 있다
When `low`, `medium`, `high`, `xhigh`, `max`와 empty effort를 각각 적용한다
Then expected JSON value 모든 agent-driven phase의 `effort`는 요청값과 동일하다
And expected JSON value review depth와 model은 원래 quality binding과 동일하다
And empty effort의 expected JSON value `selection_reason`은 `canonical_balanced`이며 기존 canonical Balanced bytes와 동일하다

### S12: Route-team ultracode 정규화
Priority: Must
Given route-team binding에 explicit `ultracode`가 전달된다
When binding receipt를 생성한다
Then expected JSON value 모든 agent-driven phase의 `effort`는 `xhigh`이다
And serialized `quality` JSON에 문자열 `ultracode`의 발생 횟수는 `0`이다
And workflow `safeEfforts`의 `ultracode` 허용 값은 `false`이다

### S13: Route-team invalid effort fail-closed
Priority: Must
Given Balanced route-team 요청에 explicit `future-value`가 전달된다
When binding receipt를 생성한다
Then expected JSON value `selection_reason`은 `binding_validation_failed`이다
And expected JSON value review `verify_votes`는 canonical Full Ultra 값 `3`이다
And serialized `quality` JSON에 `future-value`의 발생 횟수는 `0`이다
And invalid 검증은 Balanced quality 선택보다 먼저 평가된다

### S14: Persistence boundary 보존
Priority: Must
Given source와 generated workflow surface를 검사한다
When effort enum과 config serialization 코드를 검색한다
Then expected value model/API/workflow enum은 `low|medium|high|xhigh|max` 다섯 값이다
And `ultracode`는 Claude CLI pass-through, normalization, help, test, docs 이외의 persisted enum entry로 존재하지 않는다

### S15: 문서와 템플릿 버전 경계
Priority: Must
Given source skill과 생성 템플릿이 동기화되어 있다
When Fable 및 effort 지원 문구를 검색한다
Then expected stdout에는 `claude-fable-5`, `fable`, `best`, `2.1.170`, `2.1.203`, `2.1.210`이 모두 포함된다
And expected stdout에는 `ultracode`가 session-only이며 실제 model effort가 `xhigh`라는 설명이 포함된다

### S16: Entitlement 독립 검증
Priority: Must
Given 로컬 Claude Code가 `2.1.198`이고 Fable entitlement는 보장되지 않는다
When deterministic verification suite를 실행한다
Then expected exit code는 focused tests, race, vet, build, strict SPEC에서 모두 `0`이다
And live Fable API call과 live `--effort ultracode` smoke의 expected status는 `N/A: local version below 2.1.203 or entitlement-dependent`이다

## Edge Cases

### Edge Case 1: Dynamic `best` alias의 비용
Priority: Must
Given `best`가 entitlement에 따라 Fable 또는 최신 Opus로 해석된다
When cost table을 조회한다
Then expected value `best` pricing entry는 absent이다
And 호출자가 resolved full model ID를 알기 전에는 deterministic cost를 생성하지 않는다

### Edge Case 2: Existing Claude max migration
Priority: Must
Given 기존 exact historical default migration이 `opus/max`를 `opus/high`로 보정한다
When 사용자가 같은 invocation에서 global `--effort max`를 명시한다
Then expected runtime provider argv의 최종 effort 값은 `max`이다
And persisted config migration 정책은 이번 변경으로 수정되지 않는다

### Edge Case 3: Missing PaneArgs
Priority: Must
Given configured Claude provider의 PaneArgs가 nil이고 Args만 존재한다
When explicit global effort를 적용한다
Then expected Args effort는 요청값이다
And expected PaneArgs는 `--effort <requested>` 두 항목으로 생성된다

## Oracle Acceptance Notes

- Must 시나리오는 file exists, heading, exit code, non-empty output만으로 닫지 않는다. 각 시나리오의 expected value, expected JSON, expected stdout, 정확한 발생 횟수 또는 명시적 N/A tolerance를 oracle로 사용한다.
- argv oracle은 인접한 flag/value 순서쌍과 occurrence count를 함께 검사해 단순 `Contains` 오탐을 피한다.
- map 기본값 oracle은 Fable 추가와 default adoption을 구분한다.
- live entitlement와 구버전 runtime은 deterministic gate가 아니며 S16의 명시적 N/A tolerance로만 기록한다.

## Definition of Done

- [x] S1-S16과 Edge Case 1-3 모두 통과
- [x] 기존 Opus/Sonnet/Haiku와 CC21 effort 테스트 회귀 없음
- [x] focused package coverage가 변경된 branch 기준 85% 이상
- [x] source code 파일이 300줄 이하
- [x] 코드 리뷰와 보안 리뷰 finding 전부 resolved
- [x] Completion Debt `none`

## Execution Evidence (2026-07-23)

| Gate | Result | Evidence |
|------|--------|----------|
| Focused packages | PASS | `go test ./pkg/workflow ./pkg/cost ./pkg/worker/adapter ./internal/cli ./pkg/content -count=1` |
| Race | PASS | `pkg/workflow`, `pkg/cost`, `pkg/worker/adapter` 전체 및 Fable/Claude override/binding `internal/cli` targeted race |
| Coverage | PASS | workflow 89.2%, cost 100%, worker adapter 92.9%, 신규 CLI helper 93.8% 이상 |
| Static/build | PASS | `go vet ./...`, `go build ./...`, `git diff --check`, `gofmt -l` |
| Architecture/SPEC | PASS | `auto arch enforce`, `auto spec validate .autopus/specs/SPEC-FABLE5-001 --strict` |
| Templates | PASS | source-first generation 후 재실행 checksum 불변 |
| Source size | PASS | 변경 Go 파일 최대 293줄 |
| Guardian | PASS | F1/F2 및 SEC-FABLE-001/002 수정 후 Reviewer·Security Auditor 재검증 PASS, Validator open finding 없음 |
| Live Fable/ultracode | N/A | 로컬 Claude Code `2.1.198`; `ultracode` 최소 버전 미달 및 Fable entitlement 의존 |

전체 `go test ./...` 병렬 실행에서는 변경 범위 밖 POSIX process/timing 테스트들이 고부하 환경의 2초/8초 경계를 초과했다. 변경된 다섯 package의 독립 실행, race, vet, build는 모두 PASS했으며 해당 full-suite flake는 이 SPEC의 Completion Debt가 아니다.
