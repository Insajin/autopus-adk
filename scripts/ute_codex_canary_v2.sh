#!/usr/bin/env bash
set -euo pipefail
umask 077

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
bootstrap_die() { printf 'ute-codex-canary-v2: %s\n' "$1" >&2; exit 1; }
BOOTSTRAP_ROOT=$(cd "$SCRIPT_DIR/.." && pwd -P)
BOOTSTRAP_EVIDENCE_DIR="$BOOTSTRAP_ROOT/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
[[ -d "$BOOTSTRAP_EVIDENCE_DIR" && ! -L "$BOOTSTRAP_EVIDENCE_DIR" &&
   "$(cd "$BOOTSTRAP_EVIDENCE_DIR" && pwd -P)" == "$BOOTSTRAP_EVIDENCE_DIR" ]] || \
  bootstrap_die "canonical evidence directory unavailable"
bootstrap_sha() { shasum -a 256 "$1" | awk '{print "sha256:" $1}'; }
bootstrap_full_chain() {
  local tmp name
  local members=(ute_codex_canary_v2.sh ute_codex_canary_v2_static.sh
    ute_codex_canary_v2_authorization.sh ute_codex_canary_v2_ledger.sh
    ute_codex_canary_v2_freeze.sh ute_codex_canary_v2_authorize.sh
    ute_codex_canary_lib.sh ute_codex_canary_prompt.sh ute_codex_canary_receipt.sh
    ute_codex_canary_exec.sh ute_codex_canary_schedule.sh ute_gpt_evidence.sh
    ute_gpt_evidence_lib.sh ute_gpt_evidence_build.sh ute_gpt_evidence_rollout.sh
    ute_gpt_full_evaluation_finalize.sh ute_gpt_full_evaluation_finalize_lib.sh)
  tmp=$(mktemp "${TMPDIR:-/tmp}/ute-v2-full17.XXXXXX") || return 1
  for name in "${members[@]}"; do
    [[ -f "$SCRIPT_DIR/$name" && ! -L "$SCRIPT_DIR/$name" ]] || { rm -f "$tmp"; return 1; }
    printf '%s  scripts/%s\n' "$(shasum -a 256 "$SCRIPT_DIR/$name" | awk '{print $1}')" "$name" >> "$tmp"
  done
  LC_ALL=C sort -k2 "$tmp" -o "$tmp"; bootstrap_sha "$tmp"; rm -f "$tmp"
}
bootstrap_validate_full_chain() {
  local line expected name chain
  [[ -f "$BOOTSTRAP_IDENTITY" && ! -L "$BOOTSTRAP_IDENTITY" &&
     -f "$BOOTSTRAP_IDENTITY_SIDECAR" && ! -L "$BOOTSTRAP_IDENTITY_SIDECAR" ]] || return 1
  IFS= read -r line < "$BOOTSTRAP_IDENTITY_SIDECAR" || return 1
  [[ "$line" =~ ^([0-9a-f]{64})[[:space:]][[:space:]]([^[:space:]]+)$ ]] || return 1
  expected=${BASH_REMATCH[1]}; name=${BASH_REMATCH[2]}
  [[ "$name" == "${BOOTSTRAP_IDENTITY##*/}" &&
     "$expected" == "$(shasum -a 256 "$BOOTSTRAP_IDENTITY" | awk '{print $1}')" ]] || return 1
  chain=$(bootstrap_full_chain) || return 1
  jq -e --arg chain "$chain" '.runtime.full_chain_harness_sha256==$chain and
    .runtime.full_chain_algorithm=="sha256-named-member-manifest-v1" and .runtime.full_chain_member_count==17' \
    "$BOOTSTRAP_IDENTITY" >/dev/null
}
bootstrap_admission_bundle() {
  local tmp name
  local members=(ute_codex_canary_v2.sh ute_codex_canary_v2_static.sh
    ute_codex_canary_v2_authorization.sh ute_codex_canary_v2_ledger.sh
    ute_codex_canary_lib.sh ute_codex_canary_prompt.sh ute_codex_canary_receipt.sh
    ute_codex_canary_exec.sh ute_codex_canary_schedule.sh)
  tmp=$(mktemp "${TMPDIR:-/tmp}/ute-v2-bootstrap.XXXXXX") || return 1
  for name in "${members[@]}"; do
    [[ -f "$SCRIPT_DIR/$name" && ! -L "$SCRIPT_DIR/$name" ]] || { rm -f "$tmp"; return 1; }
    printf '%s  scripts/%s\n' "$(shasum -a 256 "$SCRIPT_DIR/$name" | awk '{print $1}')" "$name" >> "$tmp"
  done
  LC_ALL=C sort -k2 "$tmp" -o "$tmp"
  bootstrap_sha "$tmp"
  rm -f "$tmp"
}
for bootstrap_tool in jq shasum awk mktemp sort; do
  command -v "$bootstrap_tool" >/dev/null 2>&1 || bootstrap_die "bootstrap tool unavailable"
