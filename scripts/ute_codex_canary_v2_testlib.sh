#!/usr/bin/env bash

# Hermetic only: every admitted call is handled by the fake Auto from the v1 testlib.
v2_sha() { shasum -a 256 "$1" | awk '{print $1}'; }
v2_bucket() { python3 -c 'import hashlib,sys;print(int.from_bytes(hashlib.sha256((sys.argv[1]+"\0"+sys.argv[2]).encode()).digest()[:8],"big")%100)' "$1" "$2"; }
v2_sidecar() {
  printf '%s  %s\n' "$(v2_sha "$1")" "$(basename "$1")" > "${1%.json}.sha256"
}

v2_require_sources() {
  local path stem
  for path in "$SCRIPT_DIR/ute_codex_canary_v2.sh" "$SCRIPT_DIR/ute_codex_canary_v2_freeze.sh"; do
    [[ -f "$path" ]] || { printf 'RED: missing v2 harness/artifact: %s\n' "$path" >&2; exit 1; }
  done
  for stem in corpus-v1 gpt-canary-cohort-v1 gpt-canary-preflight-v1 \
    gpt-codex-config-v1 gpt-codex-policy-v1 gpt-verdict-schema-v1 \
    gpt-primary-call-ledger-v1.partial-fail gpt-transport-smoke-terminal-outcome-v8; do
    for path in "$SOURCE/$stem.json" "$SOURCE/$stem.sha256"; do
      [[ -f "$path" ]] || { printf 'RED: missing v2 harness/artifact: %s\n' "$path" >&2; exit 1; }
    done
  done
}

v2_wrap_launcher() {
  local path=$1 kind=$2 real="$1.real"
  mv "$path" "$real"
  if [[ "$kind" == auto ]]; then
    sed 's/^+//' > "$path" <<'WRAP'
+#!/usr/bin/env bash
+set -euo pipefail
+if [[ ${1:-} == version && ${2:-} == --short && -n ${V2_AUTO_VERSION_COUNT_FILE:-} ]]; then
+  printf '1\n' >> "$V2_AUTO_VERSION_COUNT_FILE"
+  if [[ -n ${V2_VERSION_BARRIER_READY:-} && -n ${V2_VERSION_BARRIER_RELEASE:-} ]]; then
+    : > "$V2_VERSION_BARRIER_READY"
+    while [[ ! -e "$V2_VERSION_BARRIER_RELEASE" ]]; do sleep 0.01; done
+  fi
+fi
+exec "${BASH_SOURCE[0]}.real" "$@"
WRAP
  else
    sed 's/^+//' > "$path" <<'WRAP'
+#!/usr/bin/env bash
+set -euo pipefail
+if [[ ${1:-} == --version && -n ${V2_CODEX_VERSION_COUNT_FILE:-} ]]; then
+  printf '1\n' >> "$V2_CODEX_VERSION_COUNT_FILE"
+fi
+exec "${BASH_SOURCE[0]}.real" "$@"
WRAP
  fi
  chmod 755 "$path"
}

v2_named_manifest_hash() {
  local dir=$1; shift
  local tmp="$TMP_ROOT/manifest.$$" name
  : > "$tmp"
  for name in "$@"; do printf '%s  scripts/%s\n' "$(v2_sha "$dir/$name")" "$name" >> "$tmp"; done
  LC_ALL=C sort -k2 "$tmp" -o "$tmp"
  printf 'sha256:%s\n' "$(v2_sha "$tmp")"; rm -f "$tmp"
}

v2_full_chain_hash() {
  v2_named_manifest_hash "${1:-$SCRIPT_DIR}" ute_codex_canary_v2.sh ute_codex_canary_v2_static.sh \
    ute_codex_canary_v2_authorization.sh ute_codex_canary_v2_ledger.sh \
    ute_codex_canary_v2_freeze.sh ute_codex_canary_v2_authorize.sh ute_codex_canary_lib.sh \
    ute_codex_canary_prompt.sh ute_codex_canary_receipt.sh ute_codex_canary_exec.sh \
    ute_codex_canary_schedule.sh ute_gpt_evidence.sh ute_gpt_evidence_lib.sh \
    ute_gpt_evidence_build.sh ute_gpt_evidence_rollout.sh \
    ute_gpt_full_evaluation_finalize.sh ute_gpt_full_evaluation_finalize_lib.sh
}

