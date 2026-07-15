#!/usr/bin/env bash

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

make_harness_install() {
  local label=$1 root="$TMP_ROOT/install-$1" evidence source base
  evidence="$root/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
  source="$REPO_ROOT/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
  mkdir -p "$root/scripts" "$evidence"
  cp "$SCRIPT_DIR"/ute_codex_canary*.sh "$root/scripts/"
  for base in corpus-v1 gpt-canary-cohort-v1 gpt-canary-preflight-v1 \
    gpt-codex-config-v1 gpt-codex-policy-v1 gpt-verdict-schema-v1; do
    cp "$source/$base.json" "$source/$base.sha256" "$evidence/"
  done
  HARNESS="$root/scripts/ute_codex_canary.sh"
  CANONICAL_OUTPUT=$evidence
  HARNESS_INSTALL_ROOT=$root
}

assert_eq() {
  [[ "$1" == "$2" ]] || fail "$3: want=$1 got=$2"
}

invocation_count() {
  if [[ ! -s "$1" ]]; then printf '0\n'; else wc -l < "$1" | tr -d ' '; fi
}

write_named_sidecar() {
  local file=$1
  printf '%s  %s\n' "$(shasum -a 256 "$file" | awk '{print $1}')" "$(basename "$file")" > "${file%.json}.sha256"
}

write_applied_receipt() {
  local receipt=$1 primary=$2 evidence=$3 policy config primary_hash
  policy="sha256:$(shasum -a 256 "$evidence/gpt-codex-policy-v1.json" | awk '{print $1}')"
  config="sha256:$(shasum -a 256 "$evidence/gpt-codex-config-v1.json" | awk '{print $1}')"
  primary_hash="sha256:$(shasum -a 256 "$primary" | awk '{print $1}')"
  jq -n --arg policy "$policy" --arg config "$config" --arg primary "$primary_hash" '{version:1,
    spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",evidence_kind:"applied_policy_rollback",decision:"ROLLBACK",
    active_profile:"full_ultra",applied:true,atomic_replace:true,state_readback:"full_ultra",
    policy_sha256:$policy,config_sha256:$config,primary_ledger_sha256:$primary,
    logical_rollback_result_sha256:("sha256:" + ("a" * 64)),
    before_binding_sha256:("sha256:" + ("b" * 64)),after_binding_sha256:("sha256:" + ("c" * 64))}' > "$receipt"
}

verify_progress() {
  local file=$1 count=$2
  assert_eq "$count" "$(wc -l < "$file" | tr -d ' ')" "progress line count"
  awk -v total="$count" '
    BEGIN { ok=1 }
    $0 !~ ("^call=[0-9]+/" total " task=ute-corpus-v1-[0-9][0-9][0-9] role=[a-z0-9-]+ raw=[0-9]+ cumulative=[0-9]+$") { ok=0 }
    END { exit ok ? 0 : 1 }
  ' "$file" || fail "progress output contains non-sanitized lines"
}

assert_primary_ledger() {
  local ledger=$1
  jq -e '
    .completed == true and .evaluation_eligible == true and .promotion_eligible == false and
    .planned_calls == 58 and .observed_calls == 58 and
    .planned_worst_case_raw_tokens == 1332000 and
    ([.calls[].effort] | map(select(. == "xhigh")) | length) == 44 and
    ([.calls[].effort] | map(select(. == "max")) | length) == 14 and
    ([.calls[].arm] | unique | sort) == ["A","B"] and
    (all(.calls[]; .result.status == "success" and .result.verdict == "PASS" and
      .result.finding_count == 0 and .result.tool_calls == 0 and
      .result.usage_status == "actual" and .result.unique_model_call_count == 1))
  ' "$ledger" >/dev/null || fail "primary ledger contract"
  verify_named_sidecar_test "$ledger"
  local schedule
  schedule=$(jq -r '.calls[] | [.task_id,.arm,.profile,.role,.role_ordinal,.effort] | @tsv' "$ledger")
  diff -u <(expected_primary_schedule) <(printf '%s\n' "$schedule") >/dev/null || fail "primary schedule mismatch"
}

