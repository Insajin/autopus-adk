---
name: auto-doctor
description: 상태 진단 — 하네스 설치 상태와 플랫폼 wiring을 점검합니다
compatibility: omp
---

# auto-doctor

## OMP Invocation

- `/auto doctor ...`
- `/auto-doctor ...`
- Load detail skill `auto-doctor` for either entrypoint.

## 실행 계약

상태 진단 — 하네스 설치 상태와 플랫폼 wiring을 점검합니다

1. 대상 디렉터리와 전달된 인자를 확인합니다.
2. Bash tool로 `auto doctor`를 실행해 harness 설치와 platform wiring을 값으로 진단합니다.
3. finding별 expected/got과 repair action을 보고합니다.
