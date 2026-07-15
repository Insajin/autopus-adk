#!/usr/bin/env bash

run_gpt_evaluator() {
  local input=$1 output=$2 stdout="$WORK/evaluator.stdout" stderr="$WORK/evaluator.stderr"
  [[ "$(gpt_sha_file "$AUTO")" == "$AUTO_SHA" ]] || return 1
  mkdir -p "$WORK/evaluator-home" "$WORK/evaluator-config"
  : > "$stdout"; : > "$stderr"; chmod 600 "$stdout" "$stderr"
  HOME="$WORK/evaluator-home" XDG_CONFIG_HOME="$WORK/evaluator-config" \
    "$AUTO" telemetry efficiency --evidence-json "$input" --format json > "$stdout" 2> "$stderr" || return 1
  jq -e 'type == "object" and .version == 1' "$stdout" >/dev/null || return 1
  jq -S '.' "$stdout" > "$output"
  chmod 600 "$output"
  gpt_write_sidecar "$output"
  [[ "$(gpt_sha_file "$AUTO")" == "$AUTO_SHA" ]]
}

gpt_expected_audit_bucket() {
  local input=$1 task policy
  task=$(jq -er '.rollout.audit_task_hash | select(type == "string" and test("^sha256:[0-9a-f]{64}$"))' "$input") || return 1
  policy=$(jq -er '.rollout.policy_hash | select(type == "string" and test("^sha256:[0-9a-f]{64}$"))' "$input") || return 1
  python3 - "$task" "$policy" <<'PY'
import hashlib, sys
digest = hashlib.sha256((sys.argv[1] + "\0" + sys.argv[2]).encode()).digest()
print(int.from_bytes(digest[:8], "big") % 100)
PY
}

validate_primary_evaluator_result() {
  local result=$1 input=$2 bucket
  bucket=$(gpt_expected_audit_bucket "$input") || return 1
  jq -e --argjson bucket "$bucket" '.measurement.measurement_gate == "PASS" and .measurement.neutrality_gate == "PASS" and
    .measurement.actual_usage_capture_pct == 100 and .comparison.expected_task_count == 7 and
    .comparison.paired_expected_task_count == 7 and .comparison.expected_corpus_complete == true and
    .comparison.paired_task_count == 7 and (.comparison.unpaired_task_ids | length) == 0 and
    (.comparison.excluded_tasks | length) == 0 and
    .comparison.median_paired_raw_reduction_pct >= 25 and .comparison.provisional_25_pct_target == "PASS" and
    .quality.expected_task_count == 7 and .quality.outcome_row_count == 7 and .quality.complete == true and
    .quality.consistent == true and .quality.objective_pass_count == 7 and .quality.security_pass_count == 7 and
    (.quality.candidate_failure_task_ids | length) == 0 and (.quality.derived_regressions | length) == 0 and
    .promotion.high_critical_regressions == 0 and .promotion.rollout_decision == "ELIGIBLE_NEXT_CANARY" and
    .rollout_receipt.decision == "CANARY" and .rollout_receipt.active_profile == "compact_ultra" and
    .rollout_receipt.full_depth == false and
    .rollout_receipt.audit_selection == {selected:false,bucket:$bucket,rate_percent:20,
      algorithm:"sha256_mod_100_v1"}' "$result" >/dev/null
}

