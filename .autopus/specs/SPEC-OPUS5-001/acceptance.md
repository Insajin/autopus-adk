# SPEC-OPUS5-001 Acceptance: Claude Opus 5 기본 경로 승격

## Test Scenarios

### S1: Closed whitelist와 injection 경계
Priority: Must
Given generated workflow model trust boundary가 활성화되어 있다
When `claude-opus-5`, `claude-opus-4-8`, `claude-opus-5");evil((`를 각각 검증한다
Then expected safe verdict는 순서대로 `true`, `true`, `false`이다
And breakout 입력의 schema parse result는 non-nil error이다

### S2: Opus 5 deterministic pricing
Priority: Must
Given canonical pricing table을 생성한다
When `claude-opus-5`와 `opus` key를 조회한다
Then Opus 5의 expected input price는 정확히 `5.0`이다
And expected output price는 정확히 `25.0`이다
And dynamic `opus` key는 존재하지 않는다

### S3: Opus 5 Ultra effort
Priority: Must
Given quality mode가 `ultra`이다
When model `claude-opus-5`와 normalized slug `opus-5`를 각각 resolve한다
Then 두 result의 expected effort는 `max`이다
And expected source는 `quality_mode`이다

### S4: Ultra와 Balanced model map
Priority: Must
Given canonical quality model map을 생성한다
When `ultra`와 `balanced`를 조회한다
Then Ultra의 9개 managed agent model은 모두 `claude-opus-5`이다
And Balanced의 `planner`, `architect`는 `claude-opus-5`이다
And Balanced의 나머지 7개 execution/validation agent는 `claude-sonnet-5`이다

### S5: Fable과 Sonnet 정책 보존
Priority: Must
Given Opus 5 default 승격이 적용되어 있다
When Fable catalog와 Balanced execution mapping을 조회한다
Then `claude-fable-5` 가격은 input `10.0`, output `50.0`을 유지한다
And Fable은 default model map에 포함되지 않는다
And Balanced `executor` model은 `claude-sonnet-5`이다

### S6: Premium, complex, canonical workflow routing
Priority: Must
Given default config, worker routing config, route-team schema를 로드한다
When premium tier, Claude complex route, planning phase model을 읽는다
Then 세 expected value는 모두 `claude-opus-5`이다
And simple/medium Claude route는 각각 `claude-sonnet-5`이다

### S7: Claude Code alias/version/provider matrix
Priority: Must
Given 문서 기준일이 2026-07-25이다
When `opus` alias compatibility table을 읽는다
Then v2.1.219+의 Anthropic API, Claude Platform on AWS, Bedrock, Google Cloud 값은 각각 `Opus 5`이다
And Microsoft Foundry 값은 `Opus 4.6`이다
And v2.1.218의 직전 지원 alias 값은 `Opus 4.8`이다

### S8: Migration과 thinking/effort 계약
Priority: Must
Given Opus 4.8에서 Opus 5로 migration guidance를 읽는다
When runtime defaults와 invalid combination을 비교한다
Then migration type은 `drop-in`이다
And adaptive thinking default는 `enabled`, default effort는 `high`이다
And allowed effort set은 `low|medium|high|xhigh|max`이다
And thinking-disabled 상태의 `xhigh|max`는 HTTP `400`으로 문서화되며 ADK argv에는 thinking disable flag가 없다

### S9: Context와 output 계약
Priority: Must
Given canonical Opus 5 model facts를 읽는다
When context window와 max output을 조회한다
Then expected context window는 `1,000,000` tokens이다
And expected max output은 `128,000` tokens이다
And fixed model ID는 `claude-opus-5`이다

### S10: Source/generated semantic parity
Priority: Must
Given source skill과 generated platform template를 함께 렌더링한다
When Opus tier 표와 default examples를 비교한다
Then 모든 surface의 Opus fixed ID는 `claude-opus-5`이다
And Ultra effort는 `max`, Balanced execution model은 `claude-sonnet-5`이다
And 두 번째 generation의 expected changed file count는 `0`이다

