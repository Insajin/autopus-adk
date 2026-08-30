#!/usr/bin/env bash

# @AX:ANCHOR [AUTO]: Verify every pinned predecessor archive through one four-way digest path.
# @AX:REASON [AUTO]: API metadata, downloaded bytes, immutable pins, and checksums.txt must agree before Darwin trust evidence is extracted.
verify_public_key_lineage_assets() {
  local darwin_amd64_asset="autopus-adk_${prior_version}_darwin_amd64.tar.gz"
  local darwin_arm64_asset="autopus-adk_${prior_version}_darwin_arm64.tar.gz"
  local linux_amd64_asset="autopus-adk_${prior_version}_linux_amd64.tar.gz"
  local linux_arm64_asset="autopus-adk_${prior_version}_linux_arm64.tar.gz"
  local download_dir="$temp_dir/downloads"
  local index asset archive_pin asset_digest downloaded_asset actual_asset_digest
  local checksum_line
  local -a archive_assets=("$darwin_amd64_asset" "$darwin_arm64_asset")
  local -a archive_pins=("$prior_amd64_archive" "$prior_arm64_archive")

  if [[ -n "$prior_linux_amd64_archive" || -n "$prior_linux_arm64_archive" ]]; then
    [[ -n "$prior_linux_amd64_archive" && -n "$prior_linux_arm64_archive" ]] \
      || fail prior_evidence_unverifiable \
        "${prior_phase} Linux archive pins must be provisioned together"
    archive_assets+=("$linux_amd64_asset" "$linux_arm64_asset")
    archive_pins+=("$prior_linux_amd64_archive" "$prior_linux_arm64_archive")
  fi

  install -m 0700 -d "$download_dir"
  for ((index = 0; index < ${#archive_assets[@]}; index++)); do
    asset=${archive_assets[index]}
    archive_pin=${archive_pins[index]}
    asset_digest=$(jq -er --arg name "$asset" \
      '[.assets[] | select(.name == $name)] | select(length == 1) | .[0] |
       select(.state == "uploaded") | .digest |
       select(type == "string" and test("^sha256:[0-9a-f]{64}$"))' "$release_json") \
      || fail prior_evidence_malformed "exact asset metadata is missing for ${asset}"
    env -i PATH="$PATH" HOME="${HOME-}" GITHUB_TOKEN="$GITHUB_TOKEN" \
      gh release download "$prior_tag" --repo "$prior_repository" \
      --pattern "$asset" --dir "$download_dir" \
      || fail prior_evidence_absent "exact ${prior_phase} asset is absent: ${asset}"
    downloaded_asset="$download_dir/$asset"
    [[ -f "$downloaded_asset" && ! -L "$downloaded_asset" ]] \
      || fail prior_evidence_absent "downloaded ${prior_phase} asset is invalid: ${asset}"
    actual_asset_digest=$(sha256_file "$downloaded_asset")
    [[ "$actual_asset_digest" == "$asset_digest" ]] \
      || fail prior_evidence_unverifiable "server digest differs for ${asset}"
    [[ -z "$archive_pin" || "$actual_asset_digest" == "sha256:$archive_pin" ]] \
      || fail prior_archive_digest_mismatch \
        "${prior_phase} archive differs from its pin: ${asset}"
  done

  asset_digest=$(jq -er --arg name "$CHECKSUMS_NAME" \
    '[.assets[] | select(.name == $name)] | select(length == 1) | .[0] |
     select(.state == "uploaded") | .digest |
     select(type == "string" and test("^sha256:[0-9a-f]{64}$"))' "$release_json") \
    || fail prior_evidence_malformed 'exact checksums.txt metadata is missing'
  env -i PATH="$PATH" HOME="${HOME-}" GITHUB_TOKEN="$GITHUB_TOKEN" \
    gh release download "$prior_tag" --repo "$prior_repository" \
    --pattern "$CHECKSUMS_NAME" --dir "$download_dir" \
    || fail prior_evidence_absent "exact ${prior_phase} asset is absent: ${CHECKSUMS_NAME}"
  checksums="$download_dir/$CHECKSUMS_NAME"
  [[ -f "$checksums" && ! -L "$checksums" ]] \
    || fail prior_evidence_absent "downloaded ${prior_phase} checksums.txt is invalid"
  [[ "$(sha256_file "$checksums")" == "$asset_digest" ]] \
    || fail prior_evidence_unverifiable 'server digest differs for checksums.txt'
  [[ "$(sha256_file "$checksums")" == "sha256:$prior_checksums" ]] \
    || fail prior_checksums_bytes_mismatch "checksums.txt differs from its ${prior_phase} pin"

  for asset in "${archive_assets[@]}"; do
    checksum_line=$(grep -E "^[0-9a-f]{64}  ${asset}$" "$checksums") \
      || fail prior_checksums_malformed "checksum entry is absent for ${asset}"
    [[ "$(grep -Ec "^[0-9a-f]{64}  ${asset}$" "$checksums")" == '1' ]] \
      || fail prior_checksums_malformed "checksum entry is not unique for ${asset}"
    [[ "$(sha256_file "$download_dir/$asset")" == "sha256:${checksum_line%% *}" ]] \
      || fail prior_archive_checksum_mismatch "archive differs from checksums.txt: ${asset}"
  done

  extract_bundle "$download_dir/$darwin_amd64_asset" "$temp_dir/amd64" amd64 \
    "$prior_amd64_manifest"
  extract_bundle "$download_dir/$darwin_arm64_asset" "$temp_dir/arm64" arm64 \
    "$prior_arm64_manifest"
  prior_receipt="$temp_dir/amd64/$RECEIPT_NAME"
  prior_signature="$temp_dir/amd64/$SIGNATURE_NAME"
  cmp -- "$prior_receipt" "$temp_dir/arm64/$RECEIPT_NAME" \
    || fail prior_receipt_bytes_mismatch "${prior_phase} architecture receipt bytes differ"
  cmp -- "$prior_signature" "$temp_dir/arm64/$SIGNATURE_NAME" \
    || fail prior_signature_bytes_mismatch "${prior_phase} architecture signature bytes differ"
}

verify_a22_bridge_lineage_assets() {
  local download_dir="$temp_dir/a22-downloads" asset expected api_digest destination
  local checksum_line archive archive_lineage archive_lineage_signature receipt_bundle
  install -m 0700 -d "$download_dir"
  while read -r asset expected; do
    api_digest=$(jq -er --arg name "$asset" \
      '[.assets[] | select(.name == $name)] | select(length == 1) | .[0] |
       select(.state == "uploaded") | .digest |
       select(type == "string" and test("^sha256:[0-9a-f]{64}$"))' "$release_json") ||
      fail prior_evidence_malformed "exact A22 asset metadata is missing for ${asset}"
    [[ "$api_digest" == "sha256:$expected" ]] || fail prior_archive_digest_mismatch "A22 API digest differs for ${asset}"
    destination="$download_dir/$asset"
    env -i PATH="$PATH" HOME="${HOME-}" GITHUB_TOKEN="$GITHUB_TOKEN" \
      gh release download "$prior_tag" --repo "$prior_repository" \
      --pattern "$asset" --dir "$download_dir" ||
      fail prior_evidence_absent "exact A22 asset is absent: ${asset}"
    [[ -f "$destination" && ! -L "$destination" ]] || fail prior_evidence_absent "downloaded A22 asset is invalid: ${asset}"
    [[ "$(sha256_file "$destination")" == "$api_digest" ]] || fail prior_evidence_unverifiable "A22 downloaded bytes differ for ${asset}"
  done <<A22_ASSETS
checksums.txt $A22_CHECKSUMS_SHA256
checksums.txt.bundle $A22_CHECKSUMS_BUNDLE_SHA256
checksums.txt.signatures $A22_CHECKSUMS_SIGNATURES_SHA256
autopus-adk_0.50.109_darwin_arm64.tar.gz $A22_ARM64_ARCHIVE_SHA256
omp-context-bridge-release.v1.json $A22_BRIDGE_MANIFEST_SHA256
release-lineage-v1.json $A22_LINEAGE_SHA256
release-lineage-v1.sig $A22_LINEAGE_SIGNATURE_SHA256
A22_ASSETS
  archive="$download_dir/autopus-adk_0.50.109_darwin_arm64.tar.gz"
  checksum_line=$(grep -E \
    '^[0-9a-f]{64}  autopus-adk_0\.50\.109_darwin_arm64\.tar\.gz$' "$download_dir/checksums.txt") ||
    fail prior_checksums_malformed 'A22 arm64 checksum entry is absent'
  [[ "$(grep -Ec '^[0-9a-f]{64}  autopus-adk_0\.50\.109_darwin_arm64\.tar\.gz$' \
      "$download_dir/checksums.txt")" == '1' &&
     "${checksum_line%% *}" == "$A22_ARM64_ARCHIVE_SHA256" ]] \
    || fail prior_archive_checksum_mismatch 'A22 arm64 archive differs from checksums.txt'
  extract_bundle "$archive" "$temp_dir/a22-arm64" arm64 "$A22_ARM64_MANIFEST_SHA256"
  prior_receipt="$temp_dir/a22-arm64/$RECEIPT_NAME" prior_signature="$temp_dir/a22-arm64/$SIGNATURE_NAME"
  [[ "$(sha256_file "$temp_dir/a22-arm64/$MANIFEST_SIGNATURE_NAME")" == \
     "sha256:$A22_ARM64_MANIFEST_SIGNATURE_SHA256" ]] \
    || fail prior_manifest_signature_mismatch 'A22 companion signature differs from its pin'
  archive_lineage="$temp_dir/a22-archive-lineage.json" archive_lineage_signature="$temp_dir/a22-archive-lineage.sig"
  tar -xOzf "$archive" release-lineage-v1.json >"$archive_lineage" \
    || fail prior_evidence_absent 'A22 archived lineage is absent'
  tar -xOzf "$archive" release-lineage-v1.sig >"$archive_lineage_signature" \
    || fail prior_evidence_absent 'A22 archived lineage signature is absent'
  cmp -- "$download_dir/release-lineage-v1.json" "$archive_lineage" \
    || fail prior_evidence_unverifiable 'A22 standalone and archived lineage differ'
  cmp -- "$download_dir/release-lineage-v1.sig" "$archive_lineage_signature" \
    || fail prior_evidence_unverifiable 'A22 standalone and archived lineage signatures differ'
  receipt_bundle="$temp_dir/a22-receipt-bundle"
  install -m 0700 -d "$receipt_bundle"
  install -m 0600 "$prior_receipt" "$prior_signature" "$receipt_bundle"
  env -i PATH="$PATH" HOME="${HOME-}" TMPDIR="${TMPDIR:-/tmp}" \
    "$OMP_CONTEXT_LINEAGE_VERIFIER" \
    --lineage "$download_dir/release-lineage-v1.json" \
    --signature "$download_dir/release-lineage-v1.sig" \
    --receipt-bundle "$receipt_bundle" --key-id "$COMPANION_KEY_ID" \
    --handoff "$COMPANION_HANDOFF" --minimum-rollback-floor "$COMPANION_ROLLBACK_FLOOR" \
    --upstream-sha256 "sha256:$A22_UPSTREAM_SHA256" \
    --executable-sha256 "sha256:$A22_EXECUTABLE_SHA256" \
    --source-repository "$prior_repository" --source-commit "$prior_commit" \
    --source-tree "$prior_tree" --target darwin-arm64 --version "$prior_version" \
    || fail prior_evidence_unverifiable 'A22 bridge lineage signature or coordinates differ'
}