v2_admission_bundle_hash() {
  v2_named_manifest_hash "$1" ute_codex_canary_v2.sh ute_codex_canary_v2_static.sh \
    ute_codex_canary_v2_authorization.sh ute_codex_canary_v2_ledger.sh ute_codex_canary_lib.sh \
    ute_codex_canary_prompt.sh ute_codex_canary_receipt.sh ute_codex_canary_exec.sh ute_codex_canary_schedule.sh
}

v2_snapshot_v1() {
  (cd "$OUTPUT" && find . -type f -name '*v1*' -print | LC_ALL=C sort | \
    while IFS= read -r file; do shasum -a 256 "$file"; done)
}

v2_make_install() {
  local label=$1 auto=$2 stem file
  local p c s schedule snapshot prior corpus cohort terminal identity preflight bundle chain codex
  INSTALL="$TMP_ROOT/install-$label"
  OUTPUT="$INSTALL/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
  mkdir -p "$INSTALL/scripts" "$OUTPUT"
  cp "$SCRIPT_DIR"/ute_codex_canary*.sh "$INSTALL/scripts/"
  cp "$SCRIPT_DIR"/ute_gpt_evidence*.sh "$SCRIPT_DIR"/ute_gpt_full_evaluation_finalize*.sh "$INSTALL/scripts/"
  for stem in corpus-v1 gpt-canary-cohort-v1 gpt-canary-preflight-v1 \
    gpt-codex-config-v1 gpt-codex-policy-v1 gpt-verdict-schema-v1 \
    gpt-primary-call-ledger-v1.partial-fail gpt-transport-smoke-terminal-outcome-v8; do
    cp "$SOURCE/$stem.json" "$SOURCE/$stem.sha256" "$OUTPUT/"
  done
  for stem in gpt-codex-policy-v2 gpt-codex-config-v2 gpt-verdict-schema-v2 \
    gpt-canary-schedule-v2 gpt-snapshot-manifest-v2 gpt-prior-evidence-manifest-v2 \
    gpt-full-evaluation-identity-v2 gpt-canary-preflight-v2; do
    cp "$FROZEN/$stem.json" "$FROZEN/$stem.sha256" "$OUTPUT/"
  done
  HARNESS="$INSTALL/scripts/ute_codex_canary_v2.sh"
  p="$OUTPUT/gpt-codex-policy-v2.json"; c="$OUTPUT/gpt-codex-config-v2.json"
  s="$OUTPUT/gpt-verdict-schema-v2.json"; schedule="$OUTPUT/gpt-canary-schedule-v2.json"
  snapshot="$OUTPUT/gpt-snapshot-manifest-v2.json"; prior="$OUTPUT/gpt-prior-evidence-manifest-v2.json"
  identity="$OUTPUT/gpt-full-evaluation-identity-v2.json"; preflight="$OUTPUT/gpt-canary-preflight-v2.json"
  corpus="sha256:$(v2_sha "$OUTPUT/corpus-v1.json")"; cohort="sha256:$(v2_sha "$OUTPUT/gpt-canary-cohort-v1.json")"
  terminal="sha256:$(v2_sha "$OUTPUT/gpt-transport-smoke-terminal-outcome-v8.json")"
  c="sha256:$(v2_sha "$c")"; s="sha256:$(v2_sha "$s")"; schedule="sha256:$(v2_sha "$schedule")"
  snapshot="sha256:$(v2_sha "$snapshot")"; prior="sha256:$(v2_sha "$prior")"
  bundle=$(v2_admission_bundle_hash "$INSTALL/scripts")
  chain=$(v2_full_chain_hash "$INSTALL/scripts")
  codex="sha256:$(v2_sha "$(command -v codex)")"
  file="$OUTPUT/.policy.$$"
  jq --arg harness "sha256:$(v2_sha "$HARNESS")" --arg auto "sha256:$(v2_sha "$auto")" \
    --arg bundle "$bundle" --arg chain "$chain" --arg codex "$codex" \
    --arg c "$c" --arg s "$s" --arg schedule "$schedule" --arg snapshot "$snapshot" \
    --arg prior "$prior" --arg corpus "$corpus" --arg cohort "$cohort" --arg terminal "$terminal" '
    .execution_runtime.harness_sha256=$harness |
    .execution_runtime.full_chain_harness_sha256=$chain |
    .execution_runtime.admission_bundle_sha256=$bundle |
    .execution_runtime.auto_executable_sha256=$auto | .execution_runtime.auto_version="0.50.99-test" |
    .execution_runtime.codex_executable_sha256=$codex |
    .frozen_artifacts={config_sha256:$c,verdict_schema_sha256:$s,schedule_sha256:$schedule,
      snapshot_manifest_sha256:$snapshot,prior_evidence_manifest_sha256:$prior,corpus_sha256:$corpus,
      cohort_sha256:$cohort,transport_terminal_sha256:$terminal}' "$p" > "$file"
  mv "$file" "$p"; v2_sidecar "$p"; p="sha256:$(v2_sha "$p")"
  file="$OUTPUT/.identity.$$"
  jq --arg p "$p" --arg c "$c" --arg s "$s" --arg schedule "$schedule" --arg snapshot "$snapshot" \
    --arg prior "$prior" --arg corpus "$corpus" --arg cohort "$cohort" --arg terminal "$terminal" \
    --arg harness "sha256:$(v2_sha "$HARNESS")" --arg auto "sha256:$(v2_sha "$auto")" \
    --arg bundle "$bundle" --arg chain "$chain" --arg codex "$codex" '
    .version=2 | .admission_generation="v2" |
    .frozen_artifacts={policy_sha256:$p,config_sha256:$c,verdict_schema_sha256:$s,
      schedule_sha256:$schedule,snapshot_manifest_sha256:$snapshot,prior_evidence_manifest_sha256:$prior,
      corpus_sha256:$corpus,cohort_sha256:$cohort,transport_terminal_sha256:$terminal} |
    .runtime={harness_sha256:$harness,
      full_chain_harness_sha256:$chain,full_chain_algorithm:"sha256-named-member-manifest-v1",
      full_chain_member_count:17,
      admission_bundle_sha256:$bundle,admission_bundle_algorithm:"sha256-named-member-manifest-v1",
      admission_bundle_member_count:9,auto_executable_sha256:$auto,auto_version:"0.50.99-test",
      codex_executable_sha256:$codex,codex_cli_version:"0.144.1"}' \
    "$identity" > "$file"
  mv "$file" "$identity"; v2_sidecar "$identity"; AUTH="sha256:$(v2_sha "$identity")"
  file="$OUTPUT/.preflight.$$"
  jq --arg auth "$AUTH" --arg identity "sha256:$(v2_sha "$identity")" --arg p "$p" \
    --arg c "$c" --arg s "$s" --arg auto "sha256:$(v2_sha "$auto")" \
    --arg bundle "$bundle" --arg chain "$chain" --arg codex "$codex" --arg harness "sha256:$(v2_sha "$HARNESS")" '
    .version=2 | .admission_generation="v2" | .authorization_identity.sha256=$auth |
    .frozen_artifacts.policy_sha256=$p | .frozen_artifacts.config_sha256=$c |
    .frozen_artifacts.verdict_schema_sha256=$s | .frozen_artifacts.full_evaluation_identity_sha256=$identity |
    .runtime_candidate.harness_sha256=$harness | .runtime_candidate.admission_bundle_sha256=$bundle |
    .runtime_candidate.full_chain_harness_sha256=$chain |
    .runtime_candidate.auto_executable_sha256=$auto | .runtime_candidate.auto_version="0.50.99-test" |
    .runtime_candidate.codex_executable_sha256=$codex |
    .observed_spend.provider_calls_made=0 | .observed_spend.raw_total_tokens=0 |
    .decision.status="AWAITING_EXPLICIT_FULL_EVALUATION_AUTHORIZATION" |
    .decision.provider_execution_started=false | .decision.activation=false |
    .decision.promotion=false | .decision.implemented=false' "$preflight" > "$file"
  mv "$file" "$preflight"; v2_sidecar "$preflight"
  PREFLIGHT="sha256:$(v2_sha "$preflight")"
  CLAIM="$INSTALL/.autopus/runtime/ute-full-evaluation-authorizations/${AUTH#sha256:}"
  V1_BEFORE=$(v2_snapshot_v1)
}

