#!/usr/bin/env bash
set -euo pipefail

readonly A2_A1_ANCESTOR_SHA='e25e8be02b55b9385f58919c30ad1ccf92179030'
readonly A2_MAIN_ANCESTOR_SHA='acb735cca0ef120cfed0d01863de09535310b5a3'
readonly A3_A2_ANCESTOR_SHA='7b5b52822b0cda75bf6c971f5f1c2a713881008c'
readonly A4_A3_ANCESTOR_SHA='ba5509b692a43dc8a70e0bd6173acb56166ed67f'
readonly A5_A4_ANCESTOR_SHA='334b297f05942accbecdfa15b54e38e005c82f2d'
readonly A6_A5_ANCESTOR_SHA='b27252cb1148192a8ae1a95195c50e5f221453a4'

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
  v0.50.73) release_phase='A4' ;;
  v0.50.74) release_phase='A5' ;;
  v0.50.75) release_phase='A6' ;;
  *) fail 'release tag is outside the frozen A0/A1/A2/A3/A4/A5/A6 policy' ;;
esac
[[ "$GITHUB_REF_TYPE" == 'tag' ]] || fail 'release ref is not a tag'
[[ "$GITHUB_SHA" =~ ^[0-9a-f]{40}$ ]] || fail 'source commit is not exact 40-hex'

head_commit=$(git rev-parse --verify 'HEAD^{commit}') \
  || fail 'cannot resolve checked-out source commit'
tag_commit=$(git rev-parse --verify "${GITHUB_REF_NAME}^{commit}") \
  || fail 'cannot resolve release tag commit'
[[ "$head_commit" == "$GITHUB_SHA" && "$tag_commit" == "$GITHUB_SHA" ]] \
  || fail 'checked-out source, tag, and release commit differ'

if [[ "$release_phase" == 'A2' || "$release_phase" == 'A3' ||
      "$release_phase" == 'A4' || "$release_phase" == 'A5' || "$release_phase" == 'A6' ]]; then
  tag_object_type=$(git cat-file -t "refs/tags/$GITHUB_REF_NAME" 2>/dev/null) \
    || fail "cannot resolve exact ${release_phase} tag object"
  [[ "$tag_object_type" == 'tag' ]] \
    || fail "${release_phase} release tag must be annotated"
  if [[ "$release_phase" == 'A2' ]]; then
    git merge-base --is-ancestor "$A2_A1_ANCESTOR_SHA" "$GITHUB_SHA" \
      >/dev/null 2>&1 || fail 'A2 source does not contain the immutable A1 release'
    git merge-base --is-ancestor "$A2_MAIN_ANCESTOR_SHA" "$GITHUB_SHA" \
      >/dev/null 2>&1 || fail 'A2 source does not contain the integrated main base'
  elif [[ "$release_phase" == 'A3' ]]; then
    git merge-base --is-ancestor "$A3_A2_ANCESTOR_SHA" "$GITHUB_SHA" \
      >/dev/null 2>&1 || fail 'A3 source does not contain the immutable A2 release'
  elif [[ "$release_phase" == 'A4' ]]; then
    git merge-base --is-ancestor "$A4_A3_ANCESTOR_SHA" "$GITHUB_SHA" \
      >/dev/null 2>&1 || fail 'A4 source does not contain the immutable A3 release'
  elif [[ "$release_phase" == 'A5' ]]; then
    git merge-base --is-ancestor "$A5_A4_ANCESTOR_SHA" "$GITHUB_SHA" \
      >/dev/null 2>&1 || fail 'A5 source does not contain the immutable A4 release'
  else
    git merge-base --is-ancestor "$A6_A5_ANCESTOR_SHA" "$GITHUB_SHA" \
      >/dev/null 2>&1 || fail 'A6 source does not contain the immutable A5 release'
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
