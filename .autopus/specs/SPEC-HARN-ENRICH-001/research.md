# SPEC-HARN-ENRICH-001 리서치

## 기존 코드 분석

### 에이전트 소스 정의 (content/agents/*.md)

16개 에이전트가 `content/agents/`에 정의되어 있으며, 각각 YAML frontmatter + 마크다운 본문 구조:

| 파일 | 핵심 역할 | frontmatter skills | 본문 줄 수 (추정) |
|------|-----------|-------------------|-------------------|
| executor.md | TDD/DDD 코드 구현 | tdd, ddd, debugging, ast-refactoring | ~190줄 |
| reviewer.md | TRUST 5 코드 리뷰 | review, verification | ~210줄 |
| tester.md | 테스트 설계/구현 | tdd, testing-strategy, verification | ~150줄+ |
| debugger.md | 근본 원인 분석 | debugging, tdd, verification | ~130줄+ |
| planner.md | SPEC → 태스크 분해 | planning | ~120줄+ |
| architect.md | 시스템 설계 | - | ~100줄+ |
| 기타 10개 | 전문 분야별 에이전트 | 다양 | 50-150줄 |

**frontmatter 필드**: name, description, model, tools, permissionMode, maxTurns, skills

### Codex 에이전트 템플릿 현황 (templates/codex/agents/*.toml.tmpl)

16개 파일 모두 동일한 7줄 스텁 패턴:
```toml
name = "{agent-name}"
description = "{1줄 설명}"
model = "gpt-5.4"

[developer_instructions]
text = "{1-2문장 지시}"
```

**격차**: Claude 에이전트의 역할/원칙/워크플로우/제약/완료기준/보고형식이 전부 누락

### Gemini 현황 (templates/gemini/)

- **에이전트**: 정의 파일 없음. `templates/gemini/agents/` 디렉토리 자체가 존재하지 않음
- **스킬**: 6개만 존재 (auto-go, auto-plan, auto-idea, auto-fix, auto-review, auto-sync)
- **룰**: 7개 존재 (file-size-limit, subagent-delegation, language-policy, lore-commit, worktree-safety, context7-docs, doc-storage, objective-reasoning)
- **커맨드**: 7개 (plan, go, fix, review, sync, idea, canary)

### 스킬 변환 엔진 (pkg/content/skill_transformer.go)

- `SkillTransformer`: `content/skills/*.md` 로드 → frontmatter 파싱 → 플랫폼별 변환
- `TransformForPlatform(platform)`: 호환성 검사 + `FilterPlatformReferences` 적용
- `IsCompatible(meta, platform)`: `platforms` 필드 기반 호환성 체크 (비어있으면 전체 호환)
- `NewSkillTransformerFromFS`: embed.FS 지원

**핵심 함수 경로**:
- `autopus-adk/pkg/content/skill_transformer.go:86` — `TransformForPlatform`
- `autopus-adk/pkg/content/skill_transformer.go:113` — `IsCompatible`
- `autopus-adk/pkg/content/skill_transformer_filter.go:19` — `FilterPlatformReferences`

### 플랫폼 참조 필터 (pkg/content/skill_transformer_filter.go)

현재 **삭제 방식**: `mcp__`, `Agent(subagent_type=`, `.claude/` 패턴이 포함된 줄을 통째로 제거.

**문제점**: 삭제하면 해당 기능의 동등한 대체가 제공되지 않아 스킬 품질이 저하됨.

**해결 방향**: 삭제 → 대체(replacement) 방식. 플랫폼별 매핑 테이블 기반으로 Claude 참조를 Codex/Gemini 동등 참조로 치환.

### Codex 에이전트 생성 로직 (pkg/adapter/codex/codex_agents.go)

- `generateAgents`: `templates/codex/agents/*.toml.tmpl` 읽기 → Go 템플릿 렌더링 → `.codex/agents/` 쓰기
- `prepareAgentFiles`: 동일 로직이나 파일 쓰기 없이 매핑만 반환 (dry-run용)
- `renderAgentsSection`: `content/agents/*.md`를 읽어 AGENTS.md 인라인 섹션 생성
- 렌더링 데이터: `{{.ProjectName}}`, `{{.IsFullMode}}` 등 `HarnessConfig` 필드 사용

