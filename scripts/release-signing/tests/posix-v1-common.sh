#!/bin/sh

set -eu

TESTS_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$TESTS_DIR/../../.." && pwd)
V1_HELPER="$REPO_ROOT/scripts/release-signing/verify-checksums-v1.sh"
V1_SIGNER="$REPO_ROOT/scripts/release-signing/sign-checksums.sh"
V1_NOW=2027-01-01

v1_test_setup() {
    command -v openssl >/dev/null 2>&1 || {
        printf 'openssl is required for POSIX signing tests\n' >&2
        exit 1
    }
    V1_WORK=$(mktemp -d "${TMPDIR:-/tmp}/autopus-posix-v1.XXXXXX")
    trap 'rm -rf -- "$V1_WORK"' EXIT HUP INT TERM
    . "$V1_HELPER"

    for key in k1 k2 attacker; do
        openssl ecparam -name prime256v1 -genkey -noout \
            -out "$V1_WORK/$key-private.pem" 2>/dev/null
        openssl ec -in "$V1_WORK/$key-private.pem" -pubout -outform DER \
            -out "$V1_WORK/$key-spki.der" 2>/dev/null
        eval "${key}_spki=\$(openssl base64 -A -in \"$V1_WORK/$key-spki.der\")"
        eval "${key}_fp=\$(openssl dgst -sha256 \"$V1_WORK/$key-spki.der\" | sed 's/^.*= *//' | tr '[:upper:]' '[:lower:]')"
    done

    printf '0123456789abcdef  autopus-adk_0.50.73_darwin_arm64.tar.gz\n' \
        > "$V1_WORK/checksums.txt"
    printf '1123456789abcdef  autopus-adk_0.50.73_darwin_arm64.tar.gz\n' \
        > "$V1_WORK/checksums-tampered.txt"
    "$V1_SIGNER" "$V1_WORK/checksums.txt" "$V1_WORK/k1-envelope" \
        "$V1_WORK/k1-private.pem"
    "$V1_SIGNER" "$V1_WORK/checksums.txt" "$V1_WORK/k2-envelope" \
        "$V1_WORK/k2-private.pem"
    "$V1_SIGNER" "$V1_WORK/checksums.txt" "$V1_WORK/attacker-envelope" \
        "$V1_WORK/attacker-private.pem"
    "$V1_SIGNER" "$V1_WORK/checksums.txt" "$V1_WORK/both-envelope" \
        "$V1_WORK/k2-private.pem" "$V1_WORK/k1-private.pem"
    v1_write_trust "$V1_WORK/trust" 2099-12-31 2099-12-31
}

v1_write_trust() {
    output=$1
    k1_expiry=$2
    k2_expiry=$3
    printf '%s\t%s\t%s\n%s\t%s\t%s\n' \
        "$k1_fp" "$k1_expiry" "$k1_spki" \
        "$k2_fp" "$k2_expiry" "$k2_spki" > "$output"
}

v1_record() {
    sed -n '2p' "$1"
}

v1_write_records() {
    output=$1
    shift
    {
        printf 'AUTOPUS-RELEASE-SIGNATURE-V1\n'
        for envelope do
            v1_record "$envelope"
        done
    } > "$output"
}

v1_verify() {
    checksums=$1
    envelope=$2
    trust=$3
    now=$4
    run_dir=$(mktemp -d "$V1_WORK/run.XXXXXX")
    verify_release_checksums_v1_with_trust \
        "$checksums" "$envelope" "$trust" "$now" "$run_dir"
}

v1_expect_success() {
    name=$1
    shift
    if output=$("$@" 2>&1); then
        printf '  [OK] %s\n' "$name"
        return
    fi
    printf '  [FAIL] %s: %s\n' "$name" "$output" >&2
    exit 1
}

v1_expect_failure() {
    name=$1
    code=$2
    shift 2
    if output=$("$@" 2>&1); then
        printf '  [FAIL] %s: unexpectedly succeeded\n' "$name" >&2
        exit 1
    fi
    case "$output" in
        *"$code"*) printf '  [OK] %s\n' "$name" ;;
        *) printf '  [FAIL] %s: missing %s: %s\n' "$name" "$code" "$output" >&2; exit 1 ;;
    esac
}

v1_base64_file() {
    openssl base64 -A -in "$1"
}

v1_sha256_file() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | awk '{print $1}'
    else
        shasum -a 256 "$1" | awk '{print $1}'
    fi
}

v1_hex_file() {
    hex=$1
    output=$2
    : > "$output"
    while [ -n "$hex" ]; do
        byte=$(printf '%.2s' "$hex")
        hex=${hex#??}
        octal=$(printf '%03o' "0x$byte")
        printf "\\$octal" >> "$output"
    done
}

v1_write_der_envelope() {
    output=$1
    der_file=$2
    fingerprint=${3:-$k1_fp}
    printf 'AUTOPUS-RELEASE-SIGNATURE-V1\n%s\t%s\n' \
        "$fingerprint" "$(v1_base64_file "$der_file")" > "$output"
}
