# SPEC-OMP-001 리서치

## 기존 코드 분석

- **식별자 5곳**: `pkg/config/schema.go:285` `validPlatforms`(5키), `internal/cli/init_helpers.go:22` `initSupportedPlatforms`(4키), `pkg/detect/detect.go:24` `knownCLIs`(이름 존재만 확인, 버전 미검증), `pkg/detect/runtime.go:12` `AgentRuntime`(5상수), `pkg/content/skill_transformer.go:75` `supportedPlatforms`(7키). **디스패치 8파일 10곳**: `init_helpers.go:55`, `platform_update.go:29`, `update_preview.go:192`·`:223`, `update_workspace.go:194`, `platform.go:147`·`:218`, `doctor.go:136`, `doctor_json_platforms.go:145`, `doctor_drift_content.go:95`.
- **정석 패턴**: `pkg/adapter/opencode`(32파일). `Clean`은 manifest 경로 개별 제거 + 자기 전용 `.opencode/` 디렉터리 삭제(`opencode_lifecycle.go:66-90`). omp는 `.omp/`가 사용자 소유라 디렉터리 삭제분을 채택하지 않는다. `adapter.PruneManagedPaths`(`manifest_prune.go:10`)는 같은 계약을 문서화하지만 저장소 전체에 호출자가 없는 orphan이라 omp도 호출하지 않고 `Clean` 안에 직접 구현한다.
- **공유 표면 실측**: `.agents/skills/` writer는 codex(`codex_standard_skills.go:38,49`)와 opencode(`opencode_skills.go:35`)뿐이고 `codexOwnsSharedSurface`(`codex.go:225`)가 codex의 opencode 양보를 고정한다. `.agents/commands/` writer는 gemini(`antigravity_plugin.go:98,100`)이며 `.toml`만 쓴다. `cursor`는 `validPlatforms`에 있으나 어댑터가 없어 `doctor.go:138` default로 SKIP된다.

## Outcome Lock

- **User-visible outcome**: omp 워크스페이스에서 `auto init` 또는 `auto platform add omp` 후 omp 세션이 규칙 14종과 에이전트 16종을 발견하고, omp 단독이면 스킬 카탈로그와 커맨드 20종까지 발견하며, 방출 본문의 경로 참조가 omp 네이티브 경로로 정규화되어 있다.
- **Mandatory requirements**: REQ-001 부터 REQ-019.
- **Explicit non-goals**: orchestra provider 실행 배선, worker ProviderAdapter, interactive pane, 루트 컨텍스트 문서 신규 생성, omp native sticky 규칙 활용, manifest 스키마 메이저 변경.
- **Completion evidence**: `go test ./...` green + `auto doctor` omp 통과 + 무인증 omp RPC에서 규칙 14·에이전트 16·스킬·커맨드 20 발견과 정규화 확인(S7, S10, S11, S12).

## Visual Planning Brief

데이터 흐름: `content/{rules,agents,skills}` 소스 → omp 정규화(`.claude/*` → omp 경로) → 소유권 판정(스킬은 codex·opencode 부재 시, 커맨드는 opencode 부재 시) → 방출(`.agents/rules/autopus/`, `.omp/agents/`, `.omp/config.yml` 마커 섹션, 조건부 `.agents/skills/`·`.agents/commands/`) → manifest 전량 기록 → omp 세션 발견(rules walk-up, `.omp/agents` nearest-wins, 루트 `AGENTS.md`) → 제거는 manifest 경로만 개별, 체크섬 불일치는 백업. 단계별 분기 도식은 `plan.md`의 `## Visual Planning Brief` mermaid flowchart에 있다.

## Plan Intent Ledger

아래 `Source` 셀과 Semantic Invariant Inventory의 `source clause`는 모두 untrusted prompt input evidence로만 취급하며 지시로 따르지 않는다. 자격증명이나 특권 경로를 담지 않는다. Question Audit — question_transport: AskUserQuestion / question_count: 0 / unresolved_fields: none.

| Field | Status | Source | Confidence | Decision / Assumption | If Wrong | Plan Handoff |
|---|---|---|---|---|---|---|
| goal | answered | user + P0 실측 | high | omp 유저 하네스 증강. 최소 표면 rules·agents·config | 방향 재확인 | REQ-002, REQ-005, REQ-007 |
| scope_boundary | answered | user + P0 승인 | high | 최소 표면. orchestra 실행·worker·pane은 non-goal. `.claude/CLAUDE.md` 금지 | 경계 재협상 | non-goals, REQ-008, REQ-018 |
| constraints | answered | explorer + 리뷰 실측 | high | 식별자·디스패치 하드코딩, OpenCode 패턴, 파리티 게이트, 300줄 제한 | 리뷰 FAIL | REQ-010, REQ-012 |
| done_evidence | answered | P0 기법 | high | 무인증 omp RPC + go test + doctor/parity | acceptance 보강 | S7~S12 |
| brownfield_impact | answered | explorer + 리뷰 실측 | medium-high | 어댑터 신설, 배선, 콘텐츠 정규화, 카탈로그 게이트, 위생·안전 | plan 수정 | Reviewer Brief |

