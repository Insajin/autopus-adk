# Autopus-ADK 제품 가이드

## 프로젝트 개요

### 프로젝트명
Autopus-ADK (Autopus Agentic Development Kit)

### 설명
Autopus-ADK는 AI 기반 코딩 CLI (Claude Code, Codex, Gemini CLI, OpenCode, Cursor)에 Autopus 하네스를 설치하는 Go 기반 셋업 도구입니다. 프로젝트 아키텍처를 분석하고, 개발 표준을 적용하며, 자동화된 워크플로우를 구성하는 통합 개발 환경을 제공합니다.

### 대상 사용자
- AI 코딩 도구 사용자 (개발자, 엔지니어)
- 개발팀 리더 (팀 표준화 및 자동화)
- DevOps 엔지니어 (설정 관리 및 배포)

### 핵심 컨셉
Autopus-ADK는 단순한 설정 도구를 넘어, 다음을 통해 개발 생산성을 극대화합니다:
1. **플랫폼 감지 및 자동 어댑테이션**: 설치된 코딩 CLI 자동 감지 및 플랫폼별 최적화 설정
2. **하네스 설치**: Full/Lite 모드를 통한 유연한 설치 및 AUTOPUS:BEGIN/END 마커 기반 업데이트
3. **아키텍처 분석**: 프로젝트 구조 자동 분석 및 ARCHITECTURE.md 생성
4. **SPEC 엔진**: EARS 형식 요구사항 파싱 및 SPEC 문서 자동 생성
5. **결정 기록**: 9-trailer 깃 커밋 프로토콜을 통한 아키텍처 의사결정 추적

---

## 핵심 기능 상세 설명

### 1. 플랫폼 감지 및 어댑테이션 (Platform Detection & Adaptation)
**목표**: 여러 코딩 CLI 환경에서 자동으로 작동하는 통일된 경험 제공

**구현 방식**:
- 5개 플랫폼 어댑터 자동 로드 (Claude Code, Codex, Gemini CLI, OpenCode, Cursor)
- 각 플랫폼의 설치 위치, 설정 구조, 지원 기능 감지
- 플랫폼별 템플릿 렌더링 및 파일 배치
- 플랫폼 검증 및 상태 리포트

**사용 사례**:
- 새 팀원이 Autopus-ADK 실행 시 자동으로 현재 환경에 맞는 설정 적용
- 여러 CLI 동시 사용 시 각각 최적화된 하네스 설치

---

### 2. 하네스 설치 및 업데이트 (Harness Installation & Update)
**목표**: 사용자 커스터마이제이션을 보존하면서 안전한 설치 및 업데이트

**Full 모드 (포괄적)**:
- 완전한 Autopus 프레임워크 설치
- 모든 에이전트, 스킬, 훅, 워크플로우 포함
- 프로젝트 아키텍처 분석 및 ARCHITECTURE.md 생성
- LSP 통합, 검색 기능, 콘텐츠 생성 도구 포함

**Lite 모드 (최소화)**:
- 핵심 기능만 설치 (에이전트, 기본 스킬, 필수 훅)
- 대역폭/저장소 제약이 있는 환경에 최적화
- 필요시 Full 모드로 업그레이드 가능

**마커 기반 업데이트 (AUTOPUS:BEGIN/END)**:
- 기존 설정 파일에 AUTOPUS:BEGIN과 AUTOPUS:END 마커 삽입
- 마커 섹션만 선택적으로 업데이트하여 사용자 수정사항 보존
- 충돌 감지 및 병합 전략 자동 적용

---

### 3. 아키텍처 분석 및 생성 (Architecture Analysis & Generation)
**목표**: 프로젝트 구조를 자동으로 이해하고 문서화

**분석 기능**:
- 디렉토리 계층 구조 스캔 및 모듈 관계도 생성
- 주요 파일 식별 (entry points, interfaces, configurations)
- 의존성 그래프 분석
- 리스크 영역 감지 (순환 의존성, 높은 복잡도)

**생성 기능**:
- ARCHITECTURE.md 자동 생성 (시스템 다이어그램, 컴포넌트 설명)
- Mermaid 다이어그램 생성 (C4 모델, 의존성 그래프)
- 아키텍처 린팅 (패턴 위반, 모범 사례 검증)

**사용 사례**:
- 신입 개발자 온보딩 시 자동 생성된 아키텍처 문서 제공
- 리팩토링 전 현재 상태 정확히 파악
- PR 리뷰 시 구조적 영향 범위 명확화

---

### 4. SPEC 엔진 (SPEC Engine)
**목표**: EARS 형식 요구사항으로부터 SPEC 문서 자동 생성

**EARS 형식 지원**:
- **Ubiquitous**: 항상 활성화된 요구사항 ("사용자가 로그인하면 토큰 발급")
- **Event-driven**: 이벤트 기반 요구사항 ("X 이벤트 발생 시 Y 수행")
- **State-driven**: 상태 기반 요구사항 ("상태 X일 때 Y 가능")
- **Unwanted**: 금지 요구사항 ("X는 하지 않음")
- **Optional**: 선택적 요구사항 ("가능하면 X 구현")

