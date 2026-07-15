#!/usr/bin/env bash

write_ledger_sidecar() {
  local file=$1
  printf '%s  %s\n' "$(sha256_file "$file")" "$(basename "$file")" > "${file%.json}.sha256"
  chmod 600 "${file%.json}.sha256"
}

ledger_target_for() {
  local mode=$1 completed=$2 stem
  if [[ "$mode" == primary ]]; then stem=gpt-primary-call-ledger-v1; else stem=gpt-rollback-call-ledger-v1; fi
  [[ "$completed" == true ]] && printf '%s/%s.json\n' "$OUTPUT" "$stem" || printf '%s/%s.partial-fail.json\n' "$OUTPUT" "$stem"
}

persist_ledger() {
  local mode=$1 completed=$2 failure_code=$3 rows=$4 target calls_file
  local kind planned worst evaluation primary_raw=0 combined_raw
  target=$(ledger_target_for "$mode" "$completed")
  calls_file="$TEMP_ROOT/ledger-calls.json"
  jq -s '.' "$rows" > "$calls_file"
  if [[ "$mode" == primary ]]; then
    kind=gpt_codex_primary_call_ledger; planned=58; worst=1332000
    [[ "$completed" == true ]] && evaluation=true || evaluation=false
  else
    kind=applied_rollback_replay; planned=5; worst=114000; evaluation=false
    primary_raw=$(jq -r '.observed_raw_total_tokens' "$PRIMARY_LEDGER")
  fi
  combined_raw=$(jq --argjson primary "$primary_raw" '[.[].result.raw_total_tokens // 0] | add + $primary' "$calls_file")
  local tmp="$OUTPUT/.$(basename "$target").tmp.$$"
  jq -n --slurpfile calls "$calls_file" --arg kind "$kind" --arg mode "$mode" \
    --argjson completed "$completed" --argjson evaluation "$evaluation" --arg failure "$failure_code" \
    --argjson planned "$planned" --argjson worst "$worst" --arg corpus "$CORPUS_HASH" \
    --arg cohort "$COHORT_HASH" --arg policy "$POLICY_HASH" --arg config "$CONFIG_HASH" \
    --arg schema "$SCHEMA_HASH" --arg auto_hash "$AUTO_SHA" --arg auto_version "$AUTO_VERSION" \
    --arg codex_version "$CODEX_VERSION" --arg codex_version_hash "$CODEX_VERSION_HASH" \
    --arg rollback_hash "${ROLLBACK_RECEIPT_HASH:-}" --arg primary_hash "${PRIMARY_LEDGER_HASH:-}" \
    --argjson combined "$combined_raw" '
      ($calls[0] // []) as $rows |
      {version:1,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",evidence_kind:$kind,mode:$mode,
       completed:$completed,evaluation_eligible:$evaluation,promotion_eligible:false,
       circuit_breaker:(if $completed then "CLOSED" else "OPEN" end),
       failure_code:(if $completed then null else $failure end),attempted_calls:($rows|length),
       successful_calls:([$rows[] | select(.result.status == "success" and .result.verdict == "PASS")]|length),
       planned_calls:$planned,observed_calls:($rows|length),planned_worst_case_raw_tokens:$worst,
       observed_raw_total_tokens:([$rows[].result.raw_total_tokens // 0]|add // 0),
       combined_primary_and_replay_observed_raw_tokens:$combined,
       authorization:{provider_call_cap:64,raw_token_cap:1500000,concurrency:1,retries:0,
         planned_total_calls:63,planned_worst_case_raw_tokens:1446000,replay_reserve_tokens:114000},
       identity:{provider:"codex",model:"gpt-5.6-sol",provider_version:"0.144.1",
         model_version:"gpt-5.6-sol",effort_policy:"codex_review_xhigh_security_max_v1",
         cache_stratum:"provider-managed-stable-prefix-v1",corpus_sha256:$corpus,cohort_sha256:$cohort,
         policy_sha256:$policy,config_sha256:$config,verdict_schema_sha256:$schema,
         auto_executable_sha256:$auto_hash,auto_version:$auto_version,codex_cli_version:$codex_version,
         codex_cli_version_receipt_sha256:$codex_version_hash},
       applied_rollback_receipt_sha256:(if $mode == "rollback" then $rollback_hash else null end),
       primary_ledger_sha256:(if $mode == "rollback" then $primary_hash else null end),
       calls:$rows,privacy:{raw_prompt_retained:false,raw_patch_retained:false,raw_response_retained:false,
         provider_stdout_stderr_retained:false,isolated_telemetry_retained:false,absolute_paths_retained:false}}
  ' > "$tmp"
  chmod 600 "$tmp"
  if ! scan_retained_ledger "$tmp"; then rm -f "$tmp"; return 1; fi
  mv "$tmp" "$target"
  write_ledger_sidecar "$target"
  PERSISTED_LEDGER=$target
}

persist_retention_failure() {
  local mode=$1 attempted=$2 target tmp
  target=$(ledger_target_for "$mode" false)
  tmp="$OUTPUT/.$(basename "$target").tmp.$$"
  jq -n --arg mode "$mode" --argjson attempted "$attempted" '{version:1,
    spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",evidence_kind:"retention_scan_failure",
    mode:$mode,completed:false,evaluation_eligible:false,promotion_eligible:false,
    circuit_breaker:"OPEN",failure_code:"retained_field_scan",attempted_calls:$attempted,
    successful_calls:0,calls:[],privacy:{raw_artifacts_retained:false}}' > "$tmp"
  chmod 600 "$tmp"
  scan_retained_ledger "$tmp" || { rm -f "$tmp"; return 1; }
  mv "$tmp" "$target"
  write_ledger_sidecar "$target"
  PERSISTED_LEDGER=$target
}

scan_retained_ledger() {
  local ledger=$1 value
  jq -e '
    [.. | objects | keys[] | select(. == "prompt" or . == "description" or . == "patch" or
      . == "raw_output" or . == "stdout" or . == "stderr" or . == "session_id" or
      . == "environment" or . == "cwd" or . == "path")] | length == 0
  ' "$ledger" >/dev/null || return 1
  while IFS= read -r value; do
    [[ "$value" != *UTE-RAW-PROMPT-* && "$value" != *FAKE-RAW-PROVIDER-BODY* ]] || return 1
    [[ "$value" != "$REPO" && "$value" != "$STATE" && "$value" != "$AUTO" ]] || return 1
    [[ -z "${PRIMARY_LEDGER:-}" || "$value" != "$PRIMARY_LEDGER" ]] || return 1
    [[ -z "${ROLLBACK_RECEIPT:-}" || "$value" != "$ROLLBACK_RECEIPT" ]] || return 1
  done < <(jq -r '.. | strings' "$ledger")
}

ensure_ledger_targets_absent() {
  local mode=$1 stem
  [[ "$mode" == primary ]] && stem=gpt-primary-call-ledger-v1 || stem=gpt-rollback-call-ledger-v1
  local suffix file
  for suffix in .json .sha256 .partial-fail.json .partial-fail.sha256; do
    file="$OUTPUT/$stem$suffix"
    [[ ! -e "$file" ]] || return 1
  done
}
