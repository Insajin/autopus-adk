---
description: "Autopus 명령 라우터 — OpenCode helper 및 workflow 서브커맨드를 해석합니다"
agent: build
---

`$ARGUMENTS`

Treat the text above as the full payload passed after `/auto`.
Immediately load skill `auto` and use it as the canonical router.
Strip global flags first, resolve the first remaining token as the subcommand, then load the matching `auto-*` skill.
Preserve `--model <provider/model>` and `--variant <value>` when present.
Do not restate or expand the arguments unless needed for execution.
