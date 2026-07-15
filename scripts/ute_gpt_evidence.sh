#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
SOURCE_ROOT=$(cd "$SCRIPT_DIR/.." && pwd -P)
STATIC_DIR="$SOURCE_ROOT/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"

_gpt_v2_bootstrap_chain() {
  local identity="$STATIC_DIR/gpt-full-evaluation-identity-v2.json" sidecar line expected file_name chain actual name
  local members=(ute_codex_canary_v2.sh ute_codex_canary_v2_static.sh
    ute_codex_canary_v2_authorization.sh ute_codex_canary_v2_ledger.sh
    ute_codex_canary_v2_freeze.sh ute_codex_canary_v2_authorize.sh
    ute_codex_canary_lib.sh ute_codex_canary_prompt.sh ute_codex_canary_receipt.sh
    ute_codex_canary_exec.sh ute_codex_canary_schedule.sh ute_gpt_evidence.sh
    ute_gpt_evidence_lib.sh ute_gpt_evidence_build.sh ute_gpt_evidence_rollout.sh
    ute_gpt_full_evaluation_finalize.sh ute_gpt_full_evaluation_finalize_lib.sh)
  sidecar="${identity%.json}.sha256"
  [[ -d "$STATIC_DIR" && ! -L "$STATIC_DIR" && -f "$identity" && ! -L "$identity" &&
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

_gpt_v2_requested=false
_gpt_args=("$@")
for ((_gpt_i=0; _gpt_i<${#_gpt_args[@]}; _gpt_i++)); do
  if [[ "${_gpt_args[$_gpt_i]}" == --admission-generation && "${_gpt_args[$((_gpt_i + 1))]:-}" == v2 ]]; then
    _gpt_v2_requested=true
  fi
done
[[ "$_gpt_v2_requested" != true ]] || _gpt_v2_bootstrap_chain || {
  printf '%s\n' 'ute-gpt-evidence: v2 pre-source full-chain validation failed' >&2; exit 1;
}
# shellcheck source=ute_codex_canary_lib.sh
source "$SCRIPT_DIR/ute_codex_canary_lib.sh"
# shellcheck source=ute_codex_canary_receipt.sh
source "$SCRIPT_DIR/ute_codex_canary_receipt.sh"
# shellcheck source=ute_gpt_evidence_lib.sh
source "$SCRIPT_DIR/ute_gpt_evidence_lib.sh"
# shellcheck source=ute_gpt_evidence_build.sh
source "$SCRIPT_DIR/ute_gpt_evidence_build.sh"
# shellcheck source=ute_gpt_evidence_rollout.sh
source "$SCRIPT_DIR/ute_gpt_evidence_rollout.sh"

gpt_validate_full_chain() {
  local dir=$1 expected raw="$WORK/full-chain.raw" manifest="$WORK/full-chain.manifest" name
  local members=(ute_codex_canary_v2.sh ute_codex_canary_v2_static.sh
    ute_codex_canary_v2_authorization.sh ute_codex_canary_v2_ledger.sh
    ute_codex_canary_v2_freeze.sh ute_codex_canary_v2_authorize.sh
    ute_codex_canary_lib.sh ute_codex_canary_prompt.sh ute_codex_canary_receipt.sh
    ute_codex_canary_exec.sh ute_codex_canary_schedule.sh ute_gpt_evidence.sh
    ute_gpt_evidence_lib.sh ute_gpt_evidence_build.sh ute_gpt_evidence_rollout.sh
    ute_gpt_full_evaluation_finalize.sh ute_gpt_full_evaluation_finalize_lib.sh)
  expected=$(jq -er '.runtime.full_chain_harness_sha256 |
    select(type == "string" and test("^sha256:[0-9a-f]{64}$"))' \
    "$dir/gpt-full-evaluation-identity-v2.json") || return 1
  : > "$raw"
  for name in "${members[@]}"; do
    [[ -f "$SCRIPT_DIR/$name" && ! -L "$SCRIPT_DIR/$name" ]] || return 1
    printf '%s  scripts/%s\n' "$(shasum -a 256 "$SCRIPT_DIR/$name" | awk '{print $1}')" "$name" >> "$raw"
  done
  LC_ALL=C sort -k2 "$raw" > "$manifest" || return 1
  [[ "$(gpt_sha_file "$manifest")" == "$expected" ]]
}

gpt_opaque_id() {
  local kind=$1 mode=$2 seq=$3 task=$4 digest
  digest=$(printf '%s\0%s\0%s\0%s\0%s' "$CONFIG_HASH" "$kind" "$mode" "$seq" "$task" |
    shasum -a 256 | awk '{print $1}')
  printf '%s%s\n' "${kind:0:1}" "${digest:0:24}"
}

gpt_validate_opaque_ids() {
  local file=$1 mode=$2 rows="$WORK/opaque-$2.tsv" seq task call_id run_id
  jq -er '.calls[] | [.sequence,.task_id,.result.call_id,.result.run_id] | @tsv' "$file" > "$rows" || return 1
  while IFS=$'\t' read -r seq task call_id run_id; do
    [[ "$seq" =~ ^[1-9][0-9]*$ && "$task" =~ ^ute-corpus-v1-[0-9]{3}$ ]] || return 1
    [[ "$call_id" == "$(gpt_opaque_id call "$mode" "$seq" "$task")" ]] || return 1
    [[ "$run_id" == "$(gpt_opaque_id run "$mode" "$seq" "$task")" ]] || return 1
  done < "$rows"
}

usage() {
  printf '%s\n' 'usage: ute_gpt_evidence.sh evaluate [--admission-generation v1|v2] --primary-ledger FILE --repo BARE_REPO --auto BIN --output DIR'
}

MODE=${1:-}
[[ "$MODE" == evaluate ]] || { usage >&2; exit 2; }
shift
PRIMARY_LEDGER= REPO= AUTO= OUTPUT= ADMISSION_GENERATION=v1 ADMISSION_SET=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --admission-generation) [[ $# -ge 2 && "$ADMISSION_SET" == false ]] || gpt_die "invalid --admission-generation"; ADMISSION_GENERATION=$2; ADMISSION_SET=true; shift 2 ;;
    --primary-ledger) [[ $# -ge 2 && -z "$PRIMARY_LEDGER" ]] || gpt_die "invalid --primary-ledger"; PRIMARY_LEDGER=$2; shift 2 ;;
    --repo) [[ $# -ge 2 && -z "$REPO" ]] || gpt_die "invalid --repo"; REPO=$2; shift 2 ;;
    --auto) [[ $# -ge 2 && -z "$AUTO" ]] || gpt_die "invalid --auto"; AUTO=$2; shift 2 ;;
    --output) [[ $# -ge 2 && -z "$OUTPUT" ]] || gpt_die "invalid --output"; OUTPUT=$2; shift 2 ;;
    *) gpt_die "unsupported argument" ;;
  esac
done
[[ -n "$PRIMARY_LEDGER" && -n "$REPO" && -n "$AUTO" && -n "$OUTPUT" ]] || gpt_die "all inputs are required"
[[ "$ADMISSION_GENERATION" == v1 || "$ADMISSION_GENERATION" == v2 ]] || gpt_die "unsupported admission generation"

gpt_require_tools
[[ -d "$STATIC_DIR" ]] || gpt_die "static evidence unavailable"
gpt_prepare_paths
WORK=$(mktemp -d "${TMPDIR:-/tmp}/ute-gpt-evidence-work.XXXXXX")
STAGE=$(mktemp -d "${TMPDIR:-/tmp}/ute-gpt-evidence-stage.XXXXXX")
ORACLES="$WORK/selected-oracles.json"
trap 'rm -rf "$WORK" "$STAGE"' EXIT INT TERM
chmod 700 "$WORK" "$STAGE"

gpt_validate_static "$STATIC_DIR" || gpt_die "static evidence validation failed"
gpt_prepare_identity
validate_repo_snapshot "$REPO" "$CORPUS" || gpt_die "repo snapshot validation failed"
gpt_validate_primary_ledger || gpt_die "primary ledger validation failed"
gpt_validate_oracles || gpt_die "oracle validation failed"
build_security_receipts || gpt_die "security receipt construction failed"
build_quality_ledger || gpt_die "quality ledger construction failed"
build_efficiency_input || gpt_die "efficiency input construction failed"
build_rollout_evidence || gpt_die "strict evaluator or rollout gate failed"
build_applied_rollback || gpt_die "applied rollback proof failed"
build_primary_summary || gpt_die "summary construction failed"
gpt_publish_stage || gpt_die "final evidence validation failed"
trap - EXIT INT TERM
rm -rf "$WORK" "$STAGE"
printf '%s\n' 'gpt evidence evaluation: PASS (provider_calls=0 promotion_eligible=false replay_eligible=true)'