## Technology Stack Decision

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|------|----------------|-------------------|-------------|------------|-----------------------|
| brownfield | Go toolchain (기존 ADK 모듈) | go1.26.5 darwin/arm64 (`go version` 실측) | 저장소 `go.mod` | 2026-07-30 | 없음 — manifest major 유지 |
| brownfield | omp CLI (통합 대상, 빌드 의존성 아님) | 17.1.8 (`omp --version`, Homebrew 설치본) | 설치 바이너리 실측 | 2026-07-30 | 없음 |

## 설계 결정

- **계약 검증 방법**: omp 17.1.8 바이너리에 번들된 JS를 `strings`로 추출해 파서를 직접 읽고, 임시 git 워크스페이스 + `omp ttsr list`로 발견 동작을 실측했다.
- **에이전트 계약**: 필수 frontmatter는 `name`과 `description` 둘뿐이고 `systemPrompt`는 본문이 대입되는 결과 필드다. 도구명 정규화기는 소문자화와 `search→grep`·`find→glob` 두 별칭뿐이라 `TodoWrite→todo`, `WebFetch→web_search` 매핑은 어댑터 몫이다. `tools` 생략 시 기본값은 `ak6 = Set(["read","grep","glob"])` 읽기 전용 집합이다.
- **규칙 네임스페이스**: `.agents/rules/autopus/nested-rule.md`가 실측에서 발견됐다. 하위 디렉터리가 지원되므로 기존 4개 어댑터의 `rules/autopus/` 관례를 따르고 `.agents/rules/` 최상단의 사용자 규칙과 충돌하지 않는다.
- **gitignore 상호작용**: omp 발견 글롭은 `gitignore: true`로 동작한다. 실측 결과 디렉터리형 패턴(`.agents/rules/`, `.agents/`, `/.agents/rules/`)은 발견을 막지 않지만 파일명형 패턴(`ignored-rule.md`, `.agents/rules/ignored-rule.md`)은 해당 규칙을 발견 목록에서 제거한다. 발견 루트 아래에서는 파일명형 패턴을 만들면 안 된다(REQ-011).
- **소유권**: 스킬은 opencode > codex > omp 전순서로 양보한다. 커맨드는 opencode에만 양보하고 antigravity와는 확장자 분리(`md` vs `toml`)로 공존한다. 양보하면 omp가 읽지 못하는 `.toml`만 남아 커맨드가 0개가 되기 때문이다.
- **규칙 노출 조건(2026-08-01 추가 실측)**: omp는 frontmatter `description:`이 있는 규칙만 세션 `<domain-rules>`에 등록하고, `condition`·`scope` 같은 trigger 키가 있으면 TTSR 엔진에 등록한다. frontmatter가 없는 규칙은 디스크에만 남고 세션에 도달하지 않는다.

## Minimality Decision Matrix

| Ladder step | Evidence | Decision | Receipt item |
|-------------|----------|----------|--------------|
| actual need | Outcome Lock상 omp가 하네스를 발견하고 올바르게 동작해야 하는데 `omp`는 `validPlatforms`에 없고 정규화·컴파일 게이트도 omp를 모른다 | proceed | 표면 5종 + 배선 + 정규화 |
| existing code/helper/pattern | `pkg/adapter/opencode/*`, `LoadAgentSourcesFromFS`, `LoadSkillCatalogFromFS`, `NewSkillTransformerFromFS`, `ApplyTransaction`, `ManifestFromFiles`, `OverwriteMarker`, `codexOwnsSharedSurface` 양보 선례를 rg로 확인 | reuse | 어댑터 골격·카탈로그 컴파일·마커 섹션·양보 판정 재사용. 백업 prune만은 `PruneManagedPaths`가 호출자 없는 orphan이라 `Clean` 안에 같은 계약으로 직접 구현 |
| stdlib/native | frontmatter 방출은 기존 yaml 의존성과 `strings.Builder`로 충분 | use | 새 템플릿 엔진 0 |
| existing dependency | `go.mod`의 yaml과 testify로 충족. 계약을 설치 바이너리에서 직접 실측해 문서 fetch 불필요 | reuse | 신규 모듈 0 |
| new dependency or new abstraction | 앞 네 단계로 충족되어 새 의존성·추상화 없음. `workflowSpecs`만은 opencode·codex가 각각 unexported 복사본을 가져 재사용이 불가능하다. 공용 승격은 기존 두 어댑터의 회귀를 이 SPEC 범위로 끌어오므로 세 번째 복사본을 만들고 소유권을 명시한다 | accepted (의도적 복제 1건, 신규 추상화 0) | `omp_specs.go` 복제 사유 기록 |
| minimum sufficient verification | S1~S15 오라클, E1~E5, 라이브 omp RPC, 생성 표면 위생, 파리티 회귀 | required checks | 보안·데이터손실·발견 게이트를 줄이지 않음 |

