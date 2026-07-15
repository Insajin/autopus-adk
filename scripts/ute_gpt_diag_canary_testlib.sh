#!/usr/bin/env bash

diag_fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
diag_assert_eq() { [[ "$1" == "$2" ]] || diag_fail "$3: want=$1 got=$2"; }
diag_count() { [[ -s "$1" ]] && wc -l < "$1" | tr -d ' ' || printf '0\n'; }

make_diag_install() {
  local label=$1 root="$TMP_ROOT/install-$1" evidence source base
  evidence="$root/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
  source="$REPO_ROOT/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
  mkdir -p "$root/scripts" "$evidence"
  cp "$SCRIPT_DIR"/ute_gpt_diag_canary*.sh "$SCRIPT_DIR/ute_codex_canary_lib.sh" \
    "$SCRIPT_DIR/ute_codex_canary_schedule.sh" "$root/scripts/"
  cp "$source/corpus-v1.json" "$source/corpus-v1.sha256" "$evidence/"
  for base in cohort policy config verdict-schema preflight; do
    cp "$source/gpt-diagnostic-$base-v1.json" "$source/gpt-diagnostic-$base-v1.sha256" "$evidence/"
  done
  HARNESS="$root/scripts/ute_gpt_diag_canary.sh"; CANONICAL_OUTPUT=$evidence; INSTALL_ROOT=$root
}

diag_assert_progress() {
  local file=$1 total=$2
  diag_assert_eq "$total" "$(wc -l < "$file" | tr -d ' ')" "progress count"
  awk -v total="$total" '$0 !~ ("^call=[0-9]+/" total " task=ute-corpus-v1-006 role=[a-z0-9-]+ verdict=(PASS|FAIL) raw=[0-9]+ cumulative=[0-9]+$") {exit 1}' "$file" || diag_fail "progress leaked"
}

diag_assert_ledger() {
  local ledger=$1
  jq -e '.completed == true and .diagnostic_only == true and .promotion_eligible == false and
    .planned_calls == 10 and .observed_calls == 10 and .planned_worst_case_raw_tokens == 228000 and
    ([.calls[].effort] | map(select(. == "xhigh")) | length) == 8 and
    ([.calls[].effort] | map(select(. == "max")) | length) == 2 and
    ([.calls[].arm] | unique | sort) == ["A","B"] and
    ([.calls[] | select(.result.verdict == "FAIL")] | length) == 2 and
    (all(.calls[]; .result.status == "success" and .result.finding_count >= 0 and
      (.result.finding_codes | length) <= 3 and (.result.finding_scope_hashes | length) <= 3))' "$ledger" >/dev/null || diag_fail "ledger contract"
  diag_verify_sidecar "$ledger"
}

diag_verify_sidecar() {
  local file=$1 expected name actual
  read -r expected name < "${file%.json}.sha256"
  actual=$(shasum -a 256 "$file" | awk '{print $1}')
  [[ "$expected" == "$actual" && "$name" == "$(basename "$file")" ]] || diag_fail "sidecar mismatch"
}

diag_assert_no_raw() {
  local state=$1 output=$2
  if rg -uuu -l 'UTE-DIAG-RAW-|FAKE-DIAG-RAW|"(prompt|description|raw_output|stdout|stderr|session_id|environment|cwd|path)"[[:space:]]*:' "$state" "$output" >/dev/null 2>&1; then
    diag_fail "raw diagnostic data retained"
  fi
  [[ ! -d "$state/.autopus/runs" || -z "$(find "$state/.autopus/runs" -mindepth 1 -print -quit)" ]] || diag_fail "run data retained"
}

make_diag_fake_codex() {
  sed 's/^+//' > "$1" <<'EOF'
+#!/usr/bin/env bash
+[[ ${1:-} == --version ]] && { printf '%s\n' 'codex-cli 0.144.1'; exit 0; }
+exit 91
EOF
  chmod 755 "$1"
}

