#!/usr/bin/env bash

diag_auth_root() {
  local install_root
  install_root=$(cd "$SCRIPT_DIR/.." && pwd -P)
  printf '%s/.autopus/runtime/ute-diagnostic-authorizations\n' "$install_root"
}

diag_claim_dir() { printf '%s/%s\n' "$(diag_auth_root)" "${DIAG_AUTHORIZATION_ID#sha256:}"; }

prepare_diag_auth_root() {
  local root parent grandparent
  root=$(diag_auth_root); parent=$(dirname "$root"); grandparent=$(dirname "$parent")
  [[ -d "$grandparent" && ! -L "$grandparent" && -O "$grandparent" ]] || return 1
  if [[ -e "$parent" ]]; then
    [[ -d "$parent" && ! -L "$parent" && -O "$parent" ]] || return 1
  else
    mkdir "$parent" || return 1; chmod 700 "$parent"
  fi
  if [[ -e "$root" ]]; then
    [[ -d "$root" && ! -L "$root" && -O "$root" ]] || return 1
  else
    mkdir "$root" || return 1; chmod 700 "$root"
  fi
  DIAG_AUTH_ROOT=$root
}

detect_diag_consumption() {
  local file sidecar aggregate
  local files=()
  shopt -s nullglob; files=("$EVIDENCE_DIR"/gpt-diagnostic-call-ledger-v1*.json); shopt -u nullglob
  [[ ${#files[@]} -gt 0 ]] || return 1
  DIAG_EXISTING_VALID=false; DIAG_EXISTING_CALLS=0; DIAG_EXISTING_RAW=0; DIAG_EXISTING_STATE=UNTRUSTED
  aggregate=$(for file in "${files[@]}"; do sha256_file "$file"; done | shasum -a 256 | awk '{print $1}')
  DIAG_EXISTING_HASH="sha256:$aggregate"
  [[ ${#files[@]} -eq 1 ]] || return 0
  file=${files[0]}; sidecar="${file%.json}.sha256"
  verify_named_sidecar "$file" "$sidecar" || return 0
  jq -e --arg auth "$DIAG_AUTHORIZATION_ID" '
    .spec_id == "SPEC-ADK-ULTRA-EFFICIENCY-001" and .identity.authorization_identity == $auth and
    .diagnostic_only == true and .promotion_eligible == false and .attempted_calls >= 0 and
    (.completed == true or .completed == false)
  ' "$file" >/dev/null || return 0
  DIAG_EXISTING_VALID=true; DIAG_EXISTING_HASH="sha256:$(sha256_file "$file")"
  DIAG_EXISTING_CALLS=$(jq -r '.attempted_calls' "$file"); DIAG_EXISTING_RAW=$(jq -r '.observed_raw_total_tokens' "$file")
  [[ "$(jq -r '.completed' "$file")" == true ]] && DIAG_EXISTING_STATE=COMPLETE || DIAG_EXISTING_STATE=PARTIAL
}

write_diag_reservation() {
  local claim=$1 kind=$2 tmp="$1/.reservation.tmp.$$" receipt="$1/reservation.json"
  local source=${DIAG_EXISTING_HASH:-null} valid=${DIAG_EXISTING_VALID:-false}
  local calls=${DIAG_EXISTING_CALLS:-0} raw=${DIAG_EXISTING_RAW:-0} terminal=${DIAG_EXISTING_STATE:-FRESH}
  jq -n --arg kind "$kind" --arg auth "$DIAG_AUTHORIZATION_ID" --arg p "$DIAG_POLICY_HASH" \
    --arg c "$DIAG_CONFIG_HASH" --arg s "$DIAG_SCHEMA_HASH" --arg h "$DIAG_COHORT_HASH" \
    --arg source "$source" --arg terminal "$terminal" --argjson valid "$valid" --argjson calls "$calls" --argjson raw "$raw" \
    '{version:1,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",kind:$kind,authorization_identity:$auth,
      policy_sha256:$p,config_sha256:$c,verdict_schema_sha256:$s,cohort_sha256:$h,
      provider:"codex",model:"gpt-5.6-sol",calls_reserved:10,raw_tokens_reserved:228000,
      concurrency:1,retries:0,single_use:true,state:"CONSUMED_ON_RESERVATION",
      source_ledger_sha256:$source,source_ledger_valid:$valid,observed_calls:$calls,
      observed_raw_total_tokens:$raw,terminal_state:$terminal}' > "$tmp" || return 1
  chmod 600 "$tmp"; mv "$tmp" "$receipt" || return 1
  printf '%s  reservation.json\n' "$(sha256_file "$receipt")" > "$claim/reservation.sha256"
  chmod 600 "$claim/reservation.sha256"
}

reconcile_diag_consumption() {
  local claim
  prepare_diag_auth_root || return 1; claim=$(diag_claim_dir)
  [[ ! -e "$claim" ]] || return 0
  mkdir "$claim" || return 1; chmod 700 "$claim"
  write_diag_reservation "$claim" diagnostic_consumption_reconciliation
}

reserve_diag_authorization() {
  local claim
  [[ "$OUTPUT" == "$EVIDENCE_DIR" ]] || return 1
  if detect_diag_consumption; then reconcile_diag_consumption || true; return 1; fi
  prepare_diag_auth_root || return 1; claim=$(diag_claim_dir)
  mkdir "$claim" || return 1; chmod 700 "$claim"
  write_diag_reservation "$claim" diagnostic_authorization_reservation
}
