package cli

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestWorkflowContextObserveVersion_InheritedModeUsesDirectVerifiedImage(t *testing.T) {
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
	assert.True(t, command.directDarwinImage)
	assert.NotEqual(t, "/usr/bin/sandbox-exec", command.cmd.Path)
	require.NoError(t, command.Close())

	got, err := probeWorkflowContextObserveVersion(ctx, options, pipelineOMPActiveSandboxInheritedParent)
	require.NoError(t, err)
	assert.Equal(t, "omp/17.2.7", got)
}