**SPEC 생성 프로세스**:
1. 요구사항 파일(EARS 형식) 파싱
2. SPEC 템플릿 선택 (기능, API, 모듈, 통합 등)
3. 요구사항 → SPEC 필드 자동 매핑
4. SPEC 검증 (필드 완전성, 형식 정확성)
5. SPEC 문서 생성 (.moai/specs/SPEC-XXX/spec.md)

**사용 사례**:
- 제품 요구사항 명세 → SPEC 문서 자동 변환
- SPEC 검증을 통한 요구사항 품질 확보
- 개발 팀과 제품 팀 간 명확한 의사소통

---

### 5. 결정 기록 시스템 (Lore - Decision Tracking)
**목표**: 아키텍처 의사결정을 체계적으로 기록 및 추적

**9-Trailer 깃 커밋 프로토콜**:
```
<타입>: <제목> [<태그>]

<본문 (2-4줄)>

<이유: 의사결정 배경>
<영향: 코드베이스 영향 범위>
<대안: 검토한 다른 옵션들>
<리스크: 예상 위험 사항>
<팀: 승인자 정보>
<참고: 관련 이슈/SPEC 링크>
<날짜: ISO 8601 형식>
<버전: 의사결정 버전>
```

**의사결정 저장소**:
- .lore/ 디렉토리에 구조화된 YAML 형식으로 저장
- 타임스탬프, 작성자, 태그로 검색 가능
- 이전 의사결정 이유 및 변경사항 추적

**사용 사례**:
- "왜 이 기술 스택을 선택했는가?" 질문에 즉시 답변 제공
- 6개월 후 인수인계 시 의사결정 맥락 이해
- 아키텍처 리뷰 시 일관성 검증

---

### 6. LSP 통합 (Language Server Protocol Integration)
**목표**: IDE의 언어 서버와 통합하여 실시간 진단 및 코드 품질 검증

**LSP 클라이언트 기능**:
- 언어 서버 자동 감지 (Python, TypeScript, Go, Rust 등)
- 타입 에러, 린트 에러, 보안 경고 실시간 수집
- LSP 진단 결과 파싱 및 분류

**LSP 명령어 지원**:
- `lsp status`: 연결된 언어 서버 상태 확인
- `lsp diagnose`: 프로젝트 진단 결과 리포트 (에러/경고/정보)
- `lsp check`: 특정 파일 또는 디렉토리 검증

**사용 사례**:
- CI/CD 파이프라인에서 코드 품질 게이트 (0 에러, 경고 < 10)
- 개발 중 타입 안정성 실시간 모니터링
- 배포 전 품질 기준 자동 검증

---

### 7. 지식 검색 (Knowledge Search)
**목표**: Context7 라이브러리 및 웹 기반 검색으로 개발 참고 자료 제공

**검색 소스**:
- **Context7 MCP**: 공식 라이브러리 문서 (API 문서, 가이드)
- **Exa API**: 웹 기반 고품질 검색 결과
- **해시 기반 검색**: 프로젝트 내 유사 패턴 검색

**검색 명령어**:
- `search query`: 키워드로 라이브러리 및 웹 검색
- `search --library <name>`: 특정 라이브러리 검색
- `search --hash <value>`: 코드 패턴 해시 기반 검색

**사용 사례**:
- 개발 중 비동기 처리 패턴 빠르게 찾기
- 라이브러리 API 문서 즉시 확인
- 프로젝트 내 유사 구현 찾아 재사용

---

### 8. 콘텐츠 생성 (Content Generation)
**목표**: 보일러플레이트 코드, 에이전트, 스킬 등 자동 생성

**생성 가능 콘텐츠**:
- **에이전트**: 역할, 권한, 프롬프트 자동 생성
- **스킬**: Quick Reference, Implementation, Advanced 섹션 포함
- **훅**: SessionStart, PreToolUse, PostToolUse 등 사전 정의 훅
- **워크플로우**: Plan-Run-Sync 자동화 워크플로우
- **방법론**: TDD/DDD 방법론 가이드 및 템플릿
- **의도 라우팅**: 사용자 입력 → 에이전트/스킬 자동 라우팅 설정
- **MX 태그**: 코드 주석 자동 생성 (@MX:NOTE, @MX:WARN, @MX:ANCHOR)

**생성 명령어**:
- `skill generate --name <name>`: 스킬 템플릿 생성
- `agent generate --role <role>`: 에이전트 템플릿 생성
- `hook generate --event <event>`: 훅 템플릿 생성

**사용 사례**:
- 팀 온보딩 시 스킬 템플릿 빠르게 생성
- 신규 기능 개발 시 TDD 관련 파일 일괄 생성
- 코드 품질 표준 자동 적용 (MX 태그)

---