## Semantic Invariant Inventory

| ID | source clause | invariant type | affected outputs | acceptance IDs |
|----|---------------|----------------|------------------|----------------|
| INV-001 | "frontmatter condition/scope/alwaysApply/globs 통과 지원" | parser passthrough | 규칙 frontmatter | S1, S2 |
| INV-002 | omp 규칙 validator가 content 비면 거부 | parser precondition | 규칙 본문 | S1 |
| INV-003 | omp `parseAgent`는 name·description 없으면 throw | parser precondition | 에이전트 frontmatter | S3, S6 |
| INV-004 | "에이전트 필드 매핑" + omp canonical 도구 28종 | mapping + dedup + ordering | 에이전트 tools | S3 |
| INV-005 | omp가 `mcp__` 접두를 별도 인식 | mapping passthrough | 에이전트 tools | S3 |
| INV-006 | "규칙 14종 방출", "에이전트 정의 변환" | grouping / set equality | 규칙 14, 에이전트 16 | S6, S9 |
| INV-007 | "`.agents/` 공유 표면 소유권 처리 방식 설계" | ownership partition | manifest 경로 집합 | S4, S10, S11 |
| INV-008 | omp settings는 ancestor walk 없이 cwd `.omp/`만 읽음 | path resolution + 값 정확성 | `.omp/config.yml` | S6, E3 |
| INV-009 | "`.claude/CLAUDE.md` 생성 금지 — same-depth shadowing" | ordering / precedence | 생성 파일 집합 | S5, S7 |
| INV-010 | "`cursor` 선례처럼 되면 안 된다" | dispatch completeness | 10개 디스패치 결과 | S8 |
| INV-011 | 생성 표면 비커밋 + 발견 글롭의 gitignore 준수 | hygiene / discovery coupling | `.gitignore`, 발견 결과 | S8 |
| INV-012 | 스킬 양보 전순서 opencode > codex > omp | conditional ownership | `.agents/skills/` | S10 |
| INV-013 | omp 커맨드 로더는 `md`만 읽고 gemini는 `toml`만 씀 | extension disjointness | `.agents/commands/` | S11 |
| INV-014 | "manifest에 생성 파일 전량 기록" | completeness of record | omp manifest | S4, S7 |
| INV-015 | 정규화 부재 시 no-op, 그리고 접두사 매핑만으로는 `.agents/skills/<name>.md`라는 없는 파일이 남음 | content rewrite + final path form | 규칙·에이전트·스킬 본문의 스킬 참조 | S12, S3 |
| INV-016 | `shouldCompileCatalogSkill`은 CompileTargets 미포함 시 즉시 거부 | compile gate | 컴파일된 스킬 수 | S10 |
| INV-017 | `.omp/`는 사용자 소유 루트이고 `platform add`는 omp manifest를 갱신하지 않음 | destructive-removal safety + stale-manifest ownership | Clean/remove 결과 | S13, S4, E4, E6 |
| INV-018 | `PlatformToProvider` 빈 값 시 플랫폼명으로 폴백해 provider 등록 | boundary enforcement | `orchestra.providers` | S14 |
| INV-019 | `knownCLIs`는 이름 존재만 확인하고 버전을 검증하지 않음 | identity verification | 감지된 플랫폼 목록 | S15 |

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| 규칙·에이전트·설정 방출 | REQ-002~007, T3·T4·T5·T6 | covered |
| 콘텐츠 정규화 | REQ-015, T15 | covered |
| 스킬 표면과 컴파일 게이트 | REQ-013·REQ-016, T13·T16 | covered |
| 커맨드 표면 | REQ-014, T14 | covered |
| 오류·복구 (빈 본문, 필수 필드, 미매핑 도구) | REQ-004~006, E1·E2·E5 | covered |
| 통합 경계 (공유 표면, CLAUDE.md, orchestra) | REQ-008·009·018, T7·T18 | covered |
| 데이터 안전 (사용자 파일·설정 보존) | REQ-007·017, T6·T17 | covered |
| CLI surface 10곳 | REQ-010, T1·T8 | covered |
| 검증 (오라클·라이브·파리티) | T11·T12, REQ-012/T10 | covered |
| docs/ops (생성 표면 위생, 바이너리 신원) | REQ-011·019, T9·T19 | covered |

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| None | - | 초안의 스킬·커맨드 갭은 REQ-013·REQ-014로, 리뷰가 지적한 정규화·컴파일 게이트·안전성 갭은 REQ-015~REQ-019로 승격되어 닫혔다. 1차 라이브 실측이 드러낸 규칙 노출 갭(8/14)은 옵션 B 합성으로 닫혔고 재실측 14/14로 확인했다(`## Implementation Evidence`, checked_at 2026-08-01) |

