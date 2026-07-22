#!/bin/sh

# Source-only POSIX verifier for AUTOPUS-RELEASE-SIGNATURE-V1.

release_v1_k1_fingerprint=e1fdfe066484c7eae8ff16fa4b1ee6237b8d06299c2b66ced485f029af77837f
release_v1_k1_expires=2028-07-17
release_v1_k1_spki=MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEDFjY80Lc2GJSsd8M6uAO/v7AZK3Z1sPEXrK4Hbm4m4+ykavvcoKlpZ5sn/T/l2InDXuhxkdX6aFv57bicik2Ug==
release_v1_k2_fingerprint=93d9f681d829f2d0bdba7e1853e6acf9ae2ffd2c760355853218e920c35cc5ff
release_v1_k2_expires=2030-07-17
release_v1_k2_spki=MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEp+d1byDqWFismSIMWhTEHnbo/pdp7JVZwhXOIZJb0q2WHLxwMD7P77Fkr75Xnx1qYZgfvIl9Sg8Z+V9gSaq8Og==

release_v1_fail() {
    printf 'release signing: %s\n' "$1" >&2
    return 1
}

release_v1_is_lower_hex() {
    # Shell bracket ranges follow the caller's locale collation rules.
    [ "$#" -eq 1 ] && [ -n "$1" ] || return 1
    while [ -n "$1" ]; do
        case "${1%"${1#?}"}" in
            0|1|2|3|4|5|6|7|8|9|a|b|c|d|e|f) : ;;
            *) return 1 ;;
        esac
        set -- "${1#?}"
    done
}

release_v1_require_tools() {
    for tool in openssl od awk wc sed tr cmp grep date mkdir; do
        command -v "$tool" >/dev/null 2>&1 || {
            release_v1_fail release_signature_tool_unavailable
            return 1
        }
    done
}

release_v1_valid_date() {
    LC_ALL=C awk -v value="$1" 'BEGIN {
        if (length(value) != 10 || substr(value,5,1) != "-" || substr(value,8,1) != "-") exit 1
        y=substr(value,1,4); m=substr(value,6,2); d=substr(value,9,2)
        if (y !~ /^[0-9][0-9][0-9][0-9]$/ || m !~ /^[0-9][0-9]$/ || d !~ /^[0-9][0-9]$/) exit 1
        y+=0; m+=0; d+=0
        days[1]=31; days[2]=28; days[3]=31; days[4]=30; days[5]=31; days[6]=30
        days[7]=31; days[8]=31; days[9]=30; days[10]=31; days[11]=30; days[12]=31
        if ((y%4==0 && y%100!=0) || y%400==0) days[2]=29
        if (m < 1 || m > 12 || d < 1 || d > days[m]) exit 1
    }'
}

release_v1_is_active() {
    LC_ALL=C awk -v now="x$1" -v expiry="x$2" 'BEGIN { exit !(now <= expiry) }'
}

release_v1_check_wire() {
    LC_ALL=C od -An -v -tu1 "$1" 2>/dev/null | LC_ALL=C awk '
        { for (i=1; i<=NF; i++) {
            byte=$i; total++; last=byte
            if (total > 4096) exit 1
            if (byte == 10) {
                if (line < 1 || line > 256) exit 1
                lines++; line=0; continue
            }
            if (byte != 9 && (byte < 32 || byte > 126)) exit 1
            line++
        }}
        END { if (total < 1 || total > 4096 || last != 10 || lines < 2 || lines > 17) exit 1 }
    '
}

release_v1_check_der() {
    LC_ALL=C od -An -v -tu1 "$1" 2>/dev/null | LC_ALL=C awk '
        BEGIN {
            order="ffffffff00000000ffffffffffffffffbce6faada7179e84f3b9cac2fc632551"
            zeros="0000000000000000000000000000000000000000000000000000000000000000"
        }
        function bad() { failed=1; exit 1 }
        function scalar(p, len, first, i, hex) {
            if (p+1 > n || bytes[p] != 2) bad()
            len=bytes[p+1]; p+=2
            if (len < 1 || len > 33 || p+len-1 > n) bad()
            first=bytes[p]
            if (first == 0) {
                if (len == 1 || bytes[p+1] < 128) bad()
                p++; len--
            } else if (first >= 128) bad()
            if (len > 32) bad()
            hex=""
            for (i=0; i<len; i++) hex=hex sprintf("%02x", bytes[p+i])
            hex=sprintf("%064s", hex); gsub(/ /, "0", hex)
            if (hex == zeros || ("x" hex) >= ("x" order)) bad()
            return p+len
        }
        { for (i=1; i<=NF; i++) bytes[++n]=$i }
        END {
            if (failed || n < 8 || n > 72 || bytes[1] != 48 || bytes[2] != n-2) exit 1
            p=scalar(3); p=scalar(p)
            if (p != n+1) exit 1
        }
    '
}

