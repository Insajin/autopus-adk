package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/pipeline"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func TestCompiledPipelineOMPActiveStaticPolicy_ExactCanonicalBase64URLPasses(t *testing.T) {
	_, policy, _ := pipelineOMPActiveCoordinatorFixture(t, time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC))
	body, err := json.Marshal(policy)
	require.NoError(t, err)
	for _, forbidden := range []string{`"artifact_sha256"`, `"auto_binary_sha256"`, `"report_sha256"`} {
		assert.NotContains(t, string(body), forbidden)
	}

	original := pipelineOMPActiveStaticPolicyB64
	pipelineOMPActiveStaticPolicyB64 = base64.RawURLEncoding.EncodeToString(body)
	t.Cleanup(func() { pipelineOMPActiveStaticPolicyB64 = original })
	got, err := compiledPipelineOMPActiveStaticPolicy(pipelineOMPManagedActiveCandidate{})
	require.NoError(t, err)
	assert.Equal(t, policy, got)
}

func TestCompiledPipelineOMPActiveStaticPolicy_WireDriftFailsClosed(t *testing.T) {
	_, policy, _ := pipelineOMPActiveCoordinatorFixture(t, time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC))
	body, err := json.Marshal(policy)
	require.NoError(t, err)
	unknown := bytes.Replace(body, []byte("}"), []byte(`,"unknown":true}`), 1)
	pretty, err := json.MarshalIndent(policy, "", "  ")
	require.NoError(t, err)

	original := pipelineOMPActiveStaticPolicyB64
	t.Cleanup(func() { pipelineOMPActiveStaticPolicyB64 = original })
	for name, encoded := range map[string]string{
		"unknown field": base64.RawURLEncoding.EncodeToString(unknown),
		"non canonical": base64.RawURLEncoding.EncodeToString(pretty),
		"padding":       base64.RawURLEncoding.EncodeToString(body) + "=",
		"whitespace":    " " + base64.RawURLEncoding.EncodeToString(body),
		"oversize":      base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("x", pipelineOMPActiveStaticPolicyMaxBytes+1))),
	} {
		t.Run(name, func(t *testing.T) {
			pipelineOMPActiveStaticPolicyB64 = encoded
			if _, err := compiledPipelineOMPActiveStaticPolicy(pipelineOMPManagedActiveCandidate{}); err == nil {
				t.Fatal("compiled policy wire drift was accepted")
			}
		})
	}
}

func TestPipelineOMPActivePinnedRuntime_ReplacementHookKeepsProbeAndSessionOnVerifiedCopy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture executable replacement uses a POSIX script")
	}
	requireDarwinManagedOMPSandboxForTest(t)
	config, _ := pipelineOMPBackendTestConfig(t)
	marker := filepath.Join(t.TempDir(), "unverified-executable-ran")
	replacement := "#!/bin/sh\nprintf unsafe > " + shellQuotePipelineOMP(marker) + "\nprintf 'omp/17.2.7\\n'\n"
	copyPipelineOMPActiveNativeFixture(t, config.Executable)
	config.Environment = append(config.Environment,
		pipelineOMPActiveEndpointKey+"=http://127.0.0.1:43123",
		pipelineOMPActiveCredentialKey+"=fixture-token-value",
	)
	config.PhaseModels = map[pipeline.PhaseID]string{pipeline.PhasePlan: "provider-a/model-a"}
	config, err := normalizePipelineOMPBackendConfig(config)
	require.NoError(t, err)
	originalExecutable := config.Executable
	pinned, pinRoot, err := materializePipelineOMPActiveRuntimeConfig(config)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(pinRoot) })

	info, err := os.Lstat(pinned.Executable)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	assert.NotEqual(t, originalExecutable, pinned.Executable)
	assert.Equal(t, config.executableID.digest, pinned.executableID.digest)

	hookCalls := 0
	provider := newPipelineOMPActiveCurrentRuntimeProviderWithHooks(pinned, pipelineOMPActiveCurrentRuntimeHooks{
		beforeVersionProbe: func() {
			hookCalls++
			next := originalExecutable + ".replacement"
			require.NoError(t, os.WriteFile(next, []byte(replacement), 0o700))
			require.NoError(t, os.Rename(next, originalExecutable))
		},
	})
	candidate := pipelineOMPActivePinnedRuntimeCandidate(t, pinned)
	staticPolicy := pipelineOMPActivePinnedRuntimePolicy()
	current, err := provider(context.Background(), candidate, staticPolicy)
	require.NoError(t, err)
	assert.Equal(t, 1, hookCalls)
	assert.Equal(t, "omp/17.2.7", current.OMPVersion)
	assert.Equal(t, fmt.Sprintf("sha256:%x", pinned.executableID.digest[:]), current.OMPExecutableSHA256)
	assert.True(t, validPipelineOMPActiveHash(current.ExecutableSHA256))
	expectedAuthority, err := pipelineOMPActiveProviderAuthorityDigest(
		staticPolicy.PolicyDigest, staticPolicy.PipelineImplementationDigest, candidate.ModelScopeDigest,
		pinned.ModelContextWindow, "http://127.0.0.1:43123", "fixture-token-value",
	)
	require.NoError(t, err)
	assert.Equal(t, expectedAuthority, current.ProviderAuthorityDigest)

	session, err := newPipelineOMPActiveSessionStart(pinned)(
		context.Background(), candidate, pipelineOMPActivePinnedRuntimePrepared(candidate),
	)
	require.NoError(t, err)
	require.NoError(t, session.Close())
	_, statErr := os.Lstat(marker)
	assert.ErrorIs(t, statErr, os.ErrNotExist, "the swapped source must never receive broker authority")
}