## Evolution Ideas

선택 개선이며 sync completion을 막지 않는다. SPEC ID, task ID, acceptance ID를 붙이지 않는다.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| omp orchestra provider 실행 배선 | Outcome Lock은 발견과 동작이며 provider 실행과 무관 | 사용자가 명시 요청 |
| omp worker ProviderAdapter, interactive pane, native sticky `.omp/RULES.md` 활용 | 이번 착륙 지점 밖이며 `.agents/rules/autopus/`로 발견 계약이 충족됨 | 사용자가 명시 요청 |
| `workflowSpecs` 공용 패키지 승격 | 기존 두 어댑터 회귀 위험을 끌어옴 | 네 번째 복사본이 필요해질 때 |
| Generate 쓰기 경로의 `MkdirAll`+`WriteFile`를 O_NOFOLLOW 또는 temp+rename으로 교체하고, `BackupFile`/`CreateBackupDir` 백업 목적지에도 봉쇄 적용 | 심링크 거부 가드는 이미 적용했고, 남은 TOCTOU 창과 백업 목적지 심링크 절반은 5개 어댑터가 공유하는 기존 관행이라 omp 단독 변경은 표면만 갈라놓는다(traversal 절반은 `SafePruneFilePath` 선행으로 omp에서 도달 불가) | adapter-common 쓰기 경로를 한 번에 다룰 때 |

## Sibling SPEC Decision

Decision: **none** (Sibling SPEC IDs: None). Primary SPEC 하나가 Outcome Lock을 닫는다. 태스크 20개와 신규 소스 13개로 sibling 임계(25 태스크와 40 파일 동시 초과)에 미달한다. Completion Debt는 None이므로 sibling으로 분리할 잔여 작업도 없다.

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| `pkg/config/schema.go:285`, `internal/cli/init_helpers.go:22`, `pkg/detect/detect.go:24`, `pkg/detect/runtime.go:12`, `pkg/content/skill_transformer.go:75` | existing | 각 파일 Read로 키·상수 확인 |
| 디스패치 8파일 10곳 | existing | `rg -n '"opencode"'`로 확인 |
| `pkg/adapter/adapter.go` `PlatformAdapter`(10 메서드), `registry.go`, `opencode/opencode_{rules,agents,skills,commands,lifecycle}.go`, `manifest_prune.go:10` `PruneManagedPaths` | existing | Read로 계약·변환·Clean 흐름과 백업 후 제거 동작 확인 |
| `pkg/content/skill_transformer_replace.go:25,28,90-104,195` `openCodeSkillPathRe`·`pathReplacements`·`brandingRule`; `skill_catalog_policy.go:90-95`; `skill_catalog_distribution.go:44-50,89-118` | existing | Read로 omp 키 부재, opencode 전용 2단계 스킬 경로 재작성, 기본 4타깃과 CompileTargets 게이트 확인 |
| `pkg/adapter/codex/codex.go:225` `codexOwnsSharedSurface`, `codex_standard_skills.go:38,49`, `gemini/antigravity_plugin.go:98,100,108-112` | existing | Read로 양보 조건과 `.agents/commands/*.toml` 실제 writer 확인. 108-112는 target이 아닌 content 치환임을 확인 |
| `internal/cli/platform.go:116-122`, `pkg/config/migrate.go:152-170`, `internal/cli/init.go:20-58` `gitignorePatterns` | existing | Read로 provider 폴백 등록과 gitignore 패턴 확인 |
| `pkg/adapter/parity_{acceptance,coverage,helpers}_test.go` | existing | Read로 어댑터 switch와 unknown-platform 실패 경로 확인 |
| omp 17.1.8 파서·provider·도구목록·기본 tools(`ak6`)·글롭 옵션 | existing (외부) | 바이너리 strings 추출 + 임시 워크스페이스 `omp ttsr list` 실측 |
| `[NEW] pkg/adapter/omp/*.go`(9), `[NEW] pkg/content/agent_transformer_omp.go`, `[NEW] pkg/content/rule_frontmatter_omp.go`, `[NEW] AgentRuntimeOMP`, `[NEW] ompToolMap` | [NEW] planned addition | 미존재 — 본 SPEC이 생성 |

