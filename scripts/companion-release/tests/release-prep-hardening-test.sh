#!/usr/bin/env bash
set -euo pipefail
umask 077
tests_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
script_dir=$(cd -- "$tests_dir/.." && pwd)
publisher="$script_dir/publish-release-coordinates.sh"; prep="$script_dir/prepare-release.sh"
transaction="$script_dir/release-coordinate-transaction-lib.sh"; prep_lib="$script_dir/prepare-release-runtime-lib.sh"
user_lib="$script_dir/prepare-release-user-lib.sh"; local_lib="$script_dir/prepare-release-local-lib.sh"
lock_verifier="$script_dir/verify-release-prep-lock.sh"
ruleset_verifier="$script_dir/verify-release-tag-ruleset.sh"; mock_gh="$tests_dir/testdata/mock-release-prep-gh.sh"
fail() { printf 'release prep hardening test: %s\n' "$1" >&2; exit 1; }
contains() { grep -Fq -- "$2" "$1" || fail "$1 missing $2"; }
not_contains() { ! grep -Fq -- "$2" "$1" || fail "$1 unexpectedly contains $2"; }
for file in "$publisher" "$transaction" "$prep" "$prep_lib" "$user_lib" "$local_lib" "$lock_verifier" "$ruleset_verifier" "$mock_gh" \
  "$script_dir/release-tag-signing-2026-q3-r2.pub" "$script_dir/release-tag-signing-2026-q3-r2.fingerprint" \
  "$script_dir/omp-context-promotion-2026-q3-k3.pub"; do
  [[ -f "$file" && ! -L "$file" ]] || fail "missing or unsafe release-prep component $file"
done
contains "$prep" 'usage: prepare-release.sh --endpoint URL'; contains "$prep" '(--preflight|--apply)'
contains "$prep" "readonly release_tag='v0.50.110'"; contains "$prep" "expected_go_toolchain='go1.26.6'"
contains "$prep" "expected_promotion_key_id='omp-context-promotion-2026-q3-k3'"
contains "$prep" 'prepare-release-user-lib.sh prepare-release-runtime-lib.sh prepare-release-local-lib.sh'
contains "$prep" 'go build -trimpath -o "$uidrunner" ./scripts/companion-release/uidrunner'
contains "$user_lib" '/usr/bin/dscl . -create'
contains "$user_lib" '/usr/bin/dscl . -delete'
contains "$user_lib" 'UserShell /usr/bin/false'
contains "$user_lib" 'IsHidden 1'
not_contains "$user_lib" 'sudo -n -u'
contains "$prep" 'remote_mutations:0'; contains "$prep" 'canary_records:42,provider_calls:40,task_pairs:20'
contains "$local_lib" '--endpoint "$endpoint" --credential-locator "$credential_locator"'
contains "$local_lib" '--model-context-window "$model_context_window"'
contains "$local_lib" '--promotion-signing-key-id "$expected_promotion_key_id"'
contains "$local_lib" 'length == 42'; contains "$local_lib" 'length) == 40'; contains "$local_lib" 'length) == 20'
contains "$prep_lib" 'production canary started (40 sequential provider calls, 20 task pairs)'
contains "$prep_lib" '([.[] | select(.type == "call")] | length) == 40'
contains "$prep_lib" '([.[] | select(.type == "call") | .task_id_digest] | unique | length) == 20'
contains "$prep_lib" 'live_canary_uid=59999'
contains "$user_lib" '/usr/bin/dscl . -list /Users UniqueID'
contains "$prep_lib" 'isolated_uidrunner="$root/uidrunner"'
contains "$prep_lib" 'sudo -n "$isolated_uidrunner"'
contains "$prep_lib" 'cleanup_live_canary_uid'
contains "$prep_lib" 'pkill -TERM -u "$live_canary_uid"'
contains "$prep_lib" 'pgrep -u "$live_canary_uid"'
contains "$prep_lib" 'chown -R nobody:nobody'
contains "$prep_lib" 'create_release_canary_account "$isolated_home"'
contains "$prep_lib" 'remove_release_canary_account'
contains "$prep_lib" '/bin/cat "$input_jsonl"'
not_contains "$prep_lib" 'provider-credential'
not_contains "$prep_lib" 'credential_staging'
contains "$local_lib" '--static-policy-b64 "$static_policy_b64"'; not_contains "$local_lib" '--expected-signing-key-id'
contains "$publisher" 'static policy does not own the exact K3 signer'
contains "$publisher" 'release-tag-signing-2026-q3-r2.pub'
contains "$publisher" 'SHA256:7FISPXCi8p7cFEdh4Fcyyp8RPQbXYZwmo3Mxi5+YjrQ'
contains "$publisher" 'names=(ADK_COMPANION_APPROVED_SOURCE_COMMIT ADK_COMPANION_APPROVED_SOURCE_TREE OMP_CONTEXT_EVIDENCE_TAG_OBJECT_SHA'
contains "$publisher" 'gh variable set "${names[$index]}" --repo "$repository"'
contains "$publisher" 'gh variable set "${names[$index]}" --repo "$repository" --env "$environment_name"'
contains "$publisher" 'autopus.adk_release_reservation.v1'; contains "$publisher" 'verify_owned_draft_reservation'
contains "$publisher" 'verify-release-tag-ruleset.sh --armed'; contains "$publisher" 'verify-release-tag-ruleset.sh --sealed'
contains "$publisher" 'bypass_actors:[]'; contains "$publisher" 'git push --atomic --force-with-lease='
contains "$publisher" 'immutable commit point'; contains "$publisher" 'exit 75'
contains "$transaction" 'rollback incomplete; prep lock retained for reconciliation'
contains "$prep" 'scripts/companion-release/verify-homebrew-tap-pins.sh'
for forbidden in 'canonical-full-bridge' 'omp-context-bridge-release.v1.json' 'rotation_document' 'rotation_ref_commit' \
  'release-key-rotation-v0.50.109'; do
  not_contains "$prep" "$forbidden"; not_contains "$publisher" "$forbidden"
