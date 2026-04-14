# SPEC-OPHARN-001 리서치

## 기존 코드 분석

### 1. OpenCode adapter 구조

OpenCode 어댑터는 이미 독립 패키지로 존재한다.

- `pkg/adapter/opencode/opencode.go`
- `pkg/adapter/opencode/opencode_files.go`
- `pkg/adapter/opencode/opencode_skills.go`
- `pkg/adapter/opencode/opencode_config.go`

즉 “OpenCode 지원이 없다”가 아니라 “지원은 있으나 표면과 wiring이 얇다”가 정확한 진단이다.

### 2. Router surface는 실제로 얇다

`pkg/adapter/opencode/opencode_specs.go`의 `workflowSpecs`는 8개만 정의한다.

- `auto`
- `auto-plan`
- `auto-go`
- `auto-fix`
- `auto-review`
- `auto-sync`
- `auto-idea`
- `auto-canary`

생성된 실제 산출물도 동일하게 얇다.

- 루트 `.opencode/commands/auto.md:23-35`는 `plan/go/fix/review/sync/canary/idea`만 지원한다고 선언한다.
- `.agents/skills/auto/SKILL.md`도 얇은 라우터 역할만 하며 helper flows를 제공하지 않는다.

반면 `.claude/skills/auto/SKILL.md`에는 `setup`, `status`, `map`, `why`, `verify`, `secure`, `test`, `dev`, `doctor`가 모두 존재한다.

### 3. Router source 정합성이 약하다

`pkg/adapter/opencode/opencode_skills.go`의 `renderRouterSkill()`은 다음 경로를 읽는다.

- `codex/prompts/auto.md.tmpl`

즉 OpenCode router skill은 OpenCode 전용 canonical source가 아니라 Codex prompt를 재가공한 결과다. Lore context도 이를 이미 경고한다.

`auto lore context pkg/adapter/opencode` 결과:

- Constraint: 플랫폼별 native workflow surface를 유지하고 prompt와 skill의 역할을 섞지 않는다
- Rejected: Claude router 재사용에 추가 문자열 치환만 덧대는 접근

이번 SPEC은 이 제약을 직접 반영해야 한다.

### 4. Hook plugin은 생성되지만 기본 generate 경로에서 활성화되지 않는다

현재 구조는 다음과 같다.

- `pkg/adapter/opencode/opencode_plugin.go`
  - `.opencode/plugins/autopus-hooks.js`를 생성한다.
- `pkg/adapter/opencode/opencode_config.go`
  - `renderConfigDocument(extraPlugins []string)`는 `extraPlugins`가 있을 때만 `plugin` 배열을 갱신한다.
- `pkg/adapter/opencode/opencode_files.go`
  - `prepareConfigMapping()` 호출 시 `extraPlugins=nil`

따라서 일반 `Generate()` / `Update()` 경로에서는 plugin file은 생성되지만 `opencode.json`에 자동 등록되지 않는다.

실제 루트 산출물 `opencode.json`도 `instructions`만 있고 `plugin` 배열이 없다.

이것은 단순 문서 gap이 아니라 runtime wiring bug에 가깝다.

### 5. Validate는 활성화 상태를 보지 않는다

`pkg/adapter/opencode/opencode_lifecycle.go`는 현재 다음만 검사한다.

- `opencode.json` 존재 여부
- `.opencode/plugins/autopus-hooks.js` 파일 존재 여부

하지만 config registration 여부는 검사하지 않는다. 즉 file exists/pass, but not wired 상태를 놓친다.

### 6. OpenCode runtime surface는 config append 수준이다

`pkg/adapter/opencode/opencode_config.go`가 관리하는 항목은 사실상 아래 3개뿐이다.

- `$schema`
- `instructions`
- optional `plugin`

반면 Claude adapter는 `pkg/adapter/claude/claude_settings.go`에서 다음을 관리한다.

- hooks
- permissions
- statusLine

OpenCode에 같은 구조를 그대로 이식할 수는 없지만, 최소한 “지원되는 runtime surface는 무엇인지”를 명시적으로 관리해야 한다.

