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
	artifact := writeVerifiedExecFixture(t, `printf '%s\n' "$AUTOPUS_VERIFIED_EXEC_OUTPUT"`)
	err := runVerifiedExecSmoke(verifiedExecSmokeConfig{
		artifact: artifact, ompExecutable: "/absolute/omp", policy: policy,
		timeout: 2 * time.Second, pipeWait: 100 * time.Millisecond,
		extraEnvironment: []string{`AUTOPUS_VERIFIED_EXEC_OUTPUT={"schema_version":"workflow-context-verified-exec-smoke/v1","omp_version":"omp/17.2.7","omp_executable_sha256":"sha256:` + strings.Repeat("a", 64) + `","rpc_ready":true,"provider_calls":0,"uid_isolated":true,"effective_user":"nobody"}`},
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
		body    string
		timeout time.Duration
		want    error
	}{
		{name: "output limit", body: `yes x`, timeout: 2 * time.Second, want: errOutputLimit},
		{name: "timeout", body: `sleep 10`, timeout: 100 * time.Millisecond, want: errExecutionTimeout},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifact := writeVerifiedExecFixture(t, test.body)
			err := runVerifiedExecSmoke(verifiedExecSmokeConfig{
				artifact: artifact, ompExecutable: "/absolute/omp", policy: policy,
				timeout: test.timeout, pipeWait: 100 * time.Millisecond,
			})
			if !errors.Is(err, test.want) {
				t.Fatalf("runVerifiedExecSmoke() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestValidatePinnedOMPExecutable_RejectsMissingSymlinkAndWrongDigest(t *testing.T) {
	t.Parallel()
	executable := writeVerifiedExecFixture(t, `exit 0`)
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

func writeVerifiedExecFixture(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "auto")
	contents := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return path
}
