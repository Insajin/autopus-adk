# SPEC-OMP-001 Acceptance: omp 플랫폼 최소 표면 지원

## Test Scenarios

### S1: 규칙 frontmatter passthrough — 있는 소스와 없는 소스 (Must, oracle)

Given `content/rules/lore-commit.md`는 frontmatter에 `name`, `description: Lore commit format rules for structured, traceable commit messages`, `category: workflow`를 선언한다.
And `content/rules/branding.md`는 frontmatter가 없고 첫 줄이 `# Autopus Branding`이다.
When omp 어댑터가 규칙을 방출한다.
Then `.omp/rules/autopus-lore-commit.md`의 첫 줄은 `---`이고 frontmatter는 `description: Lore commit format rules for structured, traceable commit messages`를 담는다.
And 그 frontmatter는 `category:`와 `name:`을 담지 않으며, `.omp/rules/autopus-branding.md`는 합성된 `description: Autopus Branding` 한 줄짜리 frontmatter를 얻고 본문 첫 줄은 여전히 `# Autopus Branding`이다.
And `.omp/rules/` 하위의 `autopus-` 접두사 14개 파일 각각은 frontmatter 이후 공백이 아닌 줄을 최소 1줄 갖고 `description` 또는 trigger 키를 가져 omp 세션에 도달하며, omp가 규칙 루트를 비재귀로만 탐색하므로 `.omp/rules/` 아래에 하위 디렉터리가 만들어지지 않고 `.agents/rules/`에도 omp가 만든 파일이 하나도 없다.

### S2: 조건부 규칙 필드 passthrough (Must, oracle)

Given 픽스처 `conditional-demo.md`는 `description: demo`, `condition: tool:bash`, `scope: [tool:edit]`, `alwaysApply: false`, `interruptMode: prose-only`를 선언한다.
And 픽스처 `plain-demo.md`는 frontmatter를 선언하지 않는다.
When omp 규칙 변환기가 두 픽스처를 변환한다.
Then `conditional-demo` 출력 frontmatter는 `condition: tool:bash`를 담고 `scope` 시퀀스는 `tool:edit` 한 항목만 갖는다.
And `conditional-demo` 출력 frontmatter는 `alwaysApply: false`와 `interruptMode: prose-only`도 담고, `plain-demo` 출력은 합성된 `description: Plain Demo` 한 줄만 담으며 `condition`·`scope`·`alwaysApply` 키는 담지 않는다.

### S3: 에이전트 변환 — 도구 매핑, model, 본문 정규화 (Must, oracle)

Given `content/agents/executor.md`는 `tools: Read, Write, Edit, Grep, Glob, Bash, TodoWrite`와 `model: sonnet`을 선언한다.
And `content/agents/architect.md`는 `tools: Read, Grep, Glob, Bash, mcp__sequential-thinking__sequentialthinking`를 선언한다.
And `content/agents/spec-writer.md`는 `tools: Read, Grep, Glob, Bash, WebSearch, WebFetch`를 선언한다.
And `content/agents/annotator.md` 본문은 `` `.claude/skills/autopus/ax-annotation.md` ``를 참조하고 `content/agents/frontend-specialist.md` 본문은 `` `.claude/skills/autopus/frontend-verify.md` ``를 참조한다.
And 픽스처 `toolless-demo`는 `tools` 키를 선언하지 않는다.
When `TransformAgentForOMP`가 다섯 소스를 변환한다.
Then executor 출력의 `tools`는 정확히 `bash, edit, glob, grep, read, todo, write` 7개를 이 순서로 갖는다.
And architect 출력의 `tools`는 정확히 `bash, glob, grep, mcp__sequential-thinking__sequentialthinking, read` 5개를 이 순서로 갖는다.
And spec-writer 출력의 `tools`는 정확히 `bash, glob, grep, read, web_search` 5개이며 `web_search`는 한 번만 나타난다.
And `toolless-demo` 출력은 `tools:` 키를 담지 않는다.
And executor 출력 frontmatter는 `name: executor`, 소스와 같은 `description`, `model: sonnet`을 담는다. annotator 출력 본문은 정확히 `` `.agents/skills/ax-annotation/SKILL.md` ``를 담고 `.agents/skills/ax-annotation.md`도 `.claude/`도 담지 않는다.
And frontend-specialist 출력 본문은 정확히 `` `.agents/skills/frontend-verify/SKILL.md` ``를 담는다.
And 두 출력이 지시하는 경로는 REQ-013이 실제로 방출하는 파일 경로와 문자열이 같다.
And 다섯 출력 모두 `yield`와 `spawns`를 방출하지 않는다.

