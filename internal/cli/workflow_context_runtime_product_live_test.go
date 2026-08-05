package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ompadapter "github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextProductSession_InstalledOMPWithRealOverlay_AdmitsExactProductPrompts(t *testing.T) {
	if os.Getenv("AUTOPUS_OMP_CONTEXT_LIVE") != "1" {
		t.Skip("set AUTOPUS_OMP_CONTEXT_LIVE=1 to run the installed OMP product-session proof")
	}
	executable, err := exec.LookPath("omp")
	require.NoError(t, err, "installed OMP executable is required")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	request, input := newWorkflowContextProductLiveAuthority(t)
	projectConfigPath := filepath.Join(input.ProjectDir, ".omp", "config.yml")
	projectConfigBefore, err := os.ReadFile(projectConfigPath)
	require.NoError(t, err)
	ambientMCPSentinel := filepath.Join(t.TempDir(), "ambient-mcp-started")
	ambientMCP, err := json.Marshal(map[string]any{"mcpServers": map[string]any{
		"ambient": map[string]any{"command": "/usr/bin/touch", "args": []string{ambientMCPSentinel}},
	}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(input.ProjectDir, ".mcp.json"), ambientMCP, 0o600))
	provider := newWorkflowContextProductLiveProvider(t)
	probeLayout, err := newWorkflowContextLiveLayout(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, probeLayout.writeConfig(provider.URL()))
	version, err := probeWorkflowContextLiveVersion(ctx, executable, probeLayout, provider.URL())
	require.NoError(t, err)
	require.NoError(t, verifyWorkflowContextLiveOverlay(ctx, executable, probeLayout, provider.URL()))

	// Metadata probes may create OMP-owned state, so the admitted process receives a fresh task root.
	layout, err := newWorkflowContextLiveLayout(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, writeWorkflowContextProductLiveConfig(layout, provider.URL()))
	installWorkflowContextManagedLiveBridge(t, layout.workspace)
	projectConfig, err := config.LoadPreview(input.ProjectDir)
	require.NoError(t, err)
	_, err = ompadapter.NewWithRoot(layout.workspace).Generate(ctx, projectConfig)
	require.NoError(t, err)
	overlay, configPath, err := newWorkflowContextProductOverlay(layout.runtime, request.Policy.MemoryMode)
	require.NoError(t, err)
	require.NoError(t, os.Remove(layout.overlay))
	layout.overlay = configPath

	request.Capabilities.Version = version
	request.Capabilities.ProbeSource = "installed-omp-loopback-product-rpc-real-overlay"
	request.Capabilities.CheckedAt = time.Now().UTC()
	attachValidWorkflowContextPromotion(t, &request)

	var driver *WorkflowContextManagedRPCDriver
	runtime := WorkflowContextProductRuntimeInputs{
		Capabilities: request.Capabilities,
		Promotion:    request.Promotion,
		History:      request.Binding.History,
		Overlay:      overlay,
		Supervisor:   NewWorkflowContextRuntimeSupervisor(nil),
		DriverOptions: WorkflowContextManagedRPCOptions{
			Executable: executable, Workspace: layout.workspace, RuntimeBase: layout.base,
			RuntimeRoot: layout.runtime, SessionDir: layout.sessions, ConfigPath: configPath,
			Model: "contextfake/" + workflowContextLiveModel, AllowedEndpoint: provider.URL(),
			Environment:        append(layout.env(), workflowContextProductLiveCredentialKey+"="+workflowContextProductLiveCredential),
			HistoryAfterTokens: map[string]int{"old-read": 2}, MaxTime: 55 * time.Second, CompactionCycles: 2,
		},
		NewManagedDriver: func(options WorkflowContextManagedRPCOptions) (WorkflowContextManagedProcessDriver, error) {
			managed, createErr := NewWorkflowContextManagedRPCDriver(options)
			driver = managed
			return managed, createErr
		},
	}

	receipt, err := RunWorkflowContextProductSession(ctx, input, runtime)
	if err != nil {
		requests, authHeaders, unexpectedEndpoints, failure := provider.receipt()
		observation := WorkflowContextManagedRPCObservation{}
		if driver != nil {
			observation = driver.Observation()
		}
		t.Fatalf("installed product session failed closed: reason=%s provider_requests=%d auth=%d endpoints=%d provider_failure=%s pre_ack=%d post_ack=%d native_start=%d native_end=%d",
			liveReason(err), requests, authHeaders, unexpectedEndpoints, failure,
			observation.PreACKs, observation.PostACKs, observation.NativeStarts, observation.NativeEnds)
	}
	require.NotNil(t, driver)
	assert.Equal(t, WorkflowContextOutcomeAdmitted, receipt.Outcome)
	assert.True(t, receipt.ExactMatch)
	assert.Equal(t, []string{
		"checkpointed", "compacted", "rehydrated",
		"checkpointed", "compacted", "rehydrated", "admitted",
	}, receipt.PhaseSequence)

	observation := driver.Observation()
	assert.Equal(t, 5, observation.ProviderTurns)
	assert.Equal(t, 2, observation.PreACKs)
	assert.Equal(t, 2, observation.PostACKs)
	assert.Equal(t, 2, observation.NativeStarts)
	assert.Equal(t, 2, observation.NativeEnds)
	assert.Equal(t, 2, observation.CanonicalReadmissions)
	assert.Equal(t, 2, observation.EphemeralReadmissions)
	assert.True(t, validPipelineOMPActiveHash(observation.ProviderAuthorityDigest))
	assert.True(t, observation.SameProcess)
	assert.True(t, observation.SameSession)
	assert.True(t, observation.Sandboxed)
	assert.True(t, observation.ProviderObserved)
	assert.False(t, observation.ProcessActiveAfterCleanup)
	assert.Equal(t, 2, receipt.Lifecycle.RequiredCompactionCycles)
	assert.Equal(t, observation.PreACKs, receipt.Lifecycle.PreCompactionACKs)
	assert.Equal(t, observation.PostACKs, receipt.Lifecycle.PostCompactionACKs)
	assert.Equal(t, observation.CanonicalReadmissions, receipt.Lifecycle.CanonicalReadmissions)
	assert.Equal(t, observation.EphemeralReadmissions, receipt.Lifecycle.EphemeralReadmissions)
	assert.Equal(t, observation.ProviderAuthorityDigest, receipt.Lifecycle.ProviderAuthorityDigest)
	assert.True(t, receipt.Lifecycle.SameProcess)
	assert.True(t, receipt.Lifecycle.SameSession)

	requests, authHeaders, unexpectedEndpoints, failure := provider.receipt()
	require.Empty(t, failure)
	assert.Equal(t, 5, requests)
	assert.Equal(t, 5, authHeaders)
	assert.Zero(t, unexpectedEndpoints)
	assert.Contains(t, provider.userMessage(1), "## Router Contract")
	assert.Contains(t, provider.userMessage(1), "go SPEC-OMP-004 --auto")
	assert.Equal(t, input.DecisionDelta, provider.userMessage(2))
	assertWorkflowContextManagedAdmissionMessage(t, provider.userMessage(3), request)
	assertWorkflowContextManagedAdmissionMessage(t, provider.userMessage(4), request)
	assertWorkflowContextManagedAdmissionMessage(t, provider.userMessage(5), request)
	assert.True(t, receipt.Cleanup.Verified)
	assert.Zero(t, receipt.Cleanup.UserRootAccessCount)
	assert.Zero(t, receipt.ArtifactCounts.AfterCleanup)
	assert.Zero(t, workflowContextLiveRootCount(layout.runtime))
	assert.NoFileExists(t, ambientMCPSentinel, "project MCP discovery must remain disabled")
	assertProductOMPConfigsUnchanged(t, input.ProjectDir)
	projectConfigAfter, err := os.ReadFile(projectConfigPath)
	require.NoError(t, err)
	assert.Equal(t, projectConfigBefore, projectConfigAfter)

	serialized, err := json.Marshal(receipt)
	require.NoError(t, err)
	for _, private := range []string{
		input.OriginalTask, input.DecisionDelta, input.ProjectDir, layout.base,
		request.Binding.Delivery.Prompt, request.Binding.Delivery.Layers[0].Content,
		workflowContextProductLiveCredential,
	} {
		assert.NotContains(t, string(serialized), private)
	}
	t.Logf("installed_product_context version=%s real_overlay=true provider_requests=%d auth=5 external_endpoints=0 pre_ack=2 post_ack=2 native_start=2 native_end=2 same_pid=%t same_session=%t cleanup_root_count=0 sandbox=%t",
		version, requests, observation.SameProcess, observation.SameSession, observation.Sandboxed)
}

