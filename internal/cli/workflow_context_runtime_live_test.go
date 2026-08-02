package cli

import (
	"context"
	"os"
	"os/exec"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

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
