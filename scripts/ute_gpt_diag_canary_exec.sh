#!/usr/bin/env bash

prepare_diag_runtime_identity() {
  local auto_out="$TEMP_ROOT/auto-version" codex_out="$TEMP_ROOT/codex-version"
  : > "$auto_out"; : > "$codex_out"; chmod 600 "$auto_out" "$codex_out"
  "$AUTO" version --short > "$auto_out" 2>/dev/null || return 1
  AUTO_VERSION=$(tr -d '\r\n' < "$auto_out"); [[ "$AUTO_VERSION" =~ ^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$ ]] || return 1
  AUTO_SHA="sha256:$(sha256_file "$AUTO")"
  codex --version > "$codex_out" 2>/dev/null || return 1
  [[ "$(tr -d '\r\n' < "$codex_out")" == "codex-cli 0.144.1" ]] || return 1
  CODEX_VERSION=0.144.1; rm -f "$auto_out" "$codex_out"
}

cleanup_diag_call() {
  [[ -z "${RUN_DIR:-}" ]] || rm -rf "$RUN_DIR"
  rm -f "${PATCH_FILE:-}" "${SCOPE_FILE:-}" "${PROMPT_FILE:-}" "${STDOUT_FILE:-}" "${STDERR_FILE:-}" \
    "${RESULT_JSON:-}" "${EVENT_JSON:-}" "${ROW_FILE:-}" "${TELEMETRY_FILE:-}"
  [[ ! -d "$STATE/.autopus/telemetry" ]] || find "$STATE/.autopus/telemetry" -type f -delete
}

cleanup_diag_runtime() {
  cleanup_diag_call
  [[ -z "${TEMP_ROOT:-}" ]] || rm -rf "$TEMP_ROOT"
  [[ ! -d "$STATE/.autopus/runs" ]] || find "$STATE/.autopus/runs" -depth -type d -empty -delete
  [[ ! -d "$STATE/.autopus/telemetry" ]] || find "$STATE/.autopus/telemetry" -depth -type d -empty -delete
  [[ ! -d "$STATE/.autopus" ]] || rmdir "$STATE/.autopus" 2>/dev/null || true
}

append_diag_row() { jq -c '.' "$ROW_FILE" >> "$ROWS_FILE"; chmod 600 "$ROWS_FILE"; }

capture_diag_call() {
  local seq=$1 arm=$2 role=$3 ordinal=$4 effort=$5 budget=$6 exit_code=$7 telemetry result_yaml
  result_yaml="$RUN_DIR/result.yaml"
  if [[ ! -f "$result_yaml" ]]; then
    build_diag_failure_row "$ROW_FILE" "$seq" "$arm" "$role" "$ordinal" "$effort" "$budget" missing_result
    append_diag_row; DIAG_CALL_FAILURE=missing_result; return 1
  fi
  # A non-zero provider process is operational failure, not an admitted
  # diagnostic receipt. Do not promote any unvalidated result or telemetry
  # fields into the partial ledger.
  if [[ "$exit_code" != 0 ]]; then
    build_diag_failure_row "$ROW_FILE" "$seq" "$arm" "$role" "$ordinal" "$effort" "$budget" process_nonzero
    append_diag_row; DIAG_CALL_FAILURE=process_nonzero; return 1
  fi
  telemetry=$(diag_telemetry_file) || {
    build_diag_failure_row "$ROW_FILE" "$seq" "$arm" "$role" "$ordinal" "$effort" "$budget" missing_telemetry
    append_diag_row; DIAG_CALL_FAILURE=missing_telemetry; return 1
  }
  TELEMETRY_FILE=$telemetry
  if ! materialize_diag_receipts "$result_yaml" "$telemetry" "$RESULT_JSON" "$EVENT_JSON"; then
    build_diag_failure_row "$ROW_FILE" "$seq" "$arm" "$role" "$ordinal" "$effort" "$budget" invalid_receipt
    append_diag_row; DIAG_CALL_FAILURE=invalid_receipt; return 1
  fi
  if ! build_diag_row "$RESULT_JSON" "$EVENT_JSON" "$seq" "$arm" "$role" "$ordinal" "$effort" "$budget" "$ROW_FILE"; then
    build_diag_failure_row "$ROW_FILE" "$seq" "$arm" "$role" "$ordinal" "$effort" "$budget" sanitize_failed
    append_diag_row; DIAG_CALL_FAILURE=sanitize_failed; return 1
  fi
  if ! scan_diag_ledger "$ROW_FILE"; then
    build_diag_failure_row "$ROW_FILE" "$seq" "$arm" "$role" "$ordinal" "$effort" "$budget" retained_field_scan
    append_diag_row; DIAG_CALL_FAILURE=retained_field_scan; return 1
  fi
  append_diag_row
  validate_diag_receipts "$RESULT_JSON" "$EVENT_JSON" "$role" "$effort" "$RUN_ID" "$CALL_ID" "$budget" || {
    DIAG_CALL_FAILURE=receipt_contract; return 1;
  }
  DIAG_RAW=$(jq -r '.raw_total_tokens' "$RESULT_JSON"); DIAG_VERDICT=$(jq -r '.verdict' "$RESULT_JSON")
}

