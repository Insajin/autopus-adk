// Package hooks_test provides regression tests for .claude/hooks/task-created-validate.sh.
// It exercises the 50-case synthetic payload suite required by QG2 in SPEC-CC21-001.
//
// Run with:
//
//	cd autopus-adk && go test ./tests/hooks/... -v
package hooks_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// hookPath resolves the absolute path to the source-of-truth hook script.
// Layout: autopus-adk/tests/hooks/ → ../.. → repo root → content/hooks/
func hookPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file = .../autopus-adk/tests/hooks/task_created_test.go
	// three filepath.Dir calls = repo root
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	p := filepath.Join(repoRoot, "content", "hooks", "task-created-validate.sh")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("hook not found at %s: %v", p, err)
	}
	return p
}

// runHook executes the hook with the given stdin payload and mode environment variable.
// It returns exit code, combined stdout, stderr, and the path to the tmp audit log.
func runHook(t *testing.T, hook, input, mode string) (exitCode int, stdout, stderr, logPath string) {
	t.Helper()

	// Each invocation writes to a unique log file to avoid append conflicts.
	logPath = filepath.Join(t.TempDir(), "audit.log")

	cmd := exec.Command(hook)
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"TASKCREATED_MODE=" + mode,
		"AUTOPUS_TASK_AUDIT_LOG=" + logPath,
	}
	cmd.Stdin = strings.NewReader(input)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	exitCode = 0
	if exit, ok := err.(*exec.ExitError); ok {
		exitCode = exit.ExitCode()
	} else if err != nil {
		t.Logf("hook exec error (non-exit): %v", err)
	}

	return exitCode, outBuf.String(), errBuf.String(), logPath
}

// lastLogOutcome reads the final JSONL line from the audit log and returns the outcome field.
func lastLogOutcome(t *testing.T, logPath string) string {
	t.Helper()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("cannot read audit log %s: %v", logPath, err)
	}

	var lastLine string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			lastLine = line
		}
	}
	if lastLine == "" {
		t.Fatalf("audit log is empty: %s", logPath)
	}

	var entry struct {
		Outcome string `json:"outcome"`
	}
	if err := json.Unmarshal([]byte(lastLine), &entry); err != nil {
		t.Fatalf("malformed audit log entry %q: %v", lastLine, err)
	}
	return entry.Outcome
}

// allCases assembles the full 50-case table from the sub-slices defined in task_created_cases_test.go.
func allCases() []tcase {
	all := make([]tcase, 0, 50)
	all = append(all, validCases...)
	all = append(all, titleTooShortViolations...)
	all = append(all, badVerbViolations...)
	all = append(all, invalidJSONViolations...)
	return all
}

// TestTaskCreatedHook runs the full 50-case synthetic payload suite (QG2).
func TestTaskCreatedHook(t *testing.T) {
	hook := hookPath(t)
	cases := allCases()

	// Sanity: assert we have exactly 50 cases.
	if len(cases) != 50 {
		t.Fatalf("expected 50 test cases, got %d — update task_created_cases_test.go", len(cases))
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			exitCode, _, errOut, logPath := runHook(t, hook, tc.input, tc.mode)

			// Assert exit code.
			if exitCode != tc.wantExit {
				t.Fatalf("exit code mismatch: got %d, want %d (stderr: %q)", exitCode, tc.wantExit, errOut)
			}

			// Assert audit log outcome field.
			gotOutcome := lastLogOutcome(t, logPath)
			if gotOutcome != tc.wantOutcome {
				t.Errorf("audit outcome mismatch: got %q, want %q (stderr: %q)", gotOutcome, tc.wantOutcome, errOut)
			}

			// For enforce+violation: stderr must be non-empty.
			if tc.mode == "enforce" && tc.wantExit == 2 && strings.TrimSpace(errOut) == "" {
				t.Error("enforce violation must emit stderr message, got empty stderr")
			}
		})
	}
}

// TestTaskCreatedHook_ExecutableBit verifies that the hook file has execute permission (QG1.5).
func TestTaskCreatedHook_ExecutableBit(t *testing.T) {
	hook := hookPath(t)
	info, err := os.Stat(hook)
	if err != nil {
		t.Fatalf("stat hook: %v", err)
	}
	mode := info.Mode()
	if mode&0o111 == 0 {
		t.Fatalf("hook is not executable: mode=%s path=%s", mode, hook)
	}
}

