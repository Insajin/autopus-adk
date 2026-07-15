#!/usr/bin/env bash
set -euo pipefail
umask 077

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
# shellcheck source=ute_codex_canary_lib.sh
source "$SCRIPT_DIR/ute_codex_canary_lib.sh"

freeze_die() { printf 'ute-codex-canary-v2-freeze: %s\n' "$1" >&2; exit 1; }
freeze_sha() { printf 'sha256:%s\n' "$(sha256_file "$1")"; }
freeze_sidecar() {
  local file=$1
  printf '%s  %s\n' "$(sha256_file "$file")" "$(basename "$file")" > "${file%.json}.sha256"
  chmod 600 "$file" "${file%.json}.sha256"
}
freeze_canonicalize() {
  local file=$1 tmp="$1.sorted.$$"
  jq -S '.' "$file" > "$tmp"; chmod 600 "$tmp"; mv "$tmp" "$file"; freeze_sidecar "$file"
}
freeze_usage() {
  printf '%s\n' 'usage: ute_codex_canary_v2_freeze.sh freeze --evidence-dir DIR --repo BARE_REPO --auto BIN --output EMPTY_DIR --full-chain-sha256 sha256:HEX'
}
freeze_excluded_prior() {
  case "$1" in
    gpt-codex-policy-v2.*|gpt-codex-config-v2.*|gpt-verdict-schema-v2.*|gpt-canary-schedule-v2.*|\
    gpt-snapshot-manifest-v2.*|gpt-prior-evidence-manifest-v2.*|gpt-full-evaluation-identity-v2.*|\
    gpt-canary-preflight-v2.*|gpt-full-evaluation-authorization-v2.*|gpt-full-evaluation-reservation-v2.*|\
    gpt-primary-call-ledger-v2.*|gpt-rollback-call-ledger-v2.*|gpt-security-receipts-v2.*|\
    gpt-quality-ledger-v2.*|gpt-efficiency-input-v2.*|gpt-efficiency-result-v2.*|\
    gpt-rollout-*-v2.*|gpt-rollback-*-v2.*|gpt-applied-rollback-v2.*|\
    gpt-primary-evaluation-summary-v2.*|gpt-full-evaluation-terminal-outcome-v2.*|\
    gpt-full-evaluation-authorization-closure-v2.*) return 0 ;;
    *) return 1 ;;
  esac
}

freeze_compute_full_chain() {
  local manifest=$1 raw="$1.raw" name
  local members=(ute_codex_canary_v2.sh ute_codex_canary_v2_static.sh
    ute_codex_canary_v2_authorization.sh ute_codex_canary_v2_ledger.sh
    ute_codex_canary_v2_freeze.sh ute_codex_canary_v2_authorize.sh
    ute_codex_canary_lib.sh ute_codex_canary_prompt.sh ute_codex_canary_receipt.sh
    ute_codex_canary_exec.sh ute_codex_canary_schedule.sh ute_gpt_evidence.sh
    ute_gpt_evidence_lib.sh ute_gpt_evidence_build.sh ute_gpt_evidence_rollout.sh
    ute_gpt_full_evaluation_finalize.sh ute_gpt_full_evaluation_finalize_lib.sh)
  : > "$raw"
  for name in "${members[@]}"; do
    [[ -f "$SCRIPT_DIR/$name" && ! -L "$SCRIPT_DIR/$name" ]] || return 1
    printf '%s  scripts/%s\n' "$(sha256_file "$SCRIPT_DIR/$name")" "$name" >> "$raw"
  done
  LC_ALL=C sort -k2 "$raw" > "$manifest"
  rm -f "$raw"
  freeze_sha "$manifest"
}

