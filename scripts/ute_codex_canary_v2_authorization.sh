#!/usr/bin/env bash
v2_authorization_root() {
  local install_root
  install_root=$(cd "$SCRIPT_DIR/.." && pwd -P)
  printf '%s/.autopus/runtime/ute-full-evaluation-authorizations\n' "$install_root"
}
v2_authorization_claim_dir() {
  printf '%s/%s\n' "$(v2_authorization_root)" "${V2_AUTHORIZATION_IDENTITY_SHA256#sha256:}"
}
v2_require_canonical_output() { [[ "$OUTPUT" == "$EVIDENCE_DIR" ]]; }

v2_existing_primary_consumption() {
  local file
  for file in "$EVIDENCE_DIR/gpt-primary-call-ledger-v2.json" \
    "$EVIDENCE_DIR/gpt-primary-call-ledger-v2.partial-fail.json"; do
    [[ ! -e "$file" && ! -L "$file" ]] || return 0
  done
  return 1
}

v2_prepare_authorization_root() {
  local root parent grandparent
  root=$(v2_authorization_root); parent=$(dirname "$root"); grandparent=$(dirname "$parent")
  [[ -d "$grandparent" && ! -L "$grandparent" && -O "$grandparent" ]] || return 1
  if [[ -e "$parent" || -L "$parent" ]]; then
    [[ -d "$parent" && ! -L "$parent" && -O "$parent" ]] || return 1
  else
    mkdir "$parent" || return 1; chmod 700 "$parent"
  fi
  if [[ -e "$root" || -L "$root" ]]; then
    [[ -d "$root" && ! -L "$root" && -O "$root" ]] || return 1
  else
    mkdir "$root" || return 1; chmod 700 "$root"
  fi
  V2_AUTHORIZATION_ROOT=$root
}

v2_validate_authorization_receipt() {
  local receipt="$EVIDENCE_DIR/gpt-full-evaluation-authorization-v2.json" preflight
  [[ -f "$receipt" && ! -L "$receipt" && -f "${receipt%.json}.sha256" &&
     ! -L "${receipt%.json}.sha256" ]] || return 1
  verify_named_sidecar "$receipt" "${receipt%.json}.sha256" || return 1
  preflight=$(v2_sha_uri "$EVIDENCE_DIR/gpt-canary-preflight-v2.json")
  jq -e --arg auth "$V2_AUTHORIZATION_IDENTITY_SHA256" --arg policy "$POLICY_HASH" \
    --arg config "$CONFIG_HASH" --arg preflight "$preflight" '
    (keys|sort)==(["activation","authorization_identity_sha256","authorization_source","authorized_at",
      "concurrency","config_sha256","decision","evidence_kind","implemented","model","planned_worst_case_raw_tokens",
      "policy_sha256","preflight_sha256","primary_calls","promotion","provider","provider_call_cap","provider_execution_started",
      "raw_token_cap","retries","rollback_calls","single_use","spec_id","total_calls","version"]|sort) and
    .version==2 and .spec_id=="SPEC-ADK-ULTRA-EFFICIENCY-001" and
    .evidence_kind=="gpt_codex_full_evaluation_authorization" and
    .decision=="EXPLICIT_FULL_EVALUATION_AUTHORIZATION_GRANTED" and
    .authorization_source=="user_exact_identity_confirmation" and
    .authorization_identity_sha256==$auth and .policy_sha256==$policy and .config_sha256==$config and
    .preflight_sha256==$preflight and .provider=="codex" and .model=="gpt-5.6-sol" and
    .provider_call_cap==64 and .raw_token_cap==1500000 and .primary_calls==58 and
    .rollback_calls==5 and .total_calls==63 and .planned_worst_case_raw_tokens==1446000 and
    .concurrency==1 and .retries==0 and .single_use==true and .provider_execution_started==false and
    .activation==false and .promotion==false and .implemented==false and
    (.authorized_at|test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(Z|[+-][0-9]{2}:[0-9]{2})$"))
  ' "$receipt" >/dev/null
}

v2_write_reservation_json() {
  local target=$1 tmp="$1.tmp.$$"
  jq -n --arg auth "$V2_AUTHORIZATION_IDENTITY_SHA256" --arg policy "$POLICY_HASH" \
    --arg config "$CONFIG_HASH" --arg auto "$AUTO_SHA" --arg version "$AUTO_VERSION" '{version:1,
    spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",evidence_kind:"gpt_full_evaluation_reservation",
    admission_generation:"v2",authorization_identity_sha256:$auth,policy_sha256:$policy,
    config_sha256:$config,auto_executable_sha256:$auto,auto_version:$version,
    primary_calls_reserved:58,rollback_calls_reserved:5,total_calls_reserved:63,
    planned_worst_case_raw_tokens:1446000,provider_call_cap:64,raw_token_cap:1500000,
    concurrency:1,retries:0,state:"CONSUMED_ON_RESERVATION"}' > "$tmp" || return 1
  chmod 600 "$tmp"
  ln "$tmp" "$target" || { rm -f "$tmp"; return 1; }
  rm -f "$tmp"
}

