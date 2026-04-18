---
description: "풀 사이클 개발 — plan → go → sync를 순차 실행합니다"
---

# auto-dev — Full Development Cycle

## Autopus Branding

When handling this workflow, start the response with the canonical banner from `templates/shared/branding-formats.md.tmpl`:

```text
🐙 Autopus ─────────────────────────
```

End the completed response with `🐙`.


## 설명

풀 사이클 개발 — plan → go → sync를 순차 실행합니다

## 실행 순서

1. 기능 설명을 기준으로 `auto-plan`을 먼저 수행합니다.
2. 생성된 SPEC-ID를 기준으로 `auto-go`를 진행합니다.
3. 구현이 끝나면 `auto-sync`를 수행합니다.
4. `--auto`, `--loop`, `--team`, `--multi`, `--quality` 플래그는 가능한 한 하위 단계로 전달합니다.
