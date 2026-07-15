#!/usr/bin/env bash
set -euo pipefail
umask 077

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
# shellcheck source=ute_codex_canary_lib.sh
source "$SCRIPT_DIR/ute_codex_canary_lib.sh"

die_smoke() { printf 'ute-gpt-transport-smoke: %s\n' "$1" >&2; exit 1; }
write_sidecar() { printf '%s  %s\n' "$(sha256_file "$1")" "$(basename "$1")" > "${1%.json}.sha256"; }

MODE=${1:-preflight}; [[ $# -eq 0 ]] || shift
case "$MODE" in preflight|run) ;; *) die_smoke "mode must be preflight or run" ;; esac
REPO= AUTO= STATE= OUTPUT=
while [[ $# -gt 0 ]]; do
  [[ $# -ge 2 ]] || die_smoke "missing option value"
  case "$1" in
    --repo) REPO=$2 ;;
    --auto) AUTO=$2 ;;
    --state) STATE=$2 ;;
    --output) OUTPUT=$2 ;;
    *) die_smoke "unknown option" ;;
  esac
  shift 2
done

EVIDENCE_DIR=$(cd "$SCRIPT_DIR/../.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence" && pwd -P)
PROTOCOL_VERSION=${UTE_GPT_TRANSPORT_PROTOCOL_VERSION:-v1}
case "$PROTOCOL_VERSION" in v1|v2|v3|v4|v5|v6|v7|v8) ;; *) die_smoke "unsupported protocol version" ;; esac
CORPUS="$EVIDENCE_DIR/corpus-v1.json"
POLICY="$EVIDENCE_DIR/gpt-transport-smoke-policy-$PROTOCOL_VERSION.json"
CONFIG="$EVIDENCE_DIR/gpt-transport-smoke-config-$PROTOCOL_VERSION.json"
SCHEMA="$EVIDENCE_DIR/gpt-transport-smoke-schema-$PROTOCOL_VERSION.json"
PREFLIGHT="$EVIDENCE_DIR/gpt-transport-smoke-preflight-$PROTOCOL_VERSION.json"
LEDGER_PREFIX="gpt-transport-smoke-ledger-$PROTOCOL_VERSION"