// TestTaskCreatedHook_AuditLogAppend confirms that consecutive invocations append distinct JSONL lines.
func TestTaskCreatedHook_AuditLogAppend(t *testing.T) {
	hook := hookPath(t)
	logPath := filepath.Join(t.TempDir(), "append-audit.log")

	type run struct {
		input string
		mode  string
	}
	runs := []run{
		{`{"task_subject":"add first task here"}`, "warn"},
		{`{"task_subject":"fix"}`, "warn"},
		{`{"task_subject":"update second entry"}`, "enforce"},
	}

	for i, r := range runs {
		cmd := exec.Command(hook)
		cmd.Env = []string{
			"PATH=" + os.Getenv("PATH"),
			"HOME=" + os.Getenv("HOME"),
			"TASKCREATED_MODE=" + r.mode,
			"AUTOPUS_TASK_AUDIT_LOG=" + logPath,
		}
		cmd.Stdin = strings.NewReader(r.input)
		// Ignore exit code — we only care about log appending.
		_ = cmd.Run()
		_ = i
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("audit log not created: %v", err)
	}

	var lineCount int
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "" {
			lineCount++
		}
	}
	if lineCount != len(runs) {
		t.Fatalf("expected %d audit log lines after %d runs, got %d\nlog content:\n%s",
			len(runs), len(runs), lineCount, string(data))
	}
}

// TestTaskCreatedHook_Timeout verifies that the hook exits within the 5-second hard timeout
// when stdin produces no data (simulates a blocked reader). The hook must complete within
// 8 seconds total (5s read timeout + process overhead).
func TestTaskCreatedHook_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in short mode")
	}

	hook := hookPath(t)
	logPath := filepath.Join(t.TempDir(), "timeout-audit.log")

	// Open a pipe and never write to it — hook will block on read until its internal 5s timeout.
	pr, _, _ := os.Pipe()
	defer pr.Close()

	cmd := exec.Command(hook)
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"TASKCREATED_MODE=warn",
		"AUTOPUS_TASK_AUDIT_LOG=" + logPath,
	}
	cmd.Stdin = pr

	start := time.Now()
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start hook: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	deadline := time.After(8 * time.Second)
	select {
	case <-done:
		elapsed := time.Since(start)
		// Hook should exit within 8 seconds — if it exits quickly that is also fine.
		t.Logf("hook exited after %v (within deadline)", elapsed)
	case <-deadline:
		_ = cmd.Process.Kill()
		t.Fatal("hook did not exit within 8s timeout deadline — 5s internal read timeout may be broken")
	}
}

// TestTaskCreatedHook_SpecIDExtraction verifies that SPEC-IDs present in the title are
// captured in the audit log spec_id field.
func TestTaskCreatedHook_SpecIDExtraction(t *testing.T) {
	hook := hookPath(t)
	logPath := filepath.Join(t.TempDir(), "spec-id-audit.log")

	input := `{"task_subject":"add feature for SPEC-CC21-001 integration"}`
	cmd := exec.Command(hook)
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"TASKCREATED_MODE=warn",
		"AUTOPUS_TASK_AUDIT_LOG=" + logPath,
	}
	cmd.Stdin = strings.NewReader(input)
	_ = cmd.Run()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("audit log not created: %v", err)
	}

	lastLine := ""
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		if l := strings.TrimSpace(scanner.Text()); l != "" {
			lastLine = l
		}
	}

	var entry struct {
		SpecID interface{} `json:"spec_id"`
	}
	if err := json.Unmarshal([]byte(lastLine), &entry); err != nil {
		t.Fatalf("malformed audit log entry: %v", err)
	}

	specID := fmt.Sprintf("%v", entry.SpecID)
	if !strings.Contains(specID, "SPEC-CC21-001") {
		t.Errorf("spec_id not extracted: got %q, expected it to contain SPEC-CC21-001\nlog line: %s", specID, lastLine)
	}
}
