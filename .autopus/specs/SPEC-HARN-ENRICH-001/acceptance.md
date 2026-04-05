# SPEC-HARN-ENRICH-001 수락 기준

## 시나리오

### S1: Codex 에이전트 TOML 풍부화

- Given: `content/agents/executor.md`에 역할, TDD 원칙, Phase 연동, 브랜딩 등 40줄+ 본문이 정의되어 있음
- When: 에이전트 변환기가 executor.md를 Codex TOML로 변환
- Then: `templates/codex/agents/executor.toml.tmpl`의 `developer_instructions.text`가 역할 정의, 작업 원칙, 워크플로우 단계, 완료 기준, 제약 사항을 포함하며 최소 200자 이상

### S2: Gemini 에이전트 정의 생성

- Given: `content/agents/executor.md`에 에이전트 정의가 존재
- When: 에이전트 변환기가 executor.md를 Gemini 포맷으로 변환
- Then: `templates/gemini/agents/executor.md.tmpl` 파일이 생성되고, 역할, 워크플로우, 제약 섹션이 포함됨

### S3: 도구 참조 매핑 — Agent() → spawn_agent

- Given: 스킬 콘텐츠에 `Agent(subagent_type="executor", task="implement feature")` 호출이 포함
- When: Codex 플랫폼으로 변환
- Then: 출력에 `spawn_agent executor --task "implement feature"` 형태로 대체됨

### S4: 도구 참조 매핑 — MCP → WebSearch

- Given: 스킬 콘텐츠에 `mcp__context7__resolve-library-id` 호출이 포함
- When: Codex 또는 Gemini 플랫폼으로 변환
- Then: 출력에 `WebSearch "library docs"` 또는 동등한 대체 참조가 포함됨

### S5: 도구 참조 매핑 — 경로 변환

- Given: 스킬 콘텐츠에 `.claude/agents/autopus/` 경로가 포함
- When: Codex 플랫폼으로 변환
- Then: `.codex/agents/` 경로로 대체됨
- When: Gemini 플랫폼으로 변환
- Then: `.gemini/agents/` 경로로 대체됨

### S6: 누락 스킬 34개 포팅

- Given: `content/skills/` 디렉토리에 40개 스킬이 존재하고 Codex/Gemini에는 6개만 포팅됨
- When: 스킬 변환기가 전체 스킬을 Codex와 Gemini로 변환
- Then: `templates/codex/skills/`에 34개 이상 신규 `.md.tmpl` 파일이 생성됨
- And: `templates/gemini/skills/`에 34개 이상 신규 `.md.tmpl` 파일이 생성됨

### S7: 플랫폼 비호환 스킬 처리

- Given: `content/skills/worktree-isolation.md`가 Claude Code의 워크트리 격리 기능에 의존
- When: 해당 스킬의 Codex 호환성을 검사
- Then: 호환성 매트릭스에서 `platforms: [claude]`로 표시되거나, degradation 대체 절차가 문서화됨

### S8: 기존 FilterPlatformReferences 하위 호환

- Given: 기존 `FilterPlatformReferences`가 claude/claude-code 플랫폼에서 콘텐츠를 그대로 반환
- When: `ReplacePlatformReferences`로 교체 후 claude 플랫폼으로 변환
- Then: 출력이 기존과 동일 (변경 없음)

### S9: 에이전트 소스 변경 시 템플릿 재생성

- Given: `content/agents/executor.md`의 본문을 수정
- When: `make generate-templates` 또는 동등한 생성 명령 실행
- Then: `templates/codex/agents/executor.toml.tmpl`과 `templates/gemini/agents/executor.md.tmpl`이 수정된 내용을 반영하여 재생성됨

### S10: 300줄 파일 크기 제한

- Given: 모든 신규 Go 소스 파일
- When: 줄 수를 검사
- Then: 모든 파일이 300줄 이하
