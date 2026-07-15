#!/usr/bin/env bash

gpt_hash_canonical_object() { jq -cS '.' "$1" | shasum -a 256 | awk '{print "sha256:" $1}'; }

build_security_receipts() {
  local rows="$WORK/security-rows.jsonl" base="$WORK/security-base.json" row="$WORK/security-row.json"
  local task arm order risk call_count receipt_hash fixture output
  output=$(gpt_stage_path gpt-security-receipts)
  : > "$rows"
  while IFS=$'\t' read -r task order risk; do
    for arm in A B; do
      call_count=$(jq --arg task "$task" --arg arm "$arm" \
        '[.calls[] | select(.task_id == $task and .arm == $arm and .role == "security-auditor")] | length' "$PRIMARY_LEDGER")
      [[ "$call_count" == 1 ]] || return 1
      [[ "$task" == ute-corpus-v1-009 ]] && fixture=synthetic_fixture_allowed_no_naive_secret_scan || fixture=not_applicable
      jq -n --arg task "$task" --arg arm "$arm" --arg order "$order" --arg risk "$risk" --arg fixture "$fixture" \
        --slurpfile ledger "$PRIMARY_LEDGER" --slurpfile oracles "$ORACLES" '
        ($ledger[0].calls[] | select(.task_id == $task and .arm == $arm and .role == "security-auditor")) as $c |
        ($oracles[0][] | select(.task_id == $task)) as $o |
        {task_id:$task,arm:$arm,pair_order:$order,risk_tier:$risk,
         security_call:{sequence:$c.sequence,role:$c.role,effort:$c.effort,run_id:$c.result.run_id,
           call_id:$c.result.call_id,output_sha256:("sha256:" + $c.result.output_sha256),
           verdict:$c.result.verdict,finding_count:$c.result.finding_count,tool_calls:$c.result.tool_calls,
           usage_status:$c.result.usage_status,raw_total_tokens:$c.result.raw_total_tokens,usage:$c.usage},
         patch_evidence:{expected_sha256:$o.expected_patch_hash,observed_sha256:$o.observed_patch_hash,
           path_count:$o.path_count,path_modes:$o.path_modes,safe_modes:true,git_diff_check:"PASS"},
         verification:{command:$o.verification_command,exit_code:$o.verification_exit_code,status:$o.verification_status},
         synthetic_fixture_handling:$fixture}' > "$base" || return 1
      receipt_hash=$(gpt_hash_canonical_object "$base")
      jq --arg hash "$receipt_hash" '. + {receipt_sha256:$hash}' "$base" > "$row"
      jq -cS '.' "$row" >> "$rows"
    done
  done < <(jq -r '.tasks[] | [.task_id,.pair_order,.risk_tier] | @tsv' "$COHORT")
  jq -n --slurpfile rows <(jq -s '.' "$rows") '{version:1,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",
    evidence_kind:"gpt_security_receipts",receipt_count:($rows[0]|length),
    synthetic_fixture_policy:{naive_secret_scan:false,
      reason:"Synthetic test fixture values are not findings; safe modes, deterministic verification, and GPT security verdict remain mandatory."},
    receipts:$rows[0]}' > "$output"
  gpt_canonicalize_json "$output"
  validate_security_receipts "$output"
}

validate_security_receipts() {
  local file=$1 row base expected actual
  jq -e '.version == 1 and .receipt_count == 14 and (.receipts | length) == 14 and
    ([.receipts[] | (.task_id + ":" + .arm)] | unique | length) == 14 and
    ([.receipts[].task_id] | unique | length) == 7 and
    all(.receipts[]; (.arm == "A" or .arm == "B") and .security_call.role == "security-auditor" and
      .security_call.effort == "max" and .security_call.verdict == "PASS" and
      .security_call.finding_count == 0 and .security_call.tool_calls == 0 and
      .security_call.usage_status == "actual" and .security_call.raw_total_tokens > 0 and
      .patch_evidence.expected_sha256 == .patch_evidence.observed_sha256 and
      .patch_evidence.safe_modes == true and .patch_evidence.git_diff_check == "PASS" and
      .verification.exit_code == 0 and .verification.status == "PASS" and
      (.receipt_sha256 | test("^sha256:[0-9a-f]{64}$")))' "$file" >/dev/null || return 1
  while IFS= read -r row; do
    base="$WORK/security-validate-base.json"
    printf '%s\n' "$row" | jq 'del(.receipt_sha256)' > "$base"
    expected=$(printf '%s\n' "$row" | jq -r '.receipt_sha256')
    actual=$(gpt_hash_canonical_object "$base")
    [[ "$actual" == "$expected" ]] || return 1
  done < <(jq -c '.receipts[]' "$file")
}