done
first_seal_line=$(grep -nF 'if ! ensure_committed_release_tag_is_sealed' "$publisher" | sed -n '1p' | cut -d: -f1)
last_seal_line=$(grep -nF 'if ! ensure_committed_release_tag_is_sealed' "$publisher" | sed -n '$p' | cut -d: -f1)
first_coordinate_line=$(grep -nF '! verify_coordinates ||' "$publisher" | sed -n '1p' | cut -d: -f1)
last_coordinate_line=$(grep -nF '! verify_coordinates ||' "$publisher" | sed -n '$p' | cut -d: -f1)
[[ "$first_seal_line" -lt "$first_coordinate_line" && "$last_seal_line" -lt "$last_coordinate_line" ]] ||
  fail 'committed tag coordinate checks can bypass immediate ruleset sealing'
preflight_line=$(grep -nF 'if [[ "$operation" == '\''preflight'\'' ]]' "$prep" | cut -d: -f1)
canary_line=$(grep -nF 'run_canary "$final_candidate"' "$prep" | cut -d: -f1)
evidence_line=$(grep -nF 'publish_local_evidence "$final_report"' "$prep" | cut -d: -f1)
[[ "$preflight_line" -lt "$canary_line" && "$preflight_line" -lt "$evidence_line" ]] || fail 'preflight can reach a remote mutation'

temp=$(mktemp -d "${TMPDIR:-/tmp}/release-prep-hardening.XXXXXX")
trap 'rm -rf -- "$temp"' EXIT
ruleset_state="$temp/ruleset-state"; mkdir -p "$ruleset_state" "$temp/ruleset-bin"
cp "$mock_gh" "$temp/ruleset-bin/gh"; chmod 0700 "$temp/ruleset-bin/gh"
printf '0\n' >"$ruleset_state/write-count"; : >"$ruleset_state/calls.log"; printf armed >"$ruleset_state/ruleset-state"
printf '%s\n' '[{"id":596,"type":"tag","name":"v0.50.110"}]' >"$ruleset_state/deployment-policies.json"
env PATH="$temp/ruleset-bin:$PATH" MOCK_RELEASE_PREP_STATE="$ruleset_state" "$ruleset_verifier" --armed
printf sealed >"$ruleset_state/ruleset-state"
env PATH="$temp/ruleset-bin:$PATH" MOCK_RELEASE_PREP_STATE="$ruleset_state" "$ruleset_verifier" --sealed
printf extra >"$ruleset_state/ruleset-state"
if env PATH="$temp/ruleset-bin:$PATH" MOCK_RELEASE_PREP_STATE="$ruleset_state" "$ruleset_verifier" --armed >/dev/null 2>&1; then
  fail 'armed verifier accepted extra actor or rule'
