---
description: "SPEC 작성 — 코드베이스 분석 후 EARS 요구사항, 구현 계획, 인수 기준을 생성합니다"
---

# auto-plan — SPEC 작성

## Autopus Branding

When handling this workflow, start the response with the canonical banner from `templates/shared/branding-formats.md.tmpl`:

```text
🐙 Autopus ─────────────────────────
```

End the completed response with `🐙`.


**프로젝트**: autopus-adk | **모드**: full

## 사용법

사용자가 기능을 설명하면 코드베이스를 분석하고 SPEC 문서를 생성하세요.

공통 플래그 의미는 `@auto plan ...` 라우터를 우선합니다:
- `--skip-prd`
- `--prd-mode <mode>`
- `--from-idea <BS-ID>`
- `--strategy <value>` with `--multi`
- `--target <module>`
- `--auto`

## Codex Notes

- 기본 운영 원칙은 `spawn_agent(...)` 기반 subagent-first 입니다.
- 메인 세션은 최종 SPEC 구조와 저장을 담당합니다.
- 코드 탐색, 레퍼런스 수집, 초안 작성은 `explorer` / `planner` / `spec-writer` 계열 서브에이전트에 우선 분담합니다.
- `--skip-prd`가 없으면 PRD를 먼저 생성하고, 이때 얻은 SPEC-ID를 `spec-writer`가 재사용해야 합니다.
- `--multi` 또는 `review_gate.enabled` 가 활성화되면 `auto spec review {SPEC-ID}` 를 실행해 `draft/approved` 상태를 결정해야 합니다.

## 워크플로우

1. 관련 코드 영역을 탐색하고 기존 패턴을 파악합니다
2. `auto lore context <path>`로 기존 의사결정 이력을 확인합니다
3. `auto arch enforce`로 아키텍처 위반을 검증합니다
4. EARS 형식으로 요구사항을 작성합니다
5. 구현 계획(plan.md)을 생성합니다
6. 인수 기준(acceptance.md)을 생성합니다
7. 리서치 결과(research.md)를 저장합니다

## SPEC ID 형식

`SPEC-{DOMAIN}-{NUMBER}`

## EARS 요구사항 형식

지원 타입: ubiquitous, event-driven, unwanted, optional, complex

- `The system shall [action]` — Ubiquitous
- `WHEN [trigger] THEN the system shall [action]` — Event-driven
- `WHILE [state] the system shall [action]` — State-driven
- `IF [condition] THEN the system shall [response]` — Unwanted
- `WHERE [feature] is enabled the system shall [action]` — Optional

## 출력

`.autopus/specs/SPEC-{DOMAIN}-{NUMBER}/` 디렉터리에 저장:
- `prd.md` — PRD 문서 (`--skip-prd` 시 생략)
- `spec.md` — 메인 SPEC
- `plan.md` — 구현 계획
- `acceptance.md` — 인수 기준
- `research.md` — 리서치 결과

## 규칙

- 파일 크기 제한: 소스 파일 300줄 이하
- 테스트 커버리지 목표: 85%+