execute_diag_call() {
  local seq=$1 arm=$2 role=$3 ordinal=$4 effort=$5 budget=$6 marker exit_code
  RUN_DIR="$STATE/.autopus/runs/ute-corpus-v1-006"; mkdir -p "$RUN_DIR"; chmod 700 "$RUN_DIR"
  PATCH_FILE="$TEMP_ROOT/patch-$seq"; SCOPE_FILE="$TEMP_ROOT/scope-$seq"; PROMPT_FILE="$TEMP_ROOT/prompt-$seq"
  STDOUT_FILE="$TEMP_ROOT/stdout-$seq"; STDERR_FILE="$TEMP_ROOT/stderr-$seq"
  RESULT_JSON="$TEMP_ROOT/result-$seq.json"; EVENT_JSON="$TEMP_ROOT/event-$seq.json"; ROW_FILE="$TEMP_ROOT/row-$seq.json"
  : > "$STDOUT_FILE"; : > "$STDERR_FILE"; chmod 600 "$STDOUT_FILE" "$STDERR_FILE"
  RUN_ID=$(diag_opaque_id run "$seq"); CALL_ID=$(diag_opaque_id call "$seq"); marker="UTE-DIAG-RAW-$CALL_ID"
  materialize_diag_patch_scope "$PATCH_FILE" "$SCOPE_FILE" || { DIAG_CALL_FAILURE=patch_scope; return 1; }
  materialize_diag_prompt "$role" "$ordinal" "$marker" "$PATCH_FILE" "$SCOPE_FILE" "$SUMMARIES_FILE" "$PROMPT_FILE" || {
    DIAG_CALL_FAILURE=prompt_contract; return 1;
  }
  materialize_diag_context "$role" "$effort" "$budget" "$RUN_ID" "$CALL_ID" "$PROMPT_FILE" "$RUN_DIR" || {
    DIAG_CALL_FAILURE=context_contract; return 1;
  }
  set +e
  (cd "$STATE" && "$AUTO" agent run ute-corpus-v1-006) > "$STDOUT_FILE" 2> "$STDERR_FILE"; exit_code=$?
  set -e
  capture_diag_call "$seq" "$arm" "$role" "$ordinal" "$effort" "$budget" "$exit_code"
}

fail_diag_run() {
  local code=$1
  cleanup_diag_call
  persist_diag_ledger false "$code" "$ROWS_FILE" || true
  cleanup_diag_runtime; trap - EXIT INT TERM; return 1
}

run_diag_canary() {
  local schedule seq task arm order profile role ordinal effort budget group= last_group= cumulative_worst=0 cumulative_raw=0 total
  TEMP_ROOT="$STATE/.ute-gpt-diag-temp"; mkdir "$TEMP_ROOT"; chmod 700 "$TEMP_ROOT"; trap cleanup_diag_runtime EXIT INT TERM
  ROWS_FILE="$TEMP_ROOT/rows.jsonl"; SUMMARIES_FILE="$TEMP_ROOT/summaries"; : > "$ROWS_FILE"; : > "$SUMMARIES_FILE"
  chmod 600 "$ROWS_FILE" "$SUMMARIES_FILE"
  AUTO_SHA=unavailable AUTO_VERSION=unavailable CODEX_VERSION=unavailable
  prepare_diag_runtime_identity || fail_diag_run runtime_identity
  schedule="$TEMP_ROOT/schedule"; emit_diag_schedule > "$schedule"; validate_diag_schedule "$schedule" || fail_diag_run schedule
  while IFS=$'\t' read -r seq task arm order profile role ordinal effort budget; do
    cumulative_worst=$((cumulative_worst + budget)); (( cumulative_worst <= 228000 && seq <= 10 )) || fail_diag_run admission
    group="$task:$arm"; if [[ "$group" != "$last_group" ]]; then : > "$SUMMARIES_FILE"; last_group=$group; fi
    DIAG_CALL_FAILURE=unknown
    execute_diag_call "$seq" "$arm" "$role" "$ordinal" "$effort" "$budget" || fail_diag_run "$DIAG_CALL_FAILURE"
    cumulative_raw=$((cumulative_raw + DIAG_RAW)); (( cumulative_raw <= 228000 )) || fail_diag_run observed_cap
    jq -c '{role:.role,role_ordinal:.role_ordinal,verdict:.result.verdict,finding_count:.result.finding_count,
      finding_codes:.result.finding_codes,finding_scope_hashes:.result.finding_scope_hashes}' "$ROW_FILE" >> "$SUMMARIES_FILE"
    cleanup_diag_call
    printf 'call=%s/10 task=ute-corpus-v1-006 role=%s verdict=%s raw=%s cumulative=%s\n' \
      "$seq" "$role" "$DIAG_VERDICT" "$DIAG_RAW" "$cumulative_raw"
  done < "$schedule"
  total=$(wc -l < "$ROWS_FILE" | tr -d ' '); [[ "$total" == 10 ]] || fail_diag_run incomplete
  persist_diag_ledger true none "$ROWS_FILE" || fail_diag_run persistence
  cleanup_diag_runtime; trap - EXIT INT TERM; scan_diag_ledger "$DIAG_PERSISTED_LEDGER"
}
