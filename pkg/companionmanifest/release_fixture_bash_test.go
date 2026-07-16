package companionmanifest

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	wrapperRoot, err := os.MkdirTemp("", "autopus-release-bash-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	wrapper := filepath.Join(wrapperRoot, "bash")
	if err := os.WriteFile(wrapper, []byte(releaseFixtureBash), 0o700); err != nil {
		fmt.Fprintln(os.Stderr, err)
		_ = os.RemoveAll(wrapperRoot)
		os.Exit(2)
	}
	previousPath := os.Getenv("PATH")
	_ = os.Setenv("PATH", wrapperRoot+string(os.PathListSeparator)+previousPath)
	code := m.Run()
	_ = os.Setenv("PATH", previousPath)
	_ = os.RemoveAll(wrapperRoot)
	os.Exit(code)
}

const releaseFixtureBash = `#!/bin/bash
set -euo pipefail
case "${1-}" in
  scripts/companion-release/produce.sh|*/scripts/companion-release/produce.sh)
    source_script="$1"
    shift
    fixture_root=$(mktemp -d "${TMPDIR:-/tmp}/companion-producer-fixture.XXXXXX")
    trap 'rm -rf -- "$fixture_root"' EXIT
    cp -R -- "$(dirname -- "$source_script")" "$fixture_root/companion-release"
    fixture_script="$fixture_root/companion-release/produce.sh"
    /usr/bin/sed \
      -e 's|uname -s|printf Darwin|' \
      -e 's|codesign_tool=/usr/bin/codesign|codesign_tool="$COMPANION_CODESIGN_TOOL"|' \
      -e 's|ditto_tool=/usr/bin/ditto|ditto_tool="$COMPANION_DITTO_TOOL"|' \
      -e 's|xcrun_tool=/usr/bin/xcrun|xcrun_tool="$COMPANION_XCRUN_TOOL"|' \
      -e 's|plutil_tool=/usr/bin/plutil|plutil_tool="$COMPANION_PLUTIL_TOOL"|' \
      -e 's|shasum_tool=/usr/bin/shasum|shasum_tool="$COMPANION_SHASUM_TOOL"|' \
      -e 's|env -i PATH="$PATH" HOME="${HOME-}" TMPDIR="${TMPDIR:-/tmp}"|env|' \
      "$fixture_script" >"$fixture_script.next"
    mv -- "$fixture_script.next" "$fixture_script"
    /bin/bash "$fixture_script" "$@"
    ;;
  *) exec /bin/bash "$@" ;;
esac
`
