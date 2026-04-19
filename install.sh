#!/bin/sh
set -e

# autopus-adk 설치 스크립트
# 사용법: curl -fsSL https://get.autopus.co | sh
#
# 옵션 (환경변수):
#   INSTALL_DIR   — 설치 경로 (기본: /usr/local/bin)
#   VERSION       — 특정 버전 지정 (기본: 최신)

REPO="Insajin/autopus-adk"
BINARY="auto"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
ALIAS="autopus"

# 색상 출력
info()  { printf '\033[1;34m%s\033[0m\n' "$1"; }
ok()    { printf '\033[1;32m%s\033[0m\n' "$1"; }
err()   { printf '\033[1;31m%s\033[0m\n' "$1" >&2; exit 1; }

path_contains_dir() {
    target="$1"
    case ":$PATH:" in
        *":$target:"*) return 0 ;;
        *) return 1 ;;
    esac
}

shell_rc_file() {
    case "${SHELL##*/}" in
        zsh) echo "~/.zshrc" ;;
        bash) echo "~/.bashrc" ;;
        *) echo "~/.profile" ;;
    esac
}

print_path_hint() {
    rc_file="$(shell_rc_file)"
    echo "  설치된 명령어:"
    echo "    auto"
    echo "    ${ALIAS}  # auto alias"
    if path_contains_dir "$INSTALL_DIR"; then
        echo "  PATH 확인: ${INSTALL_DIR}"
        return
    fi
    echo ""
    echo "  현재 셸 PATH에 ${INSTALL_DIR} 가 없습니다."
    echo "  새 셸에서 사용하려면 PATH에 추가하세요:"
    echo "    export PATH=\"${INSTALL_DIR}:\$PATH\""
    echo "  영구 적용:"
    echo "    echo 'export PATH=\"${INSTALL_DIR}:\$PATH\"' >> ${rc_file}"
}

print_next_steps() {
    echo "  다음 단계:"
    echo "    ${BINARY} init"
    echo "      현재 프로젝트를 초기화합니다."
    echo "      설치된 AI 코딩 CLI를 감지해 autopus.yaml과 플랫폼별 하네스 파일을 생성합니다."
    echo ""
    echo "    ${BINARY} update --self"
    echo "      auto CLI 바이너리 자체를 최신 릴리즈로 업데이트합니다."
    echo ""
    echo "    ${BINARY} update"
    echo "      현재 프로젝트의 규칙, 스킬, 에이전트, 설정 파일을 최신 템플릿으로 갱신합니다."
    echo ""
    echo "  권장 순서:"
    echo "    1. 새 프로젝트면 ${BINARY} init"
    echo "    2. 새 릴리즈를 받았으면 ${BINARY} update --self"
    echo "    3. 그 다음 프로젝트 안에서 ${BINARY} update"
}

# OS 감지
detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*)
            err "Windows 네이티브 환경은 지원하지 않습니다.
  현재 지원 OS: macOS, Linux
  Windows 사용자는 WSL2를 통해 설치할 수 있습니다:
  https://learn.microsoft.com/windows/wsl/install" ;;
        *)
            err "지원하지 않는 OS입니다: $(uname -s)
  현재 지원 OS: macOS, Linux" ;;
    esac
}

# 아키텍처 감지
detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)   echo "amd64" ;;
        arm64|aarch64)  echo "arm64" ;;
        *)
            err "지원하지 않는 아키텍처입니다: $(uname -m)
  현재 지원 아키텍처: x86_64 (amd64), arm64 (aarch64)" ;;
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
        echo "  ⚠ 다운로드 파일 무결성 검증 도구를 찾을 수 없습니다."
        echo "    macOS: 기본 포함(shasum)이므로 터미널을 재시작해보세요."
        echo "    Linux: sudo apt install coreutils (또는 yum install coreutils)"
        echo "  체크섬 검증을 건너뜁니다."
        return 0
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
    if [ -w "$INSTALL_DIR" ] || { [ ! -e "$INSTALL_DIR" ] && mkdir -p "$INSTALL_DIR" 2>/dev/null; }; then
        cp "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
        chmod +x "${INSTALL_DIR}/${BINARY}"
        ln -sf "${INSTALL_DIR}/${BINARY}" "${INSTALL_DIR}/${ALIAS}"
        USED_SUDO=""
    else
        echo ""
        echo "  시스템 폴더(${INSTALL_DIR})에 설치하기 위해 관리자 비밀번호가 필요합니다."
        sudo mkdir -p "$INSTALL_DIR"
        sudo cp "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
        sudo chmod +x "${INSTALL_DIR}/${BINARY}"
        sudo ln -sf "${INSTALL_DIR}/${BINARY}" "${INSTALL_DIR}/${ALIAS}"
        USED_SUDO="sudo"
    fi

    # macOS: clear Gatekeeper quarantine/provenance so unsigned binary can run
    if [ "$OS" = "darwin" ]; then
        $USED_SUDO xattr -c "${INSTALL_DIR}/${BINARY}" 2>/dev/null || true
        $USED_SUDO xattr -c "${INSTALL_DIR}/${ALIAS}" 2>/dev/null || true
    fi

    ok "autopus-adk v${VERSION} 설치 완료!"
    echo ""
    print_path_hint
    echo ""

    # Post-install: check and auto-install required tools only.
    info "필수 도구 확인 중... (이미 설치된 것은 건너뜀)"
    if "${INSTALL_DIR}/${BINARY}" doctor --fix --yes --required-only 2>/dev/null; then
        ok "필수 도구 점검 완료!"
    else
        echo "  일부 필수 도구를 자동 설치하지 못했습니다."
        echo "  수동 확인: ${BINARY} doctor"
    fi
    echo ""

    ok "🐙 Autopus-ADK 준비 완료!"
    echo ""
    print_next_steps
    echo ""
}

main
