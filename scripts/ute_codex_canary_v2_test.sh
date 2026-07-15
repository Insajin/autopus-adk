#!/usr/bin/env bash
set -euo pipefail
# Provider-zero RED/GREEN admission suite. It never invokes a real Auto or Codex provider.
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd -P)
SOURCE="$REPO_ROOT/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
# shellcheck source=ute_codex_canary_testlib.sh
source "$SCRIPT_DIR/ute_codex_canary_testlib.sh"
# shellcheck source=ute_codex_canary_v2_testlib.sh
source "$SCRIPT_DIR/ute_codex_canary_v2_testlib.sh"
v2_make_evaluator_evidence() {
  local primary="$OUTPUT/gpt-primary-call-ledger-v2.json" rows="$TMP_ROOT/security.jsonl" base hash task arm order risk oracle
  local security="$OUTPUT/gpt-security-receipts-v2.json" quality="$OUTPUT/gpt-quality-ledger-v2.json"
  local input="$OUTPUT/gpt-efficiency-input-v2.json" result="$OUTPUT/gpt-efficiency-result-v2.json" rate bucket selected decision raw digest algorithm
  : > "$rows"
  while IFS=$'\t' read -r task order risk oracle; do for arm in A B; do
    base="$TMP_ROOT/security-base.json"
    jq -n --arg task "$task" --arg arm "$arm" --arg order "$order" --arg risk "$risk" --arg oracle "$oracle" '
      {task_id:$task,arm:$arm,pair_order:$order,risk_tier:$risk,security_call:{role:"security-auditor",
      effort:"max",verdict:"PASS",finding_count:0,tool_calls:0,usage_status:"actual",raw_total_tokens:1},
      patch_evidence:{expected_sha256:$oracle,observed_sha256:$oracle,safe_modes:true,git_diff_check:"PASS"},
      verification:{status:"PASS",exit_code:0}}' > "$base"
    hash="sha256:$(jq -cS '.' "$base" | shasum -a 256 | awk '{print $1}')"
    jq -cS --arg hash "$hash" '.+{receipt_sha256:$hash}' "$base" >> "$rows"
  done; done < <(jq -r '.tasks[]|[.task_id,.pair_order,.risk_tier,.oracle_hash]|@tsv' "$OUTPUT/gpt-canary-cohort-v1.json")
  jq -n --slurpfile rows <(jq -s '.' "$rows") '{version:1,receipt_count:14,receipts:$rows[0]}' > "$security"
  jq -n --slurpfile s "$security" --slurpfile c "$OUTPUT/gpt-canary-cohort-v1.json" \
    '{version:1,row_count:7,outcomes:[$s[0].receipts[]|select(.arm=="A") as $a|
    ($s[0].receipts[]|select(.task_id==$a.task_id and .arm=="B")) as $b|
    ($c[0].tasks[]|select(.task_id==$a.task_id)) as $t|
    {task_id:$a.task_id,task_hash:$t.task_hash,risk_tier:$t.risk_tier,expected_oracle_hash:$t.oracle_hash,
     baseline_observed_oracle_hash:$t.oracle_hash,candidate_observed_oracle_hash:$t.oracle_hash,baseline_verification_exit_code:0,
     candidate_verification_exit_code:0,baseline_security_status:"PASS",candidate_security_status:"PASS",
     baseline_security_receipt_hash:$a.receipt_sha256,candidate_security_receipt_hash:$b.receipt_sha256}]}' > "$quality"
  jq -n --arg policy "sha256:$(v2_sha "$OUTPUT/gpt-codex-policy-v2.json")" \
    --arg config "sha256:$(v2_sha "$OUTPUT/gpt-codex-config-v2.json")" \
    '{version:1,calls:[range(0;58)],trials:[range(0;14)],quality_outcomes:[range(0;7)],
      rollout:{experiment_id:"ute-gpt-primary-v2",policy_hash:$policy,config_hash:$config,full_depth:false}}' > "$input"
  jq -n '{version:1,comparison:{median_paired_raw_reduction_pct:30,paired_task_count:7},
    quality:{complete:true,consistent:true,derived_regressions:[]},
    promotion:{high_critical_regressions:0,rollout_decision:"ELIGIBLE_NEXT_CANARY"},
    rollout_receipt:{decision:"CANARY",active_profile:"compact_ultra",full_depth:false}}' > "$result"
  for task in audit high critical; do
    case "$task" in
      audit) hash=$(jq -r '.tasks[]|select(.task_id=="ute-corpus-v1-005")|.task_hash' "$OUTPUT/gpt-canary-cohort-v1.json"); risk=medium; rate=100; selected=true; decision=AUDIT; raw=ute-gpt-rollout-audit-v2; digest=; algorithm=sha256_mod_100_v1 ;;
      high) hash=$(jq -r '.tasks[]|select(.task_id=="ute-corpus-v1-006")|.task_hash' "$OUTPUT/gpt-canary-cohort-v1.json"); risk=high; rate=0; selected=false; decision=CANARY; raw="ute-gpt-rollout-high-v2:ute-corpus-v1-006:$hash"; digest="sha256:$(printf %s "$raw"|shasum -a 256|awk '{print $1}')"; algorithm= ;;
      critical) hash=$(jq -r '.tasks[]|select(.task_id=="ute-corpus-v1-009")|.task_hash' "$OUTPUT/gpt-canary-cohort-v1.json"); risk=critical; rate=0; selected=false; decision=CANARY; raw="ute-gpt-rollout-critical-v2:ute-corpus-v1-009:$hash"; digest="sha256:$(printf %s "$raw"|shasum -a 256|awk '{print $1}')"; algorithm= ;;
    esac
    [[ "$task" == audit ]] && bucket=$(v2_bucket "$hash" "sha256:$(v2_sha "$OUTPUT/gpt-codex-policy-v2.json")") || bucket=0
    jq --arg id "$raw" --arg risk "$risk" --arg hash "$hash" --argjson rate "$rate" --argjson selected "$selected" \
      '.rollout.experiment_id=$id|.rollout.risk_tier=$risk|.rollout.audit_rate_percent=$rate|.rollout.full_depth=true|
      if $selected then .rollout.audit_task_hash=$hash else del(.rollout.audit_task_hash) end' "$input" > "$OUTPUT/gpt-rollout-$task-input-v2.json"
    jq -n --arg decision "$decision" --arg risk "$risk" --arg experiment "$digest" --arg algorithm "$algorithm" --argjson selected "$selected" --argjson rate "$rate" --argjson bucket "$bucket" \
      '{version:1,promotion:{high_critical_regressions:0,rollout_decision:"ELIGIBLE_NEXT_CANARY"},
       rollout_receipt:{decision:$decision,active_profile:"full_ultra",full_depth:true,risk_tier:$risk,
       experiment_id:$experiment,selection_reason:"risk_requires_full_depth",audit_selection:{selected:$selected,bucket:$bucket,
       rate_percent:$rate,algorithm:$algorithm}}}' > "$OUTPUT/gpt-rollout-$task-result-v2.json"
  done
  jq -n --arg pi "sha256:$(v2_sha "$input")" --arg pr "sha256:$(v2_sha "$result")" \
    --arg ai "sha256:$(v2_sha "$OUTPUT/gpt-rollout-audit-input-v2.json")" --arg ar "sha256:$(v2_sha "$OUTPUT/gpt-rollout-audit-result-v2.json")" \
    --arg hi "sha256:$(v2_sha "$OUTPUT/gpt-rollout-high-input-v2.json")" --arg hr "sha256:$(v2_sha "$OUTPUT/gpt-rollout-high-result-v2.json")" \
    --arg ci "sha256:$(v2_sha "$OUTPUT/gpt-rollout-critical-input-v2.json")" --arg cr "sha256:$(v2_sha "$OUTPUT/gpt-rollout-critical-result-v2.json")" \
    --slurpfile ai_doc "$OUTPUT/gpt-rollout-audit-input-v2.json" --slurpfile hi_doc "$OUTPUT/gpt-rollout-high-input-v2.json" \
    --slurpfile ci_doc "$OUTPUT/gpt-rollout-critical-input-v2.json" --slurpfile a "$OUTPUT/gpt-rollout-audit-result-v2.json" \
    --slurpfile h "$OUTPUT/gpt-rollout-high-result-v2.json" --slurpfile c "$OUTPUT/gpt-rollout-critical-result-v2.json" \
    --slurpfile cohort "$OUTPUT/gpt-canary-cohort-v1.json" '
    def row($x;$i;$task):{input_sha256:$i[0],result_sha256:$i[1],decision:$x.rollout_receipt.decision,
      active_profile:$x.rollout_receipt.active_profile,full_depth:$x.rollout_receipt.full_depth,risk_tier:$x.rollout_receipt.risk_tier,
      selection_reason:$x.rollout_receipt.selection_reason,audit_selection:$x.rollout_receipt.audit_selection,
      sentinel:{task_id:$task,task_hash:($cohort[0].tasks[]|select(.task_id==$task)|.task_hash),
      audit_rate_percent:$i[2],selected:$x.rollout_receipt.audit_selection.selected}};
    {version:1,evidence_kind:"gpt_rollout_receipts",primary:{input_sha256:$pi,result_sha256:$pr},
    audit:row($a[0];[$ai,$ar,$ai_doc[0].rollout.audit_rate_percent];"ute-corpus-v1-005"),
    high:(row($h[0];[$hi,$hr,$hi_doc[0].rollout.audit_rate_percent];"ute-corpus-v1-006")+
      {experiment_identity:$hi_doc[0].rollout.experiment_id,experiment_identity_sha256:$h[0].rollout_receipt.experiment_id}),
    critical:(row($c[0];[$ci,$cr,$ci_doc[0].rollout.audit_rate_percent];"ute-corpus-v1-009")+
      {experiment_identity:$ci_doc[0].rollout.experiment_id,experiment_identity_sha256:$c[0].rollout_receipt.experiment_id})}' \
    > "$OUTPUT/gpt-rollout-receipts-v2.json"
  jq '.policy_parity_passed=false|.candidate_behavior_active=true|.rollout.experiment_id="ute-gpt-rollback-v2"' \
    "$input" > "$OUTPUT/gpt-rollback-input-v2.json"
  jq -n '{version:1,promotion:{rollout_decision:"ROLLBACK"},rollout_receipt:{decision:"ROLLBACK",
    active_profile:"full_ultra",full_depth:true}}' > "$OUTPUT/gpt-rollback-result-v2.json"
  v2_finish_evaluator_evidence "$primary" "$security" "$quality" "$result"
}
v2_finish_evaluator_evidence() {
  local primary=$1 security=$2 quality=$3 result=$4 applied rollout stem file manifest rows
  applied="$OUTPUT/gpt-applied-rollback-v2.json"; rollout="$OUTPUT/gpt-rollout-receipts-v2.json"
  jq -n --arg policy "sha256:$(v2_sha "$OUTPUT/gpt-codex-policy-v2.json")" \
    --arg config "sha256:$(v2_sha "$OUTPUT/gpt-codex-config-v2.json")" --arg primary "sha256:$(v2_sha "$primary")" \
    --arg logical "sha256:$(v2_sha "$OUTPUT/gpt-rollback-result-v2.json")" '
    {version:1,evidence_kind:"applied_policy_rollback",decision:"ROLLBACK",applied:true,
    active_profile:"full_ultra",atomic_replace:true,fsync_completed:true,state_readback:"full_ultra",
    policy_sha256:$policy,config_sha256:$config,primary_ledger_sha256:$primary,
    logical_rollback_result_sha256:$logical,before_binding_sha256:("sha256:"+("b"*64)),
    after_binding_sha256:("sha256:"+("c"*64)),state_readback_sha256:("sha256:"+("c"*64))}' > "$applied"
  manifest="$TMP_ROOT/evaluator-artifacts.json"; rows="$TMP_ROOT/evaluator-artifacts.jsonl"; : > "$rows"
  for stem in gpt-security-receipts gpt-quality-ledger gpt-efficiency-input gpt-efficiency-result \
    gpt-rollout-audit-input gpt-rollout-audit-result gpt-rollout-high-input gpt-rollout-high-result \
    gpt-rollout-critical-input gpt-rollout-critical-result gpt-rollout-receipts gpt-rollback-input \
    gpt-rollback-result gpt-applied-rollback; do
    file="$OUTPUT/$stem-v2.json"; v2_sidecar "$file"
    jq -cn --arg key "${file##*/}" --arg value "sha256:$(v2_sha "$file")" '{key:$key,value:$value}' >> "$rows"
  done
  jq -s 'from_entries' "$rows" > "$manifest"
  jq -n --arg auth "$AUTH" --arg primary "sha256:$(v2_sha "$primary")" \
    --arg security "sha256:$(v2_sha "$security")" --arg quality "sha256:$(v2_sha "$quality")" \
    --arg rollout "sha256:$(v2_sha "$rollout")" --arg applied "sha256:$(v2_sha "$applied")" \
    --slurpfile rollout_doc "$rollout" --slurpfile artifacts "$manifest" '
    {version:1,admission_generation:"v2",authorization_identity_sha256:$auth,primary_calls:58,
    median_paired_raw_reduction_pct:30,quality_tasks_passed:7,quality_tasks_expected:7,
    security_receipts_passed:14,security_receipts_expected:14,audit_task_005:"PASS",
    high_sentinel_006:"PASS",critical_sentinel_009:"PASS",evaluator_decision:"ELIGIBLE_NEXT_CANARY",
    applied_rollback_ready:true,rollback_replay_status:"pending",replay_eligible:true,provider_calls_made_by_builder:0,
    promotion_eligible:false,activation_eligible:false,implemented:false,
    sentinels:{audit:$rollout_doc[0].audit.sentinel,high:$rollout_doc[0].high.sentinel,
      critical:$rollout_doc[0].critical.sentinel},evaluator_artifacts:$artifacts[0],
    hashes:{primary_ledger_sha256:$primary,security_receipts_sha256:$security,
      quality_ledger_sha256:$quality,rollout_receipts_sha256:$rollout,applied_rollback_sha256:$applied}}' \
    > "$OUTPUT/gpt-primary-evaluation-summary-v2.json"
  v2_sidecar "$OUTPUT/gpt-primary-evaluation-summary-v2.json"
}
v2_require_sources
TMP_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/ute-codex-canary-v2-test.XXXXXX")
trap 'rm -rf "$TMP_ROOT"' EXIT
SNAPSHOT="$TMP_ROOT/snapshot.git"; COUNT_FILE="$TMP_ROOT/invocations"
FAKE_AUTO="$TMP_ROOT/auto"; FAKE_CODEX="$TMP_ROOT/codex"
AUTO_VERSION_FILE="$TMP_ROOT/auto-versions"; CODEX_VERSION_FILE="$TMP_ROOT/codex-versions"
export V2_AUTO_VERSION_COUNT_FILE="$AUTO_VERSION_FILE" V2_CODEX_VERSION_COUNT_FILE="$CODEX_VERSION_FILE"
INSTALL= OUTPUT= HARNESS= AUTH= PREFLIGHT= CLAIM= V1_BEFORE=
make_fake_auto "$FAKE_AUTO"; make_fake_codex "$FAKE_CODEX"
v2_wrap_launcher "$FAKE_AUTO" auto; v2_wrap_launcher "$FAKE_CODEX" codex
export PATH="$TMP_ROOT:$PATH"
git clone -q --bare "$REPO_ROOT" "$SNAPSHOT"
v2_reset_runtime_counts; v2_freeze_candidate
assert_eq 1 "$(invocation_count "$AUTO_VERSION_FILE")" "freeze Auto version count"
assert_eq 1 "$(invocation_count "$CODEX_VERSION_FILE")" "freeze Codex version count"

