#!/usr/bin/env bash

gpt_die() { printf 'ute-gpt-evidence: %s\n' "$1" >&2; exit 1; }

gpt_sha_file() { printf 'sha256:%s\n' "$(shasum -a 256 "$1" | awk '{print $1}')"; }

gpt_write_sidecar() {
  local file=$1
  printf '%s  %s\n' "$(shasum -a 256 "$file" | awk '{print $1}')" "$(basename "$file")" > "${file%.json}.sha256"
  chmod 600 "${file%.json}.sha256"
}

gpt_canonicalize_json() {
  local file=$1 tmp="${1}.canonical.$$"
  jq -S '.' "$file" > "$tmp" || { rm -f "$tmp"; return 1; }
  chmod 600 "$tmp"
  mv "$tmp" "$file"
  gpt_write_sidecar "$file"
}

gpt_require_tools() {
  local tool
  for tool in jq shasum git awk sed find mktemp realpath python3 sort; do
    command -v "$tool" >/dev/null 2>&1 || gpt_die "required tool unavailable: $tool"
  done
}

gpt_stage_path() { printf '%s/%s-%s.json\n' "$STAGE" "$1" "$ADMISSION_GENERATION"; }

gpt_validate_v2_static() {
  local dir=$1 base identity policy config schema schedule snapshot prior corpus cohort transport
  verify_corpus "$dir" || return 1
  for base in gpt-canary-cohort-v1 gpt-canary-preflight-v1 gpt-codex-policy-v2 gpt-codex-config-v2 gpt-verdict-schema-v2 \
    gpt-canary-schedule-v2 gpt-snapshot-manifest-v2 gpt-prior-evidence-manifest-v2 \
    gpt-full-evaluation-identity-v2 gpt-canary-preflight-v2; do
    verify_named_sidecar "$dir/$base.json" "$dir/$base.sha256" || return 1
    jq -e 'type == "object"' "$dir/$base.json" >/dev/null || return 1
  done
  identity="sha256:$(sha256_file "$dir/gpt-full-evaluation-identity-v2.json")"
  policy="sha256:$(sha256_file "$dir/gpt-codex-policy-v2.json")"
  config="sha256:$(sha256_file "$dir/gpt-codex-config-v2.json")"
  schema="sha256:$(sha256_file "$dir/gpt-verdict-schema-v2.json")"
  schedule="sha256:$(sha256_file "$dir/gpt-canary-schedule-v2.json")"
  snapshot="sha256:$(sha256_file "$dir/gpt-snapshot-manifest-v2.json")"
  prior="sha256:$(sha256_file "$dir/gpt-prior-evidence-manifest-v2.json")"
  corpus="sha256:$(sha256_file "$dir/corpus-v1.json")"
  cohort="sha256:$(sha256_file "$dir/gpt-canary-cohort-v1.json")"
  transport="sha256:$(sha256_file "$dir/gpt-transport-smoke-terminal-outcome-v8.json")"
  jq -e --arg policy "$policy" --arg config "$config" --arg schema "$schema" --arg schedule "$schedule" \
    --arg snapshot "$snapshot" --arg prior "$prior" --arg corpus "$corpus" --arg cohort "$cohort" \
    --arg transport "$transport" --slurpfile policy_doc "$dir/gpt-codex-policy-v2.json" '
    .version == 2 and .admission_generation == "v2" and
    .evidence_kind == "gpt_codex_full_evaluation_identity" and
    .frozen_artifacts.policy_sha256 == $policy and .frozen_artifacts.config_sha256 == $config and
    .frozen_artifacts.verdict_schema_sha256 == $schema and .frozen_artifacts.schedule_sha256 == $schedule and
    .frozen_artifacts.snapshot_manifest_sha256 == $snapshot and
    .frozen_artifacts.prior_evidence_manifest_sha256 == $prior and
    .frozen_artifacts.corpus_sha256 == $corpus and .frozen_artifacts.cohort_sha256 == $cohort and
    .frozen_artifacts.transport_terminal_sha256 == $transport and
    .runtime.full_chain_harness_sha256 == $policy_doc[0].execution_runtime.full_chain_harness_sha256 and
    .runtime.full_chain_algorithm == "sha256-named-member-manifest-v1" and
    .runtime.full_chain_member_count == 17 and
    .authorization_envelope == {provider_call_cap:64,raw_token_cap:1500000,primary_calls:58,
      rollback_calls:5,total_calls:63,planned_worst_case_raw_tokens:1446000,concurrency:1,retries:0}
  ' "$dir/gpt-full-evaluation-identity-v2.json" >/dev/null || return 1
  jq -e --arg identity "$identity" --slurpfile c "$dir/corpus-v1.json" \
    --slurpfile p "$dir/gpt-canary-preflight-v1.json" '.version == 2 and .admission_generation == "v2" and
    .authorization_identity.sha256 == $identity and .observed_spend.provider_calls_made == 0 and
    .observed_spend.raw_total_tokens == 0 and (.deterministic_target_preflight as $d |
    $d==$p[0].deterministic_target_preflight and $d.status=="PASS" and $d.task_count==12 and
    $d.passed_tasks==12 and $d.failed_tasks==0 and ($d.records|length)==12 and
    ([$d.records[].task_id]|unique|length)==12 and all($d.records[]; . as $r | any($c[0].tasks[];
    .task_id==$r.task_id and .base_commit==$r.base_commit and .target_commit==$r.target_commit and
    .verification_command==$r.verification_command and .oracle.hash==$r.expected_patch_hash and
    .oracle.hash==$r.observed_patch_hash and $r.verification_exit_code==0 and $r.status=="PASS")))' \
    "$dir/gpt-canary-preflight-v2.json" >/dev/null || return 1
  gpt_validate_full_chain "$dir"
}

