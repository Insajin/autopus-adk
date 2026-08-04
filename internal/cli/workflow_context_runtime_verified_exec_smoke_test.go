package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkflowContextVerifiedExecSmokeRejectsCurrentUID(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("verified executable smoke is Darwin/Linux only")
	}
	cmd := newWorkflowContextVerifiedExecSmokeCmd()
	cmd.SetArgs([]string{"--omp-executable", "/tmp/omp", "--canary-root", "/tmp/canary", "--format", "json"})
	require.ErrorContains(t, cmd.Execute(), "effective user is not nobody")
}

func TestValidateWorkflowContextReleaseCanaryIsolation(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("UID validation is Darwin/Linux only")
	}
	uid, err := workflowContextCanaryEffectiveUID()
	require.NoError(t, err)
	root := filepath.Join(t.TempDir(), "canary")
	require.NoError(t, os.Mkdir(root, 0o755))
	for _, name := range []string{"home", "tmp"} {
		require.NoError(t, os.Mkdir(filepath.Join(root, name), 0o700))
	}
	executable := filepath.Join(root, "omp-darwin-arm64")
	require.NoError(t, os.WriteFile(executable, []byte("fixture"), 0o555))
	require.NoError(t, os.Chmod(executable, 0o555))
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("TMPDIR", filepath.Join(root, "tmp"))
	require.NoError(t, validateWorkflowContextReleaseCanaryIsolation(root, executable, uid, uid))

	require.NoError(t, os.Chmod(root, 0o700))
	require.Error(t, validateWorkflowContextReleaseCanaryIsolation(root, executable, uid, uid))
	require.NoError(t, os.Chmod(root, 0o755))
	require.Error(t, validateWorkflowContextCanaryDirectory(root, uid+1, 0o755))
	require.NoError(t, os.Chmod(filepath.Join(root, "home"), 0o755))
	require.Error(t, validateWorkflowContextReleaseCanaryIsolation(root, executable, uid, uid))
	require.NoError(t, os.Chmod(filepath.Join(root, "home"), 0o700))
	require.NoError(t, os.Chmod(executable, 0o755))
	require.Error(t, validateWorkflowContextReleaseCanaryIsolation(root, executable, uid, uid))
}

func TestWorkflowContextVerifiedExecRPCSmokeObservesReadyWithoutProviderCall(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("verified executable smoke is Darwin/Linux only")
	}
	requireDarwinManagedOMPSandboxForTest(t)
	executable := workflowContextVerifiedExecSmokeFixture(t)
	_, identity, err := canonicalPipelineOMPExecutable(executable)
	require.NoError(t, err)
	calls, err := runWorkflowContextVerifiedExecRPCSmokeWithModel(
		context.Background(), executable, identity, "canary", "model-canary",
	)
	require.NoError(t, err)
	require.Zero(t, calls)
}

func TestWorkflowContextVerifiedExecRPCSmokeReportsCleanupFailure(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("verified executable smoke is Darwin/Linux only")
	}
	executable := workflowContextVerifiedExecSmokeFixture(t)
	_, identity, err := canonicalPipelineOMPExecutable(executable)
	require.NoError(t, err)
	want := errors.New("cleanup fixture failure")
	_, err = runWorkflowContextVerifiedExecRPCSmokeWithCleanup(
		context.Background(), executable, identity, "canary", "model-canary",
		func(root string) error {
			require.NoError(t, os.RemoveAll(root))
			return want
		},
	)
	require.ErrorIs(t, err, want)
}

func TestWorkflowContextVerifiedExecSmokeRejectsUnsafeInputs(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("verified executable smoke is Darwin/Linux only")
	}
	executable := workflowContextVerifiedExecSmokeFixture(t)
	symlink := filepath.Join(t.TempDir(), "omp")
	require.NoError(t, os.Symlink(executable, symlink))
	tests := [][]string{
		{"--omp-executable", "relative/omp"},
		{"--omp-executable", symlink},
		{"--omp-executable", executable, "--format", "yaml"},
		{"--omp-executable", executable, "extra"},
	}
	for _, args := range tests {
		cmd := newWorkflowContextVerifiedExecSmokeCmd()
		cmd.SetArgs(args)
		require.Error(t, cmd.Execute(), "args=%v", args)
	}
}

func workflowContextVerifiedExecSmokeFixture(t *testing.T) string {
	t.Helper()
	current, err := os.Executable()
	require.NoError(t, err)
	target := filepath.Join(t.TempDir(), "omp")
	require.NoError(t, os.Link(current, target))
	resolved, err := filepath.EvalSymlinks(target)
	require.NoError(t, err)
	return resolved
}
