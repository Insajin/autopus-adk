---
name: auto
description: Autopus 명령 라우터 — oh-my-pi helper
compatibility: omp
---

# Autopus 명령 라우터

`$ARGUMENTS`

## Router Contract

Treat the payload above as the complete text supplied after `/auto`.
Route to exactly one matching detail skill from this exact map: `auto-setup`, `auto-status`, `auto-goal`, `auto-update`, `auto-plan`, `auto-go`, `auto-fix`, `auto-review`, `auto-sync`, `auto-idea`, `auto-map`, `auto-why`, `auto-verify`, `auto-secure`, `auto-test`, `auto-qa`, `auto-dev`, `auto-canary`, `auto-doctor`.
Preserve `--model <provider/model>` and `--variant <value>` exactly as supplied.
For `go`, preserve at most one `--execution-owner omp|orca` pair exactly; omission defaults to `omp`, and aliases or fuzzy correction are forbidden.
Do not fuzzy-correct an unknown subcommand; report it as unsupported and show the exact map.
This is an emitted routing contract, not an OMP runtime parser or model-quality claim.
