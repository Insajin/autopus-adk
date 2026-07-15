#!/usr/bin/env bash

make_opaque_id() {
  local kind=$1 mode=$2 seq=$3 task=$4 digest
  digest=$(printf '%s\0%s\0%s\0%s\0%s' "$CONFIG_HASH" "$kind" "$mode" "$seq" "$task" | shasum -a 256 | awk '{print $1}')
  printf '%s%s\n' "${kind:0:1}" "${digest:0:24}"
}

materialize_patch() {
  local task=$1 patch_file=$2 base target expected actual
  base=$(jq -r --arg task "$task" '.tasks[] | select(.task_id == $task) | .base_commit' "$COHORT_FILE")
  target=$(jq -r --arg task "$task" '.tasks[] | select(.task_id == $task) | .target_commit' "$COHORT_FILE")
  expected=$(jq -r --arg task "$task" '.tasks[] | select(.task_id == $task) | .oracle_hash' "$COHORT_FILE")
  snapshot_git "$REPO" diff --no-ext-diff --no-textconv --binary "$base" "$target" > "$patch_file" || return 1
  chmod 600 "$patch_file"
  actual="sha256:$(sha256_file "$patch_file")"
  [[ "$actual" == "$expected" ]]
}

role_instruction() {
  case "$1" in
    reviewer)
      printf '%s\n' 'Review the exact patch for correctness, regression risk, test adequacy, and task fidelity.'
      ;;
    security-auditor)
      printf '%s\n' 'Review the exact patch for security, privacy, trust-boundary, migration, and fail-closed regressions.'
      ;;
    review-consolidator)
      printf '%s\n' 'Consolidate the bounded prior receipts against the exact patch; do not infer details absent from the patch.'
      ;;
    *) return 1 ;;
  esac
}

materialize_prompt() {
  local task=$1 role=$2 ordinal=$3 marker=$4 patch_file=$5 summaries=$6 prompt_file=$7
  local brief risk verify oracle instruction bounded='none'
  brief=$(jq -r --arg task "$task" '.tasks[] | select(.task_id == $task) | .task_brief' "$COHORT_FILE")
  risk=$(jq -r --arg task "$task" '.tasks[] | select(.task_id == $task) | .risk_tier' "$COHORT_FILE")
  verify=$(jq -r --arg task "$task" '.tasks[] | select(.task_id == $task) | .verification_command' "$COHORT_FILE")
  oracle=$(jq -r --arg task "$task" '.tasks[] | select(.task_id == $task) | .oracle_hash' "$COHORT_FILE")
  instruction=$(role_instruction "$role") || return 1
  if [[ "$role" == review-consolidator ]]; then
    bounded=$(awk 'NF {printf "%s%s", sep, $0; sep="; "}' "$summaries")
    [[ -n "$bounded" && ${#bounded} -le 512 ]] || return 1
  fi
  {
    printf 'Receipt marker: %s\n' "$marker"
    printf 'Task identifier: %s\n' "$task"
    printf 'Task brief: %s\n' "$brief"
    printf 'Risk tier: %s\n' "$risk"
    printf 'Deterministic target receipt: command=%s; exit_code=0; expected_patch_sha256=%s; observed_patch_sha256=%s.\n' \
      "$verify" "$oracle" "$oracle"
    printf 'Review role: %s; ordinal=%s.\n' "$role" "$ordinal"
    printf 'Role instruction: %s\n' "$instruction"
    [[ "$role" != review-consolidator ]] || printf 'Bounded prior receipts: %s\n' "$bounded"
    printf '%s\n' 'A finding is a material patch-introduced blocker against the task brief, deterministic receipt, or security boundary.'
    printf '%s\n' 'Ignore style preferences, optional hardening, pre-existing issues, and out-of-scope concerns.'
    printf '%s\n' 'PASS if and only if no material finding exists; otherwise return FAIL with the material finding count.'
    printf '%s\n' 'The deterministic receipt is authoritative. Report any conflict as FAIL.'
    printf '%s\n' 'Do not call tools, execute commands, modify files, or rely on external context.'
    printf '%s\n' 'Return only one JSON object with exactly verdict and finding_count. PASS requires finding_count 0.'
    printf '%s\n' 'BEGIN EXACT TARGET PATCH'
    cat "$patch_file"
    printf '%s\n' 'END EXACT TARGET PATCH'
  } > "$prompt_file"
  chmod 600 "$prompt_file"
}

materialize_context() {
  local task=$1 role=$2 effort=$3 budget=$4 run_id=$5 call_id=$6 prompt=$7 run_dir=$8
  jq -n --rawfile description "$prompt" --arg task "$task" --arg role "$role" \
    --arg effort "$effort" --arg run "$run_id" --arg call "$call_id" --arg config "$CONFIG_HASH" --arg policy "$POLICY_HASH" \
    --argjson budget "$budget" '{task_id:$task,description:$description,provider:"codex",model:"gpt-5.6-sol",
      effort:$effort,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",run_id:$run,call_id:$call,attempt:1,
      phase:"review",role:$role,provider_version:"0.144.1",model_version:"gpt-5.6-sol",
      risk_policy:$policy,cache_stratum:"provider-managed-stable-prefix-v1",config_hash:$config,
      evidence_mode:true,strict_verdict:true,zero_tool_calls_required:true,codex:{sandbox:"read-only",
      ephemeral:true,ignore_user_config:true,ignore_rules:true,skip_git_repo_check:true,
      output_schema:"gpt-verdict-schema.json",zero_tool_mode:true,raw_token_budget:$budget}}' \
    > "$run_dir/context.yaml"
  cp "$SCHEMA_FILE" "$run_dir/gpt-verdict-schema.json"
  chmod 600 "$run_dir/context.yaml" "$run_dir/gpt-verdict-schema.json"
}
