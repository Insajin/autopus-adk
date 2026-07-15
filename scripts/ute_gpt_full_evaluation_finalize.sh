#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
_ff_bootstrap_chain() {
  local dir=$1 canonical canonical_path identity sidecar line expected file_name chain actual name
  local members=(ute_codex_canary_v2.sh ute_codex_canary_v2_static.sh
    ute_codex_canary_v2_authorization.sh ute_codex_canary_v2_ledger.sh
    ute_codex_canary_v2_freeze.sh ute_codex_canary_v2_authorize.sh
    ute_codex_canary_lib.sh ute_codex_canary_prompt.sh ute_codex_canary_receipt.sh
    ute_codex_canary_exec.sh ute_codex_canary_schedule.sh ute_gpt_evidence.sh
    ute_gpt_evidence_lib.sh ute_gpt_evidence_build.sh ute_gpt_evidence_rollout.sh
    ute_gpt_full_evaluation_finalize.sh ute_gpt_full_evaluation_finalize_lib.sh)
  [[ "$dir" == /* && -d "$dir" && ! -L "$dir" ]] || return 1
  dir=$(cd "$dir" && pwd -P) || return 1
  canonical_path="$SCRIPT_DIR/../.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
  [[ -d "$canonical_path" && ! -L "$canonical_path" ]] || return 1
  canonical=$(cd "$canonical_path" && pwd -P) || return 1
  [[ "$dir" == "$canonical" ]] || return 1
  identity="$dir/gpt-full-evaluation-identity-v2.json"; sidecar="${identity%.json}.sha256"
  [[ -f "$identity" && ! -L "$identity" &&
    -f "$sidecar" && ! -L "$sidecar" ]] || return 1
  IFS= read -r line < "$sidecar" || return 1
  [[ "$line" =~ ^([0-9a-f]{64})[[:space:]][[:space:]]([^[:space:]]+)$ ]] || return 1
  expected=${BASH_REMATCH[1]}; file_name=${BASH_REMATCH[2]}
  [[ "$file_name" == "$(basename "$identity")" ]] || return 1
  [[ "$expected" == "$(shasum -a 256 "$identity" | awk '{print $1}')" ]] || return 1
  chain=$(jq -er 'if .version==2 and .admission_generation=="v2" and
    .runtime.full_chain_algorithm=="sha256-named-member-manifest-v1" and
    .runtime.full_chain_member_count==17 and
    (.runtime.full_chain_harness_sha256|test("^sha256:[0-9a-f]{64}$"))
    then .runtime.full_chain_harness_sha256 else empty end' "$identity") || return 1
  actual=$({ for name in "${members[@]}"; do
    [[ -f "$SCRIPT_DIR/$name" && ! -L "$SCRIPT_DIR/$name" ]] || exit 1
    printf '%s  scripts/%s\n' "$(shasum -a 256 "$SCRIPT_DIR/$name" | awk '{print $1}')" "$name"
  done; } | LC_ALL=C sort -k2 | shasum -a 256 | awk '{print "sha256:" $1}') || return 1
  [[ "$actual" == "$chain" ]]
}
_ff_args=("$@")
_ff_bootstrap_dir=
for ((_ff_i=0; _ff_i<${#_ff_args[@]}; _ff_i++)); do
  [[ "${_ff_args[$_ff_i]}" != --evidence-dir ]] || _ff_bootstrap_dir="${_ff_args[$((_ff_i + 1))]:-}"
done
[[ -n "$_ff_bootstrap_dir" ]] && _ff_bootstrap_chain "$_ff_bootstrap_dir" || {
  printf '%s\n' 'ute-gpt-full-evaluation-finalize: pre-source full-chain validation failed' >&2; exit 1;
}
# shellcheck source=ute_gpt_full_evaluation_finalize_lib.sh
source "$SCRIPT_DIR/ute_gpt_full_evaluation_finalize_lib.sh"
ff_read_bounded_integer() {
  local file=$1 key=$2 maximum=$3 value
  case "$key" in observed_calls|attempted_calls|observed_raw_total_tokens) ;; *) return 1 ;; esac
  value=$(jq -er --arg key "$key" --argjson maximum "$maximum" '
    .[$key] | select(type == "number" and floor == . and . >= 0 and . <= $maximum) | tostring
  ' "$file") || return 1
  [[ "$value" =~ ^(0|[1-9][0-9]*)$ ]] || return 1
  printf '%s\n' "$value"
}
ff_load_primary_counters() {
  local calls raw
  calls=$(ff_read_bounded_integer "$PRIMARY_LEDGER" observed_calls 64) || return 1
  raw=$(ff_read_bounded_integer "$PRIMARY_LEDGER" observed_raw_total_tokens 1500000) || return 1
  FF_PRIMARY_CALLS=$calls
  FF_PRIMARY_RAW=$raw
}
ff_load_replay_counters() {
  local calls raw
  calls=$(ff_read_bounded_integer "$ROLLBACK_LEDGER" observed_calls 64) || return 1
  raw=$(ff_read_bounded_integer "$ROLLBACK_LEDGER" observed_raw_total_tokens 1500000) || return 1
  FF_REPLAY_CALLS=$calls
  FF_REPLAY_RAW=$raw
}
ff_validate_full_chain() {
  local expected=$1 work raw manifest name actual
  local members=(ute_codex_canary_v2.sh ute_codex_canary_v2_static.sh
    ute_codex_canary_v2_authorization.sh ute_codex_canary_v2_ledger.sh
    ute_codex_canary_v2_freeze.sh ute_codex_canary_v2_authorize.sh
    ute_codex_canary_lib.sh ute_codex_canary_prompt.sh ute_codex_canary_receipt.sh
    ute_codex_canary_exec.sh ute_codex_canary_schedule.sh ute_gpt_evidence.sh
    ute_gpt_evidence_lib.sh ute_gpt_evidence_build.sh ute_gpt_evidence_rollout.sh
    ute_gpt_full_evaluation_finalize.sh ute_gpt_full_evaluation_finalize_lib.sh)
  [[ "$expected" =~ ^sha256:[0-9a-f]{64}$ ]] || return 1
  work=$(mktemp -d "${TMPDIR:-/tmp}/ute-gpt-finalize-chain.XXXXXX") || return 1
  chmod 700 "$work"; raw="$work/raw"; manifest="$work/manifest"; : > "$raw"
  for name in "${members[@]}"; do
    if [[ ! -f "$SCRIPT_DIR/$name" || -L "$SCRIPT_DIR/$name" ]]; then rm -rf "$work"; return 1; fi
    printf '%s  scripts/%s\n' "$(shasum -a 256 "$SCRIPT_DIR/$name" | awk '{print $1}')" "$name" >> "$raw"
  done
  LC_ALL=C sort -k2 "$raw" > "$manifest" || { rm -rf "$work"; return 1; }
  actual=$(ff_sha "$manifest"); rm -rf "$work"
  [[ "$actual" == "$expected" ]]
}
ff_opaque_id() {
  local kind=$1 mode=$2 seq=$3 task=$4 digest
  digest=$(printf '%s\0%s\0%s\0%s\0%s' "$CONFIG_HASH" "$kind" "$mode" "$seq" "$task" |
    shasum -a 256 | awk '{print $1}')
  printf '%s%s\n' "${kind:0:1}" "${digest:0:24}"
}

ff_validate_opaque_ids() {
  local file=$1 mode=$2 rows seq task call_id run_id
  rows=$(mktemp "${TMPDIR:-/tmp}/ute-gpt-finalize-ids.XXXXXX") || return 1
  jq -er '.calls[] | [.sequence,.task_id,.result.call_id,.result.run_id] | @tsv' "$file" > "$rows" || {
    rm -f "$rows"; return 1;
  }
  while IFS=$'\t' read -r seq task call_id run_id; do
    if [[ ! "$seq" =~ ^[1-9][0-9]*$ || ! "$task" =~ ^ute-corpus-v1-[0-9]{3}$ ||
      "$call_id" != "$(ff_opaque_id call "$mode" "$seq" "$task")" ||
      "$run_id" != "$(ff_opaque_id run "$mode" "$seq" "$task")" ]]; then rm -f "$rows"; return 1; fi
  done < "$rows"
  rm -f "$rows"
}

ff_validate_v2_ledger_root() {
  jq -e '
    def exact_keys($allowed): ((keys - $allowed) | length) == 0 and (($allowed - keys) | length) == 0;
    (["version","admission_generation","authorization_identity_sha256","reservation_sha256",
      "spec_id","evidence_kind","mode","completed","evaluation_eligible","promotion_eligible",
      "circuit_breaker","failure_code","attempted_calls","successful_calls","planned_calls",
      "observed_calls","planned_worst_case_raw_tokens","observed_raw_total_tokens",
      "combined_primary_and_replay_observed_raw_tokens","authorization","identity",
      "applied_rollback_receipt_sha256","primary_ledger_sha256","calls","privacy"] +
      (if .mode=="rollback" then ["rollback_reservation_sha256","evaluator_summary_sha256"] else [] end)) as $keys |
    exact_keys($keys) and
    .version == 1 and .admission_generation == "v2" and
    (.authorization_identity_sha256 | type) == "string" and
    (.authorization_identity_sha256 | test("^sha256:[0-9a-f]{64}$")) and
    (.reservation_sha256 | type) == "string" and
    (.reservation_sha256 | test("^sha256:[0-9a-f]{64}$")) and
    (if .mode=="rollback" then
      (.rollback_reservation_sha256|test("^sha256:[0-9a-f]{64}$")) and
      (.evaluator_summary_sha256|test("^sha256:[0-9a-f]{64}$"))
     else ((has("rollback_reservation_sha256") or has("evaluator_summary_sha256"))|not) end)
  ' "$1" >/dev/null
}

ff_expected_bucket() {
  local file=$1 task policy
  task=$(jq -er '.rollout.audit_task_hash|select(type=="string" and test("^sha256:[0-9a-f]{64}$"))' "$file") || return 1
  policy=$(jq -er '.rollout.policy_hash|select(type=="string" and test("^sha256:[0-9a-f]{64}$"))' "$file") || return 1
  python3 - "$task" "$policy" <<'PY'
import hashlib, sys
print(int.from_bytes(hashlib.sha256((sys.argv[1] + "\0" + sys.argv[2]).encode()).digest()[:8], "big") % 100)
PY
}

ff_validate_security_quality() {
  local row expected actual
  jq -e --slurpfile c "$COHORT" '.version==1 and .receipt_count==14 and (.receipts|length)==14 and
    ([.receipts[]|(.task_id+":"+.arm)]|unique|length)==14 and ([.receipts[].receipt_sha256]|unique|length)==14 and
    ([.receipts[].task_id]|unique|sort)==([$c[0].tasks[].task_id]|sort) and all(.receipts[];. as $r |
      any($c[0].tasks[];.task_id==$r.task_id and .pair_order==$r.pair_order and .risk_tier==$r.risk_tier) and
      (.arm=="A" or .arm=="B") and .security_call.role=="security-auditor" and .security_call.effort=="max" and
      .security_call.verdict=="PASS" and .security_call.finding_count==0 and .security_call.tool_calls==0 and
      .security_call.usage_status=="actual" and .security_call.raw_total_tokens>0 and
      .patch_evidence.expected_sha256==.patch_evidence.observed_sha256 and .patch_evidence.safe_modes==true and
      .patch_evidence.git_diff_check=="PASS" and .verification.exit_code==0 and .verification.status=="PASS")' \
    "$SECURITY" >/dev/null || return 1
  while IFS= read -r row; do
    expected=$(printf '%s\n' "$row" | jq -r '.receipt_sha256')
    actual="sha256:$(printf '%s\n' "$row" | jq -cS 'del(.receipt_sha256)' | shasum -a 256 | awk '{print $1}')"
    [[ "$actual" == "$expected" ]] || return 1
  done < <(jq -c '.receipts[]' "$SECURITY")
  jq -e --slurpfile s "$SECURITY" --slurpfile c "$COHORT" '.version==1 and .row_count==7 and
    (.outcomes|length)==7 and ([.outcomes[].task_id]|unique|length)==7 and all(.outcomes[];. as $o |
      any($c[0].tasks[];.task_id==$o.task_id and .task_hash==$o.task_hash and .risk_tier==$o.risk_tier and
        .oracle_hash==$o.expected_oracle_hash) and
      .expected_oracle_hash==.baseline_observed_oracle_hash and .expected_oracle_hash==.candidate_observed_oracle_hash and
      .baseline_verification_exit_code==0 and .candidate_verification_exit_code==0 and
      .baseline_security_status=="PASS" and .candidate_security_status=="PASS" and
      any($s[0].receipts[];.task_id==$o.task_id and .arm=="A" and .receipt_sha256==$o.baseline_security_receipt_hash) and
      any($s[0].receipts[];.task_id==$o.task_id and .arm=="B" and .receipt_sha256==$o.candidate_security_receipt_hash))' \
    "$QUALITY" >/dev/null
}

ff_validate_rollout_strong() {
  local audit_i="$EVIDENCE_DIR/gpt-rollout-audit-input-v2.json" audit_r="$EVIDENCE_DIR/gpt-rollout-audit-result-v2.json"
  local high_i="$EVIDENCE_DIR/gpt-rollout-high-input-v2.json" high_r="$EVIDENCE_DIR/gpt-rollout-high-result-v2.json"
  local critical_i="$EVIDENCE_DIR/gpt-rollout-critical-input-v2.json" critical_r="$EVIDENCE_DIR/gpt-rollout-critical-result-v2.json"
  local rollback_i="$EVIDENCE_DIR/gpt-rollback-input-v2.json" t005 t006 t009 ba high_exp critical_exp high_digest critical_digest
  t005=$(jq -er '.tasks[]|select(.task_id=="ute-corpus-v1-005")|.task_hash' "$COHORT") || return 1
  t006=$(jq -er '.tasks[]|select(.task_id=="ute-corpus-v1-006")|.task_hash' "$COHORT") || return 1
  t009=$(jq -er '.tasks[]|select(.task_id=="ute-corpus-v1-009")|.task_hash' "$COHORT") || return 1
  ba=$(ff_expected_bucket "$audit_i") || return 1
  high_exp="ute-gpt-rollout-high-v2:ute-corpus-v1-006:$t006"; critical_exp="ute-gpt-rollout-critical-v2:ute-corpus-v1-009:$t009"
  high_digest="sha256:$(printf '%s' "$high_exp"|shasum -a 256|awk '{print $1}')"
  critical_digest="sha256:$(printf '%s' "$critical_exp"|shasum -a 256|awk '{print $1}')"
  jq -e --arg policy "$POLICY_HASH" --arg config "$CONFIG_HASH" --arg a "$t005" --arg hx "$high_exp" --arg cx "$critical_exp" \
    --slurpfile ai "$audit_i" --slurpfile hi "$high_i" --slurpfile ci "$critical_i" '
    .version==1 and (.calls|length)==58 and (.trials|length)==14 and (.quality_outcomes|length)==7 and
    .rollout.experiment_id=="ute-gpt-primary-v2" and .rollout.policy_hash==$policy and .rollout.config_hash==$config and
    all([$ai[0],$hi[0],$ci[0]][];.rollout.policy_hash==$policy and .rollout.config_hash==$config and .rollout.full_depth==true) and
    $ai[0].rollout.experiment_id=="ute-gpt-rollout-audit-v2" and $ai[0].rollout.risk_tier=="medium" and
    $ai[0].rollout.audit_task_hash==$a and $ai[0].rollout.audit_rate_percent==100 and
    $hi[0].rollout.experiment_id==$hx and $hi[0].rollout.risk_tier=="high" and
    ($hi[0].rollout|has("audit_task_hash")|not) and ($hi[0].rollout|has("audit_rate_percent")) and $hi[0].rollout.audit_rate_percent==0 and
    $ci[0].rollout.experiment_id==$cx and $ci[0].rollout.risk_tier=="critical" and
    ($ci[0].rollout|has("audit_task_hash")|not) and ($ci[0].rollout|has("audit_rate_percent")) and $ci[0].rollout.audit_rate_percent==0' "$INPUT" >/dev/null || return 1
  jq -e --argjson ba "$ba" --arg hd "$high_digest" --arg cd "$critical_digest" --slurpfile a "$audit_r" \
    --slurpfile h "$high_r" --slurpfile c "$critical_r" '
    .comparison.median_paired_raw_reduction_pct>=25 and .comparison.paired_task_count==7 and
    .quality.complete==true and .quality.consistent==true and .promotion.rollout_decision=="ELIGIBLE_NEXT_CANARY" and
    $a[0].promotion.high_critical_regressions==0 and $a[0].rollout_receipt.decision=="AUDIT" and
    $a[0].rollout_receipt.active_profile=="full_ultra" and $a[0].rollout_receipt.full_depth==true and
    $a[0].rollout_receipt.risk_tier=="medium" and
    $a[0].rollout_receipt.audit_selection=={selected:true,bucket:$ba,rate_percent:100,algorithm:"sha256_mod_100_v1"} and
    all([$h[0],$c[0]][];.promotion.high_critical_regressions==0 and .rollout_receipt.decision=="CANARY" and
      .rollout_receipt.active_profile=="full_ultra" and .rollout_receipt.full_depth==true and
      .rollout_receipt.selection_reason=="risk_requires_full_depth") and
    $h[0].rollout_receipt.risk_tier=="high" and $h[0].rollout_receipt.experiment_id==$hd and
    $h[0].rollout_receipt.audit_selection=={selected:false,bucket:0,rate_percent:0,algorithm:""} and
    $c[0].rollout_receipt.risk_tier=="critical" and $c[0].rollout_receipt.experiment_id==$cd and
    $c[0].rollout_receipt.audit_selection=={selected:false,bucket:0,rate_percent:0,algorithm:""}' \
    "$RESULT" >/dev/null || return 1
  jq -e --arg pi "$(ff_sha "$INPUT")" --arg pr "$(ff_sha "$RESULT")" --arg ai "$(ff_sha "$audit_i")" \
    --arg ar "$(ff_sha "$audit_r")" --arg hi "$(ff_sha "$high_i")" --arg hr "$(ff_sha "$high_r")" \
    --arg ci "$(ff_sha "$critical_i")" --arg cr "$(ff_sha "$critical_r")" --arg a "$t005" --arg h "$t006" --arg c "$t009" \
    --arg hx "$high_exp" --arg cx "$critical_exp" --arg hd "$high_digest" --arg cd "$critical_digest" --argjson ba "$ba" '
    .version==1 and .evidence_kind=="gpt_rollout_receipts" and
    .primary.input_sha256==$pi and .primary.result_sha256==$pr and .audit.input_sha256==$ai and .audit.result_sha256==$ar and
    .high.input_sha256==$hi and .high.result_sha256==$hr and .critical.input_sha256==$ci and .critical.result_sha256==$cr and
    .audit.sentinel=={task_id:"ute-corpus-v1-005",task_hash:$a,audit_rate_percent:100,selected:true} and
    .high.sentinel=={task_id:"ute-corpus-v1-006",task_hash:$h,audit_rate_percent:0,selected:false} and
    .critical.sentinel=={task_id:"ute-corpus-v1-009",task_hash:$c,audit_rate_percent:0,selected:false} and
    .audit.audit_selection=={selected:true,bucket:$ba,rate_percent:100,algorithm:"sha256_mod_100_v1"} and
    .high.experiment_identity==$hx and .high.experiment_identity_sha256==$hd and
    .critical.experiment_identity==$cx and .critical.experiment_identity_sha256==$cd and
    .high.audit_selection=={selected:false,bucket:0,rate_percent:0,algorithm:""} and
    .critical.audit_selection=={selected:false,bucket:0,rate_percent:0,algorithm:""}' \
    "$ROLLOUT" >/dev/null || return 1
  jq -e '.policy_parity_passed==false and .candidate_behavior_active==true and
    .rollout.experiment_id=="ute-gpt-rollback-v2" and .rollout.full_depth==false' "$rollback_i" >/dev/null &&
    jq -e '.promotion.rollout_decision=="ROLLBACK" and .rollout_receipt.decision=="ROLLBACK" and
      .rollout_receipt.active_profile=="full_ultra" and .rollout_receipt.full_depth==true' "$ROLLBACK_RESULT" >/dev/null
}

ff_validate_summary_manifest() {
  local stem file
  jq -e --arg auth "$AUTHORIZATION_IDENTITY_SHA256" --arg primary "$PRIMARY_HASH" --arg security "$(ff_sha "$SECURITY")" \
    --arg quality "$(ff_sha "$QUALITY")" --arg rollout "$(ff_sha "$ROLLOUT")" --arg applied "$(ff_sha "$APPLIED")" \
    --slurpfile r "$ROLLOUT" --slurpfile result "$RESULT" '
    .admission_generation=="v2" and .authorization_identity_sha256==$auth and .primary_calls==58 and
    .rollback_replay_status=="pending" and .provider_calls_made_by_builder==0 and .promotion_eligible==false and
    .activation_eligible==false and .implemented==false and .replay_eligible==true and
    .quality_tasks_passed==7 and .quality_tasks_expected==7 and .security_receipts_passed==14 and
    .security_receipts_expected==14 and .evaluator_decision=="ELIGIBLE_NEXT_CANARY" and
    .applied_rollback_ready==true and .audit_task_005=="PASS" and
    .median_paired_raw_reduction_pct==$result[0].comparison.median_paired_raw_reduction_pct and
    .high_sentinel_006=="PASS" and .critical_sentinel_009=="PASS" and .sentinels==
      {audit:$r[0].audit.sentinel,high:$r[0].high.sentinel,critical:$r[0].critical.sentinel} and
    .hashes=={primary_ledger_sha256:$primary,security_receipts_sha256:$security,quality_ledger_sha256:$quality,
      rollout_receipts_sha256:$rollout,applied_rollback_sha256:$applied} and
    (.evaluator_artifacts|length)==14' "$SUMMARY" >/dev/null || return 1
  for stem in gpt-security-receipts gpt-quality-ledger gpt-efficiency-input gpt-efficiency-result \
    gpt-rollout-audit-input gpt-rollout-audit-result gpt-rollout-high-input gpt-rollout-high-result \
    gpt-rollout-critical-input gpt-rollout-critical-result gpt-rollout-receipts gpt-rollback-input \
    gpt-rollback-result gpt-applied-rollback; do
    file="$EVIDENCE_DIR/$stem-v2.json"
    jq -e --arg name "$(basename "$file")" --arg hash "$(ff_sha "$file")" \
      '.evaluator_artifacts[$name]==$hash' "$SUMMARY" >/dev/null || return 1
  done
}

ff_validate_evaluator_strong() {
  ff_validate_security_quality || return 1
  ff_validate_rollout_strong || return 1
  jq -e --arg policy "$POLICY_HASH" --arg config "$CONFIG_HASH" --arg primary "$PRIMARY_HASH" \
    --arg logical "$(ff_sha "$ROLLBACK_RESULT")" '.version==1 and .evidence_kind=="applied_policy_rollback" and
    .decision=="ROLLBACK" and .applied==true and .active_profile=="full_ultra" and .atomic_replace==true and
    .fsync_completed==true and .state_readback=="full_ultra" and .policy_sha256==$policy and .config_sha256==$config and
    .primary_ledger_sha256==$primary and .logical_rollback_result_sha256==$logical and
    .state_readback_sha256==.after_binding_sha256 and .before_binding_sha256!=.after_binding_sha256' "$APPLIED" >/dev/null || return 1
  ff_validate_summary_manifest
}
usage() {
  printf '%s\n' 'usage: ute_gpt_full_evaluation_finalize.sh finalize --evidence-dir DIR --primary-ledger FILE [--rollback-ledger FILE] --auto BIN'
}

[[ ${1:-} == finalize ]] || { usage >&2; exit 2; }
shift
EVIDENCE_DIR= PRIMARY_LEDGER= ROLLBACK_LEDGER= AUTO=
while [[ $# -gt 0 ]]; do
  case "$1" in
    --evidence-dir) [[ $# -ge 2 && -z "$EVIDENCE_DIR" ]] || ff_die "invalid --evidence-dir"; EVIDENCE_DIR=$2; shift 2 ;;
    --primary-ledger) [[ $# -ge 2 && -z "$PRIMARY_LEDGER" ]] || ff_die "invalid --primary-ledger"; PRIMARY_LEDGER=$2; shift 2 ;;
    --rollback-ledger) [[ $# -ge 2 && -z "$ROLLBACK_LEDGER" ]] || ff_die "invalid --rollback-ledger"; ROLLBACK_LEDGER=$2; shift 2 ;;
    --auto) [[ $# -ge 2 && -z "$AUTO" ]] || ff_die "invalid --auto"; AUTO=$2; shift 2 ;;
    *) ff_die "unsupported argument" ;;
  esac
done
[[ -n "$EVIDENCE_DIR" && -n "$PRIMARY_LEDGER" && -n "$AUTO" ]] || ff_die "required input missing"

ff_prepare
if ff_evaluate_chain; then
  FF_CODE=
  ff_publish_terminal true || ff_die "terminal publish failed"
  printf '%s\n' 'gpt full evaluation finalizer: PASS (provider_calls=63 promotion=false activation=false)'
  exit 0
fi
ff_publish_terminal false || ff_die "failure terminal publish failed"
printf 'gpt full evaluation finalizer: BLOCKED (%s)\n' "$FF_CODE" >&2
exit 1
