package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/stretchr/testify/require"
)

const runtimeSpecDir = ".autopus/specs/SPEC-OMP-004"

func newWorkflowContextRuntimeFixture(t *testing.T) WorkflowContextRuntimeRequest {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"AGENTS.md":                       "project rules",
		".autopus/project/workspace.md":   "workspace policy",
		runtimeSpecDir + "/spec.md":       "# SPEC-OMP-004: Runtime\n\n---\nid: SPEC-OMP-004\n---\n\nrequirements",
		runtimeSpecDir + "/plan.md":       "implementation plan",
		runtimeSpecDir + "/acceptance.md": "acceptance oracle",
	}
	for ref, body := range files {
		path := filepath.Join(root, filepath.FromSlash(ref))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	}
	opts := promptlayer.ContextDeliveryOptions{Root: root, Command: "go", SpecDir: runtimeSpecDir}
	delivery, err := promptlayer.BuildContextDelivery(opts)
	require.NoError(t, err)
	request := WorkflowContextRuntimeRequest{
		Policy: WorkflowContextEffectivePolicy{
			Profile: "active", HistoryMode: config.OMPContextHistoryActive,
			MemoryMode: config.OMPContextMemoryOff, HistoryTargetTokens: 1000,
			Fallback:          config.OMPContextFallbackCanonicalFull,
			CapabilityPolicy:  config.OMPContextCapabilityProbeRequired,
			RuntimeRootPolicy: config.OMPContextRuntimeIsolatedTaskOwned,
			MutationScope:     config.OMPContextMutationSessionOverlay,
		},
		Capabilities: validWorkflowContextCapabilities(),
		Binding: promptlayer.OMPContextBindingInput{
			WorkspaceID: "autopus-adk", SpecID: "SPEC-OMP-004", TaskID: "TASK-7", Phase: "go", SessionID: "session-1",
			DeliveryOptions: opts, Delivery: delivery,
			Ephemeral: promptlayer.OMPContextEphemeral{
				OriginalTask: "implement runtime supervisor", DecisionDelta: "preserve canonical authority",
				FrozenFindingIDs: []string{"F-002", "F-010"}, OwnershipPaths: []string{"internal/cli"},
				ForbiddenPaths: []string{"pkg/adapter", "pkg/promptlayer"},
			},
			History: []promptlayer.OMPContextHistoryRow{{
				ID: "old-read", SourceRef: "tool/read-old", Body: "completed superseded body", Completed: true, Superseded: true,
			}},
		},
		RootClass: config.OMPContextRuntimeIsolatedTaskOwned,
	}
	attachValidWorkflowContextPromotion(t, &request)
	return request
}

func attachValidWorkflowContextPromotion(t *testing.T, request *WorkflowContextRuntimeRequest) {
	t.Helper()
	binding, err := promptlayer.BuildOMPContextBinding(request.Binding)
	require.NoError(t, err)
	rows := make([]promptlayer.OMPContextCanaryRowV1, 0, 40)
	for index := 0; index < 20; index++ {
		fullOrder, optimizedOrder := 1, 2
		if index%2 == 1 {
			fullOrder, optimizedOrder = 2, 1
		}
		taskID := fmt.Sprintf("promotion-%02d", index)
		rows = append(rows,
			promptlayer.OMPContextCanaryRowV1{TaskID: taskID, Variant: promptlayer.OMPContextCanaryVariantFullV1, Order: fullOrder, Tokens: 10000, IntegrityPassed: true, SecurityPassed: true, QualityScore: 100},
			promptlayer.OMPContextCanaryRowV1{TaskID: taskID, Variant: promptlayer.OMPContextCanaryVariantOptimizedV1, Order: optimizedOrder, Tokens: 7500, IntegrityPassed: true, SecurityPassed: true, QualityScore: 100, FallbackVerified: true, RollbackVerified: true},
		)
	}
	checkedAt := time.Now().UTC()
	attestation, err := promptlayer.BuildOMPContextPromotionAttestationV1(promptlayer.OMPContextPromotionAttestationInputV1{
		Subject: workflowContextPromotionSubject(*request, binding.BindingHash), Policy: workflowContextPromotionPolicy(*request),
		Rows: rows, CheckedAt: checkedAt, ValidFor: time.Hour,
	})
	require.NoError(t, err)
	request.Promotion = promptlayer.OMPContextPromotionEvidenceV1{Rows: rows, Attestation: attestation}
}

func validWorkflowContextCapabilities() WorkflowContextCapabilities {
	return WorkflowContextCapabilities{
		Version: "omp/17.1.8", ExecutableIdentity: true, SettingsSchema: true, OverlayReadback: true,
		PreCompactionEvent: true, PostCompactionEvent: true, CanonicalInjection: true,
		AdmissionBlocking: true, IsolatedTaskRoot: true, CleanupReadback: true,
		MemoryInterception: true, AuthNoneLoopback: true, ProviderCredentialAuthority: true, ProbeSource: "fake-runtime",
		CheckedAt: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC),
	}
}

func activeOverlayReadback() WorkflowContextOverlayReadback {
	hash := runtimeHash("active-overlay")
	return WorkflowContextOverlayReadback{
		RequestedHistoryMode: config.OMPContextHistoryActive, EffectiveHistoryMode: config.OMPContextHistoryActive,
		EffectiveMemoryMode: config.OMPContextMemoryOff, PreviousHistoryMode: config.OMPContextHistoryShadow,
		OverlayHash: hash, ReadbackHash: hash,
	}
}

func shadowOverlayReadback() WorkflowContextOverlayReadback {
	hash := runtimeHash("shadow-overlay")
	return WorkflowContextOverlayReadback{
		RequestedHistoryMode: config.OMPContextHistoryShadow, EffectiveHistoryMode: config.OMPContextHistoryShadow,
		EffectiveMemoryMode: config.OMPContextMemoryOff, PreviousHistoryMode: config.OMPContextHistoryActive,
		OverlayHash: hash, ReadbackHash: hash,
	}
}

func runtimeHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

type workflowContextCanonicalSourceFunc func(context.Context, promptlayer.ContextDeliveryOptions) (promptlayer.ContextDeliveryResult, promptlayer.OMPContextEphemeral, error)

func (fn workflowContextCanonicalSourceFunc) Rebuild(ctx context.Context, opts promptlayer.ContextDeliveryOptions) (promptlayer.ContextDeliveryResult, promptlayer.OMPContextEphemeral, error) {
	return fn(ctx, opts)
}
