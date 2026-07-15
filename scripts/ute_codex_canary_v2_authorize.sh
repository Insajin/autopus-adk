#!/usr/bin/env bash
set -euo pipefail
umask 077

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
authorize_die() { printf 'ute-codex-canary-v2-authorize: %s\n' "$1" >&2; exit 1; }
authorize_bootstrap_chain() {
  local tmp name
  local members=(ute_codex_canary_v2.sh ute_codex_canary_v2_static.sh
    ute_codex_canary_v2_authorization.sh ute_codex_canary_v2_ledger.sh
    ute_codex_canary_v2_freeze.sh ute_codex_canary_v2_authorize.sh
    ute_codex_canary_lib.sh ute_codex_canary_prompt.sh ute_codex_canary_receipt.sh
    ute_codex_canary_exec.sh ute_codex_canary_schedule.sh ute_gpt_evidence.sh
    ute_gpt_evidence_lib.sh ute_gpt_evidence_build.sh ute_gpt_evidence_rollout.sh
    ute_gpt_full_evaluation_finalize.sh ute_gpt_full_evaluation_finalize_lib.sh)
  tmp=$(mktemp "${TMPDIR:-/tmp}/ute-v2-authorize-full17.XXXXXX") || return 1
  for name in "${members[@]}"; do
    [[ -f "$SCRIPT_DIR/$name" && ! -L "$SCRIPT_DIR/$name" ]] || { rm -f "$tmp"; return 1; }
    printf '%s  scripts/%s\n' "$(shasum -a 256 "$SCRIPT_DIR/$name" | awk '{print $1}')" "$name" >> "$tmp"
  done
  LC_ALL=C sort -k2 "$tmp" -o "$tmp"
  printf 'sha256:%s\n' "$(shasum -a 256 "$tmp" | awk '{print $1}')"; rm -f "$tmp"
}
authorize_bootstrap_validate() {
  local identity=$1 sidecar="${1%.json}.sha256" line expected name chain
  [[ -f "$identity" && ! -L "$identity" && -f "$sidecar" && ! -L "$sidecar" ]] || return 1
  IFS= read -r line < "$sidecar" || return 1
  [[ "$line" =~ ^([0-9a-f]{64})[[:space:]][[:space:]]([^[:space:]]+)$ ]] || return 1
  expected=${BASH_REMATCH[1]}; name=${BASH_REMATCH[2]}
  [[ "$name" == "${identity##*/}" && "$expected" == "$(shasum -a 256 "$identity" | awk '{print $1}')" ]] || return 1
  chain=$(authorize_bootstrap_chain) || return 1
  jq -e --arg chain "$chain" '.runtime.full_chain_harness_sha256==$chain and
    .runtime.full_chain_algorithm=="sha256-named-member-manifest-v1" and .runtime.full_chain_member_count==17' \
    "$identity" >/dev/null
}
authorize_usage() {
  printf '%s\n' 'usage: UTE_CODEX_CANARY_V2_AUTHORIZE=YES ute_codex_canary_v2_authorize.sh authorize --evidence-dir DIR --identity sha256:HEX --authorized-at RFC3339'
}

MODE=${1:-}
[[ "$MODE" == authorize ]] || { authorize_usage >&2; exit 2; }
shift
EVIDENCE_DIR= IDENTITY_ARGUMENT= AUTHORIZED_AT=
while [[ $# -gt 0 ]]; do
  [[ $# -ge 2 ]] || authorize_die "missing option value"
  case "$1" in
    --evidence-dir) [[ -z "$EVIDENCE_DIR" ]] || authorize_die "duplicate --evidence-dir"; EVIDENCE_DIR=$2 ;;
    --identity) [[ -z "$IDENTITY_ARGUMENT" ]] || authorize_die "duplicate --identity"; IDENTITY_ARGUMENT=$2 ;;
    --authorized-at) [[ -z "$AUTHORIZED_AT" ]] || authorize_die "duplicate --authorized-at"; AUTHORIZED_AT=$2 ;;
    *) authorize_die "unknown option" ;;
  esac
  shift 2
