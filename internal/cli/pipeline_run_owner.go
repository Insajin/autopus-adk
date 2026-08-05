package cli

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

const (
	pipelineExecutionOwnerOMP            = "omp"
	pipelineExecutionOwnerOrca           = "orca"
	pipelineExecutionOwnerSourceDefault  = "default"
	pipelineExecutionOwnerSourceExplicit = "explicit"
	pipelineExecutionOwnerReceiptSchema  = "pipeline_execution_owner_receipt.v1"
	pipelineExecutionOwnerResultSchema   = "pipeline_execution_owner_result.v1"
)

var errPipelineExecutionOwnerHandoffRequired = errors.New(
	"pipeline: execution owner orca requires a supervised Orca orchestration handoff",
)

type executionOwnerValue struct {
	value    *string
	explicit *bool
}

func newExecutionOwnerValue(value *string, explicit *bool) *executionOwnerValue {
	*value = pipelineExecutionOwnerOMP
	*explicit = false
	return &executionOwnerValue{value: value, explicit: explicit}
}

func (v *executionOwnerValue) String() string { return *v.value }

func (v *executionOwnerValue) Type() string { return "execution-owner" }

func (v *executionOwnerValue) Set(owner string) error {
	if *v.explicit {
		return errors.New("--execution-owner must be specified exactly once")
	}
	if owner != pipelineExecutionOwnerOMP && owner != pipelineExecutionOwnerOrca {
		return fmt.Errorf("invalid execution owner %q: must be exactly omp or orca", owner)
	}
	*v.value = owner
	*v.explicit = true
	return nil
}

type pipelineExecutionOwnerDecision struct {
	Owner  string
	Source string
	Reason string
}

func resolvePipelineExecutionOwner(cfg *pipelineRunConfig) (pipelineExecutionOwnerDecision, error) {
	if cfg == nil {
		return pipelineExecutionOwnerDecision{}, errors.New("pipeline: execution owner config is required")
	}
	owner := cfg.ExecutionOwner
	if owner == "" {
		owner = pipelineExecutionOwnerOMP
	}
	if owner != pipelineExecutionOwnerOMP && owner != pipelineExecutionOwnerOrca {
		return pipelineExecutionOwnerDecision{}, fmt.Errorf(
			"invalid execution owner %q: must be exactly omp or orca", owner,
		)
	}
	if owner == pipelineExecutionOwnerOrca && !cfg.executionOwnerExplicit {
		return pipelineExecutionOwnerDecision{}, errors.New(
			"pipeline: execution owner orca must be selected explicitly",
		)
	}
	source := pipelineExecutionOwnerSourceDefault
	if cfg.executionOwnerExplicit {
		if cfg.Platform != "omp" {
			return pipelineExecutionOwnerDecision{}, errors.New(
				"pipeline: --execution-owner requires exact --platform omp",
			)
		}
		source = pipelineExecutionOwnerSourceExplicit
	}
	reason := "native_omp_dag_selected"
	if owner == pipelineExecutionOwnerOrca {
		reason = "supervised_durable_cross_worktree_run_selected"
	}
	return pipelineExecutionOwnerDecision{Owner: owner, Source: source, Reason: reason}, nil
}

type pipelineExecutionOwnerReceipt struct {
	Schema             string    `json:"schema"`
	Owner              string    `json:"owner"`
	Source             string    `json:"source"`
	Reason             string    `json:"reason"`
	SpecID             string    `json:"spec_id"`
	RunID              string    `json:"run_id"`
	CheckedAt          time.Time `json:"checked_at"`
	VerificationStatus string    `json:"verification_status"`
}

type pipelineExecutionOwnerResult struct {
	Schema         string `json:"schema"`
	Status         string `json:"status"`
	Owner          string `json:"owner"`
	Source         string `json:"source"`
	Reason         string `json:"reason"`
	SpecID         string `json:"spec_id"`
	RunID          string `json:"run_id"`
	ReceiptPath    string `json:"receipt_path"`
	RequiredAction string `json:"required_action"`
}

func persistPipelineExecutionOwnerReceipt(
	specID string,
	decision pipelineExecutionOwnerDecision,
) (pipelineExecutionOwnerReceipt, string, error) {
	receipt := pipelineExecutionOwnerReceipt{
		Schema: pipelineExecutionOwnerReceiptSchema, Owner: decision.Owner,
		Source: decision.Source, Reason: decision.Reason, SpecID: specID,
		RunID: newPipelineExecutionOwnerRunID(), CheckedAt: time.Now().UTC(),
		VerificationStatus: "verified",
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return pipelineExecutionOwnerReceipt{}, "", fmt.Errorf("encode execution owner receipt: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(pipelineStateDir, 0o700); err != nil {
		return pipelineExecutionOwnerReceipt{}, "", fmt.Errorf("create execution owner receipt directory: %w", err)
	}
	path := filepath.Join(pipelineStateDir, specID+".execution-owner.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return pipelineExecutionOwnerReceipt{}, "", fmt.Errorf("write execution owner receipt: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return pipelineExecutionOwnerReceipt{}, "", fmt.Errorf("secure execution owner receipt: %w", err)
	}
	return receipt, filepath.ToSlash(path), nil
}

func newPipelineExecutionOwnerRunID() string {
	var value [8]byte
	if _, err := rand.Read(value[:]); err == nil {
		return "pipeline-owner-" + hex.EncodeToString(value[:])
	}
	return fmt.Sprintf("pipeline-owner-%d", time.Now().UTC().UnixNano())
}

func emitPipelineExecutionOwnerHandoff(
	cmd *cobra.Command,
	receipt pipelineExecutionOwnerReceipt,
	receiptPath string,
) error {
	result := pipelineExecutionOwnerResult{
		Schema: pipelineExecutionOwnerResultSchema, Status: "handoff_required",
		Owner: receipt.Owner, Source: receipt.Source, Reason: receipt.Reason,
		SpecID: receipt.SpecID, RunID: receipt.RunID, ReceiptPath: receiptPath,
		RequiredAction: "orca skills get orchestration --full",
	}
	if err := json.NewEncoder(cmd.OutOrStdout()).Encode(result); err != nil {
		return fmt.Errorf("encode execution owner handoff result: %w", err)
	}
	return &jsonFatalError{cause: errPipelineExecutionOwnerHandoffRequired}
}