const (
	workflowContextProductLiveCredentialKey = "AUTOPUS_OMP_CONTEXT_PROVIDER_TOKEN"
	workflowContextProductLiveCredential    = "task-owned-product-live-token"
)

func newWorkflowContextProductLiveAuthority(
	t *testing.T,
) (WorkflowContextRuntimeRequest, WorkflowContextProductSessionInput) {
	t.Helper()
	request := newWorkflowContextRuntimeFixture(t)
	cfg := config.DefaultFullConfig("autopus-adk")
	cfg.Platforms = []string{"omp"}
	require.NoError(t, config.Save(request.Binding.DeliveryOptions.Root, cfg))
	_, err := ompadapter.NewWithRoot(request.Binding.DeliveryOptions.Root).Generate(context.Background(), cfg)
	require.NoError(t, err)
	cfg.OMPContextPolicy = config.OMPContextPolicyConf{
		Profile: "active",
		Profiles: map[string]config.OMPContextProfileConf{"active": {
			HistoryMode: request.Policy.HistoryMode, MemoryMode: request.Policy.MemoryMode,
			HistoryTargetTokens: request.Policy.HistoryTargetTokens, Fallback: request.Policy.Fallback,
			CapabilityPolicy: request.Policy.CapabilityPolicy, RuntimeRootPolicy: request.Policy.RuntimeRootPolicy,
			MutationScope: request.Policy.MutationScope,
		}},
	}
	require.NoError(t, config.Save(request.Binding.DeliveryOptions.Root, cfg))
	_, err = ompadapter.NewWithRoot(request.Binding.DeliveryOptions.Root).Generate(context.Background(), cfg)
	require.NoError(t, err)
	prompts := []string{
		"/auto go SPEC-OMP-004 --auto",
		"Continue the authorized SPEC-OMP-004 implementation and preserve the current ownership boundaries.",
	}
	request.Binding.Ephemeral.OriginalTask = prompts[0]
	request.Binding.Ephemeral.DecisionDelta = prompts[1]

	input := WorkflowContextProductSessionInput{
		ProjectDir: request.Binding.DeliveryOptions.Root, Command: "go", SpecDir: runtimeSpecDir,
		SpecID: request.Binding.SpecID, TaskID: request.Binding.TaskID, Phase: request.Binding.Phase,
		SessionID: request.Binding.SessionID, OriginalTask: request.Binding.Ephemeral.OriginalTask,
		DecisionDelta:    request.Binding.Ephemeral.DecisionDelta,
		FrozenFindingIDs: request.Binding.Ephemeral.FrozenFindingIDs,
		OwnershipPaths:   request.Binding.Ephemeral.OwnershipPaths,
		ForbiddenPaths:   request.Binding.Ephemeral.ForbiddenPaths,
	}
	delivery, err := promptlayer.BuildContextDelivery(promptlayer.ContextDeliveryOptions{
		Root: input.ProjectDir, Command: input.Command, SpecDir: input.SpecDir,
	})
	require.NoError(t, err)
	request.Binding.DeliveryOptions = promptlayer.ContextDeliveryOptions{
		Root: input.ProjectDir, Command: input.Command, SpecDir: input.SpecDir,
	}
	request.Binding.Delivery = delivery
	return request, input
}

