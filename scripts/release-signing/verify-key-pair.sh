#!/bin/sh

set -eu
umask 077

die() {
    printf 'release signing preflight: %s\n' "$1" >&2
    exit 1
}

[ "$#" -eq 3 ] || die "usage: verify-key-pair.sh PRIVATE_KEY PUBLIC_KEY FINGERPRINT_FILE"
private_key=$1
public_key=$2
fingerprint_file=$3

for path in "$private_key" "$public_key" "$fingerprint_file"; do
    [ -f "$path" ] && [ ! -L "$path" ] || die "inputs must be regular non-symlink files"
done

workdir=$(mktemp -d "${TMPDIR:-/tmp}/autopus-release-preflight.XXXXXX") || die "cannot create temporary directory"
cleanup() {
    rm -rf -- "$workdir"
}
trap cleanup EXIT HUP INT TERM

private_curve="$workdir/private-curve.txt"
public_curve="$workdir/public-curve.txt"
private_der="$workdir/private-public.der"
public_der="$workdir/pinned-public.der"

openssl ec -in "$private_key" -pubout -text -noout > "$private_curve" 2>/dev/null || die "private key is not EC"
openssl ec -pubin -in "$public_key" -text -noout > "$public_curve" 2>/dev/null || die "pinned public key is not EC"
grep -q 'ASN1 OID: prime256v1' "$private_curve" || die "private key is not ECDSA P-256 (prime256v1)"
grep -q 'ASN1 OID: prime256v1' "$public_curve" || die "pinned public key is not ECDSA P-256 (prime256v1)"

openssl ec -in "$private_key" -pubout -outform DER -out "$private_der" 2>/dev/null || die "cannot derive private-key SPKI"
openssl ec -pubin -in "$public_key" -outform DER -out "$public_der" 2>/dev/null || die "cannot encode pinned SPKI"
cmp -s "$private_der" "$public_der" || die "private key does not pair with checked-in K1 public key"

[ "$(wc -l < "$fingerprint_file" | tr -d ' ')" -eq 1 ] || die "fingerprint file must contain exactly one line"
IFS= read -r expected_fingerprint < "$fingerprint_file"
[ "${#expected_fingerprint}" -eq 64 ] || die "checked-in fingerprint must be full SHA-256"
case "$expected_fingerprint" in
    *[!0-9a-f]*) die "checked-in fingerprint must be lowercase hexadecimal" ;;
esac
actual_fingerprint=$(openssl dgst -sha256 "$public_der" | sed 's/^.*= *//')
[ "$actual_fingerprint" = "$expected_fingerprint" ] || die "checked-in K1 fingerprint does not match its SPKI"