build_rollout_evidence() {
  local primary_input primary_result audit_input audit_result high_input high_result critical_input critical_result
  local rollback_input rollback_result rollback_experiment audit_bucket
  primary_input=$(gpt_stage_path gpt-efficiency-input); primary_result=$(gpt_stage_path gpt-efficiency-result)
  audit_input=$(gpt_stage_path gpt-rollout-audit-input); audit_result=$(gpt_stage_path gpt-rollout-audit-result)
  high_input=$(gpt_stage_path gpt-rollout-high-input); high_result=$(gpt_stage_path gpt-rollout-high-result)
  critical_input=$(gpt_stage_path gpt-rollout-critical-input); critical_result=$(gpt_stage_path gpt-rollout-critical-result)
  rollback_input=$(gpt_stage_path gpt-rollback-input); rollback_result=$(gpt_stage_path gpt-rollback-result)
  rollback_experiment="ute-gpt-rollback-$ADMISSION_GENERATION"
  run_gpt_evaluator "$primary_input" "$primary_result" || return 1
  validate_primary_evaluator_result "$primary_result" "$primary_input" || return 1

  clone_rollout_input "$primary_input" "$audit_input" audit
  run_gpt_evaluator "$audit_input" "$audit_result" || return 1
  audit_bucket=$(gpt_expected_audit_bucket "$audit_input") || return 1
  jq -e --arg task "$(jq -r '.tasks[]|select(.task_id=="ute-corpus-v1-005")|.task_hash' "$COHORT")" '
    .rollout.risk_tier=="medium" and .rollout.audit_task_hash==$task and
    .rollout.audit_rate_percent==100 and .rollout.full_depth==true' "$audit_input" >/dev/null || return 1
  jq -e --argjson bucket "$audit_bucket" '.promotion.rollout_decision == "ELIGIBLE_NEXT_CANARY" and .rollout_receipt.decision == "AUDIT" and
    .rollout_receipt.active_profile == "full_ultra" and .rollout_receipt.full_depth == true and
    .rollout_receipt.risk_tier == "medium" and
    .rollout_receipt.audit_selection == {selected:true,bucket:$bucket,rate_percent:100,
      algorithm:"sha256_mod_100_v1"}' \
    "$audit_result" >/dev/null || return 1

  clone_rollout_input "$primary_input" "$high_input" high
  run_gpt_evaluator "$high_input" "$high_result" || return 1
  validate_full_risk_result "$high_result" "$high_input" high ute-corpus-v1-006 || return 1
  clone_rollout_input "$primary_input" "$critical_input" critical
  run_gpt_evaluator "$critical_input" "$critical_result" || return 1
  validate_full_risk_result "$critical_result" "$critical_input" critical ute-corpus-v1-009 || return 1
  build_rollout_aggregate

  jq --arg experiment "$rollback_experiment" '.policy_parity_passed = false | .candidate_behavior_active = true |
    .current_stage = "canary" | .rollout.experiment_id = $experiment' "$primary_input" > "$rollback_input"
  gpt_canonicalize_json "$rollback_input"
  run_gpt_evaluator "$rollback_input" "$rollback_result" || return 1
  jq -e '.promotion.rollout_decision == "ROLLBACK" and
    (.promotion.reason_codes | index("policy_parity_failed")) != null and
    .rollout_receipt.decision == "ROLLBACK" and .rollout_receipt.active_profile == "full_ultra" and
    .rollout_receipt.full_depth == true' "$rollback_result" >/dev/null
}

clone_rollout_input() {
  local source=$1 target=$2 kind=$3 task_id task_hash risk rate=0 experiment bind=false
  case "$kind" in
    # @AX:NOTE [AUTO] @AX:SPEC: SPEC-ADK-ULTRA-EFFICIENCY-001: Targeted task 005 proof selects its isolated audit receipt at 100%; the primary sampling gate remains 20%.
    audit) task_id=ute-corpus-v1-005; risk=medium; rate=100 ;;
    high) task_id=ute-corpus-v1-006; risk=high ;;
    critical) task_id=ute-corpus-v1-009; risk=critical ;;
    *) return 1 ;;
  esac
  task_hash=$(jq -r --arg task "$task_id" '.tasks[] | select(.task_id == $task) | .task_hash' "$COHORT")
  experiment="ute-gpt-rollout-$kind-$ADMISSION_GENERATION"
  if [[ "$ADMISSION_GENERATION" == v2 && "$kind" != audit ]]; then
    experiment="$experiment:$task_id:$task_hash"; bind=true
  fi
  if [[ "$ADMISSION_GENERATION" == v1 && "$kind" != audit ]]; then task_hash=; fi
  [[ -z "$task_hash" || "$task_hash" =~ ^sha256:[0-9a-f]{64}$ ]] || return 1
  jq --arg experiment "$experiment" --arg task "$task_hash" --arg risk "$risk" \
    --argjson rate "$rate" --argjson bind "$bind" '
    .rollout.experiment_id = $experiment |
    .rollout.risk_tier = $risk | .rollout.full_depth = true |
    if $bind then del(.rollout.audit_task_hash) | .rollout.audit_rate_percent = 0
    elif $task == "" then del(.rollout.audit_task_hash,.rollout.audit_rate_percent)
    else .rollout.audit_task_hash = $task | .rollout.audit_rate_percent = $rate end' "$source" > "$target"
  gpt_canonicalize_json "$target"
}

