# SPEC-OMP-001: omp(oh-my-pi) 플랫폼 최소 표면 지원

**Status**: completed
**Created**: 2026-07-30
**Revision**: 2 (debate review REVISE 23건 반영)
**Domain**: OMP

## 목적

omp(oh-my-pi) CLI 사용자는 현재 Autopus ADK 하네스를 받지 못한다. `omp`는 `validPlatforms`에 없고 어댑터도 없어서 `auto init`이 어떤 omp 표면도 만들지 않는다.

omp는 규칙을 `.claude/rules/`에서 읽지 않는다. 실측된 omp 17.1.8의 규칙 소스는 native `.omp/rules/`, omp-plugins, 그리고 `agents` provider(`.agent/rules`, `.agents/rules` ancestor walk-up)이며, task agent는 `.omp/agents/`에서만 읽는다. 기존 어댑터 표면 중 omp에 도달하는 것은 `.agents/skills/`(codex·opencode), `.agents/commands/auto*`(antigravity), `.opencode/commands/`(opencode)뿐이고 규칙과 에이전트는 전달 경로 자체가 없다.

이 SPEC은 omp 사용자가 ADK 하네스를 발견하고 **omp에서 올바르게 동작하도록** 하는 최소 착륙 지점을 정의한다. 발견만으로는 부족하다. 방출되는 본문이 omp가 읽지 않는 경로(`.claude/...`)를 지시하면 하네스는 오작동한다.

## Outcome Boundary

**User-visible outcome**: omp가 설치된 워크스페이스에서 `auto init` 또는 `auto platform add omp`를 실행하면, 이후 omp 세션이 ADK 규칙 14종과 에이전트 16종을 발견하고, omp 단독 워크스페이스에서는 스킬 카탈로그와 워크플로 커맨드까지 발견하며, 방출된 본문의 경로 참조가 omp 네이티브 경로로 정규화되어 있다.

**Mandatory requirements**: REQ-001 부터 REQ-019 까지 전체.

**Explicit non-goals**

- omp orchestra provider 실행 배선(subprocess 호출, 모델 라우팅). REQ-018은 그 반대로 **자동 등록을 막는** 요구사항이다
- worker `ProviderAdapter` 구현 및 interactive pane 주입
- 루트 컨텍스트 문서 신규 생성 — 루트 `AGENTS.md`가 omp agents-md provider로 이미 주입된다
- omp native `.omp/rules/`, `.omp/RULES.md` sticky 규칙 활용
- 기존 플랫폼 manifest 스키마 메이저 버전 변경 및 마이그레이션

**Completion evidence**: `go test ./...` green, `auto doctor` omp 통과, 그리고 무인증 omp RPC 실측(`omp --mode rpc --no-session` + `/context`, `get_available_commands`)에서 규칙 14종·에이전트 16종·스킬 카탈로그·커맨드 20종 발견과 정규화된 경로 참조 확인.

## Requirements

### REQ-001 — 어댑터 등록과 manifest 전량 기록

THE SYSTEM SHALL provide an `omp` platform adapter that implements all ten `PlatformAdapter` methods and records every file it generates, across every surface it owns, in `.autopus/omp-manifest.json`.

- EARS type: Ubiquitous / Priority: Must
- manifest 범위는 omp가 실제로 만든 파일 전체다. `.agents/` 소유권 제한(REQ-009)은 어느 경로를 만들 수 있는지의 제약이지 manifest 기록 대상의 제약이 아니다
- 관측 지점: `.autopus/omp-manifest.json` 파일 목록

### REQ-002 — 규칙 방출 (네임스페이스)

WHEN platform file generation runs for a configuration that lists `omp`, THEN THE SYSTEM SHALL write all 14 ADK rules to `.agents/rules/autopus/<rule-name>.md`.

- EARS type: Event-driven / Priority: Must
- 네임스페이스 근거: omp 17.1.8 실측에서 `.agents/rules/` 하위 디렉터리의 규칙이 발견된다(플랫 전용이 아니다). 기존 4개 어댑터가 모두 `rules/autopus/`로 네임스페이스하는 관례와도 맞고, `.agents/rules/` 최상단의 사용자 규칙과 충돌하지 않는다
- 관측 지점: `.agents/rules/autopus/` 디렉터리 내용과 omp 세션 규칙 목록

### REQ-003 — 규칙 frontmatter passthrough

WHERE a rule source declares YAML frontmatter keys that the omp rule parser recognizes, THEN THE SYSTEM SHALL emit those keys unchanged, drop source keys outside that recognized set, and synthesize a deterministic `description` from the rule body when the source yields no recognized key.

