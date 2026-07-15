#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd -P)
SOURCE="$REPO_ROOT/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
TMP_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/ute-gpt-transport-v7-test.XXXXXX")
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

verify_frozen_v1_v6() {
  local version base
  for version in v1 v2 v3 v4 v5 v6; do
    for base in policy config schema preflight; do
      (cd "$SOURCE" && shasum -a 256 -c "gpt-transport-smoke-$base-$version.sha256") >/dev/null
    done
  done
  [[ "$(<"$SOURCE/corpus-v1.sha256")" == "sha256:$(sha "$SOURCE/corpus-v1.json")" ]] ||
    fail "frozen corpus sidecar mismatch"
}

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
+  printf '%s\n' '0.50.68-ute-transport-diagnosis-v7'
+  exit 0
+fi
+[[ ${1:-} == agent && ${2:-} == run && ${3:-} == ute-corpus-v1-006 ]] || exit 91
+printf '1\n' >> "${FAKE_PROVIDER_COUNT_FILE:?}"
+run_dir="$PWD/.autopus/runs/ute-corpus-v1-006"
+if [[ ${FAKE_V7_MODE:-success} == failure ]]; then
+  jq -n '{task_id:"ute-corpus-v1-006",status:"failed",operational_error_class:"authentication",
+    operational_error_fingerprint:"sha256:7ef173ce9315debba3596a48c416d4681eadfd7f6eca40b046e8b1a0202816ec",
+    operational_error_stage:"process_wait",operational_error_signals:["stderr"],
+    provider_message:"RAW-V7-PROVIDER-ERROR",secret:"RAW-V7-SECRET"}' > "$run_dir/result.yaml"
+  printf '%s\n' 'RAW-V7-STDERR' >&2
+  exit 77
+fi
+ctx="$run_dir/context.yaml"
+jq -e '.task_id == "ute-corpus-v1-006" and .provider == "codex" and .model == "gpt-5.6-sol" and
+  .effort == "xhigh" and .role == "reviewer" and .evidence_mode == true and .diagnostic_mode == true and
+  .strict_verdict == true and .zero_tool_calls_required == true and .codex.raw_token_budget == 22000 and
+  .codex.output_schema == "gpt-diagnostic-verdict-schema-v1.json"' "$ctx" >/dev/null
+get() { jq -r "$1" "$ctx"; }
+run=$(get .run_id); call=$(get .call_id); policy=$(get .risk_policy); config=$(get .config_hash)
+output_hash=$(printf '%s' FAKE-V7-RAW | shasum -a 256 | awk '{print $1}')
+if [[ ${FAKE_V7_MODE:-success} == producer_omitempty ]]; then
+  jq -n --arg run "$run" --arg call "$call" --arg output "$output_hash" '
+    {task_id:"ute-corpus-v1-006",status:"success",provider:"codex",model:"gpt-5.6-sol",effort:"xhigh",
+     run_id:$run,call_id:$call,attempt:1,phase:"review",role:"reviewer",verdict:"PASS",finding_count:0,
+     output_sha256:$output,usage_status:"actual",unique_model_call_count:1,raw_total_tokens:100,tool_calls:0}' > "$run_dir/result.yaml"
+elif [[ ${FAKE_V7_MODE:-success} == producer_nonempty ]]; then
+  jq -n --arg run "$run" --arg call "$call" --arg output "$output_hash" '
+    {task_id:"ute-corpus-v1-006",status:"success",provider:"codex",model:"gpt-5.6-sol",effort:"xhigh",
+     run_id:$run,call_id:$call,attempt:1,phase:"review",role:"reviewer",verdict:"PASS",finding_count:0,
+     finding_codes:["correctness"],finding_scope_hashes:[],output_sha256:$output,usage_status:"actual",
+     unique_model_call_count:1,raw_total_tokens:100,tool_calls:0}' > "$run_dir/result.yaml"
+elif [[ ${FAKE_V7_MODE:-success} == producer_null ]]; then
+  jq -n --arg run "$run" --arg call "$call" --arg output "$output_hash" '
+    {task_id:"ute-corpus-v1-006",status:"success",provider:"codex",model:"gpt-5.6-sol",effort:"xhigh",
+     run_id:$run,call_id:$call,attempt:1,phase:"review",role:"reviewer",verdict:"PASS",finding_count:0,
+     finding_codes:null,finding_scope_hashes:null,output_sha256:$output,usage_status:"actual",
+     unique_model_call_count:1,raw_total_tokens:100,tool_calls:0}' > "$run_dir/result.yaml"
+else
+  jq -n --arg run "$run" --arg call "$call" --arg output "$output_hash" '
+    {task_id:"ute-corpus-v1-006",status:"success",provider:"codex",model:"gpt-5.6-sol",effort:"xhigh",
+     run_id:$run,call_id:$call,attempt:1,phase:"review",role:"reviewer",verdict:"PASS",finding_count:0,
+     finding_codes:[],finding_scope_hashes:[],output_sha256:$output,usage_status:"actual",
+     unique_model_call_count:1,raw_total_tokens:100,tool_calls:0}' > "$run_dir/result.yaml"
+fi
+mkdir -p "$PWD/.autopus/telemetry"
+jq -cn --arg run "$run" --arg call "$call" --arg policy "$policy" --arg config "$config" '
+  {type:"agent_run",data:{agent_name:"reviewer",spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",
+   task_id:"ute-corpus-v1-006",run_id:$run,call_id:$call,attempt:1,provider:"codex",model:"gpt-5.6-sol",
+   effort:"xhigh",phase:"review",role:"reviewer",status:"PASS",acceptance_status:"PASS",tool_calls:0,
+   usage:[{version:1,run_id:$run,call_id:$call,task_id:"ute-corpus-v1-006",attempt:1,provider:"codex",
+   model:"gpt-5.6-sol",effort:"xhigh",provider_version:"0.144.1",model_version:"gpt-5.6-sol",
+   risk_policy:$policy,cache_stratum:"provider-managed-stable-prefix-v1",config_hash:$config,phase:"review",
+   role:"reviewer",usage_status:"actual",usage_source:"provider",source_schema:"codex.exec-json.turn.completed.v1",
+   input_tokens_total:90,output_tokens_total:10,raw_total_tokens:100}]}}' > "$PWD/.autopus/telemetry/smoke.jsonl"
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
  policy="$OUTPUT/gpt-transport-smoke-policy-v7.json"
  config="$OUTPUT/gpt-transport-smoke-config-v7.json"
  schema="$OUTPUT/gpt-transport-smoke-schema-v7.json"
  preflight="$OUTPUT/gpt-transport-smoke-preflight-v7.json"
  harness_hash="sha256:$(sha "$HARNESS")"; auto_hash="sha256:$(sha "$FAKE_AUTO")"

  jq --arg h "$harness_hash" --arg a "$auto_hash" '
    .version=7 | .evidence_kind="gpt_codex_task006_structured_outputs_transport_policy" |
    .execution_runtime.harness_sha256=$h |
    .execution_runtime.auto_version="0.50.68-ute-transport-diagnosis-v7" |
    .execution_runtime.auto_executable_sha256=$a' "$SOURCE/gpt-transport-smoke-policy-v6.json" > "$policy"
  jq '.version=7 | .evidence_kind="gpt_codex_task006_structured_outputs_transport_config" |
    .context.codex.source_schema="gpt-transport-smoke-schema-v7.json"' \
    "$SOURCE/gpt-transport-smoke-config-v6.json" > "$config"
  jq -n '{type:"object",additionalProperties:false,
    required:["verdict","finding_count","finding_codes","finding_scope_hashes"],
    properties:{verdict:{type:"string",enum:["PASS"]},finding_count:{type:"integer",enum:[0]},
      finding_codes:{type:"array",items:{type:"string"}},
      finding_scope_hashes:{type:"array",items:{type:"string"}}}}' > "$schema"
  jq -e '. == {type:"object",additionalProperties:false,
    required:["verdict","finding_count","finding_codes","finding_scope_hashes"],
    properties:{verdict:{type:"string",enum:["PASS"]},finding_count:{type:"integer",enum:[0]},
      finding_codes:{type:"array",items:{type:"string"}},
      finding_scope_hashes:{type:"array",items:{type:"string"}}}}' "$schema" >/dev/null ||
    fail "v7 conservative schema mismatch"
  jq -e 'all(.. | objects | keys[];
    . != "$schema" and . != "$id" and . != "const" and . != "maxItems" and . != "minItems" and
    . != "uniqueItems" and . != "pattern" and . != "allOf" and . != "anyOf" and . != "oneOf" and
    . != "not" and . != "if" and . != "then" and . != "else")' "$schema" >/dev/null ||
    fail "v7 schema contains a prohibited keyword"
  sidecar "$policy"; sidecar "$config"; sidecar "$schema"
  p="sha256:$(sha "$policy")"; c="sha256:$(sha "$config")"; s="sha256:$(sha "$schema")"
  corpus="sha256:$(sha "$OUTPUT/corpus-v1.json")"
  AUTH="sha256:$(printf '%s\0%s\0%s\0%s' "$p" "$c" "$s" "$corpus" | shasum -a 256 | awk '{print $1}')"
  jq --arg p "$p" --arg c "$c" --arg s "$s" --arg corpus "$corpus" --arg auth "$AUTH" \
    --arg h "$harness_hash" --arg a "$auto_hash" '
    .version=7 | .evidence_kind="gpt_codex_task006_structured_outputs_transport_preflight" |
    .frozen_artifacts={policy_sha256:$p,config_sha256:$c,schema_sha256:$s,corpus_sha256:$corpus} |
    .authorization_identity.sha256=$auth |
    .runtime_candidate={auto_version:"0.50.68-ute-transport-diagnosis-v7",auto_executable_sha256:$a,harness_sha256:$h,codex_cli_version:"0.144.1"} |
    .checks=[{name:"conservative_structured_outputs_schema",status:"PASS"},
      {name:"success_and_sanitized_failure_paths",status:"PASS"}] |
    .decision.status="AWAITING_EXPLICIT_LIVE_AUTHORIZATION" |
    .decision.provider_execution_started=false | .decision.authorized_at=null' \
    "$SOURCE/gpt-transport-smoke-preflight-v6.json" > "$preflight"
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

