#!/usr/bin/env bash
set -euo pipefail
umask 077
tests_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
script_dir=$(cd -- "$tests_dir/.." && pwd)
publisher="$script_dir/publish-release-coordinates.sh"
prep="$script_dir/prepare-release.sh"
publisher_lib="$script_dir/release-coordinate-transaction-lib.sh"
prep_lib="$script_dir/prepare-release-runtime-lib.sh"
prep_local_lib="$script_dir/prepare-release-local-lib.sh"
lock_verifier="$script_dir/verify-release-prep-lock.sh"
mock_gh="$tests_dir/testdata/mock-release-prep-gh.sh"
fail() { printf 'release prep hardening test: %s\n' "$1" >&2; exit 1; }
contains() { grep -Fq -- "$2" "$1" || fail "$1 missing $2"; }
not_contains() { ! grep -Fq -- "$2" "$1" || fail "$1 unexpectedly contains $2"; }
for file in "$publisher" "$publisher_lib" "$prep" "$prep_lib" "$prep_local_lib" "$lock_verifier" "$mock_gh" \
  "$script_dir/materialize-omp-release-canary.sh" "$script_dir/remove-omp-release-canary.sh" \
  "$script_dir/release-tag-signing-2026-q3.pub" "$script_dir/release-tag-signing-2026-q3.fingerprint"; do
  [[ -f "$file" && ! -L "$file" ]] || fail "missing or unsafe release-prep component $file"