fi
if env PATH="$temp/ruleset-bin:$PATH" MOCK_RELEASE_PREP_STATE="$ruleset_state" "$ruleset_verifier" --sealed >/dev/null 2>&1; then
  fail 'sealed verifier accepted extra actor or rule'
fi
ssh-keygen -q -t ed25519 -N '' -f "$temp/signing-key"
ssh-keygen -q -t ed25519 -N '' -f "$temp/wrong-signing-key"
printf 'release-test %s\n' "$(<"$temp/signing-key.pub")" >"$temp/allowed-signers"
git_env=(GIT_CONFIG_COUNT=5 GIT_CONFIG_KEY_0=gpg.format GIT_CONFIG_VALUE_0=ssh
  GIT_CONFIG_KEY_1=user.signingkey GIT_CONFIG_VALUE_1="$temp/signing-key"
  GIT_CONFIG_KEY_2=gpg.ssh.allowedSignersFile GIT_CONFIG_VALUE_2="$temp/allowed-signers"
  GIT_CONFIG_KEY_3=user.name GIT_CONFIG_VALUE_3='Release Test'
  GIT_CONFIG_KEY_4=user.email GIT_CONFIG_VALUE_4='release-test@example.invalid')
real_ssh_keygen=$(command -v ssh-keygen)
policy_value() { jq -cnS --arg key "$1" '{promotion_signing_key_id:$key}' | base64 | tr '/+' '_-' | tr -d '=\n'; printf '\n'; }
setup_fixture() {
  local name=$1
  fixture="$temp/$name"; state="$fixture/state"; work="$fixture/work"; remote="$fixture/remote.git"
  mkdir -p "$state" "$fixture/bin"; cp "$mock_gh" "$fixture/bin/gh"; chmod 0700 "$fixture/bin/gh"
  printf '%s\n' '[{"name":"UNRELATED","value":"repository-before"}]' >"$state/repository-variables.json"
  printf '%s\n' '[{"name":"UNRELATED","value":"environment-before"}]' >"$state/environment-variables.json"
  printf '%s\n' '[{"id":596,"type":"tag","name":"v0.50.110"}]' >"$state/deployment-policies.json"
  printf '%s\n' '[]' >"$state/releases.json"
  printf '0\n' >"$state/write-count"; : >"$state/calls.log"; printf armed >"$state/ruleset-state"
  git init --quiet --bare "$remote"; git init --quiet "$work"; printf 'release source\n' >"$work/source.txt"
  mkdir -p "$work/scripts/companion-release"
  cp "$temp/signing-key.pub" "$work/scripts/companion-release/release-tag-signing-2026-q3-r2.pub"
  cp "$transaction" "$work/scripts/companion-release/release-coordinate-transaction-lib.sh"
  cp "$lock_verifier" "$work/scripts/companion-release/verify-release-prep-lock.sh"
  printf '%s\n' 'SHA256:7FISPXCi8p7cFEdh4Fcyyp8RPQbXYZwmo3Mxi5+YjrQ' >"$work/scripts/companion-release/release-tag-signing-2026-q3-r2.fingerprint"
  cat >"$fixture/bin/ssh-keygen" <<EOF
#!/usr/bin/env bash
if [[ "\${1-}" == '-lf' ]]; then
  printf '%s\n' '256 SHA256:7FISPXCi8p7cFEdh4Fcyyp8RPQbXYZwmo3Mxi5+YjrQ release-test (ED25519)'
  exit 0
fi
exec '$real_ssh_keygen' "\$@"
EOF
  chmod 0700 "$fixture/bin/ssh-keygen"
  cp "$ruleset_verifier" "$work/scripts/companion-release/verify-release-tag-ruleset.sh"
  chmod 0700 "$work/scripts/companion-release/verify-release-tag-ruleset.sh" \
    "$work/scripts/companion-release/verify-release-prep-lock.sh"
  env "${git_env[@]}" git -C "$work" add .; env "${git_env[@]}" git -C "$work" commit --quiet -m source
  git -C "$work" branch -M main; git -C "$work" remote add origin "$remote"; git -C "$work" push --quiet -u origin main
  source_commit=$(git -C "$work" rev-parse HEAD); source_tree=$(git -C "$work" rev-parse 'HEAD^{tree}')
  report_blob=$(printf 'report\n' | git -C "$work" hash-object -w --stdin)
  prep_tree=$(printf '100644 blob %s\tomp-context-promotion-report.v1.json\n' "$report_blob" | git -C "$work" mktree)
  prep_lock_commit=$(printf 'prep lock\n' | env "${git_env[@]}" git -C "$work" commit-tree "$prep_tree")
  attestation_blob=$(printf 'attestation\n' | git -C "$work" hash-object -w --stdin)
  evidence_tree=$(printf '100644 blob %s\tomp-context-promotion-report.v1.json\n100644 blob %s\tomp-context-promotion-attestation.v2.json\n' \
    "$report_blob" "$attestation_blob" | git -C "$work" mktree)
  evidence_commit=$(printf 'evidence\n' | env "${git_env[@]}" git -C "$work" commit-tree "$evidence_tree")
  env "${git_env[@]}" git -C "$work" tag -s omp-context-evidence-v0.50.110 "$evidence_commit" -m evidence
  evidence_tag_object=$(git -C "$work" rev-parse refs/tags/omp-context-evidence-v0.50.110)
  git -C "$work" push --quiet origin refs/tags/omp-context-evidence-v0.50.110
  git -C "$work" push --quiet origin "$prep_lock_commit:refs/heads/omp-context-evidence-v0.50.110-source"
  git -C "$work" tag -d omp-context-evidence-v0.50.110 >/dev/null
  printf 'report\n' >"$fixture/report"; printf 'attestation\n' >"$fixture/attestation"
  policy_value omp-context-promotion-2026-q3-k3 >"$fixture/static-policy.b64"
  chmod 0600 "$fixture/report" "$fixture/attestation" "$fixture/static-policy.b64"
  mkdir -p "$remote/hooks"
  cat >"$remote/hooks/pre-receive" <<'HOOK'
#!/usr/bin/env bash
set -euo pipefail
while read -r _ _ ref; do
  if [[ "$ref" == 'refs/tags/v0.50.110' ]]; then
    printf '%s\n' release-tag-push >>"$MOCK_RELEASE_PREP_STATE/calls.log"
    [[ "${MOCK_RELEASE_PREP_REJECT_TAG:-0}" -eq 0 ]] || exit 1
  fi
done
HOOK
  chmod 0700 "$remote/hooks/pre-receive"
}
run_publisher() {
  local lock_argument=${RELEASE_PREP_LOCK_ARGUMENT:-fresh:$prep_lock_commit}
  local signing_key=${RELEASE_PREP_SIGNING_KEY:-$temp/signing-key}
  env "${git_env[@]}" PATH="$fixture/bin:$PATH" MOCK_RELEASE_PREP_STATE="$state" \
    MOCK_RELEASE_PREP_FAIL_AT="${MOCK_RELEASE_PREP_FAIL_AT:-}" MOCK_RELEASE_PREP_FAIL_FROM="${MOCK_RELEASE_PREP_FAIL_FROM:-}" \
    MOCK_RELEASE_PREP_RELEASE_DELETE_FAIL="${MOCK_RELEASE_PREP_RELEASE_DELETE_FAIL:-0}" \
    MOCK_RELEASE_PREP_REJECT_TAG="${MOCK_RELEASE_PREP_REJECT_TAG:-0}" \
    bash "$publisher" Insajin/autopus-adk adk-companion-release v0.50.110 "$source_commit" "$source_tree" \
    "$fixture/static-policy.b64" "$evidence_tag_object" "$evidence_commit" "$evidence_tree" \
    "$(shasum -a 256 "$fixture/report" | awk '{print $1}')" \
    "$(shasum -a 256 "$fixture/attestation" | awk '{print $1}')" "$lock_argument" "$signing_key"
}
setup_fixture k3_mismatch
policy_value omp-context-promotion-2026-q3-k2 >"$fixture/static-policy.b64"
if (cd "$work" && run_publisher >/dev/null 2>&1); then fail 'publisher accepted a K2 active policy'; fi
[[ "$(<"$state/write-count")" == '0' ]] || fail 'K3 mismatch mutated release state'
setup_fixture r2_mismatch
if (cd "$work" && RELEASE_PREP_SIGNING_KEY="$temp/wrong-signing-key" run_publisher >/dev/null 2>&1); then fail 'publisher accepted a non-R2 tag signer'; fi
[[ "$(<"$state/write-count")" == '0' ]] || fail 'R2 mismatch mutated release state'
setup_fixture success
(cd "$work" && run_publisher >"$fixture/result.json")
jq -e '.mode == "committed" and .release_tag == "v0.50.110" and .promotion_signing_key_id == "omp-context-promotion-2026-q3-k3"' "$fixture/result.json" >/dev/null || fail 'success receipt differs'
[[ "$(jq 'length' "$state/repository-variables.json")" == '9' && "$(jq 'length' "$state/environment-variables.json")" == '9' ]] || fail 'coordinate transaction did not converge both variable scopes'
jq -e 'length == 1 and .[0].draft == true and (.[0].assets | length) == 0 and (.[0].body | fromjson | .schema_version == "autopus.adk_release_reservation.v1")' "$state/releases.json" >/dev/null || fail 'operator draft reservation differs'
tag_call=$(grep -n '^release-tag-push$' "$state/calls.log" | cut -d: -f1); seal_call=$(grep -n '^ruleset-seal$' "$state/calls.log" | cut -d: -f1)
[[ "$tag_call" -lt "$seal_call" && "$(<"$state/ruleset-state")" == sealed ]] || fail 'ruleset was not sealed only after the R2 commit'
calls_before=$(wc -l <"$state/calls.log" | tr -d ' ')
(cd "$work" && RELEASE_PREP_LOCK_ARGUMENT=reconcile run_publisher >"$fixture/reconciled.json")
jq -e '.mode == "reconciled"' "$fixture/reconciled.json" >/dev/null || fail 'committed release did not reconcile'
[[ "$(wc -l <"$state/calls.log" | tr -d ' ')" == "$calls_before" ]] || fail 'reconciliation mutated release state'
setup_fixture rollback
cp "$state/repository-variables.json" "$fixture/repository-before.json"
if (cd "$work" && MOCK_RELEASE_PREP_FAIL_AT=5 run_publisher >/dev/null 2>&1); then fail 'injected coordinate failure succeeded'; fi
jq -e --slurpfile before "$fixture/repository-before.json" '. == $before[0]' \
  "$state/repository-variables.json" >/dev/null || fail 'CAS rollback did not restore repository variables'
[[ -z "$(git -C "$work" ls-remote --refs origin refs/tags/v0.50.110)" ]] || fail 'rollback created a release tag'
setup_fixture retained
retained_status=0
(cd "$work" && MOCK_RELEASE_PREP_REJECT_TAG=1 MOCK_RELEASE_PREP_RELEASE_DELETE_FAIL=1 run_publisher >/dev/null 2>&1) || retained_status=$?
[[ "$retained_status" -eq 75 ]] || fail 'incomplete rollback did not request reconciliation'
(cd "$work" && RELEASE_PREP_LOCK_ARGUMENT="retained:$prep_lock_commit" run_publisher >"$fixture/retry.json")
jq -e '.mode == "committed"' "$fixture/retry.json" >/dev/null || fail 'retained exit-75 transaction did not reconcile'
printf 'release prep hardening test: PASS\n'
