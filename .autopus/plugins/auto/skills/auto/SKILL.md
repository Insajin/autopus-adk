---
name: auto
description: >
  Autopus Codex router skill. Use when the user wants `@auto ...` or `$auto ...` workflows such as setup, status, plan, go, fix, review, sync, idea, map, why, verify, secure, test, dev, canary, and doctor.
---

# auto — Autopus Command Router

## Autopus Branding

When handling `@auto` router responses, start the response with the canonical banner from `templates/shared/branding-formats.md.tmpl`:

```text
🐙 Autopus ─────────────────────────
```

End the completed response with `🐙`.


## Router Execution Contract

- Treat this file as a thin entrypoint only.
- After resolving the subcommand, immediately load the matching detailed workflow surface (`auto-setup`, `auto-status`, `auto-plan`, `auto-go`, `auto-fix`, `auto-review`, `auto-sync`, `auto-idea`, `auto-map`, `auto-why`, `auto-verify`, `auto-secure`, `auto-test`, `auto-dev`, `auto-canary`, `auto-doctor`) before answering or acting.
- Do not stay at the router layer when a detailed workflow exists for the request.
- Always load the project context documents before routing or executing the workflow.

## Context Load

Before processing any `@auto` subcommand, read these files if they exist:

1. `ARCHITECTURE.md`
2. `.autopus/project/product.md`
3. `.autopus/project/structure.md`
4. `.autopus/project/tech.md`
5. `.autopus/project/workspace.md`
6. `.autopus/project/scenarios.md`
7. `.autopus/project/canary.md`
8. `.autopus/context/signatures.md`
9. `.autopus/learnings/pipeline.jsonl`

- If none of these files exist, explicitly note that project context is missing and recommend `@auto setup`.
- Do not skip this load step just because the subcommand looks obvious.

## SPEC Path Resolution

When any workflow receives a SPEC-ID, resolve the actual file path before opening files, spawning workers, or running build/test commands:

1. Check `.autopus/specs/{SPEC-ID}/spec.md` (top-level, cross-module or legacy SPECs).
2. Recursively search `**/.autopus/specs/{SPEC-ID}/spec.md`, skipping `.git`, `node_modules`, `vendor`, `.cache`, and `dist`.

From the resolved path, extract:

- `SPEC_PATH`: full path to `spec.md`
- `SPEC_DIR`: parent SPEC directory
- `TARGET_MODULE`: submodule path, or `.` for top-level SPECs
- `WORKING_DIR`: the directory where build/test commands run (`TARGET_MODULE` or `.`)

Error handling:

- 0 matches: report the SPEC is missing and list available SPEC IDs.
- 2+ matches: report the duplicate paths and stop for clarification.
- All detailed workflows must use the resolved values instead of assuming `.autopus/specs/{SPEC-ID}` is rooted at the current directory.



## Codex Invocation

Use this skill through either of these surfaces:

- `@auto <subcommand> ...` — preferred when the local Autopus plugin is installed from `/plugins`
- `$auto <subcommand> ...` — direct repository skill invocation

Direct skill loads should treat the user's latest `auto ...` request as the arguments.
This skill is a thin router. After resolving the subcommand, load the matching detailed skill (`auto-setup`, `auto-status`, `auto-plan`, `auto-go`, `auto-fix`, `auto-review`, `auto-sync`, `auto-idea`, `auto-map`, `auto-why`, `auto-verify`, `auto-secure`, `auto-test`, `auto-dev`, `auto-canary`, `auto-doctor`) before executing the workflow.


**프로젝트**: autopus-adk | **모드**: full

사용자가 `@auto <subcommand> ...` 형태로 호출했다고 가정하고 첫 토큰을 해석합니다.

이 문서는 Codex용 canonical router surface 입니다. shared skill과 parity는 유지하되, 의미 해석은 Codex 규약(`spawn_agent(...)`, `@auto`, `--auto`, `--team`)을 기준으로 하세요. 이 프롬프트는 얇은 진입점이며, 플래그 의미(`--auto`, `--loop`, `--multi`, `--quality`, `--team`, `--solo`)를 축약해서 덮어쓰면 안 됩니다.

지원 서브커맨드:
- `setup`: 프로젝트 컨텍스트 생성 — 코드베이스를 분석하고 ARCHITECTURE.md 및 .autopus/project 문서를 생성합니다
- `status`: SPEC 대시보드 — 현재 프로젝트와 서브모듈의 SPEC 상태를 표시합니다
- `plan`: SPEC 작성 — 코드베이스 분석 후 EARS 요구사항, 구현 계획, 인수 기준을 생성합니다
- `go`: SPEC 구현 — SPEC 문서를 기반으로 코드를 구현합니다
- `fix`: 버그 수정 — 재현과 최소 수정 중심으로 문제를 해결합니다
- `review`: 코드 리뷰 — TRUST 5 기준으로 변경된 코드를 리뷰합니다
- `sync`: 문서 동기화 — 구현 이후 SPEC, CHANGELOG, 문서를 반영합니다
- `idea`: 아이디어 브레인스토밍 — 멀티 프로바이더 토론과 ICE 평가로 아이디어를 정리합니다
- `map`: 코드베이스 분석 — 구조, 엔트리포인트, 의존성을 빠르게 요약합니다
- `why`: 의사결정 근거 조회 — Lore, SPEC, ARCHITECTURE에서 이유를 추적합니다
- `verify`: 프론트엔드 UX 검증 — Playwright 기반 비주얼 검증을 실행합니다
- `secure`: 보안 감사 — OWASP Top 10 관점에서 변경 범위를 점검합니다
- `test`: E2E 시나리오 실행 — scenarios.md 기반 검증을 수행합니다
- `dev`: 풀 사이클 개발 — plan → go → sync를 순차 실행합니다
- `canary`: 배포 검증 — build, E2E, 브라우저 건강 검진을 실행합니다
- `doctor`: 상태 진단 — 하네스 설치 상태와 플랫폼 wiring을 점검합니다

규칙:
- 첫 토큰이 없는 경우, 사용자의 의도를 위 16개 중 하나로 분류해서 진행합니다.
- 가능하면 같은 이름의 상세 스킬/프롬프트(`auto-setup`, `auto-status`, `auto-plan`, `auto-go`, `auto-fix`, `auto-review`, `auto-sync`, `auto-idea`, `auto-map`, `auto-why`, `auto-verify`, `auto-secure`, `auto-test`, `auto-dev`, `auto-canary`, `auto-doctor`) 의미를 따릅니다.
- 지원하지 않는 서브커맨드면 지원 목록을 짧게 안내하고 가장 가까운 워크플로우를 제안합니다.
- Codex 하네스 기본값은 `spawn_agent(...)` 기반 subagent-first 입니다.
- Codex에서 `--auto`가 있으면, 기본 subagent pipeline 진행에 대한 명시적 승인으로 해석합니다.
- 단, `--auto`가 없고 현재 Codex 런타임 정책이 암묵적 `spawn_agent(...)` 호출을 허용하지 않으면 조용히 단일 세션으로 폴백하지 말고, 하네스 기본값과 제약을 명시적으로 설명한 뒤 사용자에게 서브에이전트 진행 여부 또는 `--solo` 선택을 받으세요.