- EARS type: State-driven / Priority: Must
- 인식 키: `description`, `globs`, `alwaysApply`, `condition`, `astCondition`, `scope`, `interruptMode`
- 합성 규칙: 본문 첫 H1 제목, 없으면 첫 산문 줄에서 한 줄을 만든다. 펜스 코드는 제외하고 인라인 마크업을 평탄화하며 120 rune으로 자른다. 같은 입력은 같은 바이트를 낸다
- 합성 조건: 인식 키가 하나라도 살아남으면 합성하지 않는다. trigger 키(`condition`·`astCondition`·`scope`)를 가진 규칙이 description을 얻으면 TTSR 엔진에서 always-listed 블록으로 라우팅이 바뀌기 때문이다
- 근거: omp 17.1.8 실측상 frontmatter에 `description`이 없는 규칙은 파일로만 남고 세션에 등록되지 않아 REQ-002의 방출이 무의미해진다
- 관측 지점: 생성된 규칙 파일의 frontmatter 블록과 omp 세션 `<domain-rules>` 목록

### REQ-004 — 규칙 본문 비어 있지 않음

THE SYSTEM SHALL emit every generated rule with a non-empty markdown body, because the omp rule validator rejects a rule whose content is empty.

- EARS type: Ubiquitous / Priority: Must
- 관측 지점: 생성된 규칙 파일의 frontmatter 이후 본문

### REQ-005 — 에이전트 방출

WHEN platform file generation runs for `omp`, THEN THE SYSTEM SHALL write all 16 ADK agent definitions to `.omp/agents/<agent-name>.md` with a populated `name`, `description`, and `model`, and with the agent source body emitted after omp reference normalization.

- EARS type: Event-driven / Priority: Must
- `model`은 ADK의 품질 라우팅 계층이며 omp `parseAgent`가 `model`을 인식한다. 무변환 본문은 `content/agents/annotator.md`·`frontend-specialist.md`가 참조하는 `.claude/skills/autopus/<name>.md`를 그대로 남겨 존재하지 않는 경로를 지시하게 되므로 정규화가 필수다
- 관측 지점: `.omp/agents/` 내용과 각 파일 frontmatter·본문

### REQ-006 — 에이전트 도구 매핑

WHERE an agent source declares a `tools` value, THEN THE SYSTEM SHALL translate each entry to its omp canonical tool name, pass `mcp__`-prefixed entries through unchanged, drop entries that have no omp canonical equivalent, and emit a deduplicated alphabetically sorted list; and where no entry survives translation, the system shall omit the `tools` key.

- EARS type: State-driven / Priority: Must
- `tools` 생략 시 omp 기본값은 실측상 `read`, `grep`, `glob` 읽기 전용 집합이다. 즉 생략은 권한 확대가 아니라 축소이며 fail-closed다
- 관측 지점: 생성된 에이전트 파일의 `tools` 목록

### REQ-007 — 설정 방출 (사용자 내용 보존)

WHEN platform file generation runs for `omp`, THEN THE SYSTEM SHALL write `.omp/config.yml` using a marker-delimited managed section so that keys outside that section survive regeneration unchanged.

- EARS type: Event-driven / Priority: Must
- `OverwriteMerge`는 YAML 병합이 아니라 `WriteFileIfChanged` 바이트 교체이므로 사용자 키 보존을 보장하지 못한다. 마커 섹션 기계장치(`OverwriteMarker`)를 쓴다
- 관리 대상 키: `skills.customDirectories`. 사용자가 추가한 `disabledProviders` 등은 마커 밖에 남는다
- 관측 지점: `.omp/config.yml` 마커 섹션과 그 바깥 내용

### REQ-008 — CLAUDE.md shadowing 금지

IF an omp code path would create `.claude/CLAUDE.md`, THEN THE SYSTEM SHALL refuse to create it, because omp's claude context provider loads `CLAUDE.md` from `.claude/` directories at priority 80 and shadows the root `AGENTS.md` supplied at priority 10.

- EARS type: Unwanted behavior / Priority: Must
- 관측 지점: 생성 파일 집합과 omp 세션 컨텍스트 파일 목록

### REQ-009 — 소유권 경계

WHERE a path is not one that omp created and recorded in its own manifest, THEN THE SYSTEM SHALL leave that path byte-identical during `Generate`, `Update`, and `Clean`.

- EARS type: State-driven / Priority: Must
- 특히 `.agents/plugins/`, `.agents/hooks.json`, 그리고 양보된 `.agents/skills/`·`.agents/commands/`는 omp가 만들지 않으므로 건드리지 않는다
- 관측 지점: omp manifest 경로 집합과 작업 전후 바이트 비교

