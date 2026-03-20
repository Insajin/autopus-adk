# Autopus-ADK 진입점 및 이벤트 핸들러

## 메인 진입점

### cmd/auto/main.go

**진입점**: `main()` 함수

```
main()
  └── cli.Execute()
      └── NewRootCmd().Execute()
          ├── 커맨드 파싱
          ├── 플래그 처리
          └── 해당 커맨드 핸들러 실행
```

**역할**: 프로그램을 시작하고 CLI 실행 엔진에 제어권을 넘김

---

## CLI 커맨드 계층

### internal/cli/root.go

**진입점**: `Execute()` 함수

```go
func Execute() {
  if err := NewRootCmd().Execute(); err != nil {
    fmt.Fprintf(os.Stderr, "Error: %v\n", err)
    os.Exit(1)
  }
}
```

**책임**:
1. 루트 Cobra 커맨드 생성
2. 모든 서브커맨드 등록
3. 오류 처리 및 종료

**플래그**:
- `--verbose, -v`: 상세 출력 활성화
- `--config`: 설정 파일 경로

---

## 12개의 서브커맨드

### 1. version 커맨드

**파일**: `internal/cli/root.go` (newVersionCmd)

**목적**: 버전 정보 출력

**실행 흐름**:
```
version
  └── version.String()
      ├── 버전 번호
      ├── Git 커밋 해시
      ├── 빌드 날짜
      └── 빌드 환경
```

**출력**: 포맷된 버전 정보

---

### 2. init 커맨드

**파일**: `internal/cli/init.go`

**목적**: Autopus 하네스를 코딩 CLI에 설치

**실행 흐름**:
```
init
  ├── 플래그 파싱 (--mode, --platform 등)
  ├── 플랫폼 자동 감지 (adapter.DetectAll)
  │   ├── Claude Code 감지
  │   ├── Codex 감지
  │   ├── Gemini CLI 감지
  │   ├── OpenCode 감지
  │   └── Cursor 감지
  ├── 설정 로드 또는 기본값 생성
  │   ├── DefaultFullConfig() 또는 DefaultLiteConfig()
  │   └── YAML 파일에서 로드 (지정된 경우)
  ├── 각 감지된 플랫폼별 파일 생성
  │   ├── adapter.Generate()
  │   ├── 템플릿 렌더링
  │   └── PlatformFiles 반환
  ├── 파일 시스템에 쓰기
  │   └── WriteFiles()
  └── 성공 메시지 출력
```

**주요 변수**:
- `mode`: "full" 또는 "lite"
- `configPath`: 설정 파일 경로
- `platforms`: 특정 플랫폼 지정 (기본: 감지된 모든 플랫폼)

**에러 처리**:
- 플랫폼 미감지 → 사용자에게 수동 선택 요청
- 파일 쓰기 실패 → 오류 메시지 출력
- 설정 로드 실패 → 기본값 사용 또는 오류 반환

---

### 3. update 커맨드

**파일**: `internal/cli/update.go`

**목적**: 기존 하네스 파일 업데이트 (사용자 수정 보존)

**실행 흐름**:
```
update
  ├── 기존 설정 로드
  │   └── LoadConfig()
  ├── 감지된 플랫폼 목록 조회
  ├── 각 플랫폼별 파일 업데이트
  │   ├── adapter.Update()
  │   ├── 마커 기반 부분 업데이트
  │   │   └── AUTOPUS:BEGIN/END 사이 부분만 갱신
  │   └── 사용자 수정 섹션 보존
  ├── 파일 시스템에 쓰기
  │   └── WriteFiles()
  └── 변경 사항 요약 출력
```

**마커 기반 업데이트**:
- `AUTOPUS:BEGIN`: 관리되는 섹션 시작
- 사이: Autopus 관리 콘텐츠 (덮어쓰기 대상)
- `AUTOPUS:END`: 관리되는 섹션 종료
- 마커 외부: 사용자 수정 섹션 (보존)

