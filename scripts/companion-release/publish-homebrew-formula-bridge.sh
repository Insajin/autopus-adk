#!/usr/bin/env bash
set -euo pipefail
umask 077

readonly RELEASE_TAG='v0.50.80'
readonly RELEASE_VERSION='0.50.80'
readonly RELEASE_POLICY='cask-only'
readonly TAP_REPOSITORY='Insajin/homebrew-autopus'
readonly TAP_BRANCH='main'
readonly PRIOR_TAP_COMMIT='2838951580d16348e12be39c09553cf6765504cb'
readonly CASK_PATH='Casks/auto.rb'
readonly PRIOR_CASK_BLOB='979e62e34124b9f3c68bb2b8e1d0163047ea3ee3'
readonly FORMULA_PATH='Formula/auto.rb'
readonly FROZEN_FORMULA_BLOB='4ebc6c38925002dec00759823d4dd847a499818a'

fail() {
  printf 'homebrew cask publication: %s\n' "$1" >&2
  exit 1
}

require_environment() {
  local name="$1"
  [[ -n "${!name-}" ]] || fail "required environment variable ${name} is missing"
}

for name in \
  GITHUB_REF_NAME COMPANION_VERSION COMPANION_HOMEBREW_POLICY COMPANION_CHECKSUMS_PATH
do
  require_environment "$name"
done
[[ "$GITHUB_REF_NAME" == "$RELEASE_TAG" ]] \
  || fail 'release tag is outside the exact Cask publication policy'
[[ "$COMPANION_VERSION" == "$RELEASE_VERSION" ]] \
  || fail 'release version is outside the exact Cask publication policy'
[[ "$COMPANION_HOMEBREW_POLICY" == "$RELEASE_POLICY" ]] \
  || fail 'release policy is not exact cask-only publication'

tap_token="${HOMEBREW_TAP_TOKEN-}"
if [[ -z "$tap_token" ]]; then
  tap_token="${GH_TOKEN-}"
fi
[[ -n "$tap_token" ]] \
  || fail 'HOMEBREW_TAP_TOKEN or GH_TOKEN is required for the tap API'

for tool in gh jq base64 mktemp cmp tr; do
  command -v "$tool" >/dev/null 2>&1 || fail "required tool ${tool} is unavailable"
done
if command -v shasum >/dev/null 2>&1; then
  sha256_command=(shasum -a 256)
elif command -v sha256sum >/dev/null 2>&1; then
  sha256_command=(sha256sum)
else
  fail 'required SHA-256 tool shasum or sha256sum is unavailable'
fi
if printf '' | base64 --decode >/dev/null 2>&1; then
  base64_decode=(base64 --decode)
elif printf '' | base64 -D >/dev/null 2>&1; then
  base64_decode=(base64 -D)
else
  fail 'base64 decoder is unavailable'
fi

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd) \
  || fail 'cannot resolve Formula bridge script directory'
render_helper="$script_dir/publish-homebrew-formula-bridge-render.sh"
[[ -f "$render_helper" && ! -L "$render_helper" ]] \
  || fail 'Formula bridge renderer is not a regular non-symlink file'
# shellcheck source=publish-homebrew-formula-bridge-render.sh
source "$render_helper"
declare -F render_homebrew_cask >/dev/null \
  || fail 'Cask renderer contract is incomplete'

checksums_path="$COMPANION_CHECKSUMS_PATH"
[[ -f "$checksums_path" && ! -L "$checksums_path" ]] \
  || fail 'checksums input is not a regular file'

