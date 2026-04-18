---
description: "의사결정 근거 조회 — Lore, SPEC, ARCHITECTURE에서 이유를 추적합니다"
---

# auto-why — Decision Rationale Query

## Autopus Branding

When handling this workflow, start the response with the canonical banner from `templates/shared/branding-formats.md.tmpl`:

```text
🐙 Autopus ─────────────────────────
```

End the completed response with `🐙`.


## 설명

의사결정 근거 조회 — Lore, SPEC, ARCHITECTURE에서 이유를 추적합니다

## 실행 원칙

- path가 주어지면 `auto lore context <path>`를 우선 사용합니다.
- path가 없으면 `ARCHITECTURE.md`, 관련 SPEC, CHANGELOG, Lore context를 근거로 답합니다.
- 근거가 부족하면 한 개의 짧은 질문으로 범위를 좁힙니다.