### REQ-010 — 식별자와 디스패치 일관성

THE SYSTEM SHALL register `omp` in the five platform identifier tables and the ten CLI dispatch sites so that `auto init`, `auto platform add`, `auto platform remove`, `auto update`, `auto update --preview`, and `auto doctor` handle `omp` without reaching any unknown-platform skip branch.

- EARS type: Ubiquitous / Priority: Must
- 관측 지점: 열 개 디스패치 사이트 각각의 관측 가능한 결과(S8)

### REQ-011 — 생성 표면 위생

WHEN `auto init` writes `.gitignore` patterns, THEN THE SYSTEM SHALL add only directory-form patterns that cover ADK-authored omp paths, and shall not add any file-name-form pattern under a discovery root.

- EARS type: Event-driven / Priority: Must
- 추가 패턴: `.agents/rules/autopus/`, `.omp/agents/`, `.omp/config.yml`. `/.omp/` 전체는 사용자 소유 native 표면(`.omp/rules/`, `.omp/RULES.md`)을 포함하므로 쓰지 않는다
- 실측 제약: omp 발견 글롭은 `gitignore: true`로 동작한다. 디렉터리형 패턴(`.agents/rules/`)은 발견을 막지 않지만 파일명형 패턴(`ignored-rule.md`)은 해당 규칙을 발견 목록에서 제거한다. `.omp/config.yml`은 발견 글롭 루트가 아니라 settings 직접 읽기 대상이므로 파일형 패턴이 안전하다
- 관측 지점: `.gitignore` 내용, `git status --porcelain`, 그리고 gitignore 적용 후의 omp 발견 결과

### REQ-012 — 파리티 커버리지 포함

THE SYSTEM SHALL include `omp` in the multi-platform parity coverage lists and in the platform-to-adapter switch of every parity gate, with an expected generated rule count of 14 and empty rule and skill exclusion sets.

- EARS type: Ubiquitous / Priority: Should
- 목록만 추가하면 `parity_coverage_test.go`의 `default: unknown platform` 에러와 `parity_acceptance_test.go`의 `t.Fatalf`로 게이트가 즉시 실패한다
- 관측 지점: 파리티 게이트 실행 결과

### REQ-013 — 스킬 표면 소유권 양보

WHERE neither `codex` nor `opencode` is listed in the active platform configuration, THEN THE SYSTEM SHALL emit the compiled ADK skill catalog to `.agents/skills/<skill-name>/SKILL.md`; and where either platform is listed, the system shall emit no skill file and yield that surface.

- EARS type: State-driven / Priority: Must
- 소유권 순서 opencode > codex > omp. 기존 `codexOwnsSharedSurface`가 codex의 opencode 양보를 이미 고정한다
- `antigravity-cli`는 이 목록에 없다. gemini 어댑터는 스킬을 `.agents/plugins/autopus/skills/`로 미러링하며 `.agents/skills/`에는 쓰지 않는다
- 관측 지점: `.agents/skills/` 경로 집합, omp manifest, 그리고 공존 설정에서의 omp 세션 스킬 발견

### REQ-014 — 커맨드 표면 소유권 양보

WHERE `opencode` is not listed in the active platform configuration, THEN THE SYSTEM SHALL emit the 20 ADK workflow commands to `.agents/commands/<command-name>.md`; and where `opencode` is listed, the system shall emit no command file and rely on the existing `.opencode/commands/` surface.

- EARS type: State-driven / Priority: Must
- `antigravity-cli`는 양보 대상이 아니다. gemini 어댑터가 같은 디렉터리에 `.agents/commands/auto.toml`과 `auto/<file>`을 만들지만 omp 커맨드 로더는 확장자 `md`만 읽으므로, antigravity에 양보하면 omp가 커맨드를 하나도 받지 못한다. 확장자가 겹치지 않아 공존이 안전하며 REQ-009와 REQ-017의 manifest 기반 경계가 gemini 파일을 보호한다
- 관측 지점: `.agents/commands/` 파일명 집합과 omp `get_available_commands` 응답

### REQ-015 — omp 콘텐츠 정규화

WHERE generated rule, agent, or skill content contains Claude-specific path references, agent-invocation syntax, or tool names, THEN THE SYSTEM SHALL rewrite them to their omp equivalents before the content is written.

