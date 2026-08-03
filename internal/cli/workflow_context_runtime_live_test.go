package cli

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"reflect"
	"sync"
	"testing"
	"time"

	ompadapter "github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextRuntime_InstalledOMPCompactionLifecycleCanary_AdmitsExactBodyOnACKedLiveSession(t *testing.T) {
	if os.Getenv("AUTOPUS_OMP_CONTEXT_LIVE") != "1" {
		t.Skip("set AUTOPUS_OMP_CONTEXT_LIVE=1 to run the installed managed OMP context lifecycle canary")
	}
	executable, err := exec.LookPath("omp")
	require.NoError(t, err, "installed OMP executable is required")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	layout, err := newWorkflowContextLiveLayout(t.TempDir())
	require.NoError(t, err)
	provider := newWorkflowContextLiveProvider(t)
	require.NoError(t, layout.writeConfig(provider.URL()))
	installWorkflowContextManagedLiveBridge(t, layout.workspace)
	driver, err := NewWorkflowContextManagedRPCDriver(WorkflowContextManagedRPCOptions{
		Executable: executable, Workspace: layout.workspace, RuntimeBase: layout.base, RuntimeRoot: layout.runtime,
		SessionDir: layout.sessions, ConfigPath: layout.overlay,
		Model: "contextfake/" + workflowContextLiveModel, AllowedEndpoint: provider.URL(),
		Environment: layout.env(), HistoryAfterTokens: map[string]int{"old-read": 2}, MaxTime: 45 * time.Second,
	})
	require.NoError(t, err)
	version, err := probeWorkflowContextLiveVersion(ctx, executable, layout, provider.URL())
	require.NoError(t, err)
	require.NoError(t, verifyWorkflowContextLiveOverlay(ctx, executable, layout, provider.URL()))

	request := newWorkflowContextRuntimeFixture(t)
	request.Capabilities.Version = version
	request.Capabilities.ProbeSource = "installed-omp-loopback-managed-rpc"
	request.Capabilities.CheckedAt = time.Now().UTC()
	request.Overlay = newFakeWorkflowContextOverlay(t, activeOverlayReadback(), shadowOverlayReadback())
	request.CanonicalSource = workflowContextCanonicalSourceFunc(func(
		_ context.Context, opts promptlayer.ContextDeliveryOptions,
	) (promptlayer.ContextDeliveryResult, promptlayer.OMPContextEphemeral, error) {
		delivery, rebuildErr := promptlayer.BuildContextDelivery(opts)
		return delivery, request.Binding.Ephemeral, rebuildErr
	})
	receipt, err := RunWorkflowContextInstalledManagedCanary(
		ctx, NewWorkflowContextRuntimeSupervisor(nil), request, driver,
	)
	if err != nil {
		observation := driver.Observation()
		requests, authHeaders, unexpectedEndpoints, failure := provider.receipt()
		t.Fatalf("managed installed admission failed closed: reason=%s provider_requests=%d auth=%d endpoints=%d provider_failure=%s pre_ack=%d post_ack=%d native_start=%d native_end=%d provider_observed=%t",
			liveReason(err), requests, authHeaders, unexpectedEndpoints, failure,
			observation.PreACKs, observation.PostACKs, observation.NativeStarts,
			observation.NativeEnds, observation.ProviderObserved)
	}
	assert.Equal(t, WorkflowContextOutcomeAdmitted, receipt.Outcome)
	assert.True(t, receipt.ExactMatch)
	assert.Equal(t, []string{"checkpointed", "compacted", "rehydrated", "admitted"}, receipt.PhaseSequence)

	observation := driver.Observation()
	assert.Equal(t, 3, observation.ProviderTurns)
	assert.Equal(t, 1, observation.PreACKs)
	assert.Equal(t, 1, observation.PostACKs)
	assert.Equal(t, 1, observation.NativeStarts)
	assert.Equal(t, 1, observation.NativeEnds)
	assert.True(t, observation.SameProcess)
	assert.True(t, observation.SameSession)
	assert.True(t, observation.Sandboxed)
	assert.True(t, observation.ProviderObserved)
	assert.False(t, observation.ProcessActiveAfterCleanup)

	requests, authHeaders, unexpectedEndpoints, failure := provider.receipt()
	require.Empty(t, failure)
	assert.Equal(t, 3, requests)
	assert.Zero(t, authHeaders)
	assert.Zero(t, unexpectedEndpoints)
	assertWorkflowContextManagedAdmissionMessage(t, provider.userMessage(3), request)
	assert.True(t, receipt.Cleanup.Verified)
	assert.Zero(t, receipt.ArtifactCounts.AfterCleanup)
	assert.Zero(t, workflowContextLiveRootCount(layout.runtime))
	t.Logf("installed_managed_context_canary version=%s provider_requests=%d pre_ack=%d post_ack=%d native_start=%d native_end=%d same_pid=%t same_session=%t exact_body=%t cleanup_root_count=0 sandbox=%t",
		version, requests, observation.PreACKs, observation.PostACKs, observation.NativeStarts,
		observation.NativeEnds, observation.SameProcess, observation.SameSession,
		observation.ProviderObserved, observation.Sandboxed)
}