done
BOOTSTRAP_IDENTITY="$BOOTSTRAP_EVIDENCE_DIR/gpt-full-evaluation-identity-v2.json"
BOOTSTRAP_IDENTITY_SIDECAR="${BOOTSTRAP_IDENTITY%.json}.sha256"
bootstrap_validate_full_chain || bootstrap_die "frozen full17 identity mismatch"
BOOTSTRAP_BUNDLE=$(bootstrap_admission_bundle) || bootstrap_die "admission bundle unavailable"
jq -e --arg bundle "$BOOTSTRAP_BUNDLE" '
  .runtime.admission_bundle_sha256==$bundle and
  .runtime.admission_bundle_algorithm=="sha256-named-member-manifest-v1" and
  .runtime.admission_bundle_member_count==9
' "$BOOTSTRAP_IDENTITY" >/dev/null || bootstrap_die "admission bundle mismatch"
# shellcheck source=ute_codex_canary_lib.sh
source "$SCRIPT_DIR/ute_codex_canary_lib.sh"
# shellcheck source=ute_codex_canary_prompt.sh
source "$SCRIPT_DIR/ute_codex_canary_prompt.sh"
# shellcheck source=ute_codex_canary_receipt.sh
source "$SCRIPT_DIR/ute_codex_canary_receipt.sh"
# shellcheck source=ute_codex_canary_exec.sh
source "$SCRIPT_DIR/ute_codex_canary_exec.sh"
# shellcheck source=ute_codex_canary_v2_static.sh
source "$SCRIPT_DIR/ute_codex_canary_v2_static.sh"
# shellcheck source=ute_codex_canary_v2_ledger.sh
source "$SCRIPT_DIR/ute_codex_canary_v2_ledger.sh"
# shellcheck source=ute_codex_canary_v2_authorization.sh
source "$SCRIPT_DIR/ute_codex_canary_v2_authorization.sh"

usage() {
  printf '%s\n' 'usage: ute_codex_canary_v2.sh preflight --repo SNAPSHOT [--auto BIN] --output DIR'
  printf '%s\n' '       ute_codex_canary_v2.sh primary --repo SNAPSHOT --auto BIN --state EMPTY_DIR --output DIR'
  printf '%s\n' '       ute_codex_canary_v2.sh rollback --repo SNAPSHOT --auto BIN --state EMPTY_DIR --output DIR --primary-ledger FILE --rollback-receipt FILE --rollback-receipt-hash sha256:HEX'
}

