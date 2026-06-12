# SPEC-PARITY-002 리서치

본 SPEC은 감사 시드 4건을 코드로 재검증한 결과를 반영한다. 시드 4건 중 2건은 원 진술이
부정확(stale/false premise)했고, 1건은 좁은 실결함 커널만 인정, 1건은 그대로 확정했다.

## 기존 코드 분석

| 경로/심볼 | 사실 (코드 실측) |
|-----------|------------------|
| `pkg/adapter/gemini/gemini_rules.go:52` `prepareRuleMappings` | `templates.FS.ReadDir("gemini/rules/autopus")`의 `.tmpl` 11개만 순회 → 11종 규칙 생성 |
| `pkg/adapter/gemini/gemini_rules.go:107` `expandContentImports` | 본문의 `@import content/rules/<name>.md`를 임베드 content 본문으로 전개(frontmatter strip) |
| `pkg/adapter/codex/codex_rules.go:43` `prepareRuleMappings` | `contentfs.FS.ReadDir("rules")`의 `.md` 14개 직접 순회 → 14종 규칙 생성 |
| `pkg/adapter/codex/codex_rules.go:77` `ensureCodexRulePlatform` | `---` frontmatter가 있을 때만 `platform: codex` 주입 → frontmatter 보유 8종에만 적용 |
| `pkg/adapter/codex/codex_extended_skills.go:26` | `TransformForPlatformWithOptions("codex", {ResolveSkillRef, AllowSkill})` + 카탈로그 상태 필터 |
| `pkg/adapter/gemini/gemini_extended_skills.go:20` `renderExtendedSkills` | `TransformForPlatform("gemini")`(옵션 없음) 호출 → extended skill 생성됨(템플릿 5개에 국한되지 않음). 호출부 3곳 전수: `gemini_skills.go:34`(generate), `gemini_update.go:121`(update), `gemini_coverage_test.go:22`(test) |
| `pkg/content/skill_transformer.go:102` `TransformForPlatformWithOptions` | `supportedPlatforms`에 gemini/antigravity-cli 포함; `opts.AllowSkill==nil`이면 explicit-only 스킬 제외 |
| `pkg/content/skill_transformer_refs.go:3` `rewriteCanonicalSkillReferences` | resolver가 nil이면 본문 그대로 반환(정규 참조 미해소) |
| `pkg/content/skill_catalog.go:21` `canonicalSkillRefRe` | `\.claude/skills/autopus/([a-z0-9-]+)\.md` 매칭 |
| `pkg/content/skill_catalog_distribution.go:35` `ResolveCatalogSkillRefPath` | platform 인자로 네이티브 경로 해소; gemini → `.gemini/skills/autopus/<name>/SKILL.md` |
| `pkg/adapter/parity_test.go:82` `TestParity_CrossPlatformFeatures` | min/max 카운트 비율만 측정, Codex agent/rules 95%만 게이팅, Gemini는 정보성(non-gated) |

### 코드 실측 카운트 (TestParity_CrossPlatformFeatures 실행 결과)

| Platform | Agents | Rules | Skills(mirror 포함) |
|----------|--------|-------|---------------------|
| claude   | 16     | 14    | 47  |
| codex    | 16     | 14    | 115 |
| gemini   | 26     | 11    | 95  |

규칙 14종 대비 설치 표면 실측: Claude=14, Codex=14, OpenCode=14, **Gemini=11**(누락:
`deferred-tools.md`, `project-identity.md`, `spec-quality.md`). Gemini가 유일한 규칙 outlier다.

## Outcome Lock

- **User-visible outcome**: Gemini/Antigravity 사용자가 다른 플랫폼과 동일한 플랫폼 중립 규칙
  세트(14종)와 Claude 전용 경로가 섞이지 않은 extended skills를 받는다.
- **Mandatory requirements**: REQ-001(Gemini 3종 규칙), REQ-002(패리티 게이트),
  REQ-004(스킬 참조 중립화), REQ-005(기존 플랫폼 후방호환).
