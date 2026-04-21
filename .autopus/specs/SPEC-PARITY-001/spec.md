---
id: SPEC-PARITY-001
title: 멀티플랫폼 하네스 패리티 완성
status: completed
target_module: autopus-adk
created: 2026-04-02
depends_on: SPEC-MULTIPLATFORM-001
---

# SPEC-PARITY-001: 멀티플랫폼 하네스 패리티 완성

**Status**: completed (2026-04-21 sync)
**Created**: 2026-04-02
**Target Module**: autopus-adk
**Depends On**: SPEC-MULTIPLATFORM-001 (completed)
**Follow-up**: Codex `--team` wording/tool contract hardening은 `SPEC-CODEXTEAM-001`로 분리 추적한다.

## 목적

SPEC-MULTIPLATFORM-001에서 기본 어댑터 확장을 완료했으나, Codex ~45%, Gemini ~75%로 Claude Code 100% 대비 패리티 갭이 존재한다. 이 SPEC은 Codex/Gemini CLI가 Claude Code와 동등한 기능 수준을 달성하도록 남은 갭을 해소한다.

### 현재 패리티 현황

| Platform | Agents | Skills | Rules | Parity |
|----------|--------|--------|-------|--------|
| Claude Code | 16 | 70+ | 7 (file-based) | 100% |
| Gemini CLI | 16 | 6 | 4 (file-based) | ~75% |
| Codex | 5 | 6 | inline (AGENTS.md) | ~45% |

---

## Domain 1: Codex 룰 격리 (P0)

### R1 — Codex 룰 파일 분리

> WHEN `auto init --platform codex` 실행 시,
> THE SYSTEM SHALL 현재 AGENTS.md에 인라인된 룰을 `.codex/rules/autopus/` 디렉토리에 개별 markdown 파일로 분리하여 생성한다.

**Priority**: P0 (Must Have)

**Rationale**: 현재 `codex_rules.go`의 `renderRulesSection()`이 모든 룰을 AGENTS.md 본문에 인라인한다. 이는 격리/확장/조건부 적용이 불가하며, Codex CLI의 `AGENTS.md` 크기를 과도하게 키운다.

**Constraints**:
- Codex CLI는 `.codex/` 하위 flat 구조만 스캔 — `.codex/rules/autopus/` 서브디렉토리가 읽히는지 검증 필요
- 서브디렉토리 미지원 시 `.codex/rules-autopus-*.md` flat naming fallback 적용
- T1은 양쪽 경로(서브디렉토리 + flat) 모두 구현하고 런타임 감지로 분기
- `content/rules/` 소스에서 branding.md, project-identity.md 제외 (Codex에 불필요)

**Acceptance Criteria**:
- AC-1.1: `.codex/rules/autopus/` (또는 fallback flat)에 7개 룰 파일 생성 (branding, project-identity 제외)
- AC-1.2: `.codex/AGENTS.md`에서 인라인 Rules 섹션 제거, 룰 디렉토리 참조로 대체
- AC-1.3: `codex_rules.go`의 `renderRulesSection()` → `generateRuleFiles()` 리팩터링

**File Ownership**: `pkg/adapter/codex/codex_rules.go`, `templates/codex/rules/`

### R2 — AGENTS.md 경량화

> WHEN 룰이 파일로 분리된 후,
> THE SYSTEM SHALL `.codex/AGENTS.md`에 룰 디렉토리 경로 참조만 남기고 인라인 콘텐츠를 제거하여 파일 크기를 50% 이상 줄인다.

**Note**: `AGENTS.md`는 Codex CLI 전용 설정 파일 (`.codex/AGENTS.md`)이며, 프로젝트 루트의 다른 파일과 무관합니다.

**Priority**: P0 (Must Have)

**File Ownership**: `pkg/adapter/codex/codex_skills.go` (`.codex/AGENTS.md` 생성 로직의 agentsMDTemplate 상수)

---

## Domain 2: Codex 에이전트 확장 (P0)

### R3 — 누락 에이전트 11개 템플릿 추가

> WHEN `auto init --platform codex` 실행 시,
> THE SYSTEM SHALL `content/agents/`의 16개 에이전트 정의를 모두 `.codex/agents/` TOML 파일로 생성한다.