v2_make_install awaiting "$FAKE_AUTO"; : > "$COUNT_FILE"; v2_reset_runtime_counts
"$HARNESS" preflight --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --output "$OUTPUT" >/dev/null
jq -e --arg auth "$AUTH" '.version==2 and .admission_generation=="v2" and
  .authorization_identity.sha256==$auth and .observed_spend.provider_calls_made==0 and
  .observed_spend.raw_total_tokens==0 and
  .decision.status=="AWAITING_EXPLICIT_FULL_EVALUATION_AUTHORIZATION" and
  .decision.provider_execution_started==false and .decision.activation==false and
  .decision.promotion==false and .decision.implemented==false' \
  "$OUTPUT/gpt-canary-preflight-v2.json" >/dev/null || fail "v2 preflight contract"
for stem in gpt-codex-policy-v2 gpt-codex-config-v2 gpt-verdict-schema-v2 \
  gpt-canary-schedule-v2 gpt-snapshot-manifest-v2 gpt-prior-evidence-manifest-v2 \
  gpt-full-evaluation-identity-v2 gpt-canary-preflight-v2; do
  (cd "$OUTPUT" && shasum -a 256 -c "$stem.sha256") >/dev/null || fail "$stem sidecar"
done
v2_assert_zero_and_unclaimed preflight
v2_assert_no_versions preflight
v2_primary "$FAKE_AUTO" "$TMP_ROOT/state-bare" >/dev/null 2>&1 && fail "bare execute opt-in admitted"
v2_assert_zero_and_unclaimed bare-opt-in
v2_assert_no_versions bare-opt-in
"$HARNESS" rollback --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$TMP_ROOT/state-rollback" \
  --output "$OUTPUT" >/dev/null 2>&1 && fail "rollback without exact admission accepted"
