# SPEC-SETUP-001: 프로젝트 컨텍스트 문서 생성 및 동기화

**Status**: done
**Created**: 2026-03-21
**Domain**: SETUP

## 목적

에이전트는 매 세션마다 초기화된다. 프로젝트의 아키텍처, 구조, 기술스택 정보를 문서로 유지하여 세션 시작 시 빠르게 컨텍스트를 복원한다.

ARCHITECTURE.md는 Autopus-ADK의 차별화 기능이며, 프로젝트 문서는 `.autopus/project/` 네임스페이스에 생성한다.

## 워크플로우 통합

| 시점 | 동작 | 트리거 |
|------|------|--------|
| 최초 설정 | 문서 생성 | `/auto setup` |
| 세션 시작 | 문서 로드 (읽기 전용) | `/auto` 실행 시 자동 |
| 워크플로우 완료 | 문서 업데이트 | `/auto sync` |
| 수동 갱신 | 문서 재생성 | `/auto setup` |

## 요구사항

### R1: 문서 생성 (setup)

- WHEN 사용자가 `/auto setup`을 실행하면, THE SYSTEM SHALL 코드베이스를 분석하여 다음 파일을 생성한다:
  - `ARCHITECTURE.md` — 도메인, 레이어, 의존성 맵 (기존 `auto arch generate` 활용)
  - `.autopus/project/product.md` — 프로젝트 설명, 핵심 기능, 유스케이스
  - `.autopus/project/structure.md` — 디렉토리 구조, 패키지 역할
  - `.autopus/project/tech.md` — 기술스택, 빌드, 테스트, 패턴

- WHEN 해당 파일이 이미 존재하면, THE SYSTEM SHALL 현재 코드베이스 상태를 반영하여 업데이트한다.

### R2: 세션 컨텍스트 로드

- WHEN `/auto` 서브커맨드가 실행되면, THE SYSTEM SHALL 다음 문서를 우선 로드하도록 안내한다:
  1. `ARCHITECTURE.md`
  2. `.autopus/project/product.md`
  3. `.autopus/project/structure.md`
  4. `.autopus/project/tech.md`

- WHEN 문서가 존재하지 않으면, THE SYSTEM SHALL `/auto setup` 실행을 안내한다.

### R3: 동기화 (sync 통합)

- WHEN `/auto sync`가 실행되면, THE SYSTEM SHALL SPEC 상태 갱신과 함께 프로젝트 문서도 업데이트한다.
- WHILE 코드베이스에 구조적 변경이 있는 동안, THE SYSTEM SHALL ARCHITECTURE.md와 structure.md를 갱신한다.

## 생성 파일 상세

### ARCHITECTURE.md

기존 `pkg/arch` 패키지의 `Analyze()` + `Generate()` 활용:
- Domains: 패키지별 도메인 분류
- Layers: cmd/pkg/internal 레이어 구조
- Dependencies: import 기반 의존성 그래프
- Violations: 아키텍처 규칙 위반

### .autopus/project/product.md

- 프로젝트명, 모듈 경로
- 바이너리명, 설치 모드 (Full/Lite)
- 핵심 기능 목록
- 플랫폼 어댑터 목록

### .autopus/project/structure.md

- 디렉토리 트리
- 각 패키지의 역할 (한 줄 설명)
- 엔트리포인트 위치
- 파일 수, 테스트 파일 수

### .autopus/project/tech.md

- 언어, 프레임워크, 주요 의존성
- 빌드 시스템 (Makefile, ldflags)
- 테스트 명령어, 커버리지 기준
- 아키텍처 패턴 (Adapter, Strategy 등)
