---
id: SPEC-PARITY-001
type: research
---

# SPEC-PARITY-001 리서치

## 기존 코드 분석

### 1. Codex 룰 시스템 (현재: 인라인)

**파일**: `pkg/adapter/codex/codex_rules.go`
- `renderRulesSection(cfg)` (line 16): `contentfs.FS`에서 `content/rules/*.md`를 읽어 AGENTS.md 본문에 인라인
- `stripFrontmatter()` (line 80): YAML frontmatter 제거 헬퍼
- `renderFileSizeRule(cfg)` (line 65): file-size-limit만 템플릿 렌더링 (stack/framework-aware exclusions)
- **문제점**: 모든 룰이 AGENTS.md에 합쳐지므로 개별 룰 on/off, 조건부 적용 불가

**비교 대상 — Gemini 룰 시스템 (파일 기반)**:
- `pkg/adapter/gemini/gemini_rules.go`의 `prepareRuleMappings(cfg)` (line 46)
- `templates/gemini/rules/autopus/` 디렉토리 자동 스캔 → 개별 `.md` 파일 생성
- 신규 룰 추가 시 `.tmpl` 파일만 넣으면 코드 수정 불필요 (확장성 우수)

**결론**: Codex도 Gemini와 동일한 디렉토리 스캔 패턴으로 리팩터링해야 한다.

### 2. Codex 에이전트 시스템

**파일**: `pkg/adapter/codex/codex_agents.go`
- `generateAgents(cfg)` (line 19): `templates/codex/agents/` 디렉토리 스캔 → `.toml` 파일 생성
- `prepareAgentFiles(cfg)` (line 65): 동일 로직, 디스크 쓰기 없음 (Update 경로용)
- `renderAgentsSection()` (line 104): AGENTS.md 인라인 섹션 — `content/agents/*.md` 스캔

**현재 템플릿**: `templates/codex/agents/` — 5개만 존재
```
debugger.toml.tmpl, executor.toml.tmpl, planner.toml.tmpl, reviewer.toml.tmpl, tester.toml.tmpl
```

**content/agents/ (source of truth)**: 16개
```
annotator, architect, debugger, deep-worker, devops, executor, explorer,
frontend-specialist, perf-engineer, planner, reviewer, security-auditor,
spec-writer, tester, ux-validator, validator
```

**누락 11개**: annotator, architect, deep-worker, devops, explorer, frontend-specialist, perf-engineer, security-auditor, spec-writer, ux-validator, validator

**코드 수정 불필요**: `generateAgents()`가 디렉토리 스캔이므로, 템플릿 파일만 추가하면 자동 포함.

### 3. Gemini 룰 시스템

**파일**: `templates/gemini/rules/autopus/` — 4개 존재
```
file-size-limit.md.tmpl, language-policy.md.tmpl, lore-commit.md.tmpl, subagent-delegation.md.tmpl
```

**content/rules/ (source of truth)**: 9개
```
branding.md, context7-docs.md, doc-storage.md, file-size-limit.md,
language-policy.md, lore-commit.md, project-identity.md, subagent-delegation.md, worktree-safety.md
```

**누락 (Gemini에 필요한 것)**: worktree-safety.md, context7-docs.md, doc-storage.md (3개)
**의도적 제외**: branding.md (Gemini는 자체 branding 없음), project-identity.md (플랫폼 무관)

**코드 수정 불필요**: `gemini_rules.go`의 `prepareRuleMappings()`이 디렉토리 자동 스캔하므로, `.tmpl` 파일만 추가하면 된다.

### 4. Codex 훅 시스템

**파일**: `pkg/adapter/codex/codex_hooks.go`
- `generateHooks(cfg)` (line 15): 템플릿 렌더링 → `OverwriteAlways`로 덮어쓰기
- `prepareHooksFile(cfg)` (line 43): 동일, 디스크 쓰기 없음

**문제점**: hooks.json을 통째로 덮어쓰므로 사용자가 추가한 커스텀 훅이 유실된다.

**해결 방향**:
- 기존 hooks.json 읽기 → JSON 파싱
- Autopus 훅 식별 마커: `"__autopus__": true` 필드 또는 command prefix `autopus-`
- 마커 있는 훅만 교체, 없는 훅 보존
- OverwritePolicy를 `OverwriteMerge`로 변경

### 5. Gemini 훅 중복 문제

**파일**: `pkg/adapter/gemini/gemini_hooks.go`
- `InjectOrchestraAfterAgentHook()` (line 14): `@AX:WARN` 어노테이션으로 중복 문제 인지됨
- line 44: `hooksMap["AfterAgent"] = append(existing, entry)` — 무조건 append

**해결**: append 전에 `existing` 슬라이스에서 동일 `command` 값 검색 → 존재하면 skip.

### 6. 스킬 구조

