package cli

import (
	"context"
	"fmt"
	"regexp"
)

var workflowContextHashPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func ApplyWorkflowContextOverlay(
	ctx context.Context,
	controller WorkflowContextOverlayController,
	request WorkflowContextOverlayRequest,
) (WorkflowContextOverlayReadback, error) {
	if controller == nil {
		return WorkflowContextOverlayReadback{}, fmt.Errorf("OMP context overlay controller is required")
	}
	readback, err := controller.Apply(ctx, request)
	if err != nil {
		return readback, fmt.Errorf("apply OMP context overlay: %w", err)
	}
	if readback.RequestedHistoryMode != request.HistoryMode || readback.EffectiveHistoryMode != request.HistoryMode {
		return readback, fmt.Errorf("OMP context overlay effective mode mismatch")
	}
	if readback.EffectiveMemoryMode != request.MemoryMode {
		return readback, fmt.Errorf("OMP context overlay changed independent memory mode")
	}
	if !workflowContextHashPattern.MatchString(readback.OverlayHash) ||
		readback.OverlayHash != readback.ReadbackHash {
		return readback, fmt.Errorf("OMP context overlay readback hash mismatch")
	}
	return readback, nil
}
