#!/usr/bin/env bash
# Fail closed when the Homebrew publisher's predecessor coordinates no longer
# match the live tap.
#
# The publisher enforces these coordinates at the very end of a release, after
# GoReleaser has already created an immutable GitHub release. A stale pin there
# burns the version outright: the release cannot be revised, and the recovery
# workflow runs the tag's own scripts, which carry the same stale pin. v0.50.105
# was lost exactly that way. Running the same comparison before any artifact is
# published turns that dead end into a fixable preflight failure.
#
# Reading the pins out of the publisher keeps one source of truth, and the tap is
# the fact they must match. The tap is public, so this needs no write credential
# and can run before release credentials exist.
set -euo pipefail
umask 077

readonly bridge="${COMPANION_HOMEBREW_BRIDGE:-scripts/companion-release/publish-homebrew-formula-bridge.sh}"

fail() {
  printf 'homebrew tap pins: %s\n' "$1" >&2
  exit 1
}

pin() {
  local name=$1 pattern=${2:-'[0-9a-f]{40}'} value
  value=$(sed -nE "s/^readonly ${name}='(${pattern})'\$/\1/p" "$bridge")
  [[ -n "$value" ]] || fail "cannot read ${name} from ${bridge}"
  printf '%s\n' "$value"
}

[[ -f "$bridge" && ! -L "$bridge" ]] || fail "missing or unsafe publisher ${bridge}"

pinned_commit=$(pin PRIOR_TAP_COMMIT)
pinned_blob=$(pin PRIOR_CASK_BLOB)
tap_repository=$(pin TAP_REPOSITORY "[^']+")
tap_branch=$(pin TAP_BRANCH "[^']+")
cask_path=$(pin CASK_PATH "[^']+")

head_sha=$(gh api "repos/${tap_repository}/git/ref/heads/${tap_branch}" --jq .object.sha) ||
  fail "cannot read ${tap_repository} branch ${tap_branch}"
[[ "$head_sha" =~ ^[0-9a-f]{40}$ ]] || fail 'tap branch head is malformed'
[[ "$head_sha" == "$pinned_commit" ]] || fail \
  "tap head ${head_sha} differs from pinned PRIOR_TAP_COMMIT ${pinned_commit}; bump the pin in ${bridge}"

blob_sha=$(gh api "repos/${tap_repository}/contents/${cask_path}?ref=${tap_branch}" --jq .sha) ||
  fail "cannot read ${cask_path} from ${tap_repository}"
[[ "$blob_sha" =~ ^[0-9a-f]{40}$ ]] || fail 'tap Cask blob is malformed'
[[ "$blob_sha" == "$pinned_blob" ]] || fail \
  "tap Cask blob ${blob_sha} differs from pinned PRIOR_CASK_BLOB ${pinned_blob}; bump the pin in ${bridge}"

printf 'homebrew tap pins: match (head %s, cask %s)\n' "$head_sha" "$blob_sha"
