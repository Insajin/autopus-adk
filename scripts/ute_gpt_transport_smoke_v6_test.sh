#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd -P)
SOURCE="$REPO_ROOT/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
TMP_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/ute-gpt-transport-v6-test.XXXXXX")
trap 'rm -rf "$TMP_ROOT"' EXIT
SNAPSHOT="$TMP_ROOT/snapshot.git"
FAKE_AUTO="$TMP_ROOT/auto"
AUTO_COUNT="$TMP_ROOT/auto-invocations"
PROVIDER_COUNT="$TMP_ROOT/provider-invocations"
INSTALL= OUTPUT= HARNESS= AUTH= CLAIM=

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
assert_eq() { [[ "$1" == "$2" ]] || fail "$3: want=$1 got=$2"; }
count_file() { [[ -s "$1" ]] && wc -l < "$1" | tr -d ' ' || printf '0\n'; }
sha() { shasum -a 256 "$1" | awk '{print $1}'; }
sidecar() { printf '%s  %s\n' "$(sha "$1")" "$(basename "$1")" > "${1%.json}.sha256"; }

make_fake_tools() {
  sed 's/^+//' > "$TMP_ROOT/codex" <<'EOF'
+#!/usr/bin/env sh
+[ "${1:-}" = --version ] && { printf '%s\n' 'codex-cli 0.144.1'; exit 0; }
+exit 90
EOF
  sed 's/^+//' > "$FAKE_AUTO" <<'EOF'
+#!/usr/bin/env bash
+set -euo pipefail
+printf '1\n' >> "${FAKE_AUTO_COUNT_FILE:?}"
+if [[ ${1:-} == version && ${2:-} == --short ]]; then
+  printf '%s\n' '0.50.68-ute-transport-diagnosis-v6'
+  exit 0
+fi
+[[ ${1:-} == agent && ${2:-} == run && ${3:-} == ute-corpus-v1-006 ]] || exit 91
+printf '1\n' >> "${FAKE_PROVIDER_COUNT_FILE:?}"
+events='[{"kind":"error","shape":["top_level_message","top_level_status_code"],"traits":["authentication"],"status_families":["http_4xx"]},{"kind":"turn_failed","shape":["nested_error_object","nested_error_message","nested_error_type","nested_error_code"],"traits":["model_access"],"status_families":[]}]'
+aggregate_kind=error_and_turn_failed
+aggregate_shape='["top_level_message","top_level_status_code","nested_error_object","nested_error_message","nested_error_type","nested_error_code"]'
+error_class=unknown
+error_fingerprint=sha256:e161c851c1a8c4fdea86c031ea524f1e0c7d39c7399eb950ea081fa8d90f0a42
+case "${FAKE_V6_MODE:-valid}" in
+  valid) ;;
+  stderr_only)
+    run_dir="$PWD/.autopus/runs/ute-corpus-v1-006"
+    jq -n '{task_id:"ute-corpus-v1-006",status:"failed",operational_error_class:"authentication",
+      operational_error_fingerprint:"sha256:7ef173ce9315debba3596a48c416d4681eadfd7f6eca40b046e8b1a0202816ec",
+      operational_error_stage:"process_wait",operational_error_signals:["stderr"],secret:"RAW-STDERR-SECRET"}' \
+      > "$run_dir/result.yaml"
+    exit 77 ;;
+  invalid_trait) events=$(jq -c '.[0].traits=["raw_private_trait"]' <<<"$events") ;;
+  invalid_order) events=$(jq -c 'reverse' <<<"$events") ;;
+  extra_event_key) events=$(jq -c '.[0].provider_message="EVENT-SECRET"' <<<"$events") ;;
+  aggregate_mismatch) aggregate_kind=error ;;
+  class_mismatch)
+    error_class=network_transport
+    error_fingerprint=sha256:e1d69bee7a31d8fa8a31c086db2cb48d614a01785d56b8fab493c5a01dc5de9a ;;
+  *) exit 92 ;;
+esac
+run_dir="$PWD/.autopus/runs/ute-corpus-v1-006"
+jq -n --arg kind "$aggregate_kind" --argjson shape "$aggregate_shape" --argjson events "$events" \
+  --arg class "$error_class" --arg fingerprint "$error_fingerprint" '
+  {task_id:"ute-corpus-v1-006",status:"failed",operational_error_class:$class,
+   operational_error_fingerprint:$fingerprint,
+   operational_error_stage:"process_wait",operational_error_signals:["provider_failure_event"],
+   operational_provider_event_kind:$kind,operational_provider_event_shape:$shape,
+   operational_provider_events:$events,provider_message:"RAW-PROVIDER-MESSAGE",
+   request_id:"RAW-REQUEST-ID","request-id":"RAW-HYPHEN-REQUEST-ID",secret:"RAW-TOP-SECRET"}' \
+  > "$run_dir/result.yaml"
+exit 77
EOF
  chmod 755 "$TMP_ROOT/codex" "$FAKE_AUTO"
}

