# SPEC-CANARY-001 구현 계획

## 태스크 목록

- [ ] T1: canary 스킬 템플릿 작성 (`templates/claude/commands/auto-canary.md.tmpl`)
  - 빌드 검증 -> E2E 실행 -> 브라우저 검진 -> 판정 파이프라인 정의
  - `--url`, `--watch`, `--compare` 플래그 처리 로직

- [ ] T2: auto-router.md.tmpl에 canary 서브커맨드 라우팅 추가
  - Subcommand Routing 테이블에 canary 행 추가
  - Operational 카테고리에 배치 (sync 다음)

- [ ] T3: codex/gemini 플랫폼 canary 프롬프트 작성
  - `templates/codex/prompts/auto-canary.md.tmpl`
  - `templates/gemini/commands/auto-canary.md.tmpl`

- [ ] T4: sync 완료 후 canary 안내 메시지 추가
  - sync 스킬 템플릿의 완료 메시지에 "다음: /auto canary" 안내 삽입

- [ ] T5: canary 결과 저장 형식 정의
  - `.autopus/canary/latest.json` 스키마 설계
  - 커밋별 스냅샷 저장/조회 로직

- [ ] T6: E2E 시나리오 추가 (scenarios.md)
  - canary 서브커맨드 자체의 E2E 시나리오 작성

## 구현 전략

### 기존 코드 활용

- **`pkg/e2e`**: E2E 시나리오 파싱(`scenario.go`) 및 실행(`runner.go`)은 이미 구현되어 있음. canary는 이를 스킬 레벨에서 호출하는 형태.
- **`pkg/browse`**: 브라우저 자동화(`agent.go`, `cmux.go`)가 이미 존재. 접근성 트리 스냅샷과 콘솔 에러 수집에 활용.
- **`pkg/e2e/build.go`**: 빌드 실행 로직 재사용.

### 변경 범위

canary는 스킬 레벨 기능이므로 Go 코드 변경은 최소화. 주요 작업은 템플릿 파일 3개 신규 + router 수정 1개.

### 의존성

- `pkg/e2e` (시나리오 실행)
- `pkg/browse` (브라우저 검진)
- `scenarios.md` (시나리오 정의)
- `tech.md` (프로젝트 타입 감지)
