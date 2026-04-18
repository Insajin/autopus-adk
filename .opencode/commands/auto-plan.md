---
description: "SPEC 작성 — 코드베이스 분석 후 EARS 요구사항, 구현 계획, 인수 기준을 생성합니다"
agent: build
---

`$ARGUMENTS`

Treat the text above as the full argument payload for `/auto-plan`.
Reinterpret it as `/auto plan ...` payload로 다시 해석하고, `skill` 도구로 `auto`를 로드한 뒤 canonical router 규칙을 따르세요.
Preserve `--model <provider/model>` and `--variant <value>` when present.
Do not restate or expand the arguments unless needed for execution.