**에러 처리**:
- 기존 파일 미존재 → init 커맨드 실행 제안
- 마커 불일치 → 경고 메시지, 수동 검토 요청

---

### 4. doctor 커맨드

**파일**: `internal/cli/doctor.go`

**목적**: 설치된 하네스의 유효성 검증

**실행 흐름**:
```
doctor
  ├── 설정 로드
  │   └── LoadConfig()
  ├── 각 플랫폼별 검증 실행
  │   ├── adapter.Validate()
  │   └── ValidationError 배열 반환
  ├── 검증 결과 분류
  │   ├── 오류 (error)
  │   └── 경고 (warning)
  ├── 리포트 생성
  │   ├── 플랫폼별 상태
  │   ├── 발견된 문제
  │   └── 권장사항
  └── 결과 출력
```

**검증 항목**:
- 필수 파일 존재 여부
- 파일 내용 검증
- 설정 일관성 확인
- 권한 확인

**종료 코드**:
- 0: 모든 검증 통과
- 1: 오류 발견
- 2: 경고만 발견

---

### 5. platform 커맨드

**파일**: `internal/cli/platform.go`

**목적**: 설치된 코딩 CLI 목록 표시

**실행 흐름**:
```
platform
  ├── 모든 어댑터 목록 조회
  │   └── registry.List()
  ├── 각 어댑터별 감지 실행
  │   ├── adapter.Detect()
  │   └── 설치 여부 확인
  └── 결과 테이블 출력
      ├── 플랫폼 이름
      ├── 설치 상태
      ├── 버전
      └── CLI 바이너리
```

**출력 형식**:
```
Platform        Installed  Version   Binary
Claude Code     ✓          1.0.0     claude
Codex           ✗          -         -
Gemini CLI      ✓          2.1.0     gemini
OpenCode        ✗          -         -
Cursor          ✓          1.5.0     cursor
```

---

### 6. arch 커맨드

**파일**: `internal/cli/arch.go`

**목적**: 코드 아키텍처 분석 및 문서 생성

**실행 흐름**:
```
arch
  ├── 플래그 파싱
  │   └── --path (분석 대상 디렉토리)
  ├── 코드 구조 분석
  │   └── arch.Analyze()
  │       ├── 디렉토리 구조 맵핑
  │       ├── 파일 타입 분류
  │       └── 의존성 추출
  ├── 아키텍처 린팅
  │   ├── arch.Lint()
  │   └── 규칙 위반 감지
  ├── 문서 생성
  │   ├── arch.Generate()
  │   └── ARCHITECTURE.md 작성
  └── 결과 리포트 출력
```

**분석 대상**:
- 디렉토리 계층 구조
- 파일 조직
- 모듈 경계
- 의존성 관계

**생성 산출물**:
- ARCHITECTURE.md: 아키텍처 문서
- 다이어그램: 모듈 관계도, 의존성 그래프

---

### 7. spec 커맨드

**파일**: `internal/cli/spec.go`

**목적**: SPEC 문서 파싱 및 검증

**실행 흐름**:
```
spec
  ├── 플래그 파싱
  │   └── --file (SPEC 파일 경로)
  ├── SPEC 파일 파싱
  │   └── spec.Parse()
  │       ├── EARS 형식 파싱
  │       └── 구조화된 데이터 반환
  ├── 템플릿 렌더링
  │   └── spec.Template()
  │       └── 문서 생성
  ├── 검증 수행
  │   └── spec.Validate()
  │       ├── 필수 필드 확인
  │       ├── 형식 검증
  │       └── 일관성 검사
  └── 결과 리포트 출력
```

**SPEC 형식** (EARS):
- Ubiquitous: 항상 활성화되는 요구사항
- Event-driven: 특정 이벤트 발생 시
- State-driven: 특정 상태에서 활동
- Unwanted: 금지되는 동작
- Optional: 선택적 요구사항

---

### 8. lore 커맨드

**파일**: `internal/cli/lore.go`

**목적**: 의사결정 기록 추적 및 쿼리