- EARS type: State-driven / Priority: Must
- 현재 `pathReplacements`, `brandingRule`, `replaceAgentCalls`, `replaceTodoWrite`, `replaceWorkflowTools`에 omp 키가 없어 `ReplacePlatformReferences(body, "omp")`는 전면 no-op이고 `NormalizeAgentReferences`는 저장소 내부 경로 `content/rules/branding.md`로 폴백한다
- 1단계 접두사 매핑: `.claude/skills/autopus/`와 `.claude/skills/` → `.agents/skills/`, `.claude/commands/` → `.agents/commands/`, `.claude/agents/` → `.omp/agents/`, `.claude/rules/` → `.agents/rules/autopus/`, `.claude/` → `.omp/`. branding 규칙 참조는 `.agents/rules/autopus/branding.md`
- 2단계 스킬 경로 보정 필수: 1단계만 적용하면 `.claude/skills/autopus/ax-annotation.md`가 `.agents/skills/ax-annotation.md`가 되는데 실제 방출 경로는 `.agents/skills/ax-annotation/SKILL.md`이므로 존재하지 않는 파일을 지시한다. `skill_transformer_replace.go:195`가 opencode에만 적용하는 `.agents/skills/<name>.md` → `.agents/skills/<name>/SKILL.md` 재작성을 omp에도 적용한다. 이 최종 형태는 omp가 스킬 표면을 소유하든 양보하든 동일하다
- 관측 지점: 생성된 규칙·에이전트·스킬 본문의 경로 문자열

### REQ-016 — 스킬 카탈로그 컴파일 타깃

THE SYSTEM SHALL include `omp` in the default skill compile-target set so that catalog skills without an explicit `platforms:` declaration compile for omp.

- EARS type: Ubiquitous / Priority: Must
- `shouldCompileCatalogSkill`은 `CompileTargets`에 정규화된 플랫폼이 없으면 즉시 거부하고, 기본 `compileTargetsForSkill`은 claude·codex·gemini·opencode만 반환한다. 식별자 테이블만 고쳐서는 스킬이 하나도 생성되지 않는다
- 관측 지점: omp 단독 설정에서 컴파일된 스킬 수

### REQ-017 — 파괴적 제거 금지

WHERE `Clean` or `auto platform remove omp` runs, THEN THE SYSTEM SHALL re-evaluate surface ownership first, skip every surface omp no longer owns without removing or backing up anything under it, remove only the remaining manifest-recorded paths, back up any such file whose current checksum differs from the manifest checksum, and shall not remove the `.omp/` or `.agents/` directories wholesale.

- EARS type: State-driven / Priority: Must
- `.omp/`는 omp의 사용자 소유 native 루트다(`.omp/rules/`, `.omp/RULES.md`, 사용자 설정). 디렉터리 통째 삭제는 사용자 데이터 손실이다
- 구현 위치: `adapter.PruneManagedPaths`는 같은 계약을 담지만 호출자 없는 orphan이라 omp `Clean`이 체크섬 불일치 백업 → 제거 → 빈 부모 정리를 직접 구현한다
- 소유권 재평가가 먼저인 이유: `auto platform add opencode`는 새 어댑터만 Generate하므로 omp manifest는 갱신되지 않은 채 `.agents/skills/` 경로를 계속 담는다. 체크섬만 보면 opencode가 덮어쓴 파일이 불일치로 판정돼 백업 후 삭제되어 타 플랫폼의 활성 파일이 사라진다. 양보한 표면은 아예 건드리지 않아야 한다
- 관측 지점: `Clean` 전후 `.omp/`와 `.agents/`의 비관리 파일

### REQ-018 — orchestra provider 자동 등록 차단

IF `auto platform add omp` would register an orchestra provider entry for a platform that has no orchestra provider mapping, THEN THE SYSTEM SHALL skip that registration.

- EARS type: Unwanted behavior / Priority: Must
- 현재 `internal/cli/platform.go:116-122`는 `PlatformToProvider`가 빈 값이면 플랫폼 이름으로 폴백해 `EnsureOrchestraProvider`를 호출하고, 미지 provider는 `ProviderEntry{Binary: "omp"}`로 등록된다. 이는 orchestra 배선을 non-goal로 둔 경계와 직접 충돌한다
- 파급: 같은 폴백을 타는 `cursor`도 함께 등록이 멈춘다. 어댑터도 provider 매핑도 없는 플랫폼이므로 의도된 수정이다
- 관측 지점: `auto platform add omp` 이후 `autopus.yaml`의 `orchestra.providers`

### REQ-019 — 바이너리 신원 확인

WHERE platform detection finds an executable named `omp` on PATH, THEN THE SYSTEM SHALL treat it as the omp platform only when its version output matches the oh-my-pi version shape.

