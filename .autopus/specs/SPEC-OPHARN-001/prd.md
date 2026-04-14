# PRD: OpenCode 하네스 표면 패리티 복원

- **SPEC-ID**: SPEC-OPHARN-001
- **Author**: Autopus
- **Status**: Draft
- **Date**: 2026-04-14

---

## Discovery Q&A Checklist

- [x] **Problem**: OpenCode 하네스는 rules/agents 파일 수는 맞춰졌지만 `/auto` 라우터, 런타임 설정, 훅 활성화, 전략 스킬 표면이 Claude 대비 얇아서 실제 사용 경험이 불완전하다.
- [x] **Target Users**: `auto init/update --platform opencode`를 사용하는 Autopus 유지보수자와 OpenCode 사용자.
- [x] **Success Metrics**: OpenCode P0 surface를 parity baseline에 맞춰 복구하고, 생성된 `opencode.json`에서 플러그인 활성화가 누락되지 않도록 한다.
- [x] **Constraints**: OpenCode는 Claude의 `settings.json`, `TeamCreate`, provider stop hook 표면을 그대로 지원하지 않는다. 플랫폼 네이티브 의미를 유지해야 한다.
- [x] **Prior Art**: `SPEC-PARITY-001`, `SPEC-HARN-ENRICH-001`, 2026-04-14 lore context에서 멀티플랫폼 패리티와 OpenCode workflow surface 정합성 복구가 이미 다뤄졌다.
- [x] **Scope Boundary**: OpenCode를 Claude와 동일한 런타임으로 위장하지 않는다. 지원 불가 기능은 대체 표면 또는 명시적 비지원으로 처리한다.

---

## 1. Problem & Context

**Current Situation**

Autopus-ADK는 OpenCode용 `.opencode/`, `.agents/skills/`, `.opencode/agents/`를 생성한다. 그러나 실제 생성 로직은 여전히 얇다. `pkg/adapter/opencode/opencode_specs.go`의 `workflowSpecs`는 8개 엔트리만 정의하고, `pkg/adapter/opencode/opencode_skills.go`의 `renderRouterSkill()`은 `codex/prompts/auto.md.tmpl`을 기반으로 얇은 라우터를 만든다. 또한 `pkg/adapter/opencode/opencode_plugin.go`는 `.opencode/plugins/autopus-hooks.js`를 생성하지만, `pkg/adapter/opencode/opencode_config.go`는 `Generate()` 경로에서 해당 플러그인을 `opencode.json`에 등록하지 않는다.

**Problem Statement**

OpenCode 하네스는 파일 존재 여부만 보면 완성된 것처럼 보이지만, 실제 동작 표면은 Claude Code 대비 상당히 축약되어 있다. 그 결과 사용자는 OpenCode가 지원해야 할 `/auto` 모드, 런타임 진단, 훅 실행, 전략 스킬을 누락된 것으로 체감한다.

**Impact**

- OpenCode 사용자는 `setup`, `status`, `map`, `why`, `verify`, `secure`, `test`, `dev`, `doctor` 같은 상위 흐름을 직접 사용할 수 없다.
- 생성된 훅 플러그인이 `opencode.json`에 연결되지 않으면 아키텍처 체크와 React check가 실제로 실행되지 않을 수 있다.
- 플랫폼 간 parity 판단이 파일 개수 중심으로 남아, 실제 UX 결손이 테스트에서 드러나지 않는다.

**Change Motivation**

사용자 피드백이 직접적으로 “OpenCode harness가 Claude Code에 비해 부실하다”고 지적했고, 현재 코드 분석도 이를 뒷받침했다. 이번 변경은 OpenCode 하네스를 실사용 가능한 표면으로 끌어올리고, 이후 parity regression을 자동으로 막기 위한 기반이다.

---

## 2. Goals & Success Metrics

| Goal | Success Metric | Target | Timeline |
|------|---------------|--------|----------|
| OpenCode `/auto` 표면 복구 | OpenCode router/command에서 parity-critical subcommand 수 | 7개에서 P0 기준 16개 수준으로 확대 | 구현 직후 |
| 훅 연결 활성화 | 새로 생성된 `opencode.json`에서 `.opencode/plugins/autopus-hooks.js` 등록률 | 100% | 구현 직후 |
| parity 회귀 방지 | OpenCode parity 테스트가 router/runtime/skill 결손을 탐지 | P0 누락 시 테스트 실패 | 구현 직후 |
| Claude 잔재 제거 | 생성된 OpenCode 파일에서 금지 토큰(`Agent(`, `TeamCreate`, `settings.json`) 검출 수 | 0 | 구현 직후 |

**Anti-Goals**

- Claude의 JSON 스키마와 권한 체계를 OpenCode에 그대로 복제하지 않는다.
- OpenCode에 존재하지 않는 팀 통신/stop hook API를 억지로 에뮬레이션하지 않는다.

---

## 3. Target Users

