#!/usr/bin/env bash

write_ledger_sidecar() {
  local file=$1 sidecar="${1%.json}.sha256" tmp="${1%.json}.sha256.tmp.$$"
  [[ -f "$file" && ! -L "$file" && ! -e "$sidecar" && ! -L "$sidecar" ]] || return 1
  printf '%s  %s\n' "$(sha256_file "$file")" "$(basename "$file")" > "$tmp" || return 1
  chmod 600 "$tmp"
  ln "$tmp" "$sidecar" || { rm -f "$tmp"; return 1; }
  rm -f "$tmp"
}

v2_validate_evaluator_security_quality() {
  local security=$1 quality=$2 row expected actual
  jq -e --slurpfile c "$COHORT_FILE" '.version==1 and .receipt_count==14 and (.receipts|length)==14 and
    ([.receipts[]|(.task_id+":"+.arm)]|unique|length)==14 and ([.receipts[].receipt_sha256]|unique|length)==14 and
    ([.receipts[].task_id]|unique|sort)==([$c[0].tasks[].task_id]|sort) and all(.receipts[];. as $r |
      any($c[0].tasks[];.task_id==$r.task_id and .pair_order==$r.pair_order and .risk_tier==$r.risk_tier) and
      (.arm=="A" or .arm=="B") and .security_call.role=="security-auditor" and .security_call.effort=="max" and
      .security_call.verdict=="PASS" and .security_call.finding_count==0 and .security_call.tool_calls==0 and
      .security_call.usage_status=="actual" and .security_call.raw_total_tokens>0 and
      .patch_evidence.expected_sha256==.patch_evidence.observed_sha256 and .patch_evidence.safe_modes==true and
      .patch_evidence.git_diff_check=="PASS" and .verification.exit_code==0 and .verification.status=="PASS")' \
    "$security" >/dev/null || return 1
  while IFS= read -r row; do
    expected=$(printf '%s\n' "$row" | jq -r '.receipt_sha256')
    actual="sha256:$(printf '%s\n' "$row" | jq -cS 'del(.receipt_sha256)' | shasum -a 256 | awk '{print $1}')"
    [[ "$actual" == "$expected" ]] || return 1
  done < <(jq -c '.receipts[]' "$security")
  jq -e --slurpfile s "$security" --slurpfile c "$COHORT_FILE" '.version==1 and .row_count==7 and
    (.outcomes|length)==7 and ([.outcomes[].task_id]|unique|sort)==([$c[0].tasks[].task_id]|sort) and
    all(.outcomes[];. as $o | any($c[0].tasks[];.task_id==$o.task_id and .task_hash==$o.task_hash and
      .risk_tier==$o.risk_tier and .oracle_hash==$o.expected_oracle_hash) and
      .expected_oracle_hash==.baseline_observed_oracle_hash and .expected_oracle_hash==.candidate_observed_oracle_hash and
      .baseline_verification_exit_code==0 and .candidate_verification_exit_code==0 and
      .baseline_security_status=="PASS" and .candidate_security_status=="PASS" and
      any($s[0].receipts[];.task_id==$o.task_id and .arm=="A" and .receipt_sha256==$o.baseline_security_receipt_hash) and
      any($s[0].receipts[];.task_id==$o.task_id and .arm=="B" and .receipt_sha256==$o.candidate_security_receipt_hash))' \
    "$quality" >/dev/null
}

