#!/usr/bin/env bash
set -euo pipefail

readonly A2_A1_ANCESTOR_SHA='e25e8be02b55b9385f58919c30ad1ccf92179030'
readonly A2_MAIN_ANCESTOR_SHA='acb735cca0ef120cfed0d01863de09535310b5a3'
readonly A3_A2_ANCESTOR_SHA='7b5b52822b0cda75bf6c971f5f1c2a713881008c'

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
  v0.50.71) release_phase='A2' ;;
  v0.50.72) release_phase='A3' ;;
  *) fail 'release tag is outside the frozen A0/A1/A2/A3 policy' ;;
esac
[[ "$GITHUB_REF_TYPE" == 'tag' ]] || fail 'release ref is not a tag'
[[ "$GITHUB_SHA" =~ ^[0-9a-f]{40}$ ]] || fail 'source commit is not exact 40-hex'

head_commit=$(git rev-parse --verify 'HEAD^{commit}') \
  || fail 'cannot resolve checked-out source commit'
tag_commit=$(git rev-parse --verify "${GITHUB_REF_NAME}^{commit}") \
  || fail 'cannot resolve release tag commit'
[[ "$head_commit" == "$GITHUB_SHA" && "$tag_commit" == "$GITHUB_SHA" ]] \
  || fail 'checked-out source, tag, and release commit differ'

if [[ "$release_phase" == 'A2' || "$release_phase" == 'A3' ]]; then
  tag_object_type=$(git cat-file -t "refs/tags/$GITHUB_REF_NAME" 2>/dev/null) \
    || fail "cannot resolve exact ${release_phase} tag object"
  [[ "$tag_object_type" == 'tag' ]] \
    || fail "${release_phase} release tag must be annotated"
  if [[ "$release_phase" == 'A2' ]]; then
    git merge-base --is-ancestor "$A2_A1_ANCESTOR_SHA" "$GITHUB_SHA" \
      >/dev/null 2>&1 || fail 'A2 source does not contain the immutable A1 release'
    git merge-base --is-ancestor "$A2_MAIN_ANCESTOR_SHA" "$GITHUB_SHA" \
      >/dev/null 2>&1 || fail 'A2 source does not contain the integrated main base'
  else
    git merge-base --is-ancestor "$A3_A2_ANCESTOR_SHA" "$GITHUB_SHA" \
      >/dev/null 2>&1 || fail 'A3 source does not contain the immutable A2 release'
  fi
  case "${COMPANION_SOURCE_PIN_REQUIRED-0}" in
    0) ;;
    1)
      for name in COMPANION_APPROVED_SOURCE_COMMIT COMPANION_APPROVED_SOURCE_TREE; do
        [[ -n "${!name-}" ]] || fail "required approved source pin ${name} is missing"
        [[ "${!name}" =~ ^[0-9a-f]{40}$ ]] || fail "approved source pin ${name} is malformed"
      done
      source_tree=$(git rev-parse --verify 'HEAD^{tree}') \
        || fail 'cannot resolve checked-out source tree'
      [[ "$GITHUB_SHA" == "$COMPANION_APPROVED_SOURCE_COMMIT" ]] \
        || fail 'release commit differs from the approved exact source commit'
      [[ "$source_tree" == "$COMPANION_APPROVED_SOURCE_TREE" ]] \
        || fail 'release tree differs from the approved exact source tree'
      ;;
    *) fail 'COMPANION_SOURCE_PIN_REQUIRED must be 0 or 1' ;;
  esac
fi

printf 'release-phase=%s\nsource-commit=%s\n' "$release_phase" "$GITHUB_SHA" \
  >>"$GITHUB_OUTPUT"