done
not_contains "$prep_lib" 'watch_dispatch'
not_contains "$prep" 'bootstrap_candidate'
contains "$prep" 'run_canary "$final_candidate"'
contains "$prep_local_lib" 'omp-context-canary-plan'
contains "$prep" "cmp \"\$static_policy_file\" \"\$final_static_policy_file\""
not_contains "$prep" 'omp-context-promote.yml'
not_contains "$prep" 'companion-release-preflight.yml'
contains "$prep_local_lib" 'omp-context-promotion-attestation'; contains "$prep_local_lib" '--valid-for 24h'; contains "$prep_local_lib" '--expected-signing-key-id "$expected_promotion_key_id"'
contains "$prep_local_lib" 'push --atomic --force-with-lease='; contains "$prep_local_lib" 'https://github.com/Insajin/autopus-adk.git'; contains "$prep_local_lib" 'verify_evidence active'
contains "$prep_lib" '/usr/bin/sudo -n -u nobody /usr/bin/env -i'
contains "$prep_lib" 'production canary started (40 sequential provider calls)'
contains "$prep_lib" 'error_code=%s error_stage=%s failed_sequence=%s'; contains "$prep_lib" 'select(.type? == "error")'; contains "$prep_lib" 'return "$canary_status"'
contains "$prep_lib" '/usr/sbin/chown -R nobody:nobody "$isolated_project"'
not_contains "$prep_lib" '"${nobody_uid}:${nobody_gid}"'
contains "$prep_lib" '/usr/bin/install -m 0440 -o root -g nobody'
contains "$prep_lib" 'IFS= read -r credential <"$credential_file"'
not_contains "$prep_lib" '"$credential_locator=${!credential_locator}"'
contains "$prep_lib" '/usr/bin/install -m 0555 -o root -g wheel "$candidate"'
contains "$prep" 'assert_source_identity'
contains "$prep" 'expected_tag_signer_fingerprint'
contains "$prep" 'sudo_keepalive_pid=$!'; contains "$prep_lib" '/usr/bin/sudo -k'
contains "$prep" 'omp-context-promotion-key-id'; contains "$prep" 'staged_promotion_signing_key='; contains "$prep" 'evidence_verification_mode=active'; contains "$prep" '/usr/bin/install -m 0600 "$tag_signing_key"'; contains "$prep" '^AUTOPUS_OMP_CONTEXT_PROVIDER_[A-Z0-9_]{1,96}$'; trap_line=$(grep -nF 'trap bootstrap_cleanup EXIT' "$prep" | cut -d: -f1); key_stage_line=$(grep -nF 'staged_tag_signing_key=' "$prep" | cut -d: -f1); [[ "$trap_line" -lt "$key_stage_line" ]] || fail 'release prep stages keys before installing cleanup trap'
contains "$prep_local_lib" 'ensure_prep_lock "$report"'; not_contains "$prep_local_lib" "--no-tags --depth=1 'https://github.com/Insajin/autopus-adk.git'"; contains "$prep_local_lib" 'local report=$1 attestation publish_root evidence_index published_tree published_commit'; contains "$prep_local_lib" 'published_tree=$(GIT_INDEX_FILE="$evidence_index"'; contains "$prep_local_lib" 'commit-tree "$published_tree"'; not_contains "$prep_local_lib" 'evidence_index evidence_tree published_commit'
contains "$prep_lib" 'publish-release-coordinates.sh'
contains "$publisher" 'gh variable set "${names[$index]}" --repo "$repository"'
contains "$publisher" 'gh variable set "${names[$index]}" --repo "$repository" --env "$environment_name"'
contains "$publisher" 'git tag -s "$release_tag"'
contains "$publisher" 'git push --atomic --force-with-lease='
contains "$publisher_lib" 'gh variable list "$@" --json name,value'; not_contains "$publisher_lib" 'gh variable list "$@" --limit'
contains "$publisher_lib" 'retry verify_owned_draft_reservation'
contains "$publisher" 'policy_creation_ambiguous=1'
temp=$(mktemp -d "${TMPDIR:-/tmp}/release-prep-hardening.XXXXXX")
cleanup() { rm -rf -- "$temp"; }
trap cleanup EXIT
ssh-keygen -q -t ed25519 -N '' -f "$temp/signing-key"
printf 'release-test %s\n' "$(<"$temp/signing-key.pub")" >"$temp/allowed-signers"
git_env=(
  GIT_CONFIG_COUNT=5
  GIT_CONFIG_KEY_0=gpg.format GIT_CONFIG_VALUE_0=ssh
  GIT_CONFIG_KEY_1=user.signingkey GIT_CONFIG_VALUE_1="$temp/signing-key"
  GIT_CONFIG_KEY_2=gpg.ssh.allowedSignersFile GIT_CONFIG_VALUE_2="$temp/allowed-signers"
  GIT_CONFIG_KEY_3=user.name GIT_CONFIG_VALUE_3='Release Test'
  GIT_CONFIG_KEY_4=user.email GIT_CONFIG_VALUE_4='release-test@example.invalid'
)
setup_fixture() {
  local name=$1
  fixture="$temp/$name"; state="$fixture/state"; work="$fixture/work"; remote="$fixture/remote.git"
  mkdir -p "$state" "$fixture/bin"
  cp "$mock_gh" "$fixture/bin/gh"; chmod 0700 "$fixture/bin/gh"
  printf '%s\n' '[{"name":"UNRELATED","value":"repository-before"}]' |
    jq . >"$state/repository-variables.json"
  printf '%s\n' '[{"name":"UNRELATED","value":"environment-before"}]' |
    jq . >"$state/environment-variables.json"
  printf '%s\n' '[]' | jq . >"$state/deployment-policies.json"
  printf '%s\n' '[]' | jq . >"$state/releases.json"
  printf '0\n' >"$state/write-count"; : >"$state/calls.log"
  git init --quiet --bare "$remote"
  git init --quiet "$work"
  printf 'release source\n' >"$work/source.txt"
  mkdir -p "$work/scripts/companion-release"
  cp "$temp/signing-key.pub" "$work/scripts/companion-release/release-tag-signing-2026-q3.pub"
  cp "$publisher_lib" "$work/scripts/companion-release/release-coordinate-transaction-lib.sh"
  ssh-keygen -lf "$temp/signing-key.pub" -E sha256 | awk '{print $2}' \
    >"$work/scripts/companion-release/release-tag-signing-2026-q3.fingerprint"
  env "${git_env[@]}" git -C "$work" add source.txt scripts/companion-release
  env "${git_env[@]}" git -C "$work" commit --quiet -m source
  git -C "$work" branch -M main
  git -C "$work" remote add origin "$remote"
  git -C "$work" push --quiet -u origin main
  source_commit=$(git -C "$work" rev-parse HEAD)
  source_tree=$(git -C "$work" rev-parse 'HEAD^{tree}')
  printf '%s\n' 'exit 99' >"$work/scripts/companion-release/release-coordinate-transaction-lib.sh"
  report_blob=$(printf 'report\n' | git -C "$work" hash-object -w --stdin)
  prep_tree=$(printf '100644 blob %s\tomp-context-promotion-report.v1.json\n' \
    "$report_blob" | git -C "$work" mktree)
  prep_lock_commit=$(printf 'prep lock\n' | env "${git_env[@]}" git -C "$work" commit-tree "$prep_tree")
  attestation_blob=$(printf 'attestation\n' | git -C "$work" hash-object -w --stdin)
  evidence_tree=$(printf '100644 blob %s\tomp-context-promotion-report.v1.json\n100644 blob %s\tomp-context-promotion-attestation.v2.json\n' \
    "$report_blob" "$attestation_blob" | git -C "$work" mktree)
  evidence_commit=$(printf 'evidence\n' | env "${git_env[@]}" git -C "$work" commit-tree "$evidence_tree")
  env "${git_env[@]}" git -C "$work" tag -s omp-context-evidence-v0.50.100 \
    "$evidence_commit" -m evidence
  evidence_tag_object=$(git -C "$work" rev-parse refs/tags/omp-context-evidence-v0.50.100)
  git -C "$work" push --quiet origin refs/tags/omp-context-evidence-v0.50.100
  git -C "$work" push --quiet origin "$prep_lock_commit:refs/heads/omp-context-evidence-v0.50.100-source"; git -C "$work" tag -d omp-context-evidence-v0.50.100 >/dev/null; git -C "$work" reflog expire --expire=now --all; git -C "$work" gc --prune=now; if git -C "$work" cat-file -e "$evidence_tag_object" 2>/dev/null; then fail 'fixture retained local evidence tag object'; fi
  printf 'report\n' >"$fixture/report"; chmod 0600 "$fixture/report"
  printf 'abc\n' >"$fixture/static-policy.b64"; chmod 0600 "$fixture/static-policy.b64"
  mkdir -p "$remote/hooks"
  cat >"$remote/hooks/pre-receive" <<'HOOK'
#!/usr/bin/env bash
set -euo pipefail
while read -r _ _ ref; do
  if [[ "$ref" == 'refs/tags/v0.50.100' ]]; then
    printf '%s\n' 'release-tag-push' >>"$MOCK_RELEASE_PREP_STATE/calls.log"
    if [[ "${MOCK_RELEASE_PREP_REJECT_TAG:-0}" -eq 1 ]]; then
      printf '%s\n' 'injected release tag rejection' >&2
      exit 1
    fi
  fi
done
HOOK
  chmod 0700 "$remote/hooks/pre-receive"
}
run_publisher() {
  local lock_argument=${RELEASE_PREP_LOCK_ARGUMENT:-fresh:$prep_lock_commit}
  env "${git_env[@]}" PATH="$fixture/bin:$PATH" MOCK_RELEASE_PREP_STATE="$state" \
    MOCK_RELEASE_PREP_FAIL_AT="${MOCK_RELEASE_PREP_FAIL_AT:-}" \
    MOCK_RELEASE_PREP_FAIL_FROM="${MOCK_RELEASE_PREP_FAIL_FROM:-}" \
    MOCK_RELEASE_PREP_POLICY_RESPONSE_LOST="${MOCK_RELEASE_PREP_POLICY_RESPONSE_LOST:-0}" \
    MOCK_RELEASE_PREP_RELEASE_RESPONSE_LOST="${MOCK_RELEASE_PREP_RELEASE_RESPONSE_LOST:-0}" \
    MOCK_RELEASE_PREP_RELEASE_VISIBILITY_DELAY="${MOCK_RELEASE_PREP_RELEASE_VISIBILITY_DELAY:-0}" \
    MOCK_RELEASE_PREP_RELEASE_DELETE_FAIL="${MOCK_RELEASE_PREP_RELEASE_DELETE_FAIL:-0}" \
    MOCK_RELEASE_PREP_REJECT_TAG="${MOCK_RELEASE_PREP_REJECT_TAG:-0}" \
    bash "$publisher" Insajin/autopus-adk adk-companion-release v0.50.100 \
    "$source_commit" "$source_tree" "$fixture/static-policy.b64" \
    "$evidence_tag_object" "$evidence_commit" "$evidence_tree" \
    "$(printf 'a%.0s' {1..64})" "$(printf 'b%.0s' {1..64})" "$lock_argument" \
    "$temp/signing-key"
}
setup_fixture success
[[ "$(cd "$work" && "$lock_verifier" \
  refs/heads/omp-context-evidence-v0.50.100-source "$prep_lock_commit" "$fixture/report")" == \
  "$prep_lock_commit" ]] || fail 'retained prep lock verifier did not adopt the exact lock'