make_install() {
  local label=$1 approval=${2:-awaiting} policy config schema preflight
  local harness_hash auto_hash p c s corpus tmp
  INSTALL="$TMP_ROOT/install-$label"
  OUTPUT="$INSTALL/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
  mkdir -p "$INSTALL/scripts" "$OUTPUT"
  cp "$SCRIPT_DIR/ute_gpt_transport_smoke.sh" "$INSTALL/scripts/"
  cp "$SCRIPT_DIR/ute_codex_canary_lib.sh" "$SCRIPT_DIR/ute_codex_canary_schedule.sh" "$INSTALL/scripts/"
  cp "$SOURCE/corpus-v1.json" "$SOURCE/corpus-v1.sha256" "$OUTPUT/"
  HARNESS="$INSTALL/scripts/ute_gpt_transport_smoke.sh"
  policy="$OUTPUT/gpt-transport-smoke-policy-v6.json"
  config="$OUTPUT/gpt-transport-smoke-config-v6.json"
  schema="$OUTPUT/gpt-transport-smoke-schema-v6.json"
  preflight="$OUTPUT/gpt-transport-smoke-preflight-v6.json"
  harness_hash="sha256:$(sha "$HARNESS")"
  auto_hash="sha256:$(sha "$FAKE_AUTO")"

  jq --arg h "$harness_hash" --arg a "$auto_hash" '
    .version=6 | .evidence_kind="gpt_codex_task006_provider_event_traits_diagnosis_policy" |
    .operational_failure_metadata.provider_event_trait_allowlist=["authentication","authorization_or_entitlement","model_access","rate_limit_or_quota","provider_unavailable","network_transport","request_validation","schema_or_response"] |
    .operational_failure_metadata.provider_event_status_family_allowlist=["http_4xx","http_5xx"] |
    .operational_failure_metadata.provider_event_receipt_required_keys=["kind","shape","traits","status_families"] |
    .operational_failure_metadata.provider_event_receipt_additional_keys=false |
    .operational_failure_metadata.provider_event_aggregate_must_match_events=true |
    .execution_runtime.harness_sha256=$h |
    .execution_runtime.auto_version="0.50.68-ute-transport-diagnosis-v6" |
    .execution_runtime.auto_executable_sha256=$a' "$SOURCE/gpt-transport-smoke-policy-v5.json" > "$policy"
  jq '.version=6 | .evidence_kind="gpt_codex_task006_provider_event_traits_diagnosis_config" |
    .context.codex.source_schema="gpt-transport-smoke-schema-v6.json" |
    .operational_failure_receipt.per_event_canonical_projection=true |
    .operational_failure_receipt.invalid_event_metadata_clears_all=true' \
    "$SOURCE/gpt-transport-smoke-config-v5.json" > "$config"
  jq '."$id"="urn:autopus:spec-adk-ultra-efficiency-001:gpt-transport-smoke:v6"' \
    "$SOURCE/gpt-transport-smoke-schema-v5.json" > "$schema"
  sidecar "$policy"; sidecar "$config"; sidecar "$schema"
  p="sha256:$(sha "$policy")"; c="sha256:$(sha "$config")"; s="sha256:$(sha "$schema")"
  corpus="sha256:$(sha "$OUTPUT/corpus-v1.json")"
  AUTH="sha256:$(printf '%s\0%s\0%s\0%s' "$p" "$c" "$s" "$corpus" | shasum -a 256 | awk '{print $1}')"
  jq --arg p "$p" --arg c "$c" --arg s "$s" --arg corpus "$corpus" --arg auth "$AUTH" \
    --arg h "$harness_hash" --arg a "$auto_hash" '
    .version=6 | .evidence_kind="gpt_codex_task006_provider_event_traits_diagnosis_preflight" |
    .frozen_artifacts={policy_sha256:$p,config_sha256:$c,schema_sha256:$s,corpus_sha256:$corpus} |
    .authorization_identity.sha256=$auth |
    .runtime_candidate={auto_version:"0.50.68-ute-transport-diagnosis-v6",auto_executable_sha256:$a,harness_sha256:$h,codex_cli_version:"0.144.1"} |
    .checks=[{name:"per_event_canonical_projection",status:"PASS"},{name:"invalid_event_metadata_fail_closed",status:"PASS"}] |
    .decision.status="AWAITING_EXPLICIT_LIVE_AUTHORIZATION" |
    .decision.provider_execution_started=false | .decision.authorized_at=null' \
    "$SOURCE/gpt-transport-smoke-preflight-v5.json" > "$preflight"
  if [[ "$approval" == approved ]]; then
    tmp="$OUTPUT/.preflight-approved.$$"
    jq '.decision.status="EXPLICIT_LIVE_AUTHORIZATION_GRANTED" |
      .decision.authorized_at="2026-07-15T12:00:00+09:00"' "$preflight" > "$tmp"
    mv "$tmp" "$preflight"
  fi
  sidecar "$preflight"
  for tmp in "$policy" "$config" "$schema" "$preflight"; do
    (cd "$OUTPUT" && shasum -a 256 -c "$(basename "${tmp%.json}.sha256")") >/dev/null
  done
  jq -e --arg auth "$AUTH" '.authorization_identity.sha256==$auth' "$preflight" >/dev/null
  CLAIM="$INSTALL/.autopus/runtime/ute-transport-smoke-authorizations/${AUTH#sha256:}"
}

