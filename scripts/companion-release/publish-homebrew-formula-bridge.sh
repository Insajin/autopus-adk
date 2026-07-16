#!/usr/bin/env bash
set -euo pipefail
umask 077

readonly RELEASE_TAG='v0.50.71'
readonly RELEASE_VERSION='0.50.71'
readonly TAP_REPOSITORY='Insajin/homebrew-autopus'
readonly TAP_BRANCH='main'
readonly CASK_PATH='Casks/auto.rb'
readonly FORMULA_PATH='Formula/auto.rb'
readonly PRIOR_FORMULA_BLOB='df2d8e25636f8a3db842948d119d46f31afd94ab'

fail() {
  printf 'homebrew formula bridge: %s\n' "$1" >&2
  exit 1
}

require_environment() {
  local name="$1"
  [[ -n "${!name-}" ]] || fail "required environment variable ${name} is missing"
}

for name in GITHUB_REF_NAME COMPANION_VERSION COMPANION_CHECKSUMS_PATH; do
  require_environment "$name"
done
[[ "$GITHUB_REF_NAME" == "$RELEASE_TAG" ]] \
  || fail 'release tag is outside the one-release Formula bridge policy'
[[ "$COMPANION_VERSION" == "$RELEASE_VERSION" ]] \
  || fail 'release version is outside the one-release Formula bridge policy'

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
declare -F render_homebrew_cask render_homebrew_formula_bridge >/dev/null \
  || fail 'Formula bridge renderer contract is incomplete'

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

decode_api_content() {
  local response="$1" output="$2" encoded
  encoded=$(jq -er '.content | select(type == "string")' "$response") \
    || fail 'Homebrew tap response is missing encoded content'
  printf '%s' "$encoded" | tr -d '\r\n' | "${base64_decode[@]}" >"$output" \
    || fail 'Homebrew tap content is not valid base64'
}

cask_response="$temp_dir/cask-response.json"
cask_content="$temp_dir/cask.rb"
cask_target="$temp_dir/cask-target.rb"
render_homebrew_cask "$cask_target" "$RELEASE_VERSION" \
  "$darwin_amd64_sha" "$darwin_arm64_sha" "$linux_amd64_sha" "$linux_arm64_sha"
api_get "$CASK_PATH" "$cask_response"
decode_api_content "$cask_response" "$cask_content"
if ! jq -e '.sha | type == "string" and test("^[0-9a-f]{40}$")' \
  "$cask_response" >/dev/null; then
  fail 'Homebrew tap Cask response has an invalid blob SHA'
fi
cmp -s "$cask_target" "$cask_content" \
  || fail 'published Cask differs from canonical GoReleaser v2.17.0 output'

formula_target="$temp_dir/formula-target.rb"
render_homebrew_formula_bridge "$formula_target" "$RELEASE_TAG" "$RELEASE_VERSION" \
  "$darwin_amd64_sha" "$darwin_arm64_sha" "$linux_amd64_sha" "$linux_arm64_sha"

sha256_file() {
  local output digest
  output=$("${sha256_command[@]}" "$1") || return 1
  digest="${output%%[[:space:]]*}"
  [[ "$digest" =~ ^[0-9a-f]{64}$ ]] || return 1
  printf '%s' "$digest"
}

formula_response="$temp_dir/formula-response.json"
formula_current="$temp_dir/formula-current.rb"
api_get "$FORMULA_PATH" "$formula_response"
decode_api_content "$formula_response" "$formula_current"
target_digest=$(sha256_file "$formula_target") || fail 'cannot digest target Formula'
current_digest=$(sha256_file "$formula_current") || fail 'cannot digest current Formula'
if [[ "$target_digest" == "$current_digest" ]] && cmp -s "$formula_target" "$formula_current"; then
  printf 'homebrew formula bridge: Formula is already current\n'
  exit 0
fi

formula_blob=$(jq -er '.sha | select(type == "string" and test("^[0-9a-f]{40}$"))' \
  "$formula_response") || fail 'Homebrew tap Formula response has an invalid blob SHA'
[[ "$formula_blob" == "$PRIOR_FORMULA_BLOB" ]] \
  || fail 'Homebrew tap Formula has drifted from the pinned prior blob'

formula_base64=$(base64 <"$formula_target" | tr -d '\r\n') \
  || fail 'cannot encode target Formula'
jq -n \
  --arg message 'Bridge legacy Formula users to the signed Cask for v0.50.71' \
  --arg content "$formula_base64" --arg sha "$formula_blob" --arg branch "$TAP_BRANCH" \
  '{message: $message, content: $content, sha: $sha, branch: $branch}' \
  >"$temp_dir/formula-request.json" || fail 'cannot construct Formula update request'
if ! GH_TOKEN="$tap_token" gh api --method PUT \
  -H 'Accept: application/vnd.github+json' \
  "repos/${TAP_REPOSITORY}/contents/${FORMULA_PATH}" \
  --input "$temp_dir/formula-request.json" \
  >"$temp_dir/formula-update-response.json" 2>"$temp_dir/gh-error"; then
  fail 'Homebrew tap Formula compare-and-swap update failed'
fi

api_get "$FORMULA_PATH" "$formula_response"
decode_api_content "$formula_response" "$formula_current"
cmp -s "$formula_target" "$formula_current" \
  || fail 'Homebrew tap Formula differs after publication'
printf 'homebrew formula bridge: Formula bridge published and verified\n'
