---
description: "버그 수정 — 재현과 최소 수정 중심으로 문제를 해결합니다"
agent: build
---

`$ARGUMENTS`

Treat the text above as the full argument payload for `/auto-fix`.
Reinterpret it as `/auto fix ...` payload로 다시 해석하고, `skill` 도구로 `auto`를 로드한 뒤 canonical router 규칙을 따르세요.
Preserve `--model <provider/model>` and `--variant <value>` when present.
Do not restate or expand the arguments unless needed for execution.
