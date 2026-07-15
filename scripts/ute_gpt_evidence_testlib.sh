#!/usr/bin/env bash

gpt_fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

gpt_assert_empty() {
  [[ -d "$1" && -z "$(find "$1" -mindepth 1 -print -quit)" ]] || gpt_fail "$2"
}

gpt_write_sidecar() {
  local file=$1
  printf '%s  %s\n' "$(shasum -a 256 "$file" | awk '{print $1}')" "$(basename "$file")" > "${file%.json}.sha256"
}

gpt_normalize_fake_ledger() {
  local source=$1 target=$2 auto=$3 auto_hash auto_version
  auto_hash="sha256:$(shasum -a 256 "$auto" | awk '{print $1}')"
  auto_version=$("$auto" version --short)
  jq --arg hash "$auto_hash" --arg version "$auto_version" \
    '.identity.auto_executable_sha256 = $hash | .identity.auto_version = $version' \
    "$source" > "$target"
  chmod 600 "$target"
  gpt_write_sidecar "$target"
}

gpt_assert_named_outputs() {
  local output=$1 stem
  for stem in \
    gpt-security-receipts-v1 gpt-quality-ledger-v1 gpt-efficiency-input-v1 \
    gpt-efficiency-result-v1 gpt-rollout-audit-input-v1 gpt-rollout-audit-result-v1 \
    gpt-rollout-high-input-v1 gpt-rollout-high-result-v1 \
    gpt-rollout-critical-input-v1 gpt-rollout-critical-result-v1 \
    gpt-rollout-receipts-v1 gpt-rollback-input-v1 gpt-rollback-result-v1 \
    gpt-applied-rollback-v1 gpt-primary-evaluation-summary-v1; do
    [[ -f "$output/$stem.json" && -f "$output/$stem.sha256" ]] || gpt_fail "missing output $stem"
    local expected name
    read -r expected name < "$output/$stem.sha256"
    [[ "$name" == "$stem.json" && "$expected" == "$(shasum -a 256 "$output/$stem.json" | awk '{print $1}')" ]] || \
      gpt_fail "bad sidecar $stem"
  done
}

gpt_assert_success_contracts() {
  local output=$1
  jq -e '.receipt_count == 14 and (.receipts | length) == 14 and
    ([.receipts[].receipt_sha256] | unique | length) == 14 and
    all(.receipts[]; .security_call.role == "security-auditor" and
      .security_call.effort == "max" and .security_call.tool_calls == 0 and
      .security_call.usage_status == "actual" and .security_call.verdict == "PASS" and
      .patch_evidence.safe_modes == true and .patch_evidence.git_diff_check == "PASS" and
      .verification.status == "PASS" and (.receipt_sha256 | test("^sha256:[0-9a-f]{64}$")))' \
    "$output/gpt-security-receipts-v1.json" >/dev/null || gpt_fail "security receipt contract"
  jq -e '.row_count == 7 and (.outcomes | length) == 7 and
    all(.outcomes[]; .expected_oracle_hash == .baseline_observed_oracle_hash and
      .expected_oracle_hash == .candidate_observed_oracle_hash and
      .baseline_verification_exit_code == 0 and .candidate_verification_exit_code == 0 and
      .baseline_security_status == "PASS" and .candidate_security_status == "PASS")' \
    "$output/gpt-quality-ledger-v1.json" >/dev/null || gpt_fail "quality ledger contract"
  jq -e '.version == 1 and (.calls | length) == 58 and (.trials | length) == 14 and
    (.quality_outcomes | length) == 7 and (.expected_task_ids | length) == 7' \
    "$output/gpt-efficiency-input-v1.json" >/dev/null || gpt_fail "efficiency input contract"
  jq -e '.measurement.measurement_gate == "PASS" and .measurement.neutrality_gate == "PASS" and
    .measurement.actual_usage_capture_pct == 100 and .comparison.expected_task_count == 7 and
    .comparison.paired_expected_task_count == 7 and .comparison.expected_corpus_complete == true and
    .comparison.median_paired_raw_reduction_pct >= 25 and
    .quality.complete == true and .quality.consistent == true and
    .quality.objective_pass_count == 7 and .quality.security_pass_count == 7 and
    .promotion.rollout_decision == "ELIGIBLE_NEXT_CANARY" and
    .rollout_receipt.decision == "CANARY" and .rollout_receipt.active_profile == "compact_ultra" and
    .rollout_receipt.full_depth == false' "$output/gpt-efficiency-result-v1.json" >/dev/null || \
    gpt_fail "primary evaluator contract"
  jq -e '.audit.decision == "AUDIT" and .audit.active_profile == "full_ultra" and
    .high.decision == "CANARY" and .high.active_profile == "full_ultra" and
    .high.selection_reason == "risk_requires_full_depth" and
    .critical.decision == "CANARY" and .critical.active_profile == "full_ultra" and
    .critical.selection_reason == "risk_requires_full_depth"' "$output/gpt-rollout-receipts-v1.json" >/dev/null || \
    gpt_fail "rollout receipt contract"
  jq -e '.promotion.rollout_decision == "ROLLBACK" and
    (.promotion.reason_codes | index("policy_parity_failed")) != null and
    .rollout_receipt.decision == "ROLLBACK" and .rollout_receipt.active_profile == "full_ultra"' \
    "$output/gpt-rollback-result-v1.json" >/dev/null || gpt_fail "logical rollback contract"
  jq -e '.decision == "ROLLBACK" and .active_profile == "full_ultra" and .applied == true and
    .atomic_replace == true and .fsync_completed == true and .state_readback == "full_ultra" and
    .before_binding_sha256 != .after_binding_sha256 and
    (.state_readback_sha256 | test("^sha256:[0-9a-f]{64}$"))' "$output/gpt-applied-rollback-v1.json" >/dev/null || \
    gpt_fail "applied rollback contract"
  jq -e '.primary_calls == 58 and .quality_tasks_passed == 7 and .quality_tasks_expected == 7 and
    .security_receipts_passed == 14 and .security_receipts_expected == 14 and
    .provider_calls_made_by_builder == 0 and .promotion_eligible == false and
    .replay_eligible == true and .rollback_replay_status == "pending"' \
    "$output/gpt-primary-evaluation-summary-v1.json" >/dev/null || gpt_fail "summary contract"
}

gpt_expect_builder_failure() {
  local harness=$1 ledger=$2 repo=$3 auto=$4 output=$5
  mkdir -p "$output"
  if "$harness" evaluate --primary-ledger "$ledger" --repo "$repo" --auto "$auto" --output "$output" >/dev/null 2>&1; then
    gpt_fail "tampered input was accepted"
  fi
  gpt_assert_empty "$output" "failure published eligible artifacts"
}
