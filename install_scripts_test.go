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

	if strings.Contains(script, `$bashPath:`) {
		t.Fatal("Git Bash PATH hint lets PowerShell parse the colon as part of the variable reference")
	}
	if !strings.Contains(script, `'    export PATH="{0}:$PATH"' -f $bashPath`) {
		t.Fatal("Git Bash PATH hint should format the path without nested PowerShell quoting")
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

func TestPOSIXInstallerRuntimeHelperIsPinnedAndFailClosed(t *testing.T) {
	installer := readInstallerFile(t, "install.sh")
	helper := readInstallerFile(t, "scripts/install-runtime-v1.sh")
	sum := sha256.Sum256([]byte(helper))
	wantPin := `RUNTIME_HELPER_SHA256="` + hex.EncodeToString(sum[:]) + `"`
	if !strings.Contains(installer, wantPin) {
		t.Fatalf("install.sh must pin the exact runtime helper digest %q", wantPin)
	}

	ordered := []string{
		`download "$RUNTIME_HELPER_URL" "$RUNTIME_HELPER_PATH"`,
		`[ -f "$RUNTIME_HELPER_PATH" ] && [ ! -L "$RUNTIME_HELPER_PATH" ]`,
		`verify_checksum "$RUNTIME_HELPER_PATH" "$RUNTIME_HELPER_SHA256"`,
		`. "$RUNTIME_HELPER_PATH"`,
	}
	position := -1
	for _, fragment := range ordered {
		next := strings.Index(installer, fragment)
		if next <= position {
			t.Fatalf("runtime helper trust sequence missing or out of order at %q", fragment)
		}
		position = next
	}
	for path, content := range map[string]string{"install.sh": installer, "scripts/install-runtime-v1.sh": helper} {
		if lines := strings.Count(content, "\n"); lines > 300 {
			t.Fatalf("%s has %d lines, want at most 300", path, lines)
		}
	}
}

func TestPOSIXInstallerPostInstallExecutionChecksAreBounded(t *testing.T) {
	script := readInstallerFile(t, "install.sh") + "\n" + readInstallerFile(t, "scripts/install-runtime-v1.sh")

	required := []string{
		`run_bounded_command()`,
		`ulimit -f "$1"`,
		`autopus-version-smoke "$VERSION_SMOKE_FILE_BLOCKS"`,
		`version_smoke_output_matches "$VERSION" "$version_smoke_output"`,
		`run_version_smoke "${INSTALL_DIR}/${BINARY}" "$version_smoke_output"`,
		`run_bounded_command "$DOCTOR_TIMEOUT_SECONDS" "${INSTALL_DIR}/${BINARY}" doctor --fix --yes --required-only`,
		`rm -f "$version_smoke_output"`,
		`macOS 실행 보안 심사가 제한 시간 안에 완료되지 않았습니다.`,
		`시스템 설정 > 개인정보 보호 및 보안`,
		`Mac을 재시동한 뒤`,
		`cat "$bounded_launch_snapshot" >> "$bounded_process_snapshot"`,
	}
	for _, fragment := range required {
		if !strings.Contains(script, fragment) {
			t.Fatalf("install.sh must contain bounded post-install contract %q", fragment)
		}
	}

	smokePosition := strings.Index(script, required[4])
	doctorPosition := strings.Index(script, required[5])
	if smokePosition < 0 || doctorPosition <= smokePosition {
		t.Fatal("version smoke must run before the doctor check")
	}
	if strings.Contains(script, `if "${INSTALL_DIR}/${BINARY}" doctor --fix --yes --required-only`) {
		t.Fatal("doctor must not execute outside the bounded command runner")
	}
	for _, bypass := range []string{
		`xattr -c`,
		`spctl --master-disable`,
		`xattr -d com.apple.quarantine`,
	} {
		if strings.Contains(script, bypass) {
			t.Fatalf("installer must not bypass macOS execution security with %q", bypass)
		}
	}
}

func TestMacOSRuntimeCIExecutesUnsignedBoundedVersionSmoke(t *testing.T) {
	workflow := readInstallerFile(t, ".github/workflows/ci.yaml")

	required := []string{
		`runs-on: macos-15`,
		`go build -trimpath`,
		`github.com/insajin/autopus-adk/pkg/version.version=`,
		`github.com/insajin/autopus-adk/pkg/version.commit=`,
		`github.com/insajin/autopus-adk/pkg/version.date=`,
		`./cmd/auto`,
		`run_version_smoke "$AUTOPUS_CI_RUNTIME_BINARY" "$runtime_output_file"`,
		`version_smoke_output_matches "$AUTOPUS_CI_RUNTIME_VERSION" "$runtime_output_file"`,
		`rm -f "$runtime_output_file"`,
		`./scripts/companion-release/execsmoke`,
		`does not replace the Developer ID signed and notarized release gate`,
	}
	for _, fragment := range required {
		if !strings.Contains(workflow, fragment) {
			t.Fatalf("ci.yaml macos-runtime must contain %q", fragment)
		}
	}

	macOSJob := strings.Index(workflow, "  macos-runtime:")
	windowsJob := strings.Index(workflow, "  windows-runtime:")
	if macOSJob < 0 || windowsJob <= macOSJob {
		t.Fatal("cannot isolate macos-runtime job")
	}
	section := workflow[macOSJob:windowsJob]
	for _, fragment := range required {
		if !strings.Contains(section, fragment) {
			t.Fatalf("macos-runtime job must own runtime smoke fragment %q", fragment)
		}
	}
}

func TestWindowsRuntimeRunsSelfUpdateAdmissionContracts(t *testing.T) {
	workflow := readInstallerFile(t, ".github/workflows/ci.yaml")
	windowsJob := strings.Index(workflow, "  windows-runtime:")
	if windowsJob < 0 {
		t.Fatal("cannot isolate windows-runtime job")
	}
	section := workflow[windowsJob:]
	for _, testPrefix := range []string{
		`TestVerifyAndReplaceSelfUpdate_`,
		`TestProbeStagedSelfUpdateVersion_`,
		`TestNormalizeSelfUpdateVersionOutput_`,
	} {
		if !strings.Contains(section, testPrefix) {
			t.Fatalf("windows-runtime must execute %s tests", testPrefix)
		}
	}
}