validate_static() {
  local base auth
  require_tools; verify_corpus "$EVIDENCE_DIR" || return 1
  for base in policy config schema preflight; do
    verify_named_sidecar "$EVIDENCE_DIR/gpt-transport-smoke-$base-$PROTOCOL_VERSION.json" \
      "$EVIDENCE_DIR/gpt-transport-smoke-$base-$PROTOCOL_VERSION.sha256" || return 1
  done
  jq -e '.authorization == {provider:"codex",model:"gpt-5.6-sol",provider_call_cap:1,
    raw_token_cap:22000,concurrency:1,retries:0,single_use:true} and
    .task.task_id == "ute-corpus-v1-006" and .call == {role:"reviewer",role_ordinal:1,
    effort:"xhigh",raw_token_budget:22000} and .transport_only == true and
    .semantic_evaluation_performed == false and .promotion_eligible == false' "$POLICY" >/dev/null || return 1
  jq -e '.provider == "codex" and .provider_version == "0.144.1" and .model == "gpt-5.6-sol" and
    .context.evidence_mode == true and .context.diagnostic_mode == true and .context.strict_verdict == true and
    .context.zero_tool_calls_required == true and .context.codex.raw_token_budget == 22000 and
    .context.codex.runtime_output_schema == "gpt-diagnostic-verdict-schema-v1.json" and
    .transport_only == true and .semantic_evaluation_performed == false and .promotion_eligible == false' \
    "$CONFIG" >/dev/null || return 1
  if [[ "$PROTOCOL_VERSION" == v7 || "$PROTOCOL_VERSION" == v8 ]]; then
    jq -e '. == {type:"object",additionalProperties:false,required:["verdict","finding_count","finding_codes","finding_scope_hashes"],properties:{verdict:{type:"string",enum:["PASS"]},finding_count:{type:"integer",enum:[0]},finding_codes:{type:"array",items:{type:"string"}},finding_scope_hashes:{type:"array",items:{type:"string"}}}}' "$SCHEMA" >/dev/null || return 1
  else
    jq -e '.type == "object" and .additionalProperties == false and .required == ["verdict","finding_count","finding_codes","finding_scope_hashes"] and .properties.verdict.const == "PASS" and .properties.finding_count.const == 0 and .properties.finding_codes.maxItems == 0 and .properties.finding_scope_hashes.maxItems == 0' "$SCHEMA" >/dev/null || return 1
  fi
  if [[ "$PROTOCOL_VERSION" == v4 || "$PROTOCOL_VERSION" == v5 || "$PROTOCOL_VERSION" == v6 || "$PROTOCOL_VERSION" == v7 || "$PROTOCOL_VERSION" == v8 ]]; then
    local harness_hash="sha256:$(sha256_file "$SCRIPT_DIR/ute_gpt_transport_smoke.sh")"
    local expected_auto_version="0.50.68-ute-transport-diagnosis-$PROTOCOL_VERSION"
    jq -e --arg harness "$harness_hash" --arg version "$expected_auto_version" '
      .execution_runtime.harness_sha256 == $harness and .execution_runtime.auto_version == $version and
      (.execution_runtime.auto_executable_sha256 | test("^sha256:[0-9a-f]{64}$")) and
      .execution_runtime.codex_cli_version == "0.144.1"' "$POLICY" >/dev/null || return 1
  fi
  if [[ "$PROTOCOL_VERSION" == v6 || "$PROTOCOL_VERSION" == v7 || "$PROTOCOL_VERSION" == v8 ]]; then
    jq -e '.operational_failure_metadata as $m | $m.provider_event_trait_allowlist == ["authentication","authorization_or_entitlement","model_access","rate_limit_or_quota","provider_unavailable","network_transport","request_validation","schema_or_response"] and $m.provider_event_status_family_allowlist == ["http_4xx","http_5xx"] and $m.provider_event_receipt_required_keys == ["kind","shape","traits","status_families"] and $m.provider_event_receipt_additional_keys == false and $m.provider_event_aggregate_must_match_events == true' "$POLICY" >/dev/null || return 1
    jq -e '.operational_failure_receipt.per_event_canonical_projection == true and .operational_failure_receipt.invalid_event_metadata_clears_all == true' "$CONFIG" >/dev/null || return 1
  fi; POLICY_HASH="sha256:$(sha256_file "$POLICY")"; CONFIG_HASH="sha256:$(sha256_file "$CONFIG")"
  SCHEMA_HASH="sha256:$(sha256_file "$SCHEMA")"; CORPUS_HASH="sha256:$(sha256_file "$CORPUS")"
  auth=$(printf '%s\0%s\0%s\0%s' "$POLICY_HASH" "$CONFIG_HASH" "$SCHEMA_HASH" "$CORPUS_HASH" |
    shasum -a 256 | awk '{print "sha256:" $1}')
  AUTH_ID=$auth
  jq -e --arg p "$POLICY_HASH" --arg c "$CONFIG_HASH" --arg s "$SCHEMA_HASH" --arg h "$CORPUS_HASH" \
    --arg a "$AUTH_ID" '.provider_receipt == false and .observed_spend == {provider_calls_made:0,raw_total_tokens:0} and
    .frozen_artifacts == {policy_sha256:$p,config_sha256:$c,schema_sha256:$s,corpus_sha256:$h} and
    .authorization_identity.sha256 == $a and .protocol.provider_call_cap == 1 and
    .protocol.raw_token_cap == 22000 and .decision.transport_only == true and
    .decision.semantic_evaluation_performed == false and .decision.promotion_eligible == false' \
    "$PREFLIGHT" >/dev/null
}

[[ -n "$REPO" ]] || die_smoke "immutable repository snapshot is required"
REPO=$(cd "$REPO" 2>/dev/null && pwd -P) || die_smoke "repository snapshot unavailable"
validate_static || die_smoke "static smoke evidence validation failed"
validate_repo_snapshot "$REPO" "$CORPUS" || die_smoke "repository snapshot validation failed"
base=$(jq -r '.task.base_commit' "$POLICY"); target=$(jq -r '.task.target_commit' "$POLICY")
expected=$(jq -r '.task.expected_patch_sha256' "$POLICY")
observed="sha256:$(snapshot_git "$REPO" diff --no-ext-diff --no-textconv --binary "$base" "$target" |
  shasum -a 256 | awk '{print $1}')"
