package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

// RunWorkflowContextObserveCall runs an observer-canary only. It deliberately
// does not consume or produce promotion authority.
func RunWorkflowContextObserveCall(
	ctx context.Context,
	request workflowContextObserveCallRequest,
	options workflowContextObserveCallOptions,
) (result workflowContextObserveCallResult, runErr error) {
	setup, err := prepareWorkflowContextObserveCall(ctx, request, options)
	if err != nil {
		return result, err
	}
	started := time.Now().UTC()
	removed := 0
	defer func() {
		removeErr := os.RemoveAll(setup.taskRoot)
		if removeErr == nil && workflowContextObservePathGone(setup.taskRoot) {
			removed = 1
		}
		result.CleanupFacts.OwnedRootsRemoved = removed
		result.CleanupFacts.OwnedRootsRemain = 1 - removed
		runErr = errors.Join(runErr, removeErr)
	}()
	ephemeral := promptlayer.OMPContextEphemeral{
		OriginalTask:  "/auto go " + options.SpecID,
		DecisionDelta: request.Prompt,
	}
	message, err := buildWorkflowContextObserveAdmission(request.Variant, setup.delivery, ephemeral)
	if err != nil {
		return result, err
	}
	driverOptions := setup.options
	driverOptions.ProjectDir = setup.projectDir
	driverOptions.Prompts = []string{
		"AUTOPUS_PROVIDER_PHASE=setup\nReply exactly SETUP_ACK_1. Do not call tools.",
		"AUTOPUS_PROVIDER_PHASE=setup\nReply exactly SETUP_ACK_2. Do not call tools.",
	}
	driverOptions.ObserveOnly = true
	driverOptions.InitialManualCompaction = request.Variant == "optimized"
	driver, err := NewWorkflowContextManagedRPCDriver(driverOptions)
	if err != nil {
		return result, err
	}
	output, usage, lifecycle, err := executeWorkflowContextObserveVariant(
		ctx, driver, request, setup, ephemeral, message,
	)
	cleanupErr := driver.Cleanup(context.Background())
	observation := driver.Observation()
	if err != nil || cleanupErr != nil {
		return result, errors.Join(err, cleanupErr)
	}
	if err := validateWorkflowContextObserveOutput(output, usage); err != nil {
		return result, err
	}
	completed := time.Now().UTC()
	result = workflowContextObserveCallResult{
		SchemaVersion: workflowContextObserveCallResultSchema,
		Sequence:      request.Sequence, PairSequence: request.PairSequence, TaskID: request.TaskID,
		Variant: request.Variant, Provider: options.Provider, Model: options.Model,
		ExecutionClass: "observer-canary", ProductionPathEquivalent: false,
		StartedAt: started.Format(time.RFC3339Nano), CompletedAt: completed.Format(time.RFC3339Nano),
		AssistantText: output,
		TokenUsage: workflowContextObserveCallTokenUsage{
			PrimaryInputTokens: usage.PrimaryInputTokens, PrimaryOutputTokens: usage.PrimaryOutputTokens,
			MaintenanceInputTokens:  usage.MaintenanceInputTokens,
			MaintenanceOutputTokens: usage.MaintenanceOutputTokens, TotalTokens: usage.TotalTokens,
		},
		LifecycleFacts: lifecycle,
		CleanupFacts: workflowContextObserveCallCleanupFacts{
			OwnedRootsCreated: 1, OwnedRootsRemain: 0,
		},
	}
	result.LifecycleFacts.Sandboxed = observation.Sandboxed
	if observation.ProcessActiveAfterCleanup {
		result.CleanupFacts.ProcessesRemain = 1
	}
	return result, nil
}

func executeWorkflowContextObserveVariant(
	ctx context.Context,
	driver *WorkflowContextManagedRPCDriver,
	request workflowContextObserveCallRequest,
	setup workflowContextObserveCallSetup,
	ephemeral promptlayer.OMPContextEphemeral,
	message string,
) (string, WorkflowContextProviderUsage, workflowContextObserveCallLifecycleFacts, error) {
	if request.Variant == "full" {
		if err := bindWorkflowContextObserveDriver(ctx, driver, request.TaskID); err != nil {
			return "", WorkflowContextProviderUsage{}, workflowContextObserveCallLifecycleFacts{}, err
		}
		output, usage, err := driver.runCanonicalPrimary(ctx, message)
		observation := driver.Observation()
		return output, usage, observeLifecycleFacts(observation, true), err
	}
	return executeWorkflowContextObserveOptimized(ctx, driver, request, setup, ephemeral)
}

