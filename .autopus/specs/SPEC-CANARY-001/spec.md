# SPEC-CANARY-001: Post-deploy Health Check (canary 서브커맨드)

**Status**: completed
**Created**: 2026-04-03
**Domain**: CANARY
**Target Version**: v0.15

## 목적

현재 autopus-adk 워크플로우는 `plan -> go -> sync`까지만 커버하며, 배포 후 검증이 빠져 있다. `/auto canary`는 배포 이후 빌드 성공 확인, E2E 시나리오 실행, 브라우저 건강 검진을 자동화하여 지속적 배포 신뢰도를 높인다.

gstack의 `/canary` (스크린샷 diff + 콘솔 에러 감지)에서 영감을 받되, autopus-adk의 기존 인프라(e2e 패키지, browse 패키지, scenarios.md)를 활용하는 접근.

## 요구사항

### R1: 빌드 검증
WHEN the user runs `/auto canary`, THE SYSTEM SHALL execute the project build command (from `scenarios.md` Build field or `tech.md`) and report build success or failure as the first step.

### R2: E2E 시나리오 실행
WHEN the build succeeds, THE SYSTEM SHALL run all active E2E scenarios from `.autopus/project/scenarios.md` using the existing `pkg/e2e` runner and collect pass/fail results per scenario.

### R3: 브라우저 건강 검진 (Frontend 프로젝트)
WHEN the project has a `--url` flag or is detected as a frontend project (via `tech.md`), THE SYSTEM SHALL perform browser health checks including:
- Accessibility tree snapshot of key pages
- Console error collection
- Snapshot diff against the previous canary run (stored in `.autopus/canary/`)

### R4: PASS/WARN/FAIL 판정
WHEN all checks complete, THE SYSTEM SHALL produce a summary with one of:
- **PASS**: build OK + all E2E pass + no console errors
- **WARN**: build OK + some E2E failures or non-critical console warnings
- **FAIL**: build failure or critical E2E failures or critical console errors

### R5: URL 대상 지정
WHEN the user provides `--url <url>`, THE SYSTEM SHALL run browser health checks against the specified URL instead of auto-detecting.

### R6: 주기적 모니터링
WHEN the user provides `--watch <interval>`, THE SYSTEM SHALL repeat the health check at the given interval (default 5 minutes, maximum 30 minutes) until interrupted or a FAIL occurs.

### R7: 커밋 비교
WHEN the user provides `--compare <commit>`, THE SYSTEM SHALL compare current canary results against the stored results from the specified commit (if available in `.autopus/canary/`).

### R8: 스킬 라우터 등록
WHEN the `/auto` router processes the `canary` subcommand, THE SYSTEM SHALL route to the canary skill handler defined in the auto-router template.

### R9: sync 이후 안내
WHEN `/auto sync` completes successfully, THE SYSTEM SHALL suggest `/auto canary` as the next step in the workflow.

### R10: canary.md 자동 생성 (setup)
WHEN `/auto setup` runs, THE SYSTEM SHALL analyze the project to auto-generate `.autopus/project/canary.md` containing: build command, health check endpoints, browser targets, deploy platform. Detection sources: Dockerfile, railway.json/vercel.json/fly.toml, HTTP handler patterns, k8s manifests, CI/CD workflows, package.json scripts.

### R11: canary.md 자동 갱신 (sync)
WHEN `/auto sync` runs, THE SYSTEM SHALL update `.autopus/project/canary.md` to reflect new handlers, pages, deploy config changes detected since the last sync.

## 생성 파일 상세

| 파일/경로 | 역할 |
|-----------|------|
| `templates/claude/commands/auto-canary.md.tmpl` | canary 서브커맨드 스킬 템플릿 |
| `templates/codex/prompts/auto-canary.md.tmpl` | codex 플랫폼 canary 프롬프트 |
| `templates/gemini/commands/auto-canary.md.tmpl` | gemini 플랫폼 canary 커맨드 |
| `auto-router.md.tmpl` (수정) | canary 서브커맨드 라우팅 추가 |
| `.autopus/canary/` (런타임 생성) | 스냅샷 및 히스토리 저장 디렉토리 |

## 데이터 저장

canary 실행 결과는 `.autopus/canary/` 디렉토리에 저장:
- `.autopus/canary/latest.json` — 최근 canary 결과
- `.autopus/canary/{commit-hash}.json` — 커밋별 스냅샷 (비교용)