make_diag_fake_auto() {
  sed 's/^+//' > "$1" <<'EOF'
+#!/usr/bin/env bash
+set -euo pipefail
+if [[ ${1:-} == version && ${2:-} == --short ]]; then printf '%s\n' 0.50.99-test; exit 0; fi
+[[ ${1:-} == agent && ${2:-} == run && ${3:-} == ute-corpus-v1-006 ]] || exit 90
+count_file=${FAKE_DIAG_COUNT_FILE:?}; printf '1\n' >> "$count_file"; n=$(wc -l < "$count_file" | tr -d ' ')
+[[ -z ${FAKE_DIAG_DELAY:-} ]] || sleep "$FAKE_DIAG_DELAY"
+task=$3; run_dir="$PWD/.autopus/runs/$task"; ctx="$run_dir/context.yaml"
+get() { jq -r "$1" "$ctx"; }
+call=$(get .call_id); run=$(get .run_id); effort=$(get .effort); role=$(get .role); config=$(get .config_hash)
+risk=$(get .risk_policy); cache=$(get .cache_stratum); raw=100; [[ "$effort" == max ]] && raw=200
+verdict=PASS; count=0; codes='[]'; scopes='[]'
+scope1=sha256:86caac50db9c74a2979ff531218a0d280b5ac4bd081fe64015a65241976151e7
+scope2=sha256:ddcf6887f5e7ac209f865859d16a5af5dcad53ccdd41f54957990090797cc77d
+if [[ $n == 1 ]]; then verdict=FAIL; count=1; codes='["correctness"]'; scopes="[\"$scope1\"]"; fi
+if [[ $n == 9 ]]; then verdict=FAIL; count=1; codes='["security"]'; scopes="[\"$scope2\"]"; fi
+if [[ -n ${FAKE_DIAG_BAD_SCOPE_AT:-} && $n == $FAKE_DIAG_BAD_SCOPE_AT ]]; then verdict=FAIL; count=1; codes='["scope_uncertain"]'; scopes='["sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"]'; fi
+status=success; event_status=PASS
+if [[ -n ${FAKE_DIAG_PROCESS_FAIL_AT:-} && $n == $FAKE_DIAG_PROCESS_FAIL_AT ]]; then status=failed; event_status=FAIL; fi
+output_hash=$(printf '%s' FAKE-DIAG-RAW | shasum -a 256 | awk '{print $1}')
+jq -n --arg task "$task" --arg status "$status" --arg effort "$effort" --arg role "$role" --arg run "$run" --arg call "$call" \
+  --arg verdict "$verdict" --arg output "$output_hash" --argjson count "$count" --argjson codes "$codes" --argjson scopes "$scopes" --argjson raw "$raw" \
+  '{task_id:$task,status:$status,provider:"codex",model:"gpt-5.6-sol",effort:$effort,run_id:$run,call_id:$call,attempt:1,
+  phase:"review",role:$role,verdict:$verdict,finding_count:$count,finding_codes:$codes,finding_scope_hashes:$scopes,
+  output_sha256:$output,usage_status:"actual",unique_model_call_count:1,raw_total_tokens:$raw,tool_calls:0}' > "$run_dir/result.yaml"
+mkdir -p "$PWD/.autopus/telemetry"
+jq -cn --arg task "$task" --arg run "$run" --arg call "$call" --arg effort "$effort" --arg role "$role" --arg config "$config" \
+  --arg risk "$risk" --arg cache "$cache" --arg status "$event_status" --arg verdict "$verdict" --argjson raw "$raw" \
+  '{type:"agent_run",data:{agent_name:$role,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",task_id:$task,run_id:$run,call_id:$call,
+  attempt:1,provider:"codex",model:"gpt-5.6-sol",effort:$effort,phase:"review",role:$role,status:$status,
+  acceptance_status:$verdict,files_modified:0,estimated_tokens:0,usage:[{version:1,run_id:$run,call_id:$call,task_id:$task,
+  attempt:1,provider:"codex",model:"gpt-5.6-sol",effort:$effort,provider_version:"0.144.1",model_version:"gpt-5.6-sol",
+  risk_policy:$risk,cache_stratum:$cache,config_hash:$config,phase:"review",role:$role,usage_status:"actual",usage_source:"provider",
+  source_schema:"codex.exec-json.turn.completed.v1",input_tokens_total:($raw-10),output_tokens_total:10,raw_total_tokens:$raw}]}}' \
+  > "$PWD/.autopus/telemetry/diag.jsonl"
+[[ "$status" == success ]]
EOF
  chmod 755 "$1"
}
