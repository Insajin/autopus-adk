# SPEC-PARITY-002: 플랫폼 어댑터 패리티 보강 (Gemini 규칙 갭 + 패리티 자동 검증)

**Status**: completed
**Created**: 2026-06-12
**Domain**: PARITY

## 목적

플랫폼 어댑터가 동일한 `content/` 소스(source-of-truth)에서 규칙·스킬을 생성하지만,
Gemini(Antigravity CLI) 어댑터만 규칙 생성 경로가 달라 일부 규칙이 누락된다.
감사 결과 Gemini는 14개 중 11개 규칙만 생성하며 `deferred-tools.md`, `project-identity.md`,
`spec-quality.md`가 빠져 있다(Claude·Codex·OpenCode는 14개 전부 생성). 원인은
`pkg/adapter/gemini/gemini_rules.go`의 `prepareRuleMappings`가 `templates.FS`의
Gemini 전용 템플릿 디렉터리(11개 `.tmpl`)만 읽는 반면, Codex·OpenCode는 `content/rules/`를
직접 열어 14개 전부를 생성하기 때문이다.

추가로, 이런 누락이 구조적으로 재발하지 않도록 source-of-truth 대비 플랫폼 커버리지를
강제하는 패리티 게이트 테스트가 필요하다. 기존 `pkg/adapter/parity_test.go`는 min/max 카운트
비율만 측정하고 Codex만 게이팅하며 Gemini는 정보성(non-gated)이라 누락을 막지 못한다.

## Outcome Boundary

- **Outcome Lock**: "Gemini/Antigravity 사용자가 플랫폼 중립 규칙·extended skills를 받으며,
  content 소스 대비 플랫폼 패리티가 테스트로 강제된다."
- **Mandatory requirements**: REQ-001(Gemini 3종 규칙 생성), REQ-002(패리티 커버리지 게이트),
  REQ-004(Gemini extended skill의 Claude 전용 경로 참조 해소), REQ-005(기존 플랫폼 출력 후방호환).
- **Explicit non-goals**:
  - 설치된 생성 표면(`.gemini/**`, `.codex/**`, `.opencode/**`)의 일괄 재생성(`auto update`)
    — 워킹트리에 무관한 prune 드리프트가 공존하므로 이 SPEC과 섞지 않는다.
  - Claude·OpenCode 규칙에 `platform:` frontmatter 필드 신규 추가 — 소비자가 없고 골든만 오염된다.
  - 스킬 카탈로그의 visibility/compile 의미(semantics) 변경.
  - Gemini 어댑터를 템플릿 방식에서 `content/rules/` 직접 읽기 방식으로 재설계하는 리팩터링.
- **Completion evidence**: `go test ./pkg/adapter/... ./pkg/content/...` 통과 + 신규 패리티 게이트
  PASS + Gemini 규칙 카운트 14 + Gemini extended skill 출력에 `.claude/skills/autopus/` 리터럴 부재.

## Requirements

### Event-Driven / Priority: Must (REQ-001 — Gemini 누락 규칙 3종 생성)
WHEN 전체(full) 구성으로 Gemini 어댑터(`pkg/adapter/gemini/gemini_rules.go`의 `prepareRuleMappings`)가 규칙 파일을 생성하면 THEN THE SYSTEM SHALL `.gemini/rules/autopus/deferred-tools.md`·`project-identity.md`·`spec-quality.md`를 생성하여 Gemini 규칙 집합이 `content/rules/` 소스 14종과 정확히 일치하게 한다. 관측 지점은 임시 디렉터리에서 `gemini.Generate` 실행 후 생성된 규칙 파일 basename 집합과 deferred-tools 본문 내용(S1, S2)이다.

### Event-Driven / Priority: Must (REQ-002 — 패리티 커버리지 게이트)
WHEN 플랫폼 패리티 게이트 테스트가 실행되면 THEN THE SYSTEM SHALL 각 플랫폼의 생성된 규칙·extended skill 집합을 `content/rules/`·`content/skills/` 소스 집합에서 해당 플랫폼의 명시적 exclusion 목록을 뺀 집합과 비교하고, exclusion에 없는 소스 항목이 생성 출력에서 누락되면 누락 항목 이름과 플랫폼 이름을 담은 finding과 함께 실패한다. 관측 지점은 게이트가 반환하는 누락 finding 목록(S2, S3)이다.

### Ubiquitous / Priority: Should (REQ-003 — platform frontmatter 값 정책 일관화)
THE SYSTEM SHALL 생성된 규칙 파일이 `platform:` frontmatter 필드를 포함하면 그 값을 생성 어댑터의 플랫폼 식별자(Codex는 `codex`, Gemini는 `antigravity-cli`)로 고정하고, 어긋난 기존 템플릿(현재 `shell-portability.md.tmpl`의 `platform: gemini` 1건)을 `antigravity-cli`로 정규화하며, 어떤 생성 규칙이 자신의 어댑터 식별자와 다른 platform 값을 선언하면 패리티 게이트가 불일치 finding을 보고하게 한다. 관측 지점은 정규화 후 게이트의 platform-value 불일치 finding 수(S5)이다.

