---
description: "E2E 시나리오 실행 — scenarios.md 기반 검증을 수행합니다"
agent: build
---

`$ARGUMENTS`

Treat the text above as the full argument payload for `/auto-test`.
Reinterpret it as `/auto test ...` payload로 다시 해석하고, `skill` 도구로 `auto`를 로드한 뒤 canonical router 규칙을 따르세요.
Preserve `--model <provider/model>` and `--variant <value>` when present.
Do not restate or expand the arguments unless needed for execution.