release_v1_parse_envelope() {
    envelope=$1
    state=$2
    records="$state/records"
    release_v1_check_wire "$envelope" || {
        release_v1_fail malformed_release_signature_envelope
        return 1
    }
    : > "$records"
    tab=$(printf '\t')
    seen='|'
    count=0
    IFS= read -r header < "$envelope" || header=
    [ "$header" = AUTOPUS-RELEASE-SIGNATURE-V1 ] || {
        release_v1_fail malformed_release_signature_envelope
        return 1
    }
    {
        IFS= read -r ignored
        while IFS= read -r line; do
            case "$line" in *"$tab"*) : ;; *) release_v1_fail malformed_release_signature_envelope; return 1 ;; esac
            fingerprint=${line%%"$tab"*}
            encoded=${line#*"$tab"}
            case "$encoded" in *"$tab"*|'') release_v1_fail malformed_release_signature_envelope; return 1 ;; esac
            [ "${#fingerprint}" -eq 64 ] || { release_v1_fail malformed_release_signature_envelope; return 1; }
            release_v1_is_lower_hex "$fingerprint" || {
                release_v1_fail malformed_release_signature_envelope; return 1
            }
            case "$encoded" in *[!A-Za-z0-9+/=]*) release_v1_fail malformed_release_signature_envelope; return 1 ;; esac
            case "$seen" in *"|$fingerprint|"*) release_v1_fail malformed_release_signature_envelope; return 1 ;; esac
            seen="$seen$fingerprint|"
            count=$((count + 1))
            signature="$state/signature-$count.der"
            printf '%s' "$encoded" | openssl base64 -d -A > "$signature" 2>/dev/null || {
                release_v1_fail malformed_release_signature_envelope
                return 1
            }
            canonical=$(openssl base64 -A -in "$signature" 2>/dev/null) || {
                release_v1_fail malformed_release_signature_envelope
                return 1
            }
            [ "$canonical" = "$encoded" ] && release_v1_check_der "$signature" || {
                release_v1_fail malformed_release_signature_envelope
                return 1
            }
            printf '%s\t%s\n' "$fingerprint" "$signature" >> "$records"
        done
    } < "$envelope"
    [ "$count" -ge 1 ] && [ "$count" -le 16 ] || {
        release_v1_fail malformed_release_signature_envelope
        return 1
    }
}