MODE=${1:-preflight}
[[ $# -eq 0 ]] || shift
case "$MODE" in preflight|primary|rollback) ;; *) usage >&2; exit 2 ;; esac
REPO= AUTO= STATE= OUTPUT= PRIMARY_LEDGER= ROLLBACK_RECEIPT= ROLLBACK_RECEIPT_HASH=
while [[ $# -gt 0 ]]; do
  [[ $# -ge 2 ]] || die "missing option value"
  case "$1" in
    --repo) [[ -z "$REPO" ]] || die "duplicate --repo"; REPO=$2 ;;
    --auto) [[ -z "$AUTO" ]] || die "duplicate --auto"; AUTO=$2 ;;
    --state) [[ -z "$STATE" ]] || die "duplicate --state"; STATE=$2 ;;
    --output) [[ -z "$OUTPUT" ]] || die "duplicate --output"; OUTPUT=$2 ;;
    --primary-ledger) [[ -z "$PRIMARY_LEDGER" ]] || die "duplicate --primary-ledger"; PRIMARY_LEDGER=$2 ;;
    --rollback-receipt) [[ -z "$ROLLBACK_RECEIPT" ]] || die "duplicate --rollback-receipt"; ROLLBACK_RECEIPT=$2 ;;
    --rollback-receipt-hash) [[ -z "$ROLLBACK_RECEIPT_HASH" ]] || die "duplicate --rollback-receipt-hash"; ROLLBACK_RECEIPT_HASH=$2 ;;
    *) die "unknown option" ;;
  esac
  shift 2
done

EVIDENCE_DIR="$SCRIPT_DIR/../.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
EVIDENCE_DIR=$(cd "$EVIDENCE_DIR" && pwd -P)
CORPUS_FILE="$EVIDENCE_DIR/corpus-v1.json"; COHORT_FILE="$EVIDENCE_DIR/gpt-canary-cohort-v1.json"
SCHEMA_FILE="$EVIDENCE_DIR/gpt-verdict-schema-v2.json"
V2_IDENTITY_FILE="$EVIDENCE_DIR/gpt-full-evaluation-identity-v2.json"
v2_validate_static_evidence "$EVIDENCE_DIR" || die "v2 static evidence validation failed"
[[ -n "$REPO" ]] || die "immutable repository snapshot is required"
REPO=$(cd "$REPO" 2>/dev/null && pwd -P) || die "repository snapshot unavailable"
v2_validate_snapshot_binding "$REPO" "$CORPUS_FILE" "$EVIDENCE_DIR/gpt-snapshot-manifest-v2.json" || \
  die "repository snapshot validation failed"
CORPUS_HASH=$(v2_sha_uri "$CORPUS_FILE"); COHORT_HASH=$(v2_sha_uri "$COHORT_FILE")
POLICY_HASH=$(v2_sha_uri "$EVIDENCE_DIR/gpt-codex-policy-v2.json")
CONFIG_HASH=$(v2_sha_uri "$EVIDENCE_DIR/gpt-codex-config-v2.json"); SCHEMA_HASH=$(v2_sha_uri "$SCHEMA_FILE")

