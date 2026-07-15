#!/usr/bin/env bash
v2_fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
v2_sha() { printf 'sha256:%s\n' "$(shasum -a 256 "$1" | awk '{print $1}')"; }
v2_sidecar() {
  local file=$1
  printf '%s  %s\n' "${2:-$(shasum -a 256 "$file" | awk '{print $1}')}" "$(basename "$file")" > "${file%.json}.sha256"
  chmod 600 "$file" "${file%.json}.sha256"
}
v2_assert_sidecar() {
  local file=$1 expected name
  [[ -f "$file" && -f "${file%.json}.sha256" ]] || v2_fail "missing artifact: $(basename "$file")"
  read -r expected name < "${file%.json}.sha256"
  [[ "$name" == "$(basename "$file")" && "$expected" == "$(shasum -a 256 "$file" | awk '{print $1}')" ]] || \
    v2_fail "invalid sidecar: $(basename "$file")"
}
v2_assert_empty() {
  [[ -d "$1" && -z "$(find "$1" -mindepth 1 -print -quit)" ]] || v2_fail "$2"
}
v2_make_chain_install() {
  local target=$1 static=$2 base evidence="$1/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
  local members=(ute_codex_canary_v2.sh ute_codex_canary_v2_static.sh ute_codex_canary_v2_authorization.sh
    ute_codex_canary_v2_ledger.sh ute_codex_canary_v2_freeze.sh ute_codex_canary_v2_authorize.sh
    ute_codex_canary_lib.sh ute_codex_canary_prompt.sh ute_codex_canary_receipt.sh ute_codex_canary_exec.sh
    ute_codex_canary_schedule.sh ute_gpt_evidence.sh ute_gpt_evidence_lib.sh ute_gpt_evidence_build.sh
    ute_gpt_evidence_rollout.sh ute_gpt_full_evaluation_finalize.sh ute_gpt_full_evaluation_finalize_lib.sh)
  mkdir -p "$target/scripts" "$evidence"
  for base in "${members[@]}"; do cp "$SCRIPT_DIR/$base" "$target/scripts/"; done
  for base in corpus-v1 gpt-canary-cohort-v1 gpt-transport-smoke-terminal-outcome-v8 \
    gpt-codex-policy-v2 gpt-codex-config-v2 gpt-verdict-schema-v2 gpt-canary-schedule-v2 \
    gpt-snapshot-manifest-v2 gpt-prior-evidence-manifest-v2 gpt-full-evaluation-identity-v2 \
    gpt-canary-preflight-v2; do cp "$static/$base.json" "$static/$base.sha256" "$evidence/"; done
}
v2_rewrite_opaque_ids() {
  local file=$1 config=$2 mode=$3 rows="${1}.ids.$$" ids="${1}.ids.json.$$" tmp="${1}.tmp.$$"
  local seq task call_digest run_digest
  : > "$rows"
  while IFS=$'\t' read -r seq task; do
    call_digest=$(printf '%s\0%s\0%s\0%s\0%s' "$config" call "$mode" "$seq" "$task" |
      shasum -a 256 | awk '{print $1}')
    run_digest=$(printf '%s\0%s\0%s\0%s\0%s' "$config" run "$mode" "$seq" "$task" |
      shasum -a 256 | awk '{print $1}')
    printf '%s\t%s\t%s\n' "$seq" "c${call_digest:0:24}" "r${run_digest:0:24}" >> "$rows"
  done < <(jq -r '.calls[] | [.sequence,.task_id] | @tsv' "$file")
  jq -Rn '[inputs | split("\t") | {sequence:(.[0]|tonumber),call_id:.[1],run_id:.[2]}]' "$rows" > "$ids"
  jq --slurpfile ids "$ids" '.calls |= map(. as $call |
    ($ids[0][] | select(.sequence == $call.sequence)) as $id |
    .result.call_id=$id.call_id | .usage.call_id=$id.call_id |
    .result.run_id=$id.run_id | .usage.run_id=$id.run_id)' "$file" > "$tmp"
  mv "$tmp" "$file"; rm -f "$rows" "$ids"
}
v2_make_primary() {
  local source=$1 target=$2 static=$3 reservation=$4 policy config schema identity
  policy=$(v2_sha "$static/gpt-codex-policy-v2.json")
  config=$(v2_sha "$static/gpt-codex-config-v2.json")
  schema=$(v2_sha "$static/gpt-verdict-schema-v2.json")
  identity=$(v2_sha "$static/gpt-full-evaluation-identity-v2.json")
  jq --arg policy "$policy" --arg config "$config" --arg schema "$schema" --arg identity "$identity" \
    --arg reservation "$reservation" '
    . + {admission_generation:"v2",authorization_identity_sha256:$identity,reservation_sha256:$reservation} |
    .authorization={provider_call_cap:64,raw_token_cap:1500000,concurrency:1,retries:0,
      primary_calls_reserved:58,rollback_calls_reserved:5,total_calls_reserved:63,
      planned_worst_case_raw_tokens:1446000} |
    .identity.policy_sha256 = $policy | .identity.config_sha256 = $config |
    .identity.verdict_schema_sha256 = $schema |
    .calls |= map(.usage.risk_policy = $policy | .usage.config_hash = $config)
  ' "$source" > "$target"
  v2_rewrite_opaque_ids "$target" "$config" primary
  v2_sidecar "$target"
}
v2_make_rollback() {
  local primary=$1 applied=$2 target=$3 count=${4:-5} completed=${5:-true}
  local primary_hash applied_hash dir summary reservation nested nested_hash summary_hash
  primary_hash=$(v2_sha "$primary"); applied_hash=$(v2_sha "$applied"); dir=$(dirname "$target")
  summary="$dir/gpt-primary-evaluation-summary-v2.json"; reservation="$dir/gpt-full-evaluation-reservation-v2.json"
  nested="$dir/gpt-rollback-reservation-v2.json"; summary_hash=$(v2_sha "$summary")
  jq -n --arg auth "$(jq -r '.authorization_identity_sha256' "$primary")" \
    --arg reservation "$(v2_sha "$reservation")" --arg primary "$primary_hash" --arg applied "$applied_hash" \
    --arg summary "$summary_hash" '{version:1,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",
    evidence_kind:"gpt_rollback_replay_reservation",admission_generation:"v2",
    authorization_identity_sha256:$auth,primary_reservation_sha256:$reservation,
    primary_ledger_sha256:$primary,applied_rollback_receipt_sha256:$applied,evaluator_summary_sha256:$summary,
    calls_reserved:5,raw_tokens_reserved:114000,concurrency:1,retries:0,state:"CONSUMED_ON_RESERVATION"}' > "$nested"
  v2_sidecar "$nested"; nested_hash=$(v2_sha "$nested")
  jq -n --slurpfile p "$primary" --arg primary "$primary_hash" --arg applied "$applied_hash" \
    --arg nested "$nested_hash" --arg summary "$summary_hash" --argjson count "$count" --argjson completed "$completed" '
    ($p[0]) as $p |
    ([$p.calls[] | select(.task_id == "ute-corpus-v1-001" and .arm == "A")]) as $base |
    ([range(0;$count) as $i | ($base[$i % 5] |
      .sequence = ($i + 1) | .arm = "R" | .order = "NA" | .profile = "full5" |
      .result.call_id = ("c" + ("2" * 23) + (($i + 1)|tostring)) |
      .usage.call_id = .result.call_id |
      .result.run_id = ("r" + ("3" * 23) + (($i + 1)|tostring)) |
      .usage.run_id = .result.run_id)]) as $calls |
    ($p | .evidence_kind = "applied_rollback_replay" | .mode = "rollback" |
      .completed = $completed | .evaluation_eligible = false | .promotion_eligible = false |
      .circuit_breaker = (if $completed and $count == 5 then "CLOSED" else "OPEN" end) |
      .failure_code = (if $completed and $count == 5 then null else "rollback_replay_incomplete" end) |
      .attempted_calls = $count | .successful_calls = $count | .planned_calls = 5 |
      .observed_calls = $count | .planned_worst_case_raw_tokens = 114000 |
      .observed_raw_total_tokens = ([$calls[].result.raw_total_tokens] | add // 0) |
      .combined_primary_and_replay_observed_raw_tokens =
        ($p.observed_raw_total_tokens + ([$calls[].result.raw_total_tokens] | add // 0)) |
      .applied_rollback_receipt_sha256 = $applied | .primary_ledger_sha256 = $primary |
      .rollback_reservation_sha256 = $nested | .evaluator_summary_sha256 = $summary |
      .calls = $calls)
  ' > "$target"
  v2_rewrite_opaque_ids "$target" "$(jq -r '.identity.config_sha256' "$primary")" rollback
  v2_sidecar "$target"
}
v2_expect_evaluator_failure() {
  local harness=$1 ledger=$2 repo=$3 auto=$4 output=$5
  mkdir -p "$output"
  if "$harness" evaluate --admission-generation v2 --primary-ledger "$ledger" \
    --repo "$repo" --auto "$auto" --output "$output" >/dev/null 2>&1; then
    v2_fail "invalid v2 primary was accepted: $(basename "$ledger")"
  fi
  v2_assert_empty "$output" "failed evaluator published artifacts"
}
v2_assert_evaluation() {
  local output=$1 auth=$2 stem
  for stem in gpt-security-receipts gpt-quality-ledger gpt-efficiency-input \
    gpt-efficiency-result gpt-rollout-audit-input gpt-rollout-audit-result \
    gpt-rollout-high-input gpt-rollout-high-result gpt-rollout-critical-input \
    gpt-rollout-critical-result gpt-rollout-receipts gpt-rollback-input \
    gpt-rollback-result gpt-applied-rollback gpt-primary-evaluation-summary; do
    v2_assert_sidecar "$output/$stem-v2.json"
  done
  jq -e '.receipt_count == 14 and ([.receipts[].task_id] | unique | length) == 7' \
    "$output/gpt-security-receipts-v2.json" >/dev/null || v2_fail "v2 security completeness"
  jq -e '.row_count == 7 and (.outcomes | length) == 7' "$output/gpt-quality-ledger-v2.json" >/dev/null || \
    v2_fail "v2 quality completeness"
  jq -e '(.calls | length) == 58 and (.trials | length) == 14 and (.quality_outcomes | length) == 7' \
    "$output/gpt-efficiency-input-v2.json" >/dev/null || v2_fail "v2 evaluator input completeness"
  jq -e '.comparison.median_paired_raw_reduction_pct >= 25 and
    .promotion.rollout_decision == "ELIGIBLE_NEXT_CANARY"' "$output/gpt-efficiency-result-v2.json" >/dev/null || \
    v2_fail "v2 efficiency decision"
  jq -e --slurpfile c "$STATIC/gpt-canary-cohort-v1.json" --slurpfile hi "$output/gpt-rollout-high-input-v2.json" \
    --slurpfile hr "$output/gpt-rollout-high-result-v2.json" --slurpfile ci "$output/gpt-rollout-critical-input-v2.json" \
    --slurpfile cr "$output/gpt-rollout-critical-result-v2.json" '
    ($c[0].tasks[]|select(.task_id=="ute-corpus-v1-006")|.task_hash) as $h |
    ($c[0].tasks[]|select(.task_id=="ute-corpus-v1-009")|.task_hash) as $x |
    .audit.active_profile == "full_ultra" and .high.active_profile == "full_ultra" and
    .critical.active_profile == "full_ultra" and .audit.sentinel.task_id == "ute-corpus-v1-005" and
    .high.sentinel.task_id == "ute-corpus-v1-006" and .high.sentinel.audit_rate_percent == 0 and
    .high.sentinel.selected == false and .critical.sentinel.task_id == "ute-corpus-v1-009" and
    .critical.sentinel.audit_rate_percent == 0 and .critical.sentinel.selected == false and
    .high.experiment_identity == ("ute-gpt-rollout-high-v2:ute-corpus-v1-006:"+$h) and
    .critical.experiment_identity == ("ute-gpt-rollout-critical-v2:ute-corpus-v1-009:"+$x) and
    .high.experiment_identity == $hi[0].rollout.experiment_id and .high.experiment_identity_sha256 == $hr[0].rollout_receipt.experiment_id and
    .critical.experiment_identity == $ci[0].rollout.experiment_id and .critical.experiment_identity_sha256 == $cr[0].rollout_receipt.experiment_id' \
    "$output/gpt-rollout-receipts-v2.json" >/dev/null || \
    v2_fail "v2 full-depth sentinels"
  jq -e '.applied == true and .active_profile == "full_ultra" and .state_readback == "full_ultra"' \
    "$output/gpt-applied-rollback-v2.json" >/dev/null || v2_fail "v2 rollback readback"
  jq -e --arg auth "$auth" '.admission_generation == "v2" and .authorization_identity_sha256 == $auth and
    .primary_calls == 58 and .rollback_replay_status == "pending" and
    .provider_calls_made_by_builder == 0 and .promotion_eligible == false and
    .activation_eligible == false and .implemented == false and (.evaluator_artifacts|length) == 14 and
    .sentinels.high.task_id == "ute-corpus-v1-006" and .sentinels.critical.task_id == "ute-corpus-v1-009"' \
    "$output/gpt-primary-evaluation-summary-v2.json" >/dev/null || v2_fail "v2 pending summary"
}
v2_make_reservation() {
  local target=$1 static=$2 auto=$3 identity policy config runtime version
  identity=$(v2_sha "$static/gpt-full-evaluation-identity-v2.json")
  policy=$(v2_sha "$static/gpt-codex-policy-v2.json")
  config=$(v2_sha "$static/gpt-codex-config-v2.json")
  runtime=$(v2_sha "$auto")
  version=$("$auto" version --short)
  jq -n --arg identity "$identity" --arg policy "$policy" --arg config "$config" \
    --arg runtime "$runtime" --arg version "$version" '{version:1,
    spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",evidence_kind:"gpt_full_evaluation_reservation",
    admission_generation:"v2",authorization_identity_sha256:$identity,policy_sha256:$policy,
    config_sha256:$config,auto_executable_sha256:$runtime,auto_version:$version,
    primary_calls_reserved:58,rollback_calls_reserved:5,total_calls_reserved:63,
    planned_worst_case_raw_tokens:1446000,provider_call_cap:64,raw_token_cap:1500000,
    concurrency:1,retries:0,state:"CONSUMED_ON_RESERVATION"}' > "$target"
  v2_sidecar "$target"
}
v2_make_authorization() {
  local target=$1 static=$2 identity policy config preflight
  identity=$(v2_sha "$static/gpt-full-evaluation-identity-v2.json")
  policy=$(v2_sha "$static/gpt-codex-policy-v2.json")
  config=$(v2_sha "$static/gpt-codex-config-v2.json")
  preflight=$(v2_sha "$static/gpt-canary-preflight-v2.json")
  jq -n --arg identity "$identity" --arg policy "$policy" --arg config "$config" \
    --arg preflight "$preflight" '{version:2,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",
    evidence_kind:"gpt_codex_full_evaluation_authorization",
    decision:"EXPLICIT_FULL_EVALUATION_AUTHORIZATION_GRANTED",
    authorization_source:"user_exact_identity_confirmation",authorization_identity_sha256:$identity,
    policy_sha256:$policy,config_sha256:$config,preflight_sha256:$preflight,provider:"codex",
    model:"gpt-5.6-sol",provider_call_cap:64,raw_token_cap:1500000,primary_calls:58,
    rollback_calls:5,total_calls:63,planned_worst_case_raw_tokens:1446000,concurrency:1,
    retries:0,single_use:true,provider_execution_started:false,activation:false,promotion:false,
    implemented:false,authorized_at:"2026-07-15T13:00:00+09:00"}' > "$target"
  v2_sidecar "$target"
}
v2_copy_canonical() {
  local target=$1 static=$2 evaluated=${3:-} auto=${4:-}
  mkdir -p "$target"
  local base root
  if [[ "$target" == "$TMP_ROOT"/finalizer-*/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence ]]; then
    root=${target%/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence}; mkdir -p "$root/scripts"
    for base in ute_codex_canary_v2.sh ute_codex_canary_v2_static.sh ute_codex_canary_v2_authorization.sh \
      ute_codex_canary_v2_ledger.sh ute_codex_canary_v2_freeze.sh ute_codex_canary_v2_authorize.sh \
      ute_codex_canary_lib.sh ute_codex_canary_prompt.sh ute_codex_canary_receipt.sh ute_codex_canary_exec.sh \
      ute_codex_canary_schedule.sh ute_gpt_evidence.sh ute_gpt_evidence_lib.sh ute_gpt_evidence_build.sh \
      ute_gpt_evidence_rollout.sh ute_gpt_full_evaluation_finalize.sh ute_gpt_full_evaluation_finalize_lib.sh; do
      cp "$SCRIPT_DIR/$base" "$root/scripts/"
    done
    FINALIZER="$root/scripts/ute_gpt_full_evaluation_finalize.sh"
  fi
  for base in corpus-v1 gpt-canary-cohort-v1 gpt-codex-policy-v2 gpt-codex-config-v2 \
    gpt-verdict-schema-v2 gpt-canary-schedule-v2 gpt-snapshot-manifest-v2 \
    gpt-prior-evidence-manifest-v2 gpt-full-evaluation-identity-v2 \
    gpt-canary-preflight-v2; do
    cp "$static/$base.json" "$static/$base.sha256" "$target/"
  done
  v2_make_authorization "$target/gpt-full-evaluation-authorization-v2.json" "$static"
  [[ -z "$evaluated" ]] || cp "$evaluated"/*.json "$evaluated"/*.sha256 "$target/"
  [[ -z "$auto" ]] || v2_make_reservation "$target/gpt-full-evaluation-reservation-v2.json" "$static" "$auto"
}
v2_case_dir() {
  local dir="$TMP_ROOT/finalizer-$1/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
  mkdir -p "$dir"; printf '%s\n' "$dir"
}

v2_assert_failure_terminal() {
  local dir=$1 code=$2 reservation_mode=${3:-bound} terminal="$1/gpt-full-evaluation-terminal-outcome-v2.json"
  local closure="$1/gpt-full-evaluation-authorization-closure-v2.json"
  v2_assert_sidecar "$terminal"; v2_assert_sidecar "$closure"
  jq -e --arg code "$code" '.success == false and .terminal_state == "BLOCKED_NO_PROMOTION" and
    .failure_code == $code and .promotion_eligible == false and .activation_eligible == false and
    .implemented == false' "$terminal" >/dev/null || v2_fail "invalid failure terminal: $code"
  jq -e '.consumed == true and .reusable == false' "$closure" >/dev/null || v2_fail "failure closure reusable"
  if [[ "$reservation_mode" == bound && -f "$dir/gpt-full-evaluation-reservation-v2.json" ]]; then
    local reservation
    reservation=$(v2_sha "$dir/gpt-full-evaluation-reservation-v2.json")
    jq -e --arg reservation "$reservation" '.reservation_sha256 == $reservation' "$terminal" >/dev/null || \
      v2_fail "failure terminal reservation binding"
    jq -e --arg reservation "$reservation" '.reservation_sha256 == $reservation' "$closure" >/dev/null || \
      v2_fail "failure closure reservation binding"
  else
    jq -e '.reservation_sha256 == null' "$terminal" "$closure" >/dev/null ||
      v2_fail "invalid reservation was represented as a trusted hash"
  fi
}

v2_assert_success_terminal() {
  local dir=$1 auth=$2 terminal="$1/gpt-full-evaluation-terminal-outcome-v2.json"
  local closure="$1/gpt-full-evaluation-authorization-closure-v2.json" reservation replay nested summary
  reservation=$(v2_sha "$dir/gpt-full-evaluation-reservation-v2.json")
  replay=$(v2_sha "$dir/gpt-rollback-call-ledger-v2.json"); nested=$(v2_sha "$dir/gpt-rollback-reservation-v2.json")
  summary=$(v2_sha "$dir/gpt-primary-evaluation-summary-v2.json")
  v2_assert_sidecar "$terminal"; v2_assert_sidecar "$closure"
  jq -e --arg auth "$auth" --arg reservation "$reservation" --arg replay "$replay" --arg nested "$nested" --arg summary "$summary" \
    '.success == true and .terminal_state == "ELIGIBLE_NEXT_CANARY" and
    .authorization_identity_sha256 == $auth and .provider_calls == 63 and .retries == 0 and
    .reservation_sha256 == $reservation and
    .hashes.rollback_ledger_sha256 == $replay and .hashes.rollback_reservation_sha256 == $nested and
    .hashes.evaluator_summary_sha256 == $summary and
    .observed_raw_total_tokens <= 1500000 and .effective_profile == "full_ultra" and
    .promotion_eligible == false and .activation_eligible == false and .implemented == false and
    .provider_calls_made_by_finalizer == 0 and .privacy.raw_retained == false' "$terminal" >/dev/null || \
    v2_fail "invalid success terminal"
  jq -e --arg auth "$auth" --arg reservation "$reservation" '.authorization_identity_sha256 == $auth and
    .reservation_sha256 == $reservation and .consumed == true and
    .reusable == false and .provider_calls == 63 and .retries == 0 and .observed_raw_total_tokens <= 1500000' \
    "$closure" >/dev/null || v2_fail "invalid success closure"
}

v2_prepare_finalizer_case() {
  local dir=$1 static=$2 evaluated=$3 auto=$4 primary=$5
  v2_copy_canonical "$dir" "$static" "$evaluated" "$auto"
  cp "$primary" "${primary%.json}.sha256" "$dir/"
  V2_CASE_PRIMARY="$dir/$(basename "$primary")"
  V2_CASE_REPLAY="$dir/gpt-rollback-call-ledger-v2.json"
  v2_make_rollback "$V2_CASE_PRIMARY" "$dir/gpt-applied-rollback-v2.json" "$V2_CASE_REPLAY"
}

v2_expect_finalizer_failure() {
  local dir=$1 code=$2
  if HOME="$HOME_DIR" "$FINALIZER" finalize --evidence-dir "$dir" --primary-ledger "$V2_CASE_PRIMARY" \
    --rollback-ledger "$V2_CASE_REPLAY" --auto "$REAL_AUTO" >/dev/null 2>&1; then
    v2_fail "tampered closure reached success: $(basename "$dir")"
  fi
  v2_assert_failure_terminal "$dir" "$code"
}

v2_rewrite_json_and_sidecar() {
  local file=$1 filter=$2 tmp="${1}.tmp"
  jq "$filter" "$file" > "$tmp"; mv "$tmp" "$file"; v2_sidecar "$file"
}

v2_run_tamper_case() {
  local label=$1 relative=$2 filter=$3 code=${4:-quality_evidence_incomplete} dir
  dir=$(v2_case_dir "$label"); v2_prepare_finalizer_case "$dir" "$STATIC" "$EVALUATED" "$REAL_AUTO" "$PRIMARY"
  if [[ "$filter" == MISSING ]]; then rm "$dir/$relative" "${dir}/${relative%.json}.sha256"
  else v2_rewrite_json_and_sidecar "$dir/$relative" "$filter"; fi
  v2_expect_finalizer_failure "$dir" "$code"
}
