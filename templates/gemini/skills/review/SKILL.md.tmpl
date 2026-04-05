---
name: auto-review
---

# Code Review Skill

TRUST 5 기준으로 코드를 체계적으로 검토하는 스킬입니다.

## TRUST 5 리뷰 기준

### T — Tested (테스트됨)
- [ ] 85% 이상 테스트 커버리지
- [ ] 모든 엣지 케이스 테스트
- [ ] 레이스/동시성 테스트 (Go: `go test -race`, Python: `pytest-asyncio`, etc.)
- [ ] 특성 테스트 존재 (기존 코드 변경 시)
- [ ] 경계 통합 검증: 패키지/모듈 간 호출이 실제로 연결됨 (스텁 아님)
- [ ] CLI/API entry point가 실제 실행 가능 (smoke test)

### R — Readable (가독성)
- [ ] 함수/변수 명명이 명확한가?
- [ ] 함수가 단일 책임을 가지는가?
- [ ] 복잡한 로직에 주석이 있는가?
- [ ] 코드 길이가 적절한가? (함수 50줄 이하 권장)

### U — Unified (일관성)
- [ ] 프로젝트 코딩 스타일 준수
- [ ] 포매터 적용됨 (Go: `gofmt`, Python: `ruff format`, TS: `prettier`, Rust: `rustfmt`)
- [ ] 린터 경고 없음 (Go: `golangci-lint`, Python: `ruff`, TS: `eslint`, Rust: `clippy`)
- [ ] 에러 처리 패턴 일관성

### S — Secured (보안)
- [ ] SQL 인젝션 방지
- [ ] 입력 검증 존재
- [ ] 인증/인가 확인
- [ ] 민감 정보 하드코딩 없음
- [ ] OWASP Top 10 고려

### T — Trackable (추적 가능)
- [ ] 의미있는 로그 메시지
- [ ] 에러에 컨텍스트 포함
- [ ] 커밋 메시지가 명확한가?
- [ ] SPEC/이슈 번호 참조

#### @AX Compliance
- [ ] @AX:REASON present on all WARN and ANCHOR tags
- [ ] Per-file limits: ANCHOR ≤ 3, WARN ≤ 5
- [ ] Agent-generated tags include [AUTO] prefix
- [ ] Comment syntax matches file language (Go: `//`, Python: `#`)
- [ ] ANCHOR tags verified: fan_in ≥ 3 callers (grep heuristic)

### Structure Gate
- [ ] No source code file exceeds 300 lines (hard limit)
- [ ] Source files over 200 lines flagged for splitting
- [ ] Non-code files excluded: generated (*_generated.go, *.pb.go), docs (*.md), config (*.yaml, *.json)
- [ ] Complex changes delegated to subagents (3+ files)
- [ ] @AX tag compliance verified (see Trackable > @AX Compliance)

## 리뷰 출력 형식

```markdown
## 코드 리뷰 결과

### 요약
변경 사항: [간단한 설명]
리뷰 결과: ✅ 승인 / ⚠️ 수정 요청 / ❌ 거부

### TRUST 5 점수
- Tested: ✅ / ⚠️ / ❌
- Readable: ✅ / ⚠️ / ❌
- Unified: ✅ / ⚠️ / ❌
- Secured: ✅ / ⚠️ / ❌
- Trackable: ✅ / ⚠️ / ❌

### 구조 검사
- File Size: ✅ / ⚠️ / ❌
- Subagent Usage: ✅ / ⚠️ / N/A

### 필수 수정 사항
1. [파일:라인] 이유 및 수정 방법

### 제안 사항 (선택)
1. [제안 내용]
```

## 자동화 게이트

리뷰 전 반드시 통과해야 하는 자동화 검사 (스택별):

| Check | Go | Python | TypeScript | Rust |
|-------|-----|--------|------------|------|
| 테스트 | `go test -race ./...` | `pytest` | `vitest run` | `cargo test` |
| 린트 | `golangci-lint run && go vet ./...` | `ruff check .` | `eslint .` | `cargo clippy` |
| 스텁 검사 | `grep -rn 'TODO\|stub\|placeholder' {changed}` | 동일 | 동일 | `grep -rn 'todo!\|unimplemented!' {changed}` |

스택은 `.autopus/project/tech.md` 또는 프로젝트 루트의 매니페스트(`go.mod`, `package.json`, `pyproject.toml`, `Cargo.toml`)에서 자동 감지합니다.
