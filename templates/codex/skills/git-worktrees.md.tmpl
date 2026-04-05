# auto-git-worktrees

# Git Worktrees Skill

Git Worktree를 사용하여 여러 브랜치를 동시에 작업하는 스킬입니다.

## Git Worktree 기본

```bash
# 새 worktree 생성 (브랜치 자동 생성)
git worktree add ../project-feature feature/new-feature

# 기존 브랜치로 worktree 생성
git worktree add ../project-hotfix hotfix/critical-fix

# worktree 목록 확인
git worktree list

# worktree 제거
git worktree remove ../project-feature
git worktree prune  # 정리
```

## 에이전트 격리 패턴

코딩 에이전트의 병렬 작업을 격리할 때:

```bash
# 에이전트별 독립 worktree 생성
git worktree add /tmp/agent-backend-work agent/backend-$(date +%s)
git worktree add /tmp/agent-frontend-work agent/frontend-$(date +%s)
git worktree add /tmp/agent-tester-work agent/tester-$(date +%s)
```

각 에이전트는 자신의 worktree에서:
- 독립적으로 파일 수정
- 파일 충돌 없이 병렬 실행
- 완료 후 메인 브랜치에 머지

## Worktree 워크플로우

### 기능 개발 + 핫픽스 동시 진행
```bash
# 현재 기능 개발 중
git worktree add ../hotfix hotfix/security-patch

# 핫픽스 worktree에서 작업
cd ../hotfix
# 수정...
git commit -m "fix(security): XSS 취약점 수정"
git push origin hotfix/security-patch

# 원래 작업으로 복귀
cd ../main-project
```

### 코드 리뷰 동시 진행
```bash
# PR 브랜치를 worktree로 체크아웃
git worktree add ../review-pr-123 pr-review/123
git fetch origin pull/123/head:pr-review/123
```

## 주의 사항

- 같은 브랜치를 두 개의 worktree에서 사용 불가
- `.git/` 디렉토리는 공유됨 (커밋, 스테이지 독립)
- Worktree 경로에 공백 사용 피할 것
- 에이전트 작업 완료 후 반드시 제거

## 정리 스크립트

```bash
#!/bin/sh
# 오래된 에이전트 worktree 정리
git worktree list --porcelain | \
  grep "^worktree " | \
  grep "/tmp/agent-" | \
  awk '{print $2}' | \
  xargs -I{} git worktree remove {}
git worktree prune
```
