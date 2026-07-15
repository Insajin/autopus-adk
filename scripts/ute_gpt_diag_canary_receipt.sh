#!/usr/bin/env bash

diag_telemetry_file() {
  local files=()
  shopt -s nullglob; files=("$STATE"/.autopus/telemetry/*.jsonl); shopt -u nullglob
  [[ ${#files[@]} -eq 1 && "$(awk 'END {print NR+0}' "${files[0]}")" == 1 ]] || return 1
  printf '%s\n' "${files[0]}"
}

materialize_diag_receipts() {
  local result_yaml=$1 telemetry=$2 result_json=$3 event_json=$4
  yq -o=json '.' "$result_yaml" > "$result_json" || return 1
  jq -e 'select(type == "object")' "$result_json" >/dev/null || return 1
  jq -e 'select((type == "object") and (.type == "agent_run") and ((.data | type) == "object"))' \
    "$telemetry" > "$event_json" || return 1
  chmod 600 "$result_json" "$event_json"
}

validate_diag_receipts() {
  local result=$1 event=$2 role=$3 effort=$4 run=$5 call=$6 budget=$7
  jq -e --slurpfile cohort "$COHORT_FILE" --arg role "$role" --arg effort "$effort" --arg run "$run" \
    --arg call "$call" --argjson budget "$budget" '
      (.finding_codes // []) as $codes | (.finding_scope_hashes // []) as $scopes |
      .task_id == "ute-corpus-v1-006" and .status == "success" and .provider == "codex" and
      .model == "gpt-5.6-sol" and .effort == $effort and .run_id == $run and .call_id == $call and
      .attempt == 1 and .phase == "review" and .role == $role and
      (.verdict == "PASS" or .verdict == "FAIL") and (.finding_count | type == "number" and . >= 0 and . <= 3) and
      ($codes | type == "array" and length <= 3 and unique == .) and
      ($scopes | type == "array" and length <= 3 and unique == .) and
      (if .verdict == "PASS" then .finding_count == 0 and ($codes|length)==0 and ($scopes|length)==0
       else .finding_count >= 1 and ($codes|length)==.finding_count and ($scopes|length)>=1 end) and
      all($codes[]; IN("correctness","security","regression","test_gap","task_mismatch","deterministic_conflict","scope_uncertain")) and
      all($scopes[]; . as $hash | any($cohort[0].task.allowed_scope_hashes[]; . == $hash)) and
      (.output_sha256 | test("^[0-9a-f]{64}$")) and .usage_status == "actual" and
      .unique_model_call_count == 1 and .tool_calls == 0 and
      (.raw_total_tokens | type == "number" and . > 0 and . <= $budget)
  ' "$result" >/dev/null || return 1
  jq -e --arg role "$role" --arg effort "$effort" --arg run "$run" --arg call "$call" \
    --arg policy "$DIAG_POLICY_HASH" --arg config "$DIAG_CONFIG_HASH" --argjson budget "$budget" '
      .type == "agent_run" and .data.task_id == "ute-corpus-v1-006" and .data.run_id == $run and
      .data.call_id == $call and .data.attempt == 1 and .data.provider == "codex" and
      .data.model == "gpt-5.6-sol" and .data.effort == $effort and .data.phase == "review" and
      .data.role == $role and .data.status == "PASS" and (.data.tool_calls // 0) == 0 and
      (.data.acceptance_status == "PASS" or .data.acceptance_status == "FAIL") and
      (.data.usage | length) == 1 and .data.usage[0] as $u |
      $u.run_id == $run and $u.call_id == $call and $u.task_id == "ute-corpus-v1-006" and
      $u.provider == "codex" and $u.model == "gpt-5.6-sol" and $u.effort == $effort and
      $u.provider_version == "0.144.1" and $u.model_version == "gpt-5.6-sol" and
      $u.risk_policy == $policy and $u.config_hash == $config and
      $u.cache_stratum == "provider-managed-stable-prefix-v1" and $u.role == $role and
      $u.usage_status == "actual" and $u.usage_source == "provider" and
      $u.source_schema == "codex.exec-json.turn.completed.v1" and
      ($u.raw_total_tokens | type == "number" and . > 0 and . <= $budget)
  ' "$event" >/dev/null || return 1
  [[ "$(jq -r '.raw_total_tokens' "$result")" == "$(jq -r '.data.usage[0].raw_total_tokens' "$event")" ]]
  [[ "$(jq -r '.verdict' "$result")" == "$(jq -r '.data.acceptance_status' "$event")" ]]
}

build_diag_row() {
  local result=$1 event=$2 seq=$3 arm=$4 role=$5 ordinal=$6 effort=$7 budget=$8 output=$9
  jq -n --slurpfile r "$result" --slurpfile e "$event" --argjson seq "$seq" --arg arm "$arm" \
    --arg role "$role" --argjson ordinal "$ordinal" --arg effort "$effort" --argjson budget "$budget" '
      $r[0] as $r | $e[0].data as $a | $a.usage[0] as $u |
      {sequence:$seq,task_id:"ute-corpus-v1-006",arm:$arm,order:"AB",profile:"full5",role:$role,
       role_ordinal:$ordinal,effort:$effort,raw_token_budget:$budget,
       result:{status:$r.status,verdict:$r.verdict,finding_count:$r.finding_count,
         finding_codes:($r.finding_codes // []),finding_scope_hashes:($r.finding_scope_hashes // []),
         output_sha256:$r.output_sha256,usage_status:$r.usage_status,
         unique_model_call_count:$r.unique_model_call_count,raw_total_tokens:$r.raw_total_tokens,
         tool_calls:$r.tool_calls,run_id:$r.run_id,call_id:$r.call_id},
       agent_run:{status:$a.status,acceptance_status:$a.acceptance_status,tool_calls:($a.tool_calls // 0)},
       usage:{provider:$u.provider,model:$u.model,effort:$u.effort,provider_version:$u.provider_version,
         model_version:$u.model_version,risk_policy:$u.risk_policy,cache_stratum:$u.cache_stratum,
         config_hash:$u.config_hash,source_schema:$u.source_schema,raw_total_tokens:$u.raw_total_tokens,
         input_tokens_total:$u.input_tokens_total,output_tokens_total:$u.output_tokens_total,
         run_id:$u.run_id,call_id:$u.call_id,task_id:$u.task_id}}
  ' > "$output"
  chmod 600 "$output"
}

build_diag_failure_row() {
  local output=$1 seq=$2 arm=$3 role=$4 ordinal=$5 effort=$6 budget=$7 code=$8
  jq -n --argjson seq "$seq" --arg arm "$arm" --arg role "$role" --argjson ordinal "$ordinal" \
    --arg effort "$effort" --argjson budget "$budget" --arg code "$code" \
    '{sequence:$seq,task_id:"ute-corpus-v1-006",arm:$arm,order:"AB",profile:"full5",role:$role,
      role_ordinal:$ordinal,effort:$effort,raw_token_budget:$budget,
      result:{status:"missing_or_invalid_sanitized_receipt",failure_code:$code}}' > "$output"
  chmod 600 "$output"
}
