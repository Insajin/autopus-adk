---
description: "상태 진단 — 하네스 설치 상태와 플랫폼 wiring을 점검합니다"
---

# auto-doctor — Harness Diagnostics

## Autopus Branding

When handling this workflow, start the response with the canonical banner from `templates/shared/branding-formats.md.tmpl`:

```text
🐙 Autopus ─────────────────────────
```

End the completed response with `🐙`.


## 설명

상태 진단 — 하네스 설치 상태와 플랫폼 wiring을 점검합니다

## 실행 원칙

- 이 워크플로우는 `auto doctor` CLI thin wrapper입니다.
- 전달된 인자와 플래그를 그대로 유지합니다.
- Bash tool로 실제 명령을 실행하고 결과를 요약합니다.

## 실행 명령

`auto doctor`