printf 'tampered\n' >"$fixture/report"
if (cd "$work" && "$lock_verifier" \
  refs/heads/omp-context-evidence-v0.50.100-source "$prep_lock_commit" "$fixture/report" \
  >/dev/null 2>&1); then
  fail 'retained prep lock verifier accepted different report bytes'
fi
printf 'report\n' >"$fixture/report"
(cd "$work" && run_publisher >"$fixture/result.json")
jq -e '.mode == "committed" and .release_tag == "v0.50.100" and .source_commit == $source and .github_release_id == 996' --arg source "$source_commit" "$fixture/result.json" >/dev/null || fail 'success receipt differs'
[[ "$(git -C "$work" ls-remote --refs origin refs/tags/v0.50.100 | cut -f2)" == 'refs/tags/v0.50.100' ]] || fail 'success did not publish release tag'
[[ "$(jq 'length' "$state/repository-variables.json")" == '9' &&
   "$(jq 'length' "$state/environment-variables.json")" == '9' ]] || fail 'success did not converge both variable scopes'
jq -e 'any(.[]; .type == "tag" and .name == "v0.50.100")' "$state/deployment-policies.json" >/dev/null || fail 'success did not install exact tag deployment policy'
jq -e 'length == 1 and .[0].id == 996 and .[0].draft == true and .[0].author.id == 204883817 and
  (.[0].body | fromjson | .schema_version == "autopus.adk_release_reservation.v1")' "$state/releases.json" >/dev/null || fail 'success did not retain the owned draft reservation'