v2_validate_rollout_evidence() {
  local aggregate=$1 ai ar hi hr ci cr input result ri rr t005 t006 t009 ba hraw craw hdigest cdigest
  ai="$EVIDENCE_DIR/gpt-rollout-audit-input-v2.json"; ar="$EVIDENCE_DIR/gpt-rollout-audit-result-v2.json"
  hi="$EVIDENCE_DIR/gpt-rollout-high-input-v2.json"; hr="$EVIDENCE_DIR/gpt-rollout-high-result-v2.json"
  ci="$EVIDENCE_DIR/gpt-rollout-critical-input-v2.json"; cr="$EVIDENCE_DIR/gpt-rollout-critical-result-v2.json"
  input="$EVIDENCE_DIR/gpt-efficiency-input-v2.json"; result="$EVIDENCE_DIR/gpt-efficiency-result-v2.json"
  ri="$EVIDENCE_DIR/gpt-rollback-input-v2.json"; rr="$EVIDENCE_DIR/gpt-rollback-result-v2.json"
  t005=$(jq -er '.tasks[]|select(.task_id=="ute-corpus-v1-005")|.task_hash' "$COHORT_FILE") || return 1
  t006=$(jq -er '.tasks[]|select(.task_id=="ute-corpus-v1-006")|.task_hash' "$COHORT_FILE") || return 1
  t009=$(jq -er '.tasks[]|select(.task_id=="ute-corpus-v1-009")|.task_hash' "$COHORT_FILE") || return 1
  ba=$(audit_bucket "$t005" "$POLICY_HASH") || return 1
  hraw="ute-gpt-rollout-high-v2:ute-corpus-v1-006:$t006"
  craw="ute-gpt-rollout-critical-v2:ute-corpus-v1-009:$t009"
  hdigest="sha256:$(printf '%s' "$hraw" | shasum -a 256 | awk '{print $1}')"
  cdigest="sha256:$(printf '%s' "$craw" | shasum -a 256 | awk '{print $1}')"
  jq -e --arg policy "$POLICY_HASH" --arg config "$CONFIG_HASH" --arg a "$t005" --arg h "$t006" --arg c "$t009" \
    --arg hraw "$hraw" --arg craw "$craw" \
    --slurpfile ai "$ai" --slurpfile hi "$hi" --slurpfile ci "$ci" '
    .version==1 and (.calls|length)==58 and (.trials|length)==14 and (.quality_outcomes|length)==7 and
    .rollout.experiment_id=="ute-gpt-primary-v2" and .rollout.policy_hash==$policy and .rollout.config_hash==$config and
    all([$ai[0],$hi[0],$ci[0]][];.rollout.policy_hash==$policy and .rollout.config_hash==$config and .rollout.full_depth==true) and
    $ai[0].rollout.experiment_id=="ute-gpt-rollout-audit-v2" and $ai[0].rollout.risk_tier=="medium" and
    $ai[0].rollout.audit_task_hash==$a and $ai[0].rollout.audit_rate_percent==100 and
    $hi[0].rollout.experiment_id==$hraw and $hi[0].rollout.risk_tier=="high" and
    ($hi[0].rollout|has("audit_task_hash")|not) and $hi[0].rollout.audit_rate_percent==0 and
    $ci[0].rollout.experiment_id==$craw and $ci[0].rollout.risk_tier=="critical" and
    ($ci[0].rollout|has("audit_task_hash")|not) and $ci[0].rollout.audit_rate_percent==0' "$input" >/dev/null || return 1
  jq -e --argjson ba "$ba" --arg hdigest "$hdigest" --arg cdigest "$cdigest" \
    --slurpfile a "$ar" --slurpfile h "$hr" --slurpfile c "$cr" '
    .comparison.median_paired_raw_reduction_pct>=25 and .comparison.paired_task_count==7 and
    .quality.complete==true and .quality.consistent==true and .promotion.rollout_decision=="ELIGIBLE_NEXT_CANARY" and
    $a[0].promotion.high_critical_regressions==0 and $a[0].rollout_receipt.decision=="AUDIT" and
    $a[0].rollout_receipt.active_profile=="full_ultra" and $a[0].rollout_receipt.full_depth==true and
    $a[0].rollout_receipt.risk_tier=="medium" and
    $a[0].rollout_receipt.audit_selection=={selected:true,bucket:$ba,rate_percent:100,algorithm:"sha256_mod_100_v1"} and
    all([$h[0],$c[0]][];.promotion.high_critical_regressions==0 and .rollout_receipt.decision=="CANARY" and
      .rollout_receipt.active_profile=="full_ultra" and .rollout_receipt.full_depth==true and
      .rollout_receipt.selection_reason=="risk_requires_full_depth") and
    $h[0].rollout_receipt.risk_tier=="high" and
    $h[0].rollout_receipt.experiment_id==$hdigest and
    $h[0].rollout_receipt.audit_selection=={selected:false,bucket:0,rate_percent:0,algorithm:""} and
    $c[0].rollout_receipt.risk_tier=="critical" and
    $c[0].rollout_receipt.experiment_id==$cdigest and
    $c[0].rollout_receipt.audit_selection=={selected:false,bucket:0,rate_percent:0,algorithm:""}' \
    "$result" >/dev/null || return 1
  jq -e --arg pi "$(v2_sha_uri "$input")" --arg pr "$(v2_sha_uri "$result")" --arg ai "$(v2_sha_uri "$ai")" \
    --arg ar "$(v2_sha_uri "$ar")" --arg hi "$(v2_sha_uri "$hi")" --arg hr "$(v2_sha_uri "$hr")" \
    --arg ci "$(v2_sha_uri "$ci")" --arg cr "$(v2_sha_uri "$cr")" --arg a "$t005" --arg h "$t006" --arg c "$t009" \
    --arg hraw "$hraw" --arg craw "$craw" --arg hdigest "$hdigest" --arg cdigest "$cdigest" --argjson ba "$ba" '
    .version==1 and .evidence_kind=="gpt_rollout_receipts" and
    .primary.input_sha256==$pi and .primary.result_sha256==$pr and .audit.input_sha256==$ai and .audit.result_sha256==$ar and
    .high.input_sha256==$hi and .high.result_sha256==$hr and .critical.input_sha256==$ci and .critical.result_sha256==$cr and
    .audit.sentinel=={task_id:"ute-corpus-v1-005",task_hash:$a,audit_rate_percent:100,selected:true} and
    .high.sentinel=={task_id:"ute-corpus-v1-006",task_hash:$h,audit_rate_percent:0,selected:false} and
    .critical.sentinel=={task_id:"ute-corpus-v1-009",task_hash:$c,audit_rate_percent:0,selected:false} and
    .high.experiment_identity==$hraw and .high.experiment_identity_sha256==$hdigest and
    .critical.experiment_identity==$craw and .critical.experiment_identity_sha256==$cdigest and
    .audit.audit_selection=={selected:true,bucket:$ba,rate_percent:100,algorithm:"sha256_mod_100_v1"} and
    .high.audit_selection=={selected:false,bucket:0,rate_percent:0,algorithm:""} and
    .critical.audit_selection=={selected:false,bucket:0,rate_percent:0,algorithm:""}' \
    "$aggregate" >/dev/null || return 1
  jq -e '.policy_parity_passed==false and .candidate_behavior_active==true and
    .rollout.experiment_id=="ute-gpt-rollback-v2" and .rollout.full_depth==false' "$ri" >/dev/null &&
    jq -e '.promotion.rollout_decision=="ROLLBACK" and .rollout_receipt.decision=="ROLLBACK" and
      .rollout_receipt.active_profile=="full_ultra" and .rollout_receipt.full_depth==true' "$rr" >/dev/null
}

