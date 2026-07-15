#!/usr/bin/env bash

authorization_root() {
  local install_root
  install_root=$(cd "$SCRIPT_DIR/.." && pwd -P)
  printf '%s/.autopus/runtime/ute-authorizations\n' "$install_root"
}

authorization_claim_dir() {
  printf '%s/%s\n' "$(authorization_root)" "${POLICY_HASH#sha256:}"
}

require_canonical_evidence_output() {
  [[ "$OUTPUT" == "$EVIDENCE_DIR" ]]
}

existing_primary_consumption() {
  local file sidecar aggregate
  local files=()
  shopt -s nullglob
  files=("$EVIDENCE_DIR"/gpt-primary-call-ledger*.json)
  shopt -u nullglob
  [[ ${#files[@]} -gt 0 ]] || return 1
  EXISTING_PRIMARY_VALID=false
  aggregate=$(for file in "${files[@]}"; do sha256_file "$file"; done | shasum -a 256 | awk '{print $1}')
  EXISTING_PRIMARY_LEDGER_HASH="sha256:$aggregate"
  EXISTING_PRIMARY_OBSERVED_CALLS=0
  EXISTING_PRIMARY_OBSERVED_RAW=0
  EXISTING_PRIMARY_TERMINAL_STATE=UNTRUSTED_LEDGER_PRESENT
  [[ ${#files[@]} -eq 1 ]] || return 0
  for file in "${files[@]}"; do
    sidecar="${file%.json}.sha256"
    verify_named_sidecar "$file" "$sidecar" || return 0
    jq -e --arg policy "$POLICY_HASH" '
      .spec_id == "SPEC-ADK-ULTRA-EFFICIENCY-001" and
      .identity.policy_sha256 == $policy and .attempted_calls >= 0 and
      (.completed == true or .completed == false)
    ' "$file" >/dev/null || return 0
    EXISTING_PRIMARY_VALID=true
    EXISTING_PRIMARY_LEDGER_HASH="sha256:$(sha256_file "$file")"
    EXISTING_PRIMARY_OBSERVED_CALLS=$(jq -r '.attempted_calls' "$file")
    EXISTING_PRIMARY_OBSERVED_RAW=$(jq -r '.observed_raw_total_tokens' "$file")
    if [[ "$(jq -r '.completed' "$file")" == true ]]; then
      EXISTING_PRIMARY_TERMINAL_STATE=COMPLETE
    else
      EXISTING_PRIMARY_TERMINAL_STATE=PARTIAL_FAIL
    fi
  done
  return 0
}

prepare_authorization_root() {
  local root parent grandparent
  root=$(authorization_root)
  parent=$(dirname "$root"); grandparent=$(dirname "$parent")
  [[ -d "$grandparent" && ! -L "$grandparent" && -O "$grandparent" ]] || return 1
  if [[ -e "$parent" ]]; then
    [[ -d "$parent" && ! -L "$parent" && -O "$parent" ]] || return 1
  else
    mkdir "$parent" || return 1
    chmod 700 "$parent"
  fi
  if [[ -e "$root" ]]; then
    [[ -d "$root" && ! -L "$root" && -O "$root" ]] || return 1
  else
    mkdir "$root" || return 1
    chmod 700 "$root"
  fi
  AUTHORIZATION_ROOT=$root
}

write_primary_reservation() {
  local claim=$1 tmp="$1/.reservation.tmp.$$" receipt="$1/reservation.json"
  jq -n --arg policy "$POLICY_HASH" --arg config "$CONFIG_HASH" --arg corpus "$CORPUS_HASH" \
    --arg cohort "$COHORT_HASH" --arg schema "$SCHEMA_HASH" '{version:1,
      spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",kind:"primary_authorization_reservation",
      authorization_identity:$policy,policy_sha256:$policy,config_sha256:$config,
      corpus_sha256:$corpus,cohort_sha256:$cohort,verdict_schema_sha256:$schema,
      provider:"codex",model:"gpt-5.6-sol",provider_call_cap:64,raw_token_cap:1500000,
      concurrency:1,retries:0,primary_calls_reserved:58,rollback_calls_reserved:5,
      total_calls_reserved:63,total_raw_tokens_reserved:1446000,state:"CONSUMED_ON_RESERVATION"}' > "$tmp" || return 1
  chmod 600 "$tmp"
  mv "$tmp" "$receipt" || return 1
  write_ledger_sidecar "$receipt" || return 1
}

write_reconciled_reservation() {
  local claim=$1 tmp="$1/.reservation.tmp.$$" receipt="$1/reservation.json"
  jq -n --arg policy "$POLICY_HASH" --arg config "$CONFIG_HASH" --arg corpus "$CORPUS_HASH" \
    --arg cohort "$COHORT_HASH" --arg schema "$SCHEMA_HASH" --arg source "$EXISTING_PRIMARY_LEDGER_HASH" \
    --arg terminal "$EXISTING_PRIMARY_TERMINAL_STATE" --argjson valid "$EXISTING_PRIMARY_VALID" \
    --argjson calls "$EXISTING_PRIMARY_OBSERVED_CALLS" --argjson raw "$EXISTING_PRIMARY_OBSERVED_RAW" \
    '{version:1,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",kind:"legacy_primary_consumption_reconciliation",
      authorization_identity:$policy,policy_sha256:$policy,config_sha256:$config,
      corpus_sha256:$corpus,cohort_sha256:$cohort,verdict_schema_sha256:$schema,
      provider:"codex",model:"gpt-5.6-sol",provider_call_cap:64,raw_token_cap:1500000,
      concurrency:1,retries:0,primary_calls_reserved:58,rollback_calls_reserved:5,
      total_calls_reserved:63,total_raw_tokens_reserved:1446000,state:"CONSUMED_ON_RECONCILIATION",
      source_ledger_sha256:$source,source_ledger_valid:$valid,
      observed_calls:$calls,observed_raw_total_tokens:$raw,terminal_state:$terminal}' > "$tmp" || return 1
  chmod 600 "$tmp"; mv "$tmp" "$receipt" || return 1
  write_ledger_sidecar "$receipt"
}

reconcile_existing_consumption() {
  local claim
  prepare_authorization_root || return 1
  claim=$(authorization_claim_dir)
  if [[ -e "$claim" ]]; then
    [[ -d "$claim" && ! -L "$claim" ]] || return 1
    return 0
  fi
  mkdir "$claim" || return 1
  chmod 700 "$claim"
  write_reconciled_reservation "$claim"
}

reserve_primary_authorization() {
  require_canonical_evidence_output || return 1
  if existing_primary_consumption; then
    reconcile_existing_consumption || true
    return 1
  fi
  prepare_authorization_root || return 1
  AUTHORIZATION_CLAIM_DIR=$(authorization_claim_dir)
  mkdir "$AUTHORIZATION_CLAIM_DIR" || return 1
  chmod 700 "$AUTHORIZATION_CLAIM_DIR"
  write_primary_reservation "$AUTHORIZATION_CLAIM_DIR"
}

validate_primary_reservation() {
  local claim receipt
  prepare_authorization_root || return 1
  claim=$(authorization_claim_dir); receipt="$claim/reservation.json"
  [[ -d "$claim" && ! -L "$claim" && -O "$claim" ]] || return 1
  verify_named_sidecar "$receipt" "$claim/reservation.sha256" || return 1
  jq -e --arg policy "$POLICY_HASH" --arg config "$CONFIG_HASH" --arg corpus "$CORPUS_HASH" \
    --arg cohort "$COHORT_HASH" --arg schema "$SCHEMA_HASH" '
      .version == 1 and .kind == "primary_authorization_reservation" and
      .authorization_identity == $policy and .policy_sha256 == $policy and
      .config_sha256 == $config and .corpus_sha256 == $corpus and .cohort_sha256 == $cohort and
      .verdict_schema_sha256 == $schema and .provider_call_cap == 64 and .raw_token_cap == 1500000 and
      .concurrency == 1 and .retries == 0 and .primary_calls_reserved == 58 and
      .rollback_calls_reserved == 5 and .total_calls_reserved == 63 and
      .total_raw_tokens_reserved == 1446000 and .state == "CONSUMED_ON_RESERVATION"
    ' "$receipt" >/dev/null || return 1
  AUTHORIZATION_CLAIM_DIR=$claim
}

reserve_rollback_authorization() {
  local claim tmp receipt
  require_canonical_evidence_output || return 1
  validate_primary_reservation || return 1
  claim="$AUTHORIZATION_CLAIM_DIR/rollback-reservation"
  mkdir "$claim" || return 1
  chmod 700 "$claim"
  tmp="$claim/.reservation.tmp.$$"; receipt="$claim/reservation.json"
  jq -n --arg policy "$POLICY_HASH" --arg primary "$PRIMARY_LEDGER_HASH" \
    --arg applied "$ROLLBACK_RECEIPT_HASH" '{version:1,kind:"rollback_replay_reservation",
      authorization_identity:$policy,primary_ledger_sha256:$primary,
      applied_rollback_receipt_sha256:$applied,calls_reserved:5,raw_tokens_reserved:114000,
      concurrency:1,retries:0,state:"CONSUMED_ON_RESERVATION"}' > "$tmp" || return 1
  chmod 600 "$tmp"; mv "$tmp" "$receipt" || return 1
  write_ledger_sidecar "$receipt"
}