gpt_validate_static() {
  if [[ "$ADMISSION_GENERATION" == v1 ]]; then validate_static_evidence "$1"; else gpt_validate_v2_static "$1"; fi
}

gpt_absolute_file() {
  local file=$1 dir
  [[ "$file" == /* && -f "$file" ]] || return 1
  dir=$(cd "$(dirname "$file")" && pwd -P) || return 1
  printf '%s/%s\n' "$dir" "$(basename "$file")"
}

gpt_prepare_paths() {
  [[ ! -L "$PRIMARY_LEDGER" ]] || gpt_die "primary ledger symlink is forbidden"
  PRIMARY_LEDGER=$(gpt_absolute_file "$PRIMARY_LEDGER") || gpt_die "primary ledger must be an absolute file"
  AUTO=$(gpt_absolute_file "$AUTO") || gpt_die "auto must be an absolute file"
  [[ -x "$AUTO" && ! -L "$AUTO" ]] || gpt_die "auto must be an immutable executable file"
  [[ "$REPO" == /* && -d "$REPO" ]] || gpt_die "repo must be an absolute directory"
  REPO=$(cd "$REPO" && pwd -P)
  [[ "$OUTPUT" == /* && -d "$OUTPUT" ]] || gpt_die "output must be an existing absolute directory"
  OUTPUT=$(cd "$OUTPUT" && pwd -P)
  if [[ "$ADMISSION_GENERATION" == v2 && "$OUTPUT" == "$STATIC_DIR" ]]; then
    gpt_output_targets_absent || gpt_die "canonical output targets already exist"
  else
    [[ -z "$(find "$OUTPUT" -mindepth 1 -print -quit)" ]] || gpt_die "output must be empty"
    [[ "$OUTPUT" != "$REPO" && "$OUTPUT" != "$STATIC_DIR" && "$OUTPUT" != "$SOURCE_ROOT" ]] || gpt_die "output overlaps immutable input"
    case "$OUTPUT/" in "$REPO/"*|"$SOURCE_ROOT/"*) gpt_die "output must be disjoint from repo and source" ;; esac
  fi
  case "$REPO/" in "$OUTPUT/"*) gpt_die "output must be disjoint from repo" ;; esac
}

gpt_output_targets_absent() {
  local stem
  for stem in gpt-security-receipts gpt-quality-ledger gpt-efficiency-input gpt-efficiency-result \
    gpt-rollout-audit-input gpt-rollout-audit-result gpt-rollout-high-input gpt-rollout-high-result \
    gpt-rollout-critical-input gpt-rollout-critical-result gpt-rollout-receipts gpt-rollback-input \
    gpt-rollback-result gpt-applied-rollback gpt-primary-evaluation-summary; do
    [[ ! -e "$OUTPUT/$stem-v2.json" && ! -e "$OUTPUT/$stem-v2.sha256" ]] || return 1
  done
}

gpt_prepare_identity() {
  local bare
  bare=$(snapshot_git "$REPO" rev-parse --is-bare-repository 2>/dev/null) || gpt_die "repo is not a git object repository"
  [[ "$bare" == true ]] || gpt_die "repo must be an immutable bare repository"
  AUTO_SHA=$(gpt_sha_file "$AUTO")
  AUTO_VERSION=
  CORPUS="$STATIC_DIR/corpus-v1.json"
  COHORT="$STATIC_DIR/gpt-canary-cohort-v1.json"
  POLICY="$STATIC_DIR/gpt-codex-policy-$ADMISSION_GENERATION.json"
  CONFIG="$STATIC_DIR/gpt-codex-config-$ADMISSION_GENERATION.json"
  PREFLIGHT="$STATIC_DIR/gpt-canary-preflight-$ADMISSION_GENERATION.json"
  SCHEMA="$STATIC_DIR/gpt-verdict-schema-$ADMISSION_GENERATION.json"
  CORPUS_HASH=$(gpt_sha_file "$CORPUS")
  COHORT_HASH=$(gpt_sha_file "$COHORT")
  POLICY_HASH=$(gpt_sha_file "$POLICY")
  CONFIG_HASH=$(gpt_sha_file "$CONFIG")
  SCHEMA_HASH=$(gpt_sha_file "$SCHEMA")
  PREFLIGHT_HASH=$(gpt_sha_file "$PREFLIGHT")
  AUTHORIZATION_IDENTITY_SHA256=
  if [[ "$ADMISSION_GENERATION" == v2 ]]; then
    IDENTITY="$STATIC_DIR/gpt-full-evaluation-identity-v2.json"
    AUTHORIZATION_IDENTITY_SHA256=$(gpt_sha_file "$IDENTITY")
    jq -e --arg auto "$AUTO_SHA" '.runtime.auto_executable_sha256 == $auto' "$IDENTITY" >/dev/null ||
      gpt_die "auto does not match frozen full-evaluation identity"
  fi
  AUTO_VERSION=$("$AUTO" version --short 2>/dev/null | tr -d '\r\n') || gpt_die "auto version unavailable"
  [[ "$AUTO_VERSION" =~ ^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$ ]] || gpt_die "invalid auto version"
  if [[ "$ADMISSION_GENERATION" == v2 ]]; then
    jq -e --arg version "$AUTO_VERSION" '.runtime.auto_version == $version' "$IDENTITY" >/dev/null ||
      gpt_die "auto version does not match frozen full-evaluation identity"
  fi
  PRIMARY_LEDGER_HASH=$(gpt_sha_file "$PRIMARY_LEDGER")
  STATE=__isolated_builder_state__
  ROLLBACK_RECEIPT=
  ROLLBACK_RECEIPT_HASH=
  PRIMARY_OBSERVED_RAW=0
}

gpt_validate_primary_ledger() {
  verify_named_sidecar "$PRIMARY_LEDGER" "${PRIMARY_LEDGER%.json}.sha256" || return 1
  validate_primary_ledger_for_rollback "$PRIMARY_LEDGER" || return 1
  jq -e --arg auto "$AUTO_SHA" --arg version "$AUTO_VERSION" --arg policy "$POLICY_HASH" --arg config "$CONFIG_HASH" \
    --arg corpus "$CORPUS_HASH" --arg cohort "$COHORT_HASH" --arg schema "$SCHEMA_HASH" \
    --arg generation "$ADMISSION_GENERATION" --arg auth "$AUTHORIZATION_IDENTITY_SHA256" '
    def exact_keys($allowed): ((keys - $allowed) | length) == 0 and (($allowed - keys) | length) == 0;
    (["version","spec_id","evidence_kind","mode","completed","evaluation_eligible",
      "promotion_eligible","circuit_breaker","failure_code","attempted_calls","successful_calls","planned_calls",
      "observed_calls","planned_worst_case_raw_tokens","observed_raw_total_tokens",
      "combined_primary_and_replay_observed_raw_tokens","authorization","identity",
      "applied_rollback_receipt_sha256","primary_ledger_sha256","calls","privacy"] +
      (if $generation == "v2" then ["admission_generation","authorization_identity_sha256","reservation_sha256"] else [] end)) as $root_keys |
    exact_keys($root_keys) and
    (if $generation == "v2" then
      .admission_generation == "v2" and .authorization_identity_sha256 == $auth and
      (.reservation_sha256 | type == "string" and test("^sha256:[0-9a-f]{64}$"))
     else (has("admission_generation") or has("authorization_identity_sha256") | not) end) and
    .version == 1 and .spec_id == "SPEC-ADK-ULTRA-EFFICIENCY-001" and
    .evidence_kind == "gpt_codex_primary_call_ledger" and .mode == "primary" and .completed == true and
    .evaluation_eligible == true and .promotion_eligible == false and .failure_code == null and
    .attempted_calls == 58 and .successful_calls == 58 and .planned_calls == 58 and .observed_calls == 58 and
    .planned_worst_case_raw_tokens == 1332000 and
    (.observed_raw_total_tokens | type == "number" and floor == . and . > 0) and
    .observed_raw_total_tokens <= 1332000 and .combined_primary_and_replay_observed_raw_tokens == .observed_raw_total_tokens and
    .circuit_breaker == "CLOSED" and
    (if $generation == "v2" then
      .authorization == {provider_call_cap:64,raw_token_cap:1500000,concurrency:1,retries:0,
        primary_calls_reserved:58,rollback_calls_reserved:5,total_calls_reserved:63,
        planned_worst_case_raw_tokens:1446000}
     else .authorization == {provider_call_cap:64,raw_token_cap:1500000,concurrency:1,retries:0,
        planned_total_calls:63,planned_worst_case_raw_tokens:1446000,replay_reserve_tokens:114000} end) and
    (.identity | exact_keys(["provider","model","provider_version","model_version","effort_policy","cache_stratum",
      "corpus_sha256","cohort_sha256","policy_sha256","config_sha256","verdict_schema_sha256",
      "auto_executable_sha256","auto_version","codex_cli_version","codex_cli_version_receipt_sha256"])) and
    .identity.provider == "codex" and .identity.model == "gpt-5.6-sol" and
    .identity.provider_version == "0.144.1" and .identity.model_version == "gpt-5.6-sol" and
    .identity.effort_policy == "codex_review_xhigh_security_max_v1" and
    .identity.cache_stratum == "provider-managed-stable-prefix-v1" and
    .identity.corpus_sha256 == $corpus and .identity.cohort_sha256 == $cohort and
    .identity.policy_sha256 == $policy and .identity.config_sha256 == $config and
    .identity.verdict_schema_sha256 == $schema and .identity.codex_cli_version == "0.144.1" and
    .identity.auto_executable_sha256 == $auto and .identity.auto_version == $version and
    ([.calls[].result.call_id] | unique | length) == 58 and
    ([.calls[].result.run_id] | unique | length) == 58 and
    ([.calls[].sequence] == [range(1;59)]) and
    ([.calls[].result.raw_total_tokens] | add) == .observed_raw_total_tokens and
    .privacy == {raw_prompt_retained:false,raw_patch_retained:false,raw_response_retained:false,
      provider_stdout_stderr_retained:false,isolated_telemetry_retained:false,absolute_paths_retained:false} and
    all(.calls[]; . as $c |
      exact_keys(["sequence","task_id","arm","order","profile","role","role_ordinal","effort",
        "raw_token_budget","result","agent_run","usage"]) and
      ($c.result | exact_keys(["status","verdict","finding_count","output_sha256","usage_status",
        "unique_model_call_count","raw_total_tokens","tool_calls","duration_ms","cost_usd","run_id","call_id"])) and
      ($c.agent_run | exact_keys(["status","acceptance_status","tool_calls","duration_ns","files_modified","estimated_tokens"])) and
      $c.result.status == "success" and $c.result.verdict == "PASS" and $c.result.finding_count == 0 and
      $c.result.tool_calls == 0 and $c.agent_run.tool_calls == 0 and $c.result.usage_status == "actual" and
      $c.result.unique_model_call_count == 1 and $c.result.raw_total_tokens <= $c.raw_token_budget and
      ($c.result.raw_total_tokens | type == "number" and floor == . and . > 0) and
      ($c.result.output_sha256 | test("^[0-9a-f]{64}$")) and
      ($c.result.call_id | test("^c[0-9a-f]{24}$")) and ($c.result.run_id | test("^r[0-9a-f]{24}$"))) and
    all(.calls[]; (.usage | exact_keys(["version","provider","model","effort","provider_version","model_version",
      "risk_policy","cache_stratum","config_hash","phase","role","usage_status","usage_source","source_schema",
      "input_tokens_total","uncached_input_tokens","cached_input_tokens","cache_creation_input_tokens",
      "cache_read_input_tokens","output_tokens_total","reasoning_tokens","reasoning_relation","tool_tokens",
      "tool_relation","raw_total_tokens","actual_cost_usd","run_id","call_id","task_id","attempt"])) and
      .usage.provider == "codex" and .usage.model == "gpt-5.6-sol" and
      .usage.provider_version == "0.144.1" and .usage.model_version == "gpt-5.6-sol" and
      .usage.risk_policy == $policy and .usage.config_hash == $config and
      .usage.usage_status == "actual" and .usage.usage_source == "provider" and
      .usage.source_schema == "codex.exec-json.turn.completed.v1" and
      .usage.call_id == .result.call_id and .usage.run_id == .result.run_id and
      .usage.task_id == .task_id and .usage.role == .role and .usage.effort == .effort and
      (.usage.input_tokens_total | type) == "number" and (.usage.output_tokens_total | type) == "number" and
      (.usage.raw_total_tokens | type) == "number" and
      .usage.raw_total_tokens == (.usage.input_tokens_total + .usage.output_tokens_total) and
      .usage.raw_total_tokens == .result.raw_total_tokens)
  ' "$PRIMARY_LEDGER" >/dev/null || return 1
  PRIMARY_OBSERVED_RAW=$(jq -er '.observed_raw_total_tokens |
    select(type == "number" and floor == . and . > 0 and . <= 1332000) | tostring' "$PRIMARY_LEDGER") || return 1
  [[ "$PRIMARY_OBSERVED_RAW" =~ ^[1-9][0-9]*$ ]] || return 1
  gpt_scan_json "$PRIMARY_LEDGER" || return 1
  [[ "$ADMISSION_GENERATION" != v2 ]] || gpt_validate_opaque_ids "$PRIMARY_LEDGER" primary
}

gpt_validate_oracles() {
  local rows="$WORK/oracles.jsonl" modes_tsv="$WORK/modes.tsv" modes_json="$WORK/modes.json"
  local task base target expected paths verify risk order actual count
  : > "$rows"
  while IFS=$'\t' read -r task base target expected paths verify risk order; do
    actual="sha256:$(snapshot_git "$REPO" diff --no-ext-diff --no-textconv --binary "$base" "$target" | shasum -a 256 | awk '{print $1}')"
    [[ "$actual" == "$expected" ]] || return 1
    snapshot_git "$REPO" diff --check "$base" "$target" >/dev/null || return 1
    snapshot_git "$REPO" diff-tree --no-commit-id -r --raw "$base" "$target" | awk -F '\t' '
      BEGIN {OFS="\t"} /^:/ {split($1,a," "); print $2,substr(a[1],2),a[2],a[5]}' > "$modes_tsv"
    count=$(awk 'END {print NR+0}' "$modes_tsv")
    [[ "$count" == "$paths" ]] || return 1
    awk -F '\t' 'NF != 4 {exit 1} $2 !~ /^(000000|100644|100755)$/ {exit 1}
      $3 !~ /^(000000|100644|100755)$/ {exit 1} $4 !~ /^(A|M|D)$/ {exit 1}' "$modes_tsv" || return 1
    jq -Rn '[inputs | split("\t") | {path:.[0],old_mode:.[1],new_mode:.[2],status:.[3]}]' < "$modes_tsv" > "$modes_json"
    jq -e --arg task "$task" --arg expected "$expected" --arg verify "$verify" '
      any(.deterministic_target_preflight.records[]; .task_id == $task and
        .expected_patch_hash == $expected and .observed_patch_hash == $expected and
        .verification_command == $verify and .verification_exit_code == 0 and .status == "PASS")
    ' "$PREFLIGHT" >/dev/null || return 1
    jq -cn --arg task "$task" --arg expected "$expected" --argjson paths "$paths" --arg verify "$verify" \
      --arg risk "$risk" --arg order "$order" --slurpfile modes "$modes_json" \
      '{task_id:$task,expected_patch_hash:$expected,observed_patch_hash:$expected,path_count:$paths,
        path_modes:$modes[0],verification_command:$verify,verification_exit_code:0,
        verification_status:"PASS",risk_tier:$risk,pair_order:$order}' >> "$rows"
  done < <(jq -r '.tasks[] | [.task_id,.base_commit,.target_commit,.oracle_hash,.path_count,
    .verification_command,.risk_tier,.pair_order] | @tsv' "$COHORT")
  [[ "$(awk 'END {print NR+0}' "$rows")" == 7 ]] || return 1
  jq -s '.' "$rows" > "$ORACLES"
}

gpt_scan_json() {
  local file=$1
  jq -e '[.. | objects | keys[] | select(. == "prompt" or . == "description" or . == "patch" or
    . == "raw_output" or . == "stdout" or . == "stderr" or . == "session_id" or
    . == "environment" or . == "cwd")] | length == 0' "$file" >/dev/null || return 1
  jq -e 'all(.. | strings; (startswith("/") | not) and
    (contains("FAKE-RAW-PROVIDER-BODY") | not) and (contains("UTE-RAW-PROMPT-") | not))' "$file" >/dev/null
}

gpt_publish_stage() {
  local file
  [[ "$(gpt_sha_file "$AUTO")" == "$AUTO_SHA" ]] || return 1
  [[ "$(gpt_sha_file "$PRIMARY_LEDGER")" == "$PRIMARY_LEDGER_HASH" ]] || return 1
  [[ "$(gpt_sha_file "$CORPUS")" == "$CORPUS_HASH" && "$(gpt_sha_file "$COHORT")" == "$COHORT_HASH" ]] || return 1
  [[ "$(gpt_sha_file "$POLICY")" == "$POLICY_HASH" && "$(gpt_sha_file "$CONFIG")" == "$CONFIG_HASH" ]] || return 1
  [[ "$(gpt_sha_file "$PREFLIGHT")" == "$PREFLIGHT_HASH" && "$(gpt_sha_file "$SCHEMA")" == "$SCHEMA_HASH" ]] || return 1
  verify_named_sidecar "$PRIMARY_LEDGER" "${PRIMARY_LEDGER%.json}.sha256" || return 1
  gpt_validate_static "$STATIC_DIR" || return 1
  while IFS= read -r file; do
    gpt_scan_json "$file" || return 1
    verify_named_sidecar "$file" "${file%.json}.sha256" || return 1
  done < <(find "$STAGE" -maxdepth 1 -type f -name '*.json' | sort)
  [[ "$(find "$STAGE" -maxdepth 1 -type f | wc -l | tr -d ' ')" == 30 ]] || return 1
  [[ "$ADMISSION_GENERATION" != v2 || "$(gpt_sha_file "$IDENTITY")" == "$AUTHORIZATION_IDENTITY_SHA256" ]] || return 1
  [[ "$OUTPUT" != "$STATIC_DIR" ]] || gpt_output_targets_absent || return 1
  mv "$STAGE"/* "$OUTPUT"/
}
