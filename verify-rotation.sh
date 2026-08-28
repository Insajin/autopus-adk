#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() { printf 'rotation authority: %s\n' "$1" >&2; exit 1; }
usage() {
  printf '%s\n' 'usage: verify-rotation.sh verify-rotation[ -historical] --document PATH --signature PATH --source-commit OID --source-tree OID --next-tag-public-key PATH --next-tag-fingerprint PATH --next-promotion-public-key PATH' >&2
  exit 64
}

[[ $# -ge 1 ]] || usage
command_name=$1
shift
case "$command_name" in
  verify-rotation) historical=0 ;;
  verify-rotation-historical) historical=1 ;;
  *) usage ;;
esac
while [[ $# -gt 0 ]]; do
  [[ $# -ge 2 ]] || usage
  case "$1" in
    --document) [[ "${document+x}" != x ]] || usage; document=$2 ;;
    --signature) [[ "${signature+x}" != x ]] || usage; signature=$2 ;;
    --source-commit) [[ "${source_commit+x}" != x ]] || usage; source_commit=$2 ;;
    --source-tree) [[ "${source_tree+x}" != x ]] || usage; source_tree=$2 ;;
    --next-tag-public-key) [[ "${tag_public_path+x}" != x ]] || usage; tag_public_path=$2 ;;
    --next-tag-fingerprint) [[ "${tag_fingerprint_path+x}" != x ]] || usage; tag_fingerprint_path=$2 ;;
    --next-promotion-public-key) [[ "${promotion_public_path+x}" != x ]] || usage; promotion_public_path=$2 ;;
    *) usage ;;
  esac
  shift 2
done
[[ ${document+x} == x && -n "${document-}" && ${signature+x} == x && -n "${signature-}" &&
   ${source_commit+x} == x && -n "${source_commit-}" && ${source_tree+x} == x && -n "${source_tree-}" &&
   ${tag_public_path+x} == x && -n "${tag_public_path-}" &&
   ${tag_fingerprint_path+x} == x && -n "${tag_fingerprint_path-}" &&
   ${promotion_public_path+x} == x && -n "${promotion_public_path-}" ]] || usage
for tool in cat jq mktemp openssl ssh-keygen; do
  command -v "$tool" >/dev/null 2>&1 || fail "$tool is unavailable"
done
script_dir=$(cd -P -- "${BASH_SOURCE[0]%/*}" && pwd -P) || fail 'cannot resolve authority directory'
policy="$script_dir/adk-key-rotation-authority.v1.json"
verification_root=$(mktemp -d "${TMPDIR:-/tmp}/adk-rotation-verify.XXXXXX") ||
  fail 'cannot create rotation verification workspace'
trap 'rm -rf -- "$verification_root"' EXIT
for path in "$policy" "$document" "$signature" "$tag_public_path" "$tag_fingerprint_path" "$promotion_public_path"; do
  [[ -f "$path" && ! -L "$path" ]] || fail 'an input is not a regular non-symlink file'
done
[[ "$source_commit" =~ ^[0-9a-f]{40}$ && "$source_tree" =~ ^[0-9a-f]{40}$ ]] ||
  fail 'source coordinates are malformed'

jq -en --rawfile raw "$policy" '
  try (($raw | fromjson) as $p |
    ($p | keys_unsorted) == [
      "authority_schema","rotation_schema","repository","channel","signature_domain",
      "bridge_tag","release_mode","channel_key_id","channel_public_key",
      "previous_tag_fingerprint","next_tag_public_key","next_tag_fingerprint",
      "next_promotion_key_id","next_promotion_public_key",
      "next_promotion_public_key_sha256","max_validity_seconds","verifier_sha256"
    ] and ($p | to_entries | all(.[]; (.value | type) == "string")) and
    $p == {
      authority_schema:"adk-key-rotation-authority.v1",
      rotation_schema:"adk-key-rotation.v1",repository:"Insajin/autopus-adk",channel:"stable",
      signature_domain:"autopus.adk-channel.key-rotation.v1\u0000",bridge_tag:"v0.50.109",
      release_mode:"canonical-full-bridge",channel_key_id:"adk-channel-2026-q3-a0",
      channel_public_key:"1IqFilCntaMPUxg7ndOZnyy6Lj1NBQXkXBJp3rEu6kI=",
      previous_tag_fingerprint:"SHA256:bhW+YA+FZ6G4d9Z8BM/eBss6l0I/fcVmV7k986GupK0",
      next_tag_public_key:"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIPKdXtl0E+TcLmC94idkTgtM5XUA5UqP9An0vNFp0FlY",
      next_tag_fingerprint:"SHA256:7FISPXCi8p7cFEdh4Fcyyp8RPQbXYZwmo3Mxi5+YjrQ",
      next_promotion_key_id:"omp-context-promotion-2026-q3-k3",
      next_promotion_public_key:"YkTuNcfWGTLgTglPmZq/Dj4OXwcoUwnkM2ExIGIz+jM=",
      next_promotion_public_key_sha256:"2a9b41dec1330f65937d9b25b20967cb29fd9209c722ce5fe1a9afd6ca45b937",
      max_validity_seconds:"86400",verifier_sha256:$p.verifier_sha256
    } and ($p.verifier_sha256 | test("^[0-9a-f]{64}$")) and $raw == ($p | tojson)) catch false