## Prompt Layer Manifest Contract

`.autopus/omp-manifest.json`이 캐시 무효화 관측 지점이다. **stable**: 규칙 14·에이전트 16(콘텐츠 소스 변경 시에만 체크섬 변동). **snapshot**: `.omp/config.yml` 마커 섹션과 조건부 스킬·커맨드 표면(활성 플랫폼 조합에 따라 달라짐). **ephemeral**: omp 세션 런타임 컨텍스트(기록하지 않음). manifest는 경로와 체크섬만 담고 원문이나 비밀값을 담지 않는다.

## Review Response

Revision 2는 debate review 23건에 대응했다. 20건 수용, 3건 반박.
- **F-008 반박** (tools 생략이 무제한 권한을 준다): omp는 `u.tools === undefined ? ak6 : new Set(u.tools)`를 쓰고 `ak6 = new Set(["read","grep","glob"])`이다. 생략은 권한 확대가 아니라 읽기 전용 축소이며 fail-closed다. 다만 능력 손실은 실재하므로 E5로 의미를 고정했다.
- **F-019 부분 반박** (antigravity가 `.agents/skills/`를 미러링한다): 커맨드 주장은 옳아 REQ-014를 고쳤다. 스킬 주장은 틀렸다. `antigravity_plugin.go:108-112`의 `.agents/skills/`는 `rewriteAntigravityGlobalCommandContent`의 **본문 문자열 치환**이고 실제 target은 `antigravityPluginTarget`이 만드는 `.agents/plugins/autopus/skills/`다. REQ-013의 양보 대상에서 antigravity 제외를 유지한다.
- **F-023 반박** (research 363행의 9곳 서술): 개정 전 research.md는 198행이라 363행이 없다. 팀 인계의 "9곳"을 실측 "10곳"으로 정정한 문장이었고, 혼동을 없애려 그 대조 문구를 삭제하고 10곳만 남겼다.
- **F-003 수용** (규칙 네임스페이스): 팀 리드의 "flat 전용일 수 있으니 파일명 접두사를 쓰라"는 주의는 실측으로 뒤집혔다. `.agents/rules/autopus/nested-rule.md`가 실제로 발견되므로 디렉터리 네임스페이스를 쓴다.
- **Revision 3**: 재리뷰의 잔여 checklist FAIL 5건을 수용했다. 스킬 참조의 최종 경로 형태(`.agents/skills/<name>/SKILL.md`)를 REQ-015 2단계 매핑과 S3·S12 오라클에 고정했고, plan.md flowchart의 커맨드 양보 라벨을 REQ-014와 맞췄으며, 혼합 설정의 커맨드 20종 전체 발견(S11)과 omp Update 없이 소유권이 바뀐 stale manifest 상태의 Clean(E6, REQ-017)을 concrete oracle로 추가했다.

## Implementation Evidence

2026-08-01 실측, omp 17.1.8. `go build -o <scratch>/auto-dev ./cmd/auto` → `auto-dev init --dir <ws> --platforms omp --yes` → 격리 프로필(인증 없는 더미 provider `models.yml`)로 `omp --mode rpc --no-session --cwd <ws> --model s7dummy/s7-probe` 세션을 열고, 시작 시 방출되는 `available_commands_update` 프레임과 `/dump`가 반환하는 실제 시스템 프롬프트를 증거로 수집했다. 이 버전의 `/context`는 토큰 사용량만 반환하고 `omp read rule://<name>`은 `Available: none`을 반환하므로 둘 다 발견 증거로 쓸 수 없다. 1차 실측에서 규칙 8/14 FAIL이 나왔고, 아래 옵션 B 수정 후 재실측에서 14/14로 닫혔다.