**Priority**: P0 (Must Have)

**현재 상태**: `templates/codex/agents/`에 5개만 존재 (debugger, executor, planner, reviewer, tester)

**누락 목록**: annotator, architect, deep-worker, devops, explorer, frontend-specialist, perf-engineer, security-auditor, spec-writer, ux-validator, validator (11개)

**Acceptance Criteria**:
- AC-3.1: `templates/codex/agents/`에 16개 `.toml.tmpl` 파일 존재
- AC-3.2: 각 TOML에 `name`, `model`, `instructions` 필드 포함 (Codex agent schema 준수)
- AC-3.3: `content/agents/*.md`의 역할 설명이 `instructions` 필드에 반영

**File Ownership**: `templates/codex/agents/*.toml.tmpl`, `pkg/adapter/codex/codex_agents.go`

---

## Domain 3: Gemini 누락 룰 추가 (P0)

### R4 — Gemini 룰 3개 추가

> WHEN `auto init --platform gemini` 실행 시,
> THE SYSTEM SHALL `.gemini/rules/autopus/`에 worktree-safety.md, context7-docs.md, doc-storage.md 룰 파일을 추가 생성한다.

**Priority**: P0 (Must Have)

**현재 상태**: `templates/gemini/rules/autopus/`에 4개 (file-size-limit, language-policy, lore-commit, subagent-delegation)
**추가 필요**: worktree-safety, context7-docs, doc-storage (3개) → 총 7개

**Acceptance Criteria**:
- AC-4.1: `templates/gemini/rules/autopus/`에 7개 `.md.tmpl` 파일 존재
- AC-4.2: 각 룰 파일이 `content/rules/` 소스와 동일한 내용 (플랫폼 변환 적용)
- AC-4.3: `gemini_rules.go`의 `prepareRuleMappings()`이 새 룰을 자동 포함

**File Ownership**: `templates/gemini/rules/autopus/*.md.tmpl`, `pkg/adapter/gemini/gemini_rules.go`

---

## Domain 4: 확장 스킬 배포 파이프라인 (P1)

### R5 — 스킬 변환 엔진

> WHEN `auto init` 실행 시,
> THE SYSTEM SHALL `content/skills/`의 40개 확장 스킬 중 플랫폼-agnostic 스킬을 감지하여 Codex/Gemini 포맷으로 변환 후 배포한다.

**Priority**: P1 (Should Have)

**Rationale**: 현재 Codex/Gemini는 6개 `/auto` 명령 스킬만 보유. `content/skills/`의 40개 중 범용 스킬(debugging, refactoring, performance 등)은 플랫폼 공통으로 활용 가능하다.

**Acceptance Criteria**:
- AC-5.1: `pkg/content/skill_transformer.go` 신규 — 스킬 markdown을 플랫폼별 포맷으로 변환
- AC-5.2: Codex → `.codex/skills/{skill-name}.md`, Gemini → `.gemini/skills/{skill-name}/`
- AC-5.3: 플랫폼 특정 MCP/tool 참조는 자동 필터링 또는 주석 처리

**File Ownership**: `pkg/content/skill_transformer.go`, `pkg/adapter/codex/codex_skills.go`, `pkg/adapter/gemini/gemini_skills.go`

### R6 — 스킬 호환성 매트릭스

> WHERE 스킬이 플랫폼 특정 기능(MCP tool 호출, Claude-specific 문법 등)을 참조하는 경우,
> THE SYSTEM SHALL 해당 스킬을 호환 불가(platform-specific)로 표시하고 변환 대상에서 제외한다.

**Priority**: P1 (Should Have)

**Acceptance Criteria**:
- AC-6.1: `content/skills/*.md` YAML frontmatter에 `platforms: [claude, codex, gemini]` 메타 지원
- AC-6.2: `platforms` 미지정 시 모든 플랫폼 호환으로 간주
- AC-6.3: 변환 리포트에 호환/비호환 스킬 목록 출력

---

## Domain 5: Codex 훅 병합 로직 (P1)

### R7 — hooks.json 사용자 훅 보존

