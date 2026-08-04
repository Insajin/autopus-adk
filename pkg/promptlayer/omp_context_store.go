package promptlayer

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"sync"
)

type OMPContextTransientStore struct {
	mu      sync.Mutex
	entries map[string]*ompContextTransientState
}

type ompContextTransientState struct {
	binding OMPContextBindingReceipt
	bodies  *ompContextBodies
}

type OMPContextTransientView struct {
	bodies *ompContextBodies
}

func NewOMPContextTransientStore() *OMPContextTransientStore {
	return &OMPContextTransientStore{entries: make(map[string]*ompContextTransientState)}
}

func (store *OMPContextTransientStore) Checkpoint(input OMPContextBindingInput) (OMPContextBindingReceipt, error) {
	if store == nil {
		return OMPContextBindingReceipt{}, fmt.Errorf("OMP context transient store is nil")
	}
	binding, bodies, err := buildOMPContextBinding(input)
	if err != nil {
		return OMPContextBindingReceipt{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.entries == nil {
		store.entries = make(map[string]*ompContextTransientState)
	}
	if _, exists := store.entries[binding.BindingHash]; exists {
		bodies.zeroize()
		return OMPContextBindingReceipt{}, fmt.Errorf("OMP context binding already exists: %s", binding.BindingHash)
	}
	store.entries[binding.BindingHash] = &ompContextTransientState{binding: cloneOMPContextBinding(binding), bodies: bodies}
	return cloneOMPContextBinding(binding), nil
}

func (store *OMPContextTransientStore) Rehydrate(
	bindingHash string,
	opts ContextDeliveryOptions,
	consume func(OMPContextTransientView) error,
) (OMPContextTerminalReceipt, error) {
	state, err := store.take(bindingHash)
	if err != nil {
		return OMPContextTerminalReceipt{}, err
	}
	defer state.bodies.zeroize()
	receipt := terminalOMPContextReceipt("rehydrate", state.binding, "required-context-mismatch")
	optionsHash, optionsErr := ompContextOptionsHash(opts)
	if optionsErr != nil || optionsHash != state.binding.OptionsHash {
		receipt.Reason = "context-options-mismatch"
		return receipt, fmt.Errorf("OMP context options mismatch")
	}
	rebuilt, buildErr := BuildContextDelivery(opts)
	if buildErr != nil {
		return receipt, fmt.Errorf("rebuild OMP required context: %w", buildErr)
	}
	return rehydrateOMPContextState(state, opts, rebuilt, consume)
}

// RehydrateCanonical verifies a supervisor-provided canonical rebuild before admission.
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: public boundary for supervisor-provided canonical rehydration.
// @AX:REASON [AUTO]: managed admission depends on one-shot state consumption, exact delivery verification, and zeroization remaining atomic from the caller's perspective.
func (store *OMPContextTransientStore) RehydrateCanonical(
	bindingHash string,
	opts ContextDeliveryOptions,
	rebuilt ContextDeliveryResult,
	consume func(OMPContextTransientView) error,
) (OMPContextTerminalReceipt, error) {
	state, err := store.take(bindingHash)
	if err != nil {
		return OMPContextTerminalReceipt{}, err
	}
	defer state.bodies.zeroize()
	return rehydrateOMPContextState(state, opts, rebuilt, consume)
}

func rehydrateOMPContextState(
	state *ompContextTransientState,
	opts ContextDeliveryOptions,
	rebuilt ContextDeliveryResult,
	consume func(OMPContextTransientView) error,
) (OMPContextTerminalReceipt, error) {
	receipt := terminalOMPContextReceipt("rehydrate", state.binding, "required-context-mismatch")
	optionsHash, optionsErr := ompContextOptionsHash(opts)
	if optionsErr != nil || optionsHash != state.binding.OptionsHash {
		receipt.Reason = "context-options-mismatch"
		return receipt, fmt.Errorf("OMP context options mismatch")
	}
	if verifyErr := VerifyContextDeliveryForOptions(opts, rebuilt); verifyErr != nil {
		return receipt, fmt.Errorf("verify OMP required context: %w", verifyErr)
	}
	fullRefs, refsErr := ompFullDocumentReferences(rebuilt)
	if refsErr != nil || rebuilt.SnapshotHash != state.binding.SnapshotHash ||
		rebuilt.PromptManifestHash != state.binding.PromptManifestHash ||
		!reflect.DeepEqual(fullRefs, state.binding.FullDocumentRefs) {
		return receipt, fmt.Errorf("OMP required context changed across compaction")
	}
	if !reflect.DeepEqual(state.bodies.references(), state.binding.RequiredEphemeralRefs) {
		receipt.Reason = "ephemeral-context-mismatch"
		return receipt, fmt.Errorf("OMP ephemeral context changed across compaction")
	}
	receipt.ExactMatch = true
	if consume == nil {
		receipt.Reason = "rehydration-consumer-missing"
		return receipt, fmt.Errorf("OMP rehydration consumer is required")
	}
	if consumeErr := consume(OMPContextTransientView{bodies: state.bodies}); consumeErr != nil {
		receipt.Reason = "rehydration-consumer-rejected"
		return receipt, fmt.Errorf("consume OMP rehydrated context: %w", consumeErr)
	}
	receipt.Admission = true
	receipt.Reason = "verified"
	return receipt, nil
}

func (store *OMPContextTransientStore) Abort(bindingHash, reason string) (OMPContextTerminalReceipt, error) {
	reason, err := validateOMPContextMetadata("abort_reason", reason)
	if err != nil {
		return OMPContextTerminalReceipt{}, err
	}
	state, err := store.take(bindingHash)
	if err != nil {
		return OMPContextTerminalReceipt{}, err
	}
	defer state.bodies.zeroize()
	return terminalOMPContextReceipt("abort", state.binding, reason), nil
}

func (store *OMPContextTransientStore) Pending() int {
	if store == nil {
		return 0
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	return len(store.entries)
}

func (store *OMPContextTransientStore) take(bindingHash string) (*ompContextTransientState, error) {
	if store == nil {
		return nil, ErrOMPContextBindingUnavailable
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	state, ok := store.entries[bindingHash]
	if !ok {
		return nil, ErrOMPContextBindingUnavailable
	}
	delete(store.entries, bindingHash)
	return state, nil
}

func terminalOMPContextReceipt(event string, binding OMPContextBindingReceipt, reason string) OMPContextTerminalReceipt {
	return OMPContextTerminalReceipt{
		SchemaVersion: OMPContextReceiptSchemaVersion, Event: event, BindingHash: binding.BindingHash,
		Reason: reason, SnapshotHash: binding.SnapshotHash, PromptManifestHash: binding.PromptManifestHash,
		FullDocumentRefs:      append([]OMPContextDocumentReference(nil), binding.FullDocumentRefs...),
		RequiredEphemeralRefs: append([]OMPContextHashedReference(nil), binding.RequiredEphemeralRefs...),
	}
}

func cloneOMPContextBinding(binding OMPContextBindingReceipt) OMPContextBindingReceipt {
	binding.FullDocumentRefs = append([]OMPContextDocumentReference(nil), binding.FullDocumentRefs...)
	binding.RequiredEphemeralRefs = append([]OMPContextHashedReference(nil), binding.RequiredEphemeralRefs...)
	binding.EligibleHistoryRefs = append([]OMPContextHistoryReference(nil), binding.EligibleHistoryRefs...)
	binding.ShadowPlanRefs = append([]OMPContextPlanReference(nil), binding.ShadowPlanRefs...)
	return binding
}

func (bodies *ompContextBodies) references() []OMPContextHashedReference {
	refs := make([]OMPContextHashedReference, 0, len(bodies.values))
	for id, body := range bodies.values {
		refs = append(refs, OMPContextHashedReference{ID: id, Hash: canonicalHash(body)})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].ID < refs[j].ID })
	return refs
}

func (bodies *ompContextBodies) zeroize() {
	if bodies == nil {
		return
	}
	for id, body := range bodies.values {
		for i := range body {
			body[i] = 0
		}
		delete(bodies.values, id)
	}
}

func (view OMPContextTransientView) MarshalJSON() ([]byte, error) {
	return nil, ErrOMPContextBodySerialization
}

func (view OMPContextTransientView) OriginalTask() string { return string(view.body("original_task")) }
func (view OMPContextTransientView) DecisionDelta() string {
	return string(view.body("decision_delta"))
}

func (view OMPContextTransientView) FrozenFindingIDs() []string {
	return view.list("frozen_findings")
}
func (view OMPContextTransientView) OwnershipPaths() []string { return view.list("ownership_paths") }
func (view OMPContextTransientView) ForbiddenPaths() []string { return view.list("forbidden_paths") }
func (view OMPContextTransientView) WorkerResultSchema() []string {
	return view.list("worker_result_schema")
}

func (view OMPContextTransientView) body(id string) []byte {
	if view.bodies == nil {
		return nil
	}
	return view.bodies.values[id]
}

func (view OMPContextTransientView) list(id string) []string {
	var values []string
	_ = json.Unmarshal(view.body(id), &values)
	return values
}
