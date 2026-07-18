#!/bin/sh

set -eu
. "$(CDPATH= cd -- "$(dirname "$0")" && pwd)/posix-v1-common.sh"
v1_test_setup

printf 'POSIX V1 installer oracle\n'

live_dir="$TESTS_DIR/fixtures/v0.50.73"
v1_expect_success "live v0.50.73 K1 fixture" verify_release_checksums_v1 \
    "$live_dir/checksums.txt" "$live_dir/checksums.txt.signatures" \
    "$(mktemp -d "$V1_WORK/live.XXXXXX")"

v1_expect_success "K2 rotation" v1_verify "$V1_WORK/checksums.txt" \
    "$V1_WORK/k2-envelope" "$V1_WORK/trust" "$V1_NOW"
v1_write_trust "$V1_WORK/expired-trust" 2020-01-01 2020-01-01
v1_expect_failure "all keys expired" all_release_signing_keys_expired v1_verify \
    "$V1_WORK/checksums.txt" "$V1_WORK/k1-envelope" "$V1_WORK/expired-trust" "$V1_NOW"
printf '%s\t2099-12-31\t%s\n' "$k1_fp" 'not-base64' > "$V1_WORK/malformed-trust"
v1_expect_failure "malformed embedded key" malformed_embedded_release_key v1_verify \
    "$V1_WORK/checksums.txt" "$V1_WORK/k1-envelope" "$V1_WORK/malformed-trust" "$V1_NOW"

AUTOPUS_INSTALLER_TEST_SOURCE=1 . "$REPO_ROOT/install.sh"
OS=$(detect_os)
ARCH=$(detect_arch)
ARCHIVE="autopus-adk_0.50.73_${OS}_${ARCH}.tar.gz"
mkdir "$V1_WORK/archive-content"
printf '#!/bin/sh\nexit 0\n' > "$V1_WORK/archive-content/auto"
chmod +x "$V1_WORK/archive-content/auto"
tar -czf "$V1_WORK/$ARCHIVE" -C "$V1_WORK/archive-content" auto
archive_sha=$(v1_sha256_file "$V1_WORK/$ARCHIVE")
printf '%s  %s\n' "$archive_sha" "$ARCHIVE" > "$V1_WORK/install-checksums"
"$V1_SIGNER" "$V1_WORK/install-checksums" "$V1_WORK/install-envelope" "$V1_WORK/k1-private.pem"

cp "$V1_HELPER" "$V1_WORK/test-helper.sh"
{
    printf '\nrelease_v1_k1_fingerprint=%s\n' "$k1_fp"
    printf 'release_v1_k1_expires=2099-12-31\n'
    printf 'release_v1_k1_spki=%s\n' "$k1_spki"
    printf 'release_v1_k2_fingerprint=%s\n' "$k2_fp"
    printf 'release_v1_k2_expires=2099-12-31\n'
    printf 'release_v1_k2_spki=%s\n' "$k2_spki"
} >> "$V1_WORK/test-helper.sh"
VERIFIER_SHA256=$(v1_sha256_file "$V1_WORK/test-helper.sh")
DOWNLOAD_HELPER="$V1_WORK/test-helper.sh"
DOWNLOAD_CHECKSUMS="$V1_WORK/install-checksums"
DOWNLOAD_ENVELOPE="$V1_WORK/install-envelope"
DOWNLOAD_ARCHIVE="$V1_WORK/$ARCHIVE"

download() {
    source_url=$1
    destination=$2
    case "$source_url" in
        *verify-checksums-v1.sh) /bin/cp "$DOWNLOAD_HELPER" "$destination" ;;
        *checksums.txt.signatures) /bin/cp "$DOWNLOAD_ENVELOPE" "$destination" ;;
        *checksums.txt) /bin/cp "$DOWNLOAD_CHECKSUMS" "$destination" ;;
        *.tar.gz) /bin/cp "$DOWNLOAD_ARCHIVE" "$destination" ;;
        *) return 1 ;;
    esac
}

run_main() (
    install_dir=$1
    run_version=$2
    run_path=$3
    INSTALL_DIR="$install_dir"
    VERSION="$run_version"
    PATH="$run_path"
    SHELL=/bin/sh
    export INSTALL_DIR VERSION PATH SHELL
    hash -r 2>/dev/null || true
    main
)

expect_mode_0755() {
    name=$1
    target=$2
    if actual=$(stat -f '%Lp' "$target" 2>/dev/null); then
        :
    elif actual=$(stat -c '%a' "$target" 2>/dev/null); then
        :
    else
        printf '  [FAIL] %s: cannot read mode\n' "$name" >&2
        exit 1
    fi
    if [ "$actual" != 755 ]; then
        printf '  [FAIL] %s: mode %s, want 755\n' "$name" "$actual" >&2
        exit 1
    fi
    printf '  [OK] %s mode 0755\n' "$name"
}

success_dir="$V1_WORK/install-success"
if ! success_output=$(run_main "$success_dir" 0.50.73 "$PATH" 2>&1); then
    printf '  [FAIL] normal install: %s\n' "$success_output" >&2
    exit 1
fi
test -x "$success_dir/auto"
expect_mode_0755 "non-sudo install directory" "$success_dir"
expect_mode_0755 "non-sudo installed binary" "$success_dir/auto"
case "$success_output" in
    *"릴리스 서명 검증 통과"*"autopus-adk v0.50.73 설치 완료"*) printf '  [OK] normal install UX\n' ;;
    *) printf '  [FAIL] normal install UX: %s\n' "$success_output" >&2; exit 1 ;;