**실행 흐름**:
```
lore
  ├── 플래그 파싱
  │   ├── --query (검색 키워드)
  │   └── --format (출력 형식: json, text)
  ├── Git 커밋 로그 쿼리
  │   └── lore.Query()
  │       ├── 9-트레일러 프로토콜 검색
  │       └── 매칭 커밋 반환
  ├── 의사결정 데이터 추출
  │   ├── Decision-Reason
  │   ├── Decision-Consequence
  │   ├── Decision-Alternative
  │   └── 기타 트레일러
  └── 결과 출력
      └── 의사결정 히스토리
```

**트레일러 종류** (9-trailer protocol):
- Decision-Reason: 의사결정 이유
- Decision-Consequence: 결과 및 영향
- Decision-Alternative: 검토된 대안
- Decision-Status: 현재 상태
- Decision-Owner: 의사결정자
- Decision-Date: 결정 날짜
- Decision-Context: 배경 정보
- Decision-Review: 재검토 요청
- Decision-Deprecation: 폐기 예정 여부

**출력 형식**:
- JSON: 구조화된 데이터
- Text: 읽기 좋은 형식

---

### 9. lsp 커맨드

**파일**: `internal/cli/lsp.go`

**목적**: Language Server Protocol 통합 및 실행

**실행 흐름**:
```
lsp
  ├── LSP 서버 초기화
  │   └── lsp.NewServer()
  ├── 표준 입력 수신
  │   └── JSON-RPC 메시지 처리
  ├── 코드 분석 요청 처리
  │   ├── textDocument/hover
  │   ├── textDocument/completion
  │   ├── textDocument/definition
  │   └── 기타 LSP 요청
  ├── 응답 생성
  │   └── 분석 결과 포함
  └── 표준 출력으로 응답 전송
      └── JSON-RPC 메시지
```

**LSP 기능**:
- Hover: 마우스 호버 정보
- Completion: 자동완성
- Definition: 정의 위치 찾기
- References: 참조 찾기
- Diagnostics: 오류 진단

---

### 10. search 커맨드

**파일**: `internal/cli/search.go`

**목적**: 외부 지식 소스 검색

**실행 흐름**:
```
search
  ├── 플래그 파싱
  │   ├── --query (검색어)
  │   ├── --source (검색 소스)
  │   └── --limit (결과 개수)
  ├── 검색 소스 선택
  │   ├── Context7 문서 검색
  │   ├── Exa API 검색
  │   └── 로컬 해시 검색
  ├── 검색 실행
  │   └── search.Query()
  │       └── 매칭 결과 반환
  └── 결과 출력
      ├── 제목
      ├── 요약
      └── 링크
```

**검색 소스**:
- Context7: 공식 라이브러리 문서
- Exa: 웹 검색 API
- 로컬: 로컬 인덱스

---

### 11. skill 커맨드

**파일**: `internal/cli/skill.go`

**목적**: 스킬 파일 관리 및 생성

**실행 흐름**:
```
skill
  ├── 서브커맨드 파싱
  │   ├── create: 새 스킬 생성
  │   ├── list: 스킬 목록 표시
  │   ├── edit: 스킬 편집
  │   └── validate: 스킬 검증
  ├── 해당 작업 실행
  │   └── skill 관련 함수 호출
  └── 결과 출력
```

**스킬 작업**:
- create: 새로운 스킬 SKILL.md 생성
- list: 등록된 스킬 목록 표시
- edit: 기존 스킬 편집
- validate: 스킬 문법 검증

---

### 12. hash 커맨드

**파일**: `internal/cli/hash.go`

**목적**: 파일 또는 콘텐츠의 해시값 계산

**실행 흐름**:
```
hash
  ├── 플래그 파싱
  │   ├── --file (파일 경로)
  │   ├── --text (텍스트 입력)
  │   └── --algorithm (해시 알고리즘)
  ├── 해시 계산
  │   ├── SHA256 (기본값)
  │   ├── MD5
  │   └── SHA1
  └── 결과 출력
      ├── 해시값
      └── 파일명/입력 정보
```

