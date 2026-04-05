---
name: auto-hash-anchored-edit
---

# Hash-Anchored Edit Skill

파일 수정 전 SHA256 해시로 무결성을 검증하여 충돌 없이 안전하게 수정하는 스킬입니다.

## 핵심 개념

### 왜 해시 앵커가 필요한가?
병렬 에이전트 환경에서 동일 파일에 여러 에이전트가 접근할 경우:
- 에이전트 A가 읽은 후 에이전트 B가 수정
- 에이전트 A가 구버전 기반으로 수정 → 충돌 또는 유실

해시 앵커는 파일 상태를 고정하여 이를 방지합니다.

## 해시 앵커 적용 절차

### 1단계: 현재 상태 기록
```bash
# 수정 전 해시 계산
sha256sum target_file.go > target_file.go.hash
```

### 2단계: 수정 전 검증
```bash
# 수정 시도 전 해시 비교
sha256sum -c target_file.go.hash
# 성공: target_file.go: OK
# 실패: 파일이 변경되었음 — 재읽기 필요
```

### 3단계: 수정 실행
검증 성공 시에만 수정을 진행합니다.

### 4단계: 수정 후 해시 갱신
```bash
sha256sum target_file.go > target_file.go.hash
```

## Edit 도구와의 통합

Edit 도구 사용 시 내장 충돌 감지:
```
old_string: <정확한 현재 내용>
new_string: <새로운 내용>
```

`old_string`이 현재 파일에 없으면 에러 → 재읽기 후 재시도

## 멀티 에이전트 파일 락

팀 모드에서 파일 소유권 분리:
```yaml
# 에이전트별 파일 소유권
file_ownership:
  backend-dev: ["**/*.go", "!**/*_test.go"]
  tester: ["**/*_test.go"]
  frontend-dev: ["**/*.ts", "**/*.tsx"]
```

소유권 외 파일 수정은 금지 → 자동 충돌 방지

## 주의 사항

- `.hash` 파일은 임시 파일 — 커밋하지 않음
- 해시 불일치 시 브루트포스 재시도 금지 (최대 3회)
- 3회 실패 시 팀 리드에게 보고
