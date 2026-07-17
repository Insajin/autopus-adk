//go:build !windows

package autopusadk_test

import (
	"os/exec"
	"testing"
)

func runPOSIXSigningOracle(t *testing.T, path string) {
	t.Helper()
	command := exec.Command("sh", path)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s failed: %v\n%s", path, err, output)
	}
}

func TestPOSIXInstallerV1EnvelopeOracle(t *testing.T) {
	runPOSIXSigningOracle(t, "scripts/release-signing/tests/posix-v1-envelope-test.sh")
}

func TestPOSIXInstallerV1InstallOracle(t *testing.T) {
	runPOSIXSigningOracle(t, "scripts/release-signing/tests/posix-v1-install-test.sh")
}
