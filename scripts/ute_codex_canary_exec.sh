#!/usr/bin/env bash

prepare_runtime_identity() {
  local auto_out="$TEMP_ROOT/auto-version" auto_err="$TEMP_ROOT/auto-version.err"
  local codex_out="$TEMP_ROOT/codex-version" codex_err="$TEMP_ROOT/codex-version.err" codex_bin
  : > "$auto_out"; : > "$auto_err"; : > "$codex_out"; : > "$codex_err"
  chmod 600 "$auto_out" "$auto_err" "$codex_out" "$codex_err"
  "$AUTO" version --short > "$auto_out" 2> "$auto_err" || return 1
  AUTO_VERSION=$(tr -d '\r\n' < "$auto_out")
  [[ "$AUTO_VERSION" =~ ^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$ ]] || return 1
  AUTO_SHA="sha256:$(sha256_file "$AUTO")"
  codex_bin=$(command -v codex) || return 1
  codex --version > "$codex_out" 2> "$codex_err" || return 1
  [[ "$(tr -d '\r\n' < "$codex_out")" == "codex-cli 0.144.1" ]] || return 1
  CODEX_VERSION=0.144.1
  CODEX_VERSION_HASH="sha256:$(sha256_file "$codex_out")"
  rm -f "$auto_out" "$auto_err" "$codex_out" "$codex_err"
}

cleanup_call_artifacts() {
  [[ -z "${RUN_DIR:-}" ]] || rm -rf "$RUN_DIR"
  rm -f "${PATCH_FILE:-}" "${PROMPT_FILE:-}" "${STDOUT_FILE:-}" "${STDERR_FILE:-}" \
    "${RESULT_JSON:-}" "${EVENT_JSON:-}" "${ROW_FILE:-}" "${TELEMETRY_FILE:-}"
  [[ ! -d "$STATE/.autopus/telemetry" ]] || find "$STATE/.autopus/telemetry" -type f -delete
}

cleanup_runtime() {
  cleanup_call_artifacts
  [[ -z "${TEMP_ROOT:-}" ]] || rm -rf "$TEMP_ROOT"
  [[ ! -d "$STATE/.autopus/runs" ]] || find "$STATE/.autopus/runs" -depth -type d -empty -delete
  [[ ! -d "$STATE/.autopus/telemetry" ]] || find "$STATE/.autopus/telemetry" -depth -type d -empty -delete
  [[ ! -d "$STATE/.autopus" ]] || rmdir "$STATE/.autopus" 2>/dev/null || true
}

append_sanitized_row() {
  jq -c '.' "$ROW_FILE" >> "$ROWS_FILE"
  chmod 600 "$ROWS_FILE"
}

