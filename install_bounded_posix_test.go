//go:build !windows

package autopusadk_test

import (
	"bytes"
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

// boundedRun 은 게이트가 걸린 경계 명령 한 번의 관측치다. elapsed 는 프로세스
// 시작이 아니라 경계가 무장된 시점부터 잰다.
type boundedRun struct {
	childPID int
	elapsed  time.Duration
	err      error
	output   string
}

// installerClockGate 는 PATH 앞에 놓일 sleep 대체물을 만든다. 픽스처가
// readyPath 에 자식 PID 를 게시하기 전까지는 어떤 sleep 도 출발하지 않으므로,
// 설치기의 종료 경계는 픽스처 준비가 끝난 뒤에야 카운트를 시작한다. 경계의
// 길이도 유예 시간도 그대로 진짜 sleep 이 잰다 - 고정되는 것은 출발 시점뿐이다.
//
// 이 게이트가 대신하는 것은 예산이 아니라 순서다. 예전에는 픽스처의 sh exec 와
// 프로세스 두 개 생성이 측정 대상인 경계와 같은 시계를 나눠 썼고, 포화된
// 러너에서 픽스처가 지면 child.pid 가 비어 "read child pid" 로 실패했다.
// 예산을 3s -> 6s 로 넓히는 방식은 그 경쟁을 없애지 못했다.
func installerClockGate(t *testing.T, dir, readyPath string) string {
	t.Helper()
	gateDir := filepath.Join(dir, "clock-gate")
	if err := os.Mkdir(gateDir, 0o755); err != nil {
		t.Fatalf("create installer clock gate: %v", err)
	}
	realSleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Fatalf("find sleep for installer clock gate: %v", err)
	}
	gate := "#!/bin/sh\n" +
		"while [ ! -s '" + readyPath + "' ]; do\n" +
		"    '" + realSleep + "' 0.05 2>/dev/null || '" + realSleep + "' 1\n" +
		"done\n" +
		"exec '" + realSleep + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(gateDir, "sleep"), []byte(gate), 0o755); err != nil {
		t.Fatalf("write installer clock gate: %v", err)
	}
	return gateDir
}

// runGatedBoundedCommand 는 경계 명령을 띄우고 픽스처가 term 저항 자식을
// 게시할 때까지 기다린 다음, 그 시점부터 경과 시간을 잰다. 기다림과 경계가
// 같은 시계를 다투지 않으므로 준비 과정이 경계에 져서 실패하는 일은 없다.
func runGatedBoundedCommand(t *testing.T, script, label, tempDir, helperPath, childPIDPath string) boundedRun {
	t.Helper()
	gateDir := installerClockGate(t, tempDir, childPIDPath)
	command := exec.Command("sh", "-c", script, label, tempDir, helperPath, childPIDPath)
	command.Env = append(os.Environ(), "PATH="+gateDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	// 픽스처 전체를 별도 프로세스 그룹에 담는다. 재부모화된 자식도 그룹은
	// 그대로 유지하므로, 어떤 실패 경로에서든 트리를 통째로 거둘 수 있다.
	// 그룹 식별자는 설치기가 쓰는 pgid 동일성 판정에 일관되게 반영된다.
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		t.Fatalf("start bounded command: %v", err)
	}
	// 그룹 정리는 명령이 아직 살아 있는 실패 경로에서만 한다. 명령이 끝난
	// 뒤에는 리더 PID 가 회수되어 재사용될 수 있으므로 -pgid 를 쏘지 않는다.
	group := command.Process.Pid
	waited := make(chan error, 1)
	go func() { waited <- command.Wait() }()

	childPID := awaitFixtureChild(t, childPIDPath, waited, group, &output)
	armedAt := time.Now()
	err := <-waited
	return boundedRun{childPID: childPID, elapsed: time.Since(armedAt), err: err, output: output.String()}
}

// awaitFixtureChild 는 픽스처가 게시한 자식 PID 를 읽는다. 게이트가 설치기
// 타이머를 붙잡고 있으므로 이 대기에는 경계와 나눠 쓸 예산이 없다. 상한은
// 픽스처가 멈춘 경우를 잡기 위한 것이지 경쟁하기 위한 것이 아니다.
func awaitFixtureChild(t *testing.T, pidPath string, waited <-chan error, group int, output *bytes.Buffer) int {
	t.Helper()
	stuck := time.After(2 * time.Minute)
	poll := time.NewTicker(5 * time.Millisecond)
	defer poll.Stop()
	for {
		select {
		case err := <-waited:
			t.Fatalf("bounded command exited before the fixture published its child: %v\n%s", err, output.String())
		case <-stuck:
			_ = syscall.Kill(-group, syscall.SIGKILL)
			<-waited
			t.Fatalf("fixture never published its term-resistant child\n%s", output.String())
		case <-poll.C:
			raw, err := os.ReadFile(pidPath)
			if err != nil {
				continue
			}
			pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
			if err != nil {
				continue
			}
			return pid
		}
	}
}

