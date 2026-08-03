package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/pipeline"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func TestWorkflowContextPipelineAuthority_LoadsVerifiedStoreWithoutCallerEvidence(t *testing.T) {
	request := newWorkflowContextRuntimeFixture(t)
	root := request.Binding.DeliveryOptions.Root
	specDir := request.Binding.DeliveryOptions.SpecDir
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	view, err := pipeline.NewOMPExecutionView(pipeline.OMPExecutionViewInput{
		ProjectDir: root, SpecID: "SPEC-OMP-004", SpecDir: specDir,
		SnapshotHash: "sha256:" + strings.Repeat("a", 64), GitCommitHash: strings.Repeat("b", 40),
		PhaseID: pipeline.PhaseImplement, Attempt: 1, Prompt: "/auto go SPEC-OMP-004",
	})
	require.NoError(t, err)

	authority := workflowContextPipelineAuthority{
		View: view, RuntimeVersion: "omp/17.1.8", Now: func() time.Time { return now },
	}
	_, _, err = authority.Assemble(context.Background())
	require.ErrorContains(t, err, "evidence store")

	// The production owner may read verified local evidence, but external intent
	// remains unable to supply promotion, history, capabilities, or session IDs.
	intent := `{"schema_version":"workflow-context-product-intent/v1","original_task":"x","decision_delta":"y","promotion":{}}`
	_, err = decodeWorkflowContextProductIntent(strings.NewReader(intent))
	require.Error(t, err)
	_ = promptlayer.OMPContextEvidenceStoreV1{}
}

func TestWorkflowContextPipelineAuthority_FailsClosedBeforeEvidenceAdmission(t *testing.T) {
	_, _, err := (workflowContextPipelineAuthority{}).Assemble(context.Background())
	require.ErrorContains(t, err, "sealed OMP execution view")

	request := newWorkflowContextRuntimeFixture(t)
	root := request.Binding.DeliveryOptions.Root
	view, err := pipeline.NewOMPExecutionView(pipeline.OMPExecutionViewInput{
		ProjectDir: root, SpecID: "SPEC-OMP-004", SpecDir: request.Binding.DeliveryOptions.SpecDir,
		SnapshotHash: "sha256:" + strings.Repeat("a", 64), GitCommitHash: strings.Repeat("b", 40),
		PhaseID: pipeline.PhaseImplement, Attempt: 1, Prompt: "implement canonical phase",
	})
	require.NoError(t, err)
	evidencePath := promptlayer.OMPContextEvidenceStorePath(root)
	require.NoError(t, os.MkdirAll(filepath.Dir(evidencePath), 0o700))
	require.NoError(t, os.WriteFile(evidencePath, []byte("{}\n"), 0o600))

	require.NoError(t, config.Save(root, config.DefaultFullConfig("autopus-adk")))
	_, _, err = (workflowContextPipelineAuthority{View: view, RuntimeVersion: "omp/17.1.8"}).Assemble(context.Background())
	require.ErrorContains(t, err, "policy is unavailable")

	cfg := config.DefaultFullConfig("autopus-adk")
	cfg.OMPContextPolicy = config.OMPContextPolicyConf{
		Profile: "active", Profiles: map[string]config.OMPContextProfileConf{"active": {
			HistoryMode: config.OMPContextHistoryActive, MemoryMode: config.OMPContextMemoryOff,
			HistoryTargetTokens: 1000, Fallback: config.OMPContextFallbackCanonicalFull,
			CapabilityPolicy:  config.OMPContextCapabilityProbeRequired,
			RuntimeRootPolicy: config.OMPContextRuntimeIsolatedTaskOwned,
			MutationScope:     config.OMPContextMutationSessionOverlay,
		}},
	}
	require.NoError(t, config.Save(root, cfg))
	require.NoError(t, os.Remove(filepath.Join(root, "AGENTS.md")))
	_, _, err = (workflowContextPipelineAuthority{View: view, RuntimeVersion: "omp/17.1.8"}).Assemble(context.Background())
	require.Error(t, err)
}

