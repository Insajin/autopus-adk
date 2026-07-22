//go:build darwin || linux

package main

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

const expectedFixtureVersion = "0.50.84"

func TestMain(testMain *testing.M) {
	mode := os.Getenv("AUTOPUS_EXEC_SMOKE_FIXTURE")
	if mode != "" {
		os.Exit(runFixture(mode))
	}
	if filepath.Base(os.Args[0]) == "auto" && len(os.Args) == 3 &&
		os.Args[1] == "version" && os.Args[2] == "--short" {
		_, _ = os.Stdout.WriteString(expectedFixtureVersion + "\n")
		os.Exit(0)
	}
	os.Exit(testMain.Run())
}

func TestRunVersionSmoke_ExactVersionAndArguments_Succeeds(t *testing.T) {
	t.Parallel()
	artifact := testArtifact(t)

	err := runVersionSmoke(smokeConfig{
		artifact:         artifact,
		expectedVersion:  expectedFixtureVersion,
		timeout:          2 * time.Second,
		pipeWait:         100 * time.Millisecond,
		extraEnvironment: []string{"AUTOPUS_EXEC_SMOKE_FIXTURE=success"},
	})

	if err != nil {
		t.Fatalf("runVersionSmoke() error = %v", err)
	}
}

func TestRunVersionSmoke_WrongOrMalformedVersion_Fails(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		mode string
	}{
		{name: "wrong version", mode: "wrong-version"},
		{name: "trailing output", mode: "trailing-output"},
		{name: "missing newline", mode: "missing-newline"},
		{name: "leading whitespace", mode: "leading-whitespace"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			artifact := testArtifact(t)

			err := runVersionSmoke(smokeConfig{
				artifact:         artifact,
				expectedVersion:  expectedFixtureVersion,
				timeout:          2 * time.Second,
				pipeWait:         100 * time.Millisecond,
				extraEnvironment: []string{"AUTOPUS_EXEC_SMOKE_FIXTURE=" + test.mode},
			})

			if !errors.Is(err, errVersionMismatch) {
				t.Fatalf("runVersionSmoke() error = %v, want errVersionMismatch", err)
			}
		})
	}
}

func TestRunVersionSmoke_NonzeroOrSignal_Fails(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		mode string
	}{
		{name: "nonzero", mode: "nonzero"},
		{name: "signal", mode: "signal"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			artifact := testArtifact(t)

			err := runVersionSmoke(smokeConfig{
				artifact:         artifact,
				expectedVersion:  expectedFixtureVersion,
				timeout:          2 * time.Second,
				pipeWait:         100 * time.Millisecond,
				extraEnvironment: []string{"AUTOPUS_EXEC_SMOKE_FIXTURE=" + test.mode},
			})

			if err == nil || errors.Is(err, errVersionMismatch) {
				t.Fatalf("runVersionSmoke() error = %v, want execution failure", err)
			}
		})
	}
}