### S4: 소유권 경계와 manifest 범위 (Must, oracle)

Given 플랫폼으로 `opencode`와 `omp`를 나열하고 opencode가 `.agents/skills/auto/SKILL.md`를 만들었다.
And 사용자가 `.omp/rules/mine.md`와 `.agents/rules/my-own-rule.md`를 직접 만들었다.
When omp 어댑터가 `Generate` 후 `Clean`을 수행한다.
Then omp manifest는 `.omp/rules/autopus-` 접두사 14개, `.omp/agents/` 16개, `.omp/config.yml` 경로를 모두 담는다.
And omp manifest는 `.agents/skills/`, `.agents/commands/`, `.agents/plugins/`, `.agents/hooks.json` 경로를 하나도 담지 않으며 `Clean` 이후 `.agents/skills/auto/SKILL.md`의 바이트 내용은 `Clean` 이전과 같다.
And `Clean` 이후 사용자가 만든 `.omp/rules/mine.md`와 `.agents/rules/my-own-rule.md`가 그대로 남는다. `.omp/rules`가 prune 루트에 있어도 prune 대상은 이전 매니페스트에 기록된 경로뿐이므로, `.omp/rules/**`에서 ADK가 소유하는 것은 매니페스트에 기록된 `autopus-` 접두사 파일뿐이다.

### S5: `.claude/CLAUDE.md` 금지와 InstallHooks 계약 (Must, oracle)

Given 루트에 `AGENTS.md`가 있는 워크스페이스다.
When omp 어댑터의 `Generate`, `Update`, `InstallHooks`를 차례로 수행한다.
Then `SupportsHooks()`는 false를 반환하고 `InstallHooks`는 오류 없이 반환하며 어떤 파일도 만들거나 바꾸지 않는다.
And `.claude/CLAUDE.md` 경로에 파일이 만들어지지 않는다.
And omp manifest는 `.claude/`를 담는 경로를 하나도 갖지 않는다.
And 루트 `AGENTS.md`의 바이트 내용은 변하지 않는다.

### S6: 생성 표면 수량과 설정 값 (Must, oracle)

Given 플랫폼으로 `omp`만 나열한 빈 워크스페이스다.
When omp 어댑터가 `Generate`를 수행한다.
Then `.omp/rules/` 하위 `autopus-` 접두사 `.md` 파일 수는 정확히 14이고 접두사를 제거한 파일명 집합은 `content/rules/`와 같으며, `.omp/rules/` 아래에 하위 디렉터리는 만들어지지 않는다.
And `.omp/agents/` 하위 `.md` 파일 수는 정확히 16이고 파일명 집합은 `content/agents/`와 같다.
And `.omp/config.yml`은 YAML로 파싱되며 `skills.customDirectories`의 값이 정확히 `[".agents/skills"]`이고 관리 구간을 여닫는 마커 두 줄을 담는다.

### S7: 라이브 omp 세션 발견 (Must, oracle)