[[ "$(sed -n '$p' "$state/calls.log")" == 'release-tag-push' ]] || fail 'release tag was not the final external mutation'
[[ -z "$(git -C "$work" ls-remote --refs origin \
  refs/heads/omp-context-evidence-v0.50.100-source)" ]] || fail 'success did not atomically retire the prep lock'
calls_before_reconcile=$(wc -l <"$state/calls.log" | tr -d ' ')
(cd "$work" && RELEASE_PREP_LOCK_ARGUMENT=reconcile run_publisher >"$fixture/reconciled.json")
jq -e '.mode == "reconciled" and .release_tag == "v0.50.100"' "$fixture/reconciled.json" >/dev/null || fail 'committed release did not reconcile'
[[ "$(wc -l <"$state/calls.log" | tr -d ' ')" == "$calls_before_reconcile" ]] || fail 'reconciliation performed a remote mutation'

setup_fixture rollback
cp "$state/repository-variables.json" "$fixture/repository-before.json"
cp "$state/environment-variables.json" "$fixture/environment-before.json"
if (cd "$work" && MOCK_RELEASE_PREP_FAIL_AT=5 run_publisher >/dev/null 2>&1); then
  fail 'injected coordinate write failure succeeded'
fi
cmp "$fixture/repository-before.json" "$state/repository-variables.json" ||
  fail 'repository variables were not rolled back'