[[ "$observed" == "$expected" ]] || die_smoke "task006 patch hash mismatch"

if [[ "$MODE" == preflight ]]; then
  printf 'preflight=PASS authorization=%s provider_calls=0 raw=0\n' "$AUTH_ID"
  exit 0
fi

if [[ "$PROTOCOL_VERSION" == v4 || "$PROTOCOL_VERSION" == v5 || "$PROTOCOL_VERSION" == v6 || "$PROTOCOL_VERSION" == v7 || "$PROTOCOL_VERSION" == v8 ]]; then
  jq -e '.decision.status == "EXPLICIT_LIVE_AUTHORIZATION_GRANTED" and
    (.decision.authorized_at | type == "string" and length > 0) and
    .decision.provider_execution_started == false' "$PREFLIGHT" >/dev/null ||
    die_smoke "explicit live authorization not recorded"
fi

[[ ${UTE_GPT_TRANSPORT_SMOKE_EXECUTE:-} == YES ]] || die_smoke "explicit smoke execution opt-in required"
[[ -n "$AUTO" && "$AUTO" == /* && -x "$AUTO" ]] || die_smoke "built auto executable is required"
[[ -n "$STATE" && "$STATE" == /* && -d "$STATE" && ! -L "$STATE" ]] || die_smoke "isolated state required"
STATE=$(cd "$STATE" && pwd -P); [[ -z "$(find "$STATE" -mindepth 1 -print -quit)" ]] || die_smoke "state must be empty"
[[ -n "$OUTPUT" && "$OUTPUT" == /* && -d "$OUTPUT" && ! -L "$OUTPUT" ]] || die_smoke "canonical output required"
OUTPUT=$(cd "$OUTPUT" && pwd -P); [[ "$OUTPUT" == "$EVIDENCE_DIR" ]] || die_smoke "noncanonical output"

for name in "$LEDGER_PREFIX.json" "$LEDGER_PREFIX.sha256" \
  "$LEDGER_PREFIX.partial-fail.json" "$LEDGER_PREFIX.partial-fail.sha256"; do
  [[ ! -e "$OUTPUT/$name" && ! -L "$OUTPUT/$name" ]] || die_smoke "smoke authorization already consumed"
done
version_file="$STATE/.smoke-auto-version"; codex_file="$STATE/.smoke-codex-version"
"$AUTO" version --short > "$version_file" 2>/dev/null || die_smoke "auto identity unavailable"
AUTO_VERSION=$(tr -d '\r\n' < "$version_file")
[[ "$AUTO_VERSION" =~ ^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$ ]] || die_smoke "invalid auto identity"
AUTO_SHA="sha256:$(sha256_file "$AUTO")"
if [[ "$PROTOCOL_VERSION" == v4 || "$PROTOCOL_VERSION" == v5 || "$PROTOCOL_VERSION" == v6 || "$PROTOCOL_VERSION" == v7 || "$PROTOCOL_VERSION" == v8 ]]; then
  jq -e --arg auto "$AUTO_SHA" --arg version "$AUTO_VERSION" '
    .execution_runtime.auto_executable_sha256 == $auto and .execution_runtime.auto_version == $version' \
    "$POLICY" >/dev/null || die_smoke "auto runtime identity mismatch"
fi
codex --version > "$codex_file" 2>/dev/null || die_smoke "codex identity unavailable"
[[ "$(tr -d '\r\n' < "$codex_file")" == "codex-cli 0.144.1" ]] || die_smoke "codex identity mismatch"
rm -f "$version_file" "$codex_file"
INSTALL_ROOT=$(cd "$SCRIPT_DIR/.." && pwd -P)
AUTH_ROOT="$INSTALL_ROOT/.autopus/runtime/ute-transport-smoke-authorizations"
prepare_auth_dir() {
  local runtime="$INSTALL_ROOT/.autopus/runtime"
  [[ -d "$INSTALL_ROOT/.autopus" && ! -L "$INSTALL_ROOT/.autopus" && -O "$INSTALL_ROOT/.autopus" ]] || return 1
  if [[ -e "$runtime" ]]; then
    [[ -d "$runtime" && ! -L "$runtime" && -O "$runtime" ]] || return 1
  else
    mkdir "$runtime" || return 1; chmod 700 "$runtime"
  fi
  if [[ -e "$AUTH_ROOT" ]]; then
    [[ -d "$AUTH_ROOT" && ! -L "$AUTH_ROOT" && -O "$AUTH_ROOT" ]] || return 1
  else
    mkdir "$AUTH_ROOT" || return 1; chmod 700 "$AUTH_ROOT"
  fi
}
prepare_auth_dir || die_smoke "unsafe authorization root"
TMP="$STATE/.ute-gpt-transport-smoke"; RUN_DIR="$STATE/.autopus/runs/ute-corpus-v1-006"
mkdir -p "$TMP" "$RUN_DIR"; chmod 700 "$TMP" "$RUN_DIR"
cleanup() { rm -rf "$TMP"; rm -rf "$STATE/.autopus/runs" "$STATE/.autopus/telemetry"; }
trap cleanup EXIT INT TERM
RUN_ID="r$(printf '%s\0run' "$AUTH_ID" | shasum -a 256 | awk '{print substr($1,1,24)}')"
CALL_ID="c$(printf '%s\0call' "$AUTH_ID" | shasum -a 256 | awk '{print substr($1,1,24)}')"
PROMPT="$TMP/prompt"; STDOUT="$TMP/stdout"; STDERR="$TMP/stderr"
printf '%s\n' 'Return only the schema-conforming PASS acknowledgement. Do not call tools or inspect external context.' > "$PROMPT"
jq -n --rawfile description "$PROMPT" --arg run "$RUN_ID" --arg call "$CALL_ID" --arg p "$POLICY_HASH" \
  --arg c "$CONFIG_HASH" '{task_id:"ute-corpus-v1-006",description:$description,provider:"codex",model:"gpt-5.6-sol",
  effort:"xhigh",spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",run_id:$run,call_id:$call,attempt:1,phase:"review",
  role:"reviewer",provider_version:"0.144.1",model_version:"gpt-5.6-sol",risk_policy:$p,
  cache_stratum:"provider-managed-stable-prefix-v1",config_hash:$c,evidence_mode:true,diagnostic_mode:true,
  strict_verdict:true,zero_tool_calls_required:true,codex:{sandbox:"read-only",ephemeral:true,ignore_user_config:true,
  ignore_rules:true,skip_git_repo_check:true,output_schema:"gpt-diagnostic-verdict-schema-v1.json",zero_tool_mode:true,
  raw_token_budget:22000}}' > "$RUN_DIR/context.yaml"
cp "$SCHEMA" "$RUN_DIR/gpt-diagnostic-verdict-schema-v1.json"; [[ "sha256:$(sha256_file "$RUN_DIR/gpt-diagnostic-verdict-schema-v1.json")" == "$SCHEMA_HASH" ]] || die_smoke "runtime schema copy mismatch"
: > "$STDOUT"; : > "$STDERR"; chmod 600 "$PROMPT" "$STDOUT" "$STDERR" "$RUN_DIR"/*
CLAIM="$AUTH_ROOT/${AUTH_ID#sha256:}"; mkdir "$CLAIM" 2>/dev/null || die_smoke "smoke authorization already reserved"; chmod 700 "$CLAIM"
jq -n --arg a "$AUTH_ID" --arg p "$POLICY_HASH" --arg c "$CONFIG_HASH" --arg s "$SCHEMA_HASH" \
  --arg auto "$AUTO_SHA" --arg auto_version "$AUTO_VERSION" \
  '{version:1,kind:"transport_smoke_authorization_reservation",authorization_identity:$a,
    policy_sha256:$p,config_sha256:$c,schema_sha256:$s,provider:"codex",model:"gpt-5.6-sol",
    auto_executable_sha256:$auto,auto_version:$auto_version,codex_cli_version:"0.144.1",
    calls_reserved:1,raw_tokens_reserved:22000,concurrency:1,retries:0,single_use:true,
    state:"CONSUMED_ON_RESERVATION"}' > "$CLAIM/reservation.json"
write_sidecar "$CLAIM/reservation.json"

set +e
(cd "$STATE" && "$AUTO" agent run ute-corpus-v1-006) > "$STDOUT" 2> "$STDERR"
exit_code=$?
set -e

persist_failure() {
  local code=$1 target="$OUTPUT/$LEDGER_PREFIX.partial-fail.json" tmp="$OUTPUT/.smoke-partial.$$"
  local class= fingerprint= expected_fingerprint= stage= signals='[]' event_kind= event_shape='[]' event_list='[]'
  local candidate="$TMP/failure-result.json"
  if [[ -f "$RUN_DIR/result.yaml" ]] && yq -o=json '.' "$RUN_DIR/result.yaml" > "$candidate" 2>/dev/null &&
    jq -e 'type == "object" and .task_id == "ute-corpus-v1-006" and .status == "failed"' "$candidate" >/dev/null; then
    class=$(jq -r '.operational_error_class // empty' "$candidate")
    fingerprint=$(jq -r '.operational_error_fingerprint // empty' "$candidate")
    stage=$(jq -r '.operational_error_stage // empty' "$candidate")
    signals=$(jq -c '.operational_error_signals // []' "$candidate")
    event_kind=$(jq -r '.operational_provider_event_kind // empty' "$candidate")
    event_shape=$(jq -c '.operational_provider_event_shape // []' "$candidate")
    case "$class" in
      binary_missing|cli_usage_or_config|authentication|model_access|network_transport|provider_rejected|schema_or_response|unknown) ;;
      *) class=; fingerprint=; stage=; signals='[]' ;;
    esac
    case "$stage" in
      subprocess_start|stream_scan|process_wait|missing_result|evidence_postcondition) ;;
      *) class=; fingerprint=; stage=; signals='[]' ;;
    esac
    case "$signals" in
      '[]'|'["stderr"]'|'["provider_failure_event"]'|'["stream_parse_failure"]'|\
      '["stderr","provider_failure_event"]'|'["stderr","stream_parse_failure"]'|\
      '["provider_failure_event","stream_parse_failure"]'|\
      '["stderr","provider_failure_event","stream_parse_failure"]') ;;
      *) class=; fingerprint=; stage=; signals='[]' ;;
    esac
    case "$event_kind" in ''|error|turn_failed|error_and_turn_failed) ;; *) event_kind=; event_shape='[]' ;; esac
    jq -e '(.operational_provider_event_shape // []) as $s |
      $s == [["top_level_message","top_level_code","top_level_status","top_level_status_code",
        "top_level_error_string","nested_error_object","nested_error_message","nested_error_type",
        "nested_error_code"][] as $x | select($s | index($x)) | $x]' "$candidate" >/dev/null ||
      { event_kind=; event_shape='[]'; }
    if [[ "$signals" == *'"provider_failure_event"'* ]]; then
      [[ -n "$event_kind" ]] ||
        { class=; fingerprint=; stage=; signals='[]'; event_kind=; event_shape='[]'; }
    else
      event_kind=; event_shape='[]'
    fi
    expected_fingerprint="sha256:$(printf 'autopus-operational-error-v1\0%s' "$class" | shasum -a 256 | awk '{print $1}')"
    [[ "$fingerprint" == "$expected_fingerprint" ]] ||
      { class=; fingerprint=; stage=; signals='[]'; event_kind=; event_shape='[]'; }
    if [[ "$PROTOCOL_VERSION" == v6 || "$PROTOCOL_VERSION" == v7 || "$PROTOCOL_VERSION" == v8 ]]; then
      event_list=$(jq -c '(.operational_provider_events // [])' "$candidate")
      [[ -n "$class" ]] && jq -e '
        def canonical($v;$a): (($v|type) == "array") and $v == [$a[] as $x | select($v|index($x)) | $x]; def event_class: .traits as $t | [$t[] | if . == "authentication" or . == "authorization_or_entitlement" then "authentication" elif . == "model_access" then "model_access" elif . == "rate_limit_or_quota" or . == "provider_unavailable" or . == "request_validation" then "provider_rejected" elif . == "network_transport" then "network_transport" elif . == "schema_or_response" then "schema_or_response" else "unknown" end] | if (($t|index("model_access")) != null and ($t|index("authorization_or_entitlement")) != null and ($t|index("authentication")) == null) then map(select(. != "authentication")) else . end | unique | if length == 0 then "" elif length == 1 then .[0] else "unknown" end;
        ["top_level_message","top_level_code","top_level_status","top_level_status_code","top_level_error_string","nested_error_object","nested_error_message","nested_error_type","nested_error_code"] as $shapes |
        ["authentication","authorization_or_entitlement","model_access","rate_limit_or_quota","provider_unavailable","network_transport","request_validation","schema_or_response"] as $traits | ["http_4xx","http_5xx"] as $statuses | (.operational_provider_events // []) as $events |
        (($events|type) == "array") and (if (.operational_error_signals|index("provider_failure_event")) != null then
          ($events|length) > 0 and ($events|length) <= 2 and ([$events[].kind] as $k | $k == ["error"] or $k == ["turn_failed"] or $k == ["error","turn_failed"]) and
          all($events[]; (keys == ["kind","shape","status_families","traits"]) and (.kind == "error" or .kind == "turn_failed") and canonical(.shape;$shapes) and canonical(.traits;$traits) and canonical(.status_families;$statuses)) and
          ([$events[] | event_class] | map(select(. != "")) | unique | if length == 1 then .[0] else "unknown" end) as $provider_class | (if .operational_error_signals == ["provider_failure_event"] then .operational_error_class == $provider_class else .operational_error_class == $provider_class or ($provider_class != "unknown" and .operational_error_class == "unknown") end) and .operational_provider_event_kind == (if ($events|length) == 2 then "error_and_turn_failed" else $events[0].kind end) and .operational_provider_event_shape == [$shapes[] as $x | select(any($events[]; .shape|index($x))) | $x]
        else $events == [] and (.operational_provider_event_kind // null) == null and (.operational_provider_event_shape // []) == [] end)' "$candidate" >/dev/null ||
        { class=; fingerprint=; stage=; signals='[]'; event_kind=; event_shape='[]'; event_list='[]'; }
    fi
  fi
  jq -n --arg a "$AUTH_ID" --arg code "$code" --arg class "$class" --arg fingerprint "$fingerprint" \
    --arg stage "$stage" --argjson signals "$signals" --arg event_kind "$event_kind" --argjson event_shape "$event_shape" --argjson event_list "$event_list" \
    '{version:1,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",
    evidence_kind:"gpt_transport_smoke_ledger",completed:false,transport_only:true,
    semantic_evaluation_performed:false,promotion_eligible:false,failure_code:$code,planned_calls:1,
    attempted_calls:1,raw_token_cap:22000,observed_raw_total_tokens:null,actual_usage_calls:0,
    usage_status:"unavailable",operational_error_class:(if $class == "" then null else $class end),
    operational_error_fingerprint:(if $fingerprint == "" then null else $fingerprint end),
    operational_error_stage:(if $stage == "" then null else $stage end),
    operational_error_signals:$signals,
    operational_provider_event_kind:(if $event_kind == "" then null else $event_kind end),
    operational_provider_event_shape:$event_shape,
    operational_provider_events:$event_list,
    authorization_identity:{sha256:$a},
    calls:[{sequence:1,task_id:"ute-corpus-v1-006",role:"reviewer",effort:"xhigh",
      status:"operational_failure",failure_code:$code}]}' > "$tmp"
  chmod 600 "$tmp"; mv "$tmp" "$target"; write_sidecar "$target"; return 1
}

[[ "$exit_code" == 0 ]] || persist_failure process_nonzero
[[ -f "$RUN_DIR/result.yaml" ]] || persist_failure missing_or_invalid_result
shopt -s nullglob; files=("$STATE"/.autopus/telemetry/*.jsonl); shopt -u nullglob
[[ ${#files[@]} -eq 1 && "$(awk 'END {print NR+0}' "${files[0]}")" == 1 ]] || persist_failure missing_or_invalid_usage
yq -o=json '.' "$RUN_DIR/result.yaml" > "$TMP/result.json" || persist_failure missing_or_invalid_result
jq -e --arg run "$RUN_ID" --arg call "$CALL_ID" '.task_id == "ute-corpus-v1-006" and .status == "success" and
  .provider == "codex" and .model == "gpt-5.6-sol" and .effort == "xhigh" and .run_id == $run and .call_id == $call and
  .attempt == 1 and .phase == "review" and .role == "reviewer" and .verdict == "PASS" and .finding_count == 0 and
  ((has("finding_codes") | not) or .finding_codes == []) and
  ((has("finding_scope_hashes") | not) or .finding_scope_hashes == []) and .usage_status == "actual" and
  .unique_model_call_count == 1 and .tool_calls == 0 and (.raw_total_tokens > 0 and .raw_total_tokens <= 22000)' \
  "$TMP/result.json" >/dev/null || persist_failure missing_or_invalid_result
jq -e --arg run "$RUN_ID" --arg call "$CALL_ID" --arg p "$POLICY_HASH" --arg c "$CONFIG_HASH" '
  .type == "agent_run" and .data.task_id == "ute-corpus-v1-006" and .data.run_id == $run and
  .data.call_id == $call and .data.attempt == 1 and .data.provider == "codex" and
  .data.model == "gpt-5.6-sol" and .data.effort == "xhigh" and .data.phase == "review" and
  .data.role == "reviewer" and .data.status == "PASS" and .data.acceptance_status == "PASS" and
  (.data.tool_calls // 0) == 0 and (.data.usage|length) == 1 and .data.usage[0] as $u |
  $u.run_id == $run and $u.call_id == $call and $u.task_id == "ute-corpus-v1-006" and $u.attempt == 1 and
  $u.provider == "codex" and $u.model == "gpt-5.6-sol" and $u.effort == "xhigh" and
  $u.provider_version == "0.144.1" and $u.model_version == "gpt-5.6-sol" and
  $u.risk_policy == $p and $u.config_hash == $c and $u.cache_stratum == "provider-managed-stable-prefix-v1" and
  $u.phase == "review" and $u.role == "reviewer" and $u.usage_status == "actual" and
  $u.usage_source == "provider" and $u.source_schema == "codex.exec-json.turn.completed.v1" and
  ($u.raw_total_tokens > 0 and $u.raw_total_tokens <= 22000)' "${files[0]}" >/dev/null || persist_failure missing_or_invalid_usage
[[ "$(jq -r '.raw_total_tokens' "$TMP/result.json")" == "$(jq -r '.data.usage[0].raw_total_tokens' "${files[0]}")" ]] ||
  persist_failure missing_or_invalid_usage
raw=$(jq -r '.raw_total_tokens' "$TMP/result.json")
target="$OUTPUT/$LEDGER_PREFIX.json"; tmp="$OUTPUT/.smoke-complete.$$"
jq -n --arg a "$AUTH_ID" --arg auto "$AUTO_SHA" --arg auto_version "$AUTO_VERSION" --argjson raw "$raw" \
  '{version:1,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",
  evidence_kind:"gpt_transport_smoke_ledger",completed:true,transport_only:true,
  semantic_evaluation_performed:false,promotion_eligible:false,failure_code:null,planned_calls:1,attempted_calls:1,
  raw_token_cap:22000,observed_raw_total_tokens:$raw,actual_usage_calls:1,usage_status:"actual",
  operational_provider_events:[],
  authorization_identity:{sha256:$a},runtime_identity:{auto_executable_sha256:$auto,
    auto_version:$auto_version,codex_cli_version:"0.144.1"},
  calls:[{sequence:1,task_id:"ute-corpus-v1-006",role:"reviewer",effort:"xhigh",status:"success",
    transport_schema_conformance:"PASS",usage_status:"actual",unique_model_call_count:1,
    raw_total_tokens:$raw,tool_calls:0}]}' > "$tmp"
chmod 600 "$tmp"; mv "$tmp" "$target"; write_sidecar "$target"
cleanup; trap - EXIT INT TERM
printf 'smoke=PASS authorization=%s provider_calls=1 raw=%s\n' "$AUTH_ID" "$raw"