Given 임시 워크스페이스에서 `auto init --platforms omp`가 끝났고 무인증 RPC용 더미 프로바이더가 격리 프로필에 있다.
When `omp --mode rpc --no-session` 세션을 열고 시작 시 방출되는 `available_commands_update` 프레임과 `/dump`가 반환하는 시스템 프롬프트를 수집하고, 같은 워크스페이스에서 `omp ttsr list --json`을 함께 수집한다. `/context`는 토큰 사용량만 반환하고, `omp read rule://`와 디스크 형태 검사(파일 존재·경로 모양·개수)는 세션 도달을 증명하지 못하므로 증거로 쓰지 않는다.
Then 규칙 14종이 모두 세션에 도달한다. 세 주입 경로는 관측 창구가 서로 다르므로 오라클은 창구별로 수집한 뒤 합집합을 소스 규칙 집합과 비교한다 — (a) `<domain-rules>` 블록의 `- autopus-<name> (globs): description` 행 이름, (b) `omp ttsr list --json`의 `name`(경로가 `<ws>/.omp/rules/autopus-*.md`인 항목), (c) `<generic-rules>` 블록에 본문이 주입된 `alwaysApply` 규칙. 합집합에서 `autopus-` 접두사를 제거한 집합이 `content/rules/` 파일명에서 `.md`를 제거한 집합과 같고, 세 경로 중 둘 이상에 동시 등록된 규칙은 없다.
And 관측된 분해는 `<domain-rules>` 9(같은 블록에 사용자 파일 `mine` 1건이 함께 나타나 총 10행) + TTSR 3(`autopus-lore-commit`, `autopus-shell-portability`, `autopus-worktree-safety`) + `alwaysApply` 2(`objective-reasoning`, `language-policy`)다. TTSR 3종은 **시스템 프롬프트에 이름도 본문도 나타나지 않으므로** `/dump`만으로 단정하면 3건을 놓친다. `alwaysApply` 2종은 `<generic-rules>`에 본문만 주입되고 이름이 표시되지 않으므로 본문 매칭으로 식별한다. 디스크에 규칙 파일 14개가 존재한다는 사실만으로는 이 단정을 닫지 못한다 — 하위 디렉터리 배치에서는 같은 14개 파일로 세 경로 모두 0건이었다.
And task agent 목록은 ADK 에이전트 16종 이름을, 커맨드 목록은 workflowSpecs 20종 이름을 모두 담고 스킬 목록은 `auto`를 담는다.
And 루트 `AGENTS.md`는 그대로 남고 `.claude/CLAUDE.md`는 생성되지 않으며, 방출된 규칙·에이전트 본문 어디에도 `.claude/` 문자열이 없다.

### S8: 디스패치 10곳과 생성 표면 위생 (Must, oracle)

Given `autopus.yaml`이 플랫폼으로 `omp`를 나열한다.
When `auto init`, `auto platform add omp`, `auto update`, `auto update --preview`, `auto doctor`, `auto doctor --json`, `auto platform remove omp`를 차례로 실행한다.
Then 어떤 출력에도 `알 수 없는 플랫폼: omp` 문구가 없고 `auto doctor`의 omp 검증 결과는 실패 항목 0건이다.
And `auto doctor --json` 출력의 platforms 배열은 omp 항목을 담고, `auto update --preview`는 omp 표면 변경 계획을 오류 없이 출력하며, drift 게이트는 omp 어댑터를 nil이 아닌 값으로 해석한다.
And `auto init` 이후 `.gitignore`는 `.omp/rules/`, `.omp/agents/`, `.omp/config.yml` 줄을 담고 `/.omp/` 줄도, 파일명 글롭 `.omp/rules/autopus-*.md`도, 와일드카드 `.omp/rules/*`도 담지 않는다.
And `git status --porcelain`이 omp 생성 경로를 미추적 변경으로 보고하지 않는다.
And 그 `.gitignore` 적용 상태에서 omp 세션의 규칙 도달 집합이 여전히 14종이다. 이 비대칭은 **git 저장소 안에서만** 성립한다(`git init` 하지 않은 디렉터리에서는 파일명 글롭을 넣어도 14/14가 발견되므로, 오라클은 반드시 저장소로 초기화된 워크스페이스에서 측정한다). 같은 저장소에서 패턴만 바꾼 대조 측정이 `.omp/rules/`와 `/.omp/rules/`는 TTSR 3 + `<domain-rules>` 10(autopus 9 + 사용자 `mine` 1) + `alwaysApply` 2를 유지하고, `.omp/rules/autopus-*.md`는 세 경로를 각각 0·1(`mine`만)·0으로, `.omp/rules/*`는 전부 0으로 떨어뜨린다는 것을 고정한다.
And 사용자가 `.omp/rules/` 안에 둔 자기 규칙(`mine.md`)은 디렉터리 패턴 때문에 ignored 상태이며 negation(`!/.omp/rules/mine.md`)으로는 되돌아오지 않는다. 추적하려면 `git add -f .omp/rules/mine.md`를 쓰거나 규칙을 `.omp/rules/` 밖에 두어야 하고, 두 경우 모두 규칙 도달 집합은 14종을 유지한다.

### S9: 파리티 커버리지 (Must, oracle)