cmp "$fixture/environment-before.json" "$state/environment-variables.json" ||
  fail 'environment variables were not rolled back'
[[ "$(jq 'length' "$state/deployment-policies.json")" == '0' ]] ||
  fail 'failed publish left a deployment policy'
[[ -z "$(git -C "$work" ls-remote --refs origin refs/tags/v0.50.100)" ]] ||
  fail 'failed publish created release tag'
[[ -z "$(git -C "$work" ls-remote --refs origin \
  refs/heads/omp-context-evidence-v0.50.100-source)" ]] ||
  fail 'successful rollback retained the prep lock'

setup_fixture rollback_incomplete
if (cd "$work" && MOCK_RELEASE_PREP_FAIL_AT=5 MOCK_RELEASE_PREP_FAIL_FROM=6 \
  run_publisher >/dev/null 2>&1); then
  fail 'persistent rollback failure succeeded'
fi
[[ "$(git -C "$work" ls-remote --refs origin \
  refs/heads/omp-context-evidence-v0.50.100-source | cut -f1)" == "$prep_lock_commit" ]] ||
  fail 'incomplete rollback did not retain its owned prep lock'
[[ -z "$(git -C "$work" ls-remote --refs origin refs/tags/v0.50.100)" ]] ||
  fail 'incomplete rollback created a release tag'

setup_fixture policy_response_lost
if (cd "$work" && MOCK_RELEASE_PREP_POLICY_RESPONSE_LOST=1 \
  run_publisher >/dev/null 2>&1); then
  fail 'lost deployment-policy response succeeded'
fi
[[ "$(git -C "$work" ls-remote --refs origin \
  refs/heads/omp-context-evidence-v0.50.100-source | cut -f1)" == "$prep_lock_commit" ]] ||
  fail 'ambiguous policy creation did not retain its owned prep lock'
jq -e 'any(.[]; .type == "tag" and .name == "v0.50.100")' \
  "$state/deployment-policies.json" >/dev/null ||
  fail 'lost policy response fixture did not create the remote policy'

setup_fixture release_response_lost
(cd "$work" && MOCK_RELEASE_PREP_RELEASE_RESPONSE_LOST=1 \
  run_publisher >"$fixture/result.json")
jq -e '.mode == "committed" and .github_release_id == 996' \
  "$fixture/result.json" >/dev/null ||
  fail 'lost release-create response did not reconcile the owned reservation'
[[ "$(jq 'length' "$state/releases.json")" == '1' ]] ||
  fail 'lost release-create response duplicated the reservation'
[[ -z "$(git -C "$work" ls-remote --refs origin \
  refs/heads/omp-context-evidence-v0.50.100-source)" ]] ||
  fail 'lost release-create response retained the prep lock after commit'

setup_fixture release_eventual_consistency
(cd "$work" && MOCK_RELEASE_PREP_RELEASE_VISIBILITY_DELAY=2 run_publisher >"$fixture/result.json")
jq -e '.mode == "committed" and .github_release_id == 996' "$fixture/result.json" >/dev/null || fail 'eventually consistent release reservation did not commit'
[[ "$(grep -c '^release-reservation-not-yet-visible$' "$state/calls.log")" == '2' ]] || fail 'release reservation verification did not retry delayed visibility'
[[ "$(jq 'length' "$state/releases.json")" == '1' ]] || fail 'eventual-consistency retry duplicated the release reservation'
[[ -z "$(git -C "$work" ls-remote --refs origin \
  refs/heads/omp-context-evidence-v0.50.100-source)" ]] || fail 'successful eventual-consistency retry retained the prep lock'

setup_fixture release_push_rejected
if (cd "$work" && MOCK_RELEASE_PREP_REJECT_TAG=1 \
  run_publisher >/dev/null 2>&1); then
  fail 'rejected release tag push succeeded'
fi
[[ "$(jq 'length' "$state/releases.json")" == '0' ]] ||
  fail 'rejected tag push did not delete its owned draft reservation'
