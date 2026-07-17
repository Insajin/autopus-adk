#!/bin/sh

set -eu
. "$(CDPATH= cd -- "$(dirname "$0")" && pwd)/posix-v1-common.sh"
v1_test_setup

printf 'POSIX V1 envelope oracle\n'
v1_expect_success "single K1" v1_verify "$V1_WORK/checksums.txt" "$V1_WORK/k1-envelope" "$V1_WORK/trust" "$V1_NOW"
v1_expect_success "K1 plus K2" v1_verify "$V1_WORK/checksums.txt" "$V1_WORK/both-envelope" "$V1_WORK/trust" "$V1_NOW"

v1_write_records "$V1_WORK/unknown-known" "$V1_WORK/attacker-envelope" "$V1_WORK/k1-envelope"
v1_expect_success "valid unknown plus known" v1_verify "$V1_WORK/checksums.txt" "$V1_WORK/unknown-known" "$V1_WORK/trust" "$V1_NOW"
v1_expect_failure "unknown only" no_trusted_release_signature v1_verify \
    "$V1_WORK/checksums.txt" "$V1_WORK/attacker-envelope" "$V1_WORK/trust" "$V1_NOW"
v1_expect_failure "tampered checksums" no_trusted_release_signature v1_verify \
    "$V1_WORK/checksums-tampered.txt" "$V1_WORK/k1-envelope" "$V1_WORK/trust" "$V1_NOW"

attacker_sig=$(v1_record "$V1_WORK/attacker-envelope" | awk -F '\t' '{print $2}')
printf 'AUTOPUS-RELEASE-SIGNATURE-V1\n%s\t%s\n' "$k1_fp" "$attacker_sig" > "$V1_WORK/known-attacker"
v1_expect_failure "known fingerprint attacker signature" no_trusted_release_signature v1_verify \
    "$V1_WORK/checksums.txt" "$V1_WORK/known-attacker" "$V1_WORK/trust" "$V1_NOW"

: > "$V1_WORK/empty"
printf 'WRONG\n%s\n' "$(v1_record "$V1_WORK/k1-envelope")" > "$V1_WORK/wrong-header"
printf 'AUTOPUS-RELEASE-SIGNATURE-V1\n' > "$V1_WORK/header-only"
printf 'AUTOPUS-RELEASE-SIGNATURE-V1\r\n%s\r\n' "$(v1_record "$V1_WORK/k1-envelope")" > "$V1_WORK/crlf"
printf 'AUTOPUS-RELEASE-SIGNATURE-V1\n%s' "$(v1_record "$V1_WORK/k1-envelope")" > "$V1_WORK/no-final-lf"
printf '\357\273\277AUTOPUS-RELEASE-SIGNATURE-V1\n%s\n' "$(v1_record "$V1_WORK/k1-envelope")" > "$V1_WORK/bom"
printf 'AUTOPUS-RELEASE-SIGNATURE-V1\n%s\000x\n' "$(v1_record "$V1_WORK/k1-envelope")" > "$V1_WORK/nul"
printf 'AUTOPUS-RELEASE-SIGNATURE-V1\n\000%s\n' "$(v1_record "$V1_WORK/k1-envelope")" > "$V1_WORK/lf-followed-control"
printf 'AUTOPUS-RELEASE-SIGNATURE-V1\n\n%s\n' "$(v1_record "$V1_WORK/k1-envelope")" > "$V1_WORK/blank-record"

for fixture in empty wrong-header header-only crlf no-final-lf bom nul lf-followed-control blank-record; do
    v1_expect_failure "$fixture" malformed_release_signature_envelope v1_verify \
        "$V1_WORK/checksums.txt" "$V1_WORK/$fixture" "$V1_WORK/trust" "$V1_NOW"
done

awk 'BEGIN { printf "AUTOPUS-RELEASE-SIGNATURE-V1\n"; for (i=0;i<4066;i++) printf "a"; printf "\n" }' > "$V1_WORK/oversized"
awk 'BEGIN { printf "AUTOPUS-RELEASE-SIGNATURE-V1\n"; for (i=0;i<257;i++) printf "a"; printf "\n" }' > "$V1_WORK/long-line"
{
    printf 'AUTOPUS-RELEASE-SIGNATURE-V1\n'
    i=0
    while [ "$i" -lt 17 ]; do v1_record "$V1_WORK/k1-envelope"; i=$((i + 1)); done
} > "$V1_WORK/too-many"
v1_write_records "$V1_WORK/duplicate" "$V1_WORK/k1-envelope" "$V1_WORK/k1-envelope"
for fixture in oversized long-line too-many duplicate; do
    v1_expect_failure "$fixture" malformed_release_signature_envelope v1_verify \
        "$V1_WORK/checksums.txt" "$V1_WORK/$fixture" "$V1_WORK/trust" "$V1_NOW"