- EARS type: State-driven / Priority: Should
- `omp`는 짧은 이름이라 무관한 바이너리와 충돌할 수 있고, 현재 `knownCLIs`는 이름 존재만 확인한다. 실측 출력은 `omp/17.1.8` 형태다
- 관측 지점: `auto doctor`의 플랫폼 감지 결과

## 생성 파일 상세

| 경로 | 역할 | Overwrite |
|------|------|-----------|
| `[NEW] pkg/adapter/omp/omp.go` | 식별자, `New`/`NewWithRoot`, `Detect`, `Generate`, `Update` | - |
| `[NEW] pkg/adapter/omp/omp_rules.go` | 규칙 14종을 `.agents/rules/autopus/`로 방출 | always |
| `[NEW] pkg/adapter/omp/omp_agents.go` | 에이전트 16종을 `.omp/agents/`로 방출 | always |
| `[NEW] pkg/adapter/omp/omp_config.go` | `.omp/config.yml` 마커 섹션 방출 | marker |
| `[NEW] pkg/adapter/omp/omp_skills.go` | 소유 시 스킬 카탈로그 방출, 미소유 시 양보 | always |
| `[NEW] pkg/adapter/omp/omp_commands.go` | 소유 시 커맨드 20종 방출, 미소유 시 양보 | always |
| `[NEW] pkg/adapter/omp/omp_specs.go` | omp workflow spec 목록 (아래 소유권 결정 참조) | - |
| `[NEW] pkg/adapter/omp/omp_ownership.go` | `ompOwnsSharedSkillSurface`, `ompOwnsCommandSurface` | - |
| `[NEW] pkg/adapter/omp/omp_lifecycle.go` | `Validate`, `Clean`(manifest 기반), `SupportsHooks`, `InstallHooks` | - |
| `[NEW] pkg/content/agent_transformer_omp.go` | `TransformAgentForOMP` 와 omp 도구명 매핑 | - |
| `[NEW] pkg/content/rule_frontmatter_omp.go` | 규칙 frontmatter 파싱과 선별 방출 | - |

`workflowSpecs`는 opencode와 codex가 각각 unexported 복사본을 갖고 있어(`opencode_specs.go:10`, `codex_workflow_specs.go:15`) 재사용이 불가능하다. 공용 패키지 승격은 두 기존 어댑터의 회귀 위험을 이 SPEC 범위로 끌어들이므로, omp는 세 번째 복사본을 `omp_specs.go`에 두고 그 소유권을 명시한다. 방출기는 스킬과 커맨드를 각각 다른 파일로 분리해 300줄 제한을 지킨다.

`SupportsHooks()`는 `false`를 반환하고 `InstallHooks`는 아무 파일도 만들지 않는 no-op이다. omp는 훅 개념 대신 TypeScript 확장을 쓰며 확장 배선은 이 SPEC의 non-goal이다.

## Related SPECs

**SPEC-CONDRULE-001** (병렬 진행): 규칙 frontmatter에 `condition`과 `scope` 트리거 필드를 도입한다. REQ-003의 passthrough는 있으면 보존하고 없으면 생략하므로 두 SPEC은 상호 의존 없이 각각 단독 착륙한다.

Sibling SPEC은 없다.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-001 | T2 | S4, S7 | INV-014 |
| REQ-002 | T3 | S1, S6 | INV-006 |
| REQ-003 | T3 | S1, S2 | INV-001 |
| REQ-004 | T3 | S1 | INV-002 |
| REQ-005 | T4, T5 | S3, S6 | INV-003, INV-006 |
| REQ-006 | T4 | S3, E5 | INV-004, INV-005 |
| REQ-007 | T6 | S6, E3 | INV-008 |
| REQ-008 | T7 | S5 | INV-009 |
| REQ-009 | T7 | S4, S10, S11 | INV-007 |
| REQ-010 | T1, T8 | S8 | INV-010 |
| REQ-011 | T9 | S8 | INV-011 |
| REQ-012 | T10 | S9 | INV-006 |
| REQ-013 | T13 | S10 | INV-012 |
| REQ-014 | T14 | S11 | INV-013 |
| REQ-015 | T15 | S12, S3 | INV-015 |
| REQ-016 | T16 | S10 | INV-016 |
| REQ-017 | T17 | S13, S4, E4, E6 | INV-017 |
| REQ-018 | T18 | S14 | INV-018 |
| REQ-019 | T19 | S15 | INV-019 |

## Out of Scope

Outcome Boundary의 explicit non-goals와 동일하다.
