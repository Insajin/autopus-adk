---
description: "코드 리뷰 — TRUST 5 기준으로 변경된 코드를 리뷰합니다"
agent: build
---

`$ARGUMENTS`

Treat the text above as the full argument payload for `/auto-review`.
Reinterpret it as `/auto review ...` payload로 다시 해석하고, `skill` 도구로 `auto`를 로드한 뒤 canonical router 규칙을 따르세요.
Preserve `--model <provider/model>` and `--variant <value>` when present.
Do not restate or expand the arguments unless needed for execution.