Given 파리티 게이트가 `claude`, `codex`, `gemini`, `opencode`, `omp`를 대상으로 돈다.
When 게이트가 각 플랫폼의 생성 규칙을 집계한다.
Then 어떤 게이트도 `unknown platform: omp` 로 실패하지 않는다.
And omp의 생성 규칙 수는 정확히 14이고 `platform-value` 불일치 finding 수는 0이며 omp의 rule·skill exclusion 집합은 모두 비어 있다.
And 기존 4개 플랫폼의 생성 규칙 수는 omp 추가 이전 값과 같다(claude 14, codex 14, gemini 14, opencode 14).

### S10: 스킬 소유권 양보 4개 설정 (Must, oracle)

Given 설정 A는 `omp`만, B는 `opencode`+`omp`, C는 `codex`+`omp`, D는 `antigravity-cli`+`omp`를 나열한다.
And B와 C에서는 해당 어댑터가 먼저 `.agents/skills/auto/SKILL.md`를 만들었다.
When 각 설정에서 omp 어댑터가 `Generate`를 수행한다.
Then 설정 A의 `.agents/skills/` 하위 디렉터리 집합은 `auto`를 담고 `harness-workflow`와 `agent-teams`를 담지 않는다. 두 스킬의 `platforms:` 선언이 omp를 담지 않기 때문이다.
And 설정 D의 `.agents/skills/` 디렉터리명 집합은 설정 A와 같다. gemini는 스킬을 `.agents/plugins/autopus/skills/`로 미러링하고 `.agents/skills/`에는 쓰지 않기 때문이다.
And 설정 B와 C에서 omp manifest는 `.agents/skills/` 경로를 하나도 담지 않고 `.agents/skills/auto/SKILL.md`의 바이트 내용은 omp `Generate` 전후로 같으며, 설정 B의 라이브 omp 세션이 발견하는 스킬 이름 집합은 설정 A와 같다.

### S11: 커맨드 소유권 — opencode 양보, antigravity 공존 (Must, oracle)

Given 설정 A는 `omp`만, B는 `opencode`+`omp`, C는 `antigravity-cli`+`omp`를 나열한다.
And 설정 C에서 gemini 어댑터가 먼저 `.agents/commands/auto.toml`과 `.agents/commands/auto/` 하위 파일을 만들었다.
When 각 설정에서 omp 어댑터가 `Generate`를 수행한다.
Then 설정 A의 `.agents/commands/` 하위 `.md` 파일 수는 정확히 20이고 이름 집합은 workflowSpecs 20종과 같으며 각 파일은 공백이 아닌 본문을 최소 1줄 갖는다.
And 설정 B에서 omp manifest는 `.agents/commands/` 경로를 하나도 담지 않는다.
And 설정 B의 라이브 omp 세션이 발견하는 커맨드 이름 집합은 `.opencode/commands/`를 통해 설정 A와 같은 20종 전부를 담는다.
And 설정 C에서 omp는 `.md` 커맨드 20종을 방출한다. gemini는 `.toml`만 쓰고 omp 커맨드 로더는 `md`만 읽으므로 양보하면 omp가 커맨드를 하나도 받지 못한다.
And 설정 C에서 `.agents/commands/auto.toml`과 `auto/` 하위 파일의 바이트 내용은 omp `Generate`·`Clean` 전후로 모두 같고, omp manifest는 확장자가 `.toml`인 경로를 하나도 담지 않는다.

### S12: 콘텐츠 정규화 (Must, oracle)

Given `content/rules/doc-storage.md`는 본문에 `.claude/`, `.codex/`, `.gemini/`, `.opencode/`를 담는다.
And ADK 스킬 본문 중 하나는 `Agent(subagent_type=...)` 호출과 `TodoWrite` 도구명을 담는다.
When omp 정규화를 거쳐 규칙과 스킬을 방출한다.
Then 방출된 `doc-storage` 규칙 본문은 `.claude/rules/` 대신 `.omp/rules/autopus-`를 담는다.
And 방출된 본문의 스킬 참조는 `.agents/skills/<name>/SKILL.md` 형태이며 `.agents/skills/<name>.md` 형태가 하나도 남지 않는다.
And branding 규칙 참조 문자열은 `.omp/rules/autopus-branding.md`이며 `content/rules/branding.md`가 아니다.
And 방출된 스킬 본문은 `TodoWrite` 대신 omp 도구명 `todo`를 담는다.
And `ReplacePlatformReferences(body, "omp")`의 출력은 입력과 다르다. 즉 no-op이 아니다.

