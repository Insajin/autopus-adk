package autopusadk_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

func readInstallerFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestWindowsInstallerGitBashPathHintEscapesColon(t *testing.T) {
	script := readInstallerFile(t, "install.ps1")

	if strings.Contains(script, "\"$bashPath:`$PATH\"") {
		t.Fatalf("Git Bash PATH hint uses $bashPath: in a double-quoted PowerShell string; use ${bashPath}: so PowerShell does not parse the colon as part of the variable reference")
	}
	if !strings.Contains(script, "\"${bashPath}:`$PATH\"") {
		t.Fatalf("Git Bash PATH hint should interpolate ${bashPath}: before escaped $PATH")
	}
}

func TestPOSIXInstallerSigningWiringIsFailClosed(t *testing.T) {
	script := readInstallerFile(t, "install.sh")
	helper := readInstallerFile(t, "scripts/release-signing/verify-checksums-v1.sh")
	sum := sha256.Sum256([]byte(helper))
	wantPin := `VERIFIER_SHA256="` + hex.EncodeToString(sum[:]) + `"`
	if !strings.Contains(script, wantPin) {
		t.Fatalf("install.sh must pin the exact V1 verifier digest %q", wantPin)
	}

	ordered := []string{
		`download "$VERIFIER_URL" "$VERIFIER_PATH"`,
		`verify_checksum "$VERIFIER_PATH" "$VERIFIER_SHA256"`,
		`. "$VERIFIER_PATH"`,
		`download "$SIGNATURES_URL"`,
		`verify_release_checksums_v1`,
		`verify_checksum "${TMPDIR}/${ARCHIVE}"`,
		`tar -xzf`,
	}
	position := -1
	for _, fragment := range ordered {
		next := strings.Index(script, fragment)
		if next <= position {
			t.Fatalf("installer trust sequence missing or out of order at %q", fragment)
		}
		position = next
	}
	if strings.Contains(script, `checksums.txt.sig"`) {
		t.Fatal("legacy bare .sig asset must not return")
	}
}