func TestWorkflowContextPipelineAuthority_AssemblesBoundProductionEvidence(t *testing.T) {
	request := newWorkflowContextRuntimeFixture(t)
	root := request.Binding.DeliveryOptions.Root
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	cfg := config.DefaultFullConfig("autopus-adk")
	cfg.OMPContextPolicy = config.OMPContextPolicyConf{
		Profile: "active",
		Profiles: map[string]config.OMPContextProfileConf{"active": {
			HistoryMode: config.OMPContextHistoryActive, MemoryMode: config.OMPContextMemoryOff,
			HistoryTargetTokens: 1000, Fallback: config.OMPContextFallbackCanonicalFull,
			CapabilityPolicy:  config.OMPContextCapabilityProbeRequired,
			RuntimeRootPolicy: config.OMPContextRuntimeIsolatedTaskOwned,
			MutationScope:     config.OMPContextMutationSessionOverlay,
		}},
	}
	require.NoError(t, config.Save(root, cfg))

	view, err := pipeline.NewOMPExecutionView(pipeline.OMPExecutionViewInput{
		ProjectDir: root, SpecID: "SPEC-OMP-004", SpecDir: request.Binding.DeliveryOptions.SpecDir,
		SnapshotHash: "sha256:" + strings.Repeat("a", 64), GitCommitHash: strings.Repeat("b", 40),
		PhaseID: pipeline.PhaseImplement, Attempt: 1, Prompt: "implement canonical phase",
		CompletedHistory: []string{"completed superseded phase output"},
	})
	require.NoError(t, err)
	binding, err := view.Binding()
	require.NoError(t, err)
	snapshot, err := view.Open(binding)
	require.NoError(t, err)
	delivery, err := promptlayer.BuildContextDelivery(promptlayer.ContextDeliveryOptions{
		Root: root, Command: "go", SpecDir: snapshot.SpecDir,
	})
	require.NoError(t, err)
	identity := pipelineOMPContextIdentity(snapshot)
	receipt, err := promptlayer.BuildOMPContextBinding(promptlayer.OMPContextBindingInput{
		WorkspaceID: cfg.ProjectName, SpecID: snapshot.SpecID, TaskID: identity[0],
		Phase: string(snapshot.PhaseID), SessionID: identity[1],
		DeliveryOptions: promptlayer.ContextDeliveryOptions{Root: root, Command: "go", SpecDir: snapshot.SpecDir},
		Delivery:        delivery, Ephemeral: promptlayer.OMPContextEphemeral{OriginalTask: snapshot.Prompt},
		History: pipelineOMPContextHistory(snapshot.CompletedHistory),
	})
	require.NoError(t, err)
	policy := promptlayer.OMPContextPromotionPolicyV1{
		Profile: "active", HistoryMode: config.OMPContextHistoryActive, MemoryMode: config.OMPContextMemoryOff,
		HistoryTargetTokens: 1000, Fallback: config.OMPContextFallbackCanonicalFull,
		CapabilityPolicy:  config.OMPContextCapabilityProbeRequired,
		RuntimeRootPolicy: config.OMPContextRuntimeIsolatedTaskOwned,
		MutationScope:     config.OMPContextMutationSessionOverlay,
	}
	require.NoError(t, promptlayer.WriteOMPContextEvidenceStoreV1(root, promptlayer.OMPContextEvidenceStoreV1{
		Binding: promptlayer.OMPContextEvidenceStoreBindingV1{
			WorkspaceID: cfg.ProjectName, SpecID: snapshot.SpecID, SnapshotHash: snapshot.SnapshotHash,
			GitCommitHash: snapshot.GitCommitHash, RuntimeVersion: "omp/17.1.8",
			CheckedAt: now, ValidFor: time.Hour,
		},
		Policy: policy, CanaryRows: request.Promotion.Rows, HistoryRefs: receipt.EligibleHistoryRefs,
	}))

	assembled, evidence, err := (workflowContextPipelineAuthority{
		View: view, RuntimeVersion: "omp/17.1.8", Now: func() time.Time { return now.Add(time.Minute) },
	}).Assemble(context.Background())

	require.NoError(t, err)
	require.Equal(t, snapshot.Prompt, assembled.Prompt)
	require.Equal(t, snapshot.CompletedHistory, assembled.CompletedHistory)
	require.Equal(t, receipt.EligibleHistoryRefs, evidence.HistoryRefs)
	require.Len(t, evidence.Promotion.Rows, 40)
}

func TestPipelineOMPContextHelpers_MapAllPhasesAndBindHistory(t *testing.T) {
	t.Parallel()
	require.Equal(t, "plan", pipelineOMPContextCommand(pipeline.PhasePlan))
	require.Equal(t, "test", pipelineOMPContextCommand(pipeline.PhaseTestScaffold))
	require.Equal(t, "test", pipelineOMPContextCommand(pipeline.PhaseValidate))
	require.Equal(t, "review", pipelineOMPContextCommand(pipeline.PhaseReview))
	require.Equal(t, "go", pipelineOMPContextCommand(pipeline.PhaseImplement))

	history := pipelineOMPContextHistory([]string{"one", "two"})
	require.Len(t, history, 2)
	require.Equal(t, "pipeline-history-01", history[0].ID)
	require.True(t, history[1].Completed)
	require.True(t, history[1].Superseded)
}