darwin_amd64_name="autopus-adk_${RELEASE_VERSION}_darwin_amd64.tar.gz"
darwin_arm64_name="autopus-adk_${RELEASE_VERSION}_darwin_arm64.tar.gz"
linux_amd64_name="autopus-adk_${RELEASE_VERSION}_linux_amd64.tar.gz"
linux_arm64_name="autopus-adk_${RELEASE_VERSION}_linux_arm64.tar.gz"
darwin_amd64_sha='' darwin_arm64_sha='' linux_amd64_sha='' linux_arm64_sha=''
checksum_pattern='^([0-9a-f]{64})  ([A-Za-z0-9][A-Za-z0-9._+-]*)$'
seen_names=()
line_count=0
while IFS= read -r line || [[ -n "$line" ]]; do
  line_count=$((line_count + 1))
  [[ "$line" =~ $checksum_pattern ]] \
    || fail "checksums input has a malformed entry at line ${line_count}"
  digest="${BASH_REMATCH[1]}"
  archive="${BASH_REMATCH[2]}"
  for seen_name in "${seen_names[@]-}"; do
    [[ "$archive" != "$seen_name" ]] \
      || fail "checksums input has a duplicate archive at line ${line_count}"
  done
  seen_names+=("$archive")
  case "$archive" in
    "$darwin_amd64_name") darwin_amd64_sha="$digest" ;;
    "$darwin_arm64_name") darwin_arm64_sha="$digest" ;;
    "$linux_amd64_name") linux_amd64_sha="$digest" ;;
    "$linux_arm64_name") linux_arm64_sha="$digest" ;;
    *'_darwin_'*|*'_linux_'*)
      fail "checksums input has an unexpected Darwin or Linux archive at line ${line_count}"
      ;;
  esac
done <"$checksums_path"
for digest in \
  "$darwin_amd64_sha" "$darwin_arm64_sha" "$linux_amd64_sha" "$linux_arm64_sha"
do
  [[ "$digest" =~ ^[0-9a-f]{64}$ ]] \
    || fail 'checksums input is missing an exact bridge archive'
done