v2_publish_primary_reservation() {
  local claim=$1 claim_receipt="$1/reservation.json" canonical sidecar
  canonical="$EVIDENCE_DIR/gpt-full-evaluation-reservation-v2.json"
  sidecar="${canonical%.json}.sha256"
  [[ ! -e "$canonical" && ! -L "$canonical" && ! -e "$sidecar" && ! -L "$sidecar" ]] || return 1
  v2_write_reservation_json "$claim_receipt" || return 1
  write_ledger_sidecar "$claim_receipt" || return 1
  ln "$claim_receipt" "$canonical" || return 1
  write_ledger_sidecar "$canonical" || return 1
  V2_RESERVATION_SHA256=$(v2_sha_uri "$canonical")
  [[ "$(v2_sha_uri "$claim_receipt")" == "$V2_RESERVATION_SHA256" ]]
}

v2_reserve_primary_authorization() {
  local claim
  v2_require_canonical_output || return 1
  v2_validate_authorization_receipt || return 1
  v2_existing_primary_consumption && return 1
  v2_prepare_authorization_root || return 1
  claim=$(v2_authorization_claim_dir)
  mkdir "$claim" || return 1
  chmod 700 "$claim"
  v2_publish_primary_reservation "$claim" || return 1
  V2_AUTHORIZATION_CLAIM_DIR=$claim
}

v2_validate_primary_reservation() {
  local claim runtime canonical
  v2_prepare_authorization_root || return 1
  claim=$(v2_authorization_claim_dir); runtime="$claim/reservation.json"
  canonical="$EVIDENCE_DIR/gpt-full-evaluation-reservation-v2.json"
  [[ -d "$claim" && ! -L "$claim" && -O "$claim" ]] || return 1
  [[ -f "$runtime" && ! -L "$runtime" && -f "$canonical" && ! -L "$canonical" ]] || return 1
  verify_named_sidecar "$runtime" "$claim/reservation.sha256" || return 1
  verify_named_sidecar "$canonical" "${canonical%.json}.sha256" || return 1
  [[ "$(sha256_file "$runtime")" == "$(sha256_file "$canonical")" ]] || return 1
  jq -e --arg auth "$V2_AUTHORIZATION_IDENTITY_SHA256" --arg policy "$POLICY_HASH" \
    --arg config "$CONFIG_HASH" --arg auto "$AUTO_SHA" --arg version "$AUTO_VERSION" '
    .version==1 and .admission_generation=="v2" and .authorization_identity_sha256==$auth and
    .policy_sha256==$policy and .config_sha256==$config and .auto_executable_sha256==$auto and
    .auto_version==$version and .primary_calls_reserved==58 and .rollback_calls_reserved==5 and
    .total_calls_reserved==63 and .planned_worst_case_raw_tokens==1446000 and
    .provider_call_cap==64 and .raw_token_cap==1500000 and .concurrency==1 and .retries==0 and
    .state=="CONSUMED_ON_RESERVATION"
  ' "$canonical" >/dev/null || return 1
  V2_RESERVATION_SHA256=$(v2_sha_uri "$canonical")
  V2_AUTHORIZATION_CLAIM_DIR=$claim
}