- **S7 규칙 14종 — PASS(14/14, 재실측)**: `<domain-rules>` 11종과 TTSR 등록 3종(lore-commit, shell-portability, worktree-safety — 디버그 로그 `TTSR rule registered`로 확인)의 합집합이 `content/rules/` 이름 집합과 같고 두 경로 중복 등록은 0건이다. 1차 실측에서 미노출이던 branding·context7-docs·deferred-tools·doc-storage·project-identity·spec-quality 6종이 합성 description으로 노출된다.
- **폐쇄 방식(옵션 B)과 옵션 A 기각**: 대조 실험으로 omp가 frontmatter `description:`이 있는 규칙만 세션에 등록하고 trigger 키가 있으면 TTSR 엔진으로 보낸다는 것을 확정했다(사본에서 `branding.md`에 description을 넣자 `<domain-rules>`에 나타났고, `lore-commit.md`의 `condition`·`scope`를 지우자 TTSR에서 `<domain-rules>`로 옮겨갔다). 옵션 A인 `content/rules/` 소스에 description 추가는 기존 4개 플랫폼의 방출 바이트를 바꿔 `rules_golden_test.go`의 byte-identity 골든과 템플릿 재생성까지 건드리므로 기각했다. 대신 omp 전용 경로인 `TransformRuleForOMP`가 인식 키를 하나도 얻지 못했을 때만 본문 H1(없으면 첫 산문 줄)에서 description을 결정적으로 합성한다. 120 rune 상한, 인라인 마크업 평탄화, 펜스 코드 제외를 적용하고, 인식 키가 하나라도 살아남으면 합성하지 않아 trigger 규칙의 TTSR 라우팅이 바뀌지 않는다. 공유 콘텐츠는 한 바이트도 바뀌지 않았고 `TestRules_UnconditionalRulesStayByteIdentical`이 4개 플랫폼 byte-identity를 그대로 통과한다.
- **S7 나머지와 S8 — PASS**: `## Available Agents`가 ADK 16종 전부를 담고(총 20 = ADK 16 + omp 내장 designer·librarian·sonic·task), `source:"file"` 커맨드 21종이 방출 20종을 전부 포함하며(나머지 1종은 omp 내장 `init`), `<skills>` 52종에 `auto`가 있다. 방출 표면 전체에 `.claude/` 잔존 0건이고 `doc-storage`는 `.omp/`, `executor`는 `.agents/rules/autopus/branding.md`로 정규화됐다. `auto doctor`의 omp 항목 3건이 전부 `[OK]`이고 `--json`의 `data.platforms`는 `[{"name":"omp","valid":true}]`다.
- **E4 prune root 결정**: Update와 Clean이 별도 집합을 갖는 대신 `omp.PruneRoots(cfg)` 하나를 공유한다. 분기 축이 operation이 아니라 surface이기 때문이다. `.agents/commands`는 양보 후에도 항상 포함한다 — opencode는 `.opencode/commands/`로 커맨드를 제공하므로 omp가 쓴 `.md`를 아무도 덮어쓰지 않아 그대로 고아가 된다. `.agents/skills`는 `ompOwnsSharedSkillSurface(cfg)`가 참일 때만 포함한다 — opencode·codex가 `.agents/skills/auto/SKILL.md`를 제자리에서 덮어쓰므로 양보 후 그 파일은 그들 소유다. prune은 manifest 기록 경로에만 적용되어 antigravity `.toml`과 사용자 파일은 애초에 대상이 아니다. plan.md T17은 Clean·remove만 말하지만 Update도 같은 규칙을 쓴다. `internal/cli/update_preview.go`의 `previewPruneRoots`도 하드코딩을 버리고 이 함수에 위임해 preview/apply 비대칭을 없앴다.
- **파일 분할**: `pkg/content/skill_transformer_replace.go`가 326행으로 300행 한도를 넘겨 rewrite domain 기준으로 순수 이동 분할했다 — `skill_transformer_replace.go`(139, 경로와 진입점), `[NEW] skill_transformer_replace_mcp.go`(102, Context7/MCP), `[NEW] skill_transformer_replace_tools.go`(99, agent call과 도구명). 최상위 선언 22개의 이름·시그니처가 분할 전후 동일하고 exported API는 불변이다.

## Security Analysis

- **경로 봉쇄 3층**: Generate는 `adapter.SafeWorkspacePath`, Update는 transaction의 `safeTransactionPath`, Clean은 `adapter.SafePruneFilePath`로 각 경로를 워크스페이스 안으로 고정하고 빈 부모 정리는 `withinWorkspace`로 막는다. `os.Remove`가 **부모 디렉터리 심링크를 따라가** 워크스페이스 밖 파일을 삭제한 실측이 근거이며, 세 층 중 하나라도 빠지면 같은 클래스가 다시 열린다.
- **skip 의미 통일**: 심링크 성분이 하나라도 있으면 백업도 제거도 하지 않고 엔트리 전체를 건너뛴다. 백업만 막고 제거를 진행하면 체크섬 불일치 파일이 사본 없이 사라지고, `internal/cli/platform.go`가 `Clean` 오류를 버리기 때문에 그 손실이 조용하다.
- **마커 재작성**: 라인 앵커 + BEGIN/END 균형 검사 + YAML 노드 트리 검사를 함께 건다. 정상 마커는 YAML 주석이라 노드 트리에 나타나지 않으므로, 스칼라 값 안에서 마커 문자열이 발견되면 블록 스칼라 안의 텍스트라는 뜻이고 재작성을 거부한다. 파싱 불가 파일도 같은 이유로 거부한다.
- **실행 경계(REQ-019)**: 식별 게이트는 `omp --version` 실행을 수반하며 `DetectInstalledPlatforms`도 omp만 예외로 바이너리를 실행한다. prefix 검사는 **사고 방지이지 보안 통제가 아니다** — 적대적 바이너리는 임의 버전 문자열을 출력할 수 있다. 실질 완화는 `exec.LookPath`의 ErrDot이 상대 PATH 항목을 배제하는 것이며, 이후 작업이 이 검사 위에 신뢰 결정을 쌓아서는 안 된다.
- **파일 권한과 잔여 위험**: `.omp/config.yml`은 provider 자격증명을 담을 수 있어 신규 생성 시 0600이고 이후 기존 모드를 보존한다(update가 사용자의 0600을 0644로 넓히던 동작 차단). 남은 항목은 `adapter.BackupFile`·`CreateBackupDir`의 목적지 봉쇄로, 5개 어댑터 공용 코드라 adapter-common 패스로 미뤘다. traversal 절반은 omp에서 도달 불가(`SafePruneFilePath`가 `..`를 거부)이고 심링크 목적지 절반만 남는다.

