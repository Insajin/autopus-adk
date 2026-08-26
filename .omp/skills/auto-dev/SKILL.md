---
name: auto-dev
description: 풀 사이클 개발 — plan → go → sync를 순차 실행합니다
compatibility: omp
---

# auto-dev

## OMP Invocation

- `/auto dev ...`
- `/auto-dev ...`
- Load detail skill `auto-dev` for either entrypoint.

## 실행 계약

풀 사이클 개발 — plan → go → sync를 순차 실행합니다

1. 대상 디렉터리와 전달된 인자를 확인합니다.
2. `auto plan` → `auto go` → `auto sync`를 순차 실행합니다.
3. 첫 실패 단계에서 멈추고 재개 방법을 보고합니다.
