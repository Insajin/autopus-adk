# SPEC-RETRO-001 구현 계획

## 태스크 목록

- [ ] T1: retro 스킬 템플릿 작성 (`templates/claude/commands/auto-retro.md.tmpl`)
  - git log 분석 -> SPEC 스캔 -> Lore 패턴 분석 -> 문서 생성 파이프라인 정의
  - `--since`, `--until`, `--spec` 플래그 처리

- [ ] T2: auto-router.md.tmpl에 retro 서브커맨드 라우팅 추가
  - Subcommand Routing 테이블에 retro 행 추가
  - Operational 카테고리에 canary 다음으로 배치

- [ ] T3: codex/gemini 플랫폼 retro 프롬프트 작성
  - `templates/codex/prompts/auto-retro.md.tmpl`
  - `templates/gemini/commands/auto-retro.md.tmpl`

- [ ] T4: Lore 쿼리 활용 로직 설계
  - `pkg/lore/query.go`의 기존 쿼리 함수를 활용하여 기간별 Lore 항목 추출
  - confidence, scope-risk 분포 집계 로직

- [ ] T5: RETRO 문서 템플릿 정의
  - `.autopus/retro/RETRO-{YYYY}-W{NN}.md` 주간 형식
  - `.autopus/retro/RETRO-{YYYY}-{MM}.md` 월간 형식
  - What went well / What didn't / Action items 구조

- [ ] T6: SPEC별 회고 로직 (--spec 플래그)
  - SPEC 디렉토리에서 상태 변경 이력 추적
  - 관련 커밋 필터링 (Ref 트레일러 기반)

- [ ] T7: E2E 시나리오 추가 (scenarios.md)
  - retro 서브커맨드의 기본 실행 시나리오

## 구현 전략

### 기존 코드 활용

- **`pkg/lore/query.go`**: Lore 항목 쿼리. 기간별 필터링 가능. retro의 패턴 분석 데이터 소스.
- **`pkg/lore/types.go`**: `LoreEntry` 구조체 — Confidence, ScopeRisk, Rejected, NotTested 필드가 회고 분석에 직접 활용됨.
- **`pkg/spec/`** (존재 시): SPEC 파일 파싱. 없으면 glob + 텍스트 파싱으로 대체.
- **git log**: `git log --since --until --format` 으로 기간별 커밋 통계 추출.

### 변경 범위

v0.16 초기 구현은 스킬 레벨. 템플릿 3개 신규 + 라우터 수정. Go 코드 변경 없음.
Lore 쿼리가 기간 필터를 지원하지 않으면 `pkg/lore/query.go`에 `QueryByDateRange` 함수 추가 필요 (T4에서 판단).

### 의존성

- `pkg/lore` (의사결정 데이터)
- SPEC-CANARY-001 (canary가 먼저 릴리즈되어 워크플로우 확장 패턴 확립)
- SPEC-LEARN-001 연동 (향후 — Autopus 플랫폼 Learning System)