assert_rollback_ledger() {
  local ledger=$1
  jq -e '
    .completed == true and .promotion_eligible == false and .evidence_kind == "applied_rollback_replay" and
    .planned_calls == 5 and .observed_calls == 5 and .planned_worst_case_raw_tokens == 114000 and
    ([.calls[].effort] | map(select(. == "xhigh")) | length) == 4 and
    ([.calls[].effort] | map(select(. == "max")) | length) == 1 and
    (all(.calls[]; .task_id == "ute-corpus-v1-001" and .profile == "full5"))
  ' "$ledger" >/dev/null || fail "rollback ledger contract"
  verify_named_sidecar_test "$ledger"
}

verify_named_sidecar_test() {
  local file=$1 sidecar="${1%.json}.sha256" expected actual name
  read -r expected name < "$sidecar"
  actual=$(shasum -a 256 "$file" | awk '{print $1}')
  [[ "$expected" == "$actual" && "$name" == "$(basename "$file")" ]] || fail "ledger sidecar"
}

assert_no_raw_retention() {
  local state=$1 output=$2
  if rg -uuu -l 'UTE-RAW-PROMPT-|FAKE-RAW-PROVIDER-BODY|description:|exact target patch' "$state" "$output" >/dev/null 2>&1; then
    fail "raw prompt/provider data retained"
  fi
  [[ ! -d "$state/.autopus/runs" || -z "$(find "$state/.autopus/runs" -mindepth 1 -print -quit)" ]] || fail "run artifacts retained"
  [[ ! -d "$state/.autopus/telemetry" || -z "$(find "$state/.autopus/telemetry" -type f -print -quit)" ]] || fail "telemetry artifact retained"
}

expected_primary_schedule() {
  local task order profile role effort
  while IFS=$'\t' read -r task order; do
    for arm in "${order:0:1}" "${order:1:1}"; do
      profile=full5
      if [[ "$arm" == B && "$task" =~ -(001|004|011|012)$ ]]; then profile=compact2; fi
      if [[ "$profile" == full5 ]]; then
        for ordinal in 1 2 3; do printf '%s\t%s\t%s\treviewer\t%s\txhigh\n' "$task" "$arm" "$profile" "$ordinal"; done
        printf '%s\t%s\t%s\tsecurity-auditor\t1\tmax\n' "$task" "$arm" "$profile"
        printf '%s\t%s\t%s\treview-consolidator\t1\txhigh\n' "$task" "$arm" "$profile"
      else
        printf '%s\t%s\t%s\treviewer\t1\txhigh\n' "$task" "$arm" "$profile"
        printf '%s\t%s\t%s\tsecurity-auditor\t1\tmax\n' "$task" "$arm" "$profile"
      fi
    done
  done <<'EOF'
ute-corpus-v1-001	AB
ute-corpus-v1-004	BA
ute-corpus-v1-005	AB
ute-corpus-v1-011	BA
ute-corpus-v1-012	AB
ute-corpus-v1-006	BA
ute-corpus-v1-009	AB
EOF
}

