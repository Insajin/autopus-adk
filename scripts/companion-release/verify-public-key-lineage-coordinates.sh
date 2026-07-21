#!/usr/bin/env bash

# @AX:NOTE [AUTO]: Append a phase only after immutable evidence pins exist; every phase trusts its direct predecessor.
readonly A0_REPOSITORY='Insajin/autopus-adk' A0_TAG='v0.50.69' A0_VERSION='0.50.69'
readonly A1_REPOSITORY='Insajin/autopus-adk' A1_TAG='v0.50.70' A1_VERSION='0.50.70'
readonly A2_REPOSITORY='Insajin/autopus-adk' A2_TAG='v0.50.71' A2_VERSION='0.50.71'
readonly A3_TAG='v0.50.72' A3_VERSION='0.50.72'
readonly A3_REPOSITORY='Insajin/autopus-adk' A4_TAG='v0.50.73' A4_VERSION='0.50.73'
readonly A4_REPOSITORY='Insajin/autopus-adk' A5_TAG='v0.50.74' A5_VERSION='0.50.74'
readonly A5_REPOSITORY='Insajin/autopus-adk' A6_TAG='v0.50.77' A6_VERSION='0.50.77'
readonly A6_REPOSITORY='Insajin/autopus-adk' A7_TAG='v0.50.78' A7_VERSION='0.50.78'
readonly A7_REPOSITORY='Insajin/autopus-adk' A8_TAG='v0.50.79' A8_VERSION='0.50.79'
readonly A8_REPOSITORY='Insajin/autopus-adk' A9_TAG='v0.50.80' A9_VERSION='0.50.80'
readonly A9_REPOSITORY='Insajin/autopus-adk' A10_TAG='v0.50.81' A10_VERSION='0.50.81'
readonly A10_REPOSITORY='Insajin/autopus-adk' A11_TAG='v0.50.82' A11_VERSION='0.50.82'
readonly A0_EVIDENCE_SOURCE='immutable A0 GitHub release'

require_environment GITHUB_REF_NAME
COMPANION_VERSION="${GITHUB_REF_NAME#v}"
prior_tree=''
if [[ "$GITHUB_REF_NAME" == 'v0.50.69' && "$COMPANION_VERSION" == '0.50.69' ]]; then
  release_phase='A0'
  printf 'companion release lineage: %s bootstrap accepted for %s@%s\n' "$release_phase" "$A0_REPOSITORY" "$A0_TAG"
  exit 0
elif [[ "$GITHUB_REF_NAME" == "$A1_TAG" && "$COMPANION_VERSION" == "$A1_VERSION" ]]; then
  release_phase='A1' prior_phase='A0' prior_repository="$A0_REPOSITORY" prior_evidence_source="$A0_EVIDENCE_SOURCE"
  prior_tag="$A0_TAG" prior_version="$A0_VERSION" prior_commit="$A0_COMMIT_SHA"
  prior_tag_object='' prior_checksums="$A0_CHECKSUMS_SHA256" prior_amd64_archive='' prior_arm64_archive=''
  prior_amd64_manifest="$A0_AMD64_MANIFEST_SHA256" prior_arm64_manifest="$A0_ARM64_MANIFEST_SHA256"
elif [[ "$GITHUB_REF_NAME" == "$A2_TAG" && "$COMPANION_VERSION" == "$A2_VERSION" ]]; then
  release_phase='A2' prior_phase='A1' prior_repository="$A1_REPOSITORY" prior_evidence_source='immutable A1 GitHub release'
  prior_tag="$A1_TAG" prior_version="$A1_VERSION" prior_commit="$A1_COMMIT_SHA"
  prior_tag_object="$A1_TAG_OBJECT_SHA" prior_checksums="$A1_CHECKSUMS_SHA256" prior_amd64_archive="$A1_AMD64_ARCHIVE_SHA256" prior_arm64_archive="$A1_ARM64_ARCHIVE_SHA256"
  prior_amd64_manifest="$A1_AMD64_MANIFEST_SHA256" prior_arm64_manifest="$A1_ARM64_MANIFEST_SHA256"