freeze_compute_admission_bundle() {
  local manifest=$1 raw="$1.raw" name
  local members=(ute_codex_canary_v2.sh ute_codex_canary_v2_static.sh
    ute_codex_canary_v2_authorization.sh ute_codex_canary_v2_ledger.sh
    ute_codex_canary_lib.sh ute_codex_canary_prompt.sh ute_codex_canary_receipt.sh
    ute_codex_canary_exec.sh ute_codex_canary_schedule.sh)
  : > "$raw"
  for name in "${members[@]}"; do
    [[ -f "$SCRIPT_DIR/$name" && ! -L "$SCRIPT_DIR/$name" ]] || return 1
    printf '%s  scripts/%s\n' "$(sha256_file "$SCRIPT_DIR/$name")" "$name" >> "$raw"
  done
  LC_ALL=C sort -k2 "$raw" > "$manifest"; rm -f "$raw"; freeze_sha "$manifest"
}

freeze_resolve_executable() {
  local candidate link count=0
  candidate=$(command -v "$1") || return 1
  [[ "$candidate" == /* ]] || return 1
  while [[ -L "$candidate" ]]; do
    (( count+=1 )); (( count <= 40 )) || return 1
    link=$(readlink "$candidate") || return 1
    [[ "$link" == /* ]] && candidate=$link || candidate="$(dirname "$candidate")/$link"
    candidate="$(cd "$(dirname "$candidate")" 2>/dev/null && pwd -P)/$(basename "$candidate")" || return 1
  done
  candidate="$(cd "$(dirname "$candidate")" 2>/dev/null && pwd -P)/$(basename "$candidate")" || return 1
  [[ -f "$candidate" && -x "$candidate" && ! -L "$candidate" ]] || return 1
  printf '%s\n' "$candidate"
}

MODE=${1:-}
[[ "$MODE" == freeze ]] || { freeze_usage >&2; exit 2; }
shift
EVIDENCE_DIR= REPO= AUTO= OUTPUT= FULL_CHAIN_SHA256=
while [[ $# -gt 0 ]]; do
  [[ $# -ge 2 ]] || freeze_die "missing option value"
  case "$1" in
    --evidence-dir) [[ -z "$EVIDENCE_DIR" ]] || freeze_die "duplicate --evidence-dir"; EVIDENCE_DIR=$2 ;;
    --repo) [[ -z "$REPO" ]] || freeze_die "duplicate --repo"; REPO=$2 ;;
    --auto) [[ -z "$AUTO" ]] || freeze_die "duplicate --auto"; AUTO=$2 ;;
    --output) [[ -z "$OUTPUT" ]] || freeze_die "duplicate --output"; OUTPUT=$2 ;;
    --full-chain-sha256) [[ -z "$FULL_CHAIN_SHA256" ]] || freeze_die "duplicate --full-chain-sha256"; FULL_CHAIN_SHA256=$2 ;;
    *) freeze_die "unknown option" ;;
  esac
  shift 2
done
[[ "$FULL_CHAIN_SHA256" =~ ^sha256:[0-9a-f]{64}$ ]] || freeze_die "invalid full-chain hash"
for value in "$EVIDENCE_DIR" "$REPO" "$OUTPUT"; do [[ "$value" == /* ]] || freeze_die "paths must be absolute"; done
[[ "$AUTO" == /* && -f "$AUTO" && -x "$AUTO" && ! -L "$AUTO" ]] || freeze_die "exact Auto executable is required"
[[ -d "$EVIDENCE_DIR" && ! -L "$EVIDENCE_DIR" ]] || freeze_die "evidence directory unavailable"
[[ -d "$REPO" && ! -L "$REPO" ]] || freeze_die "bare repository unavailable"
[[ -d "$OUTPUT" && ! -L "$OUTPUT" && -z "$(find "$OUTPUT" -mindepth 1 -print -quit)" ]] || freeze_die "output must be an empty real directory"
EVIDENCE_DIR=$(cd "$EVIDENCE_DIR" && pwd -P); REPO=$(cd "$REPO" && pwd -P); OUTPUT=$(cd "$OUTPUT" && pwd -P)
[[ "$OUTPUT" != "$EVIDENCE_DIR" && "$OUTPUT" != "$REPO" ]] || freeze_die "output overlaps immutable input"
case "$OUTPUT/" in "$EVIDENCE_DIR/"*|"$REPO/"*) freeze_die "output overlaps immutable input" ;; esac
case "$EVIDENCE_DIR/" in "$OUTPUT/"*) freeze_die "output contains immutable input" ;; esac
case "$REPO/" in "$OUTPUT/"*) freeze_die "output contains immutable input" ;; esac
require_tools
validate_static_evidence "$EVIDENCE_DIR" || freeze_die "v1 static evidence validation failed"
CORPUS="$EVIDENCE_DIR/corpus-v1.json"; COHORT="$EVIDENCE_DIR/gpt-canary-cohort-v1.json"
TERMINAL="$EVIDENCE_DIR/gpt-transport-smoke-terminal-outcome-v8.json"
[[ -f "$CORPUS" && ! -L "$CORPUS" && -f "$COHORT" && ! -L "$COHORT" &&
   -f "$TERMINAL" && ! -L "$TERMINAL" && ! -L "${TERMINAL%.json}.sha256" ]] || \
  freeze_die "immutable input symlink rejected"
verify_named_sidecar "$TERMINAL" "${TERMINAL%.json}.sha256" || freeze_die "v8 terminal sidecar invalid"
jq -e '.outcome=="TRANSPORT_DIAGNOSIS_TERMINAL_PASS" and
  .decision.transport_precondition_for_full_evaluation_satisfied==true and
  .decision.full_58_plus_5_execution_authorized==false and
  .decision.next_full_evaluation_requires_new_frozen_admission_and_explicit_authorization==true' \
  "$TERMINAL" >/dev/null || freeze_die "v8 terminal contract invalid"
validate_repo_snapshot "$REPO" "$CORPUS" || freeze_die "repository snapshot invalid"
[[ "$(snapshot_git "$REPO" rev-parse --is-bare-repository)" == true ]] || freeze_die "repository must be bare"
AUTO_SHA=$(freeze_sha "$AUTO")
CODEX_BIN=$(freeze_resolve_executable codex) || freeze_die "exact Codex executable unavailable"
CODEX_EXECUTABLE_SHA=$(freeze_sha "$CODEX_BIN")
HARNESS="$SCRIPT_DIR/ute_codex_canary_v2.sh"
[[ -f "$HARNESS" ]] || freeze_die "v2 harness unavailable"
HARNESS_SHA=$(freeze_sha "$HARNESS"); CORPUS_SHA=$(freeze_sha "$CORPUS")
COHORT_SHA=$(freeze_sha "$COHORT"); TERMINAL_SHA=$(freeze_sha "$TERMINAL")
WORK=$(mktemp -d "${TMPDIR:-/tmp}/ute-v2-freeze.XXXXXX")
trap 'rm -rf "$WORK"' EXIT INT TERM
COMPUTED_FULL_CHAIN_SHA256=$(freeze_compute_full_chain "$WORK/full-chain.manifest") || \
  freeze_die "full-chain members unavailable"
[[ "$COMPUTED_FULL_CHAIN_SHA256" == "$FULL_CHAIN_SHA256" ]] || freeze_die "full-chain hash mismatch"
FULL_CHAIN_SHA256=$COMPUTED_FULL_CHAIN_SHA256
ADMISSION_BUNDLE_SHA256=$(freeze_compute_admission_bundle "$WORK/admission-bundle.manifest") || \
  freeze_die "admission-bundle members unavailable"
AUTO_VERSION=$("$AUTO" version --short 2>/dev/null) || freeze_die "Auto version unavailable"
[[ "$AUTO_VERSION" =~ ^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$ ]] || freeze_die "Auto version invalid"
[[ "$("$CODEX_BIN" --version 2>/dev/null)" == "codex-cli 0.144.1" ]] || freeze_die "Codex 0.144.1 required"
[[ "$(freeze_sha "$AUTO")" == "$AUTO_SHA" && "$(freeze_sha "$CODEX_BIN")" == "$CODEX_EXECUTABLE_SHA" ]] || \
  freeze_die "runtime executable changed during freeze"

SCHEMA="$OUTPUT/gpt-verdict-schema-v2.json"
jq -n '{type:"object",properties:{verdict:{type:"string",enum:["PASS","FAIL"]},
  finding_count:{type:"integer"}},required:["verdict","finding_count"],additionalProperties:false}' > "$SCHEMA"
freeze_canonicalize "$SCHEMA"; SCHEMA_SHA=$(freeze_sha "$SCHEMA")

PRIMARY_TSV="$WORK/primary.tsv"; ROLLBACK_TSV="$WORK/rollback.tsv"
emit_primary_schedule > "$PRIMARY_TSV"; emit_rollback_schedule > "$ROLLBACK_TSV"
validate_schedule_arithmetic "$PRIMARY_TSV" && validate_schedule_arithmetic "$ROLLBACK_TSV" || freeze_die "schedule arithmetic invalid"
jq -Rn '[inputs|split("\t")|{sequence:(.[0]|tonumber),task_id:.[1],arm:.[2],order:.[3],
  profile:.[4],role:.[5],role_ordinal:(.[6]|tonumber),effort:.[7],raw_token_budget:(.[8]|tonumber)}]' \
  "$PRIMARY_TSV" > "$WORK/primary.json"
jq -Rn '[inputs|split("\t")|{sequence:(.[0]|tonumber),task_id:.[1],arm:.[2],order:.[3],
  profile:.[4],role:.[5],role_ordinal:(.[6]|tonumber),effort:.[7],raw_token_budget:(.[8]|tonumber)}]' \
  "$ROLLBACK_TSV" > "$WORK/rollback.json"
SCHEDULE="$OUTPUT/gpt-canary-schedule-v2.json"
jq -n --slurpfile primary "$WORK/primary.json" --slurpfile rollback "$WORK/rollback.json" \
  '{version:2,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",evidence_kind:"gpt_codex_full_evaluation_schedule",
  admission_generation:"v2",primary:{calls:58,xhigh_calls:44,max_calls:14,worst_case_raw_tokens:1332000,
  rows:$primary[0]},rollback:{calls:5,xhigh_calls:4,max_calls:1,worst_case_raw_tokens:114000,
  rows:$rollback[0]},planned_total_calls:63,planned_worst_case_raw_tokens:1446000}' > "$SCHEDULE"
freeze_canonicalize "$SCHEDULE"; SCHEDULE_SHA=$(freeze_sha "$SCHEDULE")

HEAD_COMMIT=$(snapshot_git "$REPO" rev-parse HEAD); HEAD_TREE=$(snapshot_git "$REPO" rev-parse 'HEAD^{tree}')
jq '[.tasks[]|.base_commit,.target_commit]|unique|sort' "$CORPUS" > "$WORK/commits.json"
SNAPSHOT="$OUTPUT/gpt-snapshot-manifest-v2.json"
jq -n --slurpfile commits "$WORK/commits.json" --arg head "$HEAD_COMMIT" --arg tree "$HEAD_TREE" \
  --arg corpus "$CORPUS_SHA" '{version:2,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",
  evidence_kind:"gpt_codex_snapshot_manifest",admission_generation:"v2",
  repository:{bare:true,head_commit:$head,head_tree:$tree,object_format:"sha1"},
  source_corpus_sha256:$corpus,required_commits:$commits[0],absolute_paths_retained:false}' > "$SNAPSHOT"
freeze_canonicalize "$SNAPSHOT"; SNAPSHOT_SHA=$(freeze_sha "$SNAPSHOT")

PRIOR_ROWS="$WORK/prior.jsonl"; : > "$PRIOR_ROWS"
while IFS= read -r file; do
  name=$(basename "$file"); freeze_excluded_prior "$name" && continue
  jq -cn --arg name "$name" --arg hash "$(freeze_sha "$file")" '{name:$name,sha256:$hash}' >> "$PRIOR_ROWS"
done < <(find "$EVIDENCE_DIR" -maxdepth 1 -type f \( -name '*.json' -o -name '*.sha256' \) -print | LC_ALL=C sort)
PRIOR="$OUTPUT/gpt-prior-evidence-manifest-v2.json"
jq -n --slurpfile rows "$PRIOR_ROWS" '{version:2,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",
  evidence_kind:"gpt_codex_prior_evidence_manifest",admission_generation:"v2",
  artifact_count:($rows|length),artifacts:$rows,excluded_future_generation:"v2_dynamic_and_authorization",
  absolute_paths_retained:false}' > "$PRIOR"
freeze_canonicalize "$PRIOR"; PRIOR_SHA=$(freeze_sha "$PRIOR")

CONFIG="$OUTPUT/gpt-codex-config-v2.json"
jq '.version=2|.admission_generation="v2"|.evidence_kind="gpt_codex_full_evaluation_execution_config"|
  .risk_policy_source="gpt-codex-policy-v2.sha256"|.freeze={provider_calls_made:0,activation:false,
  promotion:false,implemented:false}' "$EVIDENCE_DIR/gpt-codex-config-v1.json" > "$CONFIG"
freeze_canonicalize "$CONFIG"; CONFIG_SHA=$(freeze_sha "$CONFIG")
[[ "$CONFIG_SHA" != "$(freeze_sha "$EVIDENCE_DIR/gpt-codex-config-v1.json")" ]] || freeze_die "v2 config did not change"

POLICY="$OUTPUT/gpt-codex-policy-v2.json"
  jq --arg harness "$HARNESS_SHA" --arg chain "$FULL_CHAIN_SHA256" --arg bundle "$ADMISSION_BUNDLE_SHA256" \
  --arg auto "$AUTO_SHA" --arg codex "$CODEX_EXECUTABLE_SHA" \
  --arg version "$AUTO_VERSION" --arg config "$CONFIG_SHA" --arg schema "$SCHEMA_SHA" \
  --arg schedule "$SCHEDULE_SHA" --arg snapshot "$SNAPSHOT_SHA" --arg prior "$PRIOR_SHA" \
  --arg corpus "$CORPUS_SHA" --arg cohort "$COHORT_SHA" --arg terminal "$TERMINAL_SHA" '
  .version=2|.admission_generation="v2"|.evidence_kind="gpt_codex_full_evaluation_policy"|
  .execution_runtime={harness_sha256:$harness,full_chain_harness_sha256:$chain,
    full_chain_algorithm:"sha256-named-member-manifest-v1",full_chain_member_count:17,
    admission_bundle_sha256:$bundle,admission_bundle_algorithm:"sha256-named-member-manifest-v1",
    admission_bundle_member_count:9,auto_executable_sha256:$auto,auto_version:$version,
    codex_executable_sha256:$codex,codex_cli_version:"0.144.1"}|
  .frozen_artifacts={config_sha256:$config,verdict_schema_sha256:$schema,schedule_sha256:$schedule,
    snapshot_manifest_sha256:$snapshot,prior_evidence_manifest_sha256:$prior,corpus_sha256:$corpus,
    cohort_sha256:$cohort,transport_terminal_sha256:$terminal}|
  .transport_gate={terminal_sha256:$terminal,status:"PASS",same_authorization_reusable:false}|
  .decision={status:"AWAITING_EXPLICIT_FULL_EVALUATION_AUTHORIZATION",activation:false,
    promotion:false,implemented:false}' "$EVIDENCE_DIR/gpt-codex-policy-v1.json" > "$POLICY"
freeze_canonicalize "$POLICY"; POLICY_SHA=$(freeze_sha "$POLICY")

IDENTITY="$OUTPUT/gpt-full-evaluation-identity-v2.json"
jq -n --arg policy "$POLICY_SHA" --arg config "$CONFIG_SHA" --arg schema "$SCHEMA_SHA" \
  --arg schedule "$SCHEDULE_SHA" --arg snapshot "$SNAPSHOT_SHA" --arg prior "$PRIOR_SHA" \
  --arg corpus "$CORPUS_SHA" --arg cohort "$COHORT_SHA" --arg terminal "$TERMINAL_SHA" \
  --arg harness "$HARNESS_SHA" --arg chain "$FULL_CHAIN_SHA256" --arg bundle "$ADMISSION_BUNDLE_SHA256" \
  --arg auto "$AUTO_SHA" --arg codex "$CODEX_EXECUTABLE_SHA" --arg version "$AUTO_VERSION" \
  '{version:2,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",evidence_kind:"gpt_codex_full_evaluation_identity",
  admission_generation:"v2",frozen_artifacts:{policy_sha256:$policy,config_sha256:$config,
  verdict_schema_sha256:$schema,schedule_sha256:$schedule,snapshot_manifest_sha256:$snapshot,
  prior_evidence_manifest_sha256:$prior,corpus_sha256:$corpus,cohort_sha256:$cohort,
  transport_terminal_sha256:$terminal},runtime:{harness_sha256:$harness,full_chain_harness_sha256:$chain,
  full_chain_algorithm:"sha256-named-member-manifest-v1",full_chain_member_count:17,
  admission_bundle_sha256:$bundle,admission_bundle_algorithm:"sha256-named-member-manifest-v1",
  admission_bundle_member_count:9,auto_executable_sha256:$auto,auto_version:$version,
  codex_executable_sha256:$codex,codex_cli_version:"0.144.1"},
  authorization_envelope:{provider_call_cap:64,raw_token_cap:1500000,primary_calls:58,rollback_calls:5,
  total_calls:63,planned_worst_case_raw_tokens:1446000,concurrency:1,retries:0},
  decision:{activation:false,promotion:false,implemented:false}}' > "$IDENTITY"
freeze_canonicalize "$IDENTITY"; IDENTITY_SHA=$(freeze_sha "$IDENTITY")

PREFLIGHT="$OUTPUT/gpt-canary-preflight-v2.json"
jq -n --arg identity "$IDENTITY_SHA" --arg policy "$POLICY_SHA" --arg config "$CONFIG_SHA" \
  --arg schema "$SCHEMA_SHA" --arg schedule "$SCHEDULE_SHA" --arg snapshot "$SNAPSHOT_SHA" \
  --arg prior "$PRIOR_SHA" --arg corpus "$CORPUS_SHA" --arg cohort "$COHORT_SHA" --arg terminal "$TERMINAL_SHA" \
  --arg harness "$HARNESS_SHA" --arg chain "$FULL_CHAIN_SHA256" --arg bundle "$ADMISSION_BUNDLE_SHA256" \
  --arg auto "$AUTO_SHA" --arg codex "$CODEX_EXECUTABLE_SHA" --arg version "$AUTO_VERSION" \
  --slurpfile v1_preflight "$EVIDENCE_DIR/gpt-canary-preflight-v1.json" \
  '{version:2,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",evidence_kind:"gpt_codex_full_evaluation_preflight",
  admission_generation:"v2",provider_receipt:false,authorization_identity:{artifact:"gpt-full-evaluation-identity-v2.json",
  sha256:$identity},frozen_artifacts:{policy_sha256:$policy,config_sha256:$config,verdict_schema_sha256:$schema,
  schedule_sha256:$schedule,snapshot_manifest_sha256:$snapshot,prior_evidence_manifest_sha256:$prior,
  corpus_sha256:$corpus,cohort_sha256:$cohort,transport_terminal_sha256:$terminal,
  full_evaluation_identity_sha256:$identity},runtime_candidate:{harness_sha256:$harness,
  full_chain_harness_sha256:$chain,full_chain_algorithm:"sha256-named-member-manifest-v1",
  full_chain_member_count:17,admission_bundle_sha256:$bundle,
  admission_bundle_algorithm:"sha256-named-member-manifest-v1",admission_bundle_member_count:9,
  auto_executable_sha256:$auto,auto_version:$version,codex_executable_sha256:$codex,codex_cli_version:"0.144.1"},
  deterministic_target_preflight:$v1_preflight[0].deterministic_target_preflight,
  observed_spend:{provider_calls_made:0,raw_total_tokens:0},decision:{status:"AWAITING_EXPLICIT_FULL_EVALUATION_AUTHORIZATION",
  provider_execution_started:false,activation:false,promotion:false,implemented:false}}' > "$PREFLIGHT"
freeze_canonicalize "$PREFLIGHT"

[[ "$(find "$OUTPUT" -maxdepth 1 -type f | wc -l | tr -d ' ')" == 16 ]] || freeze_die "unexpected output set"
trap - EXIT INT TERM; rm -rf "$WORK"
printf 'freeze=PASS artifacts=8 provider_calls=0 authorization=0 reservation=0 identity=%s\n' "$IDENTITY_SHA"
