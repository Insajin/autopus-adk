#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd -P)
TMP_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/ute-gpt-transport-smoke-test.XXXXXX")
trap 'rm -rf "$TMP_ROOT"' EXIT
COUNT_FILE="$TMP_ROOT/invocations"; FAKE_AUTO="$TMP_ROOT/auto"; SNAPSHOT="$TMP_ROOT/snapshot.git"
HARNESS= OUTPUT= INSTALL=

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
assert_eq() { [[ "$1" == "$2" ]] || fail "$3: want=$1 got=$2"; }
count_calls() { [[ -s "$COUNT_FILE" ]] && wc -l < "$COUNT_FILE" | tr -d ' ' || printf '0\n'; }

make_fake_tools() {
  sed 's/^+//' > "$TMP_ROOT/codex" <<'EOF'
+#!/usr/bin/env bash
+[[ ${1:-} == --version ]] && { printf '%s\n' "${FAKE_SMOKE_CODEX_VERSION:-codex-cli 0.144.1}"; exit 0; }
+exit 90
EOF
  sed 's/^+//' > "$FAKE_AUTO" <<'EOF'
+#!/usr/bin/env bash
+set -euo pipefail
+if [[ ${1:-} == version && ${2:-} == --short ]]; then printf '%s\n' 0.50.99-smoke-test; exit 0; fi
+[[ ${1:-} == agent && ${2:-} == run && ${3:-} == ute-corpus-v1-006 ]] || exit 91
+printf '1\n' >> "${FAKE_SMOKE_COUNT_FILE:?}"; n=$(wc -l < "$FAKE_SMOKE_COUNT_FILE" | tr -d ' ')
+[[ -z ${FAKE_SMOKE_DELAY:-} ]] || sleep "$FAKE_SMOKE_DELAY"
+if [[ -n ${FAKE_SMOKE_FAIL_AT:-} && $n == "$FAKE_SMOKE_FAIL_AT" ]]; then
+  if [[ -n ${FAKE_SMOKE_ERROR_CLASS:-} ]]; then
+    run_dir="$PWD/.autopus/runs/ute-corpus-v1-006"
+    stage=${FAKE_SMOKE_ERROR_STAGE:-process_wait}
+    signals=${FAKE_SMOKE_ERROR_SIGNALS:-'["stderr"]'}
+    event_kind=${FAKE_SMOKE_EVENT_KIND:-}; event_shape=${FAKE_SMOKE_EVENT_SHAPE:-'[]'}
+    task=${FAKE_SMOKE_ERROR_TASK:-ute-corpus-v1-006}
+    result_status=${FAKE_SMOKE_ERROR_STATUS:-failed}
+    jq -n --arg task "$task" --arg status "$result_status" --arg class "$FAKE_SMOKE_ERROR_CLASS" \
+      --arg stage "$stage" --argjson signals "$signals" --arg event_kind "$event_kind" --argjson event_shape "$event_shape" \
+      '{task_id:$task,status:$status,operational_error_class:$class,
+       operational_error_fingerprint:"sha256:7ef173ce9315debba3596a48c416d4681eadfd7f6eca40b046e8b1a0202816ec",
+       operational_error_stage:$stage,operational_error_signals:$signals,
+       operational_provider_event_kind:$event_kind,operational_provider_event_shape:$event_shape}' \
+      > "$run_dir/result.yaml"
+  fi
+  printf '%s\n' 'SUPER-SECRET-TRANSPORT-ERROR' >&2
+  exit 77
+fi
+run_dir="$PWD/.autopus/runs/ute-corpus-v1-006"; ctx="$run_dir/context.yaml"
+jq -e '.task_id == "ute-corpus-v1-006" and .provider == "codex" and .model == "gpt-5.6-sol" and
+  .effort == "xhigh" and .role == "reviewer" and .evidence_mode == true and .diagnostic_mode == true and
+  .strict_verdict == true and .zero_tool_calls_required == true and .codex.raw_token_budget == 22000 and
+  .codex.output_schema == "gpt-diagnostic-verdict-schema-v1.json"' "$ctx" >/dev/null
+get() { jq -r "$1" "$ctx"; }
+run=$(get .run_id); call=$(get .call_id); policy=$(get .risk_policy); config=$(get .config_hash)
+output_hash=$(printf '%s' FAKE-SMOKE-RAW | shasum -a 256 | awk '{print $1}')
+jq -n --arg run "$run" --arg call "$call" --arg policy "$policy" --arg config "$config" --arg output "$output_hash" \
+  '{task_id:"ute-corpus-v1-006",status:"success",provider:"codex",model:"gpt-5.6-sol",effort:"xhigh",
+   run_id:$run,call_id:$call,attempt:1,phase:"review",role:"reviewer",verdict:"PASS",finding_count:0,
+   finding_codes:[],finding_scope_hashes:[],output_sha256:$output,usage_status:"actual",
+   unique_model_call_count:1,raw_total_tokens:100,tool_calls:0}' > "$run_dir/result.yaml"
+mkdir -p "$PWD/.autopus/telemetry"
+usage_raw=${FAKE_SMOKE_USAGE_RAW:-100}
+jq -cn --arg run "$run" --arg call "$call" --arg policy "$policy" --arg config "$config" --argjson raw "$usage_raw" \
+  '{type:"agent_run",data:{agent_name:"reviewer",spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",
+   task_id:"ute-corpus-v1-006",run_id:$run,call_id:$call,attempt:1,provider:"codex",model:"gpt-5.6-sol",
+   effort:"xhigh",phase:"review",role:"reviewer",status:"PASS",acceptance_status:"PASS",tool_calls:0,
+   usage:[{version:1,run_id:$run,call_id:$call,task_id:"ute-corpus-v1-006",attempt:1,provider:"codex",
+   model:"gpt-5.6-sol",effort:"xhigh",provider_version:"0.144.1",model_version:"gpt-5.6-sol",
+   risk_policy:$policy,cache_stratum:"provider-managed-stable-prefix-v1",config_hash:$config,phase:"review",
+   role:"reviewer",usage_status:"actual",usage_source:"provider",source_schema:"codex.exec-json.turn.completed.v1",
+   input_tokens_total:90,output_tokens_total:10,raw_total_tokens:$raw}]}}' > "$PWD/.autopus/telemetry/smoke.jsonl"
+[[ -z ${FAKE_SMOKE_EXTRA_EVENT:-} ]] || printf '%s\n' '{"type":"extra"}' >> "$PWD/.autopus/telemetry/smoke.jsonl"
EOF
  chmod 755 "$TMP_ROOT/codex" "$FAKE_AUTO"
}