esac

forced_sudo_dir="$V1_WORK/install-sudo"
fake_bin="$V1_WORK/fake-bin"
mkdir "$fake_bin"
printf '%s\n' '#!/bin/sh' \
    'for arg do' \
    '    [ "$arg" = "$AUTOPUS_TEST_FORCE_SUDO_DIR" ] && exit 1' \
    'done' \
    'exec "$AUTOPUS_TEST_REAL_MKDIR" "$@"' > "$fake_bin/mkdir"
printf '%s\n' '#!/bin/sh' \
    'PATH=$AUTOPUS_TEST_REAL_PATH' \
    'export PATH' \
    'exec "$@"' > "$fake_bin/sudo"
chmod +x "$fake_bin/mkdir" "$fake_bin/sudo"
AUTOPUS_TEST_FORCE_SUDO_DIR=$forced_sudo_dir
AUTOPUS_TEST_REAL_MKDIR=$(command -v mkdir)
AUTOPUS_TEST_REAL_PATH=$PATH
export AUTOPUS_TEST_FORCE_SUDO_DIR AUTOPUS_TEST_REAL_MKDIR AUTOPUS_TEST_REAL_PATH
if ! sudo_output=$(run_main "$forced_sudo_dir" 0.50.73 "$fake_bin:$PATH" 2>&1); then
    printf '  [FAIL] forced sudo install: %s\n' "$sudo_output" >&2
    exit 1
fi
expect_mode_0755 "sudo install directory" "$forced_sudo_dir"
expect_mode_0755 "sudo installed binary" "$forced_sudo_dir/auto"

expect_main_failure() {
    name=$1
    code=$2
    shift 2
    failed_dir="$V1_WORK/fail-$(printf '%s' "$name" | tr ' /' '__')"
    if output=$(run_main "$failed_dir" "$@" 2>&1); then
        printf '  [FAIL] %s: unexpectedly succeeded\n' "$name" >&2
        exit 1
    fi
    case "$output" in
        *"$code"*) : ;;
        *) printf '  [FAIL] %s: missing %s: %s\n' "$name" "$code" "$output" >&2; exit 1 ;;
    esac
    test ! -e "$failed_dir/auto"
    printf '  [OK] %s\n' "$name"
}

expect_main_failure "unsigned floor" unsigned_release_not_supported 0.50.72 "$PATH"

saved_hash=$VERIFIER_SHA256
VERIFIER_SHA256=0000000000000000000000000000000000000000000000000000000000000000
expect_main_failure "helper drift" installer_verifier_integrity_failed 0.50.73 "$PATH"
VERIFIER_SHA256=$saved_hash

cp "$V1_WORK/install-envelope" "$V1_WORK/saved-envelope"
DOWNLOAD_ENVELOPE="$V1_WORK/attacker-envelope"
expect_main_failure "unknown attacker" no_trusted_release_signature 0.50.73 "$PATH"
DOWNLOAD_ENVELOPE="$V1_WORK/saved-envelope"

printf 'AUTOPUS-RELEASE-SIGNATURE-V1\n%s\t%%%%\n' "$attacker_fp" > "$V1_WORK/malformed-install-envelope"
DOWNLOAD_ENVELOPE="$V1_WORK/malformed-install-envelope"
expect_main_failure "malformed envelope" malformed_release_signature_envelope 0.50.73 "$PATH"
DOWNLOAD_ENVELOPE="$V1_WORK/saved-envelope"

printf '%064d  %s\n' 0 "$ARCHIVE" > "$V1_WORK/wrong-checksums"
"$V1_SIGNER" "$V1_WORK/wrong-checksums" "$V1_WORK/wrong-envelope" "$V1_WORK/k1-private.pem"
DOWNLOAD_CHECKSUMS="$V1_WORK/wrong-checksums"
DOWNLOAD_ENVELOPE="$V1_WORK/wrong-envelope"
expect_main_failure "archive checksum mismatch" release_checksum_mismatch 0.50.73 "$PATH"
DOWNLOAD_CHECKSUMS="$V1_WORK/install-checksums"
DOWNLOAD_ENVELOPE="$V1_WORK/saved-envelope"

make_path() {
    target=$1
    excluded=$2
    mkdir "$target"
    for tool in sh awk od wc date sed tr grep cmp mkdir rm shasum sha256sum tar chmod ln uname mktemp cp; do
        [ "$tool" = "$excluded" ] && continue
        tool_path=$(command -v "$tool" 2>/dev/null || true)
        [ -n "$tool_path" ] && ln -s "$tool_path" "$target/$tool"
    done
    if [ "$excluded" != openssl ]; then ln -s "$(command -v openssl)" "$target/openssl"; fi
}
make_path "$V1_WORK/no-openssl" openssl
expect_main_failure "openssl absent" release_signature_tool_unavailable 0.50.73 "$V1_WORK/no-openssl"
make_path "$V1_WORK/no-checksum" checksum
rm -f "$V1_WORK/no-checksum/shasum" "$V1_WORK/no-checksum/sha256sum"
expect_main_failure "checksum tool absent" release_checksum_tool_unavailable 0.50.73 "$V1_WORK/no-checksum"

printf 'POSIX V1 installer oracle: PASS\n'