func installWorkflowContextManagedLiveBridge(t *testing.T, workspace string) {
	t.Helper()
	cfg := config.DefaultFullConfig("managed-context-live")
	cfg.Platforms = []string{"omp"}
	cfg.OMPContextPolicy = config.OMPContextPolicyConf{
		Profile:  "managed",
		Profiles: map[string]config.OMPContextProfileConf{"managed": {}},
	}
	_, err := ompadapter.NewWithRoot(workspace).Generate(context.Background(), cfg)
	require.NoError(t, err)
}

func assertWorkflowContextManagedAdmissionMessage(
	t *testing.T, message string, request WorkflowContextRuntimeRequest,
) {
	t.Helper()
	var admission workflowContextManagedAdmission
	require.NoError(t, json.Unmarshal([]byte(message), &admission))
	assert.Equal(t, workflowContextManagedAdmissionSchemaVersion, admission.SchemaVersion)
	assert.Equal(t, WorkflowContextDispatchOptimized, admission.Mode)
	assert.Equal(t, request.Binding.Delivery.Prompt, admission.CanonicalPrompt)
	require.Len(t, admission.Documents, 5, "managed admission must carry the exact canonical five-document set")
	require.Len(t, admission.Documents, len(request.Binding.Delivery.Layers))
	for index, layer := range request.Binding.Delivery.Layers {
		assert.Equal(t, layer.SourceRef, admission.Documents[index].SourceRef)
		assert.Equal(t, layer.Content, admission.Documents[index].Body)
	}
	assert.Equal(t, request.Binding.Ephemeral.OriginalTask, admission.OriginalTask)
	assert.Equal(t, request.Binding.Ephemeral.DecisionDelta, admission.DecisionDelta)
	assert.Equal(t, request.Binding.Ephemeral.FrozenFindingIDs, admission.FrozenFindingIDs)
	assert.Equal(t, request.Binding.Ephemeral.OwnershipPaths, admission.OwnershipPaths)
	assert.Equal(t, request.Binding.Ephemeral.ForbiddenPaths, admission.ForbiddenPaths)
	assert.Equal(t, promptlayer.OMPWorkerResultSchema(), admission.WorkerResultFields)
	assert.Empty(t, admission.DocumentOmissions)
	assert.Empty(t, admission.MemoryInjections)
}

