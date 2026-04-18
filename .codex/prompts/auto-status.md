---
description: "SPEC 대시보드 — 현재 프로젝트와 서브모듈의 SPEC 상태를 표시합니다"
---

# auto-status — SPEC Dashboard

## Autopus Branding

When handling this workflow, start the response with the canonical banner from `templates/shared/branding-formats.md.tmpl`:

```text
🐙 Autopus ─────────────────────────
```

End the completed response with `🐙`.


## 설명

SPEC 대시보드 — 현재 프로젝트와 서브모듈의 SPEC 상태를 표시합니다

## 실행 원칙

- 이 워크플로우는 `auto status` CLI thin wrapper입니다.
- 전달된 인자와 플래그를 그대로 유지합니다.
- Bash tool로 실제 명령을 실행하고 결과를 요약합니다.

## 실행 명령

`auto status`
