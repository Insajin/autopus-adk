---
name: auto-test
description: E2E 시나리오 실행 — scenarios.md 기반 검증을 수행합니다
compatibility: omp
---

# auto-test

## OMP Invocation

- `/auto test ...`
- `/auto-test ...`
- Load detail skill `auto-test` for either entrypoint.

## Context Profile: test

- Required: core,test
- Optional: signature,learning
- Excluded: canary

## 실행 계약

E2E 시나리오 실행 — scenarios.md 기반 검증을 수행합니다

1. 대상 디렉터리와 전달된 인자를 확인합니다.
2. scenarios.md를 로드하고 선언된 E2E 시나리오를 실행합니다.
3. 시나리오별 PASS / WARN / FAIL과 evidence를 보고합니다.