elif [[ "$GITHUB_REF_NAME" == "$A3_TAG" && "$COMPANION_VERSION" == "$A3_VERSION" ]]; then
  release_phase='A3' prior_phase='A2' prior_repository="$A2_REPOSITORY" prior_evidence_source='immutable A2 GitHub release'
  prior_tag="$A2_TAG" prior_version="$A2_VERSION" prior_commit="$A2_COMMIT_SHA"
  prior_tag_object="$A2_TAG_OBJECT_SHA" prior_checksums="$A2_CHECKSUMS_SHA256" prior_amd64_archive="$A2_AMD64_ARCHIVE_SHA256" prior_arm64_archive="$A2_ARM64_ARCHIVE_SHA256"
  prior_amd64_manifest="$A2_AMD64_MANIFEST_SHA256" prior_arm64_manifest="$A2_ARM64_MANIFEST_SHA256"
elif [[ "$GITHUB_REF_NAME" == "$A4_TAG" && "$COMPANION_VERSION" == "$A4_VERSION" ]]; then
  release_phase='A4' prior_phase='A3' prior_repository="$A3_REPOSITORY" prior_evidence_source='immutable A3 GitHub release' prior_tag="$A3_TAG" prior_version="$A3_VERSION" prior_commit="$A3_COMMIT_SHA"
  prior_tag_object="$A3_TAG_OBJECT_SHA" prior_checksums="$A3_CHECKSUMS_SHA256" prior_amd64_archive="$A3_AMD64_ARCHIVE_SHA256" prior_arm64_archive="$A3_ARM64_ARCHIVE_SHA256" prior_amd64_manifest="$A3_AMD64_MANIFEST_SHA256" prior_arm64_manifest="$A3_ARM64_MANIFEST_SHA256"
elif [[ "$GITHUB_REF_NAME" == "$A5_TAG" && "$COMPANION_VERSION" == "$A5_VERSION" ]]; then
  release_phase='A5' prior_phase='A4' prior_repository="$A4_REPOSITORY" prior_evidence_source='immutable A4 GitHub release' prior_tag="$A4_TAG" prior_version="$A4_VERSION" prior_commit="$A4_COMMIT_SHA"
  prior_tag_object="$A4_TAG_OBJECT_SHA" prior_checksums="$A4_CHECKSUMS_SHA256" prior_amd64_archive="$A4_AMD64_ARCHIVE_SHA256" prior_arm64_archive="$A4_ARM64_ARCHIVE_SHA256" prior_amd64_manifest="$A4_AMD64_MANIFEST_SHA256" prior_arm64_manifest="$A4_ARM64_MANIFEST_SHA256"
elif [[ "$GITHUB_REF_NAME" == "$A6_TAG" && "$COMPANION_VERSION" == "$A6_VERSION" ]]; then
  release_phase='A6' prior_phase='A5' prior_repository="$A5_REPOSITORY" prior_evidence_source='immutable A5 GitHub release' prior_tag="$A5_TAG" prior_version="$A5_VERSION" prior_commit="$A5_COMMIT_SHA"
  prior_tag_object="$A5_TAG_OBJECT_SHA" prior_checksums="$A5_CHECKSUMS_SHA256" prior_amd64_archive="$A5_AMD64_ARCHIVE_SHA256" prior_arm64_archive="$A5_ARM64_ARCHIVE_SHA256" prior_amd64_manifest="$A5_AMD64_MANIFEST_SHA256" prior_arm64_manifest="$A5_ARM64_MANIFEST_SHA256"
elif [[ "$GITHUB_REF_NAME" == "$A7_TAG" && "$COMPANION_VERSION" == "$A7_VERSION" ]]; then
  release_phase='A7' prior_phase='A6' prior_repository="$A6_REPOSITORY" prior_evidence_source='immutable A6 GitHub release' prior_tag="$A6_TAG" prior_version="$A6_VERSION" prior_commit="$A6_COMMIT_SHA"
  prior_tag_object="$A6_TAG_OBJECT_SHA" prior_checksums="$A6_CHECKSUMS_SHA256" prior_amd64_archive="$A6_AMD64_ARCHIVE_SHA256" prior_arm64_archive="$A6_ARM64_ARCHIVE_SHA256" prior_amd64_manifest="$A6_AMD64_MANIFEST_SHA256" prior_arm64_manifest="$A6_ARM64_MANIFEST_SHA256"
