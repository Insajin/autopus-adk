#!/usr/bin/env bash

ff_die() { printf 'ute-gpt-full-evaluation-finalize: %s\n' "$1" >&2; exit 1; }
ff_sha() { printf 'sha256:%s\n' "$(shasum -a 256 "$1" | awk '{print $1}')"; }

ff_verify_sidecar() {
  local file=$1 sidecar="${1%.json}.sha256" expected name
  [[ -f "$file" && ! -L "$file" && -f "$sidecar" && ! -L "$sidecar" ]] || return 1
  read -r expected name < "$sidecar"
  [[ "$name" == "$(basename "$file")" && "$expected" == "$(shasum -a 256 "$file" | awk '{print $1}')" ]]
}

ff_scan() {
  jq -e '([.. | objects | keys[] | select(. == "prompt" or . == "description" or . == "patch" or
    . == "raw_output" or . == "stdout" or . == "stderr" or . == "session_id" or
    . == "environment" or . == "cwd")] | length) == 0 and
    all(.. | strings; (contains("FAKE-RAW-PROVIDER-BODY") or contains("UTE-RAW-PROMPT-")) | not)' "$1" >/dev/null
}

ff_verify_corpus() {
  local file="$EVIDENCE_DIR/corpus-v1.json" expected
  [[ -f "$file" && -f "$EVIDENCE_DIR/corpus-v1.sha256" ]] || return 1
  expected=$(tr -d '[:space:]' < "$EVIDENCE_DIR/corpus-v1.sha256")
  [[ "$expected" == "$(ff_sha "$file")" ]]
}