func TestPipelineOMPActivePinnedRuntime_PinnedPathSwapFailsClosedWithoutProbeAuthority(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Darwin sandbox profile is required for the authority-free probe assertion")
	}
	config, _ := pipelineOMPBackendTestConfig(t)
	marker := filepath.Join(t.TempDir(), "pinned-swap-probe-authority")
	replacement := "#!/bin/sh\nprintf \"$" + pipelineOMPActiveCredentialKey + "\" > " +
		shellQuotePipelineOMP(marker) + "\nprintf 'omp/17.2.7\\n'\n"
	copyPipelineOMPActiveNativeFixture(t, config.Executable)
	config.Environment = append(config.Environment,
		pipelineOMPActiveEndpointKey+"=http://127.0.0.1:43123",
		pipelineOMPActiveCredentialKey+"=fixture-token-must-not-reach-probe",
	)
	config.PhaseModels = map[pipeline.PhaseID]string{pipeline.PhasePlan: "provider-a/model-a"}
	config, err := normalizePipelineOMPBackendConfig(config)
	require.NoError(t, err)
	pinned, pinRoot, err := materializePipelineOMPActiveRuntimeConfig(config)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(pinRoot) })

	hookCalls := 0
	provider := newPipelineOMPActiveCurrentRuntimeProviderWithHooks(pinned, pipelineOMPActiveCurrentRuntimeHooks{
		afterVerifiedVersionFD: func() {
			hookCalls++
			next := pinned.Executable + ".replacement"
			require.NoError(t, os.WriteFile(next, []byte(replacement), 0o700))
			require.NoError(t, os.Rename(next, pinned.Executable))
		},
	})
	_, err = provider(context.Background(), pipelineOMPActivePinnedRuntimeCandidate(t, pinned), pipelineOMPActivePinnedRuntimePolicy())
	require.ErrorContains(t, err, "identity probe failed")
	assert.Equal(t, 1, hookCalls)
	_, statErr := os.Lstat(marker)
	assert.ErrorIs(t, statErr, os.ErrNotExist, "an unverified probe must have neither credentials nor file-write authority")
}

func TestPipelineOMPActivePinnedRuntime_MaterializeFailureCleansPrivateRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture executable replacement uses POSIX permissions")
	}
	config, _ := pipelineOMPBackendTestConfig(t)
	config, err := normalizePipelineOMPBackendConfig(config)
	require.NoError(t, err)
	replacement := config.Executable + ".replacement"
	require.NoError(t, os.WriteFile(replacement, []byte("#!/bin/sh\nexit 1\n"), 0o700))
	require.NoError(t, os.Rename(replacement, config.Executable))

	_, _, err = materializePipelineOMPActiveRuntimeConfig(config)
	require.ErrorContains(t, err, "identity changed")
	assertPipelineOMPRuntimeEmpty(t, config.RuntimeBase)
}

func TestPipelineOMPBackend_ActivePinnedRuntimeIsRemovedOnClose(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture executable uses a POSIX launcher")
	}
	config, _ := pipelineOMPBackendTestConfig(t)
	config.ManagedActive = newPipelineOMPManagedActiveCoordinator()
	backend, err := newPipelineOMPBackend(config)
	require.NoError(t, err)
	assert.NotEqual(t, config.Executable, backend.config.Executable)
	assert.FileExists(t, backend.config.Executable)

	require.NoError(t, backend.Close())
	assertPipelineOMPRuntimeEmpty(t, config.RuntimeBase)
}

