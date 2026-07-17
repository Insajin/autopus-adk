package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/worker/controlplane"
	"github.com/insajin/autopus-adk/pkg/worker/security"
)

func TestRunWorkerValidate_VerifiesPolicySignature(t *testing.T) {
	t.Setenv(controlplane.PolicySigningSecretEnv, "secret")

	dir := t.TempDir()
	policyPath := filepath.Join(dir, "autopus-policy-task-1.json")
	policy := security.SecurityPolicy{
		AllowFS:         true,
		AllowedCommands: []string{"go test"},
		TimeoutSec:      30,
	}

	data, err := json.Marshal(policy)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(policyPath, data, 0o600))

	signature, err := controlplane.SignSecurityPolicy("task-1", policy, "secret")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(policyPath+".sig", []byte(signature+"\n"), 0o600))

	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)

	err = runWorkerValidate(cmd, policyPath, "go test ./...", "")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "PASS")
}

func TestRunWorkerValidate_VerifiesFixedPolicySignatureVector(t *testing.T) {
	t.Setenv(controlplane.PolicySigningSecretEnv, "secret")

	dir := t.TempDir()
	policyPath := filepath.Join(dir, "autopus-policy-task-1.json")
	policyJSON := []byte(`{"allow_network":false,"allow_fs":true,"allowed_commands":["go test"],"timeout_sec":30}`)
	const signature = "6f287247fd97a7751f1bc68c3f7b51e5e7d995b1ca3b623dd7eaf779afb8d750"
	require.NoError(t, os.WriteFile(policyPath, policyJSON, 0o600))
	require.NoError(t, os.WriteFile(policyPath+".sig", []byte(signature+"\n"), 0o600))

	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)

	err := runWorkerValidate(cmd, policyPath, "go test ./...", "")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "PASS")
}

// runWorkerValidateSubprocess re-executes this test binary as a helper
// process so runWorkerValidate's os.Exit(1) DENY paths can be observed
// without killing the real test process. Mirrors the self-reexec pattern in
// companion_public_key_receipt_crash_test.go.
func runWorkerValidateSubprocess(t *testing.T, extraEnv ...string) (output string, exitCode int) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=TestRunWorkerValidateHelperProcess$")
	cmd.Env = append(os.Environ(), "GO_WANT_WORKER_VALIDATE_HELPER=1")
	cmd.Env = append(cmd.Env, extraEnv...)

	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exitErr *exec.ExitError
	require.True(t, errors.As(err, &exitErr), "unexpected non-exit error: %v (output=%s)", err, out)
	return string(out), exitErr.ExitCode()
}

// TestRunWorkerValidateHelperProcess is the subprocess entry point driven by
// runWorkerValidateSubprocess. It is a no-op under the normal test runner.
func TestRunWorkerValidateHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_WORKER_VALIDATE_HELPER") != "1" {
		return
	}
	cmd := &cobra.Command{}
	cmd.SetOut(os.Stdout)
	_ = runWorkerValidate(
		cmd,
		os.Getenv("WORKER_VALIDATE_HELPER_POLICY"),
		os.Getenv("WORKER_VALIDATE_HELPER_COMMAND"),
		"",
	)
	// runWorkerValidate always calls os.Exit on the DENY paths; reaching
	// here means it returned normally (PASS path), so exit 0 explicitly.
	os.Exit(0)
}

// TestRunWorkerValidate_UnsignedDisabledDeniesWithGuidance is the S9 oracle
// for REQ-011: with neither the signing secret nor the unsigned opt-out set,
// `auto worker validate` exits non-zero and reports the signing-secret env
// var plus setup guidance, without leaking any secret value.
func TestRunWorkerValidate_UnsignedDisabledDeniesWithGuidance(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "autopus-policy-task-1.json")
	policy := security.SecurityPolicy{
		AllowFS:         true,
		AllowedCommands: []string{"go test"},
		TimeoutSec:      30,
	}
	data, err := json.Marshal(policy)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(policyPath, data, 0o600))
	// No .sig file: proves the DENY below comes from the disabled-signing
	// gate, not from a missing-signature failure inside VerifyCachedPolicyFile.

	output, exitCode := runWorkerValidateSubprocess(t,
		"WORKER_VALIDATE_HELPER_POLICY="+policyPath,
		"WORKER_VALIDATE_HELPER_COMMAND=go test ./...",
		controlplane.PolicySigningSecretEnv+"=",
		controlplane.AllowUnsignedControlPlaneEnv+"=",
	)

	assert.NotEqual(t, 0, exitCode, "output=%s", output)
	assert.Contains(t, output, "DENY")
	assert.Contains(t, output, controlplane.PolicySigningSecretEnv)
	assert.Contains(t, output, controlplane.AllowUnsignedControlPlaneEnv)
}

// TestRunWorkerValidate_OptOutAllowsLegacyPassDenyBehavior is the opt-out
// half of the S9 oracle: with AllowUnsignedControlPlaneEnv set, `auto worker
// validate` skips signature verification (no .sig file exists) and falls
// through to the policy's own PASS/DENY verdict, matching pre-SPEC behavior.
func TestRunWorkerValidate_OptOutAllowsLegacyPassDenyBehavior(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "autopus-policy-task-1.json")
	policy := security.SecurityPolicy{
		AllowFS:         true,
		AllowedCommands: []string{"go test"},
		TimeoutSec:      30,
	}
	data, err := json.Marshal(policy)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(policyPath, data, 0o600))
	// No .sig file: the opt-out must reach the command verdict anyway.

	output, exitCode := runWorkerValidateSubprocess(t,
		"WORKER_VALIDATE_HELPER_POLICY="+policyPath,
		"WORKER_VALIDATE_HELPER_COMMAND=go test ./...",
		controlplane.PolicySigningSecretEnv+"=",
		controlplane.AllowUnsignedControlPlaneEnv+"=1",
	)

	assert.Equal(t, 0, exitCode, "output=%s", output)
	assert.Contains(t, output, "PASS")
}
