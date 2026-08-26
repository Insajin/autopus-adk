---
name: auto-status
description: SPEC 대시보드 — 현재 프로젝트와 서브모듈의 SPEC 상태를 표시합니다
compatibility: omp
---

# auto-status

## OMP Invocation

- `/auto status ...`
- `/auto-status ...`
- Load detail skill `auto-status` for either entrypoint.

## 실행 계약

SPEC 대시보드 — 현재 프로젝트와 서브모듈의 SPEC 상태를 표시합니다

1. 대상 디렉터리와 전달된 인자를 확인합니다.
2. Bash tool로 `auto status`를 실행해 SPEC lifecycle과 module 상태를 수집한 뒤 현재 workspace의 `.autopus/pipeline-state/<SPEC-ID>.execution-owner.json` receipt와 현재 OMP 세션의 native `hub` jobs 상태를 읽습니다. 외부 CLI가 OMP user-session roots를 검사할 수 있다고 가정하지 않습니다.
3. receipt의 owner/source/verification과 OMP-native `hub` 상태를 결합해 단일 DAG owner 위반 여부, SPEC별 status, 다음 실행 가능한 명령을 보고합니다.
