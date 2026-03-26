#!/bin/sh
set -e

# autopus-adk 설치 스크립트
# 사용법: curl -sSL https://raw.githubusercontent.com/Insajin/autopus-adk/main/install.sh | sh

REPO="Insajin/autopus-adk"
BINARY="auto"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# 색상 출력
info()  { printf '\033[1;34m%s\033[0m\n' "$1"; }
ok()    { printf '\033[1;32m%s\033[0m\n' "$1"; }
err()   { printf '\033[1;31m%s\033[0m\n' "$1" >&2; exit 1; }

# OS 감지
detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        *)       err "지원하지 않는 OS: $(uname -s)" ;;
    esac
}

# 아키텍처 감지
detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)   echo "amd64" ;;
        arm64|aarch64)  echo "arm64" ;;
        *)              err "지원하지 않는 아키텍처: $(uname -m)" ;;
    esac
}

# 최신 버전 조회
get_latest_version() {
    if command -v curl > /dev/null 2>&1; then
        curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/'
    elif command -v wget > /dev/null 2>&1; then
        wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/'
    else
        err "curl 또는 wget이 필요합니다"
    fi
}

# 다운로드
download() {
    url="$1"
    dest="$2"
    if command -v curl > /dev/null 2>&1; then
        curl -sSL "$url" -o "$dest"
    elif command -v wget > /dev/null 2>&1; then
        wget -qO "$dest" "$url"
    fi
}

# SHA256 체크섬 검증
verify_checksum() {
    archive="$1"
    expected_checksum="$2"

    if command -v sha256sum > /dev/null 2>&1; then
        actual=$(sha256sum "$archive" | awk '{print $1}')
    elif command -v shasum > /dev/null 2>&1; then
        actual=$(shasum -a 256 "$archive" | awk '{print $1}')
    else
        err "sha256sum 또는 shasum이 필요합니다 (체크섬 검증 불가)"
    fi

    if [ "$actual" != "$expected_checksum" ]; then
        err "체크섬 불일치! 다운로드가 변조되었을 수 있습니다.\n  expected: ${expected_checksum}\n  actual:   ${actual}"
    fi
}

main() {
    OS="$(detect_os)"
    ARCH="$(detect_arch)"
    VERSION="${VERSION:-$(get_latest_version)}"

    if [ -z "$VERSION" ]; then
        err "최신 버전을 가져올 수 없습니다. GitHub API 한도를 확인하세요."
    fi

    info "autopus-adk v${VERSION} 설치 중... (${OS}/${ARCH})"

    ARCHIVE="autopus-adk_${VERSION}_${OS}_${ARCH}.tar.gz"
    BASE_URL="https://github.com/${REPO}/releases/download/v${VERSION}"
    URL="${BASE_URL}/${ARCHIVE}"
    CHECKSUMS_URL="${BASE_URL}/checksums.txt"

    TMPDIR="$(mktemp -d)"
    trap 'rm -rf "$TMPDIR"' EXIT

    info "다운로드: ${URL}"
    download "$URL" "${TMPDIR}/${ARCHIVE}"

    # SHA256 체크섬 검증
    info "체크섬 검증 중..."
    download "$CHECKSUMS_URL" "${TMPDIR}/checksums.txt"
    EXPECTED=$(grep "${ARCHIVE}" "${TMPDIR}/checksums.txt" | awk '{print $1}')
    if [ -n "$EXPECTED" ]; then
        verify_checksum "${TMPDIR}/${ARCHIVE}" "$EXPECTED"
        ok "체크섬 검증 통과 ✓"
    else
        err "checksums.txt에서 ${ARCHIVE}의 체크섬을 찾을 수 없습니다"
    fi

    info "압축 해제 중..."
    tar -xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"

    info "${INSTALL_DIR}/${BINARY} 에 설치 중..."
    if [ -w "$INSTALL_DIR" ]; then
        cp "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
        chmod +x "${INSTALL_DIR}/${BINARY}"
    else
        sudo cp "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
        sudo chmod +x "${INSTALL_DIR}/${BINARY}"
    fi

    # macOS quarantine 속성 제거
    if [ "$OS" = "darwin" ]; then
        xattr -dr com.apple.quarantine "${INSTALL_DIR}/${BINARY}" 2>/dev/null || true
    fi

    ok "autopus-adk v${VERSION} 설치 완료!"
    echo ""
    echo "  ${BINARY} version"
    echo ""
}

main