validate_full_risk_result() {
  local file=$1 input=$2 risk=$3 task_id=$4 task_hash experiment digest
  if [[ "$ADMISSION_GENERATION" == v1 ]]; then
    jq -e --arg risk "$risk" '.promotion.rollout_decision == "ELIGIBLE_NEXT_CANARY" and
      .rollout_receipt.risk_tier == $risk and .rollout_receipt.decision == "CANARY" and
      .rollout_receipt.active_profile == "full_ultra" and .rollout_receipt.full_depth == true and
      .rollout_receipt.selection_reason == "risk_requires_full_depth"' "$file" >/dev/null
    return
  fi
  task_hash=$(jq -er --arg task "$task_id" '.tasks[] | select(.task_id == $task) | .task_hash' "$COHORT") || return 1
  experiment="ute-gpt-rollout-$risk-v2:$task_id:$task_hash"
  digest="sha256:$(printf '%s' "$experiment" | shasum -a 256 | awk '{print $1}')"
  jq -e --arg risk "$risk" --arg experiment "$experiment" '.rollout.risk_tier == $risk and
    .rollout.experiment_id == $experiment and (.rollout|has("audit_task_hash")|not) and
    (.rollout|has("audit_rate_percent")) and .rollout.audit_rate_percent == 0 and .rollout.full_depth == true' \
    "$input" >/dev/null || return 1
  jq -e --arg risk "$risk" --arg digest "$digest" '.promotion.rollout_decision == "ELIGIBLE_NEXT_CANARY" and
    .rollout_receipt.risk_tier == $risk and .rollout_receipt.decision == "CANARY" and
    .rollout_receipt.active_profile == "full_ultra" and .rollout_receipt.full_depth == true and
    .rollout_receipt.selection_reason == "risk_requires_full_depth" and .rollout_receipt.experiment_id == $digest and
    .rollout_receipt.audit_selection == {selected:false,bucket:0,rate_percent:0,algorithm:""}' "$file" >/dev/null
}