v2_validate_summary_manifest() {
  local summary=$1 primary=$2 security=$3 quality=$4 rollout=$5 applied=$6 result=$7 stem file
  jq -e --arg auth "$V2_AUTHORIZATION_IDENTITY_SHA256" --arg primary "$primary" \
    --arg security "$security" --arg quality "$quality" --arg rollout "$rollout" --arg applied "$applied" \
    --slurpfile r "$EVIDENCE_DIR/gpt-rollout-receipts-v2.json" --slurpfile x "$result" '
    .admission_generation=="v2" and .authorization_identity_sha256==$auth and .primary_calls==58 and
    .median_paired_raw_reduction_pct==$x[0].comparison.median_paired_raw_reduction_pct and
    .quality_tasks_passed==7 and .quality_tasks_expected==7 and .security_receipts_passed==14 and
    .security_receipts_expected==14 and .audit_task_005=="PASS" and .high_sentinel_006=="PASS" and
    .critical_sentinel_009=="PASS" and .evaluator_decision=="ELIGIBLE_NEXT_CANARY" and
    .applied_rollback_ready==true and .rollback_replay_status=="pending" and .replay_eligible==true and
    .provider_calls_made_by_builder==0 and .promotion_eligible==false and .activation_eligible==false and
    .implemented==false and .sentinels=={audit:$r[0].audit.sentinel,high:$r[0].high.sentinel,
      critical:$r[0].critical.sentinel} and .hashes=={primary_ledger_sha256:$primary,
      security_receipts_sha256:$security,quality_ledger_sha256:$quality,
      rollout_receipts_sha256:$rollout,applied_rollback_sha256:$applied} and
    (.evaluator_artifacts|keys|sort)==(["gpt-security-receipts-v2.json","gpt-quality-ledger-v2.json",
      "gpt-efficiency-input-v2.json","gpt-efficiency-result-v2.json","gpt-rollout-audit-input-v2.json",
      "gpt-rollout-audit-result-v2.json","gpt-rollout-high-input-v2.json","gpt-rollout-high-result-v2.json",
      "gpt-rollout-critical-input-v2.json","gpt-rollout-critical-result-v2.json","gpt-rollout-receipts-v2.json",
      "gpt-rollback-input-v2.json","gpt-rollback-result-v2.json","gpt-applied-rollback-v2.json"]|sort)' \
    "$summary" >/dev/null || return 1
  for stem in gpt-security-receipts gpt-quality-ledger gpt-efficiency-input gpt-efficiency-result \
    gpt-rollout-audit-input gpt-rollout-audit-result gpt-rollout-high-input gpt-rollout-high-result \
    gpt-rollout-critical-input gpt-rollout-critical-result gpt-rollout-receipts gpt-rollback-input \
    gpt-rollback-result gpt-applied-rollback; do
    file="$EVIDENCE_DIR/$stem-v2.json"
    jq -e --arg name "${file##*/}" --arg hash "$(v2_sha_uri "$file")" \
      '.evaluator_artifacts[$name]==$hash' "$summary" >/dev/null || return 1
  done
}

