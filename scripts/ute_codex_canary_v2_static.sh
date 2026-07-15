#!/usr/bin/env bash

V2_STATIC_STEMS=(gpt-codex-policy-v2 gpt-codex-config-v2 gpt-verdict-schema-v2
  gpt-canary-schedule-v2 gpt-snapshot-manifest-v2 gpt-prior-evidence-manifest-v2
  gpt-full-evaluation-identity-v2 gpt-canary-preflight-v2)
V2_ADMISSION_MEMBERS=(ute_codex_canary_v2.sh ute_codex_canary_v2_static.sh
  ute_codex_canary_v2_authorization.sh ute_codex_canary_v2_ledger.sh
  ute_codex_canary_lib.sh ute_codex_canary_prompt.sh ute_codex_canary_receipt.sh
  ute_codex_canary_exec.sh ute_codex_canary_schedule.sh)

v2_sha_uri() { printf 'sha256:%s\n' "$(sha256_file "$1")"; }

v2_compute_admission_bundle() {
  local tmp name
  tmp=$(mktemp "${TMPDIR:-/tmp}/ute-v2-admission.XXXXXX") || return 1
  for name in "${V2_ADMISSION_MEMBERS[@]}"; do
    [[ -f "$SCRIPT_DIR/$name" && ! -L "$SCRIPT_DIR/$name" ]] || { rm -f "$tmp"; return 1; }
    printf '%s  scripts/%s\n' "$(sha256_file "$SCRIPT_DIR/$name")" "$name" >> "$tmp"
  done
  LC_ALL=C sort -k2 "$tmp" -o "$tmp"
  v2_sha_uri "$tmp"
  rm -f "$tmp"
}

v2_validate_admission_bundle() {
  local policy=$1 identity=$2 preflight=$3 computed harness
  computed=$(v2_compute_admission_bundle) || return 1
  harness=$(v2_sha_uri "$SCRIPT_DIR/ute_codex_canary_v2.sh")
  jq -e --arg bundle "$computed" --arg harness "$harness" '
    .execution_runtime.harness_sha256==$harness and
    .execution_runtime.admission_bundle_sha256==$bundle and
    .execution_runtime.admission_bundle_algorithm=="sha256-named-member-manifest-v1" and
    .execution_runtime.admission_bundle_member_count==9
  ' "$policy" >/dev/null || return 1
  jq -e --arg bundle "$computed" --arg harness "$harness" '
    .runtime.harness_sha256==$harness and .runtime.admission_bundle_sha256==$bundle and
    .runtime.admission_bundle_algorithm=="sha256-named-member-manifest-v1" and
    .runtime.admission_bundle_member_count==9
  ' "$identity" >/dev/null || return 1
  jq -e --arg bundle "$computed" --arg harness "$harness" '
    .runtime_candidate.harness_sha256==$harness and
    .runtime_candidate.admission_bundle_sha256==$bundle and
    .runtime_candidate.admission_bundle_algorithm=="sha256-named-member-manifest-v1" and
    .runtime_candidate.admission_bundle_member_count==9
  ' "$preflight" >/dev/null
}