v2_write_authorization() {
  local mode=${1:-exact} receipt="$OUTPUT/gpt-full-evaluation-authorization-v2.json" auth=$AUTH
  [[ "$mode" != wrong ]] || auth="sha256:$(printf '0%.0s' {1..64})"
  jq -n --arg auth "$auth" --arg policy "sha256:$(v2_sha "$OUTPUT/gpt-codex-policy-v2.json")" \
    --arg config "sha256:$(v2_sha "$OUTPUT/gpt-codex-config-v2.json")" --arg preflight "$PREFLIGHT" '
    {version:2,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",evidence_kind:"gpt_codex_full_evaluation_authorization",
    decision:"EXPLICIT_FULL_EVALUATION_AUTHORIZATION_GRANTED",authorization_source:"user_exact_identity_confirmation",
    authorization_identity_sha256:$auth,policy_sha256:$policy,config_sha256:$config,preflight_sha256:$preflight,
    provider:"codex",model:"gpt-5.6-sol",provider_call_cap:64,raw_token_cap:1500000,
    primary_calls:58,rollback_calls:5,total_calls:63,planned_worst_case_raw_tokens:1446000,
    concurrency:1,retries:0,single_use:true,provider_execution_started:false,
    activation:false,promotion:false,implemented:false,authorized_at:"2026-07-15T13:00:00+09:00"}' > "$receipt"
  v2_sidecar "$receipt"
}