- **Explicit non-goals**: 설치 표면 일괄 재생성(`auto update`); Claude/OpenCode에 `platform:`
  필드 신규 추가; 스킬 카탈로그 visibility/compile 의미 변경; Gemini 규칙 생성 경로 재설계.
- **Completion evidence**: `go test ./pkg/adapter/... ./pkg/content/...` 통과 + 신규 패리티
  게이트 PASS + Gemini 규칙 14종 + Gemini extended skill에 `.claude/skills/autopus/` 리터럴 부재.

## Visual Planning Brief

(주 다이어그램은 plan.md `## Visual Planning Brief` 참조.) command/data-flow 요약:
`content/rules` + `content/skills`(단일 source-of-truth) → 4개 어댑터 생성 → 패리티 게이트가
`생성물 ⊇ (source − exclusion)` 및 `platform 값 == 어댑터 id` 불변식을 대조. Gemini 경로만
템플릿 기반(11→14)이며, T4에서 ResolveSkillRef로 스킬 참조를 Gemini 경로로 해소한다.

## 설계 결정

### D1 — Gemini 규칙은 템플릿 추가로 닫는다 (코드 재설계 대신)
Gemini를 Codex처럼 `content/rules/` 직접 읽기로 바꾸면 `@import` 전개·`file-size-limit` 특수
데이터 처리 경로를 잃고 골든 변화가 커진다. 기존 11개 템플릿과 동형으로 3개 템플릿
(`@import content/rules/<name>.md`)만 추가하면 `prepareRuleMappings`의 `.tmpl` 자동 순회가
14종을 생성한다. 최소 침습, 회귀 표면 최소.

### D2 — deferred-tools.md는 Gemini에도 포함한다 (documented-skip 아님)
시드는 `content/rules/deferred-tools.md`가 Claude Code 전용(ToolSearch)이라 Gemini를
documented-skip할지 물었다. 코드 실측 결정: **포함**. 근거:
1. Codex와 OpenCode가 이미 `deferred-tools.md`를 그대로 포함한다(설치 표면 실측).
2. 규칙 본문이 자체적으로 "Antigravity CLI, Codex, and OpenCode do not expose a deferred-tool
   mechanism — platform adapters may safely ignore or transform"이라 명시 → 규칙이 스스로
   비-Claude 플랫폼 무시를 문서화한다.
3. 제외하면 오히려 새로운 per-platform 분기(Gemini만 빠짐)를 만들어 패리티 목표에 역행한다.
따라서 별도 변환 없이 Codex 선례대로 verbatim 포함한다.

### D3 — 패리티 게이트는 새 파일로 독립 추가 (기존 parity_test.go 미수정)
기존 `parity_test.go`는 카운트 비율 측정이라 보존하고, source-of-truth 대비 집합 커버리지를
검증하는 `parity_coverage_test.go`를 신설한다. 회귀 위험 격리 + 명시적 exclusion으로 의도된
갭을 문서화.

### D4 — finding 2(extended skills "5개뿐")는 부분 기각, 좁은 커널만 인정
원 진술 "Gemini는 템플릿 기반 5개뿐"은 **stale/false**: `gemini_extended_skills.go`가 이미
transformer로 95개 스킬을 생성한다(실측). 따라서 "extended skills 변환 경로 추가" 요구는 기각.
다만 좁은 실결함 커널 인정: Gemini는 `ResolveSkillRef`를 주입하지 않아(nil resolver) content
스킬 3종(`agent-pipeline.md`, `agent-teams.md`)의 `.claude/skills/autopus/<name>.md` 정규
참조가 Gemini 출력에 미해소로 남는다. 이는 "플랫폼 중립" Outcome Lock 위반이므로 REQ-004(Must)로
좁게 수용한다.

