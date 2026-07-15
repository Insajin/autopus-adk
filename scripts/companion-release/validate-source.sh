#!/usr/bin/env bash
set -euo pipefail

fail() {
  printf 'companion release source: %s\n' "$1" >&2
  exit 1
}

for name in GITHUB_REF_NAME GITHUB_REF_TYPE GITHUB_SHA GITHUB_OUTPUT; do
  [[ -n "${!name-}" ]] || fail "required environment variable ${name} is missing"
done

case "$GITHUB_REF_NAME" in
  v0.50.69) release_phase='A0' ;;
  v0.50.70) release_phase='A1' ;;
  *) fail 'release tag is outside the frozen A0/A1 policy' ;;
esac
[[ "$GITHUB_REF_TYPE" == 'tag' ]] || fail 'release ref is not a tag'
[[ "$GITHUB_SHA" =~ ^[0-9a-f]{40}$ ]] || fail 'source commit is not exact 40-hex'

head_commit=$(git rev-parse --verify 'HEAD^{commit}') \
  || fail 'cannot resolve checked-out source commit'
tag_commit=$(git rev-parse --verify "${GITHUB_REF_NAME}^{commit}") \
  || fail 'cannot resolve release tag commit'
[[ "$head_commit" == "$GITHUB_SHA" && "$tag_commit" == "$GITHUB_SHA" ]] \
  || fail 'checked-out source, tag, and release commit differ'

printf 'release-phase=%s\nsource-commit=%s\n' "$release_phase" "$GITHUB_SHA" \
  >>"$GITHUB_OUTPUT"