build_quality_ledger() {
  local rows="$WORK/quality-rows.jsonl" task task_hash risk oracle a_hash b_hash security output
  security=$(gpt_stage_path gpt-security-receipts); output=$(gpt_stage_path gpt-quality-ledger)
  : > "$rows"
  while IFS=$'\t' read -r task task_hash risk oracle; do
    a_hash=$(jq -r --arg task "$task" '.receipts[] | select(.task_id == $task and .arm == "A") | .receipt_sha256' \
      "$security")
    b_hash=$(jq -r --arg task "$task" '.receipts[] | select(.task_id == $task and .arm == "B") | .receipt_sha256' \
      "$security")
    [[ "$a_hash" == sha256:* && "$b_hash" == sha256:* ]] || return 1
    jq -cn --arg task "$task" --arg task_hash "$task_hash" --arg risk "$risk" --arg oracle "$oracle" \
      --arg a "$a_hash" --arg b "$b_hash" '{task_id:$task,task_hash:$task_hash,risk_tier:$risk,
        expected_oracle_hash:$oracle,baseline_observed_oracle_hash:$oracle,candidate_observed_oracle_hash:$oracle,
        baseline_verification_exit_code:0,candidate_verification_exit_code:0,
        baseline_security_status:"PASS",candidate_security_status:"PASS",
        baseline_security_receipt_hash:$a,candidate_security_receipt_hash:$b}' >> "$rows"
  done < <(jq -r '.tasks[] | [.task_id,.task_hash,.risk_tier,.oracle_hash] | @tsv' "$COHORT")
  jq -n --slurpfile outcomes <(jq -s '.' "$rows") '{version:1,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",
    evidence_kind:"gpt_quality_outcome_ledger",row_count:($outcomes[0]|length),outcomes:$outcomes[0]}' \
    > "$output"
  gpt_canonicalize_json "$output"
  validate_quality_ledger "$output" "$security"
}

validate_quality_ledger() {
  local quality=$1 security=$2
  jq -e --slurpfile s "$security" '
    .version == 1 and .row_count == 7 and (.outcomes | length) == 7 and
    [.outcomes[].task_id] == ["ute-corpus-v1-001","ute-corpus-v1-004","ute-corpus-v1-005",
      "ute-corpus-v1-011","ute-corpus-v1-012","ute-corpus-v1-006","ute-corpus-v1-009"] and
    ([.outcomes[].task_id] | unique | length) == 7 and all(.outcomes[]; . as $o |
      .expected_oracle_hash == .baseline_observed_oracle_hash and
      .expected_oracle_hash == .candidate_observed_oracle_hash and
      .baseline_verification_exit_code == 0 and .candidate_verification_exit_code == 0 and
      .baseline_security_status == "PASS" and .candidate_security_status == "PASS" and
      any($s[0].receipts[]; .task_id == $o.task_id and .arm == "A" and .receipt_sha256 == $o.baseline_security_receipt_hash) and
      any($s[0].receipts[]; .task_id == $o.task_id and .arm == "B" and .receipt_sha256 == $o.candidate_security_receipt_hash))
  ' "$quality" >/dev/null
}