run_failure() {
  local label=$1 mode=$2 state
  state="$TMP_ROOT/state-$label"
  mkdir "$state"
  if FAKE_AUTO_COUNT_FILE="$AUTO_COUNT" FAKE_PROVIDER_COUNT_FILE="$PROVIDER_COUNT" FAKE_V6_MODE="$mode" \
    UTE_GPT_TRANSPORT_PROTOCOL_VERSION=v6 UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$OUTPUT" \
    >/dev/null 2>&1; then
    fail "$label unexpectedly succeeded"
  fi
}

assert_cleared() {
  local ledger=$1
  jq -e '.operational_error_class==null and .operational_error_fingerprint==null and
    .operational_error_stage==null and .operational_error_signals==[] and
    .operational_provider_event_kind==null and .operational_provider_event_shape==[] and
    .operational_provider_events==[]' "$ledger" >/dev/null || fail "diagnostic metadata was not cleared"
  ! rg -q 'RAW-|EVENT-SECRET|raw_private_trait' "$ledger" || fail "invalid receipt retained raw metadata"
}

for base in policy config schema preflight; do
  (cd "$SOURCE" && shasum -a 256 -c "gpt-transport-smoke-$base-v5.sha256") >/dev/null
done
[[ "$(<"$SOURCE/corpus-v1.sha256")" == "sha256:$(sha "$SOURCE/corpus-v1.json")" ]] || fail "v5 corpus sidecar mismatch"
make_fake_tools
export PATH="$TMP_ROOT:$PATH"
git clone -q --bare "$REPO_ROOT" "$SNAPSHOT"

make_install awaiting awaiting
: > "$AUTO_COUNT"; : > "$PROVIDER_COUNT"
if ! output=$(UTE_GPT_TRANSPORT_PROTOCOL_VERSION=v6 "$HARNESS" preflight --repo "$SNAPSHOT" 2>&1); then
  printf 'RED: v6 preflight unavailable: %s\n' "$output" >&2
  exit 1