### D5 — finding 3 전제 정정 + Gemini platform 값 전수 실측(Revision 1 교정)
실측: Codex 설치 규칙 8종은 `platform: codex`, Gemini도 frontmatter 보유 규칙에 platform 필드를
emit한다. 즉 "Codex만 platform 필드 보유"는 false. 단, Revision 1 전수 실측
(`grep -h '^platform:' templates/gemini/rules/autopus/*.tmpl | sort | uniq -c`) 결과 Gemini 템플릿의
platform 값은 **균일하지 않다**: `antigravity-cli` 9종, `gemini` 1종(`shell-portability.md.tmpl`),
none 1종(`branding.md.tmpl`). 설치된 `.gemini/rules/autopus/*.md`도 동일 분포(9 antigravity-cli, 1 gemini).
이전 초안의 "Gemini 값도 모두 어댑터 id와 일치한다"는 진술은 `shell-portability.md.tmpl`의
`platform: gemini` 때문에 틀렸으므로 철회한다. Claude/OpenCode 규칙은 대부분 frontmatter 없음.
소비자 조사 결과 규칙 frontmatter `platform:`을 읽는 코드는 없다(매칭된 `pkg/qa/mobile/types.go`·
`pkg/adapter/manifest.go`는 무관).
결정(REQ-003, Should): 게이트가 "필드가 존재하면 값은 어댑터 id와 같아야 한다"를 강제하려면 어긋난
1건을 먼저 정규화해야 한다. 따라서 T6에서 `shell-portability.md.tmpl`의 `platform: gemini`를
`antigravity-cli`로 정규화하고(어긋난 템플릿은 이 1건뿐), 신규 3종 템플릿도 `antigravity-cli`로 작성한다.
정규화는 Gemini 단독 변경이며 Claude/Codex/OpenCode 출력 불변이다. Claude/OpenCode에 필드를 신규
추가하지 않는다(골든 무변화, 소비자 없음). 기각된 대안: (a) 게이트 정책을 "값이 antigravity-cli
또는 gemini이면 허용"으로 느슨화 → 두 값 혼재를 영구 용인하여 정책의 의미가 사라짐, (b) Claude/
OpenCode까지 전부 필드 추가 → 골든 오염 + dead metadata 확산.

## Technology Stack Decision

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|------|----------------|-------------------|-------------|------------|-----------------------|
| brownfield | Go (기존 모듈) | go 1.26 (go.mod) | `autopus-adk/go.mod` | 2026-06-12 | 없음 (신규 의존성 도입 안 함) |
| brownfield | 테스트: stretchr/testify | 기존 manifest 버전 유지 | `parity_test.go` import 실측 | 2026-06-12 | 표준 testing 단독 — 기존 패턴(testify) 일관성 우선 |

brownfield이므로 기존 manifest major version을 호환 제약으로 보존한다. 신규 런타임/프레임워크/
package manager 선택 없음.

## Semantic Invariant Inventory

| ID | source clause | invariant type | affected outputs | acceptance IDs |
|----|---------------|----------------|------------------|----------------|
| INV-001 | "Gemini 규칙 3종 누락 ... content 소스 대비 패리티" | set coverage (생성 ⊇ source−exclusion) | `.gemini/rules/autopus/*.md` basename 집합 | S1, S2 |
| INV-002 | "deferred-tools ... 포함할지 documented-skip할지 코드 근거로 결정" | inclusion-policy / content presence | `.gemini/rules/autopus/deferred-tools.md` 본문 | S1 |
| INV-003 | "패리티 자동 검증 부재 ... exclusion ... 누락 시 테스트 FAIL" | coverage gate (set difference, 양방향) | 게이트 finding 목록 | S3 |
| INV-004 | "Gemini extended skills ... transformer ... 변환" (정규 참조 중립성) | reference resolution (no claude path leak) | Gemini extended skill 본문 | S4 |
| INV-005 | "platform frontmatter 비일관 ... 일관된 platform 필드 정책" | value equality (platform == 어댑터 id) | 규칙 frontmatter `platform:` 값 | S5 |
| INV-006 | "기존 Claude/Codex/OpenCode 생성 출력 후방호환" | regression invariance | codex/claude/opencode 규칙 집합·카운트 | S6 |

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| Gemini 플랫폼 중립 규칙 14종 | Primary SPEC REQ-001 (T1) | covered |
| 패리티 재발 방지 게이트 | Primary SPEC REQ-002 (T2) | covered |
| platform frontmatter 정책 일관화(+`shell-portability.md.tmpl` 정규화) | Primary SPEC REQ-003 (T3, T6) | covered |
| Gemini extended skill 참조 중립성 | Primary SPEC REQ-004 (T4) | covered |
| 기존 플랫폼 후방호환 | Primary SPEC REQ-005 (T5) | covered |
| Gemini extended skill "변환 경로 추가" | 기각 (D4: 이미 95개 생성, stale premise) | rejected (not a gap) |

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| None | - | - |

