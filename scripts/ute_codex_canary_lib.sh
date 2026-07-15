#!/usr/bin/env bash

UTE_LIB_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
# shellcheck source=ute_codex_canary_schedule.sh
source "$UTE_LIB_DIR/ute_codex_canary_schedule.sh"

die() { printf 'ute-codex-canary: %s\n' "$1" >&2; exit 1; }
sha256_file() { shasum -a 256 "$1" | awk '{print $1}'; }
snapshot_git() {
  local repo=$1; shift
  GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_NO_REPLACE_OBJECTS=1 git -C "$repo" "$@"
}

require_tools() {
  local tool
  for tool in jq yq shasum git awk sed find mktemp; do
    command -v "$tool" >/dev/null 2>&1 || die "required tool unavailable"
  done
}

verify_named_sidecar() {
  local file=$1 sidecar=$2 line expected name
  [[ -f "$file" && -f "$sidecar" ]] || return 1
  line=$(cat "$sidecar")
  [[ "$line" =~ ^([0-9a-f]{64})[[:space:]][[:space:]]([^[:space:]]+)$ ]] || return 1
  expected=${BASH_REMATCH[1]}; name=${BASH_REMATCH[2]}
  [[ "$name" == "$(basename "$file")" && "$expected" == "$(sha256_file "$file")" ]]
}

verify_corpus() {
  local dir=$1 file="$1/corpus-v1.json" sidecar="$1/corpus-v1.sha256"
  local expected= a
  [[ -f "$file" && -f "$sidecar" ]] || return 1
  expected=$(tr -d '[:space:]' < "$sidecar")
  a=$(sha256_file "$file")
  [[ "$expected" == "sha256:$a" && "$expected" == "sha256:a3454f01b734d3f72060bc9b93972032b908f88940960e7f7b0953ab7356958a" ]]
}