fi
state="$TMP_ROOT/state-awaiting"; mkdir "$state"
if FAKE_AUTO_COUNT_FILE="$AUTO_COUNT" FAKE_PROVIDER_COUNT_FILE="$PROVIDER_COUNT" \
  UTE_GPT_TRANSPORT_PROTOCOL_VERSION=v6 UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
  "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$OUTPUT" >/dev/null 2>&1; then
  fail "awaiting v6 authorization was executable"
fi
assert_eq 0 "$(count_file "$AUTO_COUNT")" "awaiting fake-auto invocation"
assert_eq 0 "$(count_file "$PROVIDER_COUNT")" "awaiting provider invocation"
[[ ! -e "$CLAIM" && ! -L "$CLAIM" ]] || fail "awaiting authorization created claim"

make_install valid approved
: > "$AUTO_COUNT"; : > "$PROVIDER_COUNT"
run_failure valid valid
ledger="$OUTPUT/gpt-transport-smoke-ledger-v6.partial-fail.json"
expected='[{"kind":"error","shape":["top_level_message","top_level_status_code"],"traits":["authentication"],"status_families":["http_4xx"]},{"kind":"turn_failed","shape":["nested_error_object","nested_error_message","nested_error_type","nested_error_code"],"traits":["model_access"],"status_families":[]}]'
jq -e --argjson events "$expected" '.completed==false and .attempted_calls==1 and
  .operational_provider_events==$events and .operational_provider_event_kind=="error_and_turn_failed" and
  .operational_provider_event_shape==["top_level_message","top_level_status_code","nested_error_object","nested_error_message","nested_error_type","nested_error_code"]' \
  "$ledger" >/dev/null || fail "valid per-event receipt was not projected exactly"
if rg -q 'RAW-PROVIDER-MESSAGE|RAW-REQUEST-ID|RAW-HYPHEN-REQUEST-ID|RAW-TOP-SECRET' "$ledger"; then
  fail "raw top-level provider values were retained"
fi
assert_eq 1 "$(count_file "$PROVIDER_COUNT")" "authorized provider attempts"
assert_eq 2 "$(count_file "$AUTO_COUNT")" "authorized fake-auto invocations"
[[ -f "$CLAIM/reservation.json" ]] || fail "single-use claim missing"
jq -e '.calls_reserved==1 and .retries==0 and .single_use==true and .state=="CONSUMED_ON_RESERVATION"' \
  "$CLAIM/reservation.json" >/dev/null || fail "claim was not consumed"
state="$TMP_ROOT/state-valid-reuse"; mkdir "$state"
FAKE_AUTO_COUNT_FILE="$AUTO_COUNT" FAKE_PROVIDER_COUNT_FILE="$PROVIDER_COUNT" \
  UTE_GPT_TRANSPORT_PROTOCOL_VERSION=v6 UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
  "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$OUTPUT" >/dev/null 2>&1 &&
  fail "single-use authorization was reused"
assert_eq 1 "$(count_file "$PROVIDER_COUNT")" "single-use retry count"
assert_eq 2 "$(count_file "$AUTO_COUNT")" "single-use fake-auto count"

make_install stderr-only approved
: > "$AUTO_COUNT"; : > "$PROVIDER_COUNT"
run_failure stderr-only stderr_only
ledger="$OUTPUT/gpt-transport-smoke-ledger-v6.partial-fail.json"
jq -e '.operational_error_class=="authentication" and .operational_error_stage=="process_wait" and
  .operational_error_signals==["stderr"] and .operational_provider_event_kind==null and
  .operational_provider_event_shape==[] and .operational_provider_events==[]' "$ledger" >/dev/null ||
  fail "stderr-only metadata was not retained canonically"
! rg -q 'RAW-STDERR-SECRET' "$ledger" || fail "stderr-only raw value was retained"

for mode in invalid_trait invalid_order extra_event_key aggregate_mismatch class_mismatch; do
  make_install "$mode" approved
  : > "$AUTO_COUNT"; : > "$PROVIDER_COUNT"
  run_failure "$mode" "$mode"
  assert_eq 1 "$(count_file "$PROVIDER_COUNT")" "$mode provider attempts"
  assert_cleared "$OUTPUT/gpt-transport-smoke-ledger-v6.partial-fail.json"
done

printf '%s\n' 'ute gpt transport smoke v6 hermetic tests: PASS'
