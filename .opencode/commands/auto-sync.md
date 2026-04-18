---
description: "문서 동기화 — 구현 이후 SPEC, CHANGELOG, 문서를 반영합니다"
agent: build
---

`$ARGUMENTS`

Treat the text above as the full argument payload for `/auto-sync`.
Reinterpret it as `/auto sync ...` payload로 다시 해석하고, `skill` 도구로 `auto`를 로드한 뒤 canonical router 규칙을 따르세요.
Preserve `--model <provider/model>` and `--variant <value>` when present.
Do not restate or expand the arguments unless needed for execution.
