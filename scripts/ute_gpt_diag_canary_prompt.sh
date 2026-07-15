#!/usr/bin/env bash

diag_opaque_id() {
  local kind=$1 seq=$2 digest
  digest=$(printf '%s\0%s\0%s\0ute-corpus-v1-006' "$DIAG_AUTHORIZATION_ID" "$kind" "$seq" | shasum -a 256 | awk '{print $1}')
  printf '%s%s\n' "${kind:0:1}" "${digest:0:24}"
}

materialize_diag_patch_scope() {
  local patch=$1 scope=$2 base target expected actual
  base=$(jq -r '.task.base_commit' "$COHORT_FILE"); target=$(jq -r '.task.target_commit' "$COHORT_FILE")
  expected=$(jq -r '.task.oracle_hash' "$COHORT_FILE")
  snapshot_git "$REPO" diff --no-ext-diff --no-textconv --binary "$base" "$target" > "$patch" || return 1
  actual="sha256:$(sha256_file "$patch")"; [[ "$actual" == "$expected" ]] || return 1
  snapshot_git "$REPO" diff --no-ext-diff --no-textconv --name-only "$base" "$target" |
    while IFS= read -r path; do
      printf '%s => sha256:%s\n' "$path" "$(printf '%s' "$path" | shasum -a 256 | awk '{print $1}')"
    done > "$scope"
  chmod 600 "$patch" "$scope"
}

diag_role_instruction() {
  case "$1" in
    reviewer) printf '%s\n' 'Inspect correctness, regression risk, task fidelity, and test adequacy.' ;;
    security-auditor) printf '%s\n' 'Inspect security, privacy, trust-boundary, and fail-closed regressions.' ;;
    review-consolidator) printf '%s\n' 'Consolidate bounded prior diagnostic receipts against the exact patch.' ;;
    *) return 1 ;;
  esac
}

materialize_diag_prompt() {
  local role=$1 ordinal=$2 marker=$3 patch=$4 scope=$5 summaries=$6 output=$7
  local instruction prior=none
  instruction=$(diag_role_instruction "$role") || return 1
  if [[ "$role" == review-consolidator ]]; then
    prior=$(awk 'NF {printf "%s%s", sep, $0; sep="; "}' "$summaries")
    [[ -n "$prior" && ${#prior} -le 2048 ]] || return 1
  fi
  {
    printf 'Receipt marker: %s\n' "$marker"
    printf '%s\n' 'Task: Preserve merged review findings and avoid false trim notices.'
    printf '%s\n' 'Risk: high. The exact patch and deterministic verification receipt are authoritative.'
    printf '%s\n' 'Deterministic receipt: patch_sha256=sha256:55f5f87f5521d0d595758cede60d331692a3d338f3ce99cc7d851d9deb083a2b; verification="go test ./internal/cli ./pkg/spec"; exit_code=0.'
    printf 'Role: %s; ordinal=%s. %s\n' "$role" "$ordinal" "$instruction"
    [[ "$role" != review-consolidator ]] || printf 'Bounded prior receipts: %s\n' "$prior"
    printf '%s\n' 'A finding is a material patch-introduced blocker against task fidelity, deterministic evidence, or a security boundary.'
    printf '%s\n' 'Ignore style preferences, optional hardening, pre-existing issues, and out-of-scope concerns.'
    printf '%s\n' 'Use at most three enum finding_codes and at most three hashes from the supplied scope map. Never emit finding text or paths.'
    printf '%s\n' 'PASS requires finding_count=0 and empty arrays. FAIL requires 1..3 codes and at least one supplied scope hash.'
    printf '%s\n' 'Do not call tools, execute commands, modify files, or rely on external context.'
    printf '%s\n' 'SCOPE MAP (temporary; output hashes only)'; cat "$scope"
    printf '%s\n' 'BEGIN EXACT TARGET PATCH'; cat "$patch"; printf '%s\n' 'END EXACT TARGET PATCH'
  } > "$output"
  chmod 600 "$output"
}

materialize_diag_context() {
  local role=$1 effort=$2 budget=$3 run=$4 call=$5 prompt=$6 run_dir=$7
  jq -n --rawfile description "$prompt" --arg role "$role" --arg effort "$effort" --arg run "$run" \
    --arg call "$call" --arg policy "$DIAG_POLICY_HASH" --arg config "$DIAG_CONFIG_HASH" --argjson budget "$budget" \
    '{task_id:"ute-corpus-v1-006",description:$description,provider:"codex",model:"gpt-5.6-sol",effort:$effort,
      spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",run_id:$run,call_id:$call,attempt:1,phase:"review",role:$role,
      provider_version:"0.144.1",model_version:"gpt-5.6-sol",risk_policy:$policy,
      cache_stratum:"provider-managed-stable-prefix-v1",config_hash:$config,evidence_mode:true,diagnostic_mode:true,
      strict_verdict:true,zero_tool_calls_required:true,codex:{sandbox:"read-only",ephemeral:true,
      ignore_user_config:true,ignore_rules:true,skip_git_repo_check:true,output_schema:"gpt-diagnostic-verdict-schema-v1.json",
      zero_tool_mode:true,raw_token_budget:$budget}}' > "$run_dir/context.yaml"
  cp "$SCHEMA_FILE" "$run_dir/gpt-diagnostic-verdict-schema-v1.json"
  chmod 600 "$run_dir/context.yaml" "$run_dir/gpt-diagnostic-verdict-schema-v1.json"
}
