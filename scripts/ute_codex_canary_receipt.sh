#!/usr/bin/env bash

telemetry_file_for_call() {
  local state=$1
  local files=()
  shopt -s nullglob
  files=("$state"/.autopus/telemetry/*.jsonl)
  shopt -u nullglob
  [[ ${#files[@]} -eq 1 ]] || return 1
  [[ "$(awk 'END {print NR+0}' "${files[0]}")" == 1 ]] || return 1
  printf '%s\n' "${files[0]}"
}

materialize_receipt_json() {
  local result_yaml=$1 telemetry=$2 result_json=$3 event_json=$4
  yq -o=json '.' "$result_yaml" > "$result_json" || return 1
  jq -e 'type == "object"' "$result_json" >/dev/null || return 1
  jq -e 'select((type == "object") and (.type == "agent_run") and ((.data | type) == "object"))' \
    "$telemetry" > "$event_json" || return 1
  chmod 600 "$result_json" "$event_json"
}

validate_receipt_identity() {
  local result=$1 event=$2 task=$3 role=$4 effort=$5 run_id=$6 call_id=$7 budget=$8
  jq -e --arg task "$task" --arg role "$role" --arg effort "$effort" --arg run "$run_id" \
    --arg call "$call_id" --argjson budget "$budget" '
      .task_id == $task and .status == "success" and .provider == "codex" and .model == "gpt-5.6-sol" and
      .effort == $effort and .run_id == $run and .call_id == $call and .attempt == 1 and
      .phase == "review" and .role == $role and .verdict == "PASS" and .finding_count == 0 and
      (.output_sha256 | test("^[0-9a-f]{64}$")) and .usage_status == "actual" and
      .unique_model_call_count == 1 and .tool_calls == 0 and
      (.raw_total_tokens | type == "number" and . > 0 and . <= $budget)
    ' "$result" >/dev/null || return 1
  jq -e --arg task "$task" --arg role "$role" --arg effort "$effort" --arg run "$run_id" \
    --arg call "$call_id" --arg config "$CONFIG_HASH" --arg policy "$POLICY_HASH" --argjson budget "$budget" '
      .type == "agent_run" and .data.task_id == $task and .data.run_id == $run and .data.call_id == $call and
      .data.attempt == 1 and .data.provider == "codex" and .data.model == "gpt-5.6-sol" and
      .data.effort == $effort and .data.phase == "review" and .data.role == $role and
      .data.status == "PASS" and .data.acceptance_status == "PASS" and (.data.tool_calls // 0) == 0 and
      (.data.usage | length) == 1 and .data.usage[0] as $u |
      $u.version == 1 and $u.task_id == $task and $u.run_id == $run and $u.call_id == $call and
      $u.attempt == 1 and $u.provider == "codex" and $u.model == "gpt-5.6-sol" and $u.effort == $effort and
      $u.provider_version == "0.144.1" and $u.model_version == "gpt-5.6-sol" and
      $u.risk_policy == $policy and $u.cache_stratum == "provider-managed-stable-prefix-v1" and
      $u.config_hash == $config and $u.phase == "review" and $u.role == $role and
      $u.usage_status == "actual" and $u.usage_source == "provider" and
      $u.source_schema == "codex.exec-json.turn.completed.v1" and
      ($u.raw_total_tokens | type == "number" and . > 0 and . <= $budget)
    ' "$event" >/dev/null || return 1
  local result_raw usage_raw
  result_raw=$(jq -r '.raw_total_tokens' "$result")
  usage_raw=$(jq -r '.data.usage[0].raw_total_tokens' "$event")
  [[ "$result_raw" == "$usage_raw" ]]
}

build_sanitized_row() {
  local result=$1 event=$2 seq=$3 task=$4 arm=$5 order=$6 profile=$7 role=$8 ordinal=$9
  local effort=${10} budget=${11} output=${12}
  jq -n --slurpfile r "$result" --slurpfile e "$event" --argjson sequence "$seq" --arg task "$task" \
    --arg arm "$arm" --arg order "$order" --arg profile "$profile" --arg role "$role" \
    --argjson ordinal "$ordinal" --arg effort "$effort" --argjson budget "$budget" '
      $r[0] as $r | $e[0].data as $a | $a.usage[0] as $u |
      {sequence:$sequence,task_id:$task,arm:$arm,order:$order,profile:$profile,role:$role,
       role_ordinal:$ordinal,effort:$effort,raw_token_budget:$budget,
       result:{status:$r.status,verdict:$r.verdict,finding_count:$r.finding_count,
         output_sha256:$r.output_sha256,usage_status:$r.usage_status,
         unique_model_call_count:$r.unique_model_call_count,raw_total_tokens:$r.raw_total_tokens,
         tool_calls:$r.tool_calls,duration_ms:($r.duration_ms // 0),cost_usd:($r.cost_usd // null),
         run_id:$r.run_id,call_id:$r.call_id},
       agent_run:{status:$a.status,acceptance_status:$a.acceptance_status,tool_calls:($a.tool_calls // 0),
         duration_ns:$a.duration_ns,files_modified:$a.files_modified,estimated_tokens:$a.estimated_tokens},
       usage:{version:$u.version,provider:$u.provider,model:$u.model,effort:$u.effort,
         provider_version:$u.provider_version,model_version:$u.model_version,risk_policy:$u.risk_policy,
         cache_stratum:$u.cache_stratum,config_hash:$u.config_hash,phase:$u.phase,role:$u.role,
         usage_status:$u.usage_status,usage_source:$u.usage_source,source_schema:$u.source_schema,
         input_tokens_total:$u.input_tokens_total,uncached_input_tokens:$u.uncached_input_tokens,
         cached_input_tokens:$u.cached_input_tokens,cache_creation_input_tokens:$u.cache_creation_input_tokens,
         cache_read_input_tokens:$u.cache_read_input_tokens,output_tokens_total:$u.output_tokens_total,
         reasoning_tokens:$u.reasoning_tokens,reasoning_relation:($u.reasoning_relation // null),
         tool_tokens:$u.tool_tokens,tool_relation:($u.tool_relation // null),raw_total_tokens:$u.raw_total_tokens,
         actual_cost_usd:$u.actual_cost_usd,run_id:$u.run_id,call_id:$u.call_id,task_id:$u.task_id,
         attempt:$u.attempt}}
    ' > "$output"
  chmod 600 "$output"
}

build_failure_stub() {
  local output=$1 seq=$2 task=$3 arm=$4 order=$5 profile=$6 role=$7 ordinal=$8 effort=$9 budget=${10} code=${11}
  jq -n --argjson sequence "$seq" --arg task "$task" --arg arm "$arm" --arg order "$order" \
    --arg profile "$profile" --arg role "$role" --argjson ordinal "$ordinal" --arg effort "$effort" \
    --argjson budget "$budget" --arg code "$code" '{sequence:$sequence,task_id:$task,arm:$arm,order:$order,
      profile:$profile,role:$role,role_ordinal:$ordinal,effort:$effort,raw_token_budget:$budget,
      result:{status:"missing_or_invalid_sanitized_receipt",failure_code:$code}}' > "$output"
  chmod 600 "$output"
}

validate_primary_ledger_for_rollback() {
  local ledger=$1 expected actual expected_codex_hash
  verify_named_sidecar "$ledger" "${ledger%.json}.sha256" || return 1
  expected_codex_hash="sha256:$(printf 'codex-cli 0.144.1\n' | shasum -a 256 | awk '{print $1}')"
  jq -e --arg corpus "$CORPUS_HASH" --arg cohort "$COHORT_HASH" --arg policy "$POLICY_HASH" \
    --arg config "$CONFIG_HASH" --arg schema "$SCHEMA_HASH" --arg auto "sha256:$(sha256_file "$AUTO")" \
    --arg codex_hash "$expected_codex_hash" '
    .evidence_kind == "gpt_codex_primary_call_ledger" and .completed == true and
    .evaluation_eligible == true and .promotion_eligible == false and
    .planned_calls == 58 and .observed_calls == 58 and (.calls | length) == 58 and
    .planned_worst_case_raw_tokens == 1332000 and .observed_raw_total_tokens > 0 and
    .observed_raw_total_tokens <= 1332000 and
    .identity.provider == "codex" and .identity.model == "gpt-5.6-sol" and
    .identity.provider_version == "0.144.1" and .identity.model_version == "gpt-5.6-sol" and
    .identity.effort_policy == "codex_review_xhigh_security_max_v1" and
    .identity.cache_stratum == "provider-managed-stable-prefix-v1" and
    .identity.corpus_sha256 == $corpus and .identity.cohort_sha256 == $cohort and
    .identity.policy_sha256 == $policy and .identity.config_sha256 == $config and
    .identity.verdict_schema_sha256 == $schema and .identity.auto_executable_sha256 == $auto and
    .identity.codex_cli_version == "0.144.1" and .identity.codex_cli_version_receipt_sha256 == $codex_hash and
    ([.calls[].sequence] == [range(1;59)]) and
    ([.calls[].effort] | map(select(. == "xhigh")) | length) == 44 and
    ([.calls[].effort] | map(select(. == "max")) | length) == 14 and
    ([.calls[].result.call_id] | unique | length) == 58 and
    ([.calls[].result.run_id] | unique | length) == 58 and
    ([.calls[].result.raw_total_tokens] | add) == .observed_raw_total_tokens and
    all(.calls[]; .result.status == "success" and .result.verdict == "PASS" and
      .result.finding_count == 0 and .result.tool_calls == 0 and .result.usage_status == "actual" and
      .result.unique_model_call_count == 1 and (.result.call_id | test("^c[0-9a-f]{24}$")) and
      (.result.run_id | test("^r[0-9a-f]{24}$")) and .usage.call_id == .result.call_id and
      .usage.run_id == .result.run_id and .usage.task_id == .task_id and .usage.role == .role and
      .usage.effort == .effort and .usage.raw_total_tokens == .result.raw_total_tokens and
      .usage.risk_policy == $policy and .usage.config_hash == $config and
      .usage.cache_stratum == "provider-managed-stable-prefix-v1" and
      .agent_run.status == "PASS" and .agent_run.acceptance_status == "PASS" and .agent_run.tool_calls == 0)
  ' "$ledger" >/dev/null
  expected=$(emit_primary_schedule)
  actual=$(jq -r '.calls[] | [.sequence,.task_id,.arm,.order,.profile,.role,.role_ordinal,.effort,.raw_token_budget] | @tsv' "$ledger")
  [[ "$actual" == "$expected" ]]
}

validate_applied_rollback_receipt() {
  local receipt=$1 expected=$2 actual
  [[ -f "$receipt" && "$expected" =~ ^sha256:[0-9a-f]{64}$ ]] || return 1
  actual="sha256:$(sha256_file "$receipt")"
  [[ "$actual" == "$expected" ]] || return 1
  jq -e --arg policy "$POLICY_HASH" --arg config "$CONFIG_HASH" --arg primary "$PRIMARY_LEDGER_HASH" '
    .version == 1 and .spec_id == "SPEC-ADK-ULTRA-EFFICIENCY-001" and
    .evidence_kind == "applied_policy_rollback" and .decision == "ROLLBACK" and
    .active_profile == "full_ultra" and .applied == true and .atomic_replace == true and
    .state_readback == "full_ultra" and .policy_sha256 == $policy and .config_sha256 == $config and
    .primary_ledger_sha256 == $primary and
    (.logical_rollback_result_sha256 | test("^sha256:[0-9a-f]{64}$")) and
    (.before_binding_sha256 | test("^sha256:[0-9a-f]{64}$")) and
    (.after_binding_sha256 | test("^sha256:[0-9a-f]{64}$")) and
    .before_binding_sha256 != .after_binding_sha256
  ' "$receipt" >/dev/null
}
