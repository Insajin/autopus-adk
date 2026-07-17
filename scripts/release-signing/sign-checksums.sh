#!/bin/sh

set -eu
umask 077

die() {
    printf 'release signing: %s\n' "$1" >&2
    exit 1
}

[ "$#" -ge 3 ] || die "usage: sign-checksums.sh CHECKSUMS OUTPUT KEY_FILE [KEY_FILE ...]"
[ "$#" -le 18 ] || die "at most 16 signing key files are allowed"

checksums_path=$1
output_path=$2
shift 2

[ -f "$checksums_path" ] && [ ! -L "$checksums_path" ] || die "checksums input must be a regular file"
[ ! -e "$output_path" ] && [ ! -L "$output_path" ] || die "output path already exists"

workdir=$(mktemp -d "${TMPDIR:-/tmp}/autopus-release-signing.XXXXXX") || die "cannot create temporary directory"
cleanup() {
    rm -rf -- "$workdir"
}
trap cleanup EXIT HUP INT TERM

entries="$workdir/entries"
: > "$entries"
index=0
for key_path do
    index=$((index + 1))
    [ -f "$key_path" ] && [ ! -L "$key_path" ] || die "key file $index must be a regular file"

    public_pem="$workdir/public-$index.pem"
    public_der="$workdir/public-$index.der"
    signature_der="$workdir/signature-$index.der"
    curve_text="$workdir/curve-$index.txt"

    openssl ec -in "$key_path" -pubout -text -noout > "$curve_text" 2>/dev/null || \
        die "key file $index is not an EC private key"
    grep -q 'ASN1 OID: prime256v1' "$curve_text" || die "key file $index is not ECDSA P-256 (prime256v1)"
    openssl ec -in "$key_path" -pubout -out "$public_pem" 2>/dev/null || die "cannot derive public key $index"
    openssl ec -in "$key_path" -pubout -outform DER -out "$public_der" 2>/dev/null || die "cannot encode public key $index"

    fingerprint=$(openssl dgst -sha256 "$public_der" | sed 's/^.*= *//')
    [ "${#fingerprint}" -eq 64 ] || die "cannot derive full SPKI fingerprint for key $index"
    case "$fingerprint" in
        *[!0-9a-f]*) die "SPKI fingerprint for key $index is not lowercase hexadecimal" ;;
    esac

    openssl dgst -sha256 -sign "$key_path" -out "$signature_der" "$checksums_path" 2>/dev/null || \
        die "cannot sign checksums with key $index"
    openssl dgst -sha256 -verify "$public_pem" -signature "$signature_der" "$checksums_path" >/dev/null 2>&1 || \
        die "signature self-check failed for key $index"
    signature_base64=$(openssl base64 -A -in "$signature_der")
    [ -n "$signature_base64" ] || die "empty signature for key $index"
    case "$signature_base64" in
        *[!A-Za-z0-9+/=]*) die "signature base64 for key $index is not canonical single-line data" ;;
    esac
    printf '%s\t%s\n' "$fingerprint" "$signature_base64" >> "$entries"
done

sorted="$workdir/entries.sorted"
LC_ALL=C sort "$entries" > "$sorted"
if ! awk -F '\t' 'seen[$1]++ { exit 1 }' "$sorted"; then
    die "duplicate SPKI fingerprint"
fi

envelope="$workdir/checksums.txt.signatures"
printf 'AUTOPUS-RELEASE-SIGNATURE-V1\n' > "$envelope"
cat "$sorted" >> "$envelope"
mv -- "$envelope" "$output_path"
