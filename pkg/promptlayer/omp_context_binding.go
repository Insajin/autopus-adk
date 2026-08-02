package promptlayer

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type ompContextBodies struct {
	values map[string][]byte
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-003: public binding boundary for stable, snapshot, and ephemeral context layers.
// @AX:REASON [AUTO]: runtime supervision and receipt consumers depend on its canonical hashes, references, and delivery projection.
func BuildOMPContextBinding(input OMPContextBindingInput) (OMPContextBindingReceipt, error) {
	receipt, bodies, err := buildOMPContextBinding(input)
	if bodies != nil {
		defer bodies.zeroize()
	}
	return receipt, err
}

// @AX:WARN [AUTO]: context binding validation contains 13 if branches.
// @AX:REASON [AUTO]: stable, snapshot, ephemeral, reference, and delivery invariants are independently fail-closed.
func buildOMPContextBinding(input OMPContextBindingInput) (OMPContextBindingReceipt, *ompContextBodies, error) {
	workspaceID, err := validateOMPContextMetadata("workspace_id", input.WorkspaceID)
	if err != nil {
		return OMPContextBindingReceipt{}, nil, err
	}
	specID, err := validateOMPContextMetadata("spec_id", input.SpecID)
	if err != nil {
		return OMPContextBindingReceipt{}, nil, err
	}
	taskID, err := validateOMPContextMetadata("task_id", input.TaskID)
	if err != nil {
		return OMPContextBindingReceipt{}, nil, err
	}
	phase, err := validateOMPContextMetadata("phase", input.Phase)
	if err != nil {
		return OMPContextBindingReceipt{}, nil, err
	}
	sessionID, err := validateOMPContextMetadata("session_id", input.SessionID)
	if err != nil {
		return OMPContextBindingReceipt{}, nil, err
	}
	if err := VerifyContextDeliveryForOptions(input.DeliveryOptions, input.Delivery); err != nil {
		return OMPContextBindingReceipt{}, nil, fmt.Errorf("verify OMP context delivery: %w", err)
	}

	fullRefs, err := ompFullDocumentReferences(input.Delivery)
	if err != nil {
		return OMPContextBindingReceipt{}, nil, err
	}
	bodies, ephemeralRefs, err := buildOMPContextBodies(input.Ephemeral)
	if err != nil {
		return OMPContextBindingReceipt{}, nil, err
	}
	if err := ensureOMPContextReferenceSetsDisjoint(fullRefs, ephemeralRefs); err != nil {
		bodies.zeroize()
		return OMPContextBindingReceipt{}, nil, err
	}
	historyRefs, err := ompEligibleHistoryReferences(input.History)
	if err != nil {
		bodies.zeroize()
		return OMPContextBindingReceipt{}, nil, err
	}
	shadowRefs, err := ompShadowPlanReferences(input.ShadowPlan, fullRefs)
	if err != nil {
		bodies.zeroize()
		return OMPContextBindingReceipt{}, nil, err
	}
	optionsHash, err := ompContextOptionsHash(input.DeliveryOptions)
	if err != nil {
		bodies.zeroize()
		return OMPContextBindingReceipt{}, nil, err
	}

	receipt := OMPContextBindingReceipt{
		SchemaVersion: OMPContextReceiptSchemaVersion, Event: "checkpoint",
		WorkspaceID: workspaceID, SpecID: specID, TaskID: taskID, Phase: phase, SessionID: sessionID,
		OptionsHash: optionsHash, SnapshotHash: input.Delivery.SnapshotHash,
		PromptManifestHash: input.Delivery.PromptManifestHash, FullDocumentRefs: fullRefs,
		RequiredEphemeralRefs: ephemeralRefs, EligibleHistoryRefs: historyRefs, ShadowPlanRefs: shadowRefs,
	}
	material, err := json.Marshal(receipt)
	if err != nil {
		bodies.zeroize()
		return OMPContextBindingReceipt{}, nil, fmt.Errorf("encode OMP context binding: %w", err)
	}
	receipt.BindingHash = canonicalHash(material)
	return receipt, bodies, nil
}

func ompFullDocumentReferences(delivery ContextDeliveryResult) ([]OMPContextDocumentReference, error) {
	refs := make([]OMPContextDocumentReference, 0, len(delivery.RequiredDocuments))
	seen := make(map[string]bool, len(delivery.RequiredDocuments))
	for _, document := range delivery.RequiredDocuments {
		if !document.Complete {
			return nil, fmt.Errorf("OMP context document is incomplete: %s", document.SourceRef)
		}
		ref, err := cleanContextReference(document.SourceRef, true)
		if err != nil || seen[ref] {
			return nil, fmt.Errorf("invalid or duplicate OMP context document: %s", document.SourceRef)
		}
		if _, err := validateOMPContextMetadata("document_source_ref", ref); err != nil {
			return nil, err
		}
		if !isOMPContextHash(document.SourceHash) || !isOMPContextHash(document.PromptHash) {
			return nil, fmt.Errorf("invalid OMP context document hash: %s", ref)
		}
		seen[ref] = true
		refs = append(refs, OMPContextDocumentReference{
			SourceRef: ref, SourceHash: document.SourceHash, PromptHash: document.PromptHash,
			Kind: document.Kind, Complete: true,
		})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].SourceRef < refs[j].SourceRef })
	return refs, nil
}