done

upper_fp=$(printf '%s' "$k1_fp" | tr '[:lower:]' '[:upper:]')
valid_sig=$(v1_record "$V1_WORK/k1-envelope" | awk -F '\t' '{print $2}')
printf 'AUTOPUS-RELEASE-SIGNATURE-V1\n%s\t%s\n' "$upper_fp" "$valid_sig" > "$V1_WORK/uppercase-fp"
printf 'AUTOPUS-RELEASE-SIGNATURE-V1\nabc\t%s\n' "$valid_sig" > "$V1_WORK/short-fp"
printf 'AUTOPUS-RELEASE-SIGNATURE-V1\n%s %s\n' "$k1_fp" "$valid_sig" > "$V1_WORK/space-separator"
printf 'AUTOPUS-RELEASE-SIGNATURE-V1\n%s\t%s\textra\n' "$k1_fp" "$valid_sig" > "$V1_WORK/extra-tab"
printf 'AUTOPUS-RELEASE-SIGNATURE-V1\n%s\t%%%%\n' "$k1_fp" > "$V1_WORK/invalid-base64"
printf 'AUTOPUS-RELEASE-SIGNATURE-V1\n%s\t%s=\n' "$k1_fp" "$valid_sig" > "$V1_WORK/noncanonical-base64"
for fixture in uppercase-fp short-fp space-separator extra-tab invalid-base64 noncanonical-base64; do
    v1_expect_failure "$fixture" malformed_release_signature_envelope v1_verify \
        "$V1_WORK/checksums.txt" "$V1_WORK/$fixture" "$V1_WORK/trust" "$V1_NOW"
done

order=ffffffff00000000ffffffffffffffffbce6faada7179e84f3b9cac2fc632551
v1_hex_file 3006020101020101 "$V1_WORK/minimal.der"
v1_hex_file 300602010102010100 "$V1_WORK/trailing.der"
v1_hex_file 308106020101020101 "$V1_WORK/long-form.der"
v1_hex_file 3006020180020101 "$V1_WORK/negative.der"
v1_hex_file 300702020001020101 "$V1_WORK/redundant-zero.der"
v1_hex_file 3006020100020101 "$V1_WORK/zero.der"
v1_hex_file "3026022100${order}020101" "$V1_WORK/order.der"
v1_hex_file "3026022100ffffffff00000000ffffffffffffffffbce6faada7179e84f3b9cac2fc632552020101" "$V1_WORK/order-over.der"
printf 'not-der' > "$V1_WORK/invalid.der"
for fixture in trailing long-form negative redundant-zero zero order order-over invalid; do
    v1_write_der_envelope "$V1_WORK/$fixture-envelope" "$V1_WORK/$fixture.der"
    v1_expect_failure "$fixture DER" malformed_release_signature_envelope v1_verify \
        "$V1_WORK/checksums.txt" "$V1_WORK/$fixture-envelope" "$V1_WORK/trust" "$V1_NOW"
done

real_openssl=$(command -v openssl)
mkdir "$V1_WORK/wrapped-path"
cat > "$V1_WORK/wrapped-path/openssl" <<'WRAPPER'
#!/bin/sh
case " $* " in *" -verify "*) printf 'verify\n' >> "$VERIFY_LOG" ;; esac
exec "$REAL_OPENSSL" "$@"
WRAPPER
chmod +x "$V1_WORK/wrapped-path/openssl"
printf 'AUTOPUS-RELEASE-SIGNATURE-V1\n%s\t%%%%\n%s\n' \
    "$attacker_fp" "$(v1_record "$V1_WORK/k1-envelope")" > "$V1_WORK/malformed-unknown-known"
verify_with_trace() {
    PATH="$V1_WORK/wrapped-path:$PATH" REAL_OPENSSL="$real_openssl" VERIFY_LOG="$V1_WORK/verify.log" \
        v1_verify "$@"
}
v1_expect_failure "full parse before crypto" malformed_release_signature_envelope verify_with_trace \
    "$V1_WORK/checksums.txt" "$V1_WORK/malformed-unknown-known" "$V1_WORK/trust" "$V1_NOW"
test ! -e "$V1_WORK/verify.log" || test ! -s "$V1_WORK/verify.log"

printf 'POSIX V1 envelope oracle: PASS\n'