**해시 알고리즘**:
- SHA256: 보안 해시 (권장)
- MD5: 빠른 해시 (호환성)
- SHA1: 중간 강도

---

### 추가: docs 커맨드

**파일**: `internal/cli/docs.go` (암시적)

**목적**: 문서 생성 및 Nextra 통합

**실행 흐름**:
```
docs
  ├── 플래그 파싱
  ├── 문서 소스 분석
  ├── Nextra 구조 생성
  ├── MDX 파일 생성
  ├── 검색 인덱스 생성
  └── 배포 준비
```

---

## 커맨드 실행 흐름 다이어그램

```
┌─────────────────┐
│ 사용자 입력     │
│ auto init       │
└────────┬────────┘
         │
    ┌────▼─────────────────────────┐
    │ cmd/auto/main()               │
    │ └─> cli.Execute()             │
    └────┬─────────────────────────┘
         │
    ┌────▼──────────────────┐
    │ NewRootCmd()           │
    │ (커맨드 생성)          │
    └────┬──────────────────┘
         │
    ┌────▼──────────────────┐
    │ 플래그 파싱            │
    │ (--verbose 등)        │
    └────┬──────────────────┘
         │
    ┌────▼──────────────────────────┐
    │ newInitCmd() 실행              │
    │ (init 커맨드 핸들러)           │
    └────┬──────────────────────────┘
         │
    ┌────▼──────────────────────────┐
    │ init.Run() 함수                │
    │ ├─ 플랫폼 감지                 │
    │ ├─ 설정 로드                   │
    │ ├─ 파일 생성                   │
    │ └─ 파일 쓰기                   │
    └────┬──────────────────────────┘
         │
    ┌────▼──────────────────┐
    │ 성공 또는 에러        │
    │ 메시지 출력           │
    └───────────────────────┘
```

---

## 이벤트 핸들러 패턴

### 플랫폼 감지 이벤트

```
DetectAll() 호출
  ├─ Claude Code 감지 (Detect() 메서드)
  │   └─ 실패: 다음 플랫폼으로 진행
  ├─ Codex 감지
  ├─ Gemini CLI 감지
  ├─ OpenCode 감지
  └─ Cursor 감지
```

### 파일 쓰기 이벤트

```
WriteFiles()
  ├─ 각 파일에 대해
  │   ├─ 목표 경로 생성
  │   ├─ 콘텐츠 쓰기
  │   ├─ 권한 설정
  │   └─ 검증
  └─ 전체 완료
```

### 에러 발생 이벤트

```
에러 발생
  ├─ 에러 타입 분류
  │   ├─ Fatal: 프로그램 종료
  │   ├─ Error: 실패 메시지 출력
  │   └─ Warning: 경고 메시지 출력
  └─ 종료 코드 설정
```

---

## 플래그 및 옵션 종합

### 전역 플래그 (모든 커맨드)

| 플래그 | 약자 | 기본값 | 설명 |
|--------|------|--------|------|
| `--verbose` | `-v` | false | 상세 출력 활성화 |
| `--config` | - | `./autopus.yaml` | 설정 파일 경로 |

### 커맨드별 플래그

| 커맨드 | 플래그 | 기본값 | 설명 |
|--------|--------|--------|------|
| init | `--mode` | full | full 또는 lite |
| init | `--platform` | (감지) | 특정 플랫폼 지정 |
| update | `--force` | false | 모든 파일 덮어쓰기 |
| doctor | `--format` | text | text 또는 json |
| arch | `--path` | . | 분석 대상 경로 |
| spec | `--file` | (필수) | SPEC 파일 경로 |
| lore | `--query` | (필수) | 검색 키워드 |
| lore | `--format` | text | text 또는 json |
| search | `--query` | (필수) | 검색어 |
| search | `--source` | context7 | 검색 소스 |
| hash | `--file` | - | 파일 경로 |
| hash | `--text` | - | 텍스트 입력 |
