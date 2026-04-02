---
id: SPEC-PARITY-001
type: acceptance
---

# SPEC-PARITY-001 수락 기준

## Domain 1: Codex 룰 격리

### S1: Codex 룰 파일 생성

- Given: `content/rules/`에 9개 룰 파일이 존재 (branding, project-identity 제외 시 7개)
- When: `auto init --platform codex` 실행
- Then:
  - `.codex/rules/autopus/`에 최소 7개 `.md` 파일 생성
  - 각 파일 내용이 `content/rules/` 소스와 의미적으로 동일
  - `branding.md`는 제외됨

### S2: AGENTS.md 경량화

- Given: 기존 `.codex/AGENTS.md`에 인라인 Rules 섹션이 포함
- When: `auto init --platform codex` 실행 (R1 적용 후)
- Then:
  - `.codex/AGENTS.md`에 인라인 Rules 섹션이 없음
  - 대신 `See .codex/rules/autopus/ for detailed guidelines` 참조문 존재
  - `.codex/AGENTS.md` 라인 수가 이전 대비 50% 이상 감소

### S3: Codex 서브디렉토리 fallback

- Given: Codex CLI가 `.codex/rules/` 서브디렉토리를 지원하지 않는 환경
- When: `auto init --platform codex` 실행
- Then:
  - `.codex/rules-autopus-lore-commit.md` 등 flat naming으로 룰 파일 생성
  - AGENTS.md 참조문이 flat 경로를 반영

## Domain 2: Codex 에이전트 확장

### S4: 16개 에이전트 전체 생성

- Given: `content/agents/`에 16개 에이전트 정의 존재
- When: `auto init --platform codex` 실행
- Then:
  - `.codex/agents/`에 16개 `.toml` 파일 생성
  - 각 파일에 `name`, `model`, `instructions` 필드 존재
  - 기존 5개 에이전트 (debugger, executor, planner, reviewer, tester)의 내용이 변경되지 않음

### S5: 에이전트 TOML 스키마 유효성

- Given: 생성된 16개 `.toml` 파일
- When: TOML 파서로 검증
- Then:
  - 모든 파일이 유효한 TOML
  - `name` 값이 content/agents 파일명과 매칭
  - `instructions`가 비어 있지 않음

## Domain 3: Gemini 누락 룰 추가

### S6: Gemini 룰 7개 완성

- Given: `templates/gemini/rules/autopus/`에 4개 룰 템플릿 존재
- When: 3개 템플릿 추가 후 `auto init --platform gemini` 실행
- Then:
  - `.gemini/rules/autopus/`에 7개 `.md` 파일 생성
  - worktree-safety.md, context7-docs.md, doc-storage.md 포함
  - 기존 4개 룰 파일 내용이 변경되지 않음

### S7: Gemini 룰 자동 포함 검증

- Given: `gemini_rules.go`의 디렉토리 스캔 로직
- When: 새 `.tmpl` 파일 추가
- Then: 코드 수정 없이 `prepareRuleMappings()`이 새 파일을 자동 포함

## Domain 4: 스킬 변환

### S8: 확장 스킬 변환

- Given: `content/skills/`에 40개 스킬 파일 (`platforms` 미지정 시 전체 호환 간주 — R6 규칙)
- When: 스킬 변환 엔진 실행
- Then:
  - Codex: `.codex/skills/`에 플랫폼-agnostic 스킬 `.md` 파일 생성 (호환 스킬 수는 변환 엔진이 산출)
  - Gemini: `.gemini/skills/`에 플랫폼-agnostic 스킬 디렉토리 생성
  - 플랫폼 특정 참조 (MCP tool 호출 등)가 제거 또는 주석 처리됨

### S9: 호환성 필터링

- Given: `content/skills/browser-automation.md`에 `platforms: [claude]` frontmatter
- When: Codex 스킬 변환 실행
- Then: 해당 스킬이 `.codex/skills/`에 생성되지 않음
- And: 변환 리포트에 비호환 사유 표시

## Domain 5: Codex 훅 병합

### S10: 사용자 훅 보존

- Given: `.codex/hooks.json`에 사용자 커스텀 훅 `{"event":"custom","command":"echo hello"}` 존재
- When: `auto update --platform codex` 실행
- Then:
  - 사용자 커스텀 훅이 그대로 보존
  - Autopus 훅만 업데이트됨 (마커 기반 식별)

### S11: 첫 설치 시 훅 생성

- Given: `.codex/hooks.json` 미존재
- When: `auto init --platform codex` 실행
- Then: hooks.json에 Autopus 마커 훅만 생성

## Domain 6: Gemini 훅 중복 방지

### S12: 반복 호출 멱등성

- Given: `.gemini/settings.json`에 AfterAgent 훅 `{"command":"/tmp/orchestra.sh"}` 존재
- When: `InjectOrchestraAfterAgentHook("/tmp/orchestra.sh")` 재호출
- Then:
  - AfterAgent 배열에 동일 command 훅이 1개만 존재
  - 기존 다른 AfterAgent 훅은 보존

### S13: 신규 훅 추가

- Given: `.gemini/settings.json`에 AfterAgent 훅 없음
- When: `InjectOrchestraAfterAgentHook("/tmp/new.sh")` 호출
- Then: AfterAgent 배열에 1개 훅 추가

## Domain 7: 크로스-플랫폼 스키마

### S14: 라운드트립 직렬화

- Given: `PlatformSettings` 구조체에 agents, rules, skills 설정
- When: `ToClaudeJSON()` → 파싱 → 원본 비교
- Then: 모든 필드가 동일 (3개 플랫폼 모두)

## Domain 8: 패리티 테스트 게이트

### S15: 패리티 퍼센트 검증

- Given: 3개 어댑터 Generate() 결과
- When: 패리티 테스트 실행
- Then:
  - agents 패리티 ≥ 95% (16/16 = 100%)
  - rules 패리티 ≥ 95% (7/7 = 100%)
  - skills 패리티: 리포트 생성 (P1이므로 fail 아님)

### S16: P0 패리티 미달 시 실패

- Given: Codex 에이전트가 14개만 생성 (2개 누락)
- When: 패리티 테스트 실행
- Then: 테스트 FAIL (`agents parity 87.5% < 95% threshold`)