verify_frozen_v1_v6
make_fake_tools
export PATH="$TMP_ROOT:$PATH"
git clone -q --bare "$REPO_ROOT" "$SNAPSHOT"

make_install awaiting awaiting
: > "$AUTO_COUNT"; : > "$PROVIDER_COUNT"
if ! output=$(UTE_GPT_TRANSPORT_PROTOCOL_VERSION=v7 "$HARNESS" preflight --repo "$SNAPSHOT" 2>&1); then
  printf 'RED: v7 preflight unavailable: %s\n' "$output" >&2
  exit 1
fi
state="$TMP_ROOT/state-awaiting"; mkdir "$state"
if FAKE_AUTO_COUNT_FILE="$AUTO_COUNT" FAKE_PROVIDER_COUNT_FILE="$PROVIDER_COUNT" \
  UTE_GPT_TRANSPORT_PROTOCOL_VERSION=v7 UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
  "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$OUTPUT" >/dev/null 2>&1; then
  fail "awaiting v7 authorization was executable"
fi
assert_eq 0 "$(count_file "$AUTO_COUNT")" "awaiting fake-auto invocation"
assert_eq 0 "$(count_file "$PROVIDER_COUNT")" "awaiting provider invocation"
[[ ! -e "$CLAIM" && ! -L "$CLAIM" ]] || fail "awaiting authorization created claim"