// awaitProcessExit 는 설치기 정리가 대상 프로세스를 실제로 없앴는지 확인한다.
func awaitProcessExit(t *testing.T, pid int, subject string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		if time.Now().After(deadline) {
			// 방금 살아 있음을 확인했으므로 PID 재사용 없이 안전하게 거둔다.
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("timed-out command left %s %d alive: %v", subject, pid, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestPOSIXInstallerBoundedCommandTimeoutCleansProcessTree(t *testing.T) {
	tempDir := t.TempDir()
	childPIDPath := filepath.Join(tempDir, "child.pid")
	helperPath := filepath.Join(tempDir, "hang.sh")
	helper := `#!/bin/sh
trap '' TERM
sleep 30 &
child_pid=$!
printf '%s\n' "$child_pid" > "$1.tmp"
mv "$1.tmp" "$1"
wait "$child_pid"
`
	if err := os.WriteFile(helperPath, []byte(helper), 0o755); err != nil {
		t.Fatalf("write hanging helper: %v", err)
	}

	run := runGatedBoundedCommand(t, `
set -eu
AUTOPUS_INSTALLER_TEST_SOURCE=1
export AUTOPUS_INSTALLER_TEST_SOURCE
. ./install.sh
. ./scripts/install-runtime-v1.sh
PROCESS_TERMINATION_GRACE_SECONDS=1
TMPDIR=$1
export TMPDIR
# PATH 앞의 sleep 게이트가 픽스처의 PID 게시 전에는 설치기 타이머를 붙잡는다.
# 그래서 6 은 준비 시간과 나눠 쓰는 예산이 아니라 온전한 경계다.
run_bounded_command 6 "$2" "$3"
`, "bounded-installer-test", tempDir, helperPath, childPIDPath)
	if run.err == nil {
		t.Fatalf("bounded command unexpectedly succeeded: %s", run.output)
	}
	var exitErr *exec.ExitError
	if !errors.As(run.err, &exitErr) || exitErr.ExitCode() != 124 {
		t.Fatalf("bounded command exit = %v, want 124; output: %s", run.err, run.output)
	}
	if run.elapsed > 12*time.Second {
		t.Fatalf("bounded command took %s after the bound was armed, want at most 12s", run.elapsed)
	}
	awaitProcessExit(t, run.childPID, "child process")
	assertNoInstallerBoundedState(t, tempDir)
}

func TestPOSIXInstallerTimeoutKillsReparentedTermResistantChild(t *testing.T) {
	tempDir := t.TempDir()
	childPIDPath := filepath.Join(tempDir, "child.pid")
	helperPath := filepath.Join(tempDir, "root-exits-on-term.sh")
	helper := `#!/bin/sh
trap 'exit 0' TERM
sh -c 'trap "" TERM; exec sleep 30' </dev/null >/dev/null 2>&1 &
child_pid=$!
printf '%s\n' "$child_pid" > "$1.tmp"
mv "$1.tmp" "$1"
while :; do sleep 30; done
`
	if err := os.WriteFile(helperPath, []byte(helper), 0o755); err != nil {
		t.Fatalf("write hanging helper: %v", err)
	}

	run := runGatedBoundedCommand(t, `
set -eu
AUTOPUS_INSTALLER_TEST_SOURCE=1
export AUTOPUS_INSTALLER_TEST_SOURCE
. ./install.sh
. ./scripts/install-runtime-v1.sh
PROCESS_TERMINATION_GRACE_SECONDS=1
TMPDIR=$1
export TMPDIR
# 헬퍼는 영원히 도는 루프라 경계는 반드시 발동한다. PATH 앞의 sleep 게이트가
# 픽스처의 PID 게시 전에는 설치기 타이머를 붙잡으므로, 재부모화된 term 저항
# 자식은 스냅샷 시점에 이미 존재하고 그 PID 도 이미 테스트가 쥐고 있다.
run_bounded_command 6 "$2" "$3"
`, "reparented-installer-test", tempDir, helperPath, childPIDPath)
	if run.elapsed > 12*time.Second {
		t.Fatalf("reparent cleanup took %s after the bound was armed, want at most 12s; output: %s", run.elapsed, run.output)
	}
	var exitErr *exec.ExitError
	if !errors.As(run.err, &exitErr) || exitErr.ExitCode() != 124 {
		t.Fatalf("bounded command exit = %v, want 124; output: %s", run.err, run.output)
	}
	awaitProcessExit(t, run.childPID, "reparented child")
	assertNoInstallerBoundedState(t, tempDir)
}