' >/dev/null || fail 'authority policy bytes are not exact and canonical'

IFS=$'\t' read -r rotation_schema repository channel bridge_tag release_mode channel_key_id \
  channel_public_key previous_fingerprint next_tag_public next_tag_fingerprint \
  next_promotion_key_id next_promotion_public next_promotion_sha max_validity verifier_expected_sha < <(
  jq -r '[.rotation_schema,.repository,.channel,.bridge_tag,.release_mode,.channel_key_id,
    .channel_public_key,.previous_tag_fingerprint,.next_tag_public_key,.next_tag_fingerprint,
    .next_promotion_key_id,.next_promotion_public_key,.next_promotion_public_key_sha256,
    .max_validity_seconds,.verifier_sha256] | @tsv' "$policy"
) || fail 'cannot read authority policy'
verifier_digest=$(openssl dgst -sha256 "${BASH_SOURCE[0]}") || fail 'cannot hash authority verifier'
[[ "${verifier_digest##* }" == "$verifier_expected_sha" ]] || fail 'authority verifier digest differs from policy'
[[ ${#channel_public_key} -eq 44 && "$channel_public_key" == *= &&
   "$(printf '%s' "$channel_public_key" | openssl base64 -d -A 2>/dev/null | openssl base64 -A)" == "$channel_public_key" ]] ||
  fail 'channel public key is not canonical Ed25519 base64'
[[ "$next_tag_public" =~ ^ssh-ed25519[[:space:]][A-Za-z0-9+/]+={0,2}$ ]] ||
  fail 'next tag public key is malformed'
tag_blob=${next_tag_public#ssh-ed25519 }
[[ "$(printf '%s' "$tag_blob" | openssl base64 -d -A 2>/dev/null | openssl base64 -A)" == "$tag_blob" ]] ||
  fail 'next tag public key is not canonical base64'
tag_fingerprint_line=$(ssh-keygen -lf <(printf '%s\n' "$next_tag_public") -E sha256 2>/dev/null) ||
  fail 'next tag public key is not Ed25519'
case "$tag_fingerprint_line" in "256 $next_tag_fingerprint "*) ;; *) fail 'next tag fingerprint differs' ;; esac
[[ ${#next_promotion_public} -eq 44 && "$next_promotion_public" == *= &&
   "$(printf '%s' "$next_promotion_public" | openssl base64 -d -A 2>/dev/null | openssl base64 -A)" == "$next_promotion_public" ]] ||
  fail 'next promotion public key is not canonical Ed25519 base64'
promotion_digest=$(openssl dgst -sha256 <(printf '%s' "$next_promotion_public" | openssl base64 -d -A) 2>/dev/null) ||
  fail 'cannot hash next promotion public key'
[[ "${promotion_digest##* }" == "$next_promotion_sha" ]] || fail 'next promotion public key digest differs'

jq -en --rawfile raw "$tag_public_path" --arg expected "$next_tag_public" '$raw == ($expected + "\n")' >/dev/null ||
  fail 'next tag public key file differs from authority policy'
jq -en --rawfile raw "$tag_fingerprint_path" --arg expected "$next_tag_fingerprint" '$raw == ($expected + "\n")' >/dev/null ||
  fail 'next tag fingerprint file differs from authority policy'
jq -en --rawfile raw "$promotion_public_path" --arg expected "$next_promotion_public" '$raw == ($expected + "\n")' >/dev/null ||
  fail 'next promotion public key file differs from authority policy'

jq -en --rawfile raw "$document" --arg schema "$rotation_schema" --arg repository "$repository" \
  --arg channel "$channel" --arg bridge_tag "$bridge_tag" --arg release_mode "$release_mode" \
  --arg source_commit "$source_commit" --arg source_tree "$source_tree" --arg channel_key_id "$channel_key_id" \
  --arg previous_fingerprint "$previous_fingerprint" --arg next_tag_public "$next_tag_public" \
  --arg next_tag_fingerprint "$next_tag_fingerprint" --arg next_promotion_key_id "$next_promotion_key_id" \
  --arg next_promotion_public "$next_promotion_public" --arg next_promotion_sha "$next_promotion_sha" \
  --argjson max_validity "$max_validity" --argjson historical "$historical" '
  try (($raw | fromjson) as $d |
    ($raw | utf8bytelength) > 0 and ($raw | utf8bytelength) <= 8192 and
    ($d | keys_unsorted) == ["schema_version","channel","repository","bridge_tag","release_mode",
      "source_commit","source_tree","issued_at","expires_at","channel_key_id",
      "previous_tag_fingerprint","next_tag_public_key","next_tag_fingerprint",
      "next_promotion_key_id","next_promotion_public_key","next_promotion_public_key_sha256"] and
    ($d | to_entries | all(.[]; (.value | type) == "string")) and
    $raw == ({schema_version:$d.schema_version,channel:$d.channel,repository:$d.repository,
      bridge_tag:$d.bridge_tag,release_mode:$d.release_mode,source_commit:$d.source_commit,
      source_tree:$d.source_tree,issued_at:$d.issued_at,expires_at:$d.expires_at,
      channel_key_id:$d.channel_key_id,previous_tag_fingerprint:$d.previous_tag_fingerprint,
      next_tag_public_key:$d.next_tag_public_key,next_tag_fingerprint:$d.next_tag_fingerprint,
      next_promotion_key_id:$d.next_promotion_key_id,next_promotion_public_key:$d.next_promotion_public_key,
      next_promotion_public_key_sha256:$d.next_promotion_public_key_sha256} | tojson) and
    $d.schema_version == $schema and $d.repository == $repository and $d.channel == $channel and
    $d.bridge_tag == $bridge_tag and $d.release_mode == $release_mode and
    $d.source_commit == $source_commit and $d.source_tree == $source_tree and
    $d.channel_key_id == $channel_key_id and $d.previous_tag_fingerprint == $previous_fingerprint and
    $d.next_tag_public_key == $next_tag_public and $d.next_tag_fingerprint == $next_tag_fingerprint and
    $d.next_promotion_key_id == $next_promotion_key_id and
    $d.next_promotion_public_key == $next_promotion_public and
    $d.next_promotion_public_key_sha256 == $next_promotion_sha and
    ($d.issued_at | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$")) and
    ($d.expires_at | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$")) and
    (($d.issued_at | fromdateiso8601) as $issued | ($d.expires_at | fromdateiso8601) as $expires |
      ($issued | todateiso8601) == $d.issued_at and ($expires | todateiso8601) == $d.expires_at and
      $issued < $expires and ($expires - $issued) <= $max_validity and
      ($historical == 1 or (now >= $issued and now < $expires)))) catch false
' >/dev/null || fail 'rotation document is not exact, canonical, pinned, and admissible'

signature_b64=$(openssl base64 -A -in "$signature") || fail 'cannot read rotation signature'
[[ ${#signature_b64} -eq 88 && "$signature_b64" == *== ]] || fail 'rotation signature is not 64 raw bytes'
public_der="$verification_root/channel-public.der"
message="$verification_root/message"
{ printf '\060\052\060\005\006\003\053\145\160\003\041\000'
  printf '%s' "$channel_public_key" | openssl base64 -d -A
} >"$public_der"
{ printf 'autopus.adk-channel.key-rotation.v1\000'
  cat "$document"
} >"$message"
if ! openssl pkeyutl -verify -pubin -keyform DER -inkey "$public_der" \
  -sigfile "$signature" -rawin -in "$message" \
  >/dev/null 2>&1; then
  fail 'A0 rotation signature verification failed'
fi
openssl base64 -A -in "$document" | openssl base64 -d -A
