package cli

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: versioned admission payload consumed at the live OMP provider boundary.
// @AX:REASON [AUTO]: managed dispatch and provider-observation receipts depend on stable canonical and transient field semantics.
const workflowContextManagedAdmissionSchemaVersion = "autopus.omp-context-admission.v1"

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: the 1 MiB RPC frame cap reserves 4 KiB for protocol framing overhead.
const workflowContextManagedRPCMaxInputFrameBytes = 1 << 20

type workflowContextManagedAdmissionDocument struct {
	SourceRef string `json:"source_ref"`
	Body      string `json:"body"`
}

type workflowContextManagedAdmission struct {
	SchemaVersion      string                                    `json:"schema_version"`
	Mode               string                                    `json:"mode"`
	CanonicalPrompt    string                                    `json:"canonical_prompt"`
	Documents          []workflowContextManagedAdmissionDocument `json:"documents"`
	OriginalTask       string                                    `json:"original_task"`
	DecisionDelta      string                                    `json:"decision_delta"`
	FrozenFindingIDs   []string                                  `json:"frozen_finding_ids"`
	OwnershipPaths     []string                                  `json:"ownership_paths"`
	ForbiddenPaths     []string                                  `json:"forbidden_paths"`
	WorkerResultFields []string                                  `json:"worker_result_fields"`
	DocumentOmissions  []string                                  `json:"document_omissions"`
	MemoryInjections   []string                                  `json:"memory_injections"`
}

func buildWorkflowContextManagedAdmission(dispatch WorkflowContextDispatch) (string, error) {
	if dispatch.Mode != WorkflowContextDispatchOptimized || dispatch.Delivery.Prompt == "" {
		return "", fmt.Errorf("managed OMP dispatch is not optimized canonical context")
	}
	documents := make([]workflowContextManagedAdmissionDocument, 0, len(dispatch.Delivery.Layers))
	for _, layer := range dispatch.Delivery.Layers {
		if layer.SourceRef == "" || layer.Content == "" {
			return "", fmt.Errorf("managed OMP canonical document is incomplete")
		}
		documents = append(documents, workflowContextManagedAdmissionDocument{
			SourceRef: layer.SourceRef, Body: layer.Content,
		})
	}
	if len(documents) != len(dispatch.Delivery.RequiredDocuments) {
		return "", fmt.Errorf("managed OMP canonical document set is incomplete")
	}
	payload := workflowContextManagedAdmission{
		SchemaVersion: workflowContextManagedAdmissionSchemaVersion,
		Mode:          dispatch.Mode, CanonicalPrompt: dispatch.Delivery.Prompt, Documents: documents,
		OriginalTask: dispatch.Transient.OriginalTask(), DecisionDelta: dispatch.Transient.DecisionDelta(),
		FrozenFindingIDs: dispatch.Transient.FrozenFindingIDs(),
		OwnershipPaths:   dispatch.Transient.OwnershipPaths(), ForbiddenPaths: dispatch.Transient.ForbiddenPaths(),
		WorkerResultFields: dispatch.Transient.WorkerResultSchema(),
		DocumentOmissions:  []string{}, MemoryInjections: []string{},
	}
	if !slices.Equal(payload.WorkerResultFields, promptlayer.OMPWorkerResultSchema()) {
		return "", fmt.Errorf("managed OMP worker result contract is incomplete")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode managed OMP admission: %w", err)
	}
	if len(encoded) >= workflowContextManagedRPCMaxInputFrameBytes-4096 {
		return "", fmt.Errorf("managed OMP admission exceeds the RPC input frame limit")
	}
	return string(encoded), nil
}