| User Group | Role | Usage Frequency | Key Expectation |
|------------|------|-----------------|-----------------|
| ADK Maintainer | 하네스 생성기 유지보수자 | 주간 | 플랫폼별 산출물이 실제로 동작해야 한다 |
| OpenCode User | 하네스 소비자 | 수시 | Claude와 유사한 `/auto` UX를 OpenCode-native 방식으로 사용하고 싶다 |
| Template Contributor | 스킬/룰 작성자 | 수시 | 어떤 기능이 source gap인지, platform gap인지 명확히 알고 싶다 |

**Primary User**: ADK Maintainer

---

## 4. User Stories / Job Stories

### Story 1: OpenCode 사용자 관점

**When** I install the OpenCode harness,
**I want to** access the same parity-critical `/auto` flows that exist on other platforms,
**so I can** use OpenCode without guessing which workflows are silently missing.

**Acceptance Criteria**

- Given OpenCode harness generation, when `.opencode/commands/auto.md` is created, then it lists parity-critical helper flows beyond the current 7 workflows.
- Given a generated router skill, when a natural-language request is routed, then shared triage/global flag semantics remain available in OpenCode-native form.

### Story 2: 유지보수자 관점

**As a** maintainer,
**I want** generated OpenCode plugins and runtime config to be activated automatically,
**so that** generated files are not present-but-dead.

**Acceptance Criteria**

- Given `auto init --platform opencode`, when generation completes, then `opencode.json` contains the plugin path for Autopus hooks.
- Given a broken plugin registration, when `Validate()` or `auto doctor` runs, then the mismatch is reported explicitly.

### Story 3: 콘텐츠 작성자 관점

**As a** template contributor,
**I want** unsupported OpenCode features to be classified separately from implementation gaps,
**so that** parity work does not regress into blind file copying.

**Acceptance Criteria**

- Given a Claude-only feature, when OpenCode assets are generated, then the output contains an explicit degradation note or compatibility manifest entry.
- Given parity CI, when a feature is marked unsupported-by-platform, then it is reported separately from missing implementation.

---

## 5. Functional Requirements

### P0 — Must Have

| ID | Requirement | Notes |
|----|-------------|-------|
| FR-01 | OpenCode router SHALL expose parity-critical `/auto` helper flows (`setup`, `status`, `map`, `why`, `verify`, `secure`, `test`, `dev`, `doctor`) in addition to plan/go/fix/review/sync/canary/idea. | Thin router 해소 |
| FR-02 | OpenCode generation SHALL register `.opencode/plugins/autopus-hooks.js` in `opencode.json` during normal generate/update flow. | 현재 생성 경로 bugfix |
| FR-03 | OpenCode validation SHALL verify both plugin file existence and plugin registration state. | 존재/활성화 분리 검증 |
| FR-04 | OpenCode router generation SHALL use a canonical shared source or extracted parity contract rather than relying on `codex/prompts/auto.md.tmpl` alone. | prompt/skill 역할 분리 |
| FR-05 | Generated OpenCode commands/skills SHALL not retain Claude-only runtime tokens such as `Agent(`, `TeamCreate`, `settings.json`, or provider stop-hook semantics. | 변환 품질 게이트 |

### P1 — Should Have

| ID | Requirement | Notes |
|----|-------------|-------|
| FR-10 | OpenCode generation SHOULD add or explicitly classify missing strategic skills (`product-discovery`, `competitive-analysis`, `metrics`). | source gap 정리 |
| FR-11 | OpenCode parity tests SHOULD compare router/runtime/skill surface against a canonical baseline. | 파일 수가 아닌 UX 표면 기준 |
| FR-12 | Unsupported-by-platform features SHOULD be emitted with a degradation note or manifest entry. | 미구현과 비지원 분리 |

### P2 — Could Have

| ID | Requirement | Notes |
|----|-------------|-------|
| FR-20 | OpenCode SHOULD provide a session/observability fallback surface equivalent to Claude statusline when the platform exposes such an extension point. | 직접 복제 아님 |

---

## 6. Non-Functional Requirements

| Category | Requirement | Target |
|----------|-------------|--------|
| Correctness | Generated OpenCode config and plugin wiring must be internally consistent. | `Validate()` passes on fresh generate |
| Maintainability | Router/command parity logic must be centralized, not duplicated across ad hoc prompt rewrites. | Canonical source 1개 |
| Testability | Parity regressions must fail in automated tests. | P0 gap 1개 이상이면 test fail |
| Safety | Unsupported Claude features must not be copied into OpenCode files verbatim. | 금지 토큰 0개 |

---

## 7. Technical Constraints

**Technology Stack Constraints**

- 변경 범위는 `autopus-adk`의 Go adapter와 content/template 계층에 머문다.
- Lore context 제약: 플랫폼별 native workflow surface를 유지하고 prompt와 skill의 역할을 섞지 않는다.

**External Dependencies**

