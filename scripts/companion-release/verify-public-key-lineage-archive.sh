#!/usr/bin/env bash

sha256_file() {
  local output digest
  output=$(shasum -a 256 "$1") || return 1
  digest="${output%%[[:space:]]*}"
  [[ "$digest" =~ ^[0-9a-f]{64}$ ]] || return 1
  printf 'sha256:%s' "$digest"
}

nonzero_hex() { [[ "$1" =~ ^[0-9a-f]{$2}$ && -n "${1//0/}" ]]; }

extract_bundle() {
  local archive="$1" output_dir="$2" architecture="$3" manifest_pin="$4"
  local listing="$output_dir/archive-entries"
  install -m 0700 -d "$output_dir"
  tar -tzf "$archive" >"$listing" \
    || fail prior_evidence_malformed "${prior_phase} archive cannot be listed"
  for entry in "$ARTIFACT_NAME" "$MANIFEST_NAME" "$MANIFEST_SIGNATURE_NAME" \
    "$BUNDLE_NAME/$RECEIPT_NAME" "$BUNDLE_NAME/$SIGNATURE_NAME"; do
    [[ "$(grep -Fxc "$entry" "$listing")" == '1' ]] \
      || fail prior_evidence_absent "canonical archive entry is absent: ${entry}"
  done
  unexpected=$(awk -v prefix="$BUNDLE_NAME/" -v receipt="$BUNDLE_NAME/$RECEIPT_NAME" \
    -v signature="$BUNDLE_NAME/$SIGNATURE_NAME" \
    'index($0, prefix) == 1 && $0 != prefix && $0 != receipt && $0 != signature { print; exit }' \
    "$listing")
  [[ -z "$unexpected" ]] \
    || fail prior_evidence_malformed 'canonical receipt bundle has extra entries'
  tar -xOzf "$archive" "$BUNDLE_NAME/$RECEIPT_NAME" >"$output_dir/$RECEIPT_NAME" \
    || fail prior_evidence_absent 'cannot extract prior_receipt'
  tar -xOzf "$archive" "$BUNDLE_NAME/$SIGNATURE_NAME" >"$output_dir/$SIGNATURE_NAME" \
    || fail prior_evidence_absent 'cannot extract prior signature'
  tar -xOzf "$archive" "$MANIFEST_NAME" >"$output_dir/$MANIFEST_NAME" \
    || fail prior_evidence_absent 'cannot extract prior manifest'
  tar -xOzf "$archive" "$MANIFEST_SIGNATURE_NAME" \
    >"$output_dir/$MANIFEST_SIGNATURE_NAME" \
    || fail prior_evidence_absent 'cannot extract prior manifest signature'
  tar -xOzf "$archive" "$ARTIFACT_NAME" >"$output_dir/$ARTIFACT_NAME" \
    || fail prior_evidence_absent 'cannot extract prior artifact'
  chmod 0600 "$output_dir/$RECEIPT_NAME" "$output_dir/$SIGNATURE_NAME" \
    "$output_dir/$MANIFEST_NAME" "$output_dir/$MANIFEST_SIGNATURE_NAME" \
    "$output_dir/$ARTIFACT_NAME"
  manifest="$output_dir/$MANIFEST_NAME"
  manifest_version=$(jq -er '.version | select(type == "string")' "$manifest") \
    || fail prior_evidence_malformed 'prior manifest version is malformed'
  [[ "$manifest_version" == "$prior_version" ]] \
    || fail prior_manifest_version_mismatch "prior manifest version differs from ${prior_phase}"
  manifest_key_id=$(jq -er '.key_id | select(type == "string")' "$manifest") \
    || fail prior_evidence_malformed 'prior manifest key_id is malformed'
  receipt_key_id=$(jq -er '.key_id | select(type == "string")' "$output_dir/$RECEIPT_NAME") \
    || fail prior_evidence_malformed 'prior receipt key_id is malformed'
  [[ "$manifest_key_id" == "$COMPANION_KEY_ID" && "$receipt_key_id" == "$manifest_key_id" ]] \
    || fail prior_key_overlap_mismatch "prior manifest and receipt key IDs do not overlap ${release_phase}"
  jq -e --arg arch "$architecture" --arg handoff "$COMPANION_HANDOFF" \
    --arg floor "$COMPANION_ROLLBACK_FLOOR" \
    '.schema_version == "adk-companion-manifest.v1" and .platform == "darwin" and
     .architecture == $arch and .handoff == $handoff and (.rollback_floor | tostring) == $floor' \
    "$manifest" >/dev/null || fail prior_manifest_claim_mismatch 'prior manifest claims differ'
  claimed_artifact_digest=$(jq -er '.artifact_digest | select(type == "string")' "$manifest") \
    || fail prior_evidence_malformed 'prior manifest artifact digest is malformed'
  [[ "$claimed_artifact_digest" == "$(sha256_file "$output_dir/$ARTIFACT_NAME")" ]] \
    || fail prior_manifest_artifact_mismatch 'prior manifest does not bind its artifact'
  [[ "$(sha256_file "$manifest")" == "sha256:$manifest_pin" ]] \
    || fail prior_manifest_digest_mismatch "prior manifest differs from its ${prior_phase} pin"
  if [[ -n "${COMPANION_MANIFEST_VERIFIER-}" ]]; then
    env -i PATH="$PATH" HOME="${HOME-}" \
      "$COMPANION_MANIFEST_VERIFIER" \
      --artifact "$output_dir/$ARTIFACT_NAME" \
      --manifest "$manifest" \
      --signature "$output_dir/$MANIFEST_SIGNATURE_NAME" \
      --receipt "$output_dir/$RECEIPT_NAME" \
      --receipt-signature "$output_dir/$SIGNATURE_NAME" \
      --key-id "$COMPANION_KEY_ID" \
      --version "$prior_version" \
      --platform darwin \
      --architecture "$architecture" \
      --handoff "$COMPANION_HANDOFF" \
      --minimum-rollback-floor "$COMPANION_ROLLBACK_FLOOR" \
      --public-key-sha256 "sha256:$A0_PUBLIC_KEY_SHA256" \
      --manifest-sha256 "sha256:$manifest_pin" \
      || fail prior_manifest_signature_mismatch \
        'prior manifest signature or artifact binding is invalid'
  fi
}
