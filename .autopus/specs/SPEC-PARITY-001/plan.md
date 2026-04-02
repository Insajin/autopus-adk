---
id: SPEC-PARITY-001
type: plan
---

# SPEC-PARITY-001 구현 계획

## 에이전트 배정 테이블

| Task | Agent | Parallel Group | Mode | File Ownership | Est. Lines |
|------|-------|----------------|------|----------------|------------|
| T3 | executor | G1 | parallel | templates/codex/agents/*.toml.tmpl | ~200 |
| T4 | executor | G1 | parallel | templates/gemini/rules/autopus/*.md.tmpl | ~50 |
| T1 | executor | G2 | parallel | pkg/adapter/codex/codex_rules.go, templates/codex/rules/ | ~120 |
| T2 | executor | G2 | parallel | pkg/adapter/codex/codex_skills.go | ~80 |
| T5 | executor | G2 | parallel | (validation only, no file output) | ~30 |
| T6 | executor | G3 | parallel | pkg/content/skill_transformer.go | ~150 |
| T7 | executor | G3 | parallel | pkg/adapter/codex/codex_skills.go (renderSkillTemplates), pkg/adapter/gemini/gemini_skills.go | ~80 |
| T8 | executor | G4 | parallel | pkg/adapter/codex/codex_hooks.go | ~60 |
| T9 | executor | G4 | parallel | pkg/adapter/gemini/gemini_hooks.go | ~30 |
| T10 | executor | G5 | parallel | pkg/config/platform_schema.go | ~120 |
| T11 | executor | G5 | parallel | pkg/adapter/parity_test.go | ~100 |

**Execution Order**: G1 → G2 (depends on G1: T3 에이전트 템플릿 필요) → G3, G4 (independent) → G5 (depends on G1-G4)

## 태스크 목록

### Group G1: 템플릿 생성 (의존 없음, 병렬 실행)

- [ ] T3: Codex 누락 에이전트 11개 템플릿 추가 (R3)
  - `content/agents/*.md` 16개를 기반으로 `templates/codex/agents/*.toml.tmpl` 11개 신규 생성
  - 누락: annotator, architect, deep-worker, devops, explorer, frontend-specialist, perf-engineer, security-auditor, spec-writer, ux-validator, validator
  - 기존 5개(debugger, executor, planner, reviewer, tester) 패턴 참조
  - go:embed 에 새 파일이 자동 포함되는지 확인 (templates/embed.go의 glob 패턴)

- [ ] T4: Gemini 누락 룰 3개 추가 (R4)
  - `templates/gemini/rules/autopus/`에 3개 추가:
    - worktree-safety.md.tmpl
    - context7-docs.md.tmpl
    - doc-storage.md.tmpl
  - `content/rules/` 소스를 Gemini 템플릿 포맷으로 변환
  - `gemini_rules.go`의 `prepareRuleMappings()`은 디렉토리 자동 스캔이므로 코드 수정 불필요

### Group G2: Codex 룰 격리 + 경량화 (G1 완료 후, 병렬 실행)

- [ ] T1: Codex 룰 파일 분리 (R1, R2)
  - `codex_rules.go`의 `renderRulesSection()` → `generateRuleFiles()` 리팩터링
  - `templates/codex/rules/` 디렉토리에 7개 `.md.tmpl` 파일 생성
    - file-size-limit.md.tmpl, language-policy.md.tmpl, lore-commit.md.tmpl
    - subagent-delegation.md.tmpl, worktree-safety.md.tmpl, context7-docs.md.tmpl, doc-storage.md.tmpl
  - 양쪽 경로 모두 구현: 서브디렉토리(`.codex/rules/autopus/`) + flat fallback(`.codex/rules-autopus-*.md`)
  - 런타임 감지로 분기 (`detectCodexSubdirSupport()` 헬퍼 추가)
  - `renderRulesSection()` 호출부 제거, `generateRuleFiles()` 호출부 추가 (codex.go Generate/prepareFiles)
  - `.codex/AGENTS.md`의 인라인 Rules 섹션을 디렉토리 참조로 교체
  - **CRITICAL**: `templates/embed.go`에 `codex/rules/autopus/*.tmpl` (또는 `codex/rules/*.tmpl`) glob 패턴 추가 — 누락 시 빌드 실패 (v0.28.0 선례)

- [ ] T2: `.codex/AGENTS.md` 경량화 (R2)
  - `codex_skills.go`의 `agentsMDTemplate` 상수에서 Core Guidelines 축소
  - 룰 참조: `See .codex/rules/autopus/ for detailed guidelines`
  - 에이전트 참조: `See .codex/agents/ for agent definitions`

- [ ] T5: Codex 룰 서브디렉토리 지원 검증 (R1 보조)
  - Codex CLI가 `.codex/rules/` 서브디렉토리를 읽는지 E2E 검증
  - 검증 결과에 따라 T1의 기본 분기를 확정 (subdir 또는 flat)
  - T1은 양쪽 모두 구현하므로 T5 결과가 없어도 T1은 동작 가능

### Group G3: 스킬 변환 (독립, 병렬 실행)

- [ ] T6: 스킬 변환 엔진 구현 (R5, R6)
  - `pkg/content/skill_transformer.go` 신규 생성
  - `content/skills/*.md`의 YAML frontmatter 파싱 → `platforms` 필드 체크
  - `platforms` 미지정 시 전체 플랫폼 호환으로 간주 (R6)
  - 플랫폼 특정 참조 (MCP tool 호출, Claude-specific 문법 등) 필터링
  - Codex/Gemini 포맷 변환 (파일 경로, 구조 차이 처리)
  - 변환 리포트 생성 (호환/비호환 목록)

- [ ] T7: 스킬 배포 통합 (R5)
  - `codex_skills.go`의 `renderSkillTemplates()` 확장 — 변환된 확장 스킬 포함
  - `gemini_skills.go` 동일 확장
  - 기존 6개 `/auto` 스킬과 확장 스킬 구분 (디렉토리 또는 prefix)

### Group G4: 훅 수정 (독립, 병렬 실행)

- [ ] T8: Codex hooks.json 병합 로직 (R7)
  - `codex_hooks.go`의 `generateHooks()` 수정
  - 기존 hooks.json 파싱 → Autopus 마커(`__autopus__` 키) 기반 upsert
  - OverwritePolicy: `OverwriteAlways` → `OverwriteMerge`
  - 사용자 훅 보존 테스트

- [ ] T9: Gemini AfterAgent 훅 중복 방지 (R8)
  - `gemini_hooks.go`의 `InjectOrchestraAfterAgentHook()` 수정
  - `existing` 슬라이스에서 동일 `command` 검색 → 존재 시 skip
  - `@AX:WARN` 어노테이션 제거

### Group G5: 스키마 + 테스트 게이트 (G1-G4 완료 후, 병렬 실행)

- [ ] T10: 크로스-플랫폼 설정 스키마 (R9)
  - `pkg/config/platform_schema.go` 신규 생성
  - `PlatformSettings` 구조체: agents, rules, skills, hooks 필드
  - `ToClaudeJSON()`, `ToGeminiJSON()`, `ToCodexConfig()` 직렬화 메서드 (Codex는 혼합 포맷: TOML agents + JSON hooks + MD AGENTS.md)
  - 라운드트립 테스트

- [ ] T11: 패리티 테스트 게이트 (R10)
  - `pkg/adapter/parity_test.go` 신규 생성
  - 3개 어댑터의 Generate() 결과 비교
  - agents/rules/skills 카테고리별 패리티 퍼센트 산출
  - P0 카테고리 95% 미만 시 fail
  - 실행 컨텍스트: `go test ./pkg/adapter/` (CI 통합 테스트)

## 구현 전략

### 접근 방법

1. **콘텐츠 우선, 코드 나중**: 먼저 템플릿 파일(T3, T4)을 생성한 뒤 어댑터 코드(T1, T2)를 수정한다. 이렇게 하면 기존 테스트가 깨지지 않는 상태에서 점진적 이행이 가능하다.

2. **기존 패턴 활용**: Gemini 룰 시스템(`gemini_rules.go`)이 디렉토리 자동 스캔 패턴을 사용하므로, Codex도 동일 패턴으로 리팩터링한다. Codex 에이전트 템플릿도 기존 5개의 TOML 구조를 그대로 따른다.

3. **Fallback 전략**: Codex CLI의 서브디렉토리 지원이 불확실하므로, T5에서 E2E 검증 후 T1의 분기 로직을 확정한다.

### 변경 범위

- **수정 파일**: 4개 (codex_rules.go, codex_skills.go, codex_hooks.go, gemini_hooks.go)
- **신규 파일**: ~25개 (템플릿 14개 + Go 소스 3개 + 테스트 ~8개)
- **삭제 코드**: codex_rules.go의 `renderRulesSection()` (~50줄), AGENTS.md 인라인 룰 (~100줄)

### 병렬 실행 그룹 (정합적 정의)

| Group | Tasks | Mode | Dependencies |
|-------|-------|------|-------------|
| G1 | T3, T4 | parallel | 없음 — 템플릿 파일만 생성 |
| G2 | T1, T2, T5 | parallel | G1 완료 후 (T1이 T3 에이전트 참조, T2가 AGENTS.md 경량화) |
| G3 | T6, T7 | parallel | 없음 — 독립 모듈 (G2와 병렬 가능) |
| G4 | T8, T9 | parallel | 없음 — 독립 수정 (G2와 병렬 가능) |
| G5 | T10, T11 | parallel | G1-G4 전체 완료 후 (패리티 비교 위해 전체 어댑터 필요) |

**실행 흐름**: G1 → (G2 + G3 + G4 병렬) → G5
