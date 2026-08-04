package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
)

func validateWorkflowContextManagedNativeEnd(frame workflowContextManagedRPCFrame) error {
	result := bytes.TrimSpace(frame.Result)
	if frame.Type != "auto_compaction_end" || frame.Action != "snapcompact" ||
		frame.Aborted || frame.Skipped || frame.ErrorMessage != "" ||
		len(result) == 0 || bytes.Equal(result, []byte("null")) {
		return errors.New("managed OMP native compaction end is invalid")
	}
	return nil
}

func (protocol *workflowContextManagedRPCProtocol) awaitNativeCompactionEnd(ctx context.Context) error {
	for {
		frame, err := protocol.next(ctx)
		if err != nil {
			return fmt.Errorf("await managed OMP native compaction end: %w", err)
		}
		if frame.Type == "extension_error" || frame.Type == "extension_ui_request" {
			return errors.New("managed OMP emitted unexpected activity before native completion")
		}
		if frame.Type != "auto_compaction_end" {
			continue
		}
		return validateWorkflowContextManagedNativeEnd(frame)
	}
}

// @AX:WARN [AUTO]: manual compaction completion has eight fail-closed if branches.
// @AX:REASON [AUTO]: command response and native completion must each occur exactly once without intervening extension or UI activity.
func (protocol *workflowContextManagedRPCProtocol) awaitManualCompactionCompletion(
	ctx context.Context, id string,
) error {
	response, err := protocol.next(ctx)
	if err != nil {
		return fmt.Errorf("await managed OMP manual compact response: %w", err)
	}
	if response.Type == "extension_error" {
		return errors.New("managed OMP extension failed during manual compaction")
	}
	if response.Type == "extension_ui_request" {
		return errors.New("managed OMP emitted unexpected UI activity during manual compaction")
	}
	if response.Type != "response" || response.ID != id || response.Command != "compact" ||
		response.Success == nil || !*response.Success {
		return errors.New("managed OMP manual compaction completion is invalid")
	}
	nativeEnd, err := protocol.next(ctx)
	if err != nil {
		return fmt.Errorf("await managed OMP manual native completion: %w", err)
	}
	if nativeEnd.Type == "extension_error" {
		return errors.New("managed OMP extension failed during manual compaction")
	}
	if nativeEnd.Type == "extension_ui_request" {
		return errors.New("managed OMP emitted unexpected UI activity during manual compaction")
	}
	if nativeEnd.Type == "response" {
		return errors.New("managed OMP manual compaction response was duplicated")
	}
	return validateWorkflowContextManagedNativeEnd(nativeEnd)
}