capture_call_receipt() {
  local seq=$1 task=$2 arm=$3 order=$4 profile=$5 role=$6 ordinal=$7 effort=$8 budget=$9 exit_code=${10}
  local result_yaml telemetry
  result_yaml="$RUN_DIR/result.yaml"
  if [[ ! -f "$result_yaml" ]]; then
    build_failure_stub "$ROW_FILE" "$seq" "$task" "$arm" "$order" "$profile" "$role" "$ordinal" "$effort" "$budget" missing_result
    append_sanitized_row; CALL_FAILURE=missing_result; return 1
  fi
  telemetry=$(telemetry_file_for_call "$STATE") || {
    build_failure_stub "$ROW_FILE" "$seq" "$task" "$arm" "$order" "$profile" "$role" "$ordinal" "$effort" "$budget" missing_telemetry
    append_sanitized_row; CALL_FAILURE=missing_telemetry; return 1
  }
  TELEMETRY_FILE=$telemetry
  if ! materialize_receipt_json "$result_yaml" "$telemetry" "$RESULT_JSON" "$EVENT_JSON"; then
    build_failure_stub "$ROW_FILE" "$seq" "$task" "$arm" "$order" "$profile" "$role" "$ordinal" "$effort" "$budget" invalid_receipt
    append_sanitized_row; CALL_FAILURE=invalid_receipt; return 1
  fi
  if ! build_sanitized_row "$RESULT_JSON" "$EVENT_JSON" "$seq" "$task" "$arm" "$order" "$profile" "$role" \
    "$ordinal" "$effort" "$budget" "$ROW_FILE"; then
    build_failure_stub "$ROW_FILE" "$seq" "$task" "$arm" "$order" "$profile" "$role" "$ordinal" "$effort" "$budget" sanitize_failed
    append_sanitized_row; CALL_FAILURE=sanitize_failed; return 1
  fi
  if ! scan_retained_ledger "$ROW_FILE"; then
    build_failure_stub "$ROW_FILE" "$seq" "$task" "$arm" "$order" "$profile" "$role" "$ordinal" "$effort" "$budget" retained_field_scan
    append_sanitized_row; CALL_FAILURE=retained_field_scan; return 1
  fi
  append_sanitized_row
  if [[ "$exit_code" != 0 ]]; then CALL_FAILURE=process_nonzero; return 1; fi
  validate_receipt_identity "$RESULT_JSON" "$EVENT_JSON" "$task" "$role" "$effort" "$RUN_ID" "$CALL_ID" "$budget" || {
    CALL_FAILURE=receipt_contract; return 1;
  }
  RECEIPT_RAW=$(jq -r '.raw_total_tokens' "$RESULT_JSON")
  return 0
}

execute_one_call() {
  local seq=$1 task=$2 arm=$3 order=$4 profile=$5 role=$6 ordinal=$7 effort=$8 budget=$9
  local marker exit_code
  if declare -F canary_pre_call_guard >/dev/null 2>&1; then
    canary_pre_call_guard || { CALL_FAILURE=runtime_integrity; return 1; }
  fi
  RUN_DIR="$STATE/.autopus/runs/$task"
  mkdir -p "$RUN_DIR"; chmod 700 "$RUN_DIR"
  PATCH_FILE="$TEMP_ROOT/patch-$seq"; PROMPT_FILE="$TEMP_ROOT/prompt-$seq"
  STDOUT_FILE="$TEMP_ROOT/stdout-$seq"; STDERR_FILE="$TEMP_ROOT/stderr-$seq"
  RESULT_JSON="$TEMP_ROOT/result-$seq.json"; EVENT_JSON="$TEMP_ROOT/event-$seq.json"; ROW_FILE="$TEMP_ROOT/row-$seq.json"
  : > "$STDOUT_FILE"; : > "$STDERR_FILE"; chmod 600 "$STDOUT_FILE" "$STDERR_FILE"
  RUN_ID=$(make_opaque_id run "$MODE" "$seq" "$task")
  CALL_ID=$(make_opaque_id call "$MODE" "$seq" "$task")
  marker="UTE-RAW-PROMPT-${CALL_ID}"
  materialize_patch "$task" "$PATCH_FILE" || { CALL_FAILURE=patch_integrity; return 1; }
  materialize_prompt "$task" "$role" "$ordinal" "$marker" "$PATCH_FILE" "$SUMMARIES_FILE" "$PROMPT_FILE" || {
    CALL_FAILURE=prompt_contract; return 1;
  }
  materialize_context "$task" "$role" "$effort" "$budget" "$RUN_ID" "$CALL_ID" "$PROMPT_FILE" "$RUN_DIR" || {
    CALL_FAILURE=context_contract; return 1;
  }
  set +e
  (cd "$STATE" && "$AUTO" agent run "$task") > "$STDOUT_FILE" 2> "$STDERR_FILE"
  exit_code=$?
  set -e
  capture_call_receipt "$seq" "$task" "$arm" "$order" "$profile" "$role" "$ordinal" "$effort" "$budget" "$exit_code"
}