v2_assert_zero_and_unclaimed() {
  assert_eq "0" "$(invocation_count "$COUNT_FILE")" "$1 invoked fake Auto"
  [[ ! -e "$CLAIM" && ! -L "$CLAIM" ]] || fail "$1 created authorization claim"
}

v2_tamper_full17() {
  local file="$INSTALL/scripts/ute_gpt_evidence_lib.sh" tmp="$TMP_ROOT/full17.tampered"
  awk 'NR==2 {print "printf sourced >> \"${V2_HELPER_SENTINEL:?}\""} {print}' "$file" > "$tmp"
  mv "$tmp" "$file"
}

v2_phase_tamper() {
  local mode=$1 label=$2 before ready release status pid helper
  before=$(invocation_count "$COUNT_FILE"); ready="$TMP_ROOT/$label.ready"; release="$TMP_ROOT/$label.release"
  status="$TMP_ROOT/$label.status"; helper="$INSTALL/scripts/ute_gpt_evidence_lib.sh"
  export V2_VERSION_BARRIER_READY="$ready" V2_VERSION_BARRIER_RELEASE="$release"
  ( set +e; if [[ "$mode" == primary ]]; then v2_primary "$FAKE_AUTO" "$TMP_ROOT/state-$label" >/dev/null 2>&1
    else v2_rollback "$FAKE_AUTO" "$TMP_ROOT/state-$label" >/dev/null 2>&1; fi; printf '%s\n' "$?" > "$status" ) & pid=$!
  for _ in {1..3000}; do [[ -e "$ready" ]] && break; sleep 0.01; done
  [[ -e "$ready" ]] || fail "$label phase barrier unavailable"
  printf '\n# full17 phase tamper\n' >> "$helper"; : > "$release"; wait "$pid"
  unset V2_VERSION_BARRIER_READY V2_VERSION_BARRIER_RELEASE
  [[ "$(<"$status")" != 0 ]] || fail "$label phase tamper admitted"
  cp "$SCRIPT_DIR/ute_gpt_evidence_lib.sh" "$helper"
  assert_eq "$before" "$(invocation_count "$COUNT_FILE")" "$label phase tamper added provider calls"
  if [[ "$mode" == primary ]]; then [[ ! -e "$CLAIM" ]] || fail "$label created primary claim"
  else [[ ! -e "$CLAIM/rollback-reservation" ]] || fail "$label created rollback claim"; fi
}

