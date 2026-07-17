#!/bin/sh
# test-install-signing.sh — install.sh 서명/체크섬 검증 함수 oracle 테스트.
# SPEC-ADK-RELEASE-SIGNING-001 T6. install.sh를 실행하지 않고 verify_signature/
# verify_checksum만 소싱해(INSTALL_SH_TEST_SOURCE=1) 정상/변조/공격자키/서명부재/
# 만료/회전창/openssl부재/checksum도구부재 경로를 로컬 openssl로 검증한다.
# 실제 배포 키는 쓰지 않는다 — 모든 키는 이 스크립트가 생성하는 합성 키다.
# Usage: sh scripts/test-install-signing.sh

set -eu

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
INSTALL_SH="$REPO_ROOT/install.sh"

if ! command -v openssl > /dev/null 2>&1; then
    echo "SKIP: openssl not found locally; cannot run oracle fixtures" >&2
    exit 0
fi

WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT
cd "$WORKDIR"

PASS=0
FAIL=0
ok()  { printf '  [OK] %s\n' "$1"; PASS=$((PASS + 1)); }
bad() { printf '  [FAIL] %s\n' "$1"; FAIL=$((FAIL + 1)); }

# --- Fixtures: synthetic ECDSA P-256 keys only, never production keys ---
openssl ecparam -name prime256v1 -genkey -noout -out K_priv.pem 2>/dev/null
openssl ec -in K_priv.pem -pubout -out K_pub.pem 2>/dev/null
openssl ecparam -name prime256v1 -genkey -noout -out K2_priv.pem 2>/dev/null
openssl ec -in K2_priv.pem -pubout -out K2_pub.pem 2>/dev/null
openssl ecparam -name prime256v1 -genkey -noout -out A_priv.pem 2>/dev/null
openssl ec -in A_priv.pem -pubout -out A_pub.pem 2>/dev/null

echo "abc123  autopus-adk_0.7.0_darwin_arm64.tar.gz" > checksums.txt
sed 's/abc123/abc124/' checksums.txt > checksums_tampered.txt

openssl dgst -sha256 -sign K_priv.pem -out checksums_K.sig checksums.txt 2>/dev/null
openssl dgst -sha256 -sign K2_priv.pem -out checksums_K2.sig checksums.txt 2>/dev/null
openssl dgst -sha256 -sign A_priv.pem -out checksums_A.sig checksums.txt 2>/dev/null
echo "not-a-real-signature" > checksums_garbage.sig
: > checksums_empty.sig

# Directory with a subset of tools, deliberately excluding openssl (S7).
NO_OPENSSL_DIR="$WORKDIR/no-openssl-path"
mkdir -p "$NO_OPENSSL_DIR"
for tool in sh cat mktemp date rm; do
    tool_path="$(command -v "$tool")"
    [ -n "$tool_path" ] && ln -sf "$tool_path" "$NO_OPENSSL_DIR/$tool"
done

# run_case builds a small driver script that sources install.sh (without
# triggering main()), overrides the pinned key set, and calls
# verify_signature. Running it as a separate `sh` process is required:
# verify_signature calls err(), which does exit 1 directly, so an
# expected-failure case would otherwise kill this test runner too.
run_case() {
    name="$1" checksums_file="$2" sig_file="$3" key_count="$4"
    pubkey1_file="$5" expires1="$6" pubkey2_file="$7" expires2="$8"
    want_exit="$9" want_substr="${10}" path_override="${11:-$PATH}"

    driver="$WORKDIR/driver.sh"
    {
        printf '. "%s"\n' "$INSTALL_SH"
        printf 'RELEASE_KEY_COUNT=%s\n' "$key_count"
        printf 'RELEASE_PUBKEY_1="$(cat "%s")"\n' "$pubkey1_file"
        printf 'RELEASE_PUBKEY_1_EXPIRES="%s"\n' "$expires1"
        if [ -n "$pubkey2_file" ]; then
            printf 'RELEASE_PUBKEY_2="$(cat "%s")"\n' "$pubkey2_file"
            printf 'RELEASE_PUBKEY_2_EXPIRES="%s"\n' "$expires2"
        fi
        printf 'verify_signature "%s" "%s"\n' "$checksums_file" "$sig_file"
        printf 'echo VERIFY_REACHED_END\n'
    } > "$driver"

    set +e
    output="$(env -i PATH="$path_override" HOME="${HOME:-}" INSTALL_SH_TEST_SOURCE=1 sh "$driver" 2>&1)"
    got_exit=$?
    set -e

    check_case "$name" "$got_exit" "$want_exit" "$output" "$want_substr"
}

