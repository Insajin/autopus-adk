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