> WHEN `auto update --platform codex` 실행 시,
> THE SYSTEM SHALL `.codex/hooks.json`의 기존 사용자 훅을 보존하고 Autopus 훅만 upsert한다.

**Priority**: P1 (Should Have)

**현재 상태**: `codex_hooks.go`의 `generateHooks()`가 hooks.json을 OverwriteAlways로 덮어씀 — 사용자 커스텀 훅 손실

**Acceptance Criteria**:
- AC-7.1: 기존 hooks.json 파싱 후 Autopus 마커가 있는 훅만 교체
- AC-7.2: 마커 없는 사용자 훅은 그대로 보존
- AC-7.3: OverwritePolicy를 `OverwriteMerge`로 변경

**File Ownership**: `pkg/adapter/codex/codex_hooks.go`

---

## Domain 6: Gemini AfterAgent 훅 중복 방지 (P1)

### R8 — AfterAgent 훅 멱등성

> WHEN `InjectOrchestraAfterAgentHook()`이 반복 호출될 때,
> THE SYSTEM SHALL 동일 scriptPath의 훅이 이미 존재하면 추가하지 않는다.

**Priority**: P1 (Should Have)

**현재 상태**: `gemini_hooks.go:44`에 `@AX:WARN` 어노테이션 — `append` 시 중복 체크 없음

**Acceptance Criteria**:
- AC-8.1: 동일 command의 AfterAgent 훅은 1개만 존재
- AC-8.2: 기존 훅 목록 순서 보존
- AC-8.3: `@AX:WARN` 어노테이션 제거

**File Ownership**: `pkg/adapter/gemini/gemini_hooks.go`

---

## Domain 7: 크로스-플랫폼 설정 스키마 (P2)

### R9 — 캐노니컬 설정 스키마 정의

> WHERE 플랫폼별 설정 포맷이 다른 경우 (JSON nested / JSON flat / TOML),
> THE SYSTEM SHALL 하나의 Go struct로 캐노니컬 스키마를 정의하고 플랫폼별 직렬화를 제공한다.

**Priority**: P2 (Could Have)

**Acceptance Criteria**:
- AC-9.1: `pkg/config/platform_schema.go`에 `PlatformSettings` 구조체 정의
- AC-9.2: `ToClaudeJSON()`, `ToGeminiJSON()`, `ToCodexTOML()` 메서드 제공
- AC-9.3: 라운드트립 테스트 통과 (serialize → deserialize → compare)

**File Ownership**: `pkg/config/platform_schema.go`

---

## Domain 8: 패리티 테스트 게이트 (P2)

### R10 — CI 패리티 검증 테스트

> WHEN CI 파이프라인이 실행될 때,
> THE SYSTEM SHALL 각 플랫폼 어댑터가 생성하는 기능 목록을 비교하여 패리티 갭을 리포트한다.

**Priority**: P2 (Could Have)

**Acceptance Criteria**:
- AC-10.1: `pkg/adapter/parity_test.go`에 3개 플랫폼 기능 비교 테스트
- AC-10.2: agents, rules, skills 카테고리별 패리티 퍼센트 산출
- AC-10.3: P0 기능의 패리티가 95% 미만이면 테스트 실패

**File Ownership**: `pkg/adapter/parity_test.go`

---

## 생성 파일 상세

| # | File | Role |
|---|------|------|
| 1 | `pkg/adapter/codex/codex_rules.go` | 리팩터: 인라인 → 파일 생성 |
| 2 | `templates/codex/rules/*.md.tmpl` | 신규: Codex 룰 템플릿 7개 |
| 3 | `templates/codex/agents/*.toml.tmpl` | 신규: 누락 에이전트 11개 |
| 4 | `templates/gemini/rules/autopus/*.md.tmpl` | 신규: 누락 룰 3개 |
| 5 | `pkg/content/skill_transformer.go` | 신규: 스킬 변환 엔진 |
| 6 | `pkg/adapter/codex/codex_hooks.go` | 수정: 병합 로직 |
| 7 | `pkg/adapter/gemini/gemini_hooks.go` | 수정: 중복 방지 |
| 8 | `pkg/config/platform_schema.go` | 신규: 캐노니컬 스키마 |
| 9 | `pkg/adapter/parity_test.go` | 신규: 패리티 게이트 |
