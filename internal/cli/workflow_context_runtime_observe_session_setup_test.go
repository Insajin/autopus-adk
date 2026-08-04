package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const workflowContextInheritedVersionOuterSandboxFixture = "AUTOPUS_TEST_OMP_VERSION_OUTER_SANDBOX"

func TestWorkflowContextObserveSessionPhaseModels_BindsAllCanonicalPhases(t *testing.T) {
	t.Parallel()
	const selector = "openai/gpt-5.6-sol"
	want := map[pipeline.PhaseID]string{
		pipeline.PhasePlan:         selector,
		pipeline.PhaseTestScaffold: selector,
		pipeline.PhaseImplement:    selector,
		pipeline.PhaseValidate:     selector,
		pipeline.PhaseReview:       selector,
	}

	phaseModels := workflowContextObserveSessionPhaseModels(selector)
	assert.Equal(t, want, phaseModels)
	provider, digest, err := pipelineOMPActiveModelScope(phaseModels)
	require.NoError(t, err)
	assert.Equal(t, "openai", provider)
	assert.Equal(t, "sha256:386816349ebf9b5c2a113889fb3160662121a4fd4fa4085d2bdef97660142adf", digest)
}

func TestWorkflowContextObserveSessionCommand_InheritedParentSandboxIsExplicitOptIn(t *testing.T) {
	t.Parallel()
	assert.Equal(t, pipelineOMPActiveSandboxManaged, workflowContextObserveSessionSandboxMode(false))
	assert.Equal(t, pipelineOMPActiveSandboxInheritedParent, workflowContextObserveSessionSandboxMode(true))
	flag := newWorkflowContextObserveSessionCmd().Flags().Lookup("inherit-parent-sandbox")
	require.NotNil(t, flag)
	assert.Equal(t, "false", flag.DefValue)
	assert.Nil(t, newWorkflowContextObserveCallCmd().Flags().Lookup("inherit-parent-sandbox"))

	options := workflowContextObserveSessionOptions{
		Provider: "openai", Model: "gpt-5.6-sol", SpecID: "SPEC-OMP-004",
		CredentialLocator: "AUTOPUS_OMP_CONTEXT_PROVIDER_OPENAI", TargetGitCommit: strings.Repeat("a", 40),
		Endpoint: "http://127.0.0.1:43123", Executable: "omp",
	}
	require.NoError(t, validateWorkflowContextObserveSessionOptions(options))
}

func TestWorkflowContextObserveVersion_InheritedModeUsesVerifiedPathWithoutInnerWrapper(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("inherited parent sandbox is Darwin-only")
	}
	options := WorkflowContextManagedRPCOptions{
		Executable: os.Args[0], Workspace: t.TempDir(), Environment: os.Environ(),
		AllowedEndpoint: "http://127.0.0.1:43123",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command, _, err := newWorkflowContextObserveInheritedVersionCommand(ctx, options)
	require.NoError(t, err)
	executable, _, err := canonicalPipelineOMPExecutable(os.Args[0])
	require.NoError(t, err)
	assert.True(t, command.inheritedDarwinPath)
	assert.False(t, command.inheritedDarwinPrivate)
	assert.Equal(t, executable, command.cmd.Path)
	assert.Equal(t, executable, command.cmd.Args[0])
	assert.Empty(t, command.cmd.ExtraFiles)
	assert.False(t, pipelineOMPVerifiedExecUsesDarwinPtrace(command))
	require.NoError(t, command.Close())

	got, err := probeWorkflowContextObserveVersion(ctx, options, pipelineOMPActiveSandboxInheritedParent)
	require.NoError(t, err)
	assert.Equal(t, "omp/17.2.7", got)
}

func TestWorkflowContextObserveVersion_InheritedModeStripsSecretsInsideDenyDefaultSandbox(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("inherited parent sandbox is Darwin-only")
	}
	if os.Getenv(workflowContextInheritedVersionOuterSandboxFixture) == "" {
		const profile = `(version 1)
(deny default)
(allow file-read*)
(allow file-write* (literal "/dev/null"))
(allow process*)
(allow mach*)
(allow sysctl-read)
`
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "/usr/bin/sandbox-exec", "-p", profile, os.Args[0],
			"-test.run=^TestWorkflowContextObserveVersion_InheritedModeStripsSecretsInsideDenyDefaultSandbox$")
		cmd.Env = append(os.Environ(), workflowContextInheritedVersionOuterSandboxFixture+"=1")
		require.NoError(t, cmd.Run())
		return
	}

	options := WorkflowContextManagedRPCOptions{
		Executable: os.Args[0], Workspace: filepath.Dir(os.Args[0]),
		Environment: []string{
			"PATH=/sentinel/bin", pipelineOMPActiveCredentialKey + "=pipeline-sentinel",
			"OPENAI_API_KEY=provider-sentinel", "UNRELATED_SECRET=unrelated-sentinel",
		},
		AllowedEndpoint: "http://127.0.0.1:43123",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command, _, err := newWorkflowContextObserveInheritedVersionCommand(ctx, options)
	require.NoError(t, err)
	if command.cmd.Env == nil || len(command.cmd.Env) != 0 {
		t.Fatalf("inherited version environment is not empty: entries=%d", len(command.cmd.Env))
	}
	require.NoError(t, command.Close())

	got, err := probeWorkflowContextObserveVersion(ctx, options, pipelineOMPActiveSandboxInheritedParent)
	require.NoError(t, err)
	assert.Equal(t, "omp/17.2.7", got)
}