func buildOMPContextBodies(input OMPContextEphemeral) (*ompContextBodies, []OMPContextHashedReference, error) {
	if strings.TrimSpace(input.OriginalTask) == "" {
		return nil, nil, fmt.Errorf("OMP original task is required")
	}
	findings, err := normalizeOMPContextMetadataList("frozen_finding_ids", input.FrozenFindingIDs)
	if err != nil {
		return nil, nil, err
	}
	ownership, err := normalizeOMPContextPaths("ownership_paths", input.OwnershipPaths)
	if err != nil {
		return nil, nil, err
	}
	forbidden, err := normalizeOMPContextPaths("forbidden_paths", input.ForbiddenPaths)
	if err != nil {
		return nil, nil, err
	}
	values := map[string][]byte{
		"original_task":        []byte(input.OriginalTask),
		"decision_delta":       []byte(input.DecisionDelta),
		"frozen_findings":      mustOMPContextJSON(findings),
		"ownership_paths":      mustOMPContextJSON(ownership),
		"forbidden_paths":      mustOMPContextJSON(forbidden),
		"worker_result_schema": mustOMPContextJSON(OMPWorkerResultSchema()),
	}
	bodies := &ompContextBodies{values: values}
	return bodies, bodies.references(), nil
}

func ensureOMPContextReferenceSetsDisjoint(documents []OMPContextDocumentReference, ephemeral []OMPContextHashedReference) error {
	documentRefs := make(map[string]bool, len(documents))
	for _, document := range documents {
		documentRefs[document.SourceRef] = true
	}
	for _, ref := range ephemeral {
		if documentRefs[ref.ID] {
			return fmt.Errorf("OMP document and ephemeral reference sets overlap: %s", ref.ID)
		}
	}
	return nil
}

func ompEligibleHistoryReferences(rows []OMPContextHistoryRow) ([]OMPContextHistoryReference, error) {
	refs := make([]OMPContextHistoryReference, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		if !row.Completed || !row.Superseded || row.Document || row.Unresolved {
			continue
		}
		id, err := validateOMPContextMetadata("history_id", row.ID)
		if err != nil || seen[id] {
			return nil, fmt.Errorf("invalid or duplicate OMP history row: %s", row.ID)
		}
		sourceRef, err := cleanContextReference(row.SourceRef, true)
		if err != nil {
			return nil, fmt.Errorf("invalid OMP history source ref: %w", err)
		}
		if _, err := validateOMPContextMetadata("history_source_ref", sourceRef); err != nil {
			return nil, err
		}
		if strings.TrimSpace(row.Body) == "" {
			return nil, fmt.Errorf("eligible OMP history body is empty: %s", id)
		}
		seen[id] = true
		refs = append(refs, OMPContextHistoryReference{
			ID: id, SourceRef: sourceRef, BodyHash: canonicalHash([]byte(row.Body)),
			TokenEstimate: EstimateTokens(row.Body), Reason: "completed-superseded",
		})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].ID < refs[j].ID })
	return refs, nil
}

func ompShadowPlanReferences(plan *OMPContextShadowPlan, documents []OMPContextDocumentReference) ([]OMPContextPlanReference, error) {
	if plan == nil {
		return []OMPContextPlanReference{}, nil
	}
	if plan.SchemaVersion != "autopus.context_plan.v2" || !plan.ShadowOnly || plan.ActiveMode != "full" || plan.CandidateMode != "jit" {
		return nil, fmt.Errorf("OMP context plan must remain shadow-only full-to-jit evidence")
	}
	known := make(map[string]string, len(documents))
	for _, document := range documents {
		known[document.SourceRef] = document.SourceHash
	}
	var refs []OMPContextPlanReference
	seen := map[string]bool{}
	appendRefs := func(values []OMPContextPlanReference, disposition string) error {
		for _, value := range values {
			ref, err := cleanContextReference(value.SourceRef, true)
			if err != nil || seen[ref] || known[ref] != value.SourceHash {
				return fmt.Errorf("invalid, duplicate, or stale OMP shadow reference: %s", value.SourceRef)
			}
			seen[ref] = true
			refs = append(refs, OMPContextPlanReference{SourceRef: ref, SourceHash: value.SourceHash, Disposition: disposition})
		}
		return nil
	}
	if err := appendRefs(plan.PinnedReferences, "pinned"); err != nil {
		return nil, err
	}
	if err := appendRefs(plan.SelectedReferences, "selected"); err != nil {
		return nil, err
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].SourceRef < refs[j].SourceRef })
	return refs, nil
}