assert_schema_rejected() {
  local label=$1 filter=$2 schema tmp state
  make_install "$label" approved
  schema="$OUTPUT/gpt-transport-smoke-schema-v7.json"; tmp="$OUTPUT/.schema.$$"
  jq "$filter" "$schema" > "$tmp"; mv "$tmp" "$schema"; sidecar "$schema"
  : > "$AUTO_COUNT"; : > "$PROVIDER_COUNT"; state="$TMP_ROOT/state-$label"; mkdir "$state"
  if FAKE_AUTO_COUNT_FILE="$AUTO_COUNT" FAKE_PROVIDER_COUNT_FILE="$PROVIDER_COUNT" \
    UTE_GPT_TRANSPORT_PROTOCOL_VERSION=v7 UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$OUTPUT" >/dev/null 2>&1; then
    fail "$label schema was accepted"
  fi
  assert_eq 0 "$(count_file "$AUTO_COUNT")" "$label fake-auto invocation"
  assert_eq 0 "$(count_file "$PROVIDER_COUNT")" "$label provider invocation"
  [[ ! -e "$CLAIM" && ! -L "$CLAIM" ]] || fail "$label schema created claim"
}

assert_schema_rejected missing-items 'del(.properties.finding_codes.items)'
assert_schema_rejected unsupported-composition '.allOf=[]'
assert_schema_rejected missing-required '.required=["verdict","finding_count","finding_codes"]'
assert_schema_rejected additional-properties '.additionalProperties=true'

make_install success approved
: > "$AUTO_COUNT"; : > "$PROVIDER_COUNT"; state="$TMP_ROOT/state-success"; mkdir "$state"
FAKE_AUTO_COUNT_FILE="$AUTO_COUNT" FAKE_PROVIDER_COUNT_FILE="$PROVIDER_COUNT" FAKE_V7_MODE=success \
  UTE_GPT_TRANSPORT_PROTOCOL_VERSION=v7 UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
  "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$OUTPUT" >/dev/null
ledger="$OUTPUT/gpt-transport-smoke-ledger-v7.json"
jq -e '.completed==true and .attempted_calls==1 and .actual_usage_calls==1 and
  .observed_raw_total_tokens==100 and .calls[0].transport_schema_conformance=="PASS" and
  .operational_provider_events==[]' "$ledger" >/dev/null || fail "v7 success ledger mismatch"
! rg -q 'FAKE-V7-RAW|RAW-V7-' "$ledger" || fail "v7 success ledger retained raw data"
assert_eq 2 "$(count_file "$AUTO_COUNT")" "v7 success fake-auto invocations"
assert_eq 1 "$(count_file "$PROVIDER_COUNT")" "v7 success provider attempts"
[[ -f "$CLAIM/reservation.json" ]] || fail "v7 success claim missing"