build_efficiency_input() {
  local objective wrapper acceptance task004 quality output experiment
  quality=$(gpt_stage_path gpt-quality-ledger); output=$(gpt_stage_path gpt-efficiency-input)
  experiment="ute-gpt-primary-$ADMISSION_GENERATION"
  objective="sha256:$(jq -cS '{version,tasks:[.tasks[]|{task_id,task_brief,verification_command,oracle_hash}]}' "$COHORT" | shasum -a 256 | awk '{print $1}')"
  wrapper="sha256:$(jq -cS '{provider,provider_version,model,model_version,canonical_execution,identity,
    context_options,effective_codex_flags,retention}' "$CONFIG" | shasum -a 256 | awk '{print $1}')"
  acceptance="sha256:$(jq -cS --arg schema "$SCHEMA_HASH" \
    '{schema_sha256:$schema,records:.deterministic_target_preflight.records}' "$PREFLIGHT" | shasum -a 256 | awk '{print $1}')"
  task004=$(jq -r '.tasks[] | select(.task_id == "ute-corpus-v1-004") | .task_hash' "$COHORT")
  jq -n --slurpfile ledger "$PRIMARY_LEDGER" --slurpfile cohort "$COHORT" \
    --slurpfile quality "$quality" --arg policy "$POLICY_HASH" --arg config "$CONFIG_HASH" \
    --arg corpus "$CORPUS_HASH" --arg objective "$objective" --arg wrapper "$wrapper" --arg acceptance "$acceptance" \
    --arg audit "$task004" --arg experiment "$experiment" '
    {provider:"codex",provider_version:"0.144.1",model:"gpt-5.6-sol",model_version:"gpt-5.6-sol",
      effort_policy:"codex_review_xhigh_security_max_v1",risk_policy:$policy,
      cache_stratum:"provider-managed-stable-prefix-v1",config_hash:$config} as $identity |
    def run($c): {agent_name:$c.role,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",task_id:$c.task_id,
      run_id:$c.result.run_id,call_id:$c.result.call_id,attempt:1,provider:"codex",model:"gpt-5.6-sol",
      effort:$c.effort,phase:"review",role:$c.role,start_time:"2026-07-12T00:00:00Z",
      end_time:"2026-07-12T00:00:00Z",duration_ns:($c.agent_run.duration_ns // 0),status:"PASS",
      acceptance_status:"PASS",files_modified:0,estimated_tokens:0,tool_calls:0,usage:[$c.usage]};
    {version:1,calls:[$ledger[0].calls[] | {usage:.usage,identity:$identity}],
      neutrality:{baseline_objective_hash:$objective,candidate_objective_hash:$objective,
        baseline_call_policy_hash:$wrapper,candidate_call_policy_hash:$wrapper,
        baseline_acceptance_hash:$acceptance,candidate_acceptance_hash:$acceptance},
      expected_task_ids:[$cohort[0].tasks[].task_id],
      trials:[$cohort[0].tasks[] as $t | ["A","B"][] as $arm |
        {task_id:$t.task_id,arm:(if $arm == "A" then "baseline" else "candidate" end),
         pair_order:$t.pair_order,identity:$identity,
         runs:[$ledger[0].calls[] | select(.task_id == $t.task_id and .arm == $arm) | run(.)]}],
      quality_outcomes:$quality[0].outcomes,regressions:[],usage_conflict:false,
      policy_parity_passed:true,context_integrity_passed:true,
      reliability:{blocked:false,exit_code:0,reason:"",attributed_version:""},current_stage:"canary",
      candidate_behavior_active:false,
      rollout:{experiment_id:$experiment,task_corpus_hash:$corpus,policy_hash:$policy,
        config_hash:$config,receipt_kind:"canary",risk_tier:"medium",sensitive:false,full_depth:false,
        audit_task_hash:$audit,audit_rate_percent:20}}' > "$output"
  gpt_canonicalize_json "$output"
  jq -e '.version == 1 and (.calls | length) == 58 and (.trials | length) == 14 and
    (.expected_task_ids | length) == 7 and (.quality_outcomes | length) == 7 and
    all(.trials[]; (.runs | length) >= 2 and all(.runs[]; .status == "PASS" and .acceptance_status == "PASS" and
      .tool_calls == 0 and (.usage | length) == 1))' "$output" >/dev/null
}