### 9. 템플릿 엔진 (Template Engine)
**목표**: Go text/template을 확장하여 플랫폼별 설정 렌더링

**템플릿 특징**:
- Go text/template 기반 강력한 템플릿 언어
- 플랫폼별 맞춤형 FuncMap (함수 라이브러리)
- YAML/JSON 변수 주입
- 조건부 렌더링, 반복, 함수 호출

**템플릿 디렉토리 구조**:
```
templates/
├── shared/           # 모든 플랫폼 공통 템플릿
├── claude/           # Claude Code 특화 템플릿
├── codex/            # Codex 특화 템플릿
├── gemini/           # Gemini CLI 특화 템플릿
├── opencode/         # OpenCode 특화 템플릿
└── cursor/           # Cursor 특화 템플릿
```

**사용 사례**:
- 플랫폼별로 다른 구조의 설정 파일 자동 생성
- 프로젝트 구성에 따라 동적으로 파일 생성
- 다국어 지원 (템플릿 변수로 언어 지정)

---

## 듀얼 모드 비교

### Full 모드 (포괄적)

**설치 내용**:
- 완전한 Autopus 프레임워크
- 모든 에이전트 (8개): spec, ddd, tdd, docs, quality, project, strategy, git
- 모든 스킬 (13개): foundation, library, workflow 관련 스킬
- 모든 훅: SessionStart, PreToolUse, PostToolUse, Notification 등
- 아키텍처 분석 및 ARCHITECTURE.md
- LSP 통합, 검색 기능, 콘텐츠 생성

**파일 크기**:
- 설치 파일: ~500KB
- 디스크 공간: ~1MB

**설치 시간**:
- 약 2-3분

**대상 사용자**:
- 큰 팀 (5명 이상)
- 완전한 자동화를 원하는 팀
- 복잡한 프로젝트 관리 필요

---

### Lite 모드 (최소화)

**설치 내용**:
- 핵심 에이전트 (2개): spec, ddd
- 필수 스킬 (3개): foundation-core, workflow-docs, library-nextra
- 필수 훅: SessionStart, PostToolUse
- 기본 콘텐츠 생성
- 아키텍처 분석 (ARCHITECTURE.md 제외)

**파일 크기**:
- 설치 파일: ~100KB
- 디스크 공간: ~200KB

**설치 시간**:
- 약 30초

**대상 사용자**:
- 소규모 팀 (1-3명)
- 개인 프로젝트
- 빠른 설정이 필요한 경우
- 저장소/대역폭 제약 환경

---

## 주요 유스케이스

### 초기 하네스 설치
```bash
auto init --mode full
```
1. 플랫폼 감지
2. 설정 파일 생성
3. 하네스 파일 설치
4. 플랫폼별 훅 설정
5. 검증 및 상태 리포트

---

### 하네스 업데이트
```bash
auto update
```
1. 최신 버전 확인
2. 변경사항 분석
3. AUTOPUS:BEGIN/END 마커 기반 업데이트
4. 사용자 수정사항 보존
5. 검증 및 롤백 가능

---

### 프로젝트 아키텍처 검증
```bash
auto arch
```
1. 프로젝트 구조 분석
2. 모듈 관계도 생성
3. 아키텍처 린팅 (패턴 위반 감지)
4. ARCHITECTURE.md 생성
5. 개선 권고사항 제시

---

### SPEC 문서 생성
```bash
auto spec generate --file requirements.ears
```
1. EARS 형식 요구사항 파싱
2. SPEC 템플릿 적용
3. 필드 자동 매핑
4. 검증 및 오류 리포트
5. SPEC 문서 생성

---

### LSP 검증
```bash
auto lsp check
```
1. 언어 서버 상태 확인
2. 프로젝트 전체 진단
3. 에러/경고/정보 수집
4. 결과 리포트 (카테고리별 정렬)
5. 개선 권고사항

---

## 기술 스택

| 카테고리 | 기술 |
|---------|------|
| 언어 | Go 1.23 |
| CLI 프레임워크 | Cobra |
| 설정 형식 | YAML (yaml.v3) |
| 해싱 | xxhash/v2 |
| 테스트 | testify |

---

## 개발 환경 요구사항

**필수**:
- Go 1.23 이상
- make (빌드용)
- git (버전 관리)

**권장**:
- Docker (개발 환경 격리)
- VS Code 또는 GoLand (IDE)
- golangci-lint (코드 검증)

---

## 다음 단계

1. **설치**: `make build` 후 `./bin/auto init`
2. **검증**: `./bin/auto doctor`로 설치 상태 확인
3. **문서**: `./bin/auto docs` 또는 [공식 문서](https://autopus-adk.dev)
4. **지원**: GitHub Issues로 문제 보고, Discussions로 질문

---

## 라이선스 및 기여

이 프로젝트는 Anthropic에서 개발하고 관리합니다.
- 라이선스: MIT
- 기여: pull request 환영
- 문제 보고: GitHub Issues

---

*마지막 업데이트: 2026-03-20*
*버전: 6.1.0*