make_fake_auto() {
  local path=$1
  sed 's/^+//' > "$path" <<'FAKE'
+#!/usr/bin/env bash
+set -euo pipefail
+if [[ ${1:-} == version && ${2:-} == --short ]]; then printf '%s\n' '0.50.99-test'; exit 0; fi
+[[ ${1:-} == agent && ${2:-} == run && -n ${3:-} ]] || exit 90
+count_file=${FAKE_AUTO_COUNT_FILE:?}
+printf '1\n' >> "$count_file"
+count=$(wc -l < "$count_file" | tr -d ' ')
+if [[ -n ${FAKE_AUTO_DELAY_SECONDS:-} ]]; then sleep "$FAKE_AUTO_DELAY_SECONDS"; fi
+task=$3
+run_dir="$PWD/.autopus/runs/$task"
+ctx="$run_dir/context.yaml"
+get() { jq -r "$1" "$ctx"; }
+call_id=$(get '.call_id'); run_id=$(get '.run_id'); effort=$(get '.effort'); role=$(get '.role')
+provider=$(get '.provider'); model=$(get '.model'); spec=$(get '.spec_id')
+provider_version=$(get '.provider_version'); model_version=$(get '.model_version')
+risk_policy=$(get '.risk_policy'); cache=$(get '.cache_stratum'); config=$(get '.config_hash')
+raw=100
+[[ "$effort" == max ]] && raw=200
+status=success; event_status=PASS; verdict=PASS; findings=0
+if [[ -n ${FAKE_AUTO_FAIL_AT:-} && $count -eq $FAKE_AUTO_FAIL_AT ]]; then
+  status=failed; event_status=FAIL; verdict=FAIL; findings=1
+fi
+output_hash=$(printf '%s' 'FAKE-RAW-PROVIDER-BODY' | shasum -a 256 | awk '{print $1}')
+cat > "$run_dir/result.yaml" <<EOF
+task_id: $task
+status: $status
+timestamp: "2026-07-12T00:00:00Z"
+provider: $provider
+model: $model
+effort: $effort
+run_id: $run_id
+call_id: $call_id
+attempt: 1
+phase: review
+role: $role
+verdict: $verdict
+finding_count: $findings
+output_sha256: $output_hash
+usage_status: actual
+unique_model_call_count: 1
+raw_total_tokens: $raw
+tool_calls: 0
+EOF
+chmod 600 "$run_dir/result.yaml"
+mkdir -p "$PWD/.autopus/telemetry"
+source_schema=codex.exec-json.turn.completed.v1
+if [[ -n ${FAKE_AUTO_LEAK_AT:-} && $count -eq $FAKE_AUTO_LEAK_AT ]]; then source_schema=FAKE-RAW-PROVIDER-BODY; fi
+jq -cn --arg spec "$spec" --arg task "$task" --arg run "$run_id" --arg call "$call_id" \
+  --arg provider "$provider" --arg model "$model" --arg effort "$effort" --arg role "$role" \
+  --arg pv "$provider_version" --arg mv "$model_version" --arg risk "$risk_policy" \
+  --arg cache "$cache" --arg config "$config" --arg status "$event_status" --arg verdict "$verdict" --arg source_schema "$source_schema" \
+  --argjson raw "$raw" '{type:"agent_run",timestamp:"2026-07-12T00:00:00Z",data:{agent_name:$role,
+  spec_id:$spec,task_id:$task,run_id:$run,call_id:$call,attempt:1,provider:$provider,model:$model,
+  effort:$effort,phase:"review",role:$role,start_time:"2026-07-12T00:00:00Z",end_time:"2026-07-12T00:00:01Z",
+  duration_ns:1,status:$status,acceptance_status:$verdict,files_modified:0,estimated_tokens:0,
+  usage:[{version:1,run_id:$run,call_id:$call,task_id:$task,attempt:1,provider:$provider,model:$model,
+  effort:$effort,provider_version:$pv,model_version:$mv,risk_policy:$risk,cache_stratum:$cache,
+  config_hash:$config,phase:"review",role:$role,usage_status:"actual",usage_source:"provider",
+  source_schema:$source_schema,input_tokens_total:($raw-10),uncached_input_tokens:null,
+  cached_input_tokens:null,cache_creation_input_tokens:null,cache_read_input_tokens:null,
+  output_tokens_total:10,reasoning_tokens:null,tool_tokens:null,raw_total_tokens:$raw,
+  actual_cost_usd:null,estimated_total_tokens:null,estimated_cost_usd:null}]}}' \
+  > "$PWD/.autopus/telemetry/2026-07-12-$spec.jsonl"
+[[ "$status" == success ]]
FAKE
  chmod 755 "$path"
}

make_fake_codex() {
  local path=$1
  sed 's/^+//' > "$path" <<'FAKE'
+#!/usr/bin/env bash
+set -euo pipefail
+if [[ ${1:-} == --version && $# -eq 1 ]]; then
+  if [[ ${FAKE_CODEX_BAD_VERSION:-} == YES ]]; then printf '%s\n' 'codex-cli 0.143.0'; else printf '%s\n' 'codex-cli 0.144.1'; fi
+  exit 0
+fi
+exit 91
FAKE
  chmod 755 "$path"
}