func TestPipelineOMPVerifiedExecCommand_ParentFDClosesAfterStartAndFailure(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("verified FD execution is only implemented on Darwin and Linux")
	}
	executable, identity, err := canonicalPipelineOMPExecutable("/usr/bin/true")
	require.NoError(t, err)

	success, err := newPipelineOMPVerifiedExecCommandContext(
		context.Background(), executable, identity, "--version",
	)
	require.NoError(t, err)
	require.NoError(t, configurePipelineOMPActiveVersionProbeSandbox(success.cmd, executable))
	require.NoError(t, configureWorkflowContextManagedRPCProcessGroup(success.cmd))
	require.NoError(t, success.Start())
	assert.Nil(t, success.parentFD)
	require.NoError(t, success.cmd.Wait())

	failure, err := newPipelineOMPVerifiedExecCommandContext(
		context.Background(), executable, identity, "--version",
	)
	require.NoError(t, err)
	require.NoError(t, configurePipelineOMPActiveVersionProbeSandbox(failure.cmd, executable))
	failure.cmd.Path = filepath.Join(t.TempDir(), "missing-executable")
	require.Error(t, failure.Start())
	assert.Nil(t, failure.parentFD)
}

func TestPipelineOMPVerifiedExecCommand_DarwinGateCancelKillsAndReapsBeforeTargetExec(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Darwin ptrace lifecycle is platform-specific")
	}
	executable, identity, err := canonicalPipelineOMPExecutable("/usr/bin/true")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	command, err := newPipelineOMPVerifiedExecCommandContext(ctx, executable, identity, "--version")
	require.NoError(t, err)
	require.NoError(t, configurePipelineOMPActiveVersionProbeSandbox(command.cmd, executable))
	require.NoError(t, configureWorkflowContextManagedRPCProcessGroup(command.cmd))
	require.True(t, configurePipelineOMPVerifiedExecDarwinStopForTest(command, cancel))

	err = command.Start()
	require.ErrorContains(t, err, "identity gate canceled")
	assert.Nil(t, command.parentFD)
	require.NotNil(t, command.cmd.ProcessState)
	waitStatus, ok := command.cmd.ProcessState.Sys().(syscall.WaitStatus)
	require.True(t, ok)
	assert.True(t, waitStatus.Signaled() || waitStatus.Exited())
}

func pipelineOMPActivePinnedRuntimeCandidate(
	t *testing.T,
	config pipelineOMPBackendConfig,
) pipelineOMPManagedActiveCandidate {
	t.Helper()
	snapshot := pipeline.OMPExecutionSnapshot{
		ProjectDir: config.ProjectDir, SpecID: config.SpecID, SpecDir: config.SpecDir,
		SnapshotHash: config.SnapshotHash, GitCommitHash: config.GitCommitHash,
		PhaseID: pipeline.PhasePlan, Attempt: 1, Prompt: "canonical", ActivePrompt: "active",
	}
	candidate, err := newPipelineOMPManagedActiveCandidate(
		snapshot, config.PhaseModels[pipeline.PhasePlan], config.PhaseModels,
	)
	require.NoError(t, err)
	return candidate
}

func pipelineOMPActivePinnedRuntimePrepared(
	candidate pipelineOMPManagedActiveCandidate,
) pipelineOMPManagedActivePrepared {
	return pipelineOMPManagedActivePrepared{Binding: pipelineOMPActiveLeaseBinding{
		GrantDigest:  pipelineOMPContextCohortHash("grant"),
		PolicyDigest: pipelineOMPActivePinnedRuntimePolicy().PolicyDigest,
		WorkspaceID:  "autopus-adk", SpecID: candidate.Snapshot.SpecID,
		GitCommitHash:    candidate.Snapshot.GitCommitHash,
		ModelScopeDigest: candidate.ModelScopeDigest,
	}}
}

func copyPipelineOMPActiveNativeFixture(t *testing.T, destination string) {
	t.Helper()
	input, err := os.Open(os.Args[0])
	require.NoError(t, err)
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_TRUNC, 0o700)
	require.NoError(t, err)
	_, copyErr := io.Copy(output, input)
	syncErr := output.Sync()
	closeErr := output.Close()
	require.NoError(t, errors.Join(copyErr, syncErr, closeErr))
	require.NoError(t, os.Chmod(destination, 0o700))
}

func pipelineOMPActivePinnedRuntimePolicy() promptlayer.OMPContextPromotionStaticPolicyV3 {
	return promptlayer.OMPContextPromotionStaticPolicyV3{
		PolicyDigest:                 workflowContextRuntimeHash("pinned-runtime-policy"),
		PipelineImplementationDigest: pipelineOMPActiveImplementationDigest(),
	}
}