func executeWorkflowContextObserveOptimized(
	ctx context.Context,
	driver *WorkflowContextManagedRPCDriver,
	request workflowContextObserveCallRequest,
	setup workflowContextObserveCallSetup,
	ephemeral promptlayer.OMPContextEphemeral,
) (string, WorkflowContextProviderUsage, workflowContextObserveCallLifecycleFacts, error) {
	store := promptlayer.NewOMPContextTransientStore()
	options := promptlayer.ContextDeliveryOptions{Root: setup.projectDir, Command: "go", SpecDir: setup.delivery.SpecDir}
	bindingInput := promptlayer.OMPContextBindingInput{
		WorkspaceID: "omp-observe", SpecID: request.TaskID, TaskID: request.TaskID,
		Phase: "go", SessionID: fmt.Sprintf("observe-%02d-%d", request.Sequence, request.PairSequence),
		DeliveryOptions: options, Delivery: setup.delivery, Ephemeral: ephemeral,
	}
	binding, err := store.Checkpoint(bindingInput)
	if err != nil {
		return "", WorkflowContextProviderUsage{}, workflowContextObserveCallLifecycleFacts{}, err
	}
	nonce, err := newWorkflowContextRunNonceHash()
	if err != nil {
		return "", WorkflowContextProviderUsage{}, workflowContextObserveCallLifecycleFacts{}, err
	}
	if err := driver.Bind(ctx, WorkflowContextBridgeBinding{
		SchemaVersion: workflowContextBridgeSchemaVersion, BindingHash: binding.BindingHash,
		OptionsHash: binding.OptionsHash, SessionHash: workflowContextRuntimeHash(bindingInput.SessionID), NonceHash: nonce,
	}); err != nil {
		return "", WorkflowContextProviderUsage{}, workflowContextObserveCallLifecycleFacts{}, err
	}
	lifecycle := workflowContextObserveCallLifecycleFacts{}
	var output string
	var usage WorkflowContextProviderUsage
	runErr := driver.Run(ctx, func(event WorkflowContextRuntimeEvent) error {
		switch event.Kind {
		case WorkflowContextEventPreCompaction:
			lifecycle.PreCompactionEvents++
		case WorkflowContextEventCompacted:
			lifecycle.NativeStarts++
		case WorkflowContextEventPostCompaction:
			lifecycle.PostCompactionEvents++
			_, rehydrateErr := store.RehydrateCanonical(binding.BindingHash, options, setup.delivery,
				func(view promptlayer.OMPContextTransientView) error {
					ack, dispatchErr := driver.Dispatch(ctx, WorkflowContextDispatch{
						Mode: WorkflowContextDispatchOptimized, Delivery: setup.delivery, Transient: view,
					})
					if dispatchErr == nil {
						output, usage = ack.providerOutput, ack.providerUsage
					}
					return dispatchErr
				})
			return rehydrateErr
		default:
			return fmt.Errorf("observe-call unexpected lifecycle event %q", event.Kind)
		}
		return nil
	})
	observation := driver.Observation()
	lifecycle = observeLifecycleFacts(observation, runErr == nil)
	return output, usage, lifecycle, runErr
}

func observeLifecycleFacts(
	observation WorkflowContextManagedRPCObservation,
	terminalIdle bool,
) workflowContextObserveCallLifecycleFacts {
	return workflowContextObserveCallLifecycleFacts{
		PreCompactionEvents: observation.PreACKs, PostCompactionEvents: observation.PostACKs,
		NativeStarts: observation.NativeStarts, NativeEnds: observation.NativeEnds,
		ProviderTurns: observation.ProviderTurns, SameProcess: observation.SameProcess,
		SameSession: observation.SameSession, TerminalIdle: terminalIdle,
		Sandboxed: observation.Sandboxed,
	}
}

func buildWorkflowContextObserveAdmission(
	variant string,
	delivery promptlayer.ContextDeliveryResult,
	ephemeral promptlayer.OMPContextEphemeral,
) (string, error) {
	documents := make([]workflowContextManagedAdmissionDocument, 0, len(delivery.Layers))
	for _, layer := range delivery.Layers {
		if layer.SourceRef == "" || layer.Content == "" {
			return "", errors.New("observe-call canonical document is incomplete")
		}
		documents = append(documents, workflowContextManagedAdmissionDocument{SourceRef: layer.SourceRef, Body: layer.Content})
	}
	mode := WorkflowContextDispatchCanonicalFull
	if variant == "optimized" {
		mode = WorkflowContextDispatchOptimized
	}
	payload := workflowContextManagedAdmission{
		SchemaVersion: workflowContextManagedAdmissionSchemaVersion, Mode: mode,
		CanonicalPrompt: delivery.Prompt, Documents: documents,
		OriginalTask: ephemeral.OriginalTask, DecisionDelta: ephemeral.DecisionDelta,
		FrozenFindingIDs: []string{}, OwnershipPaths: []string{}, ForbiddenPaths: []string{},
		WorkerResultFields: promptlayer.OMPWorkerResultSchema(), DocumentOmissions: []string{}, MemoryInjections: []string{},
	}
	encoded, err := json.Marshal(payload)
	if err != nil || len(encoded) >= workflowContextManagedRPCMaxInputFrameBytes-4096 {
		return "", errors.New("observe-call canonical admission is unavailable")
	}
	return string(encoded), nil
}