func writeWorkflowContextProductLiveConfig(layout workflowContextLiveLayout, endpoint string) error {
	if err := layout.writeConfig(endpoint); err != nil {
		return err
	}
	for path, replacement := range map[string][2]string{
		filepath.Join(layout.runtime, "models.yml"): {"contextWindow: 8192", "contextWindow: 128000"},
		layout.overlay: {"thresholdTokens: 7000", "thresholdTokens: 100000"},
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		updated := strings.Replace(string(body), replacement[0], replacement[1], 1)
		if updated == string(body) {
			return fmt.Errorf("product live config replacement was not applied: %s", path)
		}
		if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
			return err
		}
	}
	modelsPath := filepath.Join(layout.runtime, "models.yml")
	models, err := os.ReadFile(modelsPath)
	if err != nil {
		return err
	}
	credentialAuthority := "apiKey: " + workflowContextProductLiveCredentialKey + "\n    authHeader: true"
	updatedModels := strings.Replace(string(models), "auth: none", credentialAuthority, 1)
	if updatedModels == string(models) {
		return fmt.Errorf("product credential authority was not applied")
	}
	if err := os.WriteFile(modelsPath, []byte(updatedModels), 0o600); err != nil {
		return err
	}
	overlay, err := os.ReadFile(layout.overlay)
	if err != nil {
		return err
	}
	updated := strings.Replace(string(overlay), "enableAgentsProject: false", "enableAgentsProject: true", 1)
	updated += "  enableSkillCommands: true\n  customDirectories: [.agents/skills]\n"
	if updated == string(overlay) {
		return fmt.Errorf("product skill discovery config was not applied")
	}
	return os.WriteFile(layout.overlay, []byte(updated), 0o600)
}
