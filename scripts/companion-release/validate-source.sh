#!/usr/bin/env bash
set -euo pipefail

readonly A2_A1_ANCESTOR_SHA='e25e8be02b55b9385f58919c30ad1ccf92179030'
readonly A2_MAIN_ANCESTOR_SHA='acb735cca0ef120cfed0d01863de09535310b5a3'
readonly A3_A2_ANCESTOR_SHA='7b5b52822b0cda75bf6c971f5f1c2a713881008c'
readonly A4_A3_ANCESTOR_SHA='ba5509b692a43dc8a70e0bd6173acb56166ed67f'
readonly A5_A4_ANCESTOR_SHA='334b297f05942accbecdfa15b54e38e005c82f2d'
readonly A6_A5_ANCESTOR_SHA='b27252cb1148192a8ae1a95195c50e5f221453a4'
readonly A7_A6_ANCESTOR_SHA='902f1acfa91f1d0a2ac9471d5cd79117031a2599'
readonly A8_A7_ANCESTOR_SHA='51de6030a69a8e36fcf7e5790ef157eff6fedf00'
readonly A9_A8_ANCESTOR_SHA='dd0c2759ed5435d4634011e349caad62ea3df414'
readonly A10_A9_ANCESTOR_SHA='c9c4f49d48022eb0c8d72ee7b520136a4f21f176'
readonly A11_A10_ANCESTOR_SHA='54536edc09c37a634532c2c9b51e62869d393db4'
readonly A12_A11_ANCESTOR_SHA='a8558ccc36e04125de6b8d84c7ffc9e8ddb5a2c9'
readonly A13_A12_ANCESTOR_SHA='e6367b5375cd4cdf09cb1515877bc57323521364'
readonly A14_A13_ANCESTOR_SHA='2b7aa046bdb7861113dfa57b30489c11715582e9'
readonly A15_A14_ANCESTOR_SHA='4b8eb62200d253b46e022670c482e2f716a992a3'
readonly A16_A15_ANCESTOR_SHA='0fc4f60dac8ff8afe69b680c8bf723bfbced4769'

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
  v0.50.77) release_phase='A6' ;;
  v0.50.78) release_phase='A7' ;;
  v0.50.79) release_phase='A8' ;;
  v0.50.80) release_phase='A9' ;;
  v0.50.81) release_phase='A10' ;;
  v0.50.82) release_phase='A11' ;;
  v0.50.83) release_phase='A12' ;;
  v0.50.84) release_phase='A13' ;;
  v0.50.85) release_phase='A14' ;;
  v0.50.86) release_phase='A15' ;;
  v0.50.87) release_phase='A16' ;;
  *) fail 'release tag is outside the frozen A0/A1/A2/A3/A4/A5/A6/A7/A8/A9/A10/A11/A12/A13/A14/A15/A16 policy' ;;
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
      "$release_phase" == 'A4' || "$release_phase" == 'A5' ||
      "$release_phase" == 'A6' || "$release_phase" == 'A7' ||
      "$release_phase" == 'A8' || "$release_phase" == 'A9' ||
      "$release_phase" == 'A10' || "$release_phase" == 'A11' ||
      "$release_phase" == 'A12' || "$release_phase" == 'A13' ||
      "$release_phase" == 'A14' || "$release_phase" == 'A15' ||
      "$release_phase" == 'A16' ]]; then
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
  elif [[ "$release_phase" == 'A6' ]]; then
    git merge-base --is-ancestor "$A6_A5_ANCESTOR_SHA" "$GITHUB_SHA" \
      >/dev/null 2>&1 || fail 'A6 source does not contain the immutable A5 release'
  elif [[ "$release_phase" == 'A7' ]]; then
    git merge-base --is-ancestor "$A7_A6_ANCESTOR_SHA" "$GITHUB_SHA" \
      >/dev/null 2>&1 || fail 'A7 source does not contain the immutable A6 release'
  elif [[ "$release_phase" == 'A8' ]]; then
    git merge-base --is-ancestor "$A8_A7_ANCESTOR_SHA" "$GITHUB_SHA" \
      >/dev/null 2>&1 || fail 'A8 source does not contain the immutable A7 release'
  elif [[ "$release_phase" == 'A9' ]]; then
    git merge-base --is-ancestor "$A9_A8_ANCESTOR_SHA" "$GITHUB_SHA" \
      >/dev/null 2>&1 || fail 'A9 source does not contain the immutable A8 release'
  elif [[ "$release_phase" == 'A10' ]]; then
    git merge-base --is-ancestor "$A10_A9_ANCESTOR_SHA" "$GITHUB_SHA" \
      >/dev/null 2>&1 || fail 'A10 source does not contain the immutable A9 release'
  elif [[ "$release_phase" == 'A11' ]]; then
    git merge-base --is-ancestor "$A11_A10_ANCESTOR_SHA" "$GITHUB_SHA" \
      >/dev/null 2>&1 || fail 'A11 source does not contain the immutable A10 release'
  elif [[ "$release_phase" == 'A12' ]]; then
    git merge-base --is-ancestor "$A12_A11_ANCESTOR_SHA" "$GITHUB_SHA" \
      >/dev/null 2>&1 || fail 'A12 source does not contain the immutable A11 release'
  elif [[ "$release_phase" == 'A13' ]]; then
    git merge-base --is-ancestor "$A13_A12_ANCESTOR_SHA" "$GITHUB_SHA" \
      >/dev/null 2>&1 || fail 'A13 source does not contain the immutable A12 release'
  elif [[ "$release_phase" == 'A14' ]]; then
    git merge-base --is-ancestor "$A14_A13_ANCESTOR_SHA" "$GITHUB_SHA" \
      >/dev/null 2>&1 || fail 'A14 source does not contain the immutable A13 release'
  elif [[ "$release_phase" == 'A15' ]]; then
    git merge-base --is-ancestor "$A15_A14_ANCESTOR_SHA" "$GITHUB_SHA" \
      >/dev/null 2>&1 || fail 'A15 source does not contain the immutable A14 release'
  else
    git merge-base --is-ancestor "$A16_A15_ANCESTOR_SHA" "$GITHUB_SHA" \
      >/dev/null 2>&1 || fail 'A16 source does not contain the immutable A15 release'
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
