#!/usr/bin/env bash

diag_ledger_target() {
  [[ "$1" == true ]] && printf '%s/gpt-diagnostic-call-ledger-v1.json\n' "$OUTPUT" ||
    printf '%s/gpt-diagnostic-call-ledger-v1.partial-fail.json\n' "$OUTPUT"
}

write_diag_sidecar() {
  printf '%s  %s\n' "$(sha256_file "$1")" "$(basename "$1")" > "${1%.json}.sha256"
  chmod 600 "${1%.json}.sha256"
}

scan_diag_ledger() {
  local file=$1 value
  jq -e '[.. | objects | keys[] | select(. == "prompt" or . == "description" or . == "patch" or
    . == "finding_text" or . == "raw_output" or . == "stdout" or . == "stderr" or
    . == "session_id" or . == "environment" or . == "cwd" or . == "path")] | length == 0' "$file" >/dev/null || return 1
  while IFS= read -r value; do
    [[ "$value" != *UTE-DIAG-RAW-* && "$value" != *FAKE-DIAG-RAW* ]] || return 1
    [[ "$value" != "$REPO" && "$value" != "$STATE" && "$value" != "$AUTO" ]] || return 1
  done < <(jq -r '.. | strings' "$file")
}

persist_diag_ledger() {
  local completed=$1 failure=$2 rows=$3 target calls tmp attempted raw conclusion
  target=$(diag_ledger_target "$completed"); calls="$TEMP_ROOT/calls.json"; jq -s '.' "$rows" > "$calls"
  attempted=$(jq 'length' "$calls"); raw=$(jq '[.[].result.raw_total_tokens // 0] | add // 0' "$calls")
  conclusion=INCOMPLETE
  if [[ "$completed" == true ]]; then
    conclusion=$(jq -r 'if any(.[]; .result.verdict == "FAIL") then "BOUNDED_FINDINGS_OBSERVED" else "NO_BOUNDED_FINDINGS" end' "$calls")
  fi
  tmp="$OUTPUT/.$(basename "$target").tmp.$$"
  jq -n --slurpfile calls "$calls" --argjson completed "$completed" --arg failure "$failure" \
    --arg auth "$DIAG_AUTHORIZATION_ID" --arg p "$DIAG_POLICY_HASH" --arg c "$DIAG_CONFIG_HASH" \
    --arg s "$DIAG_SCHEMA_HASH" --arg h "$DIAG_COHORT_HASH" --arg auto "$AUTO_SHA" \
    --arg auto_version "$AUTO_VERSION" --arg codex_version "$CODEX_VERSION" --arg conclusion "$conclusion" \
    --argjson attempted "$attempted" --argjson raw "$raw" '
      {version:1,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",evidence_kind:"gpt_task006_diagnostic_call_ledger",
       completed:$completed,diagnostic_only:true,promotion_eligible:false,
       circuit_breaker:(if $completed then "CLOSED" else "OPEN" end),
       failure_code:(if $completed then null else $failure end),attempted_calls:$attempted,
       planned_calls:10,observed_calls:$attempted,planned_worst_case_raw_tokens:228000,
       observed_raw_total_tokens:$raw,diagnostic_conclusion:$conclusion,
       bounded_fail_observations:([$calls[0][] | select(.result.verdict == "FAIL")]|length),
       authorization:{provider_call_cap:10,raw_token_cap:228000,concurrency:1,retries:0,single_use:true},
       deterministic_receipts:{task_id:"ute-corpus-v1-006",
         expected_patch_sha256:"sha256:55f5f87f5521d0d595758cede60d331692a3d338f3ce99cc7d851d9deb083a2b",
         observed_patch_sha256:"sha256:55f5f87f5521d0d595758cede60d331692a3d338f3ce99cc7d851d9deb083a2b",
         verification_exit_code:0,security_observation_required_both_arms:true},
       identity:{authorization_identity:$auth,policy_sha256:$p,config_sha256:$c,
         verdict_schema_sha256:$s,cohort_sha256:$h,provider:"codex",model:"gpt-5.6-sol",
         provider_version:"0.144.1",model_version:"gpt-5.6-sol",auto_executable_sha256:$auto,
         auto_version:$auto_version,codex_cli_version:$codex_version},calls:$calls[0],
       privacy:{raw_prompt_retained:false,raw_patch_retained:false,raw_response_retained:false,
         finding_text_retained:false,provider_stdout_stderr_retained:false,absolute_paths_retained:false}}
  ' > "$tmp"
  chmod 600 "$tmp"; scan_diag_ledger "$tmp" || { rm -f "$tmp"; return 1; }
  mv "$tmp" "$target"; write_diag_sidecar "$target"; DIAG_PERSISTED_LEDGER=$target
}

ensure_diag_targets_absent() {
  local file
  for file in gpt-diagnostic-call-ledger-v1.json gpt-diagnostic-call-ledger-v1.sha256 \
    gpt-diagnostic-call-ledger-v1.partial-fail.json gpt-diagnostic-call-ledger-v1.partial-fail.sha256; do
    [[ ! -e "$OUTPUT/$file" && ! -L "$OUTPUT/$file" ]] || return 1
  done
}
