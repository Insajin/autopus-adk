#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
prep="$script_dir/prepare-release.sh"
prep_lib="$script_dir/prepare-release-runtime-lib.sh"
publisher="$script_dir/publish-release-coordinates.sh"
transaction="$script_dir/release-coordinate-transaction-lib.sh"
sidecar="$script_dir/verify-key-rotation-sidecar.sh"
authority_materializer="$script_dir/materialize-key-rotation-authority.sh"
lock_verifier="$script_dir/verify-release-prep-lock.sh"
manifest="$script_dir/produce-omp-context-bridge-manifest.sh"
fail() { printf 'release prep hardening test: %s\n' "$1" >&2; exit 1; }
contains() { grep -Fq -- "$2" "$1" || fail "$1 missing $2"; }
not_contains() { if grep -Fq -- "$2" "$1"; then fail "$1 contains forbidden $2"; fi; }

for file in "$prep" "$prep_lib" "$publisher" "$transaction" "$sidecar" \
  "$authority_materializer" "$lock_verifier" "$manifest" \
  "$script_dir/release-tag-signing-2026-q3.pub" \
  "$script_dir/release-tag-signing-2026-q3.fingerprint" \
  "$script_dir/release-tag-signing-2026-q3-r2.pub" \
  "$script_dir/release-tag-signing-2026-q3-r2.fingerprint" \
  "$script_dir/omp-context-promotion-2026-q3-k3.pub"
do
  [[ -f "$file" && ! -L "$file" ]] || fail "missing or unsafe release component $file"
done
[[ ! -e "$script_dir/prepare-release-local-lib.sh" ]] || fail 'obsolete promotion prep helper remains'
[[ ! -e "$script_dir/verify-omp-context-evidence-tag.sh" ]] || fail 'A22 evidence tag reader remains'

contains "$prep" 'usage: prepare-release.sh --tag-signing-key PATH [--rotation-document PATH --rotation-signature PATH] [--apply]'
for forbidden in '--endpoint' '--credential-locator' '--provider' '--model' \
  '--model-context-window' '--omp' '--oracle-policy-digest' '--promotion-signing-key' \
  'expected_promotion_key_id' 'static_policy' 'evidence_tag' 'run_canary'
do
  not_contains "$prep" "$forbidden"
done
for required in \
  'source worktree is not clean' \
  'source is not exact origin/main' \
  'verify-key-rotation-sidecar.sh' \
  'materialize-key-rotation-authority.sh' \
  'AUTOPUS_ADK_CHANNEL_KEY_ID' \
  'AUTOPUS_ADK_CHANNEL_PUBLIC_KEY' \
  'release-tag-signing-2026-q3-r2.pub' \
  'release-tag-signing-2026-q3-r2.fingerprint' \
  'build_bridge_candidate' \
  'produce_bridge_manifest' \
  'remote_mutations:0'
do
  contains "$prep" "$required"
done
not_contains "$prep" './internal/adkchannel/cmd'

for required in \
  'refs/heads/release-key-rotation-v0.50.109' \
  'adk-key-rotation-v1.json' 'adk-key-rotation-v1.sig' \
  'verify-rotation' '--source-commit' '--source-tree' \
  '--next-tag-public-key scripts/companion-release/release-tag-signing-2026-q3-r2.pub' \
  '--next-promotion-public-key scripts/companion-release/omp-context-promotion-2026-q3-k3.pub' \
  'git rev-list --parents -n 1 "$rotation_ref_commit"' \
  'rotation distribution commit is not orphaned' \
  'cmp -s "$document" "$verified_document"' \
  'rotation distribution ref changed during verification'
do
  contains "$sidecar" "$required"
done
sidecar_temp=$(mktemp -d "${TMPDIR:-/tmp}/rotation-sidecar-orphan-test.XXXXXX")
trap 'rm -rf -- "$sidecar_temp"' EXIT
real_git=$(command -v git)
distribution="$sidecar_temp/distribution"
work="$sidecar_temp/work"
mkdir -m 0700 "$sidecar_temp/bin"
"$real_git" init -q "$distribution"
"$real_git" -C "$distribution" config user.name rotation-sidecar-test
"$real_git" -C "$distribution" config user.email rotation-sidecar-test@example.invalid
printf '{}' >"$distribution/adk-key-rotation-v1.json"
printf '%064d' 0 >"$distribution/adk-key-rotation-v1.sig"
"$real_git" -C "$distribution" add .
distribution_tree=$("$real_git" -C "$distribution" write-tree)
orphan_commit=$(printf orphan | "$real_git" -C "$distribution" commit-tree "$distribution_tree")
parented_commit=$(printf parented | "$real_git" -C "$distribution" \
  commit-tree "$distribution_tree" -p "$orphan_commit")
