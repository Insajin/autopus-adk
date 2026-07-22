//go:build !windows

package autopusadk_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPOSIXInstallerSnapshotOrdersAncestorsBeforeDescendants(t *testing.T) {
	tempDir := t.TempDir()
	fakeBin := filepath.Join(tempDir, "bin")
	if err := os.Mkdir(fakeBin, 0o700); err != nil {
		t.Fatalf("create fake bin: %v", err)
	}
	fakePS := `#!/bin/sh
if [ "${1:-}" = -eo ]; then
    printf '%s\n' '103 102' '101 100' '100 1' '102 101'
    exit 0
fi
exit 1
`
	if err := os.WriteFile(filepath.Join(fakeBin, "ps"), []byte(fakePS), 0o755); err != nil {
		t.Fatalf("write fake ps: %v", err)
	}
	snapshotPath := filepath.Join(tempDir, "processes.snapshot")
	command := exec.Command("sh", "-c", `
set -eu
. ./scripts/install-runtime-v1.sh
process_identity() { printf 'identity-%s\n' "$1"; }
capture_process_snapshot 100 "$1"
cat "$1"
`, "snapshot-order-test", snapshotPath)
	command.Env = append(os.Environ(),
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("capture ordered snapshot: %v\n%s", err, output)
	}
	var got []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.SplitN(line, "\t", 2)
		got = append(got, fields[0])
	}
	if joined := strings.Join(got, ","); joined != "100,101,102,103,100" {
		t.Fatalf("snapshot order = %s, want ancestors before descendants", joined)
	}
}

func TestPOSIXInstallerSnapshotFailureKeepsLaunchPayloadInCleanup(t *testing.T) {
	tempDir := t.TempDir()
	realPS, err := exec.LookPath("ps")
	if err != nil {
		t.Fatalf("find ps: %v", err)
	}
	fakeBin := filepath.Join(tempDir, "bin")
	if err := os.Mkdir(fakeBin, 0o700); err != nil {
		t.Fatalf("create fake bin: %v", err)
	}
	fakePS := `#!/bin/sh
printf '%s\n' "$*" >> "$AUTOPUS_TEST_PS_LOG"
if [ "${1:-}" = -eo ]; then exit 1; fi
exec "$AUTOPUS_TEST_REAL_PS" "$@"
`
	if err := os.WriteFile(filepath.Join(fakeBin, "ps"), []byte(fakePS), 0o755); err != nil {
		t.Fatalf("write fake ps: %v", err)
	}
	payloadPIDPath := filepath.Join(tempDir, "payload.pid")
	payloadPath := filepath.Join(tempDir, "payload.sh")
	payload := `#!/bin/sh
trap '' TERM
printf '%s\n' "$$" > "$1"
while :; do :; done
`
	if err := os.WriteFile(payloadPath, []byte(payload), 0o755); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	command := exec.Command("sh", "-c", `
set -eu
AUTOPUS_INSTALLER_TEST_SOURCE=1
export AUTOPUS_INSTALLER_TEST_SOURCE
. ./install.sh
. ./scripts/install-runtime-v1.sh
PROCESS_TERMINATION_GRACE_SECONDS=1
TMPDIR=$1
export TMPDIR
run_bounded_command 3 "$2" "$3"
`, "snapshot-failure-test", tempDir, payloadPath, payloadPIDPath)
	command.Env = append(os.Environ(),
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"AUTOPUS_TEST_REAL_PS="+realPS,
		"AUTOPUS_TEST_PS_LOG="+filepath.Join(tempDir, "ps.log"),
	)
	started := time.Now()
	output, err := command.CombinedOutput()
	if elapsed := time.Since(started); elapsed > 8*time.Second {
		t.Fatalf("snapshot fallback took %s, want at most 8s; output: %s", elapsed, output)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 124 {
		t.Fatalf("bounded command exit = %v, want 124; output: %s", err, output)
	}
	psLog, err := os.ReadFile(filepath.Join(tempDir, "ps.log"))
	if err != nil || !strings.Contains(string(psLog), "-eo pid=,ppid=") {
		t.Fatalf("fake ps did not exercise tree snapshot failure: %v, log=%q", err, psLog)
	}
	rawPID, err := os.ReadFile(payloadPIDPath)
	if err != nil {
		t.Fatalf("read payload pid: %v", err)
	}
	payloadPID, err := strconv.Atoi(strings.TrimSpace(string(rawPID)))
	if err != nil {
		t.Fatalf("parse payload pid %q: %v", rawPID, err)
	}
	payloadNeedsCleanup := true
	t.Cleanup(func() {
		if payloadNeedsCleanup {
			_ = syscall.Kill(payloadPID, syscall.SIGKILL)
		}
	})
	deadline := time.Now().Add(2 * time.Second)
	for {
		err = syscall.Kill(payloadPID, 0)
		if errors.Is(err, syscall.ESRCH) {
			payloadNeedsCleanup = false
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("snapshot failure left launch payload %d alive: %v", payloadPID, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	assertNoInstallerBoundedState(t, tempDir)
}
