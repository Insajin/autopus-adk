//go:build darwin || linux

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunVerifiedExecSmoke_ExactBodyFreeOutput_Succeeds(t *testing.T) {
	t.Parallel()
	policy := ompCanaryPolicy{version: pinnedOMPVersion, sha256: "sha256:" + strings.Repeat("a", 64)}
	err := runVerifiedExecSmoke(verifiedExecSmokeConfig{
		artifact: testArtifact(t), ompExecutable: "/absolute/omp", policy: policy,
		timeout: 2 * time.Second, pipeWait: 100 * time.Millisecond,
		extraEnvironment: []string{
			"AUTOPUS_EXEC_SMOKE_FIXTURE=verified-exec-output",
			`AUTOPUS_VERIFIED_EXEC_OUTPUT={"schema_version":"workflow-context-verified-exec-smoke/v1","omp_version":"omp/17.2.7","omp_executable_sha256":"sha256:` + strings.Repeat("a", 64) + `","rpc_ready":true,"provider_calls":0,"uid_isolated":true,"effective_user":"nobody"}`,
		},
	})
	if err != nil {
		t.Fatalf("runVerifiedExecSmoke() error = %v", err)
	}
}

func TestRunVerifiedExecSmoke_MalformedOrMismatchedOutput_Fails(t *testing.T) {
	t.Parallel()
	policy := ompCanaryPolicy{version: pinnedOMPVersion, sha256: "sha256:" + strings.Repeat("a", 64)}
	tests := []string{
		`not-json`,
		`{"schema_version":"wrong","omp_version":"omp/17.2.7","omp_executable_sha256":"sha256:` + strings.Repeat("a", 64) + `","rpc_ready":true,"provider_calls":0,"uid_isolated":true,"effective_user":"nobody"}`,
		`{"schema_version":"workflow-context-verified-exec-smoke/v1","omp_version":"omp/17.2.6","omp_executable_sha256":"sha256:` + strings.Repeat("a", 64) + `","rpc_ready":true,"provider_calls":0}`,
		`{"schema_version":"workflow-context-verified-exec-smoke/v1","omp_version":"omp/17.2.7","omp_executable_sha256":"sha256:` + strings.Repeat("b", 64) + `","rpc_ready":true,"provider_calls":0}`,
		`{"schema_version":"workflow-context-verified-exec-smoke/v1","omp_version":"omp/17.2.7","omp_executable_sha256":"sha256:` + strings.Repeat("a", 64) + `","rpc_ready":false,"provider_calls":0}`,
		`{"schema_version":"workflow-context-verified-exec-smoke/v1","omp_version":"omp/17.2.7","omp_executable_sha256":"sha256:` + strings.Repeat("a", 64) + `","rpc_ready":true,"provider_calls":1}`,
		`{"schema_version":"workflow-context-verified-exec-smoke/v1","omp_version":"omp/17.2.7","omp_executable_sha256":"sha256:` + strings.Repeat("a", 64) + `","rpc_ready":true,"provider_calls":0,"extra":true}`,
		`{"schema_version":"workflow-context-verified-exec-smoke/v1","omp_version":"omp/17.2.7","omp_executable_sha256":"sha256:` + strings.Repeat("a", 64) + `","rpc_ready":true,"provider_calls":0,"uid_isolated":false,"effective_user":"nobody"}`,
	}
	for _, output := range tests {
		if err := validateVerifiedExecSmokeOutput([]byte(output+"\n"), policy); !errors.Is(err, errVerifiedExecOutput) {
			t.Fatalf("validateVerifiedExecSmokeOutput(%q) error = %v", output, err)
		}
	}
}

func TestRunVerifiedExecSmoke_OutputLimitAndTimeout_FailClosed(t *testing.T) {
	t.Parallel()
	policy := ompCanaryPolicy{version: pinnedOMPVersion, sha256: "sha256:" + strings.Repeat("a", 64)}
	tests := []struct {
		name    string
		mode    string
		timeout time.Duration
		want    error
	}{
		{name: "output limit", mode: "verified-exec-output-limit", timeout: 2 * time.Second, want: errOutputLimit},
		{name: "timeout", mode: "verified-exec-timeout", timeout: 100 * time.Millisecond, want: errExecutionTimeout},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runVerifiedExecSmoke(verifiedExecSmokeConfig{
				artifact: testArtifact(t), ompExecutable: "/absolute/omp", policy: policy,
				timeout: test.timeout, pipeWait: 100 * time.Millisecond,
				extraEnvironment: []string{"AUTOPUS_EXEC_SMOKE_FIXTURE=" + test.mode},
			})
			if !errors.Is(err, test.want) {
				t.Fatalf("runVerifiedExecSmoke() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestValidatePinnedOMPExecutable_RejectsMissingSymlinkAndWrongDigest(t *testing.T) {
	t.Parallel()
	executable := writePinnedOMPExecutableFixture(t)
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		t.Fatalf("filepath.EvalSymlinks() error = %v", err)
	}
	symlink := filepath.Join(t.TempDir(), "omp-link")
	if err := os.Symlink(resolved, symlink); err != nil {
		t.Fatalf("os.Symlink() error = %v", err)
	}
	policy := ompCanaryPolicy{version: pinnedOMPVersion, sha256: "sha256:" + strings.Repeat("0", 64)}
	for _, path := range []string{"", filepath.Join(t.TempDir(), "missing"), symlink, resolved} {
		if _, err := validatePinnedOMPExecutable(path, policy); err == nil {
			t.Fatalf("validatePinnedOMPExecutable(%q) error = nil", path)
		}
	}
}

func writePinnedOMPExecutableFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "auto")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return path
}

func runVerifiedExecFixture(mode string) int {
	if len(os.Args) != 10 || os.Args[1] != "workflow" || os.Args[2] != "context-runtime" ||
		os.Args[3] != "verified-exec-smoke" || os.Args[4] != "--omp-executable" ||
		os.Args[6] != "--canary-root" || os.Args[8] != "--format" || os.Args[9] != "json" {
		return 97
	}
	switch mode {
	case "verified-exec-output":
		_, _ = os.Stdout.WriteString(os.Getenv("AUTOPUS_VERIFIED_EXEC_OUTPUT") + "\n")
	case "verified-exec-output-limit":
		_, _ = os.Stdout.WriteString(strings.Repeat("x", maximumOutput+1))
	case "verified-exec-timeout":
		time.Sleep(10 * time.Second)
	default:
		return 97
	}
	return 0
}
