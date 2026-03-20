# Autopus-ADK 아키텍처 코드맵

이 디렉토리는 Autopus-ADK 프로젝트의 상세한 아키텍처 문서를 포함합니다.

## 문서 구성

### 📋 overview.md (163줄)

**목적**: Autopus-ADK 아키텍처의 전체 개요

**주요 내용**:
- 프로젝트 소개 및 목표
- 핵심 설계 철학 (5가지 패턴)
  - 어댑터 패턴 (Adapter Pattern)
  - 레지스트리 패턴 (Registry Pattern)
  - 전략 패턴 (Strategy Pattern)
  - 템플릿 패턴 (Template Pattern)
  - 팩토리 패턴 (Factory Pattern)
- 시스템 경계 및 모듈 구조
- 주요 설계 원칙 (5가지)
  - 플랫폼 독립성
  - 확장성
  - 안정성
  - 테스트 가능성
  - 성능

**추천 읽는 순서**: 처음 읽을 문서 (시작점)

---

### 🏗️ modules.md (367줄)

**목적**: 각 모듈의 책임과 인터페이스 명확화

**주요 내용**:
- 18개 모듈의 상세 설명
  - cmd/auto: 애플리케이션 진입점
  - internal/cli: 12개의 Cobra 커맨드 핸들러
  - pkg/adapter: 플랫폼 추상화 계층
  - pkg/adapter/*: 5개 플랫폼별 구현체
  - pkg/config: 설정 관리
  - pkg/content: 콘텐츠 생성
  - pkg/arch: 아키텍처 분석
  - pkg/spec: SPEC 엔진
  - pkg/lore: 의사결정 추적
  - pkg/lsp: LSP 통합
  - pkg/search: 지식 검색
  - pkg/detect: 플랫폼 감지
  - pkg/template: 템플릿 엔진
  - pkg/version: 버전 정보
- 모듈 책임 매트릭스
- 핵심 인터페이스
- 설계 패턴 활용

**추천 읽는 순서**: 2번째 문서 (overview 다음)

---

### 🔗 dependencies.md (389줄)

**목적**: 모듈 간 의존성 분석 및 관계도

**주요 내용**:
- 모듈 간 의존성 그래프
- 7개 계층별 의존성 분석
  - 계층 1: 초저수준 (no dependencies)
  - 계층 2: 기본 의존성
  - 계층 3: 중간 의존성
  - 계층 4: 플랫폼 어댑터
  - 계층 5: 콘텐츠 생성
  - 계층 6: CLI 계층
  - 계층 7: 진입점
- 외부 의존성 상세 분석
  - 표준 라이브러리 (13개)
  - 외부 라이브러리 (4개)
- 의존성 역전 및 순환 의존성 검사
- 의존성 관리 전략
- DOT 형식 의존성 그래프
- 최적화 기회
- 문제 해결 방법

**추천 읽는 순서**: 3번째 문서 (아키텍처 이해 후)

---

### 🎯 entry-points.md (576줄)

**목적**: CLI 진입점과 12개 커맨드의 상세 설명

**주요 내용**:
- 메인 진입점 (cmd/auto/main.go)
- CLI 커맨드 계층 구조
- 12개 서브커맨드 상세 설명
  - `version`: 버전 정보 출력
  - `init`: 하네스 설치
  - `update`: 파일 업데이트
  - `doctor`: 유효성 검증
  - `platform`: 플랫폼 목록
  - `arch`: 아키텍처 분석
  - `spec`: SPEC 파싱
  - `lore`: 의사결정 추적
  - `lsp`: LSP 통합
  - `search`: 지식 검색
  - `skill`: 스킬 관리
  - `hash`: 해시 계산
- 실행 흐름 다이어그램
- 이벤트 핸들러 패턴
- 플래그 및 옵션 종합 테이블

**추천 읽는 순서**: 4번째 문서 (CLI 사용법 이해)

---

### 🌊 data-flow.md (789줄)

**목적**: 상세한 데이터 흐름과 워크플로우 분석

**주요 내용**:
- 시스템 전체 데이터 흐름
- 5개 핵심 워크플로우 상세 분석
  - init 워크플로우 (6단계)
  - update 워크플로우 (마커 기반)
  - doctor 워크플로우 (검증)
  - arch 워크플로우 (분석)
  - spec 워크플로우 (파싱)
- 상태 관리 및 전환 다이어그램
- 데이터 구조 흐름
- 에러 처리 및 복구 전략
- 성능 고려사항
- 요약

**추천 읽는 순서**: 5번째 문서 (구현 상세 이해)

---

## 빠른 참조 가이드

### 아키텍처를 이해하려면?
→ `overview.md` 읽기

### 모듈별 책임을 알려면?
→ `modules.md` 참조

### 모듈 간의 관계를 알려면?
→ `dependencies.md` 검토

### CLI 커맨드를 이해하려면?
→ `entry-points.md` 학습

### 데이터가 어떻게 흐르는지 알려면?
→ `data-flow.md` 분석

---

## 문서 통계

| 문서 | 줄 수 | 크기 | 주제 |
|------|-------|------|------|
| overview.md | 163 | 6.3KB | 아키텍처 개요 |
| modules.md | 367 | 8.7KB | 모듈 카탈로그 |
| dependencies.md | 389 | 8.8KB | 의존성 분석 |
| entry-points.md | 576 | 15KB | CLI 커맨드 |
| data-flow.md | 789 | 23KB | 워크플로우 분석 |
| **합계** | **2,284** | **62KB** | **전체 아키텍처** |

---

## 주요 설계 원칙

### 1. 플랫폼 독립성
PlatformAdapter 인터페이스를 통해 5개 플랫폼을 추상화합니다:
- Claude Code
- Codex
- Gemini CLI
- OpenCode
- Cursor

### 2. 확장성
새로운 플랫폼 추가는 PlatformAdapter 구현과 Registry 등록으로 완료됩니다.

### 3. 안정성
- 마커 기반 파일 업데이트로 사용자 수정 보존
- 검증을 통한 설치 상태 확인
- 강력한 에러 처리 및 복구 메커니즘

### 4. 테스트 가능성
인터페이스 기반 설계로 모킹과 테스팅이 용이합니다.

### 5. 성능
병렬 플랫폼 감지와 효율적인 파일 I/O 처리입니다.

---

## 핵심 데이터 구조

### HarnessConfig
설정 정보를 담는 구조체입니다:
- Mode: "full" 또는 "lite"
- Platforms: 플랫폼 목록
- ContentOptions: 콘텐츠 생성 옵션
- FileSettings: 파일 처리 정책

### PlatformFiles
어댑터가 생성한 파일 목록입니다:
- Files: FileMapping 배열
- Checksum: 전체 체크섬

### FileMapping
단일 파일 매핑 정보입니다:
- SourceTemplate: 소스 템플릿
- TargetPath: 대상 경로
- OverwritePolicy: 덮어쓰기 정책
- Checksum: 파일 체크섬
- Content: 파일 내용

---

## 설계 패턴

### Adapter Pattern
PlatformAdapter 인터페이스로 플랫폼별 차이점을 추상화합니다.

### Registry Pattern
스레드 안전한 Registry로 어댑터를 중앙 관리합니다.

### Strategy Pattern
Full/Lite 모드로 설정 전략을 선택합니다.

### Template Pattern
Go text/template으로 파일을 동적 생성합니다.

### Factory Pattern
DefaultFullConfig(), DefaultLiteConfig() 함수로 설정을 생성합니다.

---

## 사용 가이드

### 1단계: 개요 파악
`overview.md`를 읽어 전체 아키텍처를 이해합니다.

### 2단계: 모듈 학습
`modules.md`를 참조하여 각 모듈의 책임을 파악합니다.

### 3단계: 의존성 확인
`dependencies.md`를 검토하여 모듈 간 관계를 이해합니다.

### 4단계: CLI 학습
`entry-points.md`를 학습하여 커맨드와 옵션을 알아봅니다.

### 5단계: 흐름 분석
`data-flow.md`를 분석하여 데이터가 어떻게 흐르는지 이해합니다.

---

## 개발자 온보딩

새로운 개발자가 Autopus-ADK를 이해하려면:

1. **Day 1**: overview.md와 modules.md 읽기 (2시간)
2. **Day 2**: entry-points.md 학습 및 간단한 커맨드 실행 (2시간)
3. **Day 3**: dependencies.md 검토 및 코드 탐색 (3시간)
4. **Day 4**: data-flow.md 분석 및 실제 워크플로우 추적 (3시간)
5. **Day 5**: 작은 기능 구현 또는 수정 (4시간)

---

## 아키텍처 리뷰

이 코드맵은 다음과 같은 경우에 유용합니다:

- 새로운 플랫폼 지원 추가
- 기존 모듈 리팩토링
- 성능 최적화
- 새로운 개발자 온보딩
- 코드 리뷰 및 검토
- 기술 부채 식별

---

## 문서 유지보수

이 코드맵은 다음과 같이 유지됩니다:

- 주요 아키텍처 변경 시 업데이트
- 새로운 모듈 추가 시 modules.md 업데이트
- 의존성 변경 시 dependencies.md 업데이트
- 새로운 커맨드 추가 시 entry-points.md 업데이트
- 워크플로우 변경 시 data-flow.md 업데이트

---

## 관련 문서

- `.claude/CLAUDE.md`: MoAI 실행 지침
- `.moai/specs/`: 프로젝트 SPEC 문서
- `README.md`: 프로젝트 개요
- `Makefile`: 빌드 및 테스트 명령어

---

## 작성 정보

- **작성 언어**: 한국어 (ko)
- **생성 일시**: 2026-03-20
- **생성 도구**: Autopus-ADK Codemap Generator
- **총 줄 수**: 2,284줄
- **총 크기**: 약 62KB

---

## 연락처 및 피드백

이 문서에 대한 질문이나 피드백은 프로젝트 관리자에게 연락하시기 바랍니다.

---

**행복한 코딩되세요! 🚀**