make_install() {
  local label=$1 protocol=${2:-v1} source="$REPO_ROOT/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
  INSTALL="$TMP_ROOT/install-$label"; OUTPUT="$INSTALL/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
  mkdir -p "$INSTALL/scripts" "$OUTPUT"
  [[ -x "$SCRIPT_DIR/ute_gpt_transport_smoke.sh" ]] || fail "production transport smoke harness missing"
  cp "$SCRIPT_DIR/ute_gpt_transport_smoke.sh" "$INSTALL/scripts/"
  cp "$SCRIPT_DIR/ute_codex_canary_lib.sh" "$SCRIPT_DIR/ute_codex_canary_schedule.sh" "$INSTALL/scripts/"
  cp "$source/corpus-v1.json" "$source/corpus-v1.sha256" "$OUTPUT/"
  local base
  for base in policy config schema preflight; do
    cp "$source/gpt-transport-smoke-$base-$protocol.json" "$source/gpt-transport-smoke-$base-$protocol.sha256" "$OUTPUT/"
  done
  HARNESS="$INSTALL/scripts/ute_gpt_transport_smoke.sh"
}

assert_sidecar() {
  local file=$1 expected name actual
  read -r expected name < "${file%.json}.sha256"; actual=$(shasum -a 256 "$file" | awk '{print $1}')
  [[ "$expected" == "$actual" && "$name" == "$(basename "$file")" ]] || fail "invalid sidecar"
}