# run_checksum_case exercises verify_checksum the same isolated way.
run_checksum_case() {
    name="$1" archive="$2" expected="$3" want_exit="$4" want_substr="$5" path_override="${6:-$PATH}"

    driver="$WORKDIR/driver.sh"
    {
        printf '. "%s"\n' "$INSTALL_SH"
        printf 'verify_checksum "%s" "%s"\n' "$archive" "$expected"
        printf 'echo VERIFY_REACHED_END\n'
    } > "$driver"

    set +e
    output="$(env -i PATH="$path_override" HOME="${HOME:-}" INSTALL_SH_TEST_SOURCE=1 sh "$driver" 2>&1)"
    got_exit=$?
    set -e

    check_case "$name" "$got_exit" "$want_exit" "$output" "$want_substr"
}

check_case() {
    name="$1" got_exit="$2" want_exit="$3" output="$4" want_substr="$5"
    if [ "$got_exit" != "$want_exit" ]; then
        bad "$name: exit=$got_exit want=$want_exit (output: $output)"
        return
    fi
    if [ -n "$want_substr" ]; then
        case "$output" in
            *"$want_substr"*) : ;;
            *) bad "$name: output missing '$want_substr' (output: $output)"; return ;;
        esac
    fi
    ok "$name"
}

echo "🐙 install.sh signature/checksum verification oracle tests"
echo ""

# S1/S9: normal signed release, single embedded key.
run_case "S1/S9 normal path" checksums.txt checksums_K.sig 1 K_pub.pem 2099-12-31 "" "" 0 "" "$PATH"

# S2: tampered checksums.txt, signature is for the original bytes.
run_case "S2 tampered checksums" checksums_tampered.txt checksums_K.sig 1 K_pub.pem 2099-12-31 "" "" 1 "no trusted release signing key verified" "$PATH"

# S4/S6: attacker re-signed with a key outside the embedded set.
run_case "S4/S6 attacker key outside embedded set" checksums.txt checksums_A.sig 1 K_pub.pem 2099-12-31 "" "" 1 "no trusted release signing key verified" "$PATH"

# Garbage / empty signature bytes must also fail closed, not error out oddly.
run_case "garbage signature bytes" checksums.txt checksums_garbage.sig 1 K_pub.pem 2099-12-31 "" "" 1 "no trusted release signing key verified" "$PATH"
run_case "empty signature file" checksums.txt checksums_empty.sig 1 K_pub.pem 2099-12-31 "" "" 1 "no trusted release signing key verified" "$PATH"

# S8(b): sole embedded key expired — excluded before any crypto trial, even
# though the signature itself is valid for that key.
run_case "S8b sole key expired" checksums.txt checksums_K.sig 1 K_pub.pem 2020-01-01 "" "" 1 "all embedded keys expired" "$PATH"

# S8(c): 2-key rotation window, signer is the second (newly rotated-in) key.
run_case "S8c rotation window (K2 signer)" checksums.txt checksums_K2.sig 2 K_pub.pem 2099-12-31 K2_pub.pem 2099-12-31 0 "" "$PATH"

# S7: openssl not found -> fail-closed, not a silent skip.
run_case "S7 openssl absent" checksums.txt checksums_K.sig 1 K_pub.pem 2099-12-31 "" "" 1 "서명을 검증할 수 없습니다" "$NO_OPENSSL_DIR"

# verify_checksum regression: match still succeeds, mismatch still fails.
ARCHIVE_SHA="$(shasum -a 256 checksums.txt 2>/dev/null | awk '{print $1}')"
run_checksum_case "verify_checksum match" checksums.txt "$ARCHIVE_SHA" 0 "" "$PATH"
run_checksum_case "verify_checksum mismatch" checksums.txt "0000000000000000000000000000000000000000000000000000000000000000" 1 "체크섬 불일치" "$PATH"

# verify_checksum tool absence must now be fail-closed (was fail-open before this SPEC).
run_checksum_case "verify_checksum tool absent (fail-closed)" checksums.txt "$ARCHIVE_SHA" 1 "검증 도구 없이는 설치를 진행할 수 없습니다" "$NO_OPENSSL_DIR"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Results: ${PASS} passed, ${FAIL} failed"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ "$FAIL" -ne 0 ]; then
    exit 1
fi
