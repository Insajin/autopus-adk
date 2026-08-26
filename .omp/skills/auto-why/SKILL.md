---
name: auto-why
description: 의사결정 근거 조회 — Lore, SPEC, ARCHITECTURE에서 이유를 추적합니다
compatibility: omp
---

# auto-why

## OMP Invocation

- `/auto why ...`
- `/auto-why ...`
- Load detail skill `auto-why` for either entrypoint.

## 실행 계약

의사결정 근거 조회 — Lore, SPEC, ARCHITECTURE에서 이유를 추적합니다

1. 대상 디렉터리와 전달된 인자를 확인합니다.
2. Workspace tools로 Lore, SPEC, ARCHITECTURE를 검색해 요청된 결정의 근거를 추적합니다.
3. 근거 파일과 결정 이유를 인용합니다.