v2_validate_rollback_inputs() {
  local receipt=$1 expected_hash=$2 primary_hash=$3 stem file
  local security quality result rollout rollback_result summary
  local security_hash quality_hash rollout_hash applied_hash
  local stems=(gpt-security-receipts-v2 gpt-quality-ledger-v2 gpt-efficiency-input-v2
    gpt-efficiency-result-v2 gpt-rollout-audit-input-v2 gpt-rollout-audit-result-v2
    gpt-rollout-high-input-v2 gpt-rollout-high-result-v2 gpt-rollout-critical-input-v2
    gpt-rollout-critical-result-v2 gpt-rollout-receipts-v2 gpt-rollback-input-v2
    gpt-rollback-result-v2 gpt-applied-rollback-v2 gpt-primary-evaluation-summary-v2)
  [[ "$receipt" == "$EVIDENCE_DIR/gpt-applied-rollback-v2.json" ]] || return 1
  for stem in "${stems[@]}"; do
    file="$EVIDENCE_DIR/$stem.json"
    [[ -s "$file" && ! -L "$file" && -f "${file%.json}.sha256" && ! -L "${file%.json}.sha256" ]] || return 1
    verify_named_sidecar "$file" "${file%.json}.sha256" || return 1
    jq empty "$file" >/dev/null || return 1
  done
  [[ "$expected_hash" == "$(v2_sha_uri "$receipt")" ]] || return 1
  security="$EVIDENCE_DIR/gpt-security-receipts-v2.json"
  quality="$EVIDENCE_DIR/gpt-quality-ledger-v2.json"
  result="$EVIDENCE_DIR/gpt-efficiency-result-v2.json"
  rollout="$EVIDENCE_DIR/gpt-rollout-receipts-v2.json"
  rollback_result="$EVIDENCE_DIR/gpt-rollback-result-v2.json"
  summary="$EVIDENCE_DIR/gpt-primary-evaluation-summary-v2.json"
  security_hash=$(v2_sha_uri "$security"); quality_hash=$(v2_sha_uri "$quality")
  rollout_hash=$(v2_sha_uri "$rollout"); applied_hash=$(v2_sha_uri "$receipt")
  v2_validate_evaluator_security_quality "$security" "$quality" || return 1
  v2_validate_rollout_evidence "$rollout" || return 1
  jq -e --arg policy "$POLICY_HASH" --arg config "$CONFIG_HASH" --arg primary "$primary_hash" \
    --arg logical "$(v2_sha_uri "$rollback_result")" '
    .version==1 and .evidence_kind=="applied_policy_rollback" and .decision=="ROLLBACK" and
    .applied==true and .active_profile=="full_ultra" and .atomic_replace==true and .fsync_completed==true and
    .state_readback=="full_ultra" and .policy_sha256==$policy and .config_sha256==$config and
    .primary_ledger_sha256==$primary and .logical_rollback_result_sha256==$logical and
    .state_readback_sha256==.after_binding_sha256 and .before_binding_sha256!=.after_binding_sha256
  ' "$receipt" >/dev/null || return 1
  v2_validate_summary_manifest "$summary" "$primary_hash" "$security_hash" "$quality_hash" \
    "$rollout_hash" "$applied_hash" "$result" || return 1
  V2_EVALUATOR_SUMMARY_SHA256=$(v2_sha_uri "$summary")
}

