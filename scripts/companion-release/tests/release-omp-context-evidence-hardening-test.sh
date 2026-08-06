#!/usr/bin/env bash
set -euo pipefail
umask 077

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../.." && pwd)
helper="$root/scripts/companion-release/verify-omp-context-evidence-tag.sh"
binary_helper="$root/scripts/companion-release/verify-omp-context-release-binary.sh"
temp=$(mktemp -d "${TMPDIR:-/tmp}/omp-context-evidence-test.XXXXXX")
trap 'rm -rf -- "$temp"' EXIT
repo="$temp/repo"
git init -q "$repo"
git -C "$repo" config user.name 'OMP Evidence Fixture'
git -C "$repo" config user.email 'omp-evidence@example.invalid'
report="$temp/omp-context-promotion-report.v1.json"
attestation="$temp/omp-context-promotion-attestation.v2.json"
printf '{"fixture":"report"}' >"$report"
printf '{"fixture":"attestation"}' >"$attestation"
report_blob=$(git -C "$repo" hash-object -w "$report")
attestation_blob=$(git -C "$repo" hash-object -w "$attestation")
tree=$(printf '100644 blob %s\t%s\n100644 blob %s\t%s\n' \
  "$attestation_blob" 'omp-context-promotion-attestation.v2.json' \
  "$report_blob" 'omp-context-promotion-report.v1.json' | git -C "$repo" mktree)
commit=$(printf 'fixture orphan evidence\n' | \
  GIT_AUTHOR_DATE='2026-08-04T00:00:00Z' GIT_COMMITTER_DATE='2026-08-04T00:00:00Z' \
  git -C "$repo" commit-tree "$tree")
GIT_COMMITTER_DATE='2026-08-04T00:00:01Z' \
  git -C "$repo" tag -am 'fixture evidence' omp-context-evidence-v0.50.97 "$commit"
tag_object=$(git -C "$repo" rev-parse refs/tags/omp-context-evidence-v0.50.97)
report_sha=$(shasum -a 256 "$report" | awk '{print $1}')
attestation_sha=$(shasum -a 256 "$attestation" | awk '{print $1}')

run_helper() {
  local output=$1
  shift
  env OMP_CONTEXT_EVIDENCE_TAG_OBJECT_SHA="$tag_object" \
    OMP_CONTEXT_EVIDENCE_COMMIT_SHA="$commit" \
    OMP_CONTEXT_EVIDENCE_TREE_SHA="$tree" \
    OMP_CONTEXT_EVIDENCE_REPORT_SHA256="$report_sha" \
    OMP_CONTEXT_EVIDENCE_ATTESTATION_SHA256="$attestation_sha" \
    "$@" bash "$helper" "$output"
}

(cd "$repo" && run_helper "$temp/valid")
cmp "$report" "$temp/valid/omp-context-promotion-report.v1.json"
cmp "$attestation" "$temp/valid/omp-context-promotion-attestation.v2.json"

if (cd "$repo" && run_helper "$temp/bad-digest" \
  OMP_CONTEXT_EVIDENCE_REPORT_SHA256="$(printf 'f%.0s' {1..64})"); then
  printf 'tampered report digest passed\n' >&2
  exit 1
fi

git -C "$repo" update-ref refs/tags/omp-context-evidence-v0.50.97 "$commit"
if (cd "$repo" && run_helper "$temp/lightweight" \
  OMP_CONTEXT_EVIDENCE_TAG_OBJECT_SHA="$commit"); then
  printf 'lightweight evidence tag passed\n' >&2
  exit 1
fi

parent=$(printf 'parent\n' | git -C "$repo" commit-tree "$tree")
child=$(printf 'child\n' | git -C "$repo" commit-tree "$tree" -p "$parent")
git -C "$repo" tag -fam 'parented evidence' omp-context-evidence-v0.50.97 "$child"
parented_tag=$(git -C "$repo" rev-parse refs/tags/omp-context-evidence-v0.50.97)
if (cd "$repo" && run_helper "$temp/parented" \
  OMP_CONTEXT_EVIDENCE_TAG_OBJECT_SHA="$parented_tag" \
  OMP_CONTEXT_EVIDENCE_COMMIT_SHA="$child"); then
  printf 'parented evidence commit passed\n' >&2
  exit 1
fi

artifact="$temp/auto"
printf 'canonical candidate bytes' >"$artifact"
artifact_sha=$(shasum -a 256 "$artifact" | awk '{print $1}')
env COMPANION_RELEASE_TAG='v0.50.97' COMPANION_ARTIFACT="$artifact" \
  COMPANION_PLATFORM='darwin' COMPANION_ARCHITECTURE='arm64' \
  OMP_CONTEXT_CANDIDATE_ARTIFACT_SHA256="$artifact_sha" \
  bash "$binary_helper"
if env COMPANION_RELEASE_TAG='v0.50.97' COMPANION_ARTIFACT="$artifact" \
  COMPANION_PLATFORM='darwin' COMPANION_ARCHITECTURE='arm64' \
  OMP_CONTEXT_CANDIDATE_ARTIFACT_SHA256="$(printf '0%.0s' {1..64})" \
  bash "$binary_helper"; then
  printf 'mismatched GoReleaser candidate digest passed\n' >&2
  exit 1
fi

printf 'OMP context evidence hardening: pass\n'
