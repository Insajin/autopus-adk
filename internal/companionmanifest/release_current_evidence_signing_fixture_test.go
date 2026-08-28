package companionmanifest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func (f *currentReleaseFixture) writeSignatureMocks() error {
	bin := filepath.Join(f.root, "bin")
	cosignMock := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
[[ -z "${GITHUB_TOKEN+x}" && -z "${GH_TOKEN+x}" ]]
printf 'cosign %%s\n' "$*" >> %q
[[ "${1-}" == verify-blob ]]
if [[ %t == true ]]; then exit 92; fi
`, f.signatureLog, f.cosignFail)
	if err := os.WriteFile(filepath.Join(bin, "cosign"), []byte(cosignMock), 0o700); err != nil {
		return err
	}
	realOpenSSL, err := exec.LookPath("openssl")
	if err != nil {
		return err
	}
	opensslMock := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
[[ -z "${GITHUB_TOKEN+x}" && -z "${GH_TOKEN+x}" ]]
if [[ "${1-}" == dgst ]]; then
  for argument in "$@"; do
    if [[ "$argument" == -verify ]]; then
      printf 'openssl-verify\n' >> %q
      if [[ %t == true ]]; then exit 92; fi
      exit 0
    fi
  done
fi
exec %q "$@"
`, f.signatureLog, f.openSSLVerifyFail, realOpenSSL)
	return os.WriteFile(filepath.Join(bin, "openssl"), []byte(opensslMock), 0o700)
}