v2_validate_rollback_reservation() {
  local claim runtime canonical
  claim="$V2_AUTHORIZATION_CLAIM_DIR/rollback-reservation"
  runtime="$claim/reservation.json"; canonical="$EVIDENCE_DIR/gpt-rollback-reservation-v2.json"
  [[ -d "$claim" && ! -L "$claim" && -f "$runtime" && ! -L "$runtime" &&
     -f "$claim/reservation.sha256" && ! -L "$claim/reservation.sha256" &&
     -f "$canonical" && ! -L "$canonical" && -f "${canonical%.json}.sha256" &&
     ! -L "${canonical%.json}.sha256" ]] || return 1
  verify_named_sidecar "$runtime" "$claim/reservation.sha256" || return 1
  verify_named_sidecar "$canonical" "${canonical%.json}.sha256" || return 1
  [[ "$(sha256_file "$runtime")" == "$(sha256_file "$canonical")" ]] || return 1
  jq -e --arg auth "$V2_AUTHORIZATION_IDENTITY_SHA256" --arg reservation "$V2_RESERVATION_SHA256" \
    --arg primary "$PRIMARY_LEDGER_HASH" --arg applied "$ROLLBACK_RECEIPT_HASH" \
    --arg summary "$V2_EVALUATOR_SUMMARY_SHA256" '
    (keys|sort)==(["admission_generation","applied_rollback_receipt_sha256",
      "authorization_identity_sha256","calls_reserved","concurrency","evaluator_summary_sha256",
      "evidence_kind","primary_ledger_sha256","primary_reservation_sha256","raw_tokens_reserved",
      "retries","spec_id","state","version"]|sort) and .version==1 and
    .spec_id=="SPEC-ADK-ULTRA-EFFICIENCY-001" and .evidence_kind=="gpt_rollback_replay_reservation" and
    .admission_generation=="v2" and .authorization_identity_sha256==$auth and
    .primary_reservation_sha256==$reservation and .primary_ledger_sha256==$primary and
    .applied_rollback_receipt_sha256==$applied and .evaluator_summary_sha256==$summary and
    .calls_reserved==5 and .raw_tokens_reserved==114000 and .concurrency==1 and .retries==0 and
    .state=="CONSUMED_ON_RESERVATION"' "$canonical" >/dev/null || return 1
  V2_ROLLBACK_RESERVATION_SHA256=$(v2_sha_uri "$canonical")
}

v2_reserve_rollback_authorization() {
  local claim receipt tmp canonical sidecar
  v2_require_canonical_output || return 1
  v2_validate_primary_reservation || return 1
  claim="$V2_AUTHORIZATION_CLAIM_DIR/rollback-reservation"
  canonical="$EVIDENCE_DIR/gpt-rollback-reservation-v2.json"; sidecar="${canonical%.json}.sha256"
  [[ ! -e "$claim" && ! -L "$claim" && ! -e "$canonical" && ! -L "$canonical" &&
     ! -e "$sidecar" && ! -L "$sidecar" ]] || return 1
  mkdir "$claim" || return 1
  chmod 700 "$claim"; receipt="$claim/reservation.json"; tmp="$claim/.reservation.tmp.$$"
  jq -n --arg auth "$V2_AUTHORIZATION_IDENTITY_SHA256" --arg primary "$PRIMARY_LEDGER_HASH" \
    --arg reservation "$V2_RESERVATION_SHA256" --arg applied "$ROLLBACK_RECEIPT_HASH" \
    --arg summary "$V2_EVALUATOR_SUMMARY_SHA256" '{version:1,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",
    evidence_kind:"gpt_rollback_replay_reservation",admission_generation:"v2",
    authorization_identity_sha256:$auth,primary_reservation_sha256:$reservation,
    primary_ledger_sha256:$primary,applied_rollback_receipt_sha256:$applied,evaluator_summary_sha256:$summary,
    calls_reserved:5,raw_tokens_reserved:114000,concurrency:1,retries:0,
    state:"CONSUMED_ON_RESERVATION"}' > "$tmp" || return 1
  chmod 600 "$tmp"; ln "$tmp" "$receipt" || { rm -f "$tmp"; return 1; }
  rm -f "$tmp"; write_ledger_sidecar "$receipt" || return 1
  ln "$receipt" "$canonical" || return 1
  write_ledger_sidecar "$canonical" || return 1
  v2_validate_rollback_reservation
}
