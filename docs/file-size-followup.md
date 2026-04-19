# File Size Follow-up

Last updated: 2026-04-19
Related commit: `159510e` (`refactor(lint): autopus-adk 잔여 lint 정리`)

## Context

- The lint cleanup commit was created with `--no-verify`.
- This follow-up pass focused on the memo-listed file-size offenders in `pkg/setup`, `pkg/worker`, `pkg/worker/a2a`, `internal/cli/tui`, `pkg/e2e`, and `pkg/worker/setup`.
- Verification completed in this pass:
  - `go test ./pkg/setup`
  - `go test ./pkg/worker`
  - `go test ./pkg/worker/a2a`
  - `go test ./internal/cli/tui`
  - `go test ./pkg/e2e`
  - `go test ./pkg/worker/setup`

## Completed In This Pass

| Original file | Previous lines | Result |
|---|---:|---|
| `internal/cli/tui/wizard_steps_test.go` | 311 | `wizard_steps_test.go` 109 + `wizard_flow_test.go` 204 |
| `pkg/e2e/edge_cases_test.go` | 331 | `edge_cases_test.go` 176 + `edge_cases_sync_env_test.go` 131 |
| `pkg/setup/conventions.go` | 365 | `conventions.go` 184 + `conventions_sample.go` 98 + `conventions_go.go` 90 |
| `pkg/setup/engine.go` | 301 | `engine.go` 145 + `engine_status.go` 81 + `engine_docs.go` 55 |
| `pkg/setup/renderer.go` | 466 | `renderer.go` 35 + `renderer_docs.go` 151 + `renderer_guides.go` 126 + `renderer_arch.go` 105 + `renderer_tree.go` 31 |
| `pkg/setup/scanner.go` | 572 | `scanner.go` 29 + `scanner_detect.go` 192 + `scanner_entry.go` 140 + `scanner_tree.go` 91 + `scanner_parse.go` 107 |
| `pkg/worker/a2a/server_test.go` | 619 | `server_test.go` 235 + `server_control_plane_test.go` 201 + `server_task_test.go` 170 + `server_transport_test.go` 42 |
| `pkg/worker/loop_test.go` | 653 | split across `loop_test_helpers_test.go` 192, `loop_exec_test.go` 96, `loop_handle_task_test.go` 157, `loop_handle_task_validation_test.go` 59, `loop_runtime_env_test.go` 42, `loop_approval_test.go` 54 |
| `pkg/worker/pipeline.go` | 510 | `pipeline.go` 219 + `pipeline_parse.go` 89 + `pipeline_phase.go` 216 |
| `pkg/worker/setup/auth_test.go` | 336 | `auth_test.go` 182 + `auth_pkce_refresh_test.go` 155 |

## Additional Fix Included

| File | Previous lines | Result |
|---|---:|---|
| `pkg/worker/loop.go` | 339 | `loop.go` 89 + `loop_runtime.go` 129 + `loop_task.go` 132 |
| `pkg/worker/pipeline_test.go` | 305 | `pipeline_executor_test.go` 187 + `pipeline_parse_test.go` 113 |

## Completed In Current Batch

Verification completed in this batch:

- `go test ./pkg/terminal`
- `go test ./pkg/orchestra`
- `go test ./pkg/adapter/opencode`
- `go test ./internal/cli`

| Original file | Previous lines | Result |
|---|---:|---|
| `internal/cli/check_rules_test.go` | 393 | `check_rules_test.go` 52 + `check_rules_lore_test.go` 166 + `check_rules_arch_test.go` 100 |
| `internal/cli/cli_coverage2_test.go` | 454 | `cli_coverage2_test.go` 165 + `cli_coverage2_platform_arch_test.go` 121 + `cli_coverage2_tooling_test.go` 139 |
| `internal/cli/cli_extra_test.go` | 621 | `cli_extra_test.go` 115 + `cli_extra_lore_test.go` 127 + `cli_extra_project_test.go` 167 + `cli_extra_platform_test.go` 130 + `cli_extra_lsp_skill_test.go` 122 |
| `internal/cli/coverage_gap_test.go` | 372 | `coverage_gap_test.go` 76 + `coverage_gap_setup_test.go` 123 + `coverage_gap_telemetry_test.go` 183 |
| `pkg/adapter/opencode/opencode_test.go` | 310 | `opencode_test.go` 230 + `opencode_plugins_test.go` 93 |
| `pkg/orchestra/pane_runner.go` | 303 | `pane_runner.go` 257 + `pane_output.go` 53 |
| `pkg/terminal/cmux_test.go` | 339 | `cmux_test.go` 185 + `cmux_long_text_test.go` 155 |

## Remaining Repo-Wide >300 After This Pass

These are outside the original memo batch and still fail a full repo-wide line-count scan:

1. `pkg/lsp/lsp_extra_test.go` (326)
2. `internal/cli/worker_setup_wizard.go` (319)
3. `internal/cli/doctor_test.go` (319)
4. `pkg/selfupdate/downloader_test.go` (318)

## Guardrail

- Keep each resulting source file under 300 lines.
- Prefer a target under 200 lines when the concern boundary is clear.
- Treat these as refactors only; do not mix new behavior into the split work.