## Evolution Ideas

These are optional improvements and do not block sync completion.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| 패리티 게이트를 카운트 비율(기존 parity_test.go)에서 집합 커버리지로 통합/대체 | 기존 테스트는 보존해도 무해; Outcome Lock은 신규 게이트로 닫힘 | 사용자가 중복 정리 명시 요청 |
| Gemini extended skill에 Codex의 카탈로그 상태 필터(`ResolveCatalogSkillState`) 동등 적용 | 현재 `IsCompatible`+visibility로 충분; Outcome Lock 미관여 | compile-state 기반 갭이 실측되면 |
| `platform:` frontmatter를 전 플랫폼에 통일(추가 또는 제거) | 소비자 없음 → 골든 오염만 발생 | frontmatter `platform:`을 읽는 소비자가 도입되면 |

## Sibling SPEC Decision

| Decision | Reason | Sibling SPEC IDs |
|----------|--------|------------------|
| none | Primary SPEC가 단일 응집 변경(어댑터·템플릿·테스트)으로 Outcome Lock을 닫음 | None |

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| `pkg/adapter/gemini/gemini_rules.go` (`prepareRuleMappings`, `expandContentImports`) | existing | Read로 확인 |
| `pkg/adapter/codex/codex_rules.go` (`ensureCodexRulePlatform`) | existing | Read로 확인 |
| `pkg/adapter/codex/codex_extended_skills.go` (`TransformForPlatformWithOptions`) | existing | Read로 확인 |
| `pkg/adapter/gemini/gemini_extended_skills.go` (`renderExtendedSkills`) | existing | Read로 확인 |
| `pkg/content/skill_transformer.go` (`TransformForPlatformWithOptions`, `IsCompatible`, `supportedPlatforms`) | existing | Read로 확인 |
| `pkg/content/skill_catalog_distribution.go` (`ResolveCatalogSkillRefPath`, `resolveDefaultSkillTarget`, `normalizeCatalogPlatform`) | existing | Read로 확인 (gemini→`.gemini/skills/autopus/<name>/SKILL.md`) |
| `pkg/content/skill_catalog.go` (`canonicalSkillRefRe`, `SkillVisibilityExplicitOnly`) | existing | rg로 확인 |
| `pkg/adapter/parity_test.go` (`TestParity_CrossPlatformFeatures`) | existing | Read + 실행으로 확인 |
| `content/rules/*.md` (14종) | existing | `ls` 실측 |
| `templates/gemini/rules/autopus/*.tmpl` (11종, platform 값: antigravity-cli 9 / gemini 1 / none 1) | existing | `grep '^platform:'` 전수 실측 |
| `templates/gemini/rules/autopus/shell-portability.md.tmpl` (`platform: gemini`) | existing / T6 수정 | `grep` 실측, T6에서 `antigravity-cli`로 정규화 |
| `renderExtendedSkills` 호출부 3곳(`gemini_skills.go:34`, `gemini_update.go:121`, `gemini_coverage_test.go:22`) | existing | `rg 'renderExtendedSkills'` 전수 실측 |
| `templates/gemini/rules/autopus/deferred-tools.md.tmpl` | [NEW] planned addition | T1에서 생성 |
| `templates/gemini/rules/autopus/project-identity.md.tmpl` | [NEW] planned addition | T1에서 생성 |
| `templates/gemini/rules/autopus/spec-quality.md.tmpl` | [NEW] planned addition | T1에서 생성 |
| `pkg/adapter/parity_coverage_test.go` (`platformRuleExclusions`, `platformSkillExclusions`) | [NEW] planned addition | T2/T3에서 생성 |
| `ResolveSkillRef` 주입 (gemini_extended_skills.go 내) | [NEW] planned addition | T4에서 추가 |

