---
description: "E2E 시나리오 실행 — scenarios.md 기반 검증을 수행합니다"
---

# auto-test — E2E Scenario Runner

## Autopus Branding

When handling this workflow, start the response with the canonical banner from `templates/shared/branding-formats.md.tmpl`:

```text
🐙 Autopus ─────────────────────────
```

End the completed response with `🐙`.


## 설명

E2E 시나리오 실행 — scenarios.md 기반 검증을 수행합니다

## 실행 원칙

- 이 워크플로우는 `auto test run` CLI thin wrapper입니다.
- 전달된 인자와 플래그를 그대로 유지합니다.
- Bash tool로 실제 명령을 실행하고 결과를 요약합니다.

## 실행 명령

`auto test run`