| Dependency | Version / SLA | Risk if Unavailable |
|------------|---------------|---------------------|
| OpenCode config schema | `https://opencode.ai/config.json` | 플러그인/설정 필드 오해 시 잘못된 산출물 생성 |
| Existing content transformer | `pkg/content/skill_transformer.go` | 변환 품질 회귀 시 Claude 잔재가 OpenCode에 남음 |

**Compatibility Requirements**

- 기존 OpenCode 생성 경로(`Generate`, `Update`, `Validate`, `Clean`)와 manifest 저장 방식은 유지해야 한다.
- Claude/Gemini/Codex용 산출물에는 회귀를 만들면 안 된다.

**Infrastructure Constraints**

- `auto arch enforce` 기준 현재 아키텍처 위반은 없으며, 새 변경도 `pkg/adapter/opencode`와 `pkg/content` 경계를 지켜야 한다.

---

## 8. Out of Scope

The following are out of scope for this release:

- Claude `settings.json` 구조를 OpenCode에 그대로 복사하는 작업
- OpenCode에 존재하지 않는 `TeamCreate`/직접 teammate messaging API 에뮬레이션
- provider-specific stop hook 프로토콜을 Claude와 동일하게 재현하는 작업
- OpenCode 전용 UI/제품 기능 추가

**Deferred to Future Iterations**

- OpenCode가 실제 status line API를 제공하는지 확정된 뒤의 시각적 status surface 고도화

---

## 9. Risks & Open Questions

### Risks

| Risk | Severity | Probability | Mitigation Strategy |
|------|----------|-------------|---------------------|
| Claude monolithic router를 그대로 이식해 OpenCode semantics가 다시 섞임 | High | Medium | shared contract 추출 후 OpenCode 변환 계층을 거친다 |
| 훅 플러그인 등록 수정이 기존 사용자 plugin 배열을 덮어씀 | High | Medium | merge semantics 유지 + 회귀 테스트 추가 |
| 추가 helper flow를 명령 표면에 올리지만 실제 내용이 thin wrapper로만 남음 | Medium | Medium | parity baseline을 명령 존재가 아니라 행동/섹션 기준으로 검증 |
| missing skill 3종을 OpenCode 전용으로만 추가해 canonical source가 더 분산됨 | Medium | Medium | `content/skills`를 canonical source로 승격하거나 명시적 제외 사유를 기록 |

### Open Questions

| # | Question | Owner | Due Date | Status |
|---|----------|-------|----------|--------|
| Q1 | OpenCode가 status line과 동등한 공식 extension point를 제공하는가? | Maintainer | 2026-04-21 | Open |
| Q2 | helper flows를 monolithic auto router 안에 유지할지, shared subcommand asset으로 분리할지? | Maintainer | 2026-04-21 | Open |

---

## 10. Pre-mortem

| # | Failure Scenario | Probability | Impact | Preventive Action |
|---|-----------------|-------------|--------|-------------------|
| 1 | plugin path를 등록했지만 path normalization이 틀려 훅이 로드되지 않음 | Medium | High | generate/update/validate 테스트에 등록 경로 검증 추가 |
| 2 | router 확장 후 OpenCode command와 skill이 서로 다른 표면을 설명함 | Medium | High | command/skill 모두 동일한 canonical inventory를 공유 |
| 3 | Claude-only 기능을 무리하게 포팅해 OpenCode 문서가 거짓말을 함 | Medium | High | unsupported-by-platform 분류와 degradation note를 강제 |

**Connection to Risks (Section 9)**

Failure 1은 plugin merge risk와 직접 연결되고, Failure 2는 canonical source 부재 리스크와 연결되며, Failure 3은 platform semantics 혼합 리스크를 그대로 반영한다.

---

## 11. Practitioner Q&A

**Q1: 가장 먼저 고쳐야 하는 P0는 무엇인가?**
A: `Generate()` 경로에서 OpenCode hook plugin이 `opencode.json`에 등록되지 않는 문제다. 현재는 파일만 생성되고 활성화는 별도 호출에 의존한다.

**Q2: 왜 단순히 Claude router를 그대로 복사하면 안 되는가?**
A: Lore constraint가 플랫폼별 native workflow surface 유지를 요구하고, OpenCode는 Claude의 `settings.json`, `TeamCreate`, provider stop hook semantics를 지원하지 않기 때문이다.

**Q3: missing skill 3종은 OpenCode만의 문제인가?**
A: 아니다. 현재 canonical `content/skills/`에도 해당 소스가 없어서 OpenCode는 물론 다른 non-Claude 플랫폼도 공유 소스를 생성할 수 없다.

**Q4: parity baseline은 무엇을 기준으로 잡아야 하는가?**
A: 파일 개수가 아니라 router subcommand, runtime wiring, unsupported classification, 금지 토큰 부재 같은 행동 표면을 기준으로 잡아야 한다.

**Q5: statusline은 이번 범위에서 필수인가?**
A: 아니다. OpenCode 공식 확장점이 확인되면 P2로 다루고, 이번 범위는 documented fallback 또는 비지원 분류가 우선이다.
