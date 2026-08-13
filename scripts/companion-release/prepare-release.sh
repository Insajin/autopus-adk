#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() { printf 'companion release prep: %s\n' "$1" >&2; exit 1; }
usage() {
  cat >&2 <<'USAGE'
usage: prepare-release.sh --endpoint URL --credential-locator ENV --provider NAME --model NAME --model-context-window N --omp PATH --oracle-policy-digest sha256:HEX --tag-signing-key PATH --promotion-signing-key PATH [--inherit-parent-sandbox] [--apply]
USAGE
  exit 64
}
readonly repository='Insajin/autopus-adk'
readonly environment_name='adk-companion-release'
readonly release_tag='v0.50.104'
readonly spec_id='SPEC-OMP-004'
# The OMP oracle is pinned by digest AND version string (see the conjunctive
# gate below). It has not moved since v0.50.96 and must not be advanced to make
# a release run on whatever OMP happens to be installed: shipped behaviour is
# defined against this exact build.
#   - v0.50.96 tuned RPC readiness limits to this build's cold-start timing.
#   - v0.50.98 has the isolated catalog inherit this build's exact provider and
#     model capability surface.
#   - README.md and docs/README.ko.md document strict_routing_ready=false and
#     catalog_metadata_insufficient as consequences of this build's catalog.
# Advancing the pin invalidates all three and requires re-establishing them with
# evidence, not a digest edit.
readonly expected_omp_sha256='cd2f47545cb3f8eb5e15c91bc9054d73967774652e020b432e294803d1b71ea0'
readonly expected_promotion_key_id='omp-context-promotion-2026-q3-k2'
readonly release_ref="refs/tags/${release_tag}"
readonly evidence_tag="omp-context-evidence-${release_tag}"
readonly evidence_ref="refs/tags/${evidence_tag}"
readonly evidence_source_ref="refs/heads/${evidence_tag}-source"
endpoint='' credential_locator='' provider='' model='' model_context_window=''
omp_executable='' oracle_policy_digest='' tag_signing_key='' promotion_signing_key=''
apply=0 inherit_parent_sandbox=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --endpoint) [[ $# -ge 2 ]] || usage; endpoint=$2; shift 2 ;;
    --credential-locator) [[ $# -ge 2 ]] || usage; credential_locator=$2; shift 2 ;;
    --provider) [[ $# -ge 2 ]] || usage; provider=$2; shift 2 ;;
    --model) [[ $# -ge 2 ]] || usage; model=$2; shift 2 ;;
    --model-context-window) [[ $# -ge 2 ]] || usage; model_context_window=$2; shift 2 ;;
    --omp) [[ $# -ge 2 ]] || usage; omp_executable=$2; shift 2 ;;
    --oracle-policy-digest) [[ $# -ge 2 ]] || usage; oracle_policy_digest=$2; shift 2 ;;
    --tag-signing-key) [[ $# -ge 2 ]] || usage; tag_signing_key=$2; shift 2 ;;
    --promotion-signing-key) [[ $# -ge 2 ]] || usage; promotion_signing_key=$2; shift 2 ;;
    --inherit-parent-sandbox) inherit_parent_sandbox=1; shift ;;
    --apply) apply=1; shift ;;
    *) usage ;;
  esac
done

[[ "$(uname -s)" == 'Darwin' && "$(uname -m)" == 'arm64' ]] || fail 'release prep requires Darwin arm64'
[[ "$endpoint" =~ ^http://127\.0\.0\.1:[1-9][0-9]{0,4}$ ]] || fail 'endpoint is not exact loopback HTTP'
[[ "$credential_locator" =~ ^AUTOPUS_OMP_CONTEXT_PROVIDER_[A-Z0-9_]{1,96}$ ]] || fail 'credential locator is malformed'
[[ -n "${!credential_locator-}" && "${!credential_locator}" != *$'\n'* ]] ||
  fail 'provider credential is unavailable or not single-line'
[[ "$provider" =~ ^[A-Za-z0-9_.-]+$ && "$model" =~ ^[A-Za-z0-9_.:/-]+$ ]] || fail 'provider or model is malformed'
[[ "$model_context_window" =~ ^[0-9]+$ && "$model_context_window" -ge 8192 ]] || fail 'model context window is invalid'
[[ "$oracle_policy_digest" =~ ^sha256:[0-9a-f]{64}$ ]] || fail 'oracle policy digest is malformed'
for tool in awk cmp cp gh git go install jq mktemp sed shasum ssh-keygen sudo tar tr wc; do
  command -v "$tool" >/dev/null || fail "$tool is unavailable"
done
readonly runner_uid=$(/usr/bin/id -u)
readonly runner_gid=$(/usr/bin/id -g)
readonly nobody_uid=$(/usr/bin/id -u nobody)
[[ "$runner_uid" != "$nobody_uid" ]] || fail 'release operator and canary identities must differ'
/usr/bin/sudo -n -u nobody /usr/bin/true || fail 'passwordless nobody execution is unavailable'
readonly private_tmp_identity=$(/usr/bin/stat -f '%d:%i' /private/tmp)
readonly operator_actor_id=204883817

[[ -f "$omp_executable" && ! -L "$omp_executable" && -x "$omp_executable" ]] || fail 'OMP executable is unsafe'
omp_executable=$(cd -- "$(dirname -- "$omp_executable")" && pwd)/$(basename -- "$omp_executable")
[[ -f "$tag_signing_key" && ! -L "$tag_signing_key" ]] || fail 'release tag signing key is unsafe'
tag_signing_key=$(cd -- "$(dirname -- "$tag_signing_key")" && pwd)/$(basename -- "$tag_signing_key")
[[ "$(/usr/bin/stat -f '%u:%Lp' "$tag_signing_key")" == "$(id -u):600" ]] || fail 'release tag signing key ownership or mode is unsafe'
[[ -f "$promotion_signing_key" && ! -L "$promotion_signing_key" ]] || fail 'promotion signing key is unsafe'
promotion_signing_key=$(cd -- "$(dirname -- "$promotion_signing_key")" && pwd)/$(basename -- "$promotion_signing_key")
[[ "$(/usr/bin/stat -f '%u:%Lp' "$promotion_signing_key")" == "$(id -u):600" ]] ||
  fail 'promotion signing key ownership or mode is unsafe'

repo_root=$(git rev-parse --show-toplevel)
[[ "$(pwd -P)" == "$repo_root" ]] || fail 'release prep must run at the repository root'
readonly repo_root
assert_source_identity() {
  [[ -z "$(git status --porcelain)" ]] || fail 'source worktree is not clean'
  [[ "$(git rev-parse --verify 'HEAD^{commit}')" == "$source_commit" ]] || fail 'source commit changed during release prep'
  [[ "$(git rev-parse --verify 'HEAD^{tree}')" == "$source_tree" ]] || fail 'source tree changed during release prep'
  [[ "$(git remote get-url origin)" =~ ^(https://github\.com/|git@github\.com:)(Insajin|insajin)/autopus-adk(\.git)?$ ]] || fail 'origin is not the production repository'
}
[[ -z "$(git status --porcelain)" ]] || fail 'source worktree is not clean'
source_commit=$(git rev-parse --verify 'HEAD^{commit}')
source_tree=$(git rev-parse --verify 'HEAD^{tree}')
[[ "$source_commit" =~ ^[0-9a-f]{40}$ && "$source_tree" =~ ^[0-9a-f]{40}$ ]] || fail 'source coordinates are malformed'
assert_source_identity
git fetch --no-tags origin main
[[ "$(git rev-parse --verify origin/main)" == "$source_commit" ]] || fail 'source is not exact origin/main'
assert_source_identity
[[ "$(gh api "repos/${repository}" --jq .default_branch)" == 'main' ]] || fail 'default branch differs'

readonly tag_public_key_file="$repo_root/scripts/companion-release/release-tag-signing-2026-q3.pub"
readonly tag_fingerprint_file="$repo_root/scripts/companion-release/release-tag-signing-2026-q3.fingerprint"
[[ -f "$tag_public_key_file" && ! -L "$tag_public_key_file" &&
   -f "$tag_fingerprint_file" && ! -L "$tag_fingerprint_file" ]] ||
  fail 'pinned release tag signer identity is unavailable'

bootstrap_cleanup() { rm -rf -- "$temp_dir"; }
readonly temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/companion-release-prep.XXXXXX")
chmod 0700 "$temp_dir"
trap bootstrap_cleanup EXIT
staged_tag_signing_key="$temp_dir/release-tag-signing-key"
staged_promotion_signing_key="$temp_dir/omp-context-promotion-signing-key"
/usr/bin/install -m 0600 "$tag_signing_key" "$staged_tag_signing_key"
/usr/bin/install -m 0400 "$promotion_signing_key" "$staged_promotion_signing_key"
[[ "$(/usr/bin/stat -f '%u:%Lp' "$staged_tag_signing_key")" == "$(id -u):600" &&
   "$(/usr/bin/stat -f '%u:%Lp' "$staged_promotion_signing_key")" == "$(id -u):400" ]] ||
  fail 'staged release signing key ownership or mode is unsafe'
tag_signing_key=$staged_tag_signing_key
promotion_signing_key=$staged_promotion_signing_key
evidence_source_commit=''
retain_prep_lock=0
prep_lock_mode='fresh'
isolation_roots=()
sudo_keepalive_pid=''
for runtime_lib_name in prepare-release-runtime-lib.sh prepare-release-local-lib.sh; do
  staged_runtime_lib="$temp_dir/$runtime_lib_name"
  runtime_lib_blob=$(git rev-parse --verify \
    "${source_commit}:scripts/companion-release/${runtime_lib_name}") ||
    fail "release prep runtime helper ${runtime_lib_name} is absent from the exact source"
  [[ "$(git cat-file -t "$runtime_lib_blob")" == 'blob' ]] ||
    fail "release prep runtime helper ${runtime_lib_name} is not a source blob"
  git cat-file blob "$runtime_lib_blob" >"$staged_runtime_lib"
  chmod 0400 "$staged_runtime_lib"
  [[ "$(git hash-object "$staged_runtime_lib")" == "$runtime_lib_blob" ]] ||
    fail "staged release prep runtime helper ${runtime_lib_name} differs from the exact source"
  exec 9<"$staged_runtime_lib"
  rm -f -- "$staged_runtime_lib"
  # shellcheck source=/dev/null
  source /dev/fd/9
  exec 9<&-
done
trap 'cleanup $?' EXIT
(
  while /bin/sleep 30; do
    /usr/bin/sudo -n -v || exit 1
  done
) &
sudo_keepalive_pid=$!

readonly staged_omp="$temp_dir/omp-v17.2.7"
cp "$omp_executable" "$staged_omp"
chmod 0500 "$staged_omp"
[[ "$(shasum -a 256 "$staged_omp" | awk '{print $1}')" == "$expected_omp_sha256" ]] || fail 'staged OMP executable digest differs'
[[ "$("$staged_omp" --version)" == 'omp/17.2.7' ]] || fail 'verified OMP version differs from v17.2.7'
omp_executable=$staged_omp

readonly derived_tag_public_key="$temp_dir/release-tag-signing.pub"
ssh-keygen -y -f "$tag_signing_key" >"$derived_tag_public_key"
chmod 0600 "$derived_tag_public_key"
expected_public_key=$(awk 'NF >= 2 { print $1 " " $2; exit }' "$tag_public_key_file")
expected_tag_signer_fingerprint=$(<"$tag_fingerprint_file")
derived_public_key=$(awk 'NF >= 2 { print $1 " " $2; exit }' "$derived_tag_public_key")
[[ -n "$expected_public_key" && "$derived_public_key" == "$expected_public_key" ]] || fail 'release tag signing key differs from pinned public key'
[[ "$expected_tag_signer_fingerprint" =~ ^SHA256:[A-Za-z0-9+/]{43}$ &&
   "$(ssh-keygen -lf "$derived_tag_public_key" -E sha256 | awk '{print $2}')" == "$expected_tag_signer_fingerprint" ]] ||
  fail 'release tag signer fingerprint differs'
readonly allowed_signers="$temp_dir/release-tag.allowed-signers"
printf 'autopus-adk-release-tag %s\n' "$derived_public_key" >"$allowed_signers"
chmod 0600 "$allowed_signers"
readonly signing_probe="$temp_dir/signing-probe"
git clone --quiet --no-checkout --shared . "$signing_probe"
tag_git_config=(
  GIT_CONFIG_COUNT=5
  GIT_CONFIG_KEY_0=gpg.format GIT_CONFIG_VALUE_0=ssh
  GIT_CONFIG_KEY_1=user.signingkey GIT_CONFIG_VALUE_1="$tag_signing_key"
  GIT_CONFIG_KEY_2=gpg.ssh.allowedSignersFile GIT_CONFIG_VALUE_2="$allowed_signers"
  GIT_CONFIG_KEY_3=user.name GIT_CONFIG_VALUE_3='Joseph'
  GIT_CONFIG_KEY_4=user.email GIT_CONFIG_VALUE_4='joseph@Josephui-MacBookPro.local'
)
env "${tag_git_config[@]}" git -C "$signing_probe" tag -s release-signing-probe HEAD -m 'release signing probe'
env "${tag_git_config[@]}" git -C "$signing_probe" verify-tag refs/tags/release-signing-probe >/dev/null

environment_variables=$(gh variable list --repo "$repository" --env "$environment_name" --json name,value) ||
  fail 'protected environment variables are unavailable'
jq -e 'type == "array" and all(.[]; (.name | type) == "string" and (.value | type) == "string") and (([.[].name] | length) == ([.[].name] | unique | length))' \
  <<<"$environment_variables" >/dev/null || fail 'protected environment variable inventory is malformed'
readonly environment_variables
lineage_key_id=$(matched_variable ADK_COMPANION_KEY_ID)
lineage_handoff=$(matched_variable ADK_COMPANION_HANDOFF)
rollback_floor=$(matched_variable ADK_COMPANION_ROLLBACK_FLOOR)
[[ "$lineage_key_id" =~ ^[A-Za-z0-9_.-]+$ && "$lineage_handoff" =~ ^[A-Za-z0-9_.-]+$ && "$rollback_floor" =~ ^[1-9][0-9]*$ ]] || fail 'release lineage policy is unavailable'
github_release_state() {
  local releases matches
  releases=$(gh api --paginate --slurp \
    "repos/Insajin/autopus-adk/releases?per_page=100") || return 1
  jq -e 'type == "array" and all(.[]; type == "array")' <<<"$releases" >/dev/null ||
    return 1
  matches=$(jq '[.[][] | select(.tag_name == "v0.50.104")] | length' <<<"$releases") ||
    return 1
  case "$matches" in
    0) printf 'absent\n' ;;
    1) printf 'present\n' ;;
    *) return 1 ;;
  esac
}
authenticated_actor_id=$(gh api user --jq .id) || fail 'cannot authenticate release operator'
[[ "$authenticated_actor_id" == "$operator_actor_id" ]] ||
  fail 'authenticated GitHub actor is not the release operator'

release_remote=$(git ls-remote --refs origin "$release_ref") || fail 'cannot inspect release ref'
evidence_remote=$(git ls-remote --refs origin "$evidence_ref") || fail 'cannot inspect evidence ref'
lock_remote=$(git ls-remote --refs origin "$evidence_source_ref") || fail 'cannot inspect prep lock ref'
retained_lock_commit=''
if [[ -n "$lock_remote" ]]; then
  retained_lock_commit=${lock_remote%%$'\t'*}
  [[ "$retained_lock_commit" =~ ^[0-9a-f]{40}$ &&
     "$lock_remote" == "$retained_lock_commit"$'\t'"$evidence_source_ref" ]] ||
    fail 'release-prep compare-and-swap lock is malformed'
fi
[[ -z "$release_remote" || -n "$evidence_remote" ]] || fail 'release tag exists without immutable evidence'
release_present=0 evidence_present=0
[[ -n "$release_remote" ]] && release_present=1
[[ -n "$evidence_remote" ]] && evidence_present=1
release_state=$(github_release_state) || fail 'cannot inspect GitHub Release state'
release_exists=0
[[ "$release_state" == 'present' ]] && release_exists=1
[[ "$release_exists" -eq 0 || "$release_present" -eq 1 || -n "$retained_lock_commit" ]] ||
  fail 'GitHub Release exists without its source tag or retained prep lock'
if [[ "$apply" -eq 0 ]]; then
  jq -cn --arg release_tag "$release_tag" --arg source_commit "$source_commit" --arg source_tree "$source_tree" \
    --argjson evidence_present "$evidence_present" --argjson release_present "$release_present" \
    --argjson release_exists "$release_exists" --argjson prep_lock_present "$([[ -n "$retained_lock_commit" ]] && printf 1 || printf 0)" \
    '{mode:"preflight",release_tag:$release_tag,source_commit:$source_commit,source_tree:$source_tree,evidence_present:($evidence_present == 1),release_tag_present:($release_present == 1),github_release_present:($release_exists == 1),prep_lock_present:($prep_lock_present == 1),remote_mutations:0}'
  exit 0
fi

readonly input_jsonl="$temp_dir/observe-session-input.jsonl"
readonly policy_tool="$temp_dir/auto-policy"
readonly verifier="$temp_dir/ompcontextverify"
readonly execsmoke="$temp_dir/execsmoke"
readonly final_candidate="$temp_dir/auto-final"
readonly static_policy_file="$temp_dir/static-policy.b64"
readonly final_static_policy_file="$temp_dir/final-static-policy.b64"
producer_run_id=$(date -u '+%Y%m%d%H%M%S')
producer_workflow_ref="local-release-prep@${source_commit}"
dispatch_nonce=$(printf '%s' "${source_commit}:${producer_run_id}:$$:${temp_dir}" | shasum -a 256 | awk '{print substr($1,1,32)}')
[[ "$dispatch_nonce" =~ ^[0-9a-f]{32}$ ]] || fail 'dispatch nonce is malformed'
challenge_digest="sha256:$(printf '%s' "${release_tag}:${source_commit}:${source_tree}" | shasum -a 256 | awk '{print $1}')"

go mod tidy -diff
go build -trimpath -o "$policy_tool" ./cmd/auto
go build -trimpath -o "$verifier" ./scripts/companion-release/ompcontextverify
go build -trimpath -o "$execsmoke" ./scripts/companion-release/execsmoke
[[ "$("$policy_tool" companion-manifest omp-context-promotion-key-id <"$promotion_signing_key")" == \
   "$expected_promotion_key_id" ]] || fail 'promotion signing key differs from pinned local release key'

if [[ "$evidence_present" -eq 1 ]]; then
  load_evidence
  derive_policy "$verified_report" "$static_policy_file"
  IFS= read -r static_policy_b64 <"$static_policy_file"
  build_candidate "$static_policy_b64" "$final_candidate"
  candidate_sha256=$(shasum -a 256 "$final_candidate" | awk '{print $1}')
  evidence_verification_mode=active
  if [[ "$release_present" -eq 1 ]]; then evidence_verification_mode=historical; fi
  verify_evidence "$evidence_verification_mode"
  if [[ "$release_present" -eq 1 ]]; then
    publish_coordinates reconcile
    exit 0
  fi
  ensure_prep_lock "$verified_report"
  publish_coordinates "$evidence_source_commit"
  exit 0
fi

create_canary_plan "$static_policy_file"
IFS= read -r static_policy_b64 <"$static_policy_file"
build_candidate "$static_policy_b64" "$final_candidate"
final_project="$temp_dir/final-project"; final_output="$temp_dir/final-output.jsonl"
extract_project "$final_project"
run_canary "$final_candidate" "$final_project" "$final_output" final
validate_canary "$final_project" "$final_output" "$final_candidate"
readonly final_report="$final_project/.autopus/runtime/omp-context/promotion-report-v1.json"
derive_policy "$final_report" "$final_static_policy_file"
cmp "$static_policy_file" "$final_static_policy_file" || fail 'planned static policy changed after final canary'
candidate_sha256=$(shasum -a 256 "$final_candidate" | awk '{print $1}')

git fetch --no-tags origin main
[[ "$(git rev-parse --verify origin/main)" == "$source_commit" ]] || fail 'origin/main advanced during release prep'
assert_source_identity
publish_local_evidence "$final_report"
publish_coordinates "$evidence_source_commit"