ff_prepare() {
  local tool
  for tool in jq shasum find mktemp realpath tr awk sort ln python3; do command -v "$tool" >/dev/null 2>&1 || ff_die "required tool unavailable"; done
  [[ "$EVIDENCE_DIR" == /* && -d "$EVIDENCE_DIR" && ! -L "$EVIDENCE_DIR" ]] || ff_die "invalid evidence directory"
  EVIDENCE_DIR=$(cd "$EVIDENCE_DIR" && pwd -P)
  [[ "$PRIMARY_LEDGER" == /* && -f "$PRIMARY_LEDGER" && ! -L "$PRIMARY_LEDGER" ]] || ff_die "invalid primary ledger"
  PRIMARY_LEDGER=$(realpath "$PRIMARY_LEDGER")
  [[ "$(dirname "$PRIMARY_LEDGER")" == "$EVIDENCE_DIR" ]] || ff_die "primary ledger must be canonical"
  if [[ -n "$ROLLBACK_LEDGER" ]]; then
    [[ "$ROLLBACK_LEDGER" == /* && -f "$ROLLBACK_LEDGER" && ! -L "$ROLLBACK_LEDGER" ]] || ff_die "invalid rollback ledger"
    ROLLBACK_LEDGER=$(realpath "$ROLLBACK_LEDGER")
    [[ "$(dirname "$ROLLBACK_LEDGER")" == "$EVIDENCE_DIR" ]] || ff_die "rollback ledger must be canonical"
  fi
  [[ "$AUTO" == /* && -f "$AUTO" && -x "$AUTO" && ! -L "$AUTO" ]] || ff_die "invalid auto executable"
  AUTO=$(realpath "$AUTO")
  TERMINAL="$EVIDENCE_DIR/gpt-full-evaluation-terminal-outcome-v2.json"
  CLOSURE="$EVIDENCE_DIR/gpt-full-evaluation-authorization-closure-v2.json"
  [[ ! -e "$TERMINAL" && ! -e "${TERMINAL%.json}.sha256" && ! -e "$CLOSURE" && ! -e "${CLOSURE%.json}.sha256" ]] ||
    ff_die "terminal already exists"
  POLICY="$EVIDENCE_DIR/gpt-codex-policy-v2.json"; CONFIG="$EVIDENCE_DIR/gpt-codex-config-v2.json"
  COHORT="$EVIDENCE_DIR/gpt-canary-cohort-v1.json"
  PREFLIGHT="$EVIDENCE_DIR/gpt-canary-preflight-v2.json"; IDENTITY="$EVIDENCE_DIR/gpt-full-evaluation-identity-v2.json"
  SCHEDULE="$EVIDENCE_DIR/gpt-canary-schedule-v2.json"
  AUTHORIZATION="$EVIDENCE_DIR/gpt-full-evaluation-authorization-v2.json"
  RESERVATION="$EVIDENCE_DIR/gpt-full-evaluation-reservation-v2.json"
  ROLLBACK_RESERVATION="$EVIDENCE_DIR/gpt-rollback-reservation-v2.json"
  AUTO_HASH=$(ff_sha "$AUTO"); AUTO_VERSION=null
  PRIMARY_HASH=$(ff_sha "$PRIMARY_LEDGER")
  AUTHORIZATION_IDENTITY_SHA256=null; FF_RESERVATION_HASH=null; FF_ROLLBACK_RESERVATION_HASH=null
  FF_PRIMARY_CALLS=0; FF_PRIMARY_RAW=0; FF_REPLAY_CALLS=0; FF_REPLAY_RAW=0
}

ff_validate_static_authorization() {
  local base identity policy config preflight chain
  ff_verify_corpus || return 1
  for base in gpt-canary-cohort-v1 gpt-codex-policy-v2 gpt-codex-config-v2 gpt-verdict-schema-v2 \
    gpt-canary-schedule-v2 gpt-snapshot-manifest-v2 gpt-prior-evidence-manifest-v2 \
    gpt-full-evaluation-identity-v2 gpt-canary-preflight-v2 gpt-full-evaluation-authorization-v2; do
    ff_verify_sidecar "$EVIDENCE_DIR/$base.json" || return 1
  done
  identity=$(ff_sha "$IDENTITY"); policy=$(ff_sha "$POLICY"); config=$(ff_sha "$CONFIG"); preflight=$(ff_sha "$PREFLIGHT")
  jq -e --arg identity "$identity" --arg policy "$policy" --arg config "$config" \
    --arg auto "$AUTO_HASH" '
    .version == 2 and .admission_generation == "v2" and
    .frozen_artifacts.policy_sha256 == $policy and .frozen_artifacts.config_sha256 == $config and
    .runtime.auto_executable_sha256 == $auto and
    (.runtime.full_chain_harness_sha256 | test("^sha256:[0-9a-f]{64}$")) and
    .runtime.full_chain_algorithm == "sha256-named-member-manifest-v1" and .runtime.full_chain_member_count == 17 and
    .authorization_envelope.provider_call_cap == 64 and .authorization_envelope.raw_token_cap == 1500000 and
    .authorization_envelope.primary_calls == 58 and .authorization_envelope.rollback_calls == 5 and
    .authorization_envelope.total_calls == 63 and .authorization_envelope.planned_worst_case_raw_tokens == 1446000 and
    .authorization_envelope.concurrency == 1 and .authorization_envelope.retries == 0
  ' "$IDENTITY" >/dev/null || return 1
  chain=$(jq -er '.runtime.full_chain_harness_sha256' "$IDENTITY") || return 1
  ff_validate_full_chain "$chain" || return 1
  AUTO_VERSION=$("$AUTO" version --short 2>/dev/null | tr -d '\r\n') || return 1
  [[ "$AUTO_VERSION" =~ ^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$ ]] || return 1
  jq -e --arg version "$AUTO_VERSION" '.runtime.auto_version == $version' "$IDENTITY" >/dev/null || return 1
  jq -e --arg identity "$identity" --arg policy "$policy" --arg config "$config" --arg preflight "$preflight" '
    .version == 2 and .evidence_kind == "gpt_codex_full_evaluation_authorization" and
    .decision == "EXPLICIT_FULL_EVALUATION_AUTHORIZATION_GRANTED" and
    .authorization_identity_sha256 == $identity and .policy_sha256 == $policy and
    .config_sha256 == $config and .preflight_sha256 == $preflight and .provider == "codex" and
    .model == "gpt-5.6-sol" and .provider_call_cap == 64 and .raw_token_cap == 1500000 and
    .primary_calls == 58 and .rollback_calls == 5 and .total_calls == 63 and
    .planned_worst_case_raw_tokens == 1446000 and .concurrency == 1 and .retries == 0 and
    .single_use == true and .activation == false and .promotion == false and .implemented == false
  ' "$AUTHORIZATION" >/dev/null && ff_scan "$AUTHORIZATION" || return 1
  AUTHORIZATION_IDENTITY_SHA256=$identity; POLICY_HASH=$policy; CONFIG_HASH=$config
}

ff_validate_reservation() {
  local reservation_hash
  [[ -f "$RESERVATION" ]] || { FF_CODE=reservation_missing; return 1; }
  ff_verify_sidecar "$RESERVATION" || { FF_CODE=reservation_invalid; return 1; }
  reservation_hash=$(ff_sha "$RESERVATION")
  jq -e --arg identity "$AUTHORIZATION_IDENTITY_SHA256" --arg policy "$(ff_sha "$POLICY")" \
    --arg config "$(ff_sha "$CONFIG")" --arg auto "$AUTO_HASH" --arg version "$AUTO_VERSION" '
    .version == 1 and .spec_id == "SPEC-ADK-ULTRA-EFFICIENCY-001" and
    .evidence_kind == "gpt_full_evaluation_reservation" and .admission_generation == "v2" and
    .authorization_identity_sha256 == $identity and .policy_sha256 == $policy and .config_sha256 == $config and
    .auto_executable_sha256 == $auto and .auto_version == $version and .primary_calls_reserved == 58 and
    .rollback_calls_reserved == 5 and .total_calls_reserved == 63 and
    .planned_worst_case_raw_tokens == 1446000 and .provider_call_cap == 64 and .raw_token_cap == 1500000 and
    .concurrency == 1 and .retries == 0 and .state == "CONSUMED_ON_RESERVATION"
  ' "$RESERVATION" >/dev/null && ff_scan "$RESERVATION" || { FF_CODE=reservation_invalid; return 1; }
  FF_RESERVATION_HASH=$reservation_hash
}

ff_validate_primary() {
  ff_verify_sidecar "$PRIMARY_LEDGER" || { FF_CODE=evidence_link_mismatch; return 1; }
  ff_validate_v2_ledger_root "$PRIMARY_LEDGER" || { FF_CODE=evidence_link_mismatch; return 1; }
  jq -e --arg auth "$AUTHORIZATION_IDENTITY_SHA256" --arg reservation "$FF_RESERVATION_HASH" '
    .version == 1 and .admission_generation == "v2" and
    .authorization_identity_sha256 == $auth and .reservation_sha256 == $reservation and
    .evidence_kind == "gpt_codex_primary_call_ledger" and
    .mode == "primary"' "$PRIMARY_LEDGER" >/dev/null || { FF_CODE=evidence_link_mismatch; return 1; }
  ff_load_primary_counters || { FF_CODE=primary_incomplete; return 1; }
  if ! jq -e '.completed == true and .evaluation_eligible == true and .failure_code == null' "$PRIMARY_LEDGER" >/dev/null; then
    FF_CODE=primary_incomplete; return 1
  fi
  jq -e --arg auto "$AUTO_HASH" --arg version "$AUTO_VERSION" --arg policy "$POLICY_HASH" \
    --arg config "$CONFIG_HASH" --slurpfile schedule "$SCHEDULE" '.attempted_calls == 58 and .successful_calls == 58 and
    .planned_calls == 58 and .observed_calls == 58 and (.calls | length) == 58 and
    .planned_worst_case_raw_tokens == 1332000 and .observed_raw_total_tokens > 0 and
    .observed_raw_total_tokens <= 1332000 and .combined_primary_and_replay_observed_raw_tokens == .observed_raw_total_tokens and
    .authorization == {provider_call_cap:64,raw_token_cap:1500000,concurrency:1,retries:0,
      primary_calls_reserved:58,rollback_calls_reserved:5,total_calls_reserved:63,planned_worst_case_raw_tokens:1446000} and
    .identity.auto_executable_sha256 == $auto and .identity.auto_version == $version and
    .identity.policy_sha256 == $policy and .identity.config_sha256 == $config and
    ([.calls[] | {sequence,task_id,arm,order,profile,role,role_ordinal,effort,raw_token_budget}] == $schedule[0].primary.rows) and
    ([.calls[].sequence] == [range(1;59)]) and ([.calls[].result.call_id] | unique | length) == 58 and
    ([.calls[].result.run_id] | unique | length) == 58 and
    ([.calls[].result.raw_total_tokens] | add) == .observed_raw_total_tokens and
    all(.calls[]; .result.status == "success" and .result.verdict == "PASS" and .result.finding_count == 0 and
      .result.tool_calls == 0 and .result.usage_status == "actual" and .result.unique_model_call_count == 1 and
      .usage.call_id == .result.call_id and .usage.run_id == .result.run_id and .usage.raw_total_tokens == .result.raw_total_tokens)
  ' "$PRIMARY_LEDGER" >/dev/null && ff_scan "$PRIMARY_LEDGER" &&
    ff_validate_opaque_ids "$PRIMARY_LEDGER" primary || { FF_CODE=evidence_link_mismatch; return 1; }
}

ff_validate_evaluator() {
  local stem file
  for stem in gpt-security-receipts gpt-quality-ledger gpt-efficiency-input gpt-efficiency-result \
    gpt-rollout-audit-input gpt-rollout-audit-result gpt-rollout-high-input gpt-rollout-high-result \
    gpt-rollout-critical-input gpt-rollout-critical-result gpt-rollout-receipts gpt-rollback-input \
    gpt-rollback-result gpt-applied-rollback gpt-primary-evaluation-summary; do
    file="$EVIDENCE_DIR/$stem-v2.json"
    ff_verify_sidecar "$file" || { FF_CODE=quality_evidence_incomplete; return 1; }
    ff_scan "$file" || { FF_CODE=retention_invalid; return 1; }
  done
  SECURITY="$EVIDENCE_DIR/gpt-security-receipts-v2.json"; QUALITY="$EVIDENCE_DIR/gpt-quality-ledger-v2.json"
  INPUT="$EVIDENCE_DIR/gpt-efficiency-input-v2.json"; RESULT="$EVIDENCE_DIR/gpt-efficiency-result-v2.json"
  ROLLOUT="$EVIDENCE_DIR/gpt-rollout-receipts-v2.json"; ROLLBACK_RESULT="$EVIDENCE_DIR/gpt-rollback-result-v2.json"
  APPLIED="$EVIDENCE_DIR/gpt-applied-rollback-v2.json"; SUMMARY="$EVIDENCE_DIR/gpt-primary-evaluation-summary-v2.json"
  SUMMARY_HASH=$(ff_sha "$SUMMARY")
  jq -e '.applied==true and .active_profile=="full_ultra" and .state_readback=="full_ultra" and
    .atomic_replace==true and .fsync_completed==true' "$APPLIED" >/dev/null || { FF_CODE=rollback_readback_invalid; return 1; }
  ff_validate_evaluator_strong || { FF_CODE=quality_evidence_incomplete; return 1; }
}

ff_validate_nested_reservation() {
  [[ -f "$ROLLBACK_RESERVATION" ]] || { FF_CODE=rollback_reservation_missing; return 1; }
  ff_verify_sidecar "$ROLLBACK_RESERVATION" || { FF_CODE=rollback_reservation_invalid; return 1; }
  FF_ROLLBACK_RESERVATION_HASH=$(ff_sha "$ROLLBACK_RESERVATION")
  jq -e --arg auth "$AUTHORIZATION_IDENTITY_SHA256" --arg reservation "$FF_RESERVATION_HASH" \
    --arg primary "$PRIMARY_HASH" --arg applied "$(ff_sha "$APPLIED")" --arg summary "$SUMMARY_HASH" '
    (keys|sort)==(["version","spec_id","evidence_kind","admission_generation","authorization_identity_sha256",
      "primary_reservation_sha256","primary_ledger_sha256","applied_rollback_receipt_sha256",
      "evaluator_summary_sha256","calls_reserved","raw_tokens_reserved","concurrency","retries","state"]|sort) and
    .version==1 and .spec_id=="SPEC-ADK-ULTRA-EFFICIENCY-001" and
    .evidence_kind=="gpt_rollback_replay_reservation" and .admission_generation=="v2" and
    .authorization_identity_sha256==$auth and .primary_reservation_sha256==$reservation and
    .primary_ledger_sha256==$primary and .applied_rollback_receipt_sha256==$applied and
    .evaluator_summary_sha256==$summary and .calls_reserved==5 and .raw_tokens_reserved==114000 and
    .concurrency==1 and .retries==0 and .state=="CONSUMED_ON_RESERVATION"' "$ROLLBACK_RESERVATION" >/dev/null &&
    ff_scan "$ROLLBACK_RESERVATION" || { FF_CODE=rollback_reservation_invalid; return 1; }
}

ff_validate_replay() {
  [[ -n "$ROLLBACK_LEDGER" ]] || { FF_CODE=rollback_replay_missing; return 1; }
  ff_validate_nested_reservation || return 1
  ff_verify_sidecar "$ROLLBACK_LEDGER" || { FF_CODE=evidence_link_mismatch; return 1; }
  ff_validate_v2_ledger_root "$ROLLBACK_LEDGER" || { FF_CODE=evidence_link_mismatch; return 1; }
  ff_load_replay_counters || { FF_CODE=rollback_replay_incomplete; return 1; }
  (( FF_PRIMARY_CALLS + FF_REPLAY_CALLS <= 63 )) || { FF_CODE=call_cap_exceeded; return 1; }
  jq -e '.completed == true and .failure_code == null' "$ROLLBACK_LEDGER" >/dev/null ||
    { FF_CODE=rollback_replay_incomplete; return 1; }
  local applied_hash
  applied_hash=$(ff_sha "$APPLIED")
  jq -e --arg auth "$AUTHORIZATION_IDENTITY_SHA256" --arg reservation "$FF_RESERVATION_HASH" \
    --arg rollback_reservation "$FF_ROLLBACK_RESERVATION_HASH" --arg summary "$SUMMARY_HASH" \
    --arg primary "$PRIMARY_HASH" --arg applied "$applied_hash" --argjson primary_raw "$FF_PRIMARY_RAW" \
    --slurpfile p "$PRIMARY_LEDGER" --slurpfile schedule "$SCHEDULE" '.version == 1 and .admission_generation == "v2" and
    .authorization_identity_sha256 == $auth and .reservation_sha256 == $reservation and
    .rollback_reservation_sha256 == $rollback_reservation and .evaluator_summary_sha256 == $summary and
    .evidence_kind == "applied_rollback_replay" and .mode == "rollback" and
    .identity == $p[0].identity and .authorization == $p[0].authorization and
    .attempted_calls == 5 and .successful_calls == 5 and .planned_calls == 5 and .observed_calls == 5 and
    (.calls | length) == 5 and .planned_worst_case_raw_tokens == 114000 and
    .primary_ledger_sha256 == $primary and .applied_rollback_receipt_sha256 == $applied and
    .combined_primary_and_replay_observed_raw_tokens == ($primary_raw + .observed_raw_total_tokens) and
    ([.calls[] | {sequence,task_id,arm,order,profile,role,role_ordinal,effort,raw_token_budget}] == $schedule[0].rollback.rows) and
    ([.calls[].effort] | map(select(. == "xhigh")) | length) == 4 and
    ([.calls[].effort] | map(select(. == "max")) | length) == 1 and
    [.calls[].role] == ["reviewer","reviewer","reviewer","security-auditor","review-consolidator"] and
    [.calls[].role_ordinal] == [1,2,3,1,1] and
    ([.calls[].result.call_id] | unique | length) == 5 and ([.calls[].result.run_id] | unique | length) == 5 and
    ([.calls[].result.call_id] - [$p[0].calls[].result.call_id] | length) == 5 and
    all(.calls[]; .task_id == "ute-corpus-v1-001" and .arm == "R" and .profile == "full5" and
      .result.status == "success" and .result.verdict == "PASS" and .result.finding_count == 0 and
      .result.tool_calls == 0 and .result.usage_status == "actual" and .result.unique_model_call_count == 1)
  ' "$ROLLBACK_LEDGER" >/dev/null && ff_scan "$ROLLBACK_LEDGER" &&
    ff_validate_opaque_ids "$ROLLBACK_LEDGER" rollback || { FF_CODE=evidence_link_mismatch; return 1; }
  (( FF_PRIMARY_RAW + FF_REPLAY_RAW <= 1500000 )) || { FF_CODE=raw_token_cap_exceeded; return 1; }
}

ff_evaluate_chain() {
  FF_CODE=evidence_link_mismatch
  ff_validate_static_authorization || return 1
  ff_validate_reservation || return 1
  ff_validate_primary || return 1
  ff_validate_evaluator || return 1
  ff_validate_replay || return 1
}

ff_write_sidecar() {
  local file=$1
  printf '%s  %s\n' "$(shasum -a 256 "$file" | awk '{print $1}')" "$(basename "$file")" > "${file%.json}.sha256"
  chmod 600 "$file" "${file%.json}.sha256"
}

ff_publish_terminal() {
  local success=$1 stage terminal closure total_calls total_raw terminal_hash replay_hash=null
  local nested_hash=${FF_ROLLBACK_RESERVATION_HASH:-null} summary_hash=${SUMMARY_HASH:-null}
  total_calls=$((FF_PRIMARY_CALLS + FF_REPLAY_CALLS)); total_raw=$((FF_PRIMARY_RAW + FF_REPLAY_RAW))
  [[ -z "$ROLLBACK_LEDGER" || ! -f "$ROLLBACK_LEDGER" ]] || replay_hash=$(ff_sha "$ROLLBACK_LEDGER")
  stage=$(mktemp -d "$EVIDENCE_DIR/.gpt-finalize.XXXXXX") || return 1
  terminal="$stage/$(basename "$TERMINAL")"; closure="$stage/$(basename "$CLOSURE")"
  jq -n --argjson success "$success" --arg code "$FF_CODE" --arg auth "$AUTHORIZATION_IDENTITY_SHA256" \
    --arg reservation "$FF_RESERVATION_HASH" --arg primary "$PRIMARY_HASH" --arg auto "$AUTO_HASH" \
    --arg replay "$replay_hash" --arg nested "$nested_hash" --arg summary "$summary_hash" \
    --argjson calls "$total_calls" --argjson raw "$total_raw" '
    {version:1,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",evidence_kind:"gpt_full_evaluation_terminal_outcome",
      admission_generation:"v2",authorization_identity_sha256:(if $auth == "null" then null else $auth end),success:$success,
      terminal_state:(if $success then "ELIGIBLE_NEXT_CANARY" else "BLOCKED_NO_PROMOTION" end),
      failure_code:(if $success then null else $code end),rollout_decision:(if $success then "ELIGIBLE_NEXT_CANARY" else "BLOCKED_NO_PROMOTION" end),
      provider_calls:$calls,observed_raw_total_tokens:$raw,retries:0,effective_profile:"full_ultra",
      promotion_eligible:false,activation_eligible:false,implemented:false,provider_calls_made_by_finalizer:0,
      reservation_sha256:(if $reservation == "null" then null else $reservation end),
      hashes:{primary_ledger_sha256:$primary,rollback_ledger_sha256:(if $replay=="null" then null else $replay end),
        rollback_reservation_sha256:(if $nested=="null" then null else $nested end),
        evaluator_summary_sha256:(if $summary=="null" then null else $summary end),auto_executable_sha256:$auto},
      privacy:{raw_retained:false}}' | jq -S . > "$terminal"
  ff_write_sidecar "$terminal"; terminal_hash=$(ff_sha "$terminal")
  jq -n --arg auth "$AUTHORIZATION_IDENTITY_SHA256" --arg reservation "$FF_RESERVATION_HASH" \
    --arg terminal "$terminal_hash" --argjson calls "$total_calls" --argjson raw "$total_raw" '
    {version:1,spec_id:"SPEC-ADK-ULTRA-EFFICIENCY-001",evidence_kind:"gpt_full_evaluation_authorization_closure",
      admission_generation:"v2",authorization_identity_sha256:(if $auth == "null" then null else $auth end),
      reservation_sha256:(if $reservation == "null" then null else $reservation end),terminal_sha256:$terminal,
      consumed:true,reusable:false,provider_calls:$calls,observed_raw_total_tokens:$raw,retries:0,
      promotion_eligible:false,activation_eligible:false,implemented:false}' | jq -S . > "$closure"
  ff_write_sidecar "$closure"
  ln "$terminal" "$TERMINAL" || { rm -rf "$stage"; return 1; }
  ln "${terminal%.json}.sha256" "${TERMINAL%.json}.sha256" || { rm -rf "$stage"; return 1; }
  ln "$closure" "$CLOSURE" || { rm -rf "$stage"; return 1; }
  ln "${closure%.json}.sha256" "${CLOSURE%.json}.sha256" || { rm -rf "$stage"; return 1; }
  rm -rf "$stage"
}