func TestWorkflowContextRuntime_InstalledOMPCompactionLifecycleCanary(t *testing.T) {
	if os.Getenv("AUTOPUS_OMP_CONTEXT_LIVE") != "1" {
		t.Skip("set AUTOPUS_OMP_CONTEXT_LIVE=1 to run the installed OMP context lifecycle canary")
	}
	executable, err := exec.LookPath("omp")
	if err != nil {
		t.Fatal("installed OMP executable was not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	layout, err := newWorkflowContextLiveLayout(t.TempDir())
	if err != nil {
		t.Fatalf("task-owned runtime setup failed: %s", liveReason(err))
	}
	provider := newWorkflowContextLiveProvider(t)
	if err := layout.writeConfig(provider.URL()); err != nil {
		t.Fatalf("task-owned config setup failed: %s", liveReason(err))
	}
	version, err := probeWorkflowContextLiveVersion(ctx, executable, layout, provider.URL())
	if err != nil {
		t.Fatalf("installed OMP identity failed: %s", liveReason(err))
	}
	if err := verifyWorkflowContextLiveOverlay(ctx, executable, layout, provider.URL()); err != nil {
		t.Fatalf("one-shot overlay readback failed: %s", liveReason(err))
	}

	request := newWorkflowContextRuntimeFixture(t)
	request.Capabilities.Version = version
	request.Capabilities.ProbeSource = "installed-omp-loopback-lifecycle"
	request.Capabilities.CheckedAt = time.Now().UTC()
	spy := &workflowContextLiveDispatchSpy{}
	driver := &workflowContextLiveDriver{
		executable: executable, layout: layout, endpoint: provider.URL(),
		beforePost: func() error {
			if spy.calls() != 0 {
				return errWorkflowContextLiveEarlyAdmission
			}
			return nil
		},
	}
	request.Driver = driver
	request.Overlay = newFakeWorkflowContextOverlay(t, activeOverlayReadback(), shadowOverlayReadback())
	expected, err := promptlayer.BuildOMPContextBinding(request.Binding)
	if err != nil {
		t.Fatal("canonical binding fixture failed")
	}

	receipt, err := RunWorkflowContextInstalledCanary(
		ctx, NewWorkflowContextRuntimeSupervisor(nil), request, spy.Dispatch,
	)
	if err != nil {
		t.Fatalf("installed lifecycle canary failed: %s", liveReason(err))
	}
	if receipt.Outcome != WorkflowContextOutcomeAdmitted || !receipt.ExactMatch {
		t.Fatalf("rehydration was not admitted exactly: outcome=%s exact=%t", receipt.Outcome, receipt.ExactMatch)
	}
	if !reflect.DeepEqual(receipt.FullDocumentRefs, expected.FullDocumentRefs) ||
		!reflect.DeepEqual(receipt.RequiredEphemeralRefs, expected.RequiredEphemeralRefs) {
		t.Fatal("canonical or ephemeral reference set changed across native compaction")
	}
	if !reflect.DeepEqual(receipt.PhaseSequence, []string{"checkpointed", "compacted", "rehydrated", "admitted"}) {
		t.Fatalf("unexpected supervisor phase sequence: %v", receipt.PhaseSequence)
	}
	if spy.optimizedCalls() != 1 || !spy.matches(request.Binding.Ephemeral) {
		t.Fatal("optimized dispatch did not receive exactly one exact ephemeral rehydration")
	}
	if !receipt.Cleanup.Verified || receipt.ArtifactCounts.AfterCleanup != 0 || workflowContextLiveRootCount(layout.runtime) != 0 {
		t.Fatal("task-owned runtime root cleanup was not verified")
	}
	requests, authHeaders, unexpectedEndpoints, failure := provider.receipt()
	if requests != 2 || authHeaders != 0 || unexpectedEndpoints != 0 || failure != "" {
		t.Fatalf("loopback provider proof failed: requests=%d auth=%d endpoints=%d reason=%s",
			requests, authHeaders, unexpectedEndpoints, failure)
	}
	if workflowContextLiveSandboxRequired() && !driver.sandboxed {
		t.Fatal("OS network sandbox was not applied to the installed lifecycle")
	}
	t.Logf("installed_context_canary version=%s threshold=7000 provider_turns=%d loopback_requests=%d external_requests=0 native_start=%d native_end=%d early_admission=0 optimized_dispatch=1 cleanup_root_count=0 sandbox=%t",
		version, driver.providerTurns, requests, driver.nativeStart, driver.nativeEnd, driver.sandboxed)
}

type workflowContextLiveDispatchSpy struct {
	mu        sync.Mutex
	optimized int
	ephemeral promptlayer.OMPContextEphemeral
}

func (s *workflowContextLiveDispatchSpy) Dispatch(_ context.Context, input WorkflowContextDispatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if input.Mode != WorkflowContextDispatchOptimized {
		return errWorkflowContextLiveUnexpectedDispatch
	}
	s.optimized++
	s.ephemeral = promptlayer.OMPContextEphemeral{
		OriginalTask: input.Transient.OriginalTask(), DecisionDelta: input.Transient.DecisionDelta(),
		FrozenFindingIDs: input.Transient.FrozenFindingIDs(), OwnershipPaths: input.Transient.OwnershipPaths(),
		ForbiddenPaths: input.Transient.ForbiddenPaths(),
	}
	return nil
}

func (s *workflowContextLiveDispatchSpy) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.optimized
}

func (s *workflowContextLiveDispatchSpy) optimizedCalls() int { return s.calls() }

func (s *workflowContextLiveDispatchSpy) matches(expected promptlayer.OMPContextEphemeral) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return reflect.DeepEqual(s.ephemeral, expected)
}