func TestRunVersionSmoke_Timeout_KillsProcessGroup(t *testing.T) {
	t.Parallel()
	childPIDPath := filepath.Join(t.TempDir(), "child.pid")
	artifact := testArtifact(t)
	started := time.Now()

	err := runVersionSmoke(smokeConfig{
		artifact:        artifact,
		expectedVersion: expectedFixtureVersion,
		timeout:         500 * time.Millisecond,
		pipeWait:        100 * time.Millisecond,
		extraEnvironment: []string{
			"AUTOPUS_EXEC_SMOKE_FIXTURE=timeout",
			"AUTOPUS_EXEC_SMOKE_PID_PATH=" + childPIDPath,
		},
	})

	if !errors.Is(err, errExecutionTimeout) {
		t.Fatalf("runVersionSmoke() error = %v, want errExecutionTimeout", err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("runVersionSmoke() elapsed = %v, want bounded execution", elapsed)
	}
	assertRecordedProcessGone(t, childPIDPath)
}

func TestRunVersionSmoke_InheritedPipeHang_FailsAndKillsProcessGroup(t *testing.T) {
	t.Parallel()
	childPIDPath := filepath.Join(t.TempDir(), "child.pid")
	artifact := testArtifact(t)
	started := time.Now()

	err := runVersionSmoke(smokeConfig{
		artifact:        artifact,
		expectedVersion: expectedFixtureVersion,
		timeout:         2 * time.Second,
		pipeWait:        100 * time.Millisecond,
		extraEnvironment: []string{
			"AUTOPUS_EXEC_SMOKE_FIXTURE=inherited-pipe",
			"AUTOPUS_EXEC_SMOKE_PID_PATH=" + childPIDPath,
		},
	})

	if !errors.Is(err, errInheritedPipe) {
		t.Fatalf("runVersionSmoke() error = %v, want errInheritedPipe", err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("runVersionSmoke() elapsed = %v, want bounded pipe cleanup", elapsed)
	}
	assertRecordedProcessGone(t, childPIDPath)
}

func TestRunVersionSmoke_ParentEnvironment_IsNotInherited(t *testing.T) {
	t.Setenv("AUTOPUS_EXEC_SMOKE_SECRET_SENTINEL", "must-not-leak")
	artifact := testArtifact(t)

	err := runVersionSmoke(smokeConfig{
		artifact:         artifact,
		expectedVersion:  expectedFixtureVersion,
		timeout:          2 * time.Second,
		pipeWait:         100 * time.Millisecond,
		extraEnvironment: []string{"AUTOPUS_EXEC_SMOKE_FIXTURE=environment"},
	})

	if err != nil {
		t.Fatalf("runVersionSmoke() error = %v", err)
	}
}

func testArtifact(t *testing.T) string {
	t.Helper()
	path, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	return path
}

func runFixture(mode string) int {
	switch mode {
	case "success":
		if len(os.Args) != 3 || os.Args[1] != "version" || os.Args[2] != "--short" {
			return 91
		}
		_, _ = os.Stdout.WriteString(expectedFixtureVersion + "\n")
		return 0
	case "wrong-version":
		_, _ = os.Stdout.WriteString("0.50.83\n")
		return 0
	case "trailing-output":
		_, _ = os.Stdout.WriteString(expectedFixtureVersion + "\nextra\n")
		return 0
	case "missing-newline":
		_, _ = os.Stdout.WriteString(expectedFixtureVersion)
		return 0
	case "leading-whitespace":
		_, _ = os.Stdout.WriteString(" " + expectedFixtureVersion + "\n")
		return 0
	case "nonzero":
		return 42
	case "signal":
		_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
		return 93
	case "timeout", "inherited-pipe":
		child, err := startFixtureDescendant()
		if err != nil {
			return 94
		}
		if err := os.WriteFile(os.Getenv("AUTOPUS_EXEC_SMOKE_PID_PATH"),
			[]byte(strconv.Itoa(child.Process.Pid)+"\n"), 0o600); err != nil {
			return 95
		}
		if mode == "inherited-pipe" {
			_, _ = os.Stdout.WriteString(expectedFixtureVersion + "\n")
			return 0
		}
		if err := child.Wait(); err != nil {
			return 96
		}
		return 0
	case "grandchild":
		for {
			time.Sleep(time.Second)
		}
	case "environment":
		if os.Getenv("AUTOPUS_EXEC_SMOKE_SECRET_SENTINEL") != "" {
			return 92
		}
		_, _ = os.Stdout.WriteString(expectedFixtureVersion + "\n")
		return 0
	default:
		return 90
	}
}

func startFixtureDescendant() (*exec.Cmd, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	command := exec.Command(executable)
	command.Env = replaceEnvironment(os.Environ(),
		"AUTOPUS_EXEC_SMOKE_FIXTURE", "grandchild")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		return nil, err
	}
	return command, nil
}

func replaceEnvironment(environment []string, name, value string) []string {
	prefix := name + "="
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}
	return append(result, prefix+value)
}

func assertRecordedProcessGone(t *testing.T, pidPath string) {
	t.Helper()
	contents, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", pidPath, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(contents)))
	if err != nil {
		t.Fatalf("recorded PID = %q: %v", contents, err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		err = syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("descendant PID %d still exists after process-group cleanup", pid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
