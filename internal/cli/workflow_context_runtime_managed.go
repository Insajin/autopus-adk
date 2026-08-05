package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

const (
	workflowContextBridgeSchemaVersion      = "autopus.omp-context-bridge.v1"
	workflowContextDispatchAckSchemaVersion = "autopus.omp-context-dispatch-ack.v1"
)

// RunManaged admits optimization only through one bound, supervisor-owned driver.
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: managed admission binds one nonce-scoped driver before delegating to the runtime state machine.
// @AX:REASON [AUTO]: the installed canary and managed-runtime callers depend on binding, dispatch acknowledgement, and cleanup remaining one ordered contract.
func (supervisor *WorkflowContextRuntimeSupervisor) RunManaged(
	ctx context.Context,
	request WorkflowContextRuntimeRequest,
) (WorkflowContextRuntimeReceipt, error) {
	receipt := newWorkflowContextRuntimeReceipt(request)
	driver, ok := request.Driver.(WorkflowContextManagedProcessDriver)
	if !ok || driver == nil {
		return finishManagedWorkflowContextBlock(request, receipt, "managed-driver-unavailable",
			fmt.Errorf("managed OMP context driver is required"))
	}
	if request.CanonicalSource == nil {
		return finishManagedWorkflowContextPreflight(request, receipt, driver, "canonical-source-unavailable",
			fmt.Errorf("authoritative OMP canonical source is required"))
	}
	if err := validateWorkflowContextRuntimeMetadata(request); err != nil {
		return finishManagedWorkflowContextPreflight(request, receipt, driver, "runtime-metadata-invalid", err)
	}
	preview, err := promptlayer.BuildOMPContextBinding(request.Binding)
	if err != nil {
		return finishManagedWorkflowContextPreflight(request, receipt, driver, "runtime-binding-invalid", err)
	}
	nonceHash, err := newWorkflowContextRunNonceHash()
	if err != nil {
		return finishManagedWorkflowContextPreflight(request, receipt, driver, "runtime-nonce-unavailable", err)
	}
	bound := &boundWorkflowContextManagedDriver{
		WorkflowContextManagedProcessDriver: driver,
		binding: WorkflowContextBridgeBinding{
			SchemaVersion: workflowContextBridgeSchemaVersion,
			BindingHash:   preview.BindingHash,
			OptionsHash:   preview.OptionsHash,
			SessionHash:   workflowContextRuntimeHash(request.Binding.SessionID),
			NonceHash:     nonceHash,
		},
	}
	request.Driver = bound
	return supervisor.Run(ctx, request, func(ctx context.Context, dispatch WorkflowContextDispatch) error {
		ack, dispatchErr := driver.Dispatch(ctx, dispatch)
		if dispatchErr != nil {
			return fmt.Errorf("dispatch managed OMP context: %w", dispatchErr)
		}
		if err := validateWorkflowContextDispatchAck(bound.binding, ack); err != nil {
			return err
		}
		if request.ProviderOutput != nil && ack.providerOutput != "" {
			if err := request.ProviderOutput(ack.providerOutput); err != nil {
				return fmt.Errorf("capture managed OMP provider output: %w", err)
			}
		}
		if request.ProviderUsage != nil && ack.providerUsage.TotalTokens > 0 {
			if err := request.ProviderUsage(ack.providerUsage); err != nil {
				return fmt.Errorf("capture managed OMP provider usage: %w", err)
			}
		}
		return nil
	})
}

type boundWorkflowContextManagedDriver struct {
	WorkflowContextManagedProcessDriver
	binding WorkflowContextBridgeBinding
}
type workflowContextManagedLifecycleObserver interface {
	Observation() WorkflowContextManagedRPCObservation
}

func validateWorkflowContextManagedLifecycle(
	driver WorkflowContextProcessDriver,
	requiredCycles int,
) (WorkflowContextLifecycleReceipt, error) {
	bound, ok := driver.(*boundWorkflowContextManagedDriver)
	if !ok || bound == nil {
		return WorkflowContextLifecycleReceipt{}, errors.New("managed OMP lifecycle observer is unavailable")
	}
	observer, ok := bound.WorkflowContextManagedProcessDriver.(workflowContextManagedLifecycleObserver)
	if !ok {
		return WorkflowContextLifecycleReceipt{}, errors.New("managed OMP lifecycle observer is unavailable")
	}
	observation := observer.Observation()
	receipt := WorkflowContextLifecycleReceipt{
		RequiredCompactionCycles: requiredCycles,
		PreCompactionACKs:        observation.PreACKs, PostCompactionACKs: observation.PostACKs,
		NativeStarts: observation.NativeStarts, NativeEnds: observation.NativeEnds,
		CanonicalReadmissions: observation.CanonicalReadmissions,
		EphemeralReadmissions: observation.EphemeralReadmissions, ProviderTurns: observation.ProviderTurns,
		ProviderAuthorityDigest: observation.ProviderAuthorityDigest,
		SameProcess:             observation.SameProcess, SameSession: observation.SameSession,
		ProviderObserved: observation.ProviderObserved,
	}
	if requiredCycles < 1 || observation.PreACKs != requiredCycles || observation.PostACKs != requiredCycles ||
		observation.NativeStarts != requiredCycles || observation.NativeEnds != requiredCycles ||
		observation.CanonicalReadmissions != requiredCycles ||
		observation.EphemeralReadmissions != requiredCycles || !observation.SameProcess ||
		!observation.SameSession || !observation.Sandboxed || !observation.ProviderObserved ||
		!validPipelineOMPActiveHash(observation.ProviderAuthorityDigest) {
		return receipt, errors.New("managed OMP correlated multi-compaction lifecycle evidence is incomplete")
	}
	return receipt, nil
}

