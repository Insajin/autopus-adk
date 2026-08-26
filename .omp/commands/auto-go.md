---
description: SPEC 구현 — SPEC 문서를 기반으로 코드를 구현합니다
agent: build
---

`$ARGUMENTS`

Treat the text above as the full argument payload for `/auto-go`.
Load exact detail skill `auto-go`; this is the same target selected by `/auto go ...`.
Preserve `--model <provider/model>` and `--variant <value>` when present.
Do not restate or expand the arguments unless needed for execution.
Accept at most one exact `--execution-owner omp|orca` pair; omission defaults to `omp`, with no aliases or fuzzy fallback.