temp_dir=''
cleanup() {
  local status=$?
  if [[ -n "$temp_dir" ]]; then
    rm -rf -- "$temp_dir" || status=$?
  fi
  return "$status"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/autopus-homebrew-bridge.XXXXXX") \
  || fail 'cannot create private temporary directory'

api_get() {
  local path="$1" output="$2"
  if ! GH_TOKEN="$tap_token" gh api \
    -H 'Accept: application/vnd.github+json' \
    "repos/${TAP_REPOSITORY}/contents/${path}?ref=${TAP_BRANCH}" \
    >"$output" 2>"$temp_dir/gh-error"; then
    fail "cannot read ${path} from the Homebrew tap"
  fi
}

api_json() {
  local method="$1" endpoint="$2" input="$3" output="$4" label="$5"
  local -a args=(api --method "$method" -H 'Accept: application/vnd.github+json'
    "repos/${TAP_REPOSITORY}/${endpoint}")
  [[ -z "$input" ]] || args+=(--input "$input")
  if ! GH_TOKEN="$tap_token" gh "${args[@]}" >"$output" 2>"$temp_dir/gh-error"; then
    fail "$label"
  fi
}

verify_frozen_formula() {
  local response="$temp_dir/formula-response.json" blob
  api_get "$FORMULA_PATH" "$response"
  blob=$(jq -er '.sha | select(type == "string" and test("^[0-9a-f]{40}$"))' \
    "$response") || fail 'Homebrew tap Formula response has an invalid blob SHA'
  [[ "$blob" == "$FROZEN_FORMULA_BLOB" ]] \
    || fail 'published Formula differs from the frozen v0.50.71 blob'
}

verify_prior_tap_head() {
  local response="$temp_dir/prior-tap-head.json" head_sha
  api_json GET "git/ref/heads/${TAP_BRANCH}" '' "$response" \
    'cannot read the Homebrew tap branch head'
  head_sha=$(jq -er --arg ref "refs/heads/${TAP_BRANCH}" '
    select(type == "object" and .ref == $ref and .object.type == "commit") |
    .object.sha | select(type == "string" and test("^[0-9a-f]{40}$"))
  ' "$response") || fail 'Homebrew tap branch head response is invalid'
  [[ "$head_sha" == "$PRIOR_TAP_COMMIT" ]] \
    || fail 'Homebrew tap branch differs from the pinned v0.50.79 predecessor commit'
}

decode_api_content() {
  local response="$1" output="$2" encoded
  encoded=$(jq -er '.content | select(type == "string")' "$response") \
    || fail 'Homebrew tap response is missing encoded content'
  printf '%s' "$encoded" | tr -d '\r\n' | "${base64_decode[@]}" >"$output" \
    || fail 'Homebrew tap content is not valid base64'
}

cask_target="$temp_dir/cask-target.rb"
render_homebrew_cask "$cask_target" "$RELEASE_VERSION" \
  "$darwin_amd64_sha" "$darwin_arm64_sha" "$linux_amd64_sha" "$linux_arm64_sha"
if [[ -n "${COMPANION_CASK_PATH-}" ]]; then
  [[ -f "$COMPANION_CASK_PATH" && ! -L "$COMPANION_CASK_PATH" ]] \
    || fail 'GoReleaser Cask output is not a regular file'
  cmp -s "$cask_target" "$COMPANION_CASK_PATH" \
    || fail 'GoReleaser Cask output differs from the canonical renderer'
fi

sha256_file() {
  local output digest
  output=$("${sha256_command[@]}" "$1") || return 1
  digest="${output%%[[:space:]]*}"
  [[ "$digest" =~ ^[0-9a-f]{64}$ ]] || return 1
  printf '%s' "$digest"
}

publish_cask() {
  local stem="$1" label="$2" remote_path="$3" target="$4" prior_blob="$5"
  local commit_message="$6" drift_message="$7"
  local response="$temp_dir/${stem}-response.json"
  local current="$temp_dir/${stem}-current.rb"
  local target_digest current_digest blob encoded prior_tree new_blob new_tree new_commit
  local prior_commit_response="$temp_dir/prior-commit.json" blob_request="$temp_dir/${stem}-blob-request.json"
  local blob_response="$temp_dir/${stem}-blob-response.json" tree_request="$temp_dir/${stem}-tree-request.json"
  local tree_response="$temp_dir/${stem}-tree-response.json" tree_evidence_response="$temp_dir/${stem}-tree-evidence-response.json"
  local commit_request="$temp_dir/${stem}-commit-request.json" commit_response="$temp_dir/${stem}-commit-response.json"
  local ref_request="$temp_dir/${stem}-ref-request.json" ref_response="$temp_dir/${stem}-ref-response.json"
  local final_ref_response="$temp_dir/${stem}-final-ref-response.json"

  api_get "$remote_path" "$response"
  decode_api_content "$response" "$current"
  target_digest=$(sha256_file "$target") || fail "cannot digest target ${label}"
  current_digest=$(sha256_file "$current") || fail "cannot digest current ${label}"
  if [[ "$target_digest" == "$current_digest" ]] && cmp -s "$target" "$current"; then
    printf 'homebrew cask publication: %s is already current\n' "$label"
    return 0
  fi

  verify_prior_tap_head
  blob=$(jq -er '.sha | select(type == "string" and test("^[0-9a-f]{40}$"))' \
    "$response") || fail "Homebrew tap ${label} response has an invalid blob SHA"
  [[ "$blob" == "$prior_blob" ]] || fail "$drift_message"
  api_json GET "git/commits/${PRIOR_TAP_COMMIT}" '' "$prior_commit_response" \
    'cannot read the pinned Homebrew tap predecessor commit'
  prior_tree=$(jq -er --arg commit "$PRIOR_TAP_COMMIT" '
    select(type == "object" and .sha == $commit and (.parents | type) == "array" and
      (.url | type) == "string" and (.url | length) > 0) |
    .tree.sha | select(type == "string" and test("^[0-9a-f]{40}$"))
  ' "$prior_commit_response") || fail 'pinned Homebrew predecessor commit response is invalid'
  encoded=$(base64 <"$target" | tr -d '\r\n') \
    || fail "cannot encode target ${label}"
  jq -n --arg content "$encoded" '{content:$content,encoding:"base64"}' \
    >"$blob_request" || fail "cannot construct ${label} blob request"
  api_json POST 'git/blobs' "$blob_request" "$blob_response" \
    "cannot create Homebrew tap ${label} blob"
  new_blob=$(jq -er '
    select(type == "object" and (.url | type) == "string" and (.url | length) > 0) |
    .sha | select(type == "string" and test("^[0-9a-f]{40}$"))
  ' "$blob_response") || fail "Homebrew tap ${label} blob response is invalid"
  jq -n --arg base "$prior_tree" --arg path "$remote_path" --arg sha "$new_blob" \
    '{base_tree:$base,tree:[{path:$path,mode:"100644",type:"blob",sha:$sha}]}' \
    >"$tree_request" || fail "cannot construct ${label} tree request"
  api_json POST 'git/trees' "$tree_request" "$tree_response" \
    "cannot create Homebrew tap ${label} tree"
  new_tree=$(jq -er '
    select(type == "object" and .truncated == false and (.tree | type) == "array" and
      (.url | type) == "string" and (.url | length) > 0) |
    .sha | select(type == "string" and test("^[0-9a-f]{40}$"))
  ' "$tree_response") || fail "Homebrew tap ${label} tree response is invalid"
  api_json GET "git/trees/${new_tree}?recursive=1" '' "$tree_evidence_response" \
    "cannot verify Homebrew tap ${label} tree"
  jq -e --arg tree "$new_tree" --arg cask "$remote_path" --arg blob "$new_blob" \
    --arg formula "$FORMULA_PATH" --arg frozen "$FROZEN_FORMULA_BLOB" '
    type == "object" and .sha == $tree and .truncated == false and
    (.tree | type) == "array" and
    ([.tree[] | select(.path == $cask and .mode == "100644" and
      .type == "blob" and .sha == $blob)] | length) == 1 and
    ([.tree[] | select(.path == $formula and .mode == "100644" and
      .type == "blob" and .sha == $frozen)] | length) == 1
  ' "$tree_evidence_response" >/dev/null \
    || fail "Homebrew tap ${label} tree does not preserve exact Cask/Formula blobs"
  jq -n --arg message "$commit_message" --arg tree "$new_tree" \
    --arg parent "$PRIOR_TAP_COMMIT" \
    '{message:$message,tree:$tree,parents:[$parent]}' >"$commit_request" \
    || fail "cannot construct ${label} commit request"
  api_json POST 'git/commits' "$commit_request" "$commit_response" \
    "cannot create Homebrew tap ${label} commit"
  new_commit=$(jq -er --arg message "$commit_message" --arg tree "$new_tree" \
    --arg parent "$PRIOR_TAP_COMMIT" '
    select(type == "object" and (.url | type) == "string" and (.url | length) > 0 and
      .message == $message and .tree.sha == $tree and
      (.parents | type) == "array" and (.parents | length) == 1 and
      .parents[0].sha == $parent) |
    .sha | select(type == "string" and test("^[0-9a-f]{40}$"))
  ' "$commit_response") || fail "Homebrew tap ${label} commit response is invalid"
  jq -n --arg sha "$new_commit" '{sha:$sha,force:false}' >"$ref_request" \
    || fail "cannot construct ${label} ref request"
  api_json PATCH "git/refs/heads/${TAP_BRANCH}" "$ref_request" "$ref_response" \
    "Homebrew tap ${label} expected-head update failed"
  jq -e --arg ref "refs/heads/${TAP_BRANCH}" --arg commit "$new_commit" '
    type == "object" and .ref == $ref and .object.type == "commit" and
    .object.sha == $commit and (.object.url | type) == "string" and
    (.object.url | length) > 0
  ' "$ref_response" >/dev/null || fail "Homebrew tap ${label} ref response is invalid"
  api_json GET "git/ref/heads/${TAP_BRANCH}" '' "$final_ref_response" \
    'cannot verify the final Homebrew tap branch head'
  jq -e --arg ref "refs/heads/${TAP_BRANCH}" --arg commit "$new_commit" '
    type == "object" and .ref == $ref and .object.type == "commit" and
    .object.sha == $commit and (.object.url | type) == "string" and
    (.object.url | length) > 0
  ' "$final_ref_response" >/dev/null || fail "Homebrew tap ${label} head moved after publication"

  api_get "$remote_path" "$response"
  decode_api_content "$response" "$current"
  [[ "$(jq -er '.sha' "$response")" == "$new_blob" ]] && cmp -s "$target" "$current" \
    || fail "Homebrew tap ${label} differs after publication"
  printf 'homebrew cask publication: %s published and verified\n' "$label"
}

verify_frozen_formula
publish_cask cask Cask "$CASK_PATH" "$cask_target" "$PRIOR_CASK_BLOB" \
  'Publish signed Cask for v0.50.80' \
  'published Cask differs from canonical v0.50.79 output and its pinned prior blob'