v2_nested_contract() (
  local action=${1:-validate}
  SCRIPT_DIR="$INSTALL/scripts"; EVIDENCE_DIR="$OUTPUT"; OUTPUT="$EVIDENCE_DIR"
  source "$SCRIPT_DIR/ute_codex_canary_lib.sh"; source "$SCRIPT_DIR/ute_codex_canary_v2_static.sh"
  source "$SCRIPT_DIR/ute_codex_canary_v2_ledger.sh"; source "$SCRIPT_DIR/ute_codex_canary_v2_authorization.sh"
  V2_AUTHORIZATION_IDENTITY_SHA256=$AUTH; V2_AUTHORIZATION_CLAIM_DIR=$CLAIM
  POLICY_HASH="sha256:$(v2_sha "$OUTPUT/gpt-codex-policy-v2.json")"
  CONFIG_HASH="sha256:$(v2_sha "$OUTPUT/gpt-codex-config-v2.json")"; AUTO_SHA="sha256:$(v2_sha "$FAKE_AUTO")"
  AUTO_VERSION=0.50.99-test; PRIMARY_LEDGER_HASH="sha256:$(v2_sha "$OUTPUT/gpt-primary-call-ledger-v2.json")"
  ROLLBACK_RECEIPT_HASH="sha256:$(v2_sha "$OUTPUT/gpt-applied-rollback-v2.json")"
  V2_EVALUATOR_SUMMARY_SHA256="sha256:$(v2_sha "$OUTPUT/gpt-primary-evaluation-summary-v2.json")"
  V2_RESERVATION_SHA256="sha256:$(v2_sha "$OUTPUT/gpt-full-evaluation-reservation-v2.json")"
  if [[ "$action" == validate ]]; then v2_validate_rollback_reservation; else v2_reserve_rollback_authorization; fi
)

v2_primary() {
  local auto=$1 state=$2 output=${3:-$OUTPUT}
  mkdir -p "$state"
  FAKE_AUTO_COUNT_FILE="$COUNT_FILE" UTE_CODEX_CANARY_V2_EXECUTE=YES \
    "$HARNESS" primary --repo "$SNAPSHOT" --auto "$auto" --state "$state" --output "$output"
}

v2_rollback() {
  local auto=$1 state=$2 primary="$OUTPUT/gpt-primary-call-ledger-v2.json"
  local applied="$OUTPUT/gpt-applied-rollback-v2.json"
  primary="$(cd "$(dirname "$primary")" && pwd -P)/$(basename "$primary")"
  applied="$(cd "$(dirname "$applied")" && pwd -P)/$(basename "$applied")"
  mkdir -p "$state"
  FAKE_AUTO_COUNT_FILE="$COUNT_FILE" UTE_CODEX_CANARY_V2_EXECUTE=YES \
    "$HARNESS" rollback --repo "$SNAPSHOT" --auto "$auto" --state "$state" --output "$OUTPUT" \
    --primary-ledger "$primary" --rollback-receipt "$applied" --rollback-receipt-hash "sha256:$(v2_sha "$applied")"
}

v2_reset_runtime_counts() { : > "$AUTO_VERSION_FILE"; : > "$CODEX_VERSION_FILE"; }
v2_assert_no_versions() {
  assert_eq 0 "$(invocation_count "$AUTO_VERSION_FILE")" "$1 executed Auto version"
  assert_eq 0 "$(invocation_count "$CODEX_VERSION_FILE")" "$1 executed Codex version"
}

v2_freeze_candidate() {
  FROZEN="$TMP_ROOT/frozen"; mkdir "$FROZEN"
  "$SCRIPT_DIR/ute_codex_canary_v2_freeze.sh" freeze --evidence-dir "$SOURCE" --repo "$SNAPSHOT" \
    --auto "$FAKE_AUTO" --output "$FROZEN" --full-chain-sha256 "$(v2_full_chain_hash)" >/dev/null
  jq -e --arg auto "sha256:$(v2_sha "$FAKE_AUTO")" --arg codex "sha256:$(v2_sha "$FAKE_CODEX")" \
    --arg bundle "$(v2_admission_bundle_hash "$SCRIPT_DIR")" '
    .runtime.auto_executable_sha256==$auto and .runtime.codex_executable_sha256==$codex and
    .runtime.admission_bundle_sha256==$bundle and .runtime.admission_bundle_member_count==9' \
    "$FROZEN/gpt-full-evaluation-identity-v2.json" >/dev/null || fail "frozen runtime identity"
}