v2_assert_zero_and_unclaimed rollback-without-admission

for mode in wrong tampered; do
  v2_make_install "auth-$mode" "$FAKE_AUTO"; : > "$COUNT_FILE"; v2_write_authorization "$mode"
  [[ "$mode" != tampered ]] || printf '\n' >> "$OUTPUT/gpt-full-evaluation-authorization-v2.json"
  v2_primary "$FAKE_AUTO" "$TMP_ROOT/state-auth-$mode" >/dev/null 2>&1 && fail "$mode receipt admitted"
  v2_assert_zero_and_unclaimed "$mode-receipt"
  v2_assert_no_versions "$mode-receipt"
done

v2_make_install noncanonical "$FAKE_AUTO"; : > "$COUNT_FILE"; v2_write_authorization
mkdir -p "$TMP_ROOT/noncanonical"
v2_primary "$FAKE_AUTO" "$TMP_ROOT/state-noncanonical" "$TMP_ROOT/noncanonical" >/dev/null 2>&1 &&
  fail "noncanonical output admitted"
v2_assert_zero_and_unclaimed noncanonical-output
v2_assert_no_versions noncanonical-output

v2_make_install wrong-hash "$FAKE_AUTO"; : > "$COUNT_FILE"; v2_reset_runtime_counts; v2_write_authorization
BAD_AUTO="$TMP_ROOT/auto-bad-hash"; cp "$FAKE_AUTO" "$BAD_AUTO"; cp "$FAKE_AUTO.real" "$BAD_AUTO.real"
printf '\n' >> "$BAD_AUTO"; chmod 755 "$BAD_AUTO"
v2_primary "$BAD_AUTO" "$TMP_ROOT/state-wrong-hash" >/dev/null 2>&1 && fail "wrong Auto hash admitted"
v2_assert_zero_and_unclaimed wrong-auto-hash
v2_assert_no_versions wrong-auto-hash