ledger_target_for() {
  local mode=$1 completed=$2 stem
  [[ "$mode" == primary ]] && stem=gpt-primary-call-ledger-v2 || stem=gpt-rollback-call-ledger-v2
  [[ "$completed" == true ]] && printf '%s/%s.json\n' "$OUTPUT" "$stem" || \
    printf '%s/%s.partial-fail.json\n' "$OUTPUT" "$stem"
}

persist_ledger() {
  local mode=$1 completed=$2 failure_code=$3 rows=$4 target calls_file kind planned worst evaluation
  local primary_raw=0 combined_raw tmp
  target=$(ledger_target_for "$mode" "$completed"); calls_file="$TEMP_ROOT/ledger-calls.json"
  jq -s '.' "$rows" > "$calls_file"
  if [[ "$mode" == primary ]]; then
    kind=gpt_codex_primary_call_ledger; planned=58; worst=1332000
    [[ "$completed" == true ]] && evaluation=true || evaluation=false
  else
    kind=applied_rollback_replay; planned=5; worst=114000; evaluation=false
    primary_raw=$(jq -r '.observed_raw_total_tokens' "$PRIMARY_LEDGER")
  fi
  combined_raw=$(jq --argjson primary "$primary_raw" '[.[].result.raw_total_tokens // 0]|add+$primary' "$calls_file")
  tmp="$OUTPUT/.$(basename "$target").tmp.$$"
  jq -n --slurpfile calls "$calls_file" --arg kind "$kind" --arg mode "$mode" \
    --argjson completed "$completed" --argjson evaluation "$evaluation" --arg failure "$failure_code" \
    --argjson planned "$planned" --argjson worst "$worst" --arg auth "$V2_AUTHORIZATION_IDENTITY_SHA256" \
    --arg corpus "$CORPUS_HASH" --arg cohort "$COHORT_HASH" --arg policy "$POLICY_HASH" \
    --arg config "$CONFIG_HASH" --arg schema "$SCHEMA_HASH" --arg auto_hash "$AUTO_SHA" \
    --arg auto_version "$AUTO_VERSION" --arg codex_version "$CODEX_VERSION" \
    --arg codex_version_hash "$CODEX_VERSION_HASH" --arg rollback_hash "${ROLLBACK_RECEIPT_HASH:-}" \
    --arg primary_hash "${PRIMARY_LEDGER_HASH:-}" --arg reservation "$V2_RESERVATION_SHA256" \
    --arg rollback_reservation "${V2_ROLLBACK_RESERVATION_SHA256:-}" \
    --arg evaluator_summary "${V2_EVALUATOR_SUMMARY_SHA256:-}" \
    --argjson combined "$combined_raw" '
    ($calls[0]//[]) as $rows |
    {version:1,admission_generation:"v2",authorization_identity_sha256:$auth,reservation_sha256:$reservation,
     spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",evidence_kind:$kind,mode:$mode,
     completed:$completed,evaluation_eligible:$evaluation,promotion_eligible:false,
     circuit_breaker:(if $completed then "CLOSED" else "OPEN" end),
     failure_code:(if $completed then null else $failure end),attempted_calls:($rows|length),
     successful_calls:([$rows[]|select(.result.status=="success" and .result.verdict=="PASS")]|length),
     planned_calls:$planned,observed_calls:($rows|length),planned_worst_case_raw_tokens:$worst,
     observed_raw_total_tokens:([$rows[].result.raw_total_tokens//0]|add//0),
     combined_primary_and_replay_observed_raw_tokens:$combined,
     authorization:{provider_call_cap:64,raw_token_cap:1500000,concurrency:1,retries:0,
       primary_calls_reserved:58,rollback_calls_reserved:5,total_calls_reserved:63,
       planned_worst_case_raw_tokens:1446000},
     identity:{provider:"codex",model:"gpt-5.6-sol",provider_version:"0.144.1",
       model_version:"gpt-5.6-sol",effort_policy:"codex_review_xhigh_security_max_v1",
       cache_stratum:"provider-managed-stable-prefix-v1",corpus_sha256:$corpus,cohort_sha256:$cohort,
       policy_sha256:$policy,config_sha256:$config,verdict_schema_sha256:$schema,
       auto_executable_sha256:$auto_hash,auto_version:$auto_version,codex_cli_version:$codex_version,
       codex_cli_version_receipt_sha256:$codex_version_hash},
     applied_rollback_receipt_sha256:(if $mode=="rollback" then $rollback_hash else null end),
     primary_ledger_sha256:(if $mode=="rollback" then $primary_hash else null end),calls:$rows,
     privacy:{raw_prompt_retained:false,raw_patch_retained:false,raw_response_retained:false,
       provider_stdout_stderr_retained:false,isolated_telemetry_retained:false,
       absolute_paths_retained:false}} + (if $mode=="rollback" then
       {rollback_reservation_sha256:$rollback_reservation,evaluator_summary_sha256:$evaluator_summary} else {} end)
  ' > "$tmp"
  chmod 600 "$tmp"
  if ! scan_retained_ledger "$tmp"; then rm -f "$tmp"; return 1; fi
  ln "$tmp" "$target" || { rm -f "$tmp"; return 1; }
  rm -f "$tmp"; write_ledger_sidecar "$target" || return 1; PERSISTED_LEDGER=$target
}

persist_retention_failure() {
  local mode=$1 attempted=$2 target tmp
  target=$(ledger_target_for "$mode" false); tmp="$OUTPUT/.$(basename "$target").tmp.$$"
  jq -n --arg mode "$mode" --arg auth "$V2_AUTHORIZATION_IDENTITY_SHA256" \
    --arg reservation "$V2_RESERVATION_SHA256" --argjson attempted "$attempted" \
    --arg rollback_reservation "${V2_ROLLBACK_RESERVATION_SHA256:-}" \
    --arg evaluator_summary "${V2_EVALUATOR_SUMMARY_SHA256:-}" \
    '{version:1,admission_generation:"v2",authorization_identity_sha256:$auth,reservation_sha256:$reservation,
    spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",evidence_kind:"retention_scan_failure",mode:$mode,
    completed:false,evaluation_eligible:false,promotion_eligible:false,circuit_breaker:"OPEN",
    failure_code:"retained_field_scan",attempted_calls:$attempted,successful_calls:0,calls:[],
    privacy:{raw_artifacts_retained:false}} + (if $mode=="rollback" then
    {rollback_reservation_sha256:$rollback_reservation,evaluator_summary_sha256:$evaluator_summary} else {} end)' > "$tmp"
  chmod 600 "$tmp"; scan_retained_ledger "$tmp" || { rm -f "$tmp"; return 1; }
  ln "$tmp" "$target" || { rm -f "$tmp"; return 1; }
  rm -f "$tmp"; write_ledger_sidecar "$target" || return 1; PERSISTED_LEDGER=$target
}

scan_retained_ledger() {
  local ledger=$1 value
  jq -e '[..|objects|keys[]|select(.=="prompt" or .=="description" or .=="patch" or
    .=="raw_output" or .=="stdout" or .=="stderr" or .=="session_id" or
    .=="environment" or .=="cwd" or .=="path")]|length==0' "$ledger" >/dev/null || return 1
  while IFS= read -r value; do
    [[ "$value" != *UTE-RAW-PROMPT-* && "$value" != *FAKE-RAW-PROVIDER-BODY* ]] || return 1
    [[ "$value" != "$REPO" && "$value" != "$STATE" && "$value" != "$AUTO" ]] || return 1
    [[ -z "${PRIMARY_LEDGER:-}" || "$value" != "$PRIMARY_LEDGER" ]] || return 1
    [[ -z "${ROLLBACK_RECEIPT:-}" || "$value" != "$ROLLBACK_RECEIPT" ]] || return 1
  done < <(jq -r '..|strings' "$ledger")
}

ensure_ledger_targets_absent() {
  local mode=$1 stem suffix file
  [[ "$mode" == primary ]] && stem=gpt-primary-call-ledger-v2 || stem=gpt-rollback-call-ledger-v2
  for suffix in .json .sha256 .partial-fail.json .partial-fail.sha256; do
    file="$OUTPUT/$stem$suffix"; [[ ! -e "$file" && ! -L "$file" ]] || return 1
  done
}