func (driver *boundWorkflowContextManagedDriver) Run(
	ctx context.Context,
	emit func(WorkflowContextRuntimeEvent) error,
) error {
	if err := driver.Bind(ctx, driver.binding); err != nil {
		return fmt.Errorf("bind OMP context bridge: %w", err)
	}
	return driver.WorkflowContextManagedProcessDriver.Run(ctx, emit)
}

func (supervisor *WorkflowContextRuntimeSupervisor) rehydrateWorkflowContextRuntime(
	ctx context.Context,
	request WorkflowContextRuntimeRequest,
	binding promptlayer.OMPContextBindingReceipt,
	dispatch WorkflowContextDispatchFunc,
) (promptlayer.OMPContextTerminalReceipt, error) {
	if request.CanonicalSource == nil {
		return supervisor.store.Rehydrate(binding.BindingHash, request.Binding.DeliveryOptions,
			func(view promptlayer.OMPContextTransientView) error {
				return dispatchWorkflowContextRuntime(ctx, dispatch, request.Binding.Delivery, view)
			})
	}
	delivery, ephemeral, err := request.CanonicalSource.Rebuild(ctx, request.Binding.DeliveryOptions)
	if err != nil {
		return promptlayer.OMPContextTerminalReceipt{}, fmt.Errorf("rebuild authoritative OMP context: %w", err)
	}
	if err := promptlayer.VerifyContextDeliveryForOptions(request.Binding.DeliveryOptions, delivery); err != nil {
		return promptlayer.OMPContextTerminalReceipt{}, fmt.Errorf("verify authoritative OMP context: %w", err)
	}
	freshInput := request.Binding
	freshInput.Delivery = delivery
	freshInput.Ephemeral = ephemeral
	freshBinding, err := promptlayer.BuildOMPContextBinding(freshInput)
	if err != nil {
		return promptlayer.OMPContextTerminalReceipt{}, fmt.Errorf("bind authoritative OMP context: %w", err)
	}
	return supervisor.store.RehydrateCanonical(binding.BindingHash, request.Binding.DeliveryOptions, delivery,
		func(view promptlayer.OMPContextTransientView) error {
			if freshBinding.BindingHash != binding.BindingHash || freshBinding.OptionsHash != binding.OptionsHash {
				return fmt.Errorf("authoritative OMP binding changed across compaction")
			}
			return dispatchWorkflowContextRuntime(ctx, dispatch, delivery, view)
		})
}

func dispatchWorkflowContextRuntime(
	ctx context.Context,
	dispatch WorkflowContextDispatchFunc,
	delivery promptlayer.ContextDeliveryResult,
	view promptlayer.OMPContextTransientView,
) error {
	if dispatch == nil {
		return fmt.Errorf("OMP context provider dispatch callback is required")
	}
	return dispatch(ctx, WorkflowContextDispatch{
		Mode: WorkflowContextDispatchOptimized, Delivery: delivery, Transient: view,
	})
}

func finishManagedWorkflowContextBlock(
	request WorkflowContextRuntimeRequest,
	receipt WorkflowContextRuntimeReceipt,
	reason string,
	err error,
) (WorkflowContextRuntimeReceipt, error) {
	receipt.Outcome = WorkflowContextOutcomeBlocked
	receipt.Fallback.Mode = config.OMPContextFallbackBlock
	receipt.Fallback.Reason = reason
	return finishWorkflowContextRuntime(request, receipt, err)
}

func finishManagedWorkflowContextPreflight(
	request WorkflowContextRuntimeRequest,
	receipt WorkflowContextRuntimeReceipt,
	driver WorkflowContextManagedProcessDriver,
	reason string,
	primary error,
) (WorkflowContextRuntimeReceipt, error) {
	receipt.Outcome = WorkflowContextOutcomeBlocked
	receipt.Fallback.Mode = config.OMPContextFallbackBlock
	receipt.Fallback.Reason = reason
	maintenance, cancel := workflowContextMaintenanceContext()
	defer cancel()
	cleanupErr := cleanupWorkflowContextRuntime(maintenance, driver, &receipt)
	return finishWorkflowContextRuntime(request, receipt, errors.Join(primary, cleanupErr))
}

func newWorkflowContextRunNonceHash() (string, error) {
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("generate OMP context run nonce: %w", err)
	}
	return workflowContextRuntimeHash(string(nonce[:])), nil
}

func validateWorkflowContextDispatchAck(expected WorkflowContextBridgeBinding, actual WorkflowContextDispatchAck) error {
	if actual.SchemaVersion != workflowContextDispatchAckSchemaVersion || !actual.ProviderObserved ||
		!workflowContextSecureEqual(expected.BindingHash, actual.BindingHash) ||
		!workflowContextSecureEqual(expected.OptionsHash, actual.OptionsHash) ||
		!workflowContextSecureEqual(expected.SessionHash, actual.SessionHash) ||
		!workflowContextSecureEqual(expected.NonceHash, actual.NonceHash) {
		return fmt.Errorf("managed OMP dispatch acknowledgement is invalid")
	}
	return nil
}

func workflowContextSecureEqual(expected, actual string) bool {
	return len(expected) == len(actual) && subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) == 1
}

func workflowContextRuntimeHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}