rotation_ref='refs/heads/release-key-rotation-v0.50.109'
"$real_git" -C "$distribution" update-ref "$rotation_ref" "$parented_commit"
"$real_git" init -q "$work"
"$real_git" -C "$work" remote add origin 'https://github.com/Insajin/autopus-adk'
mkdir -m 0700 -p "$work/scripts/companion-release"
cat >"$work/scripts/companion-release/verify-rotation-ref-ruleset.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod 0700 "$work/scripts/companion-release/verify-rotation-ref-ruleset.sh"
cat >"$sidecar_temp/bin/git" <<EOF
#!/usr/bin/env bash
if [[ "\${1-}" == 'ls-remote' || "\${1-}" == 'fetch' ]]; then
  args=()
  for arg in "\$@"; do
    [[ "\$arg" != origin ]] || arg='$distribution'
    args+=("\$arg")
  done
  exec '$real_git' "\${args[@]}"
fi
exec '$real_git' "\$@"
EOF
cat >"$sidecar_temp/verifier" <<EOF
#!/usr/bin/env bash
printf '%s\n' "\$*" >'$sidecar_temp/verifier-called'
exit 93
EOF
chmod 0700 "$sidecar_temp/bin/git" "$sidecar_temp/verifier"
source_commit='1111111111111111111111111111111111111111'
source_tree='2222222222222222222222222222222222222222'
if (cd "$work" && PATH="$sidecar_temp/bin:$PATH" "$sidecar" --historical \
  "$sidecar_temp/verifier" "$source_commit" "$source_tree" "$sidecar_temp/parented-output") \
  >/dev/null 2>&1; then
  fail 'parented rotation distribution commit was accepted'
fi
[[ ! -e "$sidecar_temp/verifier-called" ]] ||
  fail 'parented rotation distribution reached the external verifier'
if (cd "$work" && PATH="$sidecar_temp/bin:$PATH" "$sidecar" --public-ruleset --historical \
  "$sidecar_temp/verifier" "$source_commit" "$source_tree" "$sidecar_temp/public-parented-output") \
  >/dev/null 2>&1; then
  fail 'public-ruleset mode accepted a parented rotation distribution commit'
fi
[[ ! -e "$sidecar_temp/verifier-called" ]] ||
  fail 'public-ruleset parented rotation distribution reached the external verifier'
"$real_git" -C "$distribution" update-ref "$rotation_ref" "$orphan_commit"
if (cd "$work" && PATH="$sidecar_temp/bin:$PATH" "$sidecar" --historical \
  "$sidecar_temp/verifier" "$source_commit" "$source_tree" "$sidecar_temp/orphan-output") \
  >/dev/null 2>&1; then
  fail 'external verifier rejection was ignored'
fi
[[ -f "$sidecar_temp/verifier-called" ]] ||
  fail 'canonical orphan did not reach the external verifier boundary'
IFS= read -r verifier_call <"$sidecar_temp/verifier-called"
[[ "$verifier_call" == verify-rotation-historical\ * ]] ||
  fail 'canonical orphan reached the wrong verifier command'
rm -rf -- "$sidecar_temp"
trap - EXIT

for required in \
  'autopus.adk_release_reservation.v2' \
  "release_mode='canonical-full-bridge'" \
  'candidate_artifact_sha256:$candidate_sha256' \
  'rotation_ref_commit:$rotation_ref_commit' \
  'rotation_document_sha256:$rotation_document_sha256' \
  'promotion_key_id:$promotion_key_id' \
  'names=(ADK_COMPANION_APPROVED_SOURCE_COMMIT ADK_COMPANION_APPROVED_SOURCE_TREE)' \
  'obsolete_names=(OMP_CONTEXT_EVIDENCE_TAG_OBJECT_SHA' \
  'gh variable delete "$name" --repo "$repository"' \
  'verify-key-rotation-sidecar.sh' \
  'The independently authorized R2 tag is the final external mutation.' \
  'git push --atomic --force-with-lease=' \
  'verify_remote_release'
do
  contains "$publisher" "$required"
done
for forbidden in \
  'evidence_tag_object:$evidence_tag_object' \
  'static_policy_sha256' \
  'report_sha256:$report_sha256' \
  'attestation_sha256:$attestation_sha256'
do
  not_contains "$publisher" "$forbidden"
done
sidecar_line=$(grep -nF 'verify-key-rotation-sidecar.sh' "$publisher" | cut -d: -f1)
tag_line=$(grep -nF 'git tag -s "$release_tag"' "$publisher" | cut -d: -f1)
[[ "$sidecar_line" -lt "$tag_line" ]] ||
  fail 'R2 tag creation can run before independent sidecar verification'

contains "$prep" "bridge_lock_ref='refs/heads/release-bridge-v0.50.109-prep-lock'"
contains "$prep_lib" 'verify-release-prep-lock.sh'
contains "$lock_verifier" "expected_ref='refs/heads/release-bridge-v0.50.109-prep-lock'"
contains "$lock_verifier" "manifest_name='omp-context-bridge-release.v1.json'"
contains "$transaction" 'restore_deleted_scope'
contains "$transaction" 'all(.[]; .name != $name)'
contains "$transaction" 'rollback incomplete; prep lock retained for reconciliation'
contains "$manifest" "release_mode='canonical-full-bridge'"
contains "$manifest" 'bridge manifest forbids ${name}'

printf 'release prep hardening test: PASS\n'
