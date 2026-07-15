#!/usr/bin/env bash
set -euo pipefail
umask 077

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
# shellcheck source=ute_gpt_diag_canary_lib.sh
source "$SCRIPT_DIR/ute_gpt_diag_canary_lib.sh"
# shellcheck source=ute_gpt_diag_canary_authorization.sh
source "$SCRIPT_DIR/ute_gpt_diag_canary_authorization.sh"
# shellcheck source=ute_gpt_diag_canary_prompt.sh
source "$SCRIPT_DIR/ute_gpt_diag_canary_prompt.sh"
# shellcheck source=ute_gpt_diag_canary_receipt.sh
source "$SCRIPT_DIR/ute_gpt_diag_canary_receipt.sh"
# shellcheck source=ute_gpt_diag_canary_ledger.sh
source "$SCRIPT_DIR/ute_gpt_diag_canary_ledger.sh"
# shellcheck source=ute_gpt_diag_canary_exec.sh
source "$SCRIPT_DIR/ute_gpt_diag_canary_exec.sh"

MODE=${1:-preflight}
[[ $# -eq 0 ]] || shift
case "$MODE" in preflight|run) ;; *) diag_die "mode must be preflight or run" ;; esac
REPO= AUTO= STATE= OUTPUT=
while [[ $# -gt 0 ]]; do
  [[ $# -ge 2 ]] || diag_die "missing option value"
  case "$1" in
    --repo) REPO=$2 ;;
    --auto) AUTO=$2 ;;
    --state) STATE=$2 ;;
    --output) OUTPUT=$2 ;;
    *) diag_die "unknown option" ;;
  esac
  shift 2
done

EVIDENCE_DIR="$SCRIPT_DIR/../.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
EVIDENCE_DIR=$(cd "$EVIDENCE_DIR" && pwd -P)
CORPUS_FILE="$EVIDENCE_DIR/corpus-v1.json"
COHORT_FILE="$EVIDENCE_DIR/gpt-diagnostic-cohort-v1.json"
SCHEMA_FILE="$EVIDENCE_DIR/gpt-diagnostic-verdict-schema-v1.json"
validate_diag_static "$EVIDENCE_DIR" || diag_die "static diagnostic evidence validation failed"
DIAG_AUTHORIZATION_ID=$(diag_authorization_id)
[[ -n "$REPO" ]] || diag_die "immutable repository snapshot is required"
REPO=$(cd "$REPO" 2>/dev/null && pwd -P) || diag_die "repository snapshot unavailable"
validate_diag_repo "$REPO" "$CORPUS_FILE" "$COHORT_FILE" || diag_die "repository snapshot validation failed"

if [[ "$MODE" == preflight ]]; then
  printf 'preflight=PASS authorization=%s provider_calls=0 raw=0\n' "$DIAG_AUTHORIZATION_ID"
  exit 0
fi

[[ "${UTE_GPT_DIAG_EXECUTE:-}" == YES ]] || diag_die "explicit diagnostic execution opt-in required"
[[ -n "$AUTO" && "$AUTO" == /* && -f "$AUTO" && -x "$AUTO" ]] || diag_die "built auto executable is required"
[[ -n "$STATE" && "$STATE" == /* && -d "$STATE" && ! -L "$STATE" ]] || diag_die "isolated state must be an absolute real directory"
STATE=$(cd "$STATE" && pwd -P); [[ -z "$(find "$STATE" -mindepth 1 -print -quit)" ]] || diag_die "isolated state must be empty"
STATE_LOWER=$(printf '%s' "$STATE" | tr '[:upper:]' '[:lower:]')
[[ ! "$STATE_LOWER" =~ baseline|candidate|arm[-_]?a|arm[-_]?b ]] || diag_die "state path exposes arm identity"
[[ -n "$OUTPUT" && "$OUTPUT" == /* && -d "$OUTPUT" && ! -L "$OUTPUT" ]] || diag_die "canonical evidence output is required"
OUTPUT=$(cd "$OUTPUT" && pwd -P)
[[ "$OUTPUT" == "$EVIDENCE_DIR" ]] || diag_die "output is not the canonical diagnostic evidence root"
[[ "$STATE/" != "$OUTPUT/"* && "$OUTPUT/" != "$STATE/"* ]] || diag_die "state and output must be disjoint"
reserve_diag_authorization || diag_die "diagnostic authorization already consumed or concurrently reserved"
ensure_diag_targets_absent || diag_die "diagnostic ledger output collision"
run_diag_canary