v2_make_install wrong-codex "$FAKE_AUTO"; : > "$COUNT_FILE"; v2_reset_runtime_counts; v2_write_authorization
cp "$FAKE_CODEX" "$TMP_ROOT/codex.saved"; printf '\n' >> "$FAKE_CODEX"; chmod 755 "$FAKE_CODEX"
v2_primary "$FAKE_AUTO" "$TMP_ROOT/state-wrong-codex" >/dev/null 2>&1 && fail "wrong Codex hash admitted"
v2_assert_zero_and_unclaimed wrong-codex-hash; v2_assert_no_versions wrong-codex-hash
mv "$TMP_ROOT/codex.saved" "$FAKE_CODEX"; chmod 755 "$FAKE_CODEX"

v2_make_install helper-tamper "$FAKE_AUTO"; : > "$COUNT_FILE"; v2_reset_runtime_counts; v2_write_authorization
export V2_HELPER_SENTINEL="$TMP_ROOT/helper-sourced"
awk 'NR==2 {print "printf sourced >> \"${V2_HELPER_SENTINEL:?}\""} {print}' \
  "$INSTALL/scripts/ute_codex_canary_v2_static.sh" > "$TMP_ROOT/static.tampered"
mv "$TMP_ROOT/static.tampered" "$INSTALL/scripts/ute_codex_canary_v2_static.sh"
v2_primary "$FAKE_AUTO" "$TMP_ROOT/state-helper" >/dev/null 2>&1 && fail "tampered helper admitted"
v2_assert_zero_and_unclaimed helper-tamper; v2_assert_no_versions helper-tamper
[[ ! -e "$V2_HELPER_SENTINEL" ]] || fail "tampered helper was sourced before bundle verification"

