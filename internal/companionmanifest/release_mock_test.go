package companionmanifest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

type mockReleaseTools struct {
	root     string
	signer   string
	verifier string
	tools    map[string]string
}

func newMockReleaseTools(t *testing.T) mockReleaseTools {
	t.Helper()
	root := t.TempDir()
	signer := filepath.Join(root, "auto-companion-manifest-signer")
	verifier := filepath.Join(root, "receipt-verifier")
	buildReleaseBinary(t, signer, "./cmd/auto")
	buildReleaseBinary(t, verifier, "./internal/companionmanifest/receiptverify")
	toolsDir := filepath.Join(root, "tools")
	if err := os.Mkdir(toolsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	tools := map[string]string{}
	write := func(name, body string) {
		path := filepath.Join(toolsDir, name)
		writeExecutable(t, path, "#!/usr/bin/env bash\nset -euo pipefail\n"+body)
		tools[name] = path
	}
	write("codesign", `
if [[ " $* " == *" -dv "* ]]; then
  printf '%s\n' 'Identifier=co.autopus.adk' 'TeamIdentifier=GP2PFA2PUV' \
    'Timestamp=Jul 15, 2026 at 12:00:00' \
    'CodeDirectory v=20500 size=512 flags=0x10000(runtime) hashes=8+2 location=embedded' >&2
fi
`)
	write("ditto", `
output=''
for argument in "$@"; do output="$argument"; done
printf 'notary-container' >"$output"
`)
	write("xcrun", `
printf '{"status":"%s","id":"123e4567-e89b-42d3-a456-426614174000"}' "${MOCK_NOTARY_STATUS:-Accepted}"
`)
	write("plutil", `
field="$2"
input=''
for argument in "$@"; do input="$argument"; done
case "$field" in
  status) sed -E 's/.*"status":"([^"]+)".*/\1/' "$input" ;;
  id) sed -E 's/.*"id":"([^"]+)".*/\1/' "$input" ;;
  artifact_digest) sed -E 's/.*"artifact_digest":"([^"]+)".*/\1/' "$input" ;;
  *) exit 1 ;;
esac
`)
	write("exec-smoke-gate", `
artifact='' expected_version='' architecture='' timeout=''
while [[ $# -gt 0 ]]; do
  [[ $# -ge 2 ]] || exit 2
  case "$1" in
    --artifact) artifact="$2" ;;
    --expected-version) expected_version="$2" ;;
    --architecture) architecture="$2" ;;
    --timeout) timeout="$2" ;;
    *) exit 2 ;;
  esac
  shift 2
done
[[ -f "$artifact" && ! -L "$artifact" && -x "$artifact" ]]
[[ "$expected_version" == '0.50.69' && "$timeout" == '15s' ]]
[[ "$architecture" == 'amd64' || "$architecture" == 'arm64' ]]
`)
	shasum, err := exec.LookPath("shasum")
	if err != nil {
		t.Fatal(err)
	}
	write("shasum", fmt.Sprintf("exec %q \"$@\"\n", shasum))
	return mockReleaseTools{root: root, signer: signer, verifier: verifier, tools: tools}
}

func runMockedRelease(
	t *testing.T,
	tools mockReleaseTools,
	architecture, notaryStatus string,
) (string, []byte, error) {
	t.Helper()
	root := t.TempDir()
	artifactDir := filepath.Join(root, "auto_darwin_"+architecture)
	tmpDir := filepath.Join(root, "tmp")
	if err := os.Mkdir(artifactDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(tmpDir, 0o700); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(artifactDir, "auto")
	keyPath := filepath.Join(root, "release-key")
	apiKeyPath := filepath.Join(root, "AuthKey_FIXTURE.p8")
	if err := os.WriteFile(artifact, []byte("artifact-"+architecture), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, encodedReleaseKey(t), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(apiKeyPath, []byte("fixture-api-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("bash", filepath.Join(repositoryRoot(t), "scripts/companion-release/produce.sh"))
	command.Env = append(os.Environ(),
		"TMPDIR="+tmpDir, "MOCK_NOTARY_STATUS="+notaryStatus,
		"GITHUB_REF_NAME=v0.50.69", "COMPANION_ARTIFACT="+artifact,
		"COMPANION_PLATFORM=darwin", "COMPANION_ARCHITECTURE="+architecture,
		"COMPANION_TARGET=darwin_"+architecture, "COMPANION_VERSION=0.50.69",
		"COMPANION_BUILD_PROVENANCE=github-actions:Insajin/autopus-adk@0123456789012345678901234567890123456789",
		"COMPANION_HANDOFF=v1", "COMPANION_ROLLBACK_FLOOR=5069",
		"COMPANION_ISSUED_AT=2026-07-15T00:00:00Z",
		"COMPANION_EXPIRES_AT=2026-07-16T00:00:00Z",
		"COMPANION_KEY_ID=release-key", "COMPANION_SIGNING_KEY_FILE="+keyPath,
		"COMPANION_SIGNER="+tools.signer, "COMPANION_RECEIPT_VERIFIER="+tools.verifier,
		"COMPANION_EXEC_SMOKE_GATE="+tools.tools["exec-smoke-gate"],
		"COMPANION_PUBLIC_KEY_RECEIPT_ISSUED_AT=2026-07-14T00:00:00Z",
		"COMPANION_PUBLIC_KEY_RECEIPT_EXPIRES_AT=2027-07-15T00:00:00Z",
		"COMPANION_PUBLIC_KEY_RECEIPT_MINIMUM_LIFETIME_SECONDS=31536000",
		"APPLE_SIGNING_IDENTITY=Developer ID Application: Fixture (GP2PFA2PUV)",
		"APPLE_API_KEY=FIXTUREKEY", "APPLE_API_ISSUER=123e4567-e89b-42d3-a456-426614174000",
		"APPLE_API_KEY_PATH="+apiKeyPath,
		"COMPANION_CODESIGN_TOOL="+tools.tools["codesign"],
		"COMPANION_DITTO_TOOL="+tools.tools["ditto"],
		"COMPANION_XCRUN_TOOL="+tools.tools["xcrun"],
		"COMPANION_PLUTIL_TOOL="+tools.tools["plutil"],
		"COMPANION_SHASUM_TOOL="+tools.tools["shasum"],
	)
	output, err := command.CombinedOutput()
	return artifactDir, output, err
}
