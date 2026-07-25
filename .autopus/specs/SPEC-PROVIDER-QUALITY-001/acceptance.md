# SPEC-PROVIDER-QUALITY-001 Acceptance: Provider별 Quality Mode

## Test Scenarios

### S1: Legacy default 완전 호환
Priority: Must
Given `quality.default: ultra`이고 `quality.providers`가 없는 legacy config가 있다
When `EffectiveMode("claude")`, `EffectiveMode("codex")`, `ForProvider("codex").Default`를 계산한다
Then 세 expected value는 모두 `ultra`이다
And 원본 `QualityConf.Default`는 `ultra`, Providers length는 `0`을 유지한다

### S2: Persisted provider override와 safety fallback
Priority: Must
Given default가 `balanced`이고 providers가 `claude: ultra`, `codex: balanced`이며 별도 empty/invalid-default fixture가 있다
When 두 provider effective mode와 empty/invalid-default fixture의 effective mode를 계산한다
Then Claude는 `ultra`, Codex는 `balanced`이다
And empty config와 invalid-default fixture의 Claude/Codex expected value는 모두 `balanced`이다

### S3: Canonical provider와 preset validation
Priority: Must
Given configured presets가 `ultra`, `balanced`이다
And YAML metacharacter나 shell metacharacter를 포함한 custom preset 이름 fixture가 있다
When CLI provider `claude-code`, raw key `claude-code`, provider `gemini`, preset `turbo`, unsafe custom preset 이름을 각각 검증한다
Then CLI `claude-code`의 canonical key는 `claude`이다
And raw alias, `gemini`, `turbo`, unsafe custom preset 이름은 각각 non-nil validation error이다
And 실패 후 config bytes와 platform updater call count는 각각 원본과 `0`이다

### S4: Per-run global override 최우선
Priority: Must
Given default가 `balanced`이고 providers가 `claude: ultra`, `codex: ultra`이다
When 명시적 global `--quality balanced`로 effective runtime config를 만든다
Then Claude와 Codex runtime effective mode는 모두 `balanced`이다
And persisted provider values는 둘 다 `ultra`로 남는다
And persisted config changed byte count는 `0`이다

### S5: Provider preset 저장
Priority: Must
Given providers map이 없는 raw config와 inline comment, env placeholder, future field가 있다
When `auto quality provider claude-code ultra`를 실행한다
Then parsed `quality.providers`는 정확히 `{"claude":"ultra"}`이다
And output은 `quality.providers.claude = ultra`를 포함한다
And platform updater call count는 `0`이다

### S6: Provider inherit 삭제
Priority: Must
Given providers가 `claude: ultra`, `codex: balanced`이다
When `auto quality provider claude inherit`를 실행한다
Then `claude` key는 존재하지 않고 `codex` 값은 `balanced`이다
And `EffectiveMode("claude")`는 persisted default 값이다
And output은 `quality.providers.claude = inherit`를 포함한다

### S7: Provider별 apply 범위
Priority: Must
Given configured platforms가 `claude-code`, `codex`, `opencode`이다
When `auto quality provider claude-code ultra --apply`와 Codex equivalent를 각각 실행한다
Then 첫 updater call list는 정확히 `["claude-code"]`이다
And 두 번째 updater call list는 정확히 `["codex"]`이다
And 각 `quality.applied_platforms` expected value는 `1`이다

### S8: Global apply 하위 호환
Priority: Must
Given configured platforms가 `claude-code`, `codex`, `opencode`이다
When `auto quality ultra --apply`를 실행한다
Then updater call list는 정확히 `["claude-code","codex","opencode"]`이다
And `quality.default`는 `ultra`이다
And `quality.applied_platforms` expected value는 `3`이다

### S9: Codex effective mode consumer
Priority: Must
Given default가 `balanced`, provider override가 `codex: ultra`, supervisor policy가 `quality`이다
When Codex root, orchestra, planner agent, executor agent profile을 계산한다
Then root tuple은 `gpt-5.6-sol|ultra`이다
And orchestra tuple은 `gpt-5.6-sol|max`이다
And planner tuple은 `gpt-5.6-sol|max`이다
And executor tuple은 `gpt-5.6-sol|xhigh`이다

### S10: Claude workflow effective mode consumer
Priority: Must
Given default가 `ultra`이고 provider override가 `claude: balanced`이다
When route-team binding을 provider persisted mode와 explicit global `--quality ultra`로 각각 dispatch한다
Then persisted-mode planning은 `claude-opus-5|medium`이다
And persisted-mode implementation은 `claude-sonnet-5|medium`이다
And explicit-run의 모든 6 agent phase model은 `claude-opus-5`이다
And explicit-run의 phase effort는 기존 Ultra oracle 값을 유지한다

### S11: Raw YAML 보존
Priority: Must
Given file mode `0640`인 YAML에 `${AUTOPUS_SECRET}`, inline comment, unknown `future_extension`이 있다
When provider override를 추가하고 다시 삭제한다
Then placeholder, comment, future field bytes는 원문과 동일하다
And materialized secret occurrence count는 `0`이다
And 최종 file mode는 `0640`이다

### S12: Atomic failure 원본 보존
Priority: Must
Given 원본 config bytes와 platform updater spy가 있다
When temp write, sync, close 또는 rename failure를 각각 주입한다
Then 각 실패의 config bytes는 원본과 동일하다
And temp artifact remaining count는 `0`이다
And platform updater call count는 `0`이다

### S13: Source/generated contract parity
Priority: Must
Given source docs와 generated Claude/Codex/Gemini templates를 생성한다
When provider YAML, precedence, CLI, apply scope를 비교한다
Then 모든 surface의 canonical keys는 `claude`, `codex`이다
And precedence text는 `run-global > provider > default > balanced`이다
And provider/global apply target count example은 각각 `1`, `all configured`이다
And 두 번째 generator changed file count는 `0`이다

### S14: Backward compatibility regression bundle
Priority: Must
Given provider map 없는 기존 fixtures와 Ultra/Balanced mapping tests가 있다
When config, quality CLI/apply, Codex adapter/profile, workflow, template suites를 실행한다
Then legacy default behavior mismatch count는 `0`이다
And existing model/effort mapping mismatch count는 `0`이다
And focused provider-quality assertion failure count는 `0`이다

## Oracle Acceptance Notes

- Must scenarios는 파일 존재나 exit code만으로 닫지 않고 exact mode, map, tuple, call list, byte count를 검사한다.
- `ForProvider`는 source `QualityConf`를 mutate하지 않는다.
- Provider apply와 global apply는 같은 updater를 사용할 수 있지만 target list oracle이 다르다.
- Raw preservation은 parsed semantic equality만으로 대체하지 않고 placeholder/comment/future bytes와 file mode를 검사한다.

## Definition of Done

- [x] S1-S14 모두 통과
- [x] changed-scope coverage 85% 이상
- [x] legacy/global quality regressions `0`
- [x] generator second-run diff `0`
- [x] review/security/validator blocking finding `0`
- [x] Completion Debt `none`