### Event-Driven / Priority: Must (REQ-004 — Gemini extended skill의 Claude 전용 참조 해소)
WHEN Gemini 어댑터가 extended skill을 변환하면 THEN THE SYSTEM SHALL 정규 참조 `.claude/skills/autopus/<name>.md`를 Gemini 네이티브 스킬 경로로 해소하여 생성된 어떤 Gemini 스킬 본문에도 `.claude/skills/autopus/` 리터럴 참조가 남지 않게 한다. 관측 지점은 Gemini로 생성된 `agent-pipeline` 스킬 본문 문자열(S4)이다.

### Ubiquitous / Priority: Must (REQ-005 — 기존 플랫폼 출력 후방호환)
THE SYSTEM SHALL 기존 Claude·Codex·OpenCode의 생성 규칙 집합을 변경 없이 보존하여 Codex의 frontmatter 보유 규칙 8종에 대한 `platform: codex` 필드와 각 플랫폼의 14종 규칙 커버리지를 유지한다. 관측 지점은 변경 전후 Codex·Claude·OpenCode 규칙 집합·카운트 비교(S6)이다.

## Acceptance Criteria

수락 시나리오 S1~S6은 `acceptance.md`에 bare Given/When/Then 형식으로 정의되며, Must 시나리오는
concrete expected output(정확한 집합·문자열·카운트)을 Then에 명시한다.

## 생성 파일 상세

| 파일/심볼 | 유형 | 역할 |
|-----------|------|------|
| `templates/gemini/rules/autopus/deferred-tools.md.tmpl` | [NEW] 템플릿 | `@import content/rules/deferred-tools.md` 전개 + `platform: antigravity-cli` frontmatter |
| `templates/gemini/rules/autopus/project-identity.md.tmpl` | [NEW] 템플릿 | `@import content/rules/project-identity.md` 전개 |
| `templates/gemini/rules/autopus/spec-quality.md.tmpl` | [NEW] 템플릿 | `@import content/rules/spec-quality.md` 전개 |
| `templates/gemini/rules/autopus/shell-portability.md.tmpl` | 기존 수정 | `platform: gemini` → `antigravity-cli` 정규화 (값 정책 일관화) |
| `pkg/adapter/parity_coverage_test.go` | [NEW] 테스트 | source-of-truth 대비 커버리지 게이트 + platform-value 검증 |
| `pkg/adapter/gemini/gemini_extended_skills.go` | 기존 수정 | `TransformForPlatformWithOptions`로 전환하여 `ResolveSkillRef` 주입 |

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-001 | T1 | S1, S2 | INV-001, INV-002 |
| REQ-002 | T2, T3 | S2, S3 | INV-001, INV-003 |
| REQ-003 | T3, T6 | S5 | INV-005 |
| REQ-004 | T4 | S4 | INV-004 |
| REQ-005 | T1, T5 | S6 | INV-006 |

## Related SPECs

- SPEC-PARITY-001 (선행): Codex 규칙·스킬 패리티 정렬. 본 SPEC은 동일 패턴을 Gemini로 확장하고
  재발 방지 게이트를 추가한다. 단일 Primary SPEC으로 Outcome Lock을 닫으며 sibling SPEC은 없다.

## Out of Scope

- 설치 표면(`.gemini/**`, `.codex/**`, `.opencode/**`) 일괄 재생성(`auto update`).
- Claude·OpenCode 규칙에 `platform:` 필드 신규 추가.
- 스킬 카탈로그 visibility/compile 의미 변경.

## Completion Verdict

- Outcome Lock: satisfied — Gemini/Antigravity 사용자가 플랫폼 중립 규칙 14종(누락 3종 추가)과 extended skills(네이티브 경로 해소)를 받으며, content 소스 대비 플랫폼 패리티가 양방향 oracle 게이트 테스트(`runCoverageGate` + probe)로 강제된다. platform frontmatter 값은 어댑터 식별자로 정규화·게이팅된다.
- Mandatory requirements: 4/4 Must (REQ-001/002/004/005), Should 1/1 (REQ-003)
- Must acceptance: S1~S6 전부 oracle 테스트 green (`go test ./pkg/adapter/... -race`)
- Review: multi-provider debate PASS (64/66, Rev1 closure 후), Phase 4 reviewer APPROVE (LOW 2건 즉시 반영: 중복 H1 제거, t.TempDir 전환)
- Completion Debt: none
- Evolution Ideas: advisory로만 잔존