[[ -z "$(git -C "$work" ls-remote --refs origin \
  refs/heads/omp-context-evidence-v0.50.100-source)" ]] ||
  fail 'rejected tag push retained its lock after complete rollback'

setup_fixture release_delete_failed
delete_failure_status=0
(cd "$work" && MOCK_RELEASE_PREP_REJECT_TAG=1 MOCK_RELEASE_PREP_RELEASE_DELETE_FAIL=1 \
  run_publisher >/dev/null 2>&1) || delete_failure_status=$?
[[ "$delete_failure_status" -eq 75 ]] ||
  fail 'draft deletion failure did not return retained-transaction status'
[[ "$(jq 'length' "$state/releases.json")" == '1' ]] ||
  fail 'draft deletion failure did not retain its reservation'
[[ "$(git -C "$work" ls-remote --refs origin \
  refs/heads/omp-context-evidence-v0.50.100-source | cut -f1)" == "$prep_lock_commit" ]] ||
  fail 'draft deletion failure did not retain its prep lock'
(cd "$work" && RELEASE_PREP_LOCK_ARGUMENT="retained:$prep_lock_commit" \
  run_publisher >"$fixture/retry-result.json")
jq -e '.mode == "committed" and .github_release_id == 996' \
  "$fixture/retry-result.json" >/dev/null ||
  fail 'retained retry did not adopt the exact draft reservation'

setup_fixture release_name_collision
jq -n --arg source "$source_commit" \
  '[{id:995,tag_name:"v0.50.95-collision",target_commitish:$source,name:"v0.50.100",body:"collision",draft:true,prerelease:false,author:{id:204883817},assets:[]}]' \
  >"$state/releases.json"
if (cd "$work" && run_publisher >/dev/null 2>&1); then
  fail 'publisher accepted a GoReleaser draft-name collision'
fi
[[ "$(git -C "$work" ls-remote --refs origin \
  refs/heads/omp-context-evidence-v0.50.100-source | cut -f1)" == "$prep_lock_commit" ]] ||
  fail 'draft-name collision did not retain the prep lock'
[[ "$(jq 'length' "$state/repository-variables.json")" == '1' &&
   "$(jq 'length' "$state/environment-variables.json")" == '1' ]] ||
  fail 'draft-name collision mutated release coordinates'

setup_fixture release_ownership_mismatch
jq -n --arg source "$source_commit" \
  '[{id:997,tag_name:"v0.50.100",target_commitish:$source,name:"v0.50.100",body:"unowned",draft:true,prerelease:false,author:{id:42},assets:[]}]' \
  >"$state/releases.json"
if (cd "$work" && run_publisher >/dev/null 2>&1); then
  fail 'publisher adopted an unowned draft release'
fi
[[ "$(git -C "$work" ls-remote --refs origin \
  refs/heads/omp-context-evidence-v0.50.100-source | cut -f1)" == "$prep_lock_commit" ]] ||
  fail 'unowned draft did not retain the prep lock for operator recovery'
[[ "$(jq 'length' "$state/repository-variables.json")" == '1' &&
   "$(jq 'length' "$state/environment-variables.json")" == '1' ]] ||
  fail 'unowned draft mutated release coordinates'

setup_fixture lock_missing
git -C "$work" push --quiet origin :refs/heads/omp-context-evidence-v0.50.100-source
cp "$state/repository-variables.json" "$fixture/repository-before.json"
cp "$state/environment-variables.json" "$fixture/environment-before.json"
if (cd "$work" && run_publisher >/dev/null 2>&1); then
  fail 'publisher succeeded without owning the compare-and-swap lock'
fi
cmp "$fixture/repository-before.json" "$state/repository-variables.json" ||
  fail 'lock failure mutated repository variables'
cmp "$fixture/environment-before.json" "$state/environment-variables.json" ||
  fail 'lock failure mutated environment variables'
[[ -z "$(git -C "$work" ls-remote --refs origin refs/tags/v0.50.100)" ]] ||
  fail 'lock failure created release tag'

printf 'release prep hardening test: PASS\n'
