# auto-context-search

# Context Search Skill

이전 Claude Code 세션의 컨텍스트를 검색하여 작업을 연속적으로 진행하는 스킬입니다.

## 검색 시점

다음 상황에서 컨텍스트를 검색합니다:
- 사용자가 이전 작업을 언급할 때
- SPEC-ID가 현재 세션에 없을 때
- "지난번에", "이어서", "계속" 등의 표현 감지

## 검색 절차

### 1단계: 현재 세션 확인
먼저 현재 세션에 관련 컨텍스트가 있는지 확인합니다.
있으면 검색 불필요 → 스킵.

### 2단계: .auto-continue.md 확인
```bash
cat .auto-continue.md 2>/dev/null
```

### 3단계: 세션 인덱스 검색
```bash
# Claude Code 세션 파일 검색
grep -r "SPEC-XXX" ~/.codex/projects/ --include="*.md" -l
grep -r "관련 키워드" ~/.codex/projects/ -l | head -10
```

### 4단계: 컨텍스트 요약 주입
발견된 컨텍스트를 현재 세션에 요약하여 주입합니다.

**제한:**
- 최대 5,000 토큰 주입
- 현재 사용량 150,000 토큰 초과 시 스킵
- 중복 주입 방지

## .auto-continue.md 형식

```yaml
workflow_phase: implementation
completed_tasks:
  - "auth/login.go 구현"
  - "JWT 토큰 생성"
pending_decisions:
  - "리프레시 토큰 전략 결정 필요"
context_summary: |
  인증 시스템 구현 중. JWT HS256 방식 채택.
  login/logout 엔드포인트 완료.
  다음: 토큰 검증 미들웨어 구현 필요.
```

## 세션 재개 패턴

```markdown
## 이전 세션 요약

**진행 단계**: [단계명]
**완료된 작업**: [목록]
**미완료 작업**: [목록]
**중요 결정사항**: [결정들]
**다음 단계**: [권장 다음 작업]

---
현재 세션을 이어서 진행합니다.
```

## 토큰 예산 관리

```
컨텍스트 주입 우선순위:
1. 미완료 작업 목록 (필수)
2. 미결 의사결정 (필수)
3. 핵심 아키텍처 결정 (중요)
4. 상세 구현 내용 (선택)
```
