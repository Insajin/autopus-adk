#!/usr/bin/env bash
set -euo pipefail
umask 077

readonly RELEASE_TAG='v0.50.72'
readonly RELEASE_VERSION='0.50.72'
readonly RELEASE_POLICY='cask-only'
readonly TAP_REPOSITORY='Insajin/homebrew-autopus'
readonly TAP_BRANCH='main'
readonly CASK_PATH='Casks/auto.rb'
readonly PRIOR_CASK_BLOB='8d09a2d11a62b3db5fd7b3523f2626a34604b0b9'

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

reconcile_tap_file() {
  local stem="$1" label="$2" remote_path="$3" target="$4" prior_blob="$5"
  local commit_message="$6" drift_message="$7"
  local response="$temp_dir/${stem}-response.json"
  local current="$temp_dir/${stem}-current.rb"
  local request="$temp_dir/${stem}-request.json"
  local update_response="$temp_dir/${stem}-update-response.json"
  local target_digest current_digest blob encoded

  api_get "$remote_path" "$response"
  decode_api_content "$response" "$current"
  target_digest=$(sha256_file "$target") || fail "cannot digest target ${label}"
  current_digest=$(sha256_file "$current") || fail "cannot digest current ${label}"
  if [[ "$target_digest" == "$current_digest" ]] && cmp -s "$target" "$current"; then
    printf 'homebrew cask publication: %s is already current\n' "$label"
    return 0
  fi

  blob=$(jq -er '.sha | select(type == "string" and test("^[0-9a-f]{40}$"))' \
    "$response") || fail "Homebrew tap ${label} response has an invalid blob SHA"
  [[ "$blob" == "$prior_blob" ]] || fail "$drift_message"
  encoded=$(base64 <"$target" | tr -d '\r\n') \
    || fail "cannot encode target ${label}"
  jq -n --arg message "$commit_message" --arg content "$encoded" \
    --arg sha "$blob" --arg branch "$TAP_BRANCH" \
    '{message: $message, content: $content, sha: $sha, branch: $branch}' \
    >"$request" || fail "cannot construct ${label} update request"
  if ! GH_TOKEN="$tap_token" gh api --method PUT \
    -H 'Accept: application/vnd.github+json' \
    "repos/${TAP_REPOSITORY}/contents/${remote_path}" --input "$request" \
    >"$update_response" 2>"$temp_dir/gh-error"; then
    fail "Homebrew tap ${label} compare-and-swap update failed"
  fi

  api_get "$remote_path" "$response"
  decode_api_content "$response" "$current"
  cmp -s "$target" "$current" \
    || fail "Homebrew tap ${label} differs after publication"
  printf 'homebrew cask publication: %s published and verified\n' "$label"
}

reconcile_tap_file cask Cask "$CASK_PATH" "$cask_target" "$PRIOR_CASK_BLOB" \
  'Publish signed Cask for v0.50.72' \
  'published Cask differs from canonical v0.50.71 output and its pinned prior blob'
