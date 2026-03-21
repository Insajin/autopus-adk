# SPEC-SETUP-001 구현 계획

## 태스크 목록

### Phase 1: `/auto setup` 서브커맨드 추가

- [x] T1: `auto.md`에 setup 서브커맨드 정의 추가
- [x] T2: setup 워크플로우 구현 (코드베이스 분석 → 문서 생성)
  - ARCHITECTURE.md: 기존 `auto arch generate` 로직 호출
  - `.autopus/project/product.md`: go.mod, autopus.yaml, 소스 분석
  - `.autopus/project/structure.md`: 디렉토리 트리 + 패키지 설명
  - `.autopus/project/tech.md`: 기술스택 분석

### Phase 2: 세션 컨텍스트 로드

- [x] T3: `auto.md`의 서브커맨드 라우팅 앞에 컨텍스트 로드 지시 추가
  - 문서 존재 시 → Read로 로드
  - 문서 미존재 시 → `/auto setup` 안내

### Phase 3: `/auto sync` 통합

- [x] T4: `auto.md`의 sync 섹션에 프로젝트 문서 업데이트 단계 추가
  - ARCHITECTURE.md 재생성
  - `.autopus/project/*` 갱신

## 구현 전략

- CLI 바이너리(`auto arch generate`)가 아닌 에이전트 레벨에서 처리
- `auto.md` 슬래시 커맨드 정의만 수정하면 됨 (Go 코드 변경 불필요)
- 에이전트가 Explore + 분석 후 Write로 문서 생성
