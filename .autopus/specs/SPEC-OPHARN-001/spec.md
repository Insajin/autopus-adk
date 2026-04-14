# SPEC-OPHARN-001: OpenCode 하네스 표면 패리티 복원

**Status**: completed
**Created**: 2026-04-14
**Domain**: OPHARN
**Priority**: Must-Have
**Target Module**: autopus-adk
**Depends On**: SPEC-PARITY-001, SPEC-HARN-ENRICH-001

## 목적

OpenCode 어댑터는 rules, agents, skills, commands 파일을 생성하지만 실제 UX 표면은 Claude Code 대비 크게 축약되어 있다. 현재 `pkg/adapter/opencode/opencode_specs.go`의 `workflowSpecs`는 8개 엔트리만 정의하고, `.opencode/commands/auto.md`와 `.agents/skills/auto/SKILL.md`는 plan/go/fix/review/sync/canary/idea 7개 흐름만 노출한다. 또한 `.opencode/plugins/autopus-hooks.js`는 생성되지만 `opencode.json`에는 플러그인 등록이 자동 반영되지 않는다.

이 SPEC의 목표는 OpenCode 하네스를 “파일은 있는 상태”에서 “실제로 쓸 수 있는 상태”로 끌어올리는 것이다. 핵심은 다음 3가지다.

1. `/auto` 라우터와 helper mode 표면 복구
2. runtime wiring과 diagnostics 복구
3. source gap과 platform gap을 분리한 parity gate 도입

## 요구사항

### Domain 1: Router Surface Recovery (P0)

- **REQ-OPH-001**: WHEN `auto init --platform opencode` 또는 `auto update --platform opencode`가 OpenCode router를 생성할 때, THE SYSTEM SHALL shared triage, project-context bootstrap, global flag parsing, SPEC path resolution을 포함하는 canonical parity contract를 사용하여 router를 생성해야 한다.

- **REQ-OPH-002**: WHEN OpenCode `/auto` 표면이 생성될 때, THE SYSTEM SHALL `plan`, `go`, `fix`, `review`, `sync`, `canary`, `idea` 외에 parity-critical helper flows `setup`, `status`, `map`, `why`, `verify`, `secure`, `test`, `dev`, `doctor`를 OpenCode-native entrypoint로 노출해야 한다.

- **REQ-OPH-003**: WHEN a requested flow cannot be fully supported on OpenCode, THE SYSTEM SHALL generated router/command output에 explicit degradation note와 nearest supported alternative를 포함해야 한다. 침묵 속 누락은 허용되지 않는다.

- **REQ-OPH-004**: WHERE router content is shared across platforms, THE SYSTEM SHALL source OpenCode router behavior from a canonical shared asset or extracted parity contract rather than anchoring behavior to `codex/prompts/auto.md.tmpl` alone.

### Domain 2: Runtime Wiring & Diagnostics (P0)

- **REQ-OPH-010**: WHEN OpenCode hook plugin files are generated, THE SYSTEM SHALL register `.opencode/plugins/autopus-hooks.js` in `opencode.json` during standard `Generate()` and `Update()` flows. Post-generation manual injection MUST NOT be required.

- **REQ-OPH-011**: WHEN OpenCode runtime config is rendered, THE SYSTEM SHALL manage a canonical OpenCode runtime schema that includes instructions, plugins, and supported observability/session surface entries instead of only appending rule paths.

- **REQ-OPH-012**: WHEN `Validate()` or `auto doctor` inspects an OpenCode harness, THE SYSTEM SHALL verify both plugin file existence and plugin registration state in `opencode.json`, and SHALL report mismatches as actionable findings.

- **REQ-OPH-013**: WHEN a Claude runtime feature has no direct OpenCode equivalent (`settings.json`, permission allowlist, `TeamCreate`, provider stop hooks, Claude statusline), THE SYSTEM SHALL map it to an OpenCode-native alternative or mark it unsupported-by-platform. Claude structures MUST NOT be copied verbatim.

- **REQ-OPH-014**: IF OpenCode exposes a supported session observability extension point, THEN THE SYSTEM SHALL generate a corresponding OpenCode status surface; OTHERWISE the system shall emit a documented fallback or intentional omission note.

### Domain 3: Skill Inventory & Compatibility (P1)

- **REQ-OPH-020**: WHEN `.agents/skills/` assets are generated for OpenCode, THE SYSTEM SHALL include platform-agnostic strategic skills currently missing from canonical source (`product-discovery`, `competitive-analysis`, `metrics`) or classify them explicitly as unsupported-by-source.

- **REQ-OPH-021**: WHEN a generated OpenCode skill or command contains platform-specific references, THE SYSTEM SHALL transform them to OpenCode-native surfaces (`task`, `question`, `skill`, `todowrite`, `.opencode/`, `.agents/skills/`) or annotate the incompatibility inline.

- **REQ-OPH-022**: WHEN OpenCode skills and commands are generated, THE SYSTEM SHALL reject unsupported residual tokens such as `Agent(`, `spawn_agent`, `TeamCreate`, `settings.json`, or raw Claude permission fields.

### Domain 4: Parity Gate & Classification (P1)

- **REQ-OPH-030**: WHEN adapter tests or parity CI run, THE SYSTEM SHALL compare OpenCode router subcommands, runtime wiring, and extended skill inventory against the canonical parity baseline and fail when any P0 surface regresses.

- **REQ-OPH-031**: WHEN a feature is classified as unsupported-by-platform or unsupported-by-source, THE SYSTEM SHALL report that classification separately from missing implementation so parity reports distinguish design constraints from actual gaps.

## 생성 파일 상세

| 파일 | 역할 |
|------|------|
| `pkg/adapter/opencode/opencode_specs.go` | OpenCode surface inventory와 parity-critical workflow 정의 확장 |
| `pkg/adapter/opencode/opencode_skills.go` | canonical router/skill 렌더링 및 extended skill 배포 정렬 |
| `pkg/adapter/opencode/opencode_commands.go` | helper flow command entrypoint 생성 |
| `pkg/adapter/opencode/opencode_config.go` | plugin registration 및 runtime schema 관리 |
| `pkg/adapter/opencode/opencode_lifecycle.go` | plugin registration/unsupported classification 진단 강화 |
| `pkg/adapter/opencode/opencode_test.go` | router surface, plugin activation, 금지 토큰 회귀 테스트 |
| `pkg/content/skill_transformer*.go` | unsupported-by-source / unsupported-by-platform 분류 보강 가능 지점 |
| `content/skills/*.md` | 누락 전략 스킬 canonical source 추가 후보 |
| `templates/shared/*` 또는 새 shared router asset | platform-agnostic router contract 추출 후보 |

## 제약 조건

- Lore constraint: 플랫폼별 native workflow surface를 유지하고 prompt와 skill의 역할을 섞지 않는다.
- Rejected approach: Claude router 재사용에 문자열 치환만 덧대는 접근.
- `auto arch enforce` 기준 현재 아키텍처 위반은 없으므로 변경은 `pkg/adapter/opencode`, `pkg/content`, shared assets에 국한한다.

## Out of Scope

- OpenCode용 팀 협업 런타임을 Claude Agent Teams처럼 새로 구현하는 작업
- Claude `settings.json` / permission schema의 direct port
- provider stop hook, session IPC를 Claude와 동일한 방식으로 재현하는 작업
- OpenCode 제품 자체의 기능 추가
