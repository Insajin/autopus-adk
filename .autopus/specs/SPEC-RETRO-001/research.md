# SPEC-RETRO-001 리서치

## 기존 코드 분석

### Lore 시스템 (`pkg/lore/`)

- **`types.go`**: `LoreEntry` 구조체 — `Confidence`, `ScopeRisk`, `Rejected`, `NotTested`, `Directive`, `Constraint` 필드. 이들은 retro의 패턴 분석 핵심 데이터.
- **`query.go`**: Lore 쿼리 함수. 기간 기반 필터가 있는지 확인 필요. 없으면 `QueryByDateRange(since, until time.Time)` 추가.
- **`protocol.go`**: Lore 프로토콜 정의. 트레일러 파싱 규칙.
- **`writer.go`**: Lore 항목 작성. retro에서는 읽기만 하므로 직접 사용하지 않음.

### SPEC 파일 구조

- 각 SPEC 디렉토리에 `spec.md` 존재. 첫 줄 `**Status**: {status}` 와 `**Created**: {date}` 로 상태/생성일 파싱 가능.
- `pkg/spec/` 패키지가 있으면 활용, 없으면 glob + regex 파싱.

### git log 활용

- `git log --since="2026-03-27" --until="2026-04-03" --format="%H|%an|%ae|%ad|%s" --numstat` 로 커밋별 통계 추출.
- `git shortlog -sn --since --until` 로 기여자별 커밋 수.
- Lore 트레일러는 `git log --format="%B"` 에서 트레일러 라인 파싱.

### 라우터 구조

- auto-router.md.tmpl의 카테고리:
  - **Development Workflow**: idea, plan, go, sync, fix, dev
  - **Quality & Exploration**: review, search, browse, test
  - **Operational**: setup, version, help
- retro는 Quality & Exploration과 Operational 사이에 새 카테고리 "Feedback Loop"을 만들거나, Operational에 추가 가능. canary와 함께 "Post-deploy" 카테고리로 묶는 것이 자연스러움.

## 설계 결정

### D1: 스킬 레벨 구현 (v0.16 초기)

retro의 핵심은 데이터 수집 + 패턴 분석 + 문서 생성이며, 이는 AI 에이전트의 추론 능력에 크게 의존한다. 따라서 스킬 템플릿에서 git 명령어와 파일 읽기를 조합하는 방식이 적절하다.

**근거**: retro 분석의 "What went well / didn't" 판단은 정량 데이터(커밋 수, 테스트 결과)와 정성 판단(패턴 해석)의 조합이므로, AI 에이전트가 스킬 수준에서 처리하는 것이 자연스럽다. Go 코드로 경직된 분석 로직을 만드는 것보다 유연하다.

**대안 검토**: `pkg/retro/` 패키지 — 통계 집계 부분(커밋 카운트, Lore 분포)은 Go 코드로 구조화하면 정확도가 올라간다. 스킬 레벨에서 시작하고, 통계 정확도 문제가 발견되면 Go 패키지로 승격.

### D2: Lore 의존성

retro의 차별화 포인트는 Lore 데이터 활용이다. 단순 git 통계는 GitHub Insights로도 가능하지만, Lore 트레일러에서 의사결정 패턴을 추출하는 것은 autopus-adk만의 고유 가치.

SPEC-LEARN-001(Autopus 플랫폼 Learning System)과의 연동은 v0.16 이후로, 로컬 retro가 먼저 안정화되어야 의미 있는 데이터를 플랫폼에 전송할 수 있다.

### D3: retro 디렉토리 위치

`.autopus/retro/`는 `.autopus/specs/`, `.autopus/brainstorms/`와 동일한 레벨. doc-storage 규칙에 따라 모듈별 저장. 크로스 모듈 retro는 루트 `.autopus/retro/`에 저장.

### D4: canary와의 관계

- canary: 배포 직후 즉시 검증 (실시간)
- retro: 기간별 회고 (일괄 분석)
- canary 결과 (PASS/WARN/FAIL 히스토리)를 retro 데이터 소스로 활용 가능 (v0.17+)