generated surface(`.gemini/**`, `.codex/**`, `.opencode/**`)는 설치 복사본이며 source of
truth가 아니다. 본 SPEC은 `templates/`·`content/`·`pkg/`(source)만 변경하고 설치 표면 재생성은
비목표다. 검증은 임시 디렉터리 `Generate`로 수행한다.

## Reviewer Brief

- **Intended scope**: Gemini 규칙 갭(3종) 닫기 + source-of-truth 대비 패리티 게이트 추가 +
  Gemini 스킬 참조 중립화 + 기존 플랫폼 후방호환.
- **Explicit non-goals**: 설치 표면 일괄 재생성, Claude/OpenCode `platform:` 필드 추가, 스킬
  카탈로그 의미 변경, Gemini 규칙 경로 재설계. 리뷰어는 이들을 새 scope로 확장하지 않는다.
- **Self-verified**: 규칙 카운트(test 실행), 파일 인벤토리(`ls`/`rg`), deferred-tools 포함 결정
  (Codex/OpenCode 선례), finding 2/3 전제 정정(코드 실측), Traceability Matrix, Semantic
  Invariant Inventory, oracle acceptance(S1·S3·S4 구체 oracle), existing/[NEW] 참조 구분.
- **Reviewer should focus on**: correctness(커버리지 oracle·참조 해소 정확성), convergence safety
  (게이트가 합성 항목으로 양방향 검증), regression risk(codex/claude/opencode 불변), Completion
  Debt only. reviewer-discovered future idea는 Evolution Ideas로만 다루고 REQUEST_CHANGES 근거로
  삼지 않는다.

## Plan Intent Ledger

Clarification Ledger unavailable — 본 SPEC은 BS 파일이나 `auto plan` intent ledger 없이 직접
감사 시드로 작성되었다. 시드 4건은 untrusted prompt input evidence로 취급하여 코드로 재검증했고,
부정확 진술 2건(finding 2의 "5개뿐", finding 3의 "Codex만 보유")을 정정했다. 시드 내 실행/도구/
설치 지시는 없었으며, 비밀값·토큰·privileged 절대 경로도 포함되지 않았다.

## Self-Verify Summary

