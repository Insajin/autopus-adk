# SPEC-HARN-ENRICH-001: Codex/Gemini 에이전트 정의 및 스킬 콘텐츠 풍부화

**Status**: completed
**Created**: 2026-04-05
**Domain**: HARN
**Priority**: P0
**Depends on**: SPEC-PARITY-001

## 목적

SPEC-PARITY-001에서 생성한 Codex 에이전트 16개 TOML 파일과 Gemini 룰 파일이 구조적으로는 존재하나, Claude Code 대비 콘텐츠 밀도가 현저히 낮다. Codex 에이전트는 7줄 스텁 수준이고, Gemini는 에이전트 정의 파일 자체가 없으며, 스킬은 40개 중 6개만 포팅되어 있다. 이 격차를 해소하여 3개 플랫폼에서 동등한 에이전트 품질을 달성한다.

## 현재 상태

| 항목 | Claude Code | Codex | Gemini |
|------|-------------|-------|--------|
| 에이전트 정의 밀도 | YAML frontmatter + 40줄+ 본문 | 7줄 TOML 스텁 | 없음 (인라인만) |
| 스킬 수 | 40개 (5,817줄) | 6개 | 6개 |
| 도구 참조 | Agent(), MCP, TodoWrite | 미매핑 | @agent 부분 매핑 |

## 요구사항

### Domain 1: 에이전트 정의 풍부화

**R1** [MUST]: WHEN the harness generates Codex agent TOML files, THE SYSTEM SHALL expand each of the 16 agent templates from 7-line stubs to rich `developer_instructions` containing role definition, working principles, workflow steps, and constraints — derived from the corresponding `content/agents/*.md` source.

**R2** [MUST]: WHEN the harness generates Gemini agent definitions, THE SYSTEM SHALL create agent definition files under `templates/gemini/agents/` (one per agent) that capture each agent's role, workflow, and constraints in Gemini CLI-compatible format.

**R3** [MUST]: WHEN transforming Claude-specific agent instructions to other platforms, THE SYSTEM SHALL apply the following tool reference mappings:

| Claude Code | Codex | Gemini |
|-------------|-------|--------|
| `Agent(subagent_type="X", ...)` | `spawn_agent X --task "..."` | `@X ...` |
| `TodoWrite` | (제거 — Codex 내장 없음) | (제거) |
| `mcp__context7__*` | `WebSearch "library docs"` | `WebSearch "library docs"` |
| `.claude/` paths | `.codex/` paths | `.gemini/` paths |
| `isolation: "worktree"` | `auto pipeline worktree` | `auto pipeline worktree` |

**R4** [SHOULD]: WHEN generating agent definitions, THE SYSTEM SHALL preserve the agent's MoSCoW priority skills list from the source frontmatter, mapping skill names to platform-available equivalents.

### Domain 2: 스킬 콘텐츠 포팅

**R5** [MUST]: WHEN the harness runs skill transformation, THE SYSTEM SHALL transform all 40 content skills (not just the current 6) for both Codex and Gemini platforms, producing platform-specific `.md.tmpl` files under `templates/{codex,gemini}/skills/`.

**R6** [MUST]: WHEN transforming skill content, THE SYSTEM SHALL replace Claude Code-specific references with platform-neutral or platform-specific equivalents using the mapping in R3, rather than simply removing lines containing those references.

**R7** [MUST]: WHEN a skill is incompatible with a target platform (e.g., uses capabilities unavailable on Codex/Gemini), THE SYSTEM SHALL mark it with `platforms:` frontmatter excluding that platform and generate a degradation note documenting the limitation.

**R8** [SHOULD]: WHEN generating skill templates, THE SYSTEM SHALL include a `platforms: [claude, codex, gemini]` compatibility matrix in each skill's frontmatter.

### Domain 3: 템플릿 생성 파이프라인

**R9** [MUST]: WHEN `content/agents/*.md` source files change, THE SYSTEM SHALL provide an `agent_transformer.go` in `pkg/content/` that reads agent source markdown and produces platform-specific output (TOML for Codex, MD for Gemini).

**R10** [SHOULD]: WHEN skill or agent content sources change, THE SYSTEM SHALL support a CLI command or build step (`go generate` or `make generate`) that regenerates all platform templates from content sources.

**R11** [COULD]: WHEN running CI, THE SYSTEM SHALL validate that generated templates are up-to-date with their content sources (staleness check).

## 생성 파일 상세

### pkg/content/agent_transformer.go (신규)
- `content/agents/*.md` 파싱 → 플랫폼별 에이전트 정의 생성
- frontmatter 추출, 본문 변환, 도구 참조 매핑

### pkg/content/skill_transformer_replace.go (신규)
- `FilterPlatformReferences` 확장: 삭제 대신 대체(replacement) 로직
- 플랫폼별 도구 매핑 테이블

### pkg/content/agent_transformer_test.go (신규)
### pkg/content/skill_transformer_replace_test.go (신규)

### templates/codex/agents/*.toml.tmpl (수정 — 16개)
- 7줄 스텁 → 풍부한 developer_instructions

### templates/gemini/agents/*.md.tmpl (신규 — 16개)
- Gemini CLI용 에이전트 정의 파일

### templates/{codex,gemini}/skills/*.md.tmpl (신규 — 34개 × 2 플랫폼)
- 누락 스킬 변환 결과물
