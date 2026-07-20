#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() {
  printf 'current release evidence: %s\n' "$1" >&2
  exit 1
}

readonly RELEASE_REPOSITORY='Insajin/autopus-adk'
readonly RELEASE_TAG='v0.50.79'
readonly RELEASE_VERSION='0.50.79'

EXPECTED_ARCHIVES=(
  "autopus-adk_${RELEASE_VERSION}_darwin_amd64.tar.gz"
  "autopus-adk_${RELEASE_VERSION}_darwin_arm64.tar.gz"
  "autopus-adk_${RELEASE_VERSION}_linux_amd64.tar.gz"
  "autopus-adk_${RELEASE_VERSION}_linux_arm64.tar.gz"
  "autopus-adk_${RELEASE_VERSION}_windows_amd64.tar.gz"
  "autopus-adk_${RELEASE_VERSION}_windows_amd64.zip"
  "autopus-adk_${RELEASE_VERSION}_windows_arm64.tar.gz"
  "autopus-adk_${RELEASE_VERSION}_windows_arm64.zip"
)
readonly EXPECTED_ARCHIVES
EXPECTED_ASSETS=(
  "${EXPECTED_ARCHIVES[@]}"
  'checksums.txt'
  'checksums.txt.bundle'
  'checksums.txt.signatures'
)
readonly EXPECTED_ASSETS
[[ $# == 1 ]] || fail 'usage: verify-current-release.sh CHECKSUMS_OUTPUT'
readonly checksums_output=$1
[[ -n "${GITHUB_TOKEN:-}" ]] || fail 'missing GITHUB_TOKEN'
[[ "${COMPANION_SOURCE_COMMIT:-}" =~ ^[0-9a-f]{40}$ ]] \
  || fail 'exact source commit is missing or malformed'
for tool in gh jq shasum; do
  command -v "$tool" >/dev/null 2>&1 || fail "required tool is unavailable: ${tool}"
done

output_dir=$(dirname -- "$checksums_output")
[[ -d "$output_dir" && ! -L "$output_dir" ]] \
  || fail 'checksums output parent must be a non-symlink directory'
[[ ! -e "$checksums_output" && ! -L "$checksums_output" ]] \
  || fail 'checksums output already exists'

temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/adk-current-release.XXXXXX") \
  || fail 'cannot create release evidence directory'
cleanup() {
  local status=$?
  rm -rf -- "$temp_dir" || status=$?
  if ((status != 0)); then
    rm -f -- "$checksums_output" || true
  fi
  return "$status"
}
trap cleanup EXIT
release_json="$temp_dir/release.json"
downloaded_checksums="$temp_dir/checksums.txt"

if ! GH_TOKEN="$GITHUB_TOKEN" gh api \
  -H 'Accept: application/vnd.github+json' \
  "repos/${RELEASE_REPOSITORY}/releases/tags/${RELEASE_TAG}" > "$release_json"; then
  fail 'cannot read the exact A8 GitHub release'
fi
[[ -f "$release_json" && ! -L "$release_json" && -s "$release_json" ]] \
  || fail 'A8 GitHub release metadata is empty or unsafe'

expected_assets_json=$(printf '%s\n' "${EXPECTED_ASSETS[@]}" \
  | jq -Rsc 'split("\n") | map(select(length > 0))') \
  || fail 'cannot construct the expected A8 asset set'
if ! jq -e --arg tag "$RELEASE_TAG" --arg commit "$COMPANION_SOURCE_COMMIT" \
  --argjson expected "$expected_assets_json" '
    type == "object" and
    .tag_name == $tag and
    .target_commitish == $commit and
    .draft == false and
    .prerelease == false and
    .immutable == true and
    (.assets | type) == "array" and
    (.assets | length) == ($expected | length) and
    ([.assets[].name] | sort) == ($expected | sort) and
    ([.assets[].name] | unique | length) == ($expected | length) and
    all(.assets[];
      (.id | type) == "number" and .id > 0 and
      (.name | type) == "string" and
      .state == "uploaded" and
      (.size | type) == "number" and .size > 0 and
      (.digest | type) == "string" and
      (.digest | test("^sha256:[0-9a-f]{64}$")))
  ' "$release_json" >/dev/null; then
  fail 'A8 release is not exact, final, immutable, complete, and digest-bound'
fi

checksums_metadata=$(jq -er '
  .assets[] | select(.name == "checksums.txt") |
  [.id, .size, .digest] | @tsv
' "$release_json") || fail 'checksums.txt metadata is unavailable'
IFS=$'\t' read -r checksums_id checksums_size checksums_api_digest \
  <<< "$checksums_metadata"
[[ "$checksums_id" =~ ^[1-9][0-9]*$ && "$checksums_size" =~ ^[1-9][0-9]*$ ]] \
  || fail 'checksums.txt identifier or size is malformed'

if ! GH_TOKEN="$GITHUB_TOKEN" gh api \
  -H 'Accept: application/octet-stream' \
  "repos/${RELEASE_REPOSITORY}/releases/assets/${checksums_id}" \
  > "$downloaded_checksums"; then
  fail 'cannot download checksums.txt from the exact A8 release'
fi
[[ -f "$downloaded_checksums" && ! -L "$downloaded_checksums" \
   && -s "$downloaded_checksums" ]] \
  || fail 'downloaded checksums.txt is empty or unsafe'
downloaded_size=$(wc -c < "$downloaded_checksums" | tr -d '[:space:]')
[[ "$downloaded_size" == "$checksums_size" ]] \
  || fail 'downloaded checksums.txt size differs from GitHub metadata'
downloaded_digest=$(shasum -a 256 "$downloaded_checksums" | awk '{print $1}') \
  || fail 'cannot digest downloaded checksums.txt'
[[ "$downloaded_digest" =~ ^[0-9a-f]{64}$ \
   && "sha256:${downloaded_digest}" == "$checksums_api_digest" ]] \
  || fail 'downloaded checksums.txt differs from its GitHub API digest'

expected_archives_json=$(printf '%s\n' "${EXPECTED_ARCHIVES[@]}" \
  | jq -Rsc 'split("\n") | map(select(length > 0))') \
  || fail 'cannot construct the expected archive set'
checksum_entries_json=$(jq -Rsc '
  if endswith("\n") then .[0:-1] else error("missing final newline") end |
  split("\n") |
  map(capture("^(?<digest>[0-9a-f]{64})  (?<name>[A-Za-z0-9._-]+)$"))
' "$downloaded_checksums") || fail 'checksums.txt contains malformed lines'
if ! printf '%s' "$checksum_entries_json" | jq -e \
  --argjson expected "$expected_archives_json" '
    length == ($expected | length) and
    ([.[].name] | sort) == ($expected | sort) and
    ([.[].name] | unique | length) == ($expected | length)
  ' >/dev/null; then
  fail 'checksums.txt does not describe exactly the eight A8 archives'
fi

for archive in "${EXPECTED_ARCHIVES[@]}"; do
  api_digest=$(jq -er --arg name "$archive" \
    '.assets[] | select(.name == $name) | .digest' "$release_json") \
    || fail "API digest is unavailable for ${archive}"
  checksum_digest=$(printf '%s' "$checksum_entries_json" \
    | jq -er --arg name "$archive" '.[] | select(.name == $name) | .digest') \
    || fail "checksum entry is unavailable for ${archive}"
  [[ "$api_digest" == "sha256:${checksum_digest}" ]] \
    || fail "checksums.txt differs from the API digest for ${archive}"
done

install -m 0600 "$downloaded_checksums" "$checksums_output" \
  || fail 'cannot materialize verified checksums.txt'
cmp -s "$downloaded_checksums" "$checksums_output" \
  || fail 'materialized checksums.txt differs from verified bytes'
printf 'current release evidence: exact immutable A8 release verified\n'
