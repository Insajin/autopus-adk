#!/usr/bin/env bash

DIAG_LIB_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
# shellcheck source=ute_codex_canary_lib.sh
source "$DIAG_LIB_DIR/ute_codex_canary_lib.sh"
# shellcheck source=ute_gpt_diag_canary_schedule.sh
source "$DIAG_LIB_DIR/ute_gpt_diag_canary_schedule.sh"

diag_die() { printf 'ute-gpt-diag-canary: %s\n' "$1" >&2; exit 1; }

diag_authorization_id() {
  printf '%s\0%s\0%s\0%s' "$DIAG_POLICY_HASH" "$DIAG_CONFIG_HASH" "$DIAG_SCHEMA_HASH" "$DIAG_COHORT_HASH" |
    shasum -a 256 | awk '{print "sha256:" $1}'
}

validate_diag_static() {
  local dir=$1 base corpus cohort policy config schema preflight schedule auth
  corpus="$dir/corpus-v1.json"; cohort="$dir/gpt-diagnostic-cohort-v1.json"
  policy="$dir/gpt-diagnostic-policy-v1.json"; config="$dir/gpt-diagnostic-config-v1.json"
  schema="$dir/gpt-diagnostic-verdict-schema-v1.json"; preflight="$dir/gpt-diagnostic-preflight-v1.json"
  verify_corpus "$dir" || return 1
  for base in cohort policy config verdict-schema preflight; do
    verify_named_sidecar "$dir/gpt-diagnostic-$base-v1.json" "$dir/gpt-diagnostic-$base-v1.sha256" || return 1
    jq empty "$dir/gpt-diagnostic-$base-v1.json" >/dev/null || return 1
  done
  jq -e --slurpfile corpus "$corpus" '
    .task_count == 1 and .pair_order == "AB" and .diagnostic_only == true and .promotion_eligible == false and
    .task.task_id == "ute-corpus-v1-006" and .task.risk_tier == "high" and .task.path_count == 7 and
    .task.base_commit == $corpus[0].tasks[5].base_commit and .task.target_commit == $corpus[0].tasks[5].target_commit and
    .task.task_hash == $corpus[0].tasks[5].task_hash and .task.oracle_hash == $corpus[0].tasks[5].oracle.hash and
    .task.verification_command == $corpus[0].tasks[5].verification_command and
    [.arms[].profile] == ["full5","full5"] and (.task.allowed_scope_hashes | length) == 7
  ' "$cohort" >/dev/null || return 1
  jq -e '
    .authorization == {provider:"codex",model:"gpt-5.6-sol",provider_call_cap:10,
      raw_token_cap:228000,concurrency:1,retries:0,single_use:true} and
    .schedule.total_calls == 10 and .schedule.xhigh_calls == 8 and .schedule.max_calls == 2 and
    .schedule.worst_case_raw_tokens == 228000 and .valid_diagnostic_fail_continues == true and
    .diagnostic_only == true and .promotion_eligible == false
  ' "$policy" >/dev/null || return 1
  jq -e '
    .provider == "codex" and .provider_version == "0.144.1" and .model == "gpt-5.6-sol" and
    .canonical_execution.argv == ["auto","agent","run","ute-corpus-v1-006"] and
    .canonical_execution.direct_codex_forbidden == true and .context.evidence_mode == true and
    .context.diagnostic_mode == true and .context.strict_verdict == true and
    .context.zero_tool_calls_required == true and .context.codex.sandbox == "read-only" and
    .context.codex.ephemeral == true and .context.codex.ignore_user_config == true and
    .context.codex.ignore_rules == true and .context.codex.skip_git_repo_check == true and
    .context.codex.zero_tool_mode == true and
    .context.codex.output_schema == "gpt-diagnostic-verdict-schema-v1.json"
  ' "$config" >/dev/null || return 1
  jq -e '
    .type == "object" and .additionalProperties == false and
    .required == ["verdict","finding_count","finding_codes","finding_scope_hashes"] and
    .properties.finding_count.maximum == 3 and .properties.finding_codes.maxItems == 3 and
    .properties.finding_scope_hashes.maxItems == 3 and .properties.finding_scope_hashes.uniqueItems == true
  ' "$schema" >/dev/null || return 1
  DIAG_POLICY_HASH="sha256:$(sha256_file "$policy")"; DIAG_CONFIG_HASH="sha256:$(sha256_file "$config")"
  DIAG_SCHEMA_HASH="sha256:$(sha256_file "$schema")"; DIAG_COHORT_HASH="sha256:$(sha256_file "$cohort")"
  auth=$(diag_authorization_id)
  jq -e --arg p "$DIAG_POLICY_HASH" --arg c "$DIAG_CONFIG_HASH" --arg s "$DIAG_SCHEMA_HASH" \
    --arg h "$DIAG_COHORT_HASH" --arg auth "$auth" '
      .provider_receipt == false and .observed_spend.provider_calls_made == 0 and
      .frozen_artifacts == {policy_sha256:$p,config_sha256:$c,verdict_schema_sha256:$s,cohort_sha256:$h} and
      .authorization_identity.sha256 == $auth and .schedule.total_calls == 10 and
      .schedule.worst_case_raw_tokens == 228000 and .decision.diagnostic_only == true and
      .decision.promotion_eligible == false
  ' "$preflight" >/dev/null || return 1
  schedule=$(mktemp "${TMPDIR:-/tmp}/ute-diag-schedule.XXXXXX")
  emit_diag_schedule > "$schedule"; validate_diag_schedule "$schedule"; base=$?; rm -f "$schedule"; return "$base"
}

validate_diag_repo() {
  local repo=$1 corpus=$2 cohort=$3 base target patch expected scopes
  validate_repo_snapshot "$repo" "$corpus" || return 1
  base=$(jq -r '.task.base_commit' "$cohort"); target=$(jq -r '.task.target_commit' "$cohort")
  expected=$(jq -r '.task.oracle_hash' "$cohort")
  patch="sha256:$(snapshot_git "$repo" diff --no-ext-diff --no-textconv --binary "$base" "$target" | shasum -a 256 | awk '{print $1}')"
  [[ "$patch" == "$expected" ]] || return 1
  scopes=$(snapshot_git "$repo" diff --no-ext-diff --no-textconv --name-only "$base" "$target" |
    while IFS= read -r path; do printf 'sha256:%s\n' "$(printf '%s' "$path" | shasum -a 256 | awk '{print $1}')"; done)
  [[ "$scopes" == "$(jq -r '.task.allowed_scope_hashes[]' "$cohort")" ]]
}
