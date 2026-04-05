# SPEC-HARN-ENRICH-001 구현 계획

## 태스크 목록

### Phase A: 에이전트 변환 엔진 (Domain 1)

- [ ] T1: `pkg/content/agent_transformer.go` — 에이전트 소스 MD 파싱 및 플랫폼별 변환 엔진 구현
  - frontmatter 파싱 (name, description, model, tools, skills)
  - 본문에서 역할, 작업 원칙, 워크플로우, 제약 섹션 추출
  - Codex TOML 출력 포맷터 (rich developer_instructions 생성)
  - Gemini MD 출력 포맷터
- [ ] T2: `pkg/content/agent_transformer_test.go` — 에이전트 변환 단위 테스트
  - executor.md → executor.toml 변환 검증
  - executor.md → executor.md (Gemini) 변환 검증
  - 도구 참조 매핑 정확성 검증

### Phase B: 스킬 변환 확장 (Domain 2)

- [ ] T3: `pkg/content/skill_transformer_replace.go` — 플랫폼 참조 대체(replacement) 로직
  - 기존 `FilterPlatformReferences`의 삭제 방식 → 대체 방식으로 확장
  - 플랫폼별 도구 매핑 테이블 정의
  - `ReplacePlatformReferences(body, platform) string` 함수
- [ ] T4: `pkg/content/skill_transformer_replace_test.go` — 대체 로직 테스트
  - `Agent(subagent_type="executor", ...)` → `spawn_agent executor --task "..."` 변환 검증
  - `mcp__context7__*` → `WebSearch` 변환 검증
  - `.claude/` → `.codex/` / `.gemini/` 경로 변환 검증
- [ ] T5: `skill_transformer.go` 수정 — `TransformForPlatform`에서 `ReplacePlatformReferences` 호출
  - `FilterPlatformReferences` → `ReplacePlatformReferences`로 교체
  - 하위 호환: claude/claude-code 플랫폼은 기존 동작 유지

### Phase C: 템플릿 생성 (Domain 1 + 2)

- [ ] T6: Codex 에이전트 TOML 템플릿 16개 풍부화
  - `agent_transformer.go`를 사용하여 `content/agents/*.md` → `templates/codex/agents/*.toml.tmpl` 변환
  - 기존 7줄 스텁을 풍부한 developer_instructions로 교체
- [ ] T7: Gemini 에이전트 MD 템플릿 16개 신규 생성
  - `templates/gemini/agents/*.md.tmpl` 디렉토리 생성
  - 에이전트별 역할, 워크플로우, 제약을 Gemini CLI 포맷으로 출력
- [ ] T8: 누락 스킬 34개 × 2 플랫폼 템플릿 생성
  - `skill_transformer.go` 확장된 변환을 사용하여 일괄 생성
  - 플랫폼 호환성 매트릭스 frontmatter 적용
  - 비호환 스킬 식별 및 degradation 문서화

### Phase D: 생성 파이프라인 (Domain 3)

- [ ] T9: `cmd/generate.go` 또는 `Makefile` target — 콘텐츠 소스로부터 템플릿 자동 재생성
  - `make generate-templates` 또는 `go generate ./pkg/content/...`
  - content/agents → templates/{codex,gemini}/agents
  - content/skills → templates/{codex,gemini}/skills

### Phase E: 통합 및 검증

- [ ] T10: `codex_agents.go` 수정 — 풍부화된 TOML 템플릿 연동 확인
- [ ] T11: Gemini adapter에 에이전트 생성 로직 추가 (있는 경우)
- [ ] T12: 전체 `make test` 통과 및 기존 테스트 회귀 없음 확인

## 구현 전략

1. **기존 코드 활용**: `skill_transformer.go`의 파싱 인프라(frontmatter 분리, YAML 파싱)를 에이전트 변환에도 재활용
2. **점진적 확장**: `FilterPlatformReferences`를 깨지 않고 `ReplacePlatformReferences`를 별도 함수로 추가한 뒤 `TransformForPlatform`에서 교체
3. **변환 범위**: 16개 에이전트 × 2 플랫폼 + 34개 스킬 × 2 플랫폼 = 100+ 파일 생성. 자동화 필수
4. **300줄 제한**: 변환 로직을 concern별로 분리 (파싱, 매핑, 포매팅)

## 병렬화 가능 태스크

- T1, T3: 독립적으로 병렬 실행 가능 (에이전트 변환 ↔ 스킬 대체 로직)
- T6, T7, T8: T1+T3 완료 후 병렬 실행 가능
- T10, T11: T6+T7 완료 후 병렬 실행 가능
