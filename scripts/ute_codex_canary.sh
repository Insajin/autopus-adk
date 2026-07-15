#!/usr/bin/env bash
set -euo pipefail
umask 077

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
# shellcheck source=ute_codex_canary_lib.sh
source "$SCRIPT_DIR/ute_codex_canary_lib.sh"
# shellcheck source=ute_codex_canary_prompt.sh
source "$SCRIPT_DIR/ute_codex_canary_prompt.sh"
# shellcheck source=ute_codex_canary_receipt.sh
source "$SCRIPT_DIR/ute_codex_canary_receipt.sh"
# shellcheck source=ute_codex_canary_ledger.sh
source "$SCRIPT_DIR/ute_codex_canary_ledger.sh"
# shellcheck source=ute_codex_canary_exec.sh
source "$SCRIPT_DIR/ute_codex_canary_exec.sh"
# shellcheck source=ute_codex_canary_authorization.sh
source "$SCRIPT_DIR/ute_codex_canary_authorization.sh"

usage() {
  printf '%s\n' 'usage: ute_codex_canary.sh preflight --repo SNAPSHOT'
  printf '%s\n' '       ute_codex_canary.sh primary --repo SNAPSHOT --auto BIN --state EMPTY_DIR --output DIR'
  printf '%s\n' '       ute_codex_canary.sh rollback --repo SNAPSHOT --auto BIN --state EMPTY_DIR --output DIR --primary-ledger FILE --rollback-receipt FILE --rollback-receipt-hash sha256:HEX'
}

MODE=${1:-preflight}
[[ $# -eq 0 ]] || shift
case "$MODE" in preflight|primary|rollback) ;; *) usage >&2; exit 2 ;; esac
REPO= AUTO= STATE= OUTPUT= PRIMARY_LEDGER= ROLLBACK_RECEIPT= ROLLBACK_RECEIPT_HASH=

while [[ $# -gt 0 ]]; do
  [[ $# -ge 2 ]] || die "missing option value"
  case "$1" in
    --repo) REPO=$2 ;;
    --auto) AUTO=$2 ;;
    --state) STATE=$2 ;;
    --output) OUTPUT=$2 ;;
    --primary-ledger) PRIMARY_LEDGER=$2 ;;
    --rollback-receipt) ROLLBACK_RECEIPT=$2 ;;
    --rollback-receipt-hash) ROLLBACK_RECEIPT_HASH=$2 ;;
    *) die "unknown option" ;;
  esac
  shift 2
done

EVIDENCE_DIR="$SCRIPT_DIR/../.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
EVIDENCE_DIR=$(cd "$EVIDENCE_DIR" && pwd -P)
CORPUS_FILE="$EVIDENCE_DIR/corpus-v1.json"
COHORT_FILE="$EVIDENCE_DIR/gpt-canary-cohort-v1.json"
SCHEMA_FILE="$EVIDENCE_DIR/gpt-verdict-schema-v1.json"
validate_static_evidence "$EVIDENCE_DIR" || die "static evidence validation failed"
[[ -n "$REPO" ]] || die "immutable repository snapshot is required"
REPO=$(cd "$REPO" 2>/dev/null && pwd -P) || die "repository snapshot unavailable"
validate_repo_snapshot "$REPO" "$CORPUS_FILE" || die "repository snapshot validation failed"

CORPUS_HASH="sha256:$(sha256_file "$CORPUS_FILE")"
COHORT_HASH="sha256:$(sha256_file "$COHORT_FILE")"
POLICY_HASH="sha256:$(sha256_file "$EVIDENCE_DIR/gpt-codex-policy-v1.json")"
CONFIG_HASH="sha256:$(sha256_file "$EVIDENCE_DIR/gpt-codex-config-v1.json")"
SCHEMA_HASH="sha256:$(sha256_file "$SCHEMA_FILE")"

if [[ "$MODE" == preflight ]]; then
  printf '%s\n' 'preflight=PASS provider_calls=0 raw=0'
  exit 0
fi

[[ "${UTE_CODEX_CANARY_EXECUTE:-}" == YES ]] || die "explicit execution opt-in required"
[[ -n "$AUTO" && -f "$AUTO" && -x "$AUTO" ]] || die "built auto executable is required"
[[ "$AUTO" == /* ]] || die "auto executable path must be absolute"
[[ -n "$STATE" && "$STATE" == /* && -d "$STATE" && ! -L "$STATE" ]] || die "isolated state must be an absolute real directory"
STATE=$(cd "$STATE" && pwd -P)
[[ -z "$(find "$STATE" -mindepth 1 -print -quit)" ]] || die "isolated state must be empty"
STATE_LOWER=$(printf '%s' "$STATE" | tr '[:upper:]' '[:lower:]')
[[ ! "$STATE_LOWER" =~ baseline|candidate|arm[-_]?a|arm[-_]?b ]] || die "isolated state path exposes arm identity"
[[ -n "$OUTPUT" && "$OUTPUT" == /* && -d "$OUTPUT" && ! -L "$OUTPUT" ]] || die "evidence output must be an absolute real directory"
OUTPUT=$(cd "$OUTPUT" && pwd -P)
[[ "$STATE/" != "$OUTPUT/"* && "$OUTPUT/" != "$STATE/"* ]] || die "isolated state and evidence output must be disjoint"
require_canonical_evidence_output || die "evidence output is not the canonical authorization trust anchor"

PRIMARY_OBSERVED_RAW=0 PRIMARY_LEDGER_HASH=
if [[ "$MODE" == primary ]]; then
  reserve_primary_authorization || die "primary authorization already consumed or concurrently reserved"
  ensure_ledger_targets_absent "$MODE" || die "evidence output collision"
else
  ensure_ledger_targets_absent "$MODE" || die "evidence output collision"
  [[ -n "$PRIMARY_LEDGER" && -n "$ROLLBACK_RECEIPT" && -n "$ROLLBACK_RECEIPT_HASH" ]] || die "rollback inputs are required"
  validate_primary_ledger_for_rollback "$PRIMARY_LEDGER" || die "primary ledger validation failed"
  PRIMARY_LEDGER_HASH="sha256:$(sha256_file "$PRIMARY_LEDGER")"
  validate_applied_rollback_receipt "$ROLLBACK_RECEIPT" "$ROLLBACK_RECEIPT_HASH" || die "applied rollback receipt validation failed"
  PRIMARY_OBSERVED_RAW=$(jq -r '.observed_raw_total_tokens' "$PRIMARY_LEDGER")
  reserve_rollback_authorization || die "rollback authorization already consumed or concurrently reserved"
fi

run_canary