**content/skills/**: 40개 markdown 파일 (source of truth)
**Codex/Gemini 현재**: 6개 `/auto` 명령 스킬만 배포 (auto-plan, auto-go, auto-fix, auto-review, auto-sync, auto-idea)

스킬 파일 구조 (예시):
```markdown
---
name: debugging
description: Advanced debugging strategies
platforms: [claude, codex, gemini]  # 신규 추가 필요
---
# Debugging Skill
...
```

현재 frontmatter에 `platforms` 필드가 없음 — 추가 필요하거나 기본값 = 전체 플랫폼으로 처리.

### 7. go:embed 패턴

**파일**: `templates/embed.go`
```go
//go:embed claude/commands/*.tmpl claude/skills/*.tmpl claude/*.tmpl claude/rules/*.tmpl codex/skills/*.tmpl codex/prompts/*.tmpl codex/agents/*.tmpl codex/*.tmpl gemini/skills/*/*.tmpl gemini/commands/auto/*.tmpl gemini/rules/autopus/*.tmpl gemini/settings/*.tmpl hooks/*.tmpl shared/*.tmpl
var FS embed.FS
```

**주의**: `all:` 패턴이 아닌 **명시적 glob 패턴**을 사용한다. 따라서:
- `codex/rules/*.tmpl` 또는 `codex/rules/autopus/*.tmpl` 패턴이 **현재 없음** — T1에서 반드시 추가 필요
- `codex/agents/*.tmpl`은 이미 포함 — T3에서 추가하는 에이전트 템플릿은 자동 embed
- 이전에 `gemini/commands/auto/*.tmpl` 누락으로 v0.28.0 릴리즈가 실패한 전례 있음 — embed 패턴 누락 주의

---

## 설계 결정

### D1: Codex 룰 파일 구조

**결정**: `.codex/rules/autopus/*.md` (서브디렉토리) 우선, fallback으로 flat naming

**근거**:
- Codex CLI의 정확한 디렉토리 스캔 범위가 문서화되어 있지 않음
- 서브디렉토리를 시도한 뒤 E2E에서 미작동 확인 시 flat으로 전환
- Gemini의 `.gemini/rules/autopus/` 패턴과 일관성 유지가 이상적

**대안 검토**:
- (A) AGENTS.md 인라인 유지 → 확장성 부족, 패리티 불가
- (B) `.codex/rules/` flat (서브디렉토리 없음) → 네임스페이스 충돌 우려 (사용자 룰과)

### D2: 에이전트 TOML 생성 방식

**결정**: `content/agents/*.md`에서 자동 변환하지 않고, 수동 TOML 템플릿 유지

**근거**:
- Codex의 TOML agent schema에 `model`, `timeout` 등 플랫폼 특정 필드가 있어 1:1 자동 변환이 부정확
- 기존 5개 템플릿이 수동 작성 패턴이므로 일관성 유지
- 11개 추가는 일회성 작업이라 자동화 투자 대비 이득 적음

**대안 검토**:
- (A) content/agents/*.md → TOML 자동 변환기 → 지속 유지보수 비용, 변환 정확도 이슈
- (B) content에 TOML 원본 추가 → content 포맷 혼재 (md + toml) 복잡도 증가

### D3: 훅 병합 마커 전략

**결정**: Autopus 훅에 `"__autopus__": true` 메타 필드 추가

**근거**:
- command prefix 기반 식별은 사용자가 `autopus-`로 시작하는 훅을 만들 경우 충돌
- 별도 메타 필드가 가장 명확하고 충돌 가능성 없음
- JSON이므로 추가 필드가 스키마 문제를 일으키지 않음

**대안 검토**:
- (A) command prefix `autopus-` → 사용자 훅과 충돌 가능
- (B) 별도 마커 파일 `.codex/hooks-autopus.json` → Codex CLI가 인식 못함

### D4: 스킬 호환성 판단 기준

**결정**: `platforms` frontmatter 필드 기반, 미지정 시 전체 호환 간주

**근거**:
- 선언적 방식이 가장 명확하고 유지보수 용이
- 기존 40개 스킬에 frontmatter 추가는 일회성 작업
- 런타임 파싱으로 MCP 참조 감지하는 것은 false positive 우려

**대안 검토**:
- (A) 내용 기반 자동 감지 (MCP 키워드 스캔) → false positive/negative 위험
- (B) 별도 매트릭스 파일 → content와 분리되어 동기화 누락 위험

### D5: 패리티 테스트 범위

**결정**: P0 카테고리(agents, rules)만 95% 게이트, P1(skills)은 리포트만

**근거**:
- P1 스킬 변환은 점진적 확장이므로 초기에 게이트 걸면 CI 블로커됨
- P0는 SPEC 목표의 핵심이므로 엄격히 관리
- 추후 스킬 패리티가 안정화되면 게이트 승격 가능

---

## 리스크

| Risk | Impact | Mitigation |
|------|--------|------------|
| Codex CLI가 `.codex/rules/` 서브디렉토리 미지원 | P0 R1 설계 변경 | T5에서 조기 검증, flat fallback 준비 |
| 40개 스킬 중 플랫폼 특정 참조 누락 감지 | 변환된 스킬 런타임 오류 | platforms frontmatter + 수동 리뷰 |
| hooks.json 스키마 변경 (Codex 업데이트) | 병합 로직 파손 | JSON 스키마 버전 체크 추가 |
| 300줄 제한으로 Go 파일 분할 필요 | 파일 수 증가 | 기존 split 패턴 (generate vs prepare) 활용 |