elif [[ "$GITHUB_REF_NAME" == "$A8_TAG" && "$COMPANION_VERSION" == "$A8_VERSION" ]]; then
  release_phase='A8' prior_phase='A7' prior_repository="$A7_REPOSITORY" prior_evidence_source='immutable A7 GitHub release' prior_tag="$A7_TAG" prior_version="$A7_VERSION" prior_commit="$A7_COMMIT_SHA" prior_tree="$A7_TREE_SHA"
  prior_tag_object="$A7_TAG_OBJECT_SHA" prior_checksums="$A7_CHECKSUMS_SHA256" prior_amd64_archive="$A7_AMD64_ARCHIVE_SHA256" prior_arm64_archive="$A7_ARM64_ARCHIVE_SHA256" prior_amd64_manifest="$A7_AMD64_MANIFEST_SHA256" prior_arm64_manifest="$A7_ARM64_MANIFEST_SHA256"
elif [[ "$GITHUB_REF_NAME" == "$A9_TAG" && "$COMPANION_VERSION" == "$A9_VERSION" ]]; then
  release_phase='A9' prior_phase='A8' prior_repository="$A8_REPOSITORY" prior_evidence_source='immutable A8 GitHub release' prior_tag="$A8_TAG" prior_version="$A8_VERSION" prior_commit="$A8_COMMIT_SHA" prior_tree="$A8_TREE_SHA"
  prior_tag_object="$A8_TAG_OBJECT_SHA" prior_checksums="$A8_CHECKSUMS_SHA256" prior_amd64_archive="$A8_AMD64_ARCHIVE_SHA256" prior_arm64_archive="$A8_ARM64_ARCHIVE_SHA256" prior_amd64_manifest="$A8_AMD64_MANIFEST_SHA256" prior_arm64_manifest="$A8_ARM64_MANIFEST_SHA256"
elif [[ "$GITHUB_REF_NAME" == "$A10_TAG" && "$COMPANION_VERSION" == "$A10_VERSION" ]]; then
  release_phase='A10' prior_phase='A9' prior_repository="$A9_REPOSITORY" prior_evidence_source='immutable A9 GitHub release' prior_tag="$A9_TAG" prior_version="$A9_VERSION" prior_commit="$A9_COMMIT_SHA" prior_tree="$A9_TREE_SHA"
  prior_tag_object="$A9_TAG_OBJECT_SHA" prior_checksums="$A9_CHECKSUMS_SHA256" prior_amd64_archive="$A9_AMD64_ARCHIVE_SHA256" prior_arm64_archive="$A9_ARM64_ARCHIVE_SHA256" prior_amd64_manifest="$A9_AMD64_MANIFEST_SHA256" prior_arm64_manifest="$A9_ARM64_MANIFEST_SHA256"
elif [[ "$GITHUB_REF_NAME" == "$A11_TAG" && "$COMPANION_VERSION" == "$A11_VERSION" ]]; then
  release_phase='A11' prior_phase='A10' prior_repository="$A10_REPOSITORY" prior_evidence_source='immutable A10 GitHub release' prior_tag="$A10_TAG" prior_version="$A10_VERSION" prior_commit="$A10_COMMIT_SHA" prior_tree="$A10_TREE_SHA"
  prior_tag_object="$A10_TAG_OBJECT_SHA" prior_checksums="$A10_CHECKSUMS_SHA256" prior_amd64_archive="$A10_AMD64_ARCHIVE_SHA256" prior_arm64_archive="$A10_ARM64_ARCHIVE_SHA256" prior_amd64_manifest="$A10_AMD64_MANIFEST_SHA256" prior_arm64_manifest="$A10_ARM64_MANIFEST_SHA256"
else
  fail prior_release_identity_mismatch 'release is outside the frozen A0/A1/A2/A3/A4/A5/A6/A7/A8/A9/A10/A11 policy'
fi