### S13: `.omp/` 사용자 표면 보존 (Must, oracle)

Given omp 단독 설정으로 `Generate`가 끝났다.
And 사용자가 `.omp/rules/my-rule.md`, `.omp/RULES.md`를 만들고 `.omp/config.yml` 마커 밖에 `disabledProviders` 항목을 추가했다.
And 사용자가 `.omp/agents/executor.md`를 직접 편집했다.
When `auto platform remove omp`를 실행한다.
Then `.omp/rules/my-rule.md`와 `.omp/RULES.md`와 `.omp/` 디렉터리가 그대로 남고(`autopus-` 접두사가 아니고 manifest에도 없기 때문이다), 사용자가 편집한 `.omp/agents/executor.md`는 삭제 전에 백업 디렉터리로 보존된다.
And `.omp/config.yml`은 삭제되지 않고 관리 마커 구간만 제거된 채 남아 `disabledProviders` 같은 사용자 키가 YAML로 그대로 파싱된다. manifest 체크섬은 병합 문서 기준이라 손대지 않은 사용자 파일도 체크섬이 일치해 백업 없이 삭제될 수 있기 때문이다. 마커 밖 내용이 전혀 없으면 빈 껍데기를 남기지 않고 파일을 제거한다.
And omp가 만들고 사용자가 건드리지 않은 `.omp/agents/planner.md`와 `.omp/rules/autopus-branding.md`는 백업 없이 제거되며, `autopus.yaml`을 읽을 수 없으면 omp가 단독 소유를 증명할 수 없는 `.agents/skills/`는 건드리지 않는다.

### S14: orchestra provider 자동 등록 차단 (Must, oracle)

Given `autopus.yaml`의 `orchestra.enabled`가 참이고 `orchestra.providers`에 omp 항목이 없다.
When `auto platform add omp`를 실행한다.
Then `autopus.yaml`의 `orchestra.providers`에 `omp` 키가 생기지 않고 기존 provider 항목의 수와 내용이 변하지 않는다.
And omp 표면 생성은 정상적으로 끝난다.

### S15: 바이너리 신원 확인 (Should, oracle)

Given PATH에 `omp`라는 이름의 실행 파일이 있고 그 `--version` 출력이 `omp/17.1.8`이다.
And 다른 시험에서는 같은 이름의 실행 파일이 oh-my-pi가 아닌 문자열을 출력한다.
When 플랫폼 감지를 수행한다.
Then 첫 번째 경우만 omp가 감지된 플랫폼 목록에 들어가고 두 번째 경우는 들어가지 않는다.

## Edge Cases

### E1: omp 인식 밖 frontmatter 키만 있는 규칙

Given 규칙 소스가 `category: workflow` 하나만 frontmatter로 선언한다.
When omp 규칙 변환기가 변환한다.
Then 출력 frontmatter는 `category`를 버리고 본문 제목에서 합성한 `description` 한 줄만 담는다. omp는 description이 있는 규칙만 세션에 등록하므로 frontmatter 없이 방출하면 파일은 남고 발견되지 않는다.
And 본문은 소스 본문과 바이트 단위로 같고, 같은 입력을 반복 변환하면 같은 바이트가 나오며, trigger 키를 가진 규칙은 description을 얻지 않아 TTSR 라우팅이 그대로 유지된다.

### E2: 매핑 불가 도구만 선언한 에이전트

Given 에이전트 소스가 `tools: NotARealTool` 하나만 선언한다.
When `TransformAgentForOMP`가 변환한다.
Then 출력은 `tools:` 키를 담지 않고 `name`과 `description`은 그대로 남는다.

### E3: 사용자가 편집한 `.omp/config.yml`

Given 사용자가 마커 구간 밖에 `disabledProviders` 항목을 추가했다.
When omp 어댑터가 `Update`를 수행한다.
Then `disabledProviders` 항목이 그대로 남고 마커 구간 안의 `skills.customDirectories`만 갱신된다.

### E4: 소유권 전환 후 omp Update