### S11: Opus 4.8 backward compatibility와 historical boundary
Priority: Must
Given current defaults와 historical evidence를 분리해 검사한다
When Opus 4.8 catalog, cybersecurity fallback 문서, protected fixture diff를 조회한다
Then `claude-opus-4-8` whitelist verdict는 `true`이다
And pricing은 input `5.0`, output `25.0`이다
And cybersecurity fallback model은 `Opus 4.8`이다
And historical telemetry·pinned argv fixture·`SPEC-FABLE5-001` changed file count는 `0`이다

### S12: Route-aware v2.1.219 live gate
Priority: Must
Given 로컬 `claude --version` output이 `2.1.218 (Claude Code)`이다
When Opus 5 live smoke eligibility와 route별 workflow doctor version gate를 평가한다
Then expected eligibility는 `false`이다
And expected live smoke status는 `N/A`이다
And verification command set의 paid Opus 5 call count는 `0`이다
And 모든 required primitive가 available일 때 `route_a` doctor의 v2.1.218 expected result는 `minimum_version=2.1.154`, `version_ok=true`, `overall=pass`이다
And 동일 조건의 `route_team` doctor v2.1.218 expected result는 `minimum_version=2.1.219`, `version_ok=false`, `overall=fail`이다
And 동일 조건의 `route_team` doctor v2.1.219 expected result는 `version_ok=true`, `overall=pass`이다

### S13: Deterministic verification bundle
Priority: Must
Given S1-S12의 value assertions가 테스트로 구현되어 있다
When focused tests, race, vet, build, strict SPEC, architecture, diff hygiene를 실행한다
Then focused Opus 5 assertion failure count는 `0`이다
And source/generated parity mismatch count는 `0`이다
And unresolved Guardian blocking finding count는 `0`이다

## Oracle Acceptance Notes

- Must acceptance는 파일 존재나 exit code만으로 닫지 않고 exact model ID, 가격, effort, count, provider matrix 값을 검사한다.
- `claude-opus-4-8`의 전체 저장소 occurrence가 0일 필요는 없다. current default surface만 Opus 5로 바꾸고 protected historical surface는 diff count `0`이어야 한다.
- Live entitlement는 deterministic gate가 아니며 설치 버전이 v2.1.219 미만이면 S12의 명시적 `N/A`가 authoritative result이다.

## Definition of Done

- [x] S1-S13이 모두 충족됨
- [x] focused changed-scope coverage 85% 이상
- [x] Opus 4.8/Fable/Sonnet compatibility oracle 통과
- [x] template generation 결정성 통과
- [x] review/security/validator blocking finding `0`
- [x] Completion Debt `none`

## Execution Evidence (2026-07-25)

| Gate | Result | Evidence |
|------|--------|----------|
| Focused packages | PASS | Opus 5 관련 `internal/cli`, cost, config, worker/routing, workflow, content, Claude adapter |
| Race | PASS | 변경 패키지 전체와 병렬 timeout 두 건의 isolated 재실행 |
| Coverage | PASS | cost 100%, config 89.4%, routing 97.5%, workflow 89.3%, content 94.5%, Claude adapter 86.7%; effort 변경 함수 90.9% 이상 |
| Static/build | PASS | changed-package `go vet`, `go build ./...`, `git diff --check`, `gofmt -l` |
| Architecture/SPEC | PASS | `auto arch enforce`, strict SPEC validation |
| Templates | PASS | 두 번째 generation 전후 diff hash `f7e8b69900ae6c06b759fae6a66fe2b86d872a5623486521545972a0fd3ff20c` 동일 |
| Compatibility | PASS | Opus 4.8/Fable/Sonnet와 protected historical path diff 0 |
| Guardian | PASS | Discovery F1-F3 해결 후 Reviewer APPROVE, Security Auditor PASS, Validator PASS |
| Live Opus 5 | N/A | 로컬 Claude Code `2.1.218`; `route_team` 최소 버전 미달, 유료 호출 0회 |

전체 umbrella 병렬 실행에서는 변경 범위 밖 `pkg/worker/setup`의 2초 wall-clock
테스트와 기존 `pkg/content` hook timeout 두 건이 고부하 경계를 초과했다. 각
테스트의 isolated 재실행은 PASS했으며 Opus 5 변경 패키지와 최종 build는 PASS했다.