v2_make_install full17-outside9 "$FAKE_AUTO"; : > "$COUNT_FILE"; v2_reset_runtime_counts
export V2_HELPER_SENTINEL="$TMP_ROOT/full17-outside9-sourced"; v2_tamper_full17
UTE_CODEX_CANARY_V2_AUTHORIZE=YES "$INSTALL/scripts/ute_codex_canary_v2_authorize.sh" authorize \
  --evidence-dir "$OUTPUT" --identity "$AUTH" --authorized-at 2026-07-15T13:00:00+09:00 >/dev/null 2>&1 &&
  fail "outside-admission full17 authorized"
[[ ! -e "$OUTPUT/gpt-full-evaluation-authorization-v2.json" && ! -e "$V2_HELPER_SENTINEL" ]] || fail "authorize full17 side effect"
cp "$SCRIPT_DIR/ute_gpt_evidence_lib.sh" "$INSTALL/scripts/ute_gpt_evidence_lib.sh"; v2_write_authorization; v2_tamper_full17
v2_primary "$FAKE_AUTO" "$TMP_ROOT/state-full17-outside9" >/dev/null 2>&1 && fail "outside-admission full17 admitted"
v2_assert_zero_and_unclaimed full17-outside9; v2_assert_no_versions full17-outside9

v2_make_install phase-primary "$FAKE_AUTO"; : > "$COUNT_FILE"; v2_write_authorization
v2_phase_tamper primary phase-primary

