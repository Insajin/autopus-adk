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

func TestPOSIXInstallerBoundedCommandTimeoutCleansProcessTree(t *testing.T) {
	tempDir := t.TempDir()
	childPIDPath := filepath.Join(tempDir, "child.pid")
	helperPath := filepath.Join(tempDir, "hang.sh")
	helper := `#!/bin/sh
trap '' TERM
sleep 30 &
child_pid=$!
printf '%s\n' "$child_pid" > "$1"
wait "$child_pid"
`
	if err := os.WriteFile(helperPath, []byte(helper), 0o755); err != nil {
		t.Fatalf("write hanging helper: %v", err)
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
`, "bounded-installer-test", tempDir, helperPath, childPIDPath)
	started := time.Now()
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("bounded command unexpectedly succeeded: %s", output)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 124 {
		t.Fatalf("bounded command exit = %v, want 124; output: %s", err, output)
	}
	if elapsed := time.Since(started); elapsed > 8*time.Second {
		t.Fatalf("bounded command took %s, want at most 8s", elapsed)
	}

	rawPID, err := os.ReadFile(childPIDPath)
	if err != nil {
		t.Fatalf("read child pid: %v", err)
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(string(rawPID)))
	if err != nil {
		t.Fatalf("parse child pid %q: %v", rawPID, err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		err = syscall.Kill(childPID, 0)
		if errors.Is(err, syscall.ESRCH) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed-out command left child process %d alive: %v", childPID, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	assertNoInstallerBoundedState(t, tempDir)
}

func TestPOSIXInstallerVersionSmokeRejectsNonExactOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		wantOK bool
	}{
		{name: "exact version and final LF", output: "0.50.85\n", wantOK: true},
		{name: "empty output", output: "", wantOK: false},
		{name: "wrong version", output: "0.50.84\n", wantOK: false},
		{name: "missing final LF", output: "0.50.85", wantOK: false},
		{name: "extra line", output: "0.50.85\nextra\n", wantOK: false},
		{name: "trailing whitespace", output: "0.50.85 \n", wantOK: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outputPath := filepath.Join(t.TempDir(), "version.out")
			if err := os.WriteFile(outputPath, []byte(test.output), 0o600); err != nil {
				t.Fatalf("write version output: %v", err)
			}
			command := exec.Command("sh", "-c", `
set -eu
AUTOPUS_INSTALLER_TEST_SOURCE=1
export AUTOPUS_INSTALLER_TEST_SOURCE
. ./install.sh
. ./scripts/install-runtime-v1.sh
version_smoke_output_matches 0.50.85 "$1"
`, "version-smoke-test", outputPath)
			err := command.Run()
			if test.wantOK && err != nil {
				t.Fatalf("exact version output rejected: %v", err)
			}
			if !test.wantOK && err == nil {
				t.Fatalf("non-exact version output %q accepted", test.output)
			}
		})
	}
}

func TestPOSIXInstallerTimeoutKillsReparentedTermResistantChild(t *testing.T) {
	tempDir := t.TempDir()
	childPIDPath := filepath.Join(tempDir, "child.pid")
	helperPath := filepath.Join(tempDir, "root-exits-on-term.sh")
	helper := `#!/bin/sh
trap 'exit 0' TERM
sh -c 'trap "" TERM; exec sleep 30' </dev/null >/dev/null 2>&1 &
child_pid=$!
printf '%s\n' "$child_pid" > "$1"
while :; do sleep 30; done
`
	if err := os.WriteFile(helperPath, []byte(helper), 0o755); err != nil {
		t.Fatalf("write hanging helper: %v", err)
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
`, "reparented-installer-test", tempDir, helperPath, childPIDPath)
	started := time.Now()
	output, err := command.CombinedOutput()
	if elapsed := time.Since(started); elapsed > 8*time.Second {
		t.Fatalf("reparent cleanup took %s, want at most 8s; output: %s", elapsed, output)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 124 {
		t.Fatalf("bounded command exit = %v, want 124; output: %s", err, output)
	}

	rawPID, err := os.ReadFile(childPIDPath)
	if err != nil {
		t.Fatalf("read child pid: %v", err)
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(string(rawPID)))
	if err != nil {
		t.Fatalf("parse child pid %q: %v", rawPID, err)
	}
	childNeedsCleanup := true
	t.Cleanup(func() {
		if childNeedsCleanup {
			_ = syscall.Kill(childPID, syscall.SIGKILL)
		}
	})
	deadline := time.Now().Add(2 * time.Second)
	for {
		err = syscall.Kill(childPID, 0)
		if errors.Is(err, syscall.ESRCH) {
			childNeedsCleanup = false
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed-out command left reparented child %d alive: %v", childPID, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	assertNoInstallerBoundedState(t, tempDir)
}

func TestPOSIXInstallerSignalSnapshotRejectsReusedPIDIdentity(t *testing.T) {
	target := exec.Command("sleep", "30")
	if err := target.Start(); err != nil {
		t.Fatalf("start signal target: %v", err)
	}
	t.Cleanup(func() {
		_ = target.Process.Kill()
		_, _ = target.Process.Wait()
	})

	snapshotPath := filepath.Join(t.TempDir(), "stale.snapshot")
	stale := strconv.Itoa(target.Process.Pid) + "\tstale process identity\n"
	if err := os.WriteFile(snapshotPath, []byte(stale), 0o600); err != nil {
		t.Fatalf("write stale process snapshot: %v", err)
	}
	command := exec.Command("sh", "-c", `
set -eu
AUTOPUS_INSTALLER_TEST_SOURCE=1
export AUTOPUS_INSTALLER_TEST_SOURCE
. ./install.sh
. ./scripts/install-runtime-v1.sh
signal_process_snapshot "$1" TERM
`, "snapshot-identity-test", snapshotPath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("signal stale snapshot: %v\n%s", err, output)
	}
	if err := target.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("stale PID identity signaled live unrelated process: %v", err)
	}
}

func TestPOSIXInstallerVersionSmokeBoundsCapturedOutput(t *testing.T) {
	tempDir := t.TempDir()
	versionWriter, err := exec.LookPath("yes")
	if err != nil {
		t.Fatalf("find oversized-output fixture: %v", err)
	}
	outputPath := filepath.Join(tempDir, "version.out")
	command := exec.Command("sh", "-c", `
AUTOPUS_INSTALLER_TEST_SOURCE=1
export AUTOPUS_INSTALLER_TEST_SOURCE
. ./install.sh
. ./scripts/install-runtime-v1.sh
TMPDIR=$1
export TMPDIR
VERSION_SMOKE_TIMEOUT_SECONDS=3
run_version_smoke "$2" "$3"
`, "version-output-cap-test", tempDir, versionWriter, outputPath)
	started := time.Now()
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("oversized version output unexpectedly succeeded: %s", output)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() == 124 {
		t.Fatalf("output cap relied on the command timeout: %v; output: %s", err, output)
	}
	if elapsed := time.Since(started); elapsed > 8*time.Second {
		t.Fatalf("oversized version output took %s, want at most 8s", elapsed)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("stat capped version output: %v", err)
	}
	if info.Size() > 4096 {
		t.Fatalf("version output grew to %d bytes, want at most 4096", info.Size())
	}
	assertNoInstallerBoundedState(t, tempDir)
}

func assertNoInstallerBoundedState(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read bounded state parent: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "autopus-command.") {
			t.Fatalf("bounded command left private state %s", entry.Name())
		}
	}
}