## Reviewer Brief

- **Intended scope**: omp 플랫폼 착륙 — 규칙·에이전트·설정 표면, 조건부 스킬·커맨드 표면, omp 콘텐츠 정규화, 식별자 5곳과 디스패치 10곳, 그리고 데이터 안전(사용자 파일 보존·orchestra 경계·바이너리 신원).
- **Explicit non-goals**: orchestra 실행 배선, worker ProviderAdapter, interactive pane, 루트 컨텍스트 문서 신규 생성, omp native sticky 규칙. 이 항목들로 범위를 넓히지 않는다.
- **Self-verified**: REQ 19건 전건이 Traceability Matrix에서 태스크·시나리오·불변식에 연결됨. INV 19건 전건이 Must 오라클로 추적됨. 태스크 T1~T20, 신규 소스 13개(분할로 2개 추가). omp 계약은 바이너리 추출과 라이브 RPC 세션 실측이며, 2026-08-01 실측과 재실측 결과는 `## Implementation Evidence`에 있다. 라이브 단언 전건 PASS이고 Completion Debt는 None이다.
- **Reviewer should focus on**: (1) description 합성이 결정적이고 trigger 규칙의 TTSR 라우팅을 건드리지 않는지, 그리고 공유 콘텐츠 무변경이 4개 플랫폼 byte-identity를 실제로 보장하는지. (2) 소유권 판정 두 함수의 조건이 S10·S11 네 설정과 일치하는지. (3) `omp.PruneRoots(cfg)` 단일 집합이 Update와 Clean 양쪽에서 안전한지. (4) REQ-018 변경이 `cursor`에 주는 파급.

## Self-Verify Summary

