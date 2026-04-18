---
description: "의사결정 근거 조회 — Lore, SPEC, ARCHITECTURE에서 이유를 추적합니다"
agent: build
---

`$ARGUMENTS`

Treat the text above as the full argument payload for `/auto-why`.
Reinterpret it as `/auto why ...` payload로 다시 해석하고, `skill` 도구로 `auto`를 로드한 뒤 canonical router 규칙을 따르세요.
Preserve `--model <provider/model>` and `--variant <value>` when present.
Do not restate or expand the arguments unless needed for execution.