v2_make_install frozen-symlink "$FAKE_AUTO"; : > "$COUNT_FILE"; v2_reset_runtime_counts; v2_write_authorization
mv "$OUTPUT/gpt-codex-policy-v2.json" "$OUTPUT/policy.real"
ln -s policy.real "$OUTPUT/gpt-codex-policy-v2.json"
v2_primary "$FAKE_AUTO" "$TMP_ROOT/state-symlink" >/dev/null 2>&1 && fail "symlinked frozen policy admitted"
v2_assert_zero_and_unclaimed frozen-symlink; v2_assert_no_versions frozen-symlink

WRONG_VERSION_AUTO="$TMP_ROOT/auto-wrong-version"; cp "$FAKE_AUTO" "$WRONG_VERSION_AUTO"
cp "$FAKE_AUTO.real" "$WRONG_VERSION_AUTO.real"
sed -i.bak 's/0\.50\.99-test/0.50.98-wrong/' "$WRONG_VERSION_AUTO.real"; rm "$WRONG_VERSION_AUTO.real.bak"
v2_make_install wrong-version "$WRONG_VERSION_AUTO"; : > "$COUNT_FILE"; v2_reset_runtime_counts; v2_write_authorization
v2_primary "$WRONG_VERSION_AUTO" "$TMP_ROOT/state-wrong-version" >/dev/null 2>&1 && fail "wrong Auto version admitted"
v2_assert_zero_and_unclaimed wrong-auto-version

v2_make_install partial "$FAKE_AUTO"; : > "$COUNT_FILE"; v2_write_authorization
jq --arg auth "$AUTH" '. + {admission_generation:"v2",authorization_identity_sha256:$auth}' \
  "$OUTPUT/gpt-primary-call-ledger-v1.partial-fail.json" > "$OUTPUT/gpt-primary-call-ledger-v2.partial-fail.json"