audit_bucket() {
  local task_hash=$1 policy_hash=$2 hex byte i bucket=0
  hex=$(printf '%s\0%s' "$task_hash" "$policy_hash" | shasum -a 256 | awk '{print substr($1,1,16)}')
  for i in 0 2 4 6 8 10 12 14; do
    byte=$((16#${hex:i:2}))
    bucket=$(((bucket * 256 + byte) % 100))
  done
  printf '%s\n' "$bucket"
}

validate_json_contracts() {
  local dir=$1 corpus="$1/corpus-v1.json" cohort="$1/gpt-canary-cohort-v1.json"
  local policy="$1/gpt-codex-policy-v1.json" config="$1/gpt-codex-config-v1.json"
  local schema="$1/gpt-verdict-schema-v1.json" preflight="$1/gpt-canary-preflight-v1.json"
  jq -e '
    .version == 1 and .task_count == 12 and (.tasks | length) == 12 and
    ([.tasks[].task_id] | unique | length) == 12
  ' "$corpus" >/dev/null || return 1
  jq -e '
    .version == 1 and .selection.selected_task_count == 7 and .selection.excluded_task_count == 5 and
    [.tasks[].task_id] == ["ute-corpus-v1-001","ute-corpus-v1-004","ute-corpus-v1-005",
      "ute-corpus-v1-011","ute-corpus-v1-012","ute-corpus-v1-006","ute-corpus-v1-009"] and
    [.tasks[].pair_order] == ["AB","BA","AB","BA","AB","BA","AB"] and
    [.excluded_tasks[].task_id] == ["ute-corpus-v1-002","ute-corpus-v1-003","ute-corpus-v1-007",
      "ute-corpus-v1-008","ute-corpus-v1-010"]
  ' "$cohort" >/dev/null || return 1
  jq -e '
    .authorization == {provider:"codex",provider_call_cap:64,raw_token_cap:1500000,concurrency:1,retries:0} and
    .hard_envelope.primary_calls == 58 and .hard_envelope.primary_xhigh_calls == 44 and
    .hard_envelope.primary_max_calls == 14 and .hard_envelope.primary_worst_case_raw_tokens == 1332000 and
    .hard_envelope.rollback_replay_calls == 5 and .hard_envelope.rollback_worst_case_raw_tokens == 114000 and
    .hard_envelope.planned_total_calls == 63 and .hard_envelope.planned_worst_case_raw_tokens == 1446000 and
    .audit.selected_task_id == "ute-corpus-v1-005" and .audit.rate_percent == 20
  ' "$policy" >/dev/null || return 1
  jq -e '
    .provider == "codex" and .provider_version == "0.144.1" and .model == "gpt-5.6-sol" and
    .model_version == "gpt-5.6-sol" and .risk_policy_source == "gpt-codex-policy-v1.sha256" and
    .effort_policy == "codex_review_xhigh_security_max_v1" and
    .canonical_execution.argv == ["auto","agent","run","<corpus-task-id>"] and
    .canonical_execution.direct_codex_forbidden == true and
    .context_options == {evidence_mode:true,strict_verdict:true,zero_tool_calls_required:true,codex:{
      sandbox:"read-only",ephemeral:true,ignore_user_config:true,ignore_rules:true,skip_git_repo_check:true,
      output_schema:"gpt-verdict-schema.json",zero_tool_mode:true}} and
    .identity.cache_stratum == "provider-managed-stable-prefix-v1"
  ' "$config" >/dev/null || return 1
  jq -e '
    .type == "object" and .additionalProperties == false and
    .required == ["verdict","finding_count"] and
    .properties.verdict.enum == ["PASS","FAIL"] and .properties.finding_count.minimum == 0 and
    .properties.finding_count.maximum == 1000
  ' "$schema" >/dev/null || return 1
  jq -e '
    .provider_receipt == false and .observed_spend.provider_calls_made == 0 and
    .observed_spend.raw_total_tokens == 0 and .deterministic_target_preflight.status == "PASS" and
    .deterministic_target_preflight.task_count == 12 and .deterministic_target_preflight.passed_tasks == 12 and
    .decision.evaluation_eligible == false and .decision.promotion_eligible == false
  ' "$preflight" >/dev/null || return 1
}

validate_corpus_links() {
  local dir=$1 corpus="$1/corpus-v1.json" cohort="$1/gpt-canary-cohort-v1.json"
  local preflight="$1/gpt-canary-preflight-v1.json"
  jq -e --slurpfile c "$corpus" '
    all((.tasks + .excluded_tasks)[]; . as $x |
      any($c[0].tasks[]; .task_id == $x.task_id and .base_commit == $x.base_commit and
        .target_commit == $x.target_commit and .risk_tier == $x.risk_tier and
        .verification_command == $x.verification_command and .task_hash == $x.task_hash and
        .oracle.hash == $x.oracle_hash))
  ' "$cohort" >/dev/null || return 1
  jq -e --slurpfile c "$corpus" '
    (.deterministic_target_preflight.records | length) == 12 and
    all(.deterministic_target_preflight.records[]; . as $x |
      any($c[0].tasks[]; .task_id == $x.task_id and .base_commit == $x.base_commit and
        .target_commit == $x.target_commit and .verification_command == $x.verification_command and
        .oracle.hash == $x.expected_patch_hash and $x.observed_patch_hash == $x.expected_patch_hash and
        $x.verification_exit_code == 0 and $x.status == "PASS"))
  ' "$preflight" >/dev/null || return 1
}

validate_task_hashes() {
  local corpus=$1 version id category base target brief risk shape verify expected actual
  while IFS=$'\t' read -r version id category base target brief risk shape verify expected; do
    actual=$(printf '%s\0%s\0%s\0%s\0%s\0%s\0%s\0%s\0%s' \
      "$version" "$id" "$category" "$base" "$target" "$brief" "$risk" "$shape" "$verify" | shasum -a 256 | awk '{print $1}')
    [[ "$expected" == "sha256:$actual" ]] || return 1
  done < <(jq -r '. as $r | .tasks[] | [$r.version,.task_id,.category,.base_commit,.target_commit,
    .task_brief,.risk_tier,.execution_shape,.verification_command,.task_hash] | @tsv' "$corpus")
}

validate_audit_table() {
  local dir=$1 policy_hash rate task hash want got selected=0 selected_id=
  policy_hash="sha256:$(sha256_file "$1/gpt-codex-policy-v1.json")"
  rate=$(jq -r '.audit_selection.rate_percent' "$1/gpt-canary-preflight-v1.json")
  while IFS=$'\t' read -r task hash want; do
    got=$(audit_bucket "$hash" "$policy_hash")
    [[ "$got" == "$want" ]] || return 1
    if (( got < rate )); then selected=$((selected + 1)); selected_id=$task; fi
  done < <(jq -r '.audit_selection.buckets[] | [.task_id,.task_hash,.bucket] | @tsv' "$1/gpt-canary-preflight-v1.json")
  [[ "$selected" == 1 && "$selected_id" == ute-corpus-v1-005 ]]
}

validate_frozen_hashes() {
  local dir=$1
  jq -e --arg corpus "sha256:$(sha256_file "$dir/corpus-v1.json")" \
    --arg cohort "sha256:$(sha256_file "$dir/gpt-canary-cohort-v1.json")" \
    --arg policy "sha256:$(sha256_file "$dir/gpt-codex-policy-v1.json")" \
    --arg config "sha256:$(sha256_file "$dir/gpt-codex-config-v1.json")" \
    --arg schema "sha256:$(sha256_file "$dir/gpt-verdict-schema-v1.json")" '
      .frozen_artifacts == {corpus_sha256:$corpus,cohort_sha256:$cohort,policy_sha256:$policy,
        config_sha256:$config,verdict_schema_sha256:$schema} and
      .audit_selection.policy_hash == $policy
    ' "$dir/gpt-canary-preflight-v1.json" >/dev/null
}

validate_static_evidence() {
  local dir=$1 base schedule
  require_tools
  verify_corpus "$dir" || return 1
  for base in gpt-canary-cohort-v1 gpt-codex-policy-v1 gpt-codex-config-v1 gpt-verdict-schema-v1 gpt-canary-preflight-v1; do
    verify_named_sidecar "$dir/$base.json" "$dir/$base.sha256" || return 1
    jq empty "$dir/$base.json" >/dev/null || return 1
  done
  validate_json_contracts "$dir" || return 1
  validate_corpus_links "$dir" || return 1
  validate_task_hashes "$dir/corpus-v1.json" || return 1
  validate_audit_table "$dir" || return 1
  validate_frozen_hashes "$dir" || return 1
  schedule=$(mktemp "${TMPDIR:-/tmp}/ute-schedule.XXXXXX")
  emit_primary_schedule > "$schedule"
  validate_schedule_arithmetic "$schedule" || { rm -f "$schedule"; return 1; }
  rm -f "$schedule"
}

validate_repo_snapshot() {
  local repo=$1 corpus=$2 bare head base target parent patch_hash paths
  [[ -d "$repo" ]] || return 1
  snapshot_git "$repo" rev-parse --git-dir >/dev/null 2>&1 || return 1
  bare=$(snapshot_git "$repo" rev-parse --is-bare-repository) || return 1
  if [[ "$bare" != true ]]; then
    head=$(snapshot_git "$repo" symbolic-ref -q HEAD || true)
    [[ -z "$head" && -z "$(snapshot_git "$repo" status --porcelain=v1 --untracked-files=all)" ]] || return 1
  fi
  while IFS=$'\t' read -r base target patch_hash paths; do
    [[ "$(snapshot_git "$repo" cat-file -t "$base" 2>/dev/null)" == commit ]] || return 1
    [[ "$(snapshot_git "$repo" cat-file -t "$target" 2>/dev/null)" == commit ]] || return 1
    parent=$(snapshot_git "$repo" rev-parse "$target^1") || return 1
    [[ "$parent" == "$base" ]] || return 1
    [[ "sha256:$(snapshot_git "$repo" diff --no-ext-diff --no-textconv --binary "$base" "$target" | shasum -a 256 | awk '{print $1}')" == "$patch_hash" ]] || return 1
    [[ "$(snapshot_git "$repo" diff --no-ext-diff --no-textconv --name-only "$base" "$target" | awk 'NF {n++} END {print n+0}')" == "$paths" ]] || return 1
  done < <(jq -r '.tasks[] | [.base_commit,.target_commit,.oracle.hash,.path_count] | @tsv' "$corpus")
}