Given omp 단독 설정으로 `Generate`가 끝나 omp manifest가 `.agents/skills/`와 `.agents/commands/` 20개 경로를 담고 있다.
When 설정에 `opencode`를 추가하고 omp 어댑터가 `Update`를 수행한다.
Then omp manifest는 `.agents/skills/`와 `.agents/commands/` 경로를 하나도 담지 않는다.
And omp가 만들었고 이후 변경되지 않은 `.agents/commands/*.md` 20개는 제거된다.
And 이후 omp `Clean`은 `.agents/skills/` 하위 파일을 하나도 지우지 않는다.

### E6: stale manifest 상태의 Clean — omp Update 없이 플랫폼이 추가된 경우

Given omp 단독 설정으로 `Generate`가 끝나 omp manifest가 `.agents/skills/auto/SKILL.md`를 담고 있다.
And 설정에 `opencode`를 추가하고 `auto platform add opencode`만 실행해 opencode가 그 파일을 자기 내용으로 덮어썼다.
And omp 어댑터의 `Update`는 아직 실행되지 않아 omp manifest가 그 경로를 그대로 담고 있다.
When omp 어댑터의 `Clean`을 실행한다.
Then `.agents/skills/auto/SKILL.md`가 남고 그 바이트 내용은 opencode가 쓴 내용과 같다.
And 그 파일에 대한 백업이 만들어지지 않는다. 소유권 재평가에서 제외된 표면이므로 체크섬 비교 자체를 하지 않기 때문이다.
And omp가 계속 소유하는 `.omp/rules/autopus-` 14개와 `.omp/agents/` 16개는 정상적으로 제거된다.

### E5: tools 생략의 런타임 의미

Given 에이전트 정의가 `tools` 키를 담지 않는다.
When omp가 그 에이전트를 로드한다.
Then omp가 적용하는 기본 도구 집합은 `read`, `grep`, `glob` 세 개이며 소스가 선언했던 집합보다 넓지 않다.

### E7: 레거시 규칙 경로 업그레이드

Given 구버전 ADK가 만든 워크스페이스의 omp manifest가 `.agents/rules/autopus/` 14개 경로를 담고 그 파일들이 디스크에 남아 있다.
And 사용자가 `.agents/rules/autopus/user-extra.md`를 직접 만들었고 manifest에는 없다.
When 새 버전으로 `auto update`를 수행한다.
Then manifest에 기록됐던 `.agents/rules/autopus/` 14개는 제거되고 새 `.omp/rules/autopus-` 14개가 방출된다. `ompExclusivePruneRoots()`가 레거시 루트 `.agents/rules/autopus`를 계속 담기 때문이다.
And manifest에 없는 `.agents/rules/autopus/user-extra.md`는 그대로 남는다. prune은 이전 매니페스트에 기록된 경로로만 제한되기 때문이다.

## Oracle Acceptance Notes

- Must 시나리오는 concrete expected output 또는 explicit tolerance를 담으며 구조 확인만으로 닫지 않는다. S3와 S12는 정규화 전후 문자열을 직접 비교해 no-op 회귀를 잡고, S4·S11·S13은 바이트 동일성과 백업 존재라는 결정적 오라클을 쓰며, S6·S9·S10·S11은 집합 동등성과 정확한 개수를, S7은 라이브 omp 세션의 실제 주입 결과를 예상 값으로 고정한다. 규칙에 대해서는 디스크 형태 검사를 오라클로 쓰지 않는다 — 같은 파일 집합이 배치 위치에 따라 도달 14/14와 0/14로 갈렸으므로, 규칙 오라클은 `<domain-rules>` 목록·TTSR 등록·`alwaysApply` 본문 주입 세 경로의 합집합만 근거로 삼는다.
- 수치 허용 오차가 필요한 항목은 없다. 모든 기대값은 정확 일치다.

## Definition of Done

- [x] S1 부터 S15, E1 부터 E7 전부 통과 — 오라클 테스트와 라이브 실측으로 확인. 규칙 도달의 최신 근거는 omp 17.3.5 재실측(`.omp/rules/autopus-*` 배치에서 14/14)이며, 2026-08-01 omp 17.1.8의 `.agents/rules/autopus/` 14/14 주장은 17.3.5에서 재현되지 않았다(같은 배치 도달 0/14).
- [x] `go test ./...` green 및 코드 리뷰 완료 — 테스트 green(100 패키지, exit 0), 리뷰 APPROVE(blocker 0) + 보안 3라운드 Critical/High 0 + 적대적 폐쇄 검증 PASS (2026-08-01)