v2_sidecar "$OUTPUT/gpt-primary-call-ledger-v2.partial-fail.json"
v2_primary "$FAKE_AUTO" "$TMP_ROOT/state-partial" >/dev/null 2>&1 && fail "existing v2 partial ledger admitted"
v2_assert_zero_and_unclaimed existing-v2-partial

v2_make_install success "$FAKE_AUTO"; : > "$COUNT_FILE"; v2_reset_runtime_counts; v2_write_authorization
FAKE_AUTO_DELAY_SECONDS=0.02 v2_primary "$FAKE_AUTO" "$TMP_ROOT/state-success" > "$TMP_ROOT/progress" &
pid=$!
for _ in {1..200}; do [[ $(invocation_count "$COUNT_FILE") -ge 1 ]] && break; sleep 0.01; done
v2_primary "$FAKE_AUTO" "$TMP_ROOT/state-concurrent" >/dev/null 2>&1 && fail "concurrent identity admitted"
wait "$pid"
assert_eq "58" "$(invocation_count "$COUNT_FILE")" "authorized primary call count"
ledger="$OUTPUT/gpt-primary-call-ledger-v2.json"
assert_primary_ledger "$ledger"
jq -e --arg auth "$AUTH" '.version==1 and .admission_generation=="v2" and
  .authorization_identity_sha256==$auth and (.reservation_sha256|test("^sha256:[0-9a-f]{64}$")) and .authorization==
  {provider_call_cap:64,raw_token_cap:1500000,concurrency:1,retries:0,
   primary_calls_reserved:58,rollback_calls_reserved:5,total_calls_reserved:63,
   planned_worst_case_raw_tokens:1446000} and .promotion_eligible==false' "$ledger" >/dev/null ||
  fail "v2 ledger admission contract"
[[ -f "$CLAIM/reservation.json" ]] || fail "identity claim missing"
reservation="sha256:$(v2_sha "$OUTPUT/gpt-full-evaluation-reservation-v2.json")"
jq -e --arg reservation "$reservation" '.reservation_sha256==$reservation' "$ledger" >/dev/null ||
  fail "primary ledger reservation binding"
assert_no_raw_retention "$TMP_ROOT/state-success" "$OUTPUT"
[[ "$V1_BEFORE" == "$(v2_snapshot_v1)" ]] || fail "v1 evidence mutated"
before=$(invocation_count "$COUNT_FILE")
v2_primary "$FAKE_AUTO" "$TMP_ROOT/state-reuse" >/dev/null 2>&1 && fail "single-use identity reused"
assert_eq "$before" "$(invocation_count "$COUNT_FILE")" "reuse added calls"