build_rollout_aggregate() {
  local pi_file pr_file ai_file ar_file hi_file hr_file ci_file cr_file output
  pi_file=$(gpt_stage_path gpt-efficiency-input); pr_file=$(gpt_stage_path gpt-efficiency-result)
  ai_file=$(gpt_stage_path gpt-rollout-audit-input); ar_file=$(gpt_stage_path gpt-rollout-audit-result)
  hi_file=$(gpt_stage_path gpt-rollout-high-input); hr_file=$(gpt_stage_path gpt-rollout-high-result)
  ci_file=$(gpt_stage_path gpt-rollout-critical-input); cr_file=$(gpt_stage_path gpt-rollout-critical-result)
  output=$(gpt_stage_path gpt-rollout-receipts)
  if [[ "$ADMISSION_GENERATION" == v1 ]]; then
    jq -n --arg pi "$(gpt_sha_file "$pi_file")" --arg pr "$(gpt_sha_file "$pr_file")" \
      --arg ai "$(gpt_sha_file "$ai_file")" --arg ar "$(gpt_sha_file "$ar_file")" \
      --arg hi "$(gpt_sha_file "$hi_file")" --arg hr "$(gpt_sha_file "$hr_file")" \
      --arg ci "$(gpt_sha_file "$ci_file")" --arg cr "$(gpt_sha_file "$cr_file")" \
      --slurpfile p "$pr_file" --slurpfile a "$ar_file" --slurpfile h "$hr_file" --slurpfile c "$cr_file" '
      def outcome($x): {decision:$x.rollout_receipt.decision,active_profile:$x.rollout_receipt.active_profile,
        full_depth:$x.rollout_receipt.full_depth,risk_tier:$x.rollout_receipt.risk_tier,
        selection_reason:($x.rollout_receipt.selection_reason // null)};
      {version:1,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",evidence_kind:"gpt_rollout_receipts",
        primary:(outcome($p[0]) + {input_sha256:$pi,result_sha256:$pr}),
        audit:(outcome($a[0]) + {input_sha256:$ai,result_sha256:$ar}),
        high:(outcome($h[0]) + {input_sha256:$hi,result_sha256:$hr}),
        critical:(outcome($c[0]) + {input_sha256:$ci,result_sha256:$cr})}' > "$output"
    gpt_canonicalize_json "$output"
    return
  fi
  jq -n \
    --arg pi "$(gpt_sha_file "$pi_file")" --arg pr "$(gpt_sha_file "$pr_file")" \
    --arg ai "$(gpt_sha_file "$ai_file")" --arg ar "$(gpt_sha_file "$ar_file")" \
    --arg hi "$(gpt_sha_file "$hi_file")" --arg hr "$(gpt_sha_file "$hr_file")" \
    --arg ci "$(gpt_sha_file "$ci_file")" --arg cr "$(gpt_sha_file "$cr_file")" \
    --arg t004 "$(jq -r '.tasks[]|select(.task_id=="ute-corpus-v1-004")|.task_hash' "$COHORT")" \
    --arg t005 "$(jq -r '.tasks[]|select(.task_id=="ute-corpus-v1-005")|.task_hash' "$COHORT")" \
    --arg t006 "$(jq -r '.tasks[]|select(.task_id=="ute-corpus-v1-006")|.task_hash' "$COHORT")" \
    --arg t009 "$(jq -r '.tasks[]|select(.task_id=="ute-corpus-v1-009")|.task_hash' "$COHORT")" \
    --slurpfile pi_doc "$pi_file" --slurpfile ai_doc "$ai_file" --slurpfile hi_doc "$hi_file" --slurpfile ci_doc "$ci_file" \
    --slurpfile p "$pr_file" --slurpfile a "$ar_file" --slurpfile h "$hr_file" --slurpfile c "$cr_file" '
    def outcome($x;$i;$task;$hash): {decision:$x.rollout_receipt.decision,active_profile:$x.rollout_receipt.active_profile,
      full_depth:$x.rollout_receipt.full_depth,risk_tier:$x.rollout_receipt.risk_tier,
      selection_reason:($x.rollout_receipt.selection_reason // null),audit_selection:$x.rollout_receipt.audit_selection,
      experiment_identity:$i.rollout.experiment_id,experiment_identity_sha256:$x.rollout_receipt.experiment_id,
      sentinel:{task_id:$task,task_hash:$hash,
        audit_rate_percent:$i.rollout.audit_rate_percent,selected:$x.rollout_receipt.audit_selection.selected}};
    {version:1,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",evidence_kind:"gpt_rollout_receipts",
      primary:(outcome($p[0];$pi_doc[0];"ute-corpus-v1-004";$t004) + {input_sha256:$pi,result_sha256:$pr}),
      audit:(outcome($a[0];$ai_doc[0];"ute-corpus-v1-005";$t005) + {input_sha256:$ai,result_sha256:$ar}),
      high:(outcome($h[0];$hi_doc[0];"ute-corpus-v1-006";$t006) + {input_sha256:$hi,result_sha256:$hr}),
      critical:(outcome($c[0];$ci_doc[0];"ute-corpus-v1-009";$t009) + {input_sha256:$ci,result_sha256:$cr})}' \
    > "$output"
  gpt_canonicalize_json "$output"
}

gpt_fsync_path() {
  python3 - "$1" "$2" <<'PY'
import os, sys
fd = os.open(sys.argv[1], os.O_RDONLY)
try:
    os.fsync(fd)
finally:
    os.close(fd)
dfd = os.open(sys.argv[2], os.O_RDONLY)
try:
    os.fsync(dfd)
finally:
    os.close(dfd)
PY
}

atomic_binding_write() {
  local profile=$1 target=$2 tmp
  tmp="${target}.tmp.$$"
  jq -n --arg profile "$profile" --arg policy "$POLICY_HASH" --arg config "$CONFIG_HASH" \
    --arg primary "$PRIMARY_LEDGER_HASH" '{version:1,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",
      active_profile:$profile,policy_sha256:$policy,config_sha256:$config,primary_ledger_sha256:$primary}' \
    | jq -S '.' > "$tmp"
  chmod 600 "$tmp"
  gpt_fsync_path "$tmp" "$(dirname "$tmp")"
  mv "$tmp" "$target"
  gpt_fsync_path "$target" "$(dirname "$target")"
}

build_applied_rollback() {
  local state="$WORK/rollback-state" binding before after readback logical rollback_result output
  rollback_result=$(gpt_stage_path gpt-rollback-result); output=$(gpt_stage_path gpt-applied-rollback)
  mkdir -p "$state"; chmod 700 "$state"; binding="$state/policy-binding.json"
  atomic_binding_write compact_ultra "$binding" || return 1
  before=$(gpt_sha_file "$binding")
  atomic_binding_write full_ultra "$binding" || return 1
  after=$(gpt_sha_file "$binding")
  readback=$(jq -r '.active_profile' "$binding")
  [[ "$readback" == full_ultra && "$before" != "$after" ]] || return 1
  jq -e --arg policy "$POLICY_HASH" --arg config "$CONFIG_HASH" --arg primary "$PRIMARY_LEDGER_HASH" '
    .active_profile == "full_ultra" and .policy_sha256 == $policy and .config_sha256 == $config and
    .primary_ledger_sha256 == $primary' "$binding" >/dev/null || return 1
  logical=$(gpt_sha_file "$rollback_result")
  jq -n --arg policy "$POLICY_HASH" --arg config "$CONFIG_HASH" --arg primary "$PRIMARY_LEDGER_HASH" \
    --arg logical "$logical" --arg before "$before" --arg after "$after" \
    '{version:1,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",evidence_kind:"applied_policy_rollback",
      decision:"ROLLBACK",active_profile:"full_ultra",applied:true,atomic_replace:true,fsync_completed:true,
      state_readback:"full_ultra",policy_sha256:$policy,config_sha256:$config,primary_ledger_sha256:$primary,
      logical_rollback_result_sha256:$logical,before_binding_sha256:$before,after_binding_sha256:$after,
      state_readback_sha256:$after}' > "$output"
  gpt_canonicalize_json "$output"
}

build_primary_summary() {
  local result security quality rollout rollback output manifest rows stem file
  result=$(gpt_stage_path gpt-efficiency-result); security=$(gpt_stage_path gpt-security-receipts)
  quality=$(gpt_stage_path gpt-quality-ledger); rollout=$(gpt_stage_path gpt-rollout-receipts)
  rollback=$(gpt_stage_path gpt-applied-rollback); output=$(gpt_stage_path gpt-primary-evaluation-summary)
  manifest="$WORK/evaluator-artifacts.json"; rows="$WORK/evaluator-artifacts.jsonl"; : > "$rows"
  for stem in gpt-security-receipts gpt-quality-ledger gpt-efficiency-input gpt-efficiency-result \
    gpt-rollout-audit-input gpt-rollout-audit-result gpt-rollout-high-input gpt-rollout-high-result \
    gpt-rollout-critical-input gpt-rollout-critical-result gpt-rollout-receipts gpt-rollback-input \
    gpt-rollback-result gpt-applied-rollback; do
    file=$(gpt_stage_path "$stem")
    verify_named_sidecar "$file" "${file%.json}.sha256" || return 1
    jq -cn --arg key "$(basename "$file")" --arg value "$(gpt_sha_file "$file")" '{key:$key,value:$value}' >> "$rows"
  done
  jq -s 'from_entries' "$rows" > "$manifest" || return 1
  [[ "$(jq 'length' "$manifest")" == 14 ]] || return 1
  jq -n --slurpfile result "$result" --slurpfile rollout_doc "$rollout" --slurpfile artifacts "$manifest" \
    --arg primary "$PRIMARY_LEDGER_HASH" \
    --arg security "$(gpt_sha_file "$security")" --arg quality "$(gpt_sha_file "$quality")" \
    --arg rollout "$(gpt_sha_file "$rollout")" --arg rollback "$(gpt_sha_file "$rollback")" \
    --arg generation "$ADMISSION_GENERATION" --arg auth "$AUTHORIZATION_IDENTITY_SHA256" \
    --argjson raw "$PRIMARY_OBSERVED_RAW" '
    ({version:1,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",evidence_kind:"gpt_primary_evaluation_summary",
      primary_calls:58,observed_raw_total_tokens:$raw,
      median_paired_raw_reduction_pct:$result[0].comparison.median_paired_raw_reduction_pct,
      quality_tasks_passed:7,quality_tasks_expected:7,security_receipts_passed:14,security_receipts_expected:14,
      audit_task_005:"PASS",high_sentinel_006:"PASS",critical_sentinel_009:"PASS",
      evaluator_decision:$result[0].promotion.rollout_decision,applied_rollback_ready:true,
      provider_calls_made_by_builder:0,rollback_replay_status:"pending",promotion_eligible:false,replay_eligible:true,
      neutrality_scope:"instrumentation_wrapper_only_not_depth_policy",
      hashes:{primary_ledger_sha256:$primary,security_receipts_sha256:$security,quality_ledger_sha256:$quality,
        rollout_receipts_sha256:$rollout,applied_rollback_sha256:$rollback}} |
      if $generation == "v2" then . + {admission_generation:"v2",authorization_identity_sha256:$auth,
        sentinels:{audit:$rollout_doc[0].audit.sentinel,high:$rollout_doc[0].high.sentinel,
          critical:$rollout_doc[0].critical.sentinel},evaluator_artifacts:$artifacts[0],
        activation_eligible:false,implemented:false} else . end)' > "$output"
  gpt_canonicalize_json "$output"
}
