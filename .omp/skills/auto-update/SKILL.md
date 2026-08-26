---
name: auto-update
description: 하네스 업데이트 — 현재 repo 또는 메타 workspace를 감지해 하네스 surface를 갱신합니다
compatibility: omp
---

# auto-update

## OMP Invocation

- `/auto update ...`
- `/auto-update ...`
- Load detail skill `auto-update` for either entrypoint.

## 실행 계약

하네스 업데이트 — 현재 repo 또는 메타 workspace를 감지해 하네스 surface를 갱신합니다

1. 대상 디렉터리와 전달된 인자를 확인합니다.
2. Bash tool로 `auto update`를 실행해 현재 repo 또는 meta workspace의 harness surface를 갱신합니다.
3. 변경된 generated surface와 source-of-truth 경계를 보고합니다.