### 7. Status surface는 source asset은 있으나 OpenCode 경로에는 없음

`content/statusline.sh`는 존재하지만 파일 헤더부터 Claude Code 전용이다.

- `# 🐙 Autopus-ADK Statusline for Claude Code`

OpenCode adapter package에는 `statusline`, `statusLine`, `statusline.sh` 관련 로직이 없다. 따라서 이번 범위에서는 direct port가 아니라 “지원 여부 확인 + fallback/비지원 명시”가 맞다.

### 8. Strategic skill 3종은 OpenCode만의 결손이 아니라 canonical source gap이다

현재 canonical `content/skills/`는 40개 스킬을 가진다.

- `content/skills/` directory listing 확인

그러나 `.claude/skills/autopus/`에는 다음 3개가 추가로 있다.

- `product-discovery.md`
- `competitive-analysis.md`
- `metrics.md`

즉 이 3개는 OpenCode transform이 누락된 것이 아니라 source 자체가 `content/skills/`에 없다. 해결책은 두 가지다.

1. canonical source에 추가한다
2. parity report에서 `unsupported-by-source`로 분류한다

### 9. Existing transformer는 재사용 가치가 높다

`pkg/content/skill_transformer.go`와 `pkg/content/skill_transformer_replace.go`는 이미 `opencode`를 target platform으로 지원한다.

특히 `ReplacePlatformReferences()`는 다음을 수행한다.

- `.claude/*` 경로를 `.opencode/*`, `.agents/skills/*`로 치환
- `TeamCreate` → `task`
- `AskUserQuestion` → `question`
- `TaskCreate/TaskUpdate` → `todowrite`

따라서 이번 SPEC은 변환기를 버리기보다 “router canonical source + unsupported classification + stronger tests”를 추가하는 방향이 적절하다.

## 아키텍처 검증

`auto arch enforce` 실행 결과:

- 아키텍처 규칙 위반 없음

따라서 변경은 기존 계층을 유지하며 다음 범위 안에서 끝내는 것이 바람직하다.

- `pkg/adapter/opencode`
- `pkg/content`
- shared templates/content assets

## 설계 결정

### D1: P0의 최우선은 helper flow보다 plugin activation bug fix

이유:

- 현재 상태는 “기능이 부족해 보인다” 수준을 넘어서 “생성된 훅이 실제로 안 켜질 수 있다”는 실행 문제다.
- 수정 지점이 좁고 효과가 즉각적이다.

### D2: Router 확장은 canonical contract 추출 방식으로 진행

이유:

- `codex/prompts/auto.md.tmpl` 기반 얇은 라우터는 이미 한계가 드러났다.
- Lore constraint가 prompt/skill 혼합을 금지한다.
- shared contract가 있어야 OpenCode command와 skill이 서로 다른 설명을 하지 않는다.

### D3: Unsupported classification을 parity gate의 1급 개념으로 둔다

이유:

- OpenCode가 지원하지 않는 표면과 아직 구현하지 않은 표면을 섞어버리면 다시 “부실해 보이는” 상태로 돌아간다.
- 유지보수자는 구현 대상과 설계상 제외 대상을 즉시 구분할 수 있어야 한다.

### D4: statusline은 direct port가 아니라 fallback/비지원 명시가 우선

이유:

- 현재 source asset은 Claude Code 전용이다.
- OpenCode가 동일한 extension point를 제공하는지 코드만으로는 확인되지 않았다.
- 이번 범위에서 무리한 복제는 거짓 parity를 만든다.

## 참고 파일

- `pkg/adapter/opencode/opencode_specs.go`
- `pkg/adapter/opencode/opencode_skills.go`
- `pkg/adapter/opencode/opencode_config.go`
- `pkg/adapter/opencode/opencode_plugin.go`
- `pkg/adapter/opencode/opencode_lifecycle.go`
- `.claude/skills/auto/SKILL.md`
- `content/skills/`
- `content/statusline.sh`