record_bounded_summary() {
  local role=$1 ordinal=$2
  printf '%s#%s PASS findings=0\n' "$role" "$ordinal" >> "$SUMMARIES_FILE"
  chmod 600 "$SUMMARIES_FILE"
}

fail_execution() {
  local code=$1
  local attempted
  cleanup_call_artifacts
  attempted=$(wc -l < "$ROWS_FILE" | tr -d ' ')
  persist_ledger "$MODE" false "$code" "$ROWS_FILE" || persist_retention_failure "$MODE" "$attempted"
  cleanup_runtime
  trap - EXIT INT TERM
  scan_retained_ledger "$PERSISTED_LEDGER" || return 1
  return 1
}

run_canary() {
  local schedule_file seq task arm order profile role ordinal effort budget
  local planned cumulative_worst=0 cumulative_raw=0 last_group= group total
  TEMP_ROOT="$STATE/.ute-canary-temp"; mkdir "$TEMP_ROOT"; chmod 700 "$TEMP_ROOT"
  trap cleanup_runtime EXIT INT TERM
  ROWS_FILE="$TEMP_ROOT/rows.jsonl"; SUMMARIES_FILE="$TEMP_ROOT/summaries"; : > "$ROWS_FILE"; : > "$SUMMARIES_FILE"
  chmod 600 "$ROWS_FILE" "$SUMMARIES_FILE"
  AUTO_SHA=unavailable AUTO_VERSION=unavailable CODEX_VERSION=unavailable CODEX_VERSION_HASH=unavailable
  prepare_runtime_identity || fail_execution runtime_identity
  if declare -F canary_prepare_runtime_inputs >/dev/null 2>&1; then
    canary_prepare_runtime_inputs || fail_execution runtime_inputs
  fi
  schedule_file="$TEMP_ROOT/schedule.tsv"
  if [[ "$MODE" == primary ]]; then emit_primary_schedule > "$schedule_file"; planned=58
  else emit_rollback_schedule > "$schedule_file"; planned=5; fi
  validate_schedule_arithmetic "$schedule_file" || fail_execution schedule_arithmetic
  while IFS=$'\t' read -r seq task arm order profile role ordinal effort budget; do
    cumulative_worst=$((cumulative_worst + budget))
    if [[ "$MODE" == primary ]]; then
      (( cumulative_worst <= 1332000 && 1446000 <= 1500000 && seq <= 58 )) || fail_execution admission
    else
      (( cumulative_worst <= 114000 && seq <= 5 )) || fail_execution admission
      (( PRIMARY_OBSERVED_RAW + cumulative_raw + budget <= 1500000 )) || fail_execution admission
    fi
    group="$task:$arm"
    if [[ "$group" != "$last_group" ]]; then : > "$SUMMARIES_FILE"; last_group=$group; fi
    CALL_FAILURE=unknown
    execute_one_call "$seq" "$task" "$arm" "$order" "$profile" "$role" "$ordinal" "$effort" "$budget" || \
      fail_execution "$CALL_FAILURE"
    cumulative_raw=$((cumulative_raw + RECEIPT_RAW))
    (( cumulative_raw <= 1500000 )) || fail_execution observed_cap
    record_bounded_summary "$role" "$ordinal"
    cleanup_call_artifacts
    printf 'call=%s/%s task=%s role=%s raw=%s cumulative=%s\n' \
      "$seq" "$planned" "$task" "$role" "$RECEIPT_RAW" "$cumulative_raw"
  done < "$schedule_file"
  total=$(wc -l < "$ROWS_FILE" | tr -d ' ')
  [[ "$total" == "$planned" ]] || fail_execution incomplete_schedule
  if ! persist_ledger "$MODE" true none "$ROWS_FILE"; then
    persist_retention_failure "$MODE" "$total"
    cleanup_runtime
    trap - EXIT INT TERM
    return 1
  fi
  cleanup_runtime
  trap - EXIT INT TERM
  scan_retained_ledger "$PERSISTED_LEDGER" || return 1
}