release_v1_prepare_trust() {
    trust=$1
    now=$2
    state=$3
    validated="$state/trust"
    : > "$validated"
    tab=$(printf '\t')
    seen='|'
    count=0
    active=0
    while IFS="$tab" read -r fingerprint expiry spki extra; do
        count=$((count + 1))
        [ -z "$extra" ] && [ "${#fingerprint}" -eq 64 ] || {
            release_v1_fail malformed_embedded_release_key; return 1
        }
        release_v1_is_lower_hex "$fingerprint" || {
            release_v1_fail malformed_embedded_release_key; return 1
        }
        case "$spki" in ''|*[!A-Za-z0-9+/=]*) release_v1_fail malformed_embedded_release_key; return 1 ;; esac
        release_v1_valid_date "$expiry" || { release_v1_fail malformed_embedded_release_key; return 1; }
        case "$seen" in *"|$fingerprint|"*) release_v1_fail malformed_embedded_release_key; return 1 ;; esac
        seen="$seen$fingerprint|"
        der="$state/key-$count.der"
        canonical_der="$state/key-$count-canonical.der"
        pem="$state/key-$count.pem"
        printf '%s' "$spki" | openssl base64 -d -A > "$der" 2>/dev/null || {
            release_v1_fail malformed_embedded_release_key; return 1
        }
        [ "$(openssl base64 -A -in "$der" 2>/dev/null)" = "$spki" ] || {
            release_v1_fail malformed_embedded_release_key; return 1
        }
        openssl ec -pubin -inform DER -in "$der" -pubout -outform DER \
            -out "$canonical_der" 2>/dev/null && cmp -s "$der" "$canonical_der" || {
            release_v1_fail malformed_embedded_release_key; return 1
        }
        openssl ec -pubin -inform DER -in "$der" -text -noout 2>/dev/null | \
            grep -q 'ASN1 OID: prime256v1' || {
            release_v1_fail malformed_embedded_release_key; return 1
        }
        actual=$(openssl dgst -sha256 "$der" 2>/dev/null | sed 's/^.*= *//' | tr '[:upper:]' '[:lower:]')
        [ "$actual" = "$fingerprint" ] || { release_v1_fail malformed_embedded_release_key; return 1; }
        openssl ec -pubin -inform DER -in "$der" -pubout -out "$pem" 2>/dev/null || {
            release_v1_fail malformed_embedded_release_key; return 1
        }
        printf '%s\t%s\t%s\n' "$fingerprint" "$expiry" "$pem" >> "$validated"
        if release_v1_is_active "$now" "$expiry"; then active=$((active + 1)); fi
    done < "$trust"
    [ "$count" -ge 1 ] && [ "$count" -le 16 ] || {
        release_v1_fail malformed_embedded_release_key; return 1
    }
    [ "$active" -gt 0 ] || { release_v1_fail all_release_signing_keys_expired; return 1; }
}

release_v1_verify_records() {
    checksums=$1
    now=$2
    state=$3
    tab=$(printf '\t')
    while IFS="$tab" read -r fingerprint signature; do
        while IFS="$tab" read -r trusted expiry pem; do
            [ "$fingerprint" = "$trusted" ] || continue
            release_v1_is_active "$now" "$expiry" || continue
            if openssl dgst -sha256 -verify "$pem" -signature "$signature" \
                "$checksums" >/dev/null 2>&1; then
                return 0
            fi
        done < "$state/trust"
    done < "$state/records"
    release_v1_fail no_trusted_release_signature
}

verify_release_checksums_v1_with_trust() {
    [ "$#" -eq 5 ] || { release_v1_fail malformed_embedded_release_key; return 1; }
    checksums=$1; envelope=$2; trust=$3; now=$4; workdir=$5
    release_v1_require_tools || return 1
    for input in "$checksums" "$envelope" "$trust"; do
        [ -f "$input" ] && [ ! -L "$input" ] || {
            release_v1_fail malformed_release_signature_envelope; return 1
        }
    done
    release_v1_valid_date "$now" || { release_v1_fail malformed_embedded_release_key; return 1; }
    [ -d "$workdir" ] && [ ! -L "$workdir" ] || {
        release_v1_fail malformed_release_signature_envelope; return 1
    }
    state="$workdir/state"
    [ ! -e "$state" ] && mkdir -m 700 "$state" || {
        release_v1_fail malformed_release_signature_envelope; return 1
    }
    release_v1_parse_envelope "$envelope" "$state" || return 1
    release_v1_prepare_trust "$trust" "$now" "$state" || return 1
    release_v1_verify_records "$checksums" "$now" "$state"
}

verify_release_checksums_v1() {
    [ "$#" -eq 3 ] || { release_v1_fail malformed_embedded_release_key; return 1; }
    checksums=$1; envelope=$2; workdir=$3
    [ -d "$workdir" ] && [ ! -L "$workdir" ] || {
        release_v1_fail malformed_release_signature_envelope; return 1
    }
    trust="$workdir/default-trust"
    printf '%s\t%s\t%s\n%s\t%s\t%s\n' \
        "$release_v1_k1_fingerprint" "$release_v1_k1_expires" "$release_v1_k1_spki" \
        "$release_v1_k2_fingerprint" "$release_v1_k2_expires" "$release_v1_k2_spki" > "$trust"
    now=$(date -u +%Y-%m-%d 2>/dev/null) || {
        release_v1_fail release_signature_tool_unavailable; return 1
    }
    verify_release_checksums_v1_with_trust "$checksums" "$envelope" "$trust" "$now" "$workdir"
}
