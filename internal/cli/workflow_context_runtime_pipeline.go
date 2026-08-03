package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/insajin/autopus-adk/pkg/pipeline"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

// workflowContextPipelineAuthority verifies optional active-history evidence.
// The canonical-full RPC backend never enables compaction from this result.
type workflowContextPipelineAuthority struct {
	View           *pipeline.OMPExecutionView
	RuntimeVersion string
	Now            func() time.Time
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: this is the verified body-free promotion evidence boundary.
// @AX:REASON [AUTO]: Runtime identity, policy, delivery binding, canary evidence, and eligible history references converge here.
// @AX:WARN [AUTO]: Evidence assembly has more than eight sealed-view, policy, delivery, freshness, and binding branches.
// @AX:REASON [AUTO]: Active-history evidence must fail closed on any authority drift and never alter canonical-full execution implicitly.
// @AX:TODO [AUTO] @AX:CYCLE:1 @AX:SPEC: SPEC-OMP-004: admit active compaction only after production canary and reusable-session gates pass.
func (authority workflowContextPipelineAuthority) Assemble(
	_ context.Context,
) (pipeline.OMPExecutionSnapshot, promptlayer.OMPContextVerifiedEvidenceStoreV1, error) {
	binding, err := authority.View.Binding()
	if err != nil {
		return pipeline.OMPExecutionSnapshot{}, promptlayer.OMPContextVerifiedEvidenceStoreV1{}, err
	}
	snapshot, err := authority.View.Open(binding)
	if err != nil {
		return pipeline.OMPExecutionSnapshot{}, promptlayer.OMPContextVerifiedEvidenceStoreV1{}, err
	}
	if _, err := os.Lstat(promptlayer.OMPContextEvidenceStorePath(snapshot.ProjectDir)); err != nil {
		return pipeline.OMPExecutionSnapshot{}, promptlayer.OMPContextVerifiedEvidenceStoreV1{},
			fmt.Errorf("OMP context evidence store is unavailable: %w", err)
	}
	cfg, err := loadHarnessConfigForDir(snapshot.ProjectDir, globalFlags{})
	if err != nil {
		return pipeline.OMPExecutionSnapshot{}, promptlayer.OMPContextVerifiedEvidenceStoreV1{},
			fmt.Errorf("load OMP pipeline evidence policy: %w", err)
	}
	policy, selected, err := workflowContextPolicyFromConfig(cfg)
	if err != nil || !selected {
		return pipeline.OMPExecutionSnapshot{}, promptlayer.OMPContextVerifiedEvidenceStoreV1{},
			fmt.Errorf("OMP context evidence store policy is unavailable: %w", err)
	}
	options := promptlayer.ContextDeliveryOptions{
		Root: snapshot.ProjectDir, Command: pipelineOMPContextCommand(snapshot.PhaseID), SpecDir: snapshot.SpecDir,
	}
	delivery, err := promptlayer.BuildContextDelivery(options)
	if err != nil {
		return pipeline.OMPExecutionSnapshot{}, promptlayer.OMPContextVerifiedEvidenceStoreV1{}, err
	}
	history := pipelineOMPContextHistory(snapshot.CompletedHistory)
	identity := pipelineOMPContextIdentity(snapshot)
	bindingInput := promptlayer.OMPContextBindingInput{
		WorkspaceID: cfg.ProjectName, SpecID: snapshot.SpecID, TaskID: identity[0],
		Phase: string(snapshot.PhaseID), SessionID: identity[1], DeliveryOptions: options, Delivery: delivery,
		Ephemeral: promptlayer.OMPContextEphemeral{OriginalTask: snapshot.Prompt}, History: history,
	}
	receipt, err := promptlayer.BuildOMPContextBinding(bindingInput)
	if err != nil {
		return pipeline.OMPExecutionSnapshot{}, promptlayer.OMPContextVerifiedEvidenceStoreV1{}, err
	}
	now := time.Now().UTC()
	if authority.Now != nil {
		now = authority.Now().UTC()
	}
	expectedPolicy := promptlayer.OMPContextPromotionPolicyV1{
		Profile: policy.Profile, HistoryMode: policy.HistoryMode, MemoryMode: policy.MemoryMode,
		HistoryTargetTokens: policy.HistoryTargetTokens, Fallback: policy.Fallback,
		CapabilityPolicy: policy.CapabilityPolicy, RuntimeRootPolicy: policy.RuntimeRootPolicy,
		MutationScope: policy.MutationScope,
	}
	verified, err := promptlayer.LoadOMPContextEvidenceForExpectationV1(
		snapshot.ProjectDir,
		promptlayer.OMPContextEvidenceExpectationV1{
			WorkspaceID: cfg.ProjectName, SpecID: snapshot.SpecID, SnapshotHash: snapshot.SnapshotHash,
			GitCommitHash: snapshot.GitCommitHash, RuntimeVersion: authority.RuntimeVersion,
		},
		promptlayer.OMPContextPromotionSubjectV1{
			WorkspaceID: cfg.ProjectName, SpecID: snapshot.SpecID, TaskID: identity[0],
			Phase: string(snapshot.PhaseID), SessionID: identity[1], BindingHash: receipt.BindingHash,
		},
		expectedPolicy, now,
	)
	if err != nil {
		return pipeline.OMPExecutionSnapshot{}, promptlayer.OMPContextVerifiedEvidenceStoreV1{},
			fmt.Errorf("verify OMP context evidence store: %w", err)
	}
	if !reflect.DeepEqual(verified.HistoryRefs, receipt.EligibleHistoryRefs) {
		return pipeline.OMPExecutionSnapshot{}, promptlayer.OMPContextVerifiedEvidenceStoreV1{},
			fmt.Errorf("OMP context evidence store history mismatch")
	}
	return snapshot, verified, nil
}

func pipelineOMPContextCommand(phase pipeline.PhaseID) string {
	switch phase {
	case pipeline.PhasePlan:
		return "plan"
	case pipeline.PhaseTestScaffold, pipeline.PhaseValidate:
		return "test"
	case pipeline.PhaseReview:
		return "review"
	default:
		return "go"
	}
}

func pipelineOMPContextHistory(outputs []string) []promptlayer.OMPContextHistoryRow {
	rows := make([]promptlayer.OMPContextHistoryRow, 0, len(outputs))
	for index, output := range outputs {
		rows = append(rows, promptlayer.OMPContextHistoryRow{
			ID:        fmt.Sprintf("pipeline-history-%02d", index+1),
			SourceRef: fmt.Sprintf("pipeline/history-%02d", index+1), Body: output,
			Completed: true, Superseded: true,
		})
	}
	return rows
}

func pipelineOMPContextIdentity(snapshot pipeline.OMPExecutionSnapshot) [2]string {
	material := fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d", snapshot.SpecID, snapshot.SnapshotHash,
		snapshot.GitCommitHash, snapshot.PhaseID, snapshot.Attempt)
	sum := sha256.Sum256([]byte(material))
	digest := hex.EncodeToString(sum[:])
	return [2]string{"pipeline-task-" + digest[:16], "pipeline-session-" + digest[16:32]}
}