done
[[ "${UTE_CODEX_CANARY_V2_AUTHORIZE:-}" == YES ]] || authorize_die "explicit authorization opt-in required"
[[ "$EVIDENCE_DIR" == /* && -d "$EVIDENCE_DIR" && ! -L "$EVIDENCE_DIR" ]] || authorize_die "canonical evidence directory required"
[[ "$IDENTITY_ARGUMENT" =~ ^sha256:[0-9a-f]{64}$ ]] || authorize_die "invalid identity"
[[ "$AUTHORIZED_AT" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(Z|[+-][0-9]{2}:[0-9]{2})$ ]] || \
  authorize_die "invalid authorized-at"
EVIDENCE_DIR=$(cd "$EVIDENCE_DIR" && pwd -P)
CANONICAL_ROOT=$(cd "$SCRIPT_DIR/.." && pwd -P)
CANONICAL_DIR="$CANONICAL_ROOT/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
[[ -d "$CANONICAL_DIR" && ! -L "$CANONICAL_DIR" && "$(cd "$CANONICAL_DIR" && pwd -P)" == "$CANONICAL_DIR" ]] || \
  authorize_die "canonical evidence directory unavailable"
[[ "$EVIDENCE_DIR" == "$CANONICAL_DIR" ]] || authorize_die "noncanonical evidence directory"
V2_IDENTITY_FILE="$EVIDENCE_DIR/gpt-full-evaluation-identity-v2.json"
for authorize_tool in jq shasum awk mktemp sort; do
  command -v "$authorize_tool" >/dev/null 2>&1 || authorize_die "bootstrap tool unavailable"
done
authorize_bootstrap_validate "$V2_IDENTITY_FILE" || authorize_die "frozen full17 identity mismatch"
# shellcheck source=ute_codex_canary_lib.sh
source "$SCRIPT_DIR/ute_codex_canary_lib.sh"
# shellcheck source=ute_codex_canary_v2_static.sh
source "$SCRIPT_DIR/ute_codex_canary_v2_static.sh"
v2_validate_static_evidence "$EVIDENCE_DIR" || authorize_die "frozen v2 evidence validation failed"
[[ "$IDENTITY_ARGUMENT" == "$V2_AUTHORIZATION_IDENTITY_SHA256" ]] || authorize_die "identity confirmation mismatch"
HARNESS_SHA=$(v2_sha_uri "$SCRIPT_DIR/ute_codex_canary_v2.sh")
POLICY_HASH=$(v2_sha_uri "$EVIDENCE_DIR/gpt-codex-policy-v2.json")
CONFIG_HASH=$(v2_sha_uri "$EVIDENCE_DIR/gpt-codex-config-v2.json")
PREFLIGHT_HASH=$(v2_sha_uri "$EVIDENCE_DIR/gpt-canary-preflight-v2.json")
jq -e --arg harness "$HARNESS_SHA" --arg identity "$IDENTITY_ARGUMENT" '
  .runtime.harness_sha256==$harness and (.runtime.full_chain_harness_sha256|test("^sha256:[0-9a-f]{64}$")) and
  .runtime.full_chain_algorithm=="sha256-named-member-manifest-v1" and .runtime.full_chain_member_count==17 and
  .runtime.codex_cli_version=="0.144.1" and .decision.activation==false and
  .decision.promotion==false and .decision.implemented==false
' "$V2_IDENTITY_FILE" >/dev/null || authorize_die "frozen identity runtime mismatch"
jq -e --arg identity "$IDENTITY_ARGUMENT" '
  .authorization_identity.sha256==$identity and
  .decision.status=="AWAITING_EXPLICIT_FULL_EVALUATION_AUTHORIZATION" and
  .decision.provider_execution_started==false and .decision.activation==false and
  .decision.promotion==false and .decision.implemented==false
' "$EVIDENCE_DIR/gpt-canary-preflight-v2.json" >/dev/null || authorize_die "preflight identity mismatch"

TARGET="$EVIDENCE_DIR/gpt-full-evaluation-authorization-v2.json"
SIDECAR="$EVIDENCE_DIR/gpt-full-evaluation-authorization-v2.sha256"
[[ ! -e "$TARGET" && ! -L "$TARGET" && ! -e "$SIDECAR" && ! -L "$SIDECAR" ]] || \
  authorize_die "authorization already exists"
TMP_JSON="$EVIDENCE_DIR/.gpt-full-evaluation-authorization-v2.json.tmp.$$"
TMP_SIDECAR="$EVIDENCE_DIR/.gpt-full-evaluation-authorization-v2.sha256.tmp.$$"
trap 'rm -f "$TMP_JSON" "$TMP_SIDECAR"' EXIT INT TERM
jq -n --arg auth "$IDENTITY_ARGUMENT" --arg policy "$POLICY_HASH" --arg config "$CONFIG_HASH" \
  --arg preflight "$PREFLIGHT_HASH" --arg authorized "$AUTHORIZED_AT" \
  '{version:2,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",
  evidence_kind:"gpt_codex_full_evaluation_authorization",
  decision:"EXPLICIT_FULL_EVALUATION_AUTHORIZATION_GRANTED",
  authorization_source:"user_exact_identity_confirmation",authorization_identity_sha256:$auth,
  policy_sha256:$policy,config_sha256:$config,preflight_sha256:$preflight,
  provider:"codex",model:"gpt-5.6-sol",provider_call_cap:64,raw_token_cap:1500000,
  primary_calls:58,rollback_calls:5,total_calls:63,planned_worst_case_raw_tokens:1446000,
  concurrency:1,retries:0,single_use:true,provider_execution_started:false,
  activation:false,promotion:false,implemented:false,authorized_at:$authorized}' | jq -S '.' > "$TMP_JSON"
chmod 600 "$TMP_JSON"
printf '%s  %s\n' "$(sha256_file "$TMP_JSON")" "$(basename "$TARGET")" > "$TMP_SIDECAR"
chmod 600 "$TMP_SIDECAR"
ln "$TMP_JSON" "$TARGET" || authorize_die "authorization publication race"
ln "$TMP_SIDECAR" "$SIDECAR" || authorize_die "authorization sidecar publication race"
rm -f "$TMP_JSON" "$TMP_SIDECAR"
trap - EXIT INT TERM
printf 'authorization=CREATED provider_calls=0 claim=0 reservation=0 identity=%s\n' "$IDENTITY_ARGUMENT"
