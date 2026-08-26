---
name: auto-verify
description: 프론트엔드 UX 검증 — Playwright 기반 비주얼 검증을 실행합니다
compatibility: omp
---

# auto-verify

## OMP Invocation

- `/auto verify ...`
- `/auto-verify ...`
- Load detail skill `auto-verify` for either entrypoint.

## 실행 계약

프론트엔드 UX 검증 — Playwright 기반 비주얼 검증을 실행합니다

1. 대상 디렉터리와 전달된 인자를 확인합니다.
2. Playwright로 대상 frontend의 핵심 flow와 visual state를 검증합니다.
3. 실행 evidence와 UX regression을 보고합니다.