v2_make_evaluator_evidence
rollback_reject() {
  local label=$1 before
  before=$(invocation_count "$COUNT_FILE")
  v2_rollback "$FAKE_AUTO" "$TMP_ROOT/state-rollback-$label" >/dev/null 2>&1 && fail "$label rollback admitted"
  assert_eq "$before" "$(invocation_count "$COUNT_FILE")" "$label added provider calls"
  [[ ! -e "$CLAIM/rollback-reservation" ]] || fail "$label created nested rollback claim"
}
cp "$OUTPUT/gpt-quality-ledger-v2.json" "$TMP_ROOT/quality.saved"
rm "$OUTPUT/gpt-quality-ledger-v2.json"; rollback_reject missing-evidence
mv "$TMP_ROOT/quality.saved" "$OUTPUT/gpt-quality-ledger-v2.json"
cp "$OUTPUT/gpt-primary-evaluation-summary-v2.json" "$TMP_ROOT/summary.saved"
jq '.hashes.security_receipts_sha256=("sha256:"+("0"*64))' "$TMP_ROOT/summary.saved" > "$OUTPUT/gpt-primary-evaluation-summary-v2.json"
v2_sidecar "$OUTPUT/gpt-primary-evaluation-summary-v2.json"; rollback_reject broken-link
mv "$TMP_ROOT/summary.saved" "$OUTPUT/gpt-primary-evaluation-summary-v2.json"; v2_sidecar "$OUTPUT/gpt-primary-evaluation-summary-v2.json"
cp "$OUTPUT/gpt-rollback-input-v2.json" "$TMP_ROOT/rollback-input.saved"
jq '.rollout.experiment_id="ute-gpt-rollback-v1"' "$TMP_ROOT/rollback-input.saved" > "$OUTPUT/gpt-rollback-input-v2.json"
v2_sidecar "$OUTPUT/gpt-rollback-input-v2.json"; rollback_reject mixed-generation
mv "$TMP_ROOT/rollback-input.saved" "$OUTPUT/gpt-rollback-input-v2.json"; v2_sidecar "$OUTPUT/gpt-rollback-input-v2.json"
wrong=$(jq -r '.tasks[]|select(.task_id=="ute-corpus-v1-009")|.task_hash' "$OUTPUT/gpt-canary-cohort-v1.json"); cp "$OUTPUT/gpt-rollout-high-input-v2.json" "$TMP_ROOT/high-input.saved"
jq --arg wrong "$wrong" '.rollout.experiment_id=("ute-gpt-rollout-high-v2:ute-corpus-v1-009:"+$wrong)' "$TMP_ROOT/high-input.saved" > "$OUTPUT/gpt-rollout-high-input-v2.json"; v2_sidecar "$OUTPUT/gpt-rollout-high-input-v2.json"
rollback_reject wrong-high-task
mv "$TMP_ROOT/high-input.saved" "$OUTPUT/gpt-rollout-high-input-v2.json"; v2_sidecar "$OUTPUT/gpt-rollout-high-input-v2.json"
v2_phase_tamper rollback phase-rollback
v2_rollback "$FAKE_AUTO" "$TMP_ROOT/state-rollback-valid" >/dev/null
assert_eq 63 "$(invocation_count "$COUNT_FILE")" "primary plus rollback provider calls"
rollback_reservation="sha256:$(v2_sha "$OUTPUT/gpt-rollback-reservation-v2.json")"
summary_hash="sha256:$(v2_sha "$OUTPUT/gpt-primary-evaluation-summary-v2.json")"
jq -e --arg reservation "$reservation" --arg rollback "$rollback_reservation" --arg summary "$summary_hash" '
  .completed==true and .observed_calls==5 and .reservation_sha256==$reservation and
  .rollback_reservation_sha256==$rollback and .evaluator_summary_sha256==$summary' "$OUTPUT/gpt-rollback-call-ledger-v2.json" >/dev/null ||
  fail "rollback ledger reservation binding"
v2_nested_contract || fail "valid nested reservation contract"
nested="$OUTPUT/gpt-rollback-reservation-v2.json"; nested_before="$(v2_sha "$nested")"
v2_nested_contract reserve >/dev/null 2>&1 && fail "nested reservation clobbered"
[[ "$nested_before" == "$(v2_sha "$nested")" && "$(v2_sha "$nested")" == "$(v2_sha "$CLAIM/rollback-reservation/reservation.json")" ]] || fail "nested runtime copy"
cp "$nested" "$TMP_ROOT/nested.saved"; cp "${nested%.json}.sha256" "$TMP_ROOT/nested-sidecar.saved"
rm "${nested%.json}.sha256"; v2_nested_contract >/dev/null 2>&1 && fail "missing nested sidecar accepted"
cp "$TMP_ROOT/nested-sidecar.saved" "${nested%.json}.sha256"
jq '.calls_reserved=4' "$TMP_ROOT/nested.saved" > "$nested"; v2_sidecar "$nested"
v2_nested_contract >/dev/null 2>&1 && fail "tampered nested reservation accepted"
cp "$TMP_ROOT/nested.saved" "$nested"; cp "$TMP_ROOT/nested-sidecar.saved" "${nested%.json}.sha256"
assert_eq 63 "$(invocation_count "$COUNT_FILE")" "nested reservation tests added calls"

printf '%s\n' "ute codex canary v2 hermetic admission tests: PASS"