[[ -n "$OUTPUT" && "$OUTPUT" == /* && -d "$OUTPUT" && ! -L "$OUTPUT" ]] || die "evidence output must be an absolute real directory"
OUTPUT=$(cd "$OUTPUT" && pwd -P)
v2_require_canonical_output || die "evidence output is not the canonical authorization trust anchor"
if [[ "$MODE" == preflight ]]; then
  jq -c '.' "$EVIDENCE_DIR/gpt-canary-preflight-v2.json"
  exit 0
fi

[[ "${UTE_CODEX_CANARY_V2_EXECUTE:-}" == YES ]] || die "explicit v2 execution opt-in required"
[[ -n "$AUTO" && "$AUTO" == /* && -f "$AUTO" && -x "$AUTO" && ! -L "$AUTO" ]] || die "exact built auto executable is required"
[[ -n "$STATE" && "$STATE" == /* && -d "$STATE" && ! -L "$STATE" ]] || die "isolated state must be an absolute real directory"
STATE=$(cd "$STATE" && pwd -P)
[[ -z "$(find "$STATE" -mindepth 1 -print -quit)" ]] || die "isolated state must be empty"
STATE_LOWER=$(printf '%s' "$STATE" | tr '[:upper:]' '[:lower:]')
[[ ! "$STATE_LOWER" =~ baseline|candidate|arm[-_]?a|arm[-_]?b ]] || die "isolated state path exposes arm identity"
[[ "$STATE/" != "$OUTPUT/"* && "$OUTPUT/" != "$STATE/"* ]] || die "isolated state and evidence output must be disjoint"
v2_validate_authorization_receipt || die "exact v2 authorization receipt is required"
v2_prepare_runtime_identity || die "frozen runtime identity validation failed"

PRIMARY_OBSERVED_RAW=0 PRIMARY_LEDGER_HASH=
if [[ "$MODE" == primary ]]; then
  ensure_ledger_targets_absent primary || die "v2 primary evidence output collision"
  bootstrap_validate_full_chain || die "full17 changed before primary reservation"
  v2_reserve_primary_authorization || die "v2 primary authorization already consumed or invalid"
else
  ensure_ledger_targets_absent rollback || die "v2 rollback evidence output collision"
  [[ -n "$PRIMARY_LEDGER" && -n "$ROLLBACK_RECEIPT" && -n "$ROLLBACK_RECEIPT_HASH" ]] || die "rollback inputs are required"
  v2_validate_primary_reservation || die "v2 primary reservation validation failed"
  v2_validate_primary_ledger_for_rollback "$PRIMARY_LEDGER" || die "v2 primary ledger validation failed"
  PRIMARY_LEDGER_HASH=$(v2_sha_uri "$PRIMARY_LEDGER")
  v2_validate_rollback_inputs "$ROLLBACK_RECEIPT" "$ROLLBACK_RECEIPT_HASH" "$PRIMARY_LEDGER_HASH" || \
    die "v2 applied rollback or evaluation gate failed"
  PRIMARY_OBSERVED_RAW=$(jq -r '.observed_raw_total_tokens' "$PRIMARY_LEDGER")
  bootstrap_validate_full_chain || die "full17 changed before rollback reservation"
  v2_reserve_rollback_authorization || die "v2 rollback authorization already consumed or invalid"
fi

prepare_runtime_identity() { v2_prepare_runtime_identity; }
canary_prepare_runtime_inputs() {
  local source target expected
  for source in "$CORPUS_FILE" "$COHORT_FILE" "$SCHEMA_FILE"; do
    [[ -f "$source" && ! -L "$source" ]] || return 1
    target="$TEMP_ROOT/$(basename "$source")"
    cp "$source" "$target" || return 1
    chmod 600 "$target"
    case "$(basename "$source")" in
      corpus-v1.json) expected=$CORPUS_HASH; CORPUS_FILE=$target ;;
      gpt-canary-cohort-v1.json) expected=$COHORT_HASH; COHORT_FILE=$target ;;
      gpt-verdict-schema-v2.json) expected=$SCHEMA_HASH; SCHEMA_FILE=$target ;;
      *) return 1 ;;
    esac
    [[ "$(v2_sha_uri "$target")" == "$expected" ]] || return 1
  done
}
canary_pre_call_guard() {
  bootstrap_validate_full_chain || return 1
  [[ -f "$AUTO" && ! -L "$AUTO" && "$(v2_sha_uri "$AUTO")" == "$AUTO_SHA" ]] || return 1
  [[ -f "$CODEX_BIN" && ! -L "$CODEX_BIN" && "$(v2_sha_uri "$CODEX_BIN")" == "$CODEX_EXECUTABLE_SHA" ]] || return 1
  [[ "$MODE" != rollback ]] || v2_validate_rollback_reservation
}
bootstrap_validate_full_chain || die "full17 changed before provider execution"
if [[ "$MODE" == rollback ]]; then
  v2_validate_rollback_reservation || die "rollback reservation validation failed"
fi
run_canary