v2_resolve_executable() {
  local candidate link count=0
  candidate=$(command -v "$1") || return 1
  [[ "$candidate" == /* ]] || return 1
  while [[ -L "$candidate" ]]; do
    (( count+=1 )); (( count <= 40 )) || return 1
    link=$(readlink "$candidate") || return 1
    if [[ "$link" == /* ]]; then candidate=$link; else candidate="$(dirname "$candidate")/$link"; fi
    candidate="$(cd "$(dirname "$candidate")" 2>/dev/null && pwd -P)/$(basename "$candidate")" || return 1
  done
  candidate="$(cd "$(dirname "$candidate")" 2>/dev/null && pwd -P)/$(basename "$candidate")" || return 1
  [[ -f "$candidate" && -x "$candidate" && ! -L "$candidate" ]] || return 1
  printf '%s\n' "$candidate"
}

v2_validate_static_evidence() {
  local dir=$1 stem policy config schema schedule snapshot prior identity preflight terminal
  for stem in corpus-v1 gpt-canary-cohort-v1; do
    [[ -f "$dir/$stem.json" && ! -L "$dir/$stem.json" &&
       -f "$dir/$stem.sha256" && ! -L "$dir/$stem.sha256" ]] || return 1
  done
  for stem in "${V2_STATIC_STEMS[@]}"; do
    [[ -f "$dir/$stem.json" && ! -L "$dir/$stem.json" &&
       -f "$dir/$stem.sha256" && ! -L "$dir/$stem.sha256" ]] || return 1
  done
  validate_static_evidence "$dir" || return 1
  terminal="$dir/gpt-transport-smoke-terminal-outcome-v8.json"
  [[ -f "$terminal" && ! -L "$terminal" && -f "${terminal%.json}.sha256" &&
     ! -L "${terminal%.json}.sha256" ]] || return 1
  verify_named_sidecar "$terminal" "${terminal%.json}.sha256" || return 1
  for stem in "${V2_STATIC_STEMS[@]}"; do
    verify_named_sidecar "$dir/$stem.json" "$dir/$stem.sha256" || return 1
    jq empty "$dir/$stem.json" >/dev/null || return 1
  done
  policy="$dir/gpt-codex-policy-v2.json"; config="$dir/gpt-codex-config-v2.json"
  schema="$dir/gpt-verdict-schema-v2.json"; schedule="$dir/gpt-canary-schedule-v2.json"
  snapshot="$dir/gpt-snapshot-manifest-v2.json"; prior="$dir/gpt-prior-evidence-manifest-v2.json"
  identity="$dir/gpt-full-evaluation-identity-v2.json"; preflight="$dir/gpt-canary-preflight-v2.json"
  jq -e '
    .version==2 and .admission_generation=="v2" and
    .authorization=={provider:"codex",provider_call_cap:64,raw_token_cap:1500000,concurrency:1,retries:0} and
    .hard_envelope.primary_calls==58 and .hard_envelope.primary_xhigh_calls==44 and
    .hard_envelope.primary_max_calls==14 and .hard_envelope.primary_worst_case_raw_tokens==1332000 and
    .hard_envelope.rollback_replay_calls==5 and .hard_envelope.rollback_worst_case_raw_tokens==114000 and
    .hard_envelope.planned_total_calls==63 and .hard_envelope.planned_worst_case_raw_tokens==1446000 and
    .hard_envelope.raw_token_safety_margin==54000 and .hard_envelope.unplanned_call_slots==1 and
    (.execution_runtime.harness_sha256|test("^sha256:[0-9a-f]{64}$")) and
    (.execution_runtime.full_chain_harness_sha256|test("^sha256:[0-9a-f]{64}$")) and
    .execution_runtime.full_chain_algorithm=="sha256-named-member-manifest-v1" and
    .execution_runtime.full_chain_member_count==17 and
    (.execution_runtime.admission_bundle_sha256|test("^sha256:[0-9a-f]{64}$")) and
    .execution_runtime.admission_bundle_algorithm=="sha256-named-member-manifest-v1" and
    .execution_runtime.admission_bundle_member_count==9 and
    (.execution_runtime.auto_executable_sha256|test("^sha256:[0-9a-f]{64}$")) and
    (.execution_runtime.codex_executable_sha256|test("^sha256:[0-9a-f]{64}$")) and
    .execution_runtime.codex_cli_version=="0.144.1" and
    .decision.activation==false and .decision.promotion==false and .decision.implemented==false
  ' "$policy" >/dev/null || return 1
  jq -e '
    .version==2 and .admission_generation=="v2" and .provider=="codex" and
    .provider_version=="0.144.1" and .model=="gpt-5.6-sol" and .model_version=="gpt-5.6-sol" and
    .canonical_execution.argv==["auto","agent","run","<corpus-task-id>"] and
    .canonical_execution.direct_codex_forbidden==true and .canonical_execution.concurrency==1 and
    .canonical_execution.retries==0 and .context_options.evidence_mode==true and
    .context_options.strict_verdict==true and .context_options.zero_tool_calls_required==true and
    .context_options.codex.sandbox=="read-only" and .context_options.codex.ephemeral==true and
    .context_options.codex.zero_tool_mode==true
  ' "$config" >/dev/null || return 1
  jq -e '.=={type:"object",properties:{verdict:{type:"string",enum:["PASS","FAIL"]},finding_count:{type:"integer"}},required:["verdict","finding_count"],additionalProperties:false}' \
    "$schema" >/dev/null || return 1
  jq -e '
    .version==2 and .admission_generation=="v2" and
    .primary.calls==58 and .primary.xhigh_calls==44 and .primary.max_calls==14 and
    .primary.worst_case_raw_tokens==1332000 and (.primary.rows|length)==58 and
    .rollback.calls==5 and .rollback.xhigh_calls==4 and .rollback.max_calls==1 and
    .rollback.worst_case_raw_tokens==114000 and (.rollback.rows|length)==5
  ' "$schedule" >/dev/null || return 1
  jq -e '.version==2 and .admission_generation=="v2" and .repository.bare==true and
    (.repository.head_commit|test("^[0-9a-f]{40}$")) and (.required_commits|length)>0' "$snapshot" >/dev/null || return 1
  jq -e '.version==2 and .admission_generation=="v2" and (.artifacts|type)=="array" and
    all(.artifacts[]; (.name|type)=="string" and (.sha256|test("^sha256:[0-9a-f]{64}$")))' "$prior" >/dev/null || return 1
  jq -e --arg policy "$(v2_sha_uri "$policy")" --arg config "$(v2_sha_uri "$config")" \
    --arg schema "$(v2_sha_uri "$schema")" --arg schedule "$(v2_sha_uri "$schedule")" \
    --arg snapshot "$(v2_sha_uri "$snapshot")" --arg prior "$(v2_sha_uri "$prior")" \
    --arg corpus "$(v2_sha_uri "$dir/corpus-v1.json")" --arg cohort "$(v2_sha_uri "$dir/gpt-canary-cohort-v1.json")" \
    --arg terminal "$(v2_sha_uri "$terminal")" '
    .version==2 and .admission_generation=="v2" and
    .frozen_artifacts=={policy_sha256:$policy,config_sha256:$config,verdict_schema_sha256:$schema,
      schedule_sha256:$schedule,snapshot_manifest_sha256:$snapshot,prior_evidence_manifest_sha256:$prior,
      corpus_sha256:$corpus,cohort_sha256:$cohort,transport_terminal_sha256:$terminal} and
    (.runtime.harness_sha256|test("^sha256:[0-9a-f]{64}$")) and
    (.runtime.full_chain_harness_sha256|test("^sha256:[0-9a-f]{64}$")) and
    .runtime.full_chain_algorithm=="sha256-named-member-manifest-v1" and
    .runtime.full_chain_member_count==17 and
    (.runtime.admission_bundle_sha256|test("^sha256:[0-9a-f]{64}$")) and
    .runtime.admission_bundle_algorithm=="sha256-named-member-manifest-v1" and
    .runtime.admission_bundle_member_count==9 and
    (.runtime.auto_executable_sha256|test("^sha256:[0-9a-f]{64}$")) and
    (.runtime.codex_executable_sha256|test("^sha256:[0-9a-f]{64}$")) and
    .runtime.codex_cli_version=="0.144.1" and
    .authorization_envelope.provider_call_cap==64 and .authorization_envelope.raw_token_cap==1500000 and
    .authorization_envelope.primary_calls==58 and .authorization_envelope.rollback_calls==5 and
    .authorization_envelope.total_calls==63 and .authorization_envelope.planned_worst_case_raw_tokens==1446000 and
    .authorization_envelope.concurrency==1 and .authorization_envelope.retries==0 and
    .decision.activation==false and .decision.promotion==false and .decision.implemented==false
  ' "$identity" >/dev/null || return 1
  V2_AUTHORIZATION_IDENTITY_SHA256=$(v2_sha_uri "$identity")
  jq -e --arg auth "$V2_AUTHORIZATION_IDENTITY_SHA256" --arg identity "$V2_AUTHORIZATION_IDENTITY_SHA256" \
    --arg policy "$(v2_sha_uri "$policy")" --arg config "$(v2_sha_uri "$config")" \
    --arg schema "$(v2_sha_uri "$schema")" --slurpfile policy_doc "$policy" --slurpfile identity_doc "$identity" \
    --slurpfile corpus "$dir/corpus-v1.json" --slurpfile v1_preflight "$dir/gpt-canary-preflight-v1.json" '
    .version==2 and .admission_generation=="v2" and .authorization_identity.sha256==$auth and
    .frozen_artifacts.policy_sha256==$policy and .frozen_artifacts.config_sha256==$config and
    .frozen_artifacts.verdict_schema_sha256==$schema and
    .frozen_artifacts.full_evaluation_identity_sha256==$identity and
    .runtime_candidate.full_chain_harness_sha256==$policy_doc[0].execution_runtime.full_chain_harness_sha256 and
    .runtime_candidate.full_chain_harness_sha256==$identity_doc[0].runtime.full_chain_harness_sha256 and
    .runtime_candidate.full_chain_algorithm=="sha256-named-member-manifest-v1" and
    .runtime_candidate.full_chain_member_count==17 and
    .runtime_candidate.admission_bundle_sha256==$policy_doc[0].execution_runtime.admission_bundle_sha256 and
    .runtime_candidate.admission_bundle_sha256==$identity_doc[0].runtime.admission_bundle_sha256 and
    .runtime_candidate.admission_bundle_algorithm=="sha256-named-member-manifest-v1" and
    .runtime_candidate.admission_bundle_member_count==9 and
    .runtime_candidate.codex_executable_sha256==$policy_doc[0].execution_runtime.codex_executable_sha256 and
    .runtime_candidate.codex_executable_sha256==$identity_doc[0].runtime.codex_executable_sha256 and
    .runtime_candidate.auto_executable_sha256==$policy_doc[0].execution_runtime.auto_executable_sha256 and
    .runtime_candidate.auto_executable_sha256==$identity_doc[0].runtime.auto_executable_sha256 and
    .runtime_candidate.auto_version==$policy_doc[0].execution_runtime.auto_version and
    .runtime_candidate.auto_version==$identity_doc[0].runtime.auto_version and
    .runtime_candidate.codex_cli_version=="0.144.1" and
    .observed_spend.provider_calls_made==0 and .observed_spend.raw_total_tokens==0 and
    (.deterministic_target_preflight as $d |
      $d==$v1_preflight[0].deterministic_target_preflight and
      $d.status=="PASS" and $d.task_count==12 and $d.passed_tasks==12 and $d.failed_tasks==0 and
      ($d.records|length)==12 and ([$d.records[].task_id]|unique|length)==12 and
      all($d.records[]; . as $r | any($corpus[0].tasks[];
        .task_id==$r.task_id and .base_commit==$r.base_commit and .target_commit==$r.target_commit and
        .verification_command==$r.verification_command and .oracle.hash==$r.expected_patch_hash and
        .oracle.hash==$r.observed_patch_hash and $r.verification_exit_code==0 and $r.status=="PASS"))) and
    .decision.status=="AWAITING_EXPLICIT_FULL_EVALUATION_AUTHORIZATION" and
    .decision.provider_execution_started==false and .decision.activation==false and
    .decision.promotion==false and .decision.implemented==false
  ' "$preflight" >/dev/null || return 1
  v2_validate_admission_bundle "$policy" "$identity" "$preflight" || return 1
  jq -e '.version==8 and .outcome=="TRANSPORT_DIAGNOSIS_TERMINAL_PASS" and
    .authorization_consumed==true and .same_authorization_resumable==false and
    .execution.planned_calls==1 and .execution.attempted_calls==1 and .execution.actual_usage_calls==1 and
    .execution.usage_status=="actual" and .execution.unique_model_call_count==1 and .execution.tool_calls==0 and
    .execution.retries==0 and .execution.transport_schema_conformance=="PASS" and
    .decision.transport_precondition_for_full_evaluation_satisfied==true and
    .decision.full_58_plus_5_execution_authorized==false and
    .decision.next_full_evaluation_requires_new_frozen_admission_and_explicit_authorization==true
  ' "$terminal" >/dev/null
}

v2_validate_snapshot_binding() {
  local repo=$1 corpus=$2 manifest=$3 head
  validate_repo_snapshot "$repo" "$corpus" || return 1
  [[ "$(snapshot_git "$repo" rev-parse --is-bare-repository)" == true ]] || return 1
  head=$(snapshot_git "$repo" rev-parse HEAD) || return 1
  jq -e --arg head "$head" '.repository.bare==true and .repository.head_commit==$head' "$manifest" >/dev/null
}

v2_prepare_runtime_identity() {
  local auto_out codex_out expected_auto expected_codex expected_version
  v2_validate_admission_bundle "$EVIDENCE_DIR/gpt-codex-policy-v2.json" \
    "$V2_IDENTITY_FILE" "$EVIDENCE_DIR/gpt-canary-preflight-v2.json" || return 1
  AUTO_SHA="$(v2_sha_uri "$AUTO")"
  CODEX_BIN=$(v2_resolve_executable codex) || return 1
  CODEX_EXECUTABLE_SHA=$(v2_sha_uri "$CODEX_BIN")
  expected_auto=$(jq -r '.runtime.auto_executable_sha256' "$V2_IDENTITY_FILE")
  expected_codex=$(jq -r '.runtime.codex_executable_sha256' "$V2_IDENTITY_FILE")
  expected_version=$(jq -r '.runtime.auto_version' "$V2_IDENTITY_FILE")
  [[ "$AUTO_SHA" == "$expected_auto" && "$CODEX_EXECUTABLE_SHA" == "$expected_codex" ]] || return 1
  [[ "$(v2_sha_uri "$SCRIPT_DIR/ute_codex_canary_v2.sh")" == "$(jq -r '.runtime.harness_sha256' "$V2_IDENTITY_FILE")" ]] || return 1
  AUTO_VERSION=$("$AUTO" version --short 2>/dev/null) || return 1
  [[ "$AUTO_VERSION" =~ ^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$ ]] || return 1
  [[ "$AUTO_VERSION" == "$expected_version" ]] || return 1
  codex_out=$("$CODEX_BIN" --version 2>/dev/null) || return 1
  [[ "$codex_out" == "codex-cli 0.144.1" ]] || return 1
  CODEX_VERSION=0.144.1
  CODEX_VERSION_HASH="sha256:$(printf 'codex-cli 0.144.1\n' | shasum -a 256 | awk '{print $1}')"
  [[ "$(v2_sha_uri "$AUTO")" == "$AUTO_SHA" && "$(v2_sha_uri "$CODEX_BIN")" == "$CODEX_EXECUTABLE_SHA" ]] || return 1
  v2_validate_admission_bundle "$EVIDENCE_DIR/gpt-codex-policy-v2.json" \
    "$V2_IDENTITY_FILE" "$EVIDENCE_DIR/gpt-canary-preflight-v2.json"
}

v2_validate_primary_ledger_for_rollback() {
  local ledger=$1 expected actual
  [[ "$(cd "$(dirname "$ledger")" && pwd -P)/$(basename "$ledger")" == "$EVIDENCE_DIR/gpt-primary-call-ledger-v2.json" ]] || return 1
  verify_named_sidecar "$ledger" "${ledger%.json}.sha256" || return 1
  jq -e --arg auth "$V2_AUTHORIZATION_IDENTITY_SHA256" --arg corpus "$CORPUS_HASH" \
    --arg cohort "$COHORT_HASH" --arg policy "$POLICY_HASH" --arg config "$CONFIG_HASH" \
    --arg schema "$SCHEMA_HASH" --arg auto "$AUTO_SHA" --arg reservation "$V2_RESERVATION_SHA256" '
    .version==1 and .admission_generation=="v2" and .authorization_identity_sha256==$auth and
    .reservation_sha256==$reservation and
    .evidence_kind=="gpt_codex_primary_call_ledger" and .completed==true and
    .evaluation_eligible==true and .promotion_eligible==false and .attempted_calls==58 and
    .successful_calls==58 and .planned_calls==58 and .observed_calls==58 and (.calls|length)==58 and
    .planned_worst_case_raw_tokens==1332000 and .observed_raw_total_tokens>0 and .observed_raw_total_tokens<=1332000 and
    .authorization=={provider_call_cap:64,raw_token_cap:1500000,concurrency:1,retries:0,
      primary_calls_reserved:58,rollback_calls_reserved:5,total_calls_reserved:63,
      planned_worst_case_raw_tokens:1446000} and
    .identity.corpus_sha256==$corpus and .identity.cohort_sha256==$cohort and
    .identity.policy_sha256==$policy and .identity.config_sha256==$config and
    .identity.verdict_schema_sha256==$schema and .identity.auto_executable_sha256==$auto and
    .identity.codex_cli_version=="0.144.1" and ([.calls[].sequence]==[range(1;59)]) and
    ([.calls[].effort]|map(select(.=="xhigh"))|length)==44 and
    ([.calls[].effort]|map(select(.=="max"))|length)==14 and
    ([.calls[].result.call_id]|unique|length)==58 and ([.calls[].result.run_id]|unique|length)==58 and
    all(.calls[]; .result.status=="success" and .result.verdict=="PASS" and
      .result.finding_count==0 and .result.tool_calls==0 and .result.usage_status=="actual" and
      .result.unique_model_call_count==1 and .usage.call_id==.result.call_id and
      .usage.run_id==.result.run_id and .usage.raw_total_tokens==.result.raw_total_tokens and
      .usage.risk_policy==$policy and .usage.config_hash==$config and
      .agent_run.status=="PASS" and .agent_run.acceptance_status=="PASS" and .agent_run.tool_calls==0)
  ' "$ledger" >/dev/null || return 1
  expected=$(emit_primary_schedule)
  actual=$(jq -r '.calls[]|[.sequence,.task_id,.arm,.order,.profile,.role,.role_ordinal,.effort,.raw_token_budget]|@tsv' "$ledger")
  [[ "$actual" == "$expected" ]]
}