make_install producer-omitempty approved
: > "$AUTO_COUNT"; : > "$PROVIDER_COUNT"; state="$TMP_ROOT/state-producer-omitempty"; mkdir "$state"
if ! FAKE_AUTO_COUNT_FILE="$AUTO_COUNT" FAKE_PROVIDER_COUNT_FILE="$PROVIDER_COUNT" FAKE_V7_MODE=producer_omitempty \
  UTE_GPT_TRANSPORT_PROTOCOL_VERSION=v7 UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
  "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$OUTPUT" >/dev/null; then
  jq -e '.failure_code=="missing_or_invalid_result"' "$OUTPUT/gpt-transport-smoke-ledger-v7.partial-fail.json" >/dev/null ||
    fail "producer omitempty result failed for an unexpected reason"
  fail "producer omitempty result rejected as missing_or_invalid_result"
fi
ledger="$OUTPUT/gpt-transport-smoke-ledger-v7.json"
jq -e '.completed==true and .calls[0].transport_schema_conformance=="PASS"' "$ledger" >/dev/null ||
  fail "producer omitempty result was not normalized"
assert_eq 1 "$(count_file "$PROVIDER_COUNT")" "producer omitempty provider attempts"

make_install producer-nonempty approved
: > "$AUTO_COUNT"; : > "$PROVIDER_COUNT"; state="$TMP_ROOT/state-producer-nonempty"; mkdir "$state"
if FAKE_AUTO_COUNT_FILE="$AUTO_COUNT" FAKE_PROVIDER_COUNT_FILE="$PROVIDER_COUNT" FAKE_V7_MODE=producer_nonempty \
  UTE_GPT_TRANSPORT_PROTOCOL_VERSION=v7 UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
  "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$OUTPUT" >/dev/null 2>&1; then
  fail "producer nonempty findings were accepted"
fi
jq -e '.failure_code=="missing_or_invalid_result"' "$OUTPUT/gpt-transport-smoke-ledger-v7.partial-fail.json" >/dev/null ||
  fail "producer nonempty result failed for an unexpected reason"
assert_eq 1 "$(count_file "$PROVIDER_COUNT")" "producer nonempty provider attempts"

make_install producer-null approved
: > "$AUTO_COUNT"; : > "$PROVIDER_COUNT"; state="$TMP_ROOT/state-producer-null"; mkdir "$state"
if FAKE_AUTO_COUNT_FILE="$AUTO_COUNT" FAKE_PROVIDER_COUNT_FILE="$PROVIDER_COUNT" FAKE_V7_MODE=producer_null \
  UTE_GPT_TRANSPORT_PROTOCOL_VERSION=v7 UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
  "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$OUTPUT" >/dev/null 2>&1; then
  fail "producer null findings were accepted"
fi
jq -e '.failure_code=="missing_or_invalid_result"' "$OUTPUT/gpt-transport-smoke-ledger-v7.partial-fail.json" >/dev/null ||
  fail "producer null result failed for an unexpected reason"
assert_eq 1 "$(count_file "$PROVIDER_COUNT")" "producer null provider attempts"

make_install failure approved
: > "$AUTO_COUNT"; : > "$PROVIDER_COUNT"; state="$TMP_ROOT/state-failure"; mkdir "$state"
if FAKE_AUTO_COUNT_FILE="$AUTO_COUNT" FAKE_PROVIDER_COUNT_FILE="$PROVIDER_COUNT" FAKE_V7_MODE=failure \
  UTE_GPT_TRANSPORT_PROTOCOL_VERSION=v7 UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
  "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$OUTPUT" >/dev/null 2>&1; then
  fail "v7 sanitized failure unexpectedly succeeded"
fi
ledger="$OUTPUT/gpt-transport-smoke-ledger-v7.partial-fail.json"
jq -e '.completed==false and .attempted_calls==1 and .operational_error_class=="authentication" and
  .operational_error_stage=="process_wait" and .operational_error_signals==["stderr"] and
  .operational_provider_event_kind==null and .operational_provider_event_shape==[] and
  .operational_provider_events==[]' "$ledger" >/dev/null || fail "v7 sanitized failure ledger mismatch"
! rg -q 'RAW-V7-' "$ledger" || fail "v7 failure ledger retained raw data"
assert_eq 2 "$(count_file "$AUTO_COUNT")" "v7 failure fake-auto invocations"
assert_eq 1 "$(count_file "$PROVIDER_COUNT")" "v7 failure provider attempts"

verify_frozen_v1_v6
printf '%s\n' 'ute gpt transport smoke v7 hermetic tests: PASS'