run_preflight_and_tamper() {
  make_install preflight; : > "$COUNT_FILE"
  FAKE_SMOKE_COUNT_FILE="$COUNT_FILE" "$HARNESS" preflight --repo "$SNAPSHOT" >/dev/null
  assert_eq 0 "$(count_calls)" "preflight called auto"
  make_install tamper; printf '\n' >> "$OUTPUT/gpt-transport-smoke-config-v1.json"; : > "$COUNT_FILE"
  if FAKE_SMOKE_COUNT_FILE="$COUNT_FILE" "$HARNESS" preflight --repo "$SNAPSHOT" >/dev/null 2>&1; then
    fail "tampered static artifact accepted"
  fi
  assert_eq 0 "$(count_calls)" "tamper called auto"
}

run_complete_and_reuse() {
  local state="$TMP_ROOT/state-complete" ledger auth claim before next="$TMP_ROOT/state-reuse"
  make_install complete; mkdir "$state" "$next"; : > "$COUNT_FILE"
  FAKE_SMOKE_COUNT_FILE="$COUNT_FILE" UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$OUTPUT" >/dev/null
  assert_eq 1 "$(count_calls)" "complete call count"; ledger="$OUTPUT/gpt-transport-smoke-ledger-v1.json"
  jq -e '.completed == true and .transport_only == true and .semantic_evaluation_performed == false and
    .promotion_eligible == false and .planned_calls == 1 and .attempted_calls == 1 and
    .raw_token_cap == 22000 and .calls[0].transport_schema_conformance == "PASS" and
    .calls[0].usage_status == "actual" and .calls[0].tool_calls == 0' "$ledger" >/dev/null || fail "complete ledger"
  assert_sidecar "$ledger"
  if rg -uuu -l 'FAKE-SMOKE-RAW|"(prompt|description|raw_output|stdout|stderr|session_id|environment|cwd|path)"' "$ledger" >/dev/null; then
    fail "raw transport data retained"
  fi
  auth=$(jq -r '.authorization_identity.sha256' "$OUTPUT/gpt-transport-smoke-preflight-v1.json"); claim=${auth#sha256:}
  [[ -f "$INSTALL/.autopus/runtime/ute-transport-smoke-authorizations/$claim/reservation.json" ]] || fail "claim missing"
  [[ ! -e "$INSTALL/.autopus/runtime/ute-diagnostic-authorizations/$claim" ]] || fail "diagnostic namespace reused"
  before=$(count_calls)
  if FAKE_SMOKE_COUNT_FILE="$COUNT_FILE" UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$next" --output "$OUTPUT" >/dev/null 2>&1; then
    fail "authorization reused"
  fi
  assert_eq "$before" "$(count_calls)" "reuse called auto"
}

run_partial_and_noncanonical() {
  local state="$TMP_ROOT/state-partial" next="$TMP_ROOT/state-partial-next" before arbitrary="$TMP_ROOT/arbitrary"
  make_install partial; mkdir "$state" "$next"; : > "$COUNT_FILE"
  if FAKE_SMOKE_COUNT_FILE="$COUNT_FILE" FAKE_SMOKE_FAIL_AT=1 FAKE_SMOKE_ERROR_CLASS=authentication \
    UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$OUTPUT" >/dev/null 2>&1; then
    fail "process failure accepted"
  fi
  assert_eq 1 "$(count_calls)" "partial retries"
  jq -e '.completed == false and .attempted_calls == 1 and .failure_code == "process_nonzero"' \
    "$OUTPUT/gpt-transport-smoke-ledger-v1.partial-fail.json" >/dev/null || fail "partial ledger"
  jq -e '.operational_error_class == "authentication" and
    (.operational_error_fingerprint | test("^sha256:[0-9a-f]{64}$")) and
    .operational_error_stage == "process_wait" and .operational_error_signals == ["stderr"] and
    .operational_provider_event_kind == null and .operational_provider_event_shape == []' \
    "$OUTPUT/gpt-transport-smoke-ledger-v1.partial-fail.json" >/dev/null || fail "sanitized error class missing"
  rg -q 'SUPER-SECRET-TRANSPORT-ERROR' "$OUTPUT/gpt-transport-smoke-ledger-v1.partial-fail.json" &&
    fail "raw transport error retained"
  before=$(count_calls)
  if FAKE_SMOKE_COUNT_FILE="$COUNT_FILE" UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$next" --output "$OUTPUT" >/dev/null 2>&1; then
    fail "partial authorization reused"
  fi
  assert_eq "$before" "$(count_calls)" "partial reuse called auto"
  make_install noncanonical; mkdir "$arbitrary"; state="$TMP_ROOT/state-noncanonical"; mkdir "$state"; : > "$COUNT_FILE"
  if FAKE_SMOKE_COUNT_FILE="$COUNT_FILE" UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$arbitrary" >/dev/null 2>&1; then
    fail "noncanonical output accepted"
  fi
  assert_eq 0 "$(count_calls)" "noncanonical called auto"
}

run_concurrent() {
  local s1="$TMP_ROOT/state-concurrent-1" s2="$TMP_ROOT/state-concurrent-2" pid status i before
  make_install concurrent; mkdir "$s1" "$s2"; : > "$COUNT_FILE"
  FAKE_SMOKE_COUNT_FILE="$COUNT_FILE" FAKE_SMOKE_DELAY=2 FAKE_SMOKE_FAIL_AT=1 UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$s1" --output "$OUTPUT" >/dev/null 2>&1 & pid=$!
  for i in $(seq 1 200); do [[ "$(count_calls)" == 1 ]] && break; sleep 0.05; done
  before=$(count_calls); assert_eq 1 "$before" "concurrent first call"
  if FAKE_SMOKE_COUNT_FILE="$COUNT_FILE" UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$s2" --output "$OUTPUT" >/dev/null 2>&1; then
    fail "concurrent claim accepted"
  fi
  assert_eq "$before" "$(count_calls)" "concurrent extra call"
  set +e; wait "$pid"; status=$?; set -e; [[ "$status" != 0 ]] || fail "concurrent fixture passed"
}

run_negative_contracts() {
  local state outside
  make_install bad-version; state="$TMP_ROOT/state-bad-version"; mkdir "$state"; : > "$COUNT_FILE"
  if FAKE_SMOKE_COUNT_FILE="$COUNT_FILE" FAKE_SMOKE_CODEX_VERSION='codex-cli 9.9.9' \
    UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" \
    --state "$state" --output "$OUTPUT" >/dev/null 2>&1; then fail "codex version mismatch accepted"; fi
  assert_eq 0 "$(count_calls)" "version mismatch called auto"

  make_install extra-event; state="$TMP_ROOT/state-extra-event"; mkdir "$state"; : > "$COUNT_FILE"
  if FAKE_SMOKE_COUNT_FILE="$COUNT_FILE" FAKE_SMOKE_EXTRA_EVENT=1 UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$OUTPUT" >/dev/null 2>&1; then
    fail "multiple telemetry events accepted"
  fi
  jq -e '.usage_status == "unavailable" and .observed_raw_total_tokens == null' \
    "$OUTPUT/gpt-transport-smoke-ledger-v1.partial-fail.json" >/dev/null || fail "unknown usage claimed"

  make_install bad-root; state="$TMP_ROOT/state-bad-root"; outside="$TMP_ROOT/outside-auth"
  mkdir "$state" "$outside" "$INSTALL/.autopus/runtime"; ln -s "$outside" \
    "$INSTALL/.autopus/runtime/ute-transport-smoke-authorizations"; : > "$COUNT_FILE"
  if FAKE_SMOKE_COUNT_FILE="$COUNT_FILE" UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$OUTPUT" >/dev/null 2>&1; then
    fail "authorization root symlink accepted"
  fi
  assert_eq 0 "$(count_calls)" "unsafe auth root called auto"
}

run_invalid_operational_metadata() {
  local state ledger
  make_install invalid-operational; state="$TMP_ROOT/state-invalid-operational"; mkdir "$state"; : > "$COUNT_FILE"
  if FAKE_SMOKE_COUNT_FILE="$COUNT_FILE" FAKE_SMOKE_FAIL_AT=1 FAKE_SMOKE_ERROR_CLASS=authentication \
    FAKE_SMOKE_ERROR_STAGE=forged_stage FAKE_SMOKE_ERROR_SIGNALS='["stderr","raw_secret"]' \
    UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$OUTPUT" >/dev/null 2>&1; then
    fail "invalid operational metadata accepted as success"
  fi
  ledger="$OUTPUT/gpt-transport-smoke-ledger-v1.partial-fail.json"
  jq -e '.operational_error_class == null and .operational_error_fingerprint == null and
    .operational_error_stage == null and .operational_error_signals == [] and
    .operational_provider_event_kind == null and .operational_provider_event_shape == []' "$ledger" >/dev/null ||
    fail "invalid operational metadata was retained"

  make_install mismatched-operational; state="$TMP_ROOT/state-mismatched-operational"; mkdir "$state"; : > "$COUNT_FILE"
  if FAKE_SMOKE_COUNT_FILE="$COUNT_FILE" FAKE_SMOKE_FAIL_AT=1 FAKE_SMOKE_ERROR_CLASS=authentication \
    FAKE_SMOKE_ERROR_TASK=other-task FAKE_SMOKE_ERROR_STATUS=success UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$OUTPUT" >/dev/null 2>&1; then
    fail "mismatched operational result accepted as success"
  fi
  ledger="$OUTPUT/gpt-transport-smoke-ledger-v1.partial-fail.json"
  jq -e '.operational_error_class == null and .operational_error_fingerprint == null and
    .operational_error_stage == null and .operational_error_signals == [] and
    .operational_provider_event_kind == null and .operational_provider_event_shape == []' "$ledger" >/dev/null ||
    fail "mismatched operational result was retained"

  make_install provider-metadata; state="$TMP_ROOT/state-provider-metadata"; mkdir "$state"; : > "$COUNT_FILE"
  FAKE_SMOKE_COUNT_FILE="$COUNT_FILE" FAKE_SMOKE_FAIL_AT=1 FAKE_SMOKE_ERROR_CLASS=authentication \
    FAKE_SMOKE_ERROR_SIGNALS='["provider_failure_event"]' FAKE_SMOKE_EVENT_KIND=turn_failed \
    FAKE_SMOKE_EVENT_SHAPE='["nested_error_object","nested_error_code"]' UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$OUTPUT" >/dev/null 2>&1 || true
  ledger="$OUTPUT/gpt-transport-smoke-ledger-v1.partial-fail.json"
  jq -e '.operational_provider_event_kind == "turn_failed" and
    .operational_provider_event_shape == ["nested_error_object","nested_error_code"]' "$ledger" >/dev/null || fail "provider metadata missing"
}

run_v4_requires_recorded_authorization() {
  local state="$TMP_ROOT/state-v4-awaiting" auth claim preflight policy tmp h p c s corpus
  make_install v4-awaiting v4; mkdir "$state"; : > "$COUNT_FILE"
  preflight="$OUTPUT/gpt-transport-smoke-preflight-v4.json"
  policy="$OUTPUT/gpt-transport-smoke-policy-v4.json"
  h="sha256:$(shasum -a 256 "$HARNESS" | awk '{print $1}')"
  tmp="$OUTPUT/.policy-current-harness.$$"
  jq --arg h "$h" '.execution_runtime.harness_sha256 = $h' "$policy" > "$tmp"; mv "$tmp" "$policy"
  p="sha256:$(shasum -a 256 "$policy" | awk '{print $1}')"
  c="sha256:$(shasum -a 256 "$OUTPUT/gpt-transport-smoke-config-v4.json" | awk '{print $1}')"
  s="sha256:$(shasum -a 256 "$OUTPUT/gpt-transport-smoke-schema-v4.json" | awk '{print $1}')"
  corpus="sha256:$(shasum -a 256 "$OUTPUT/corpus-v1.json" | awk '{print $1}')"
  auth=$(printf '%s\0%s\0%s\0%s' "$p" "$c" "$s" "$corpus" | shasum -a 256 | awk '{print "sha256:" $1}')
  printf '%s  %s\n' "${p#sha256:}" "$(basename "$policy")" > "$OUTPUT/gpt-transport-smoke-policy-v4.sha256"
  tmp="$OUTPUT/.preflight-awaiting.$$"
  jq --arg p "$p" --arg auth "$auth" '.frozen_artifacts.policy_sha256 = $p |
    .authorization_identity.sha256 = $auth | .decision.status = "AWAITING_EXPLICIT_LIVE_AUTHORIZATION" |
    .decision.authorized_at = null' "$preflight" > "$tmp"; mv "$tmp" "$preflight"
  printf '%s  %s\n' "$(shasum -a 256 "$preflight" | awk '{print $1}')" "$(basename "$preflight")" \
    > "$OUTPUT/gpt-transport-smoke-preflight-v4.sha256"
  auth=$(jq -r '.authorization_identity.sha256' "$preflight")
  claim="$INSTALL/.autopus/runtime/ute-transport-smoke-authorizations/${auth#sha256:}"
  UTE_GPT_TRANSPORT_PROTOCOL_VERSION=v4 FAKE_SMOKE_COUNT_FILE="$COUNT_FILE" \
    "$HARNESS" preflight --repo "$SNAPSHOT" >/dev/null
  if UTE_GPT_TRANSPORT_PROTOCOL_VERSION=v4 FAKE_SMOKE_COUNT_FILE="$COUNT_FILE" \
    UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$OUTPUT" >/dev/null 2>&1; then
    fail "v4 awaiting authorization was executable"
  fi
  assert_eq 0 "$(count_calls)" "v4 awaiting authorization called auto"
  [[ ! -e "$claim" && ! -L "$claim" ]] || fail "v4 awaiting authorization created claim"

  tmp="$OUTPUT/.preflight-approved.$$"
  jq '.decision.status = "EXPLICIT_LIVE_AUTHORIZATION_GRANTED" |
    .decision.authorized_at = "2026-07-14T07:30:00+09:00"' "$preflight" > "$tmp"
  mv "$tmp" "$preflight"
  printf '%s  %s\n' "$(shasum -a 256 "$preflight" | awk '{print $1}')" "$(basename "$preflight")" \
    > "$OUTPUT/gpt-transport-smoke-preflight-v4.sha256"
  state="$TMP_ROOT/state-v4-runtime-mismatch"; mkdir "$state"
  if UTE_GPT_TRANSPORT_PROTOCOL_VERSION=v4 FAKE_SMOKE_COUNT_FILE="$COUNT_FILE" \
    UTE_GPT_TRANSPORT_SMOKE_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$OUTPUT" >/dev/null 2>&1; then
    fail "v4 mismatched auto identity was executable"
  fi
  assert_eq 0 "$(count_calls)" "v4 mismatched auto identity called auto"
  [[ ! -e "$claim" && ! -L "$claim" ]] || fail "v4 mismatched auto identity created claim"
}

make_fake_tools; export PATH="$TMP_ROOT:$PATH"; git clone -q --bare "$REPO_ROOT" "$SNAPSHOT"
run_preflight_and_tamper
run_complete_and_reuse
run_partial_and_noncanonical
run_concurrent
run_negative_contracts
run_invalid_operational_metadata
run_v4_requires_recorded_authorization
printf '%s\n' 'ute gpt transport smoke hermetic tests: PASS'