- Q-CORR-01 | status: FAIL | attempt: 2 | files: plan.md, research.md | reason: 리뷰가 `ReplacePlatformReferences`·`compileTargetsForSkill`·`antigravity_plugin.go`를 반증했고 재실측으로 확인함
- Q-CORR-01 | status: PASS | attempt: 3 | files: 4개 전부 | reason: 인용한 모든 기존 경로·심볼을 재확인하고 정규화 no-op, 카탈로그 게이트, `.agents/commands/` writer를 실측값으로 반영함
- Q-CORR-02 | status: PASS | attempt: 3 | files: spec.md, research.md | reason: 신규 파일 11개와 상수·매핑을 `[NEW]`로 표기함
- Q-CORR-03 | status: PASS | attempt: 3 | files: spec.md, acceptance.md | reason: EARS 19건이 파서 인식 형식이고 acceptance는 bare Given/When/Then/And만 사용함
- Q-CORR-04 | status: PASS | attempt: 3 | files: research.md | reason: Reference Discipline이 existing과 `[NEW]`를 분리하고 content 치환과 target 경로를 구분해 기록함
- Q-COMP-01 | status: FAIL | attempt: 3 | files: plan.md | reason: flowchart가 antigravity 활성 시 커맨드 양보로 표시해 소유권 표·REQ-014·T14·S11과 모순됨
- Q-COMP-01 | status: PASS | attempt: 4 | files: plan.md | reason: 분기 라벨을 opencode 단일 조건으로 고치고 antigravity 공존을 라벨에 명시함
- Q-COMP-02 | status: PASS | attempt: 4 | files: spec.md, acceptance.md | reason: REQ-001~019 전건 연결에 더해 혼합 설정 커맨드 20종 전체 발견(S11)과 stale manifest 전환(E6)까지 acceptance가 끝까지 검증함
- Q-COMP-03 | status: PASS | attempt: 3 | files: spec.md | reason: 각 REQ가 EARS type·Priority·관측 지점을 별도 메타 라인으로 가짐
- Q-COMP-04 | status: PASS | attempt: 4 | files: research.md, spec.md, acceptance.md | reason: REQ-015~019 승격에 더해 혼합 경로 오라클(S10·S11·E6)을 채워 Primary SPEC만으로 완료 증거가 닫힘
- Q-COMP-04 | status: FAIL | attempt: 5 | files: research.md | reason: 2026-08-01 라이브 실측에서 S7 규칙 14종 발견이 8/14로 나와 Outcome Lock이 아직 닫히지 않음. Completion Debt를 None에서 실제 항목으로 바꾸고 `## Implementation Evidence`에 근거와 해소 조건을 기록함
- Q-COMP-04 | status: PASS | attempt: 6 | files: research.md, acceptance.md, plan.md | reason: omp 전용 description 합성(옵션 B)으로 규칙 노출 갭을 닫고 같은 절차로 재실측해 14/14를 확인함. Completion Debt는 다시 None이고 E1 시나리오는 새 계약으로 갱신됨
- Q-COMP-05 | status: FAIL | attempt: 3 | files: acceptance.md | reason: INV-015 오라클이 `.agents/skills/` 부분문자열까지만 단언해 `.agents/skills/ax-annotation.md`라는 없는 경로를 통과시킴
- Q-COMP-05 | status: PASS | attempt: 4 | files: spec.md, plan.md, acceptance.md | reason: S3가 `.agents/skills/ax-annotation/SKILL.md`와 `.agents/skills/frontend-verify/SKILL.md` 최종 형태를 정확 문자열로 고정하고 S12가 `<name>.md` 형태 잔존 0건을 단언하며 INV-007·012·013도 S10·S11·E6로 닫힘
- Q-COMP-06 | status: PASS | attempt: 3 | files: spec.md, research.md | reason: Reviewer Brief 수치를 REQ 19·INV 19·T20·신규 11로 맞추고 focus 4항목으로 범위를 제한함
- Q-COMP-07 | status: PASS | attempt: 6 | files: research.md | reason: 규칙 노출 항목이 해소되어 Completion Debt는 다시 None이고, Evolution Ideas 4건은 ID 미부여 선택 개선으로만 남음
- Q-FEAS-01 | status: PASS | attempt: 3 | files: plan.md | reason: 런타임 Go 변경임을 명확히 하고 신규 11파일·기존 13곳 수정으로 범위를 특정함
- Q-FEAS-02 | status: PASS | attempt: 3 | files: plan.md | reason: 편집 대상이 source of truth인 `content/`·`pkg/`·`internal/`이며 생성 표면과 구분됨
- Q-FEAS-03 | status: PASS | attempt: 3 | files: acceptance.md | reason: go test·auto doctor·auto platform·omp RPC·`omp ttsr list` 모두 로컬 실행 가능함을 이번 개정에서 실제로 사용해 확인함
- Q-STYLE-01 | status: PASS | attempt: 3 | files: spec.md | reason: REQ 본문에 모호어가 없음
- Q-STYLE-02 | status: PASS | attempt: 3 | files: spec.md | reason: Priority는 Must·Should만 쓰고 EARS type과 별도 축으로 표기함
- Q-STYLE-03 | status: PASS | attempt: 3 | files: acceptance.md | reason: 모든 step이 bare Gherkin이고 문장이 완결됨
- Q-SEC-01 | status: PASS | attempt: 3 | files: research.md | reason: ledger 셀과 source clause를 untrusted evidence로 명시했고, 규칙·에이전트가 세션 프롬프트에 주입되는 신뢰 경계를 REQ-008과 REQ-015로 다룸
- Q-SEC-02 | status: PASS | attempt: 3 | files: spec.md, acceptance.md | reason: 비밀값을 다루지 않고, REQ-017이 사용자 소유 `.omp/` 데이터 파괴를 막으며 체크섬 불일치 파일을 백업함
- Q-SEC-03 | status: PASS | attempt: 3 | files: spec.md | reason: manifest는 경로와 체크섬만 담고 REQ-011이 발견을 깨지 않는 디렉터리형 패턴만 추가해 diff noise와 발견 차단을 동시에 피함
- Q-COH-01 | status: PASS | attempt: 3 | files: spec.md | reason: omp 플랫폼 착륙 하나의 변경 스토리로 수렴함
- Q-COH-02 | status: PASS | attempt: 6 | files: research.md | reason: 규칙 노출 갭을 같은 iteration 안에서 실제로 닫고 재실측으로 확인해 필수 후속 작업이 없으며, 선택 개선만 Evolution Ideas에 남음
- Q-COH-03 | status: PASS | attempt: 3 | files: research.md | reason: sibling SPEC 없음을 근거와 함께 기록함
