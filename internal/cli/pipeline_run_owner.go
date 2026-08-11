package cli

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	pipelineExecutionOwnerOMP            = "omp"
	pipelineExecutionOwnerOrca           = "orca"
	pipelineExecutionOwnerSourceDefault  = "default"
	pipelineExecutionOwnerSourceExplicit = "explicit"
	pipelineExecutionOwnerReceiptSchema  = "pipeline_execution_owner_receipt.v1"
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

func persistPipelineExecutionOwnerReceipt(
	specID string,
	decision pipelineExecutionOwnerDecision,
	verificationStatus string,
) (pipelineExecutionOwnerReceipt, string, error) {
	receipt := pipelineExecutionOwnerReceipt{
		Schema: pipelineExecutionOwnerReceiptSchema, Owner: decision.Owner,
		Source: decision.Source, Reason: decision.Reason, SpecID: specID,
		RunID: newPipelineExecutionOwnerRunID(), CheckedAt: time.Now().UTC(),
		// Derived from the tier integrity gate. It used to be a hardcoded
		// "verified" that asserted a check nobody had run.
		VerificationStatus: verificationStatus,
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

// reportPipelineTierIntegrity writes the gate's verdict to the run's diagnostic
// stream. The verdict is not fatal — an unverified tier contract degrades the
// run rather than stopping it — so this report is the only thing standing
// between a degraded contract and a silent one. The receipt path travels with it
// because the per-provider record is what makes the verdict reconstructible.
func reportPipelineTierIntegrity(w io.Writer, integrity pipelineTierIntegrityResult) {
	fmt.Fprintf(w, "Tier integrity %s: %s\n", integrity.Status, integrity.Reason)
	if integrity.ReceiptPath != "" {
		fmt.Fprintf(w, "Tier integrity receipt: %s\n", integrity.ReceiptPath)
	}
}