## 설계 결정

### D1: 대체(Replacement) vs 삭제(Removal) 전략

**결정**: 대체(Replacement) 전략 채택

**이유**: 
- 삭제 방식은 Claude-specific 기능을 사용하는 줄이 통째로 사라져, Codex/Gemini 사용자가 해당 기능의 동등한 대안을 알 수 없음
- 대체 방식은 `Agent(subagent_type="X")` → `spawn_agent X` 처럼 동등한 플랫폼 기능으로 치환하여 스킬 품질 유지

**트레이드오프**: 
- 매핑 테이블 유지보수 비용 증가 (새 도구 추가 시 매핑도 추가 필요)
- 정규식 기반 대체의 엣지 케이스 가능성

### D2: 에이전트 변환기 위치

**결정**: `pkg/content/agent_transformer.go` 신규 파일

**이유**: 
- `skill_transformer.go`와 동일 패키지에서 frontmatter 파싱 인프라 재활용
- 에이전트 변환은 스킬 변환과 유사한 패턴이나 출력 포맷이 다름 (TOML vs MD)
- 별도 파일로 분리하여 300줄 제한 준수

### D3: Gemini 에이전트 포맷

**결정**: `templates/gemini/agents/*.md.tmpl` — 마크다운 기반

**이유**:
- Gemini CLI는 TOML 에이전트 정의를 지원하지 않음
- 마크다운 형식의 지시(instruction) 파일이 Gemini의 자연스러운 포맷
- `.gemini/` 디렉토리 구조와 일관성 유지

**대안 검토**:
- JSON 기반 정의: Gemini CLI가 JSON agent schema를 공식 지원하지 않음 → 기각
- 스킬 내 인라인: 에이전트별 독립 관리 불가 → 기각

### D4: FilterPlatformReferences 하위 호환

**결정**: `FilterPlatformReferences`를 유지하고 `ReplacePlatformReferences`를 별도 추가. `TransformForPlatform`에서 호출을 교체.

**이유**:
- 기존 함수에 직접 의존하는 외부 코드가 있을 수 있음
- 새 함수를 추가하고 호출 지점만 변경하면 기존 테스트 깨짐 최소화
- 필요 시 deprecated 마킹 후 점진적 제거

### D5: 스킬 포팅 범위

**결정**: 40개 전체 변환 시도, 비호환 스킬은 `platforms` frontmatter로 제외 표시

**이유**:
- 부분 포팅은 플랫폼 간 기능 격차를 지속시킴
- 비호환 스킬도 degradation 대안을 문서화하면 사용자에게 유용
- `IsCompatible` 함수가 이미 `platforms` 필드 기반 필터링을 지원

### D6: 생성 파이프라인 트리거

**결정**: `make generate-templates` Makefile target

**이유**:
- Go 프로젝트에서 `go generate`보다 Makefile이 cross-tool 통합에 적합
- CI에서 `make generate-templates && git diff --exit-code`로 staleness 검사 가능
- 기존 Makefile 인프라와 일관성

## 스킬 호환성 예비 분석

### 높은 호환성 (대부분 텍스트 기반 지침, 플랫폼 의존도 낮음)

tdd, ddd, debugging, refactoring, migration, testing-strategy, security-audit, spec-review, prd, api-design, database, docker, ci-cd, lore-commit, performance, adaptive-quality, verification, double-diamond, experiment, brainstorming, writing-skills, using-autopus, entropy-scan

### 중간 호환성 (도구 참조 대체 필요)

agent-presets, agent-teams, agent-pipeline, context-search, subagent-dev, planning, review, idea, hash-anchored-edit, ax-annotation

### 낮은 호환성 (플랫폼 특정 기능 의존)

- **worktree-isolation**: Claude Code의 워크트리 격리 메커니즘에 강하게 결합
- **git-worktrees**: 유사하게 Claude Code 워크트리 통합에 의존
- **browser-automation / playwright-cli**: Claude Code의 MCP 기반 브라우저 제어에 의존
- **frontend-verify**: 브라우저 자동화 전제

**degradation 전략**: CLI 도구 직접 호출 (`auto pipeline worktree`, `npx playwright`) 로 대체 안내
