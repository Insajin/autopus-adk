package cli

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

const workflowContextMaintenanceTimeout = 5 * time.Second

var workflowContextMetadataPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/+:-]{0,127}$`)

func validateWorkflowContextRuntimeMetadata(request WorkflowContextRuntimeRequest) error {
	values := []struct{ name, value string }{
		{"profile", request.Policy.Profile}, {"history_mode", request.Policy.HistoryMode},
		{"memory_mode", request.Policy.MemoryMode}, {"fallback", request.Policy.Fallback},
		{"capability_policy", request.Policy.CapabilityPolicy}, {"runtime_root_policy", request.Policy.RuntimeRootPolicy},
		{"mutation_scope", request.Policy.MutationScope}, {"root_class", request.RootClass},
		{"workspace_id", request.Binding.WorkspaceID}, {"spec_id", request.Binding.SpecID},
		{"task_id", request.Binding.TaskID}, {"phase", request.Binding.Phase}, {"session_id", request.Binding.SessionID},
		{"runtime_version", request.Capabilities.Version}, {"probe_source", request.Capabilities.ProbeSource},
	}
	for _, item := range values {
		if !workflowContextMetadataPattern.MatchString(item.value) || strings.Contains(strings.ToLower(item.value), "body") ||
			workflowContextSecretPattern.MatchString(item.value) {
			return fmt.Errorf("invalid OMP context runtime metadata: %s", item.name)
		}
	}
	return nil
}

func workflowContextMaintenanceContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), workflowContextMaintenanceTimeout)
}

func cleanupUntrustedWorkflowContextRuntime(request WorkflowContextRuntimeRequest) error {
	maintenance, cancel := workflowContextMaintenanceContext()
	defer cancel()
	var result error
	if request.Driver != nil {
		if err := request.Driver.Cleanup(maintenance); err != nil {
			result = errors.Join(result, fmt.Errorf("cleanup rejected OMP context request: %w", err))
		}
	}
	if request.Overlay != nil && validWorkflowContextRollbackMemoryMode(request.Policy.MemoryMode) {
		_, err := ApplyWorkflowContextOverlay(maintenance, request.Overlay, WorkflowContextOverlayRequest{
			HistoryMode: config.OMPContextHistoryShadow, MemoryMode: request.Policy.MemoryMode, Reason: "runtime-metadata-invalid",
		})
		result = errors.Join(result, err)
	}
	return result
}

func (supervisor *WorkflowContextRuntimeSupervisor) failWorkflowContextRuntime(
	request WorkflowContextRuntimeRequest,
	receipt WorkflowContextRuntimeReceipt,
	bindingHash string,
	primary error,
) (WorkflowContextRuntimeReceipt, error) {
	maintenance, cancel := workflowContextMaintenanceContext()
	defer cancel()
	var abortErr, cleanupErr, rollbackErr error
	if bindingHash != "" {
		_, abortErr = supervisor.store.Abort(bindingHash, "runtime-terminal-error")
		if errors.Is(abortErr, promptlayer.ErrOMPContextBindingUnavailable) {
			abortErr = nil
		}
	}
	if request.Driver != nil && !receipt.Cleanup.Attempted {
		cleanupErr = cleanupWorkflowContextRuntime(maintenance, request.Driver, &receipt)
	}
	if request.Overlay != nil && validWorkflowContextRollbackMemoryMode(request.Policy.MemoryMode) {
		rollbackErr = rollbackWorkflowContextOverlay(maintenance, request, &receipt)
	}
	if cleanupErr != nil && receipt.Fallback.Reason == "" {
		receipt.Fallback.Reason = "runtime-cleanup-failed"
	} else if rollbackErr != nil && receipt.Fallback.Reason == "" {
		receipt.Fallback.Reason = "rollback-readback-mismatch"
	}
	return finishWorkflowContextRuntime(request, receipt, errors.Join(primary, abortErr, cleanupErr, rollbackErr))
}

func validWorkflowContextRollbackMemoryMode(mode string) bool {
	return mode == config.OMPContextMemoryOff || mode == config.OMPContextMemoryShadow
}