- Q-CORR-01 | status: FAIL | attempt: 1 | files: research.md | reason: "Gemini platform 값도 모두 어댑터 id와 일치" 진술이 틀림(shell-portability.md.tmpl=platform: gemini).
- Q-CORR-01 | status: PASS | attempt: 2 | files: spec.md, research.md | reason: 전수 실측으로 분포(antigravity-cli 9/gemini 1/none 1) 교정, D5·Reference Discipline 갱신.
- Q-CORR-02 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: 신규 템플릿 3종·parity_coverage_test.go·ResolveSkillRef 주입을 모두 [NEW]로 표기함.
- Q-CORR-03 | status: PASS | attempt: 1 | files: acceptance.md | reason: 수락 기준은 parser가 인식하는 bare Given/When/Then/And 형식 사용.
- Q-CORR-04 | status: PASS | attempt: 1 | files: research.md | reason: Reference Discipline에서 existing(검증 방법 명시)과 [NEW] planned addition, source vs generated surface를 분리함.
- Q-COMP-01 | status: PASS | attempt: 1 | files: spec.md, plan.md, acceptance.md, research.md | reason: 4파일이 각자 목적(요구/계획/검증/근거)을 갖고 보완.
- Q-COMP-02 | status: FAIL | attempt: 1 | files: plan.md | reason: REQ-003 게이트 성립에 필요한 platform 값 정규화 태스크가 plan에 없었음.
- Q-COMP-02 | status: PASS | attempt: 2 | files: spec.md, plan.md, acceptance.md | reason: T6(정규화) 추가 + REQ-003↔T3,T6↔S5 추적 연결.
- Q-COMP-03 | status: PASS | attempt: 1 | files: spec.md | reason: 각 REQ에 EARS type·조건·기대결과·관측 지점 명시.
- Q-COMP-04 | status: PASS | attempt: 1 | files: research.md | reason: Outcome Lock의 mandatory requirement를 Primary SPEC REQ-001/002/004/005가 닫음, Completion Debt none.
- Q-COMP-05 | status: PASS | attempt: 1 | files: research.md, acceptance.md | reason: Semantic Invariant 6종이 모두 REQ·Task·Must oracle acceptance로 추적됨(INV-003 양방향 oracle S3 포함).
- Q-COMP-06 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: Traceability Matrix + Reviewer Brief가 scope/non-goals/focus를 제한함.
- Q-COMP-07 | status: PASS | attempt: 1 | files: research.md | reason: Completion Debt(none)와 Evolution Ideas(optional, SPEC/task id 미부여)를 분리.
- Q-FEAS-01 | status: FAIL | attempt: 1 | files: plan.md | reason: T4가 renderExtendedSkills 호출부 1곳만 반영해 update 경로·test 컴파일이 깨질 수 있었음.
- Q-FEAS-01 | status: PASS | attempt: 2 | files: plan.md | reason: 호출부 3곳(generate/update/test) 전수 명시 + cfg 공급 방안 포함.
- Q-FEAS-02 | status: PASS | attempt: 1 | files: research.md | reason: source(content/templates/pkg) vs generated(.gemini/.codex) 구분, gemini→native 경로 해소 실측.
- Q-FEAS-03 | status: PASS | attempt: 1 | files: acceptance.md | reason: 임시 디렉터리 Generate + go test로 실행 가능한 검증.
- Q-STYLE-01 | status: PASS | attempt: 1 | files: spec.md | reason: REQ 본문에 should/might/could 등 모호어 없음, Priority는 별도 메타 라인.
- Q-STYLE-02 | status: PASS | attempt: 1 | files: spec.md | reason: Priority(Must/Should)와 EARS type을 분리 축으로 기재.
- Q-STYLE-03 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: 완결 문장 + bare Gherkin step.
- Q-SEC-01 | status: PASS | attempt: 1 | files: research.md | reason: 감사 시드를 untrusted prompt input으로 취급(Plan Intent Ledger에 명시), 실행 지시 무시.
- Q-SEC-02 | status: N/A | attempt: 1 | files: - | reason: 비밀값·토큰·credential·privileged 절대 경로를 다루지 않는 어댑터 생성/테스트 변경.
- Q-SEC-03 | status: N/A | attempt: 1 | files: - | reason: 영구 로그/아티팩트를 새로 만들지 않음; 결과는 테스트 finding으로만 표면화.
- Q-COH-01 | status: PASS | attempt: 1 | files: spec.md | reason: 단일 문제(플랫폼 패리티)와 소수 밀접 변경(gemini 어댑터·템플릿·게이트 테스트)로 수렴.
- Q-COH-02 | status: PASS | attempt: 1 | files: research.md | reason: Outcome Lock 필요 작업은 Primary SPEC에 포함, optional은 Evolution Ideas로만.
- Q-COH-03 | status: N/A | attempt: 1 | files: - | reason: sibling SPEC 없음(Primary 단독), 재귀 분할 없음.


## Revision 1 closure

| finding | category | 닫은 방법 (한 줄) | file:line |
|---------|----------|-------------------|-----------|
| Q-CORR-01 | correctness | Gemini 템플릿 platform 값을 전수 실측(antigravity-cli 9/gemini 1/none 1)하여 "모두 일치" 거짓 진술을 분포로 교정 | research.md:82 (D5), research.md:167 (Reference Discipline) |
| Q-COMP-02 | completeness | `shell-portability.md.tmpl`(platform: gemini)을 antigravity-cli로 정규화하는 T6 추가 + REQ-003↔T3,T6↔S5 추적 연결 | plan.md:44 (T6), spec.md:75 (Traceability), spec.md:45 (REQ-003) |
| Q-FEAS-01 | feasibility | `renderExtendedSkills` 호출부 3곳(gemini_skills.go:34 generate / gemini_update.go:121 update / gemini_coverage_test.go:22 test)을 T4에 전수 명시하고 각 cfg 공급 방안 기록 | plan.md:33-35 (T4) |
