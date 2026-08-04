package cli

import (
	"context"
	"errors"
	"fmt"
)

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: shared provider lifecycle verifier for admission and multi-cycle triggering.
// @AX:REASON [AUTO]: three production callers require identical prompt, lifecycle, UI, and optional next-compaction ordering.
// @AX:WARN [AUTO]: provider-boundary verification has cyclomatic complexity 47 and 14 fail-closed if branches.
// @AX:REASON [AUTO]: prompt acceptance, provider lifecycle, and one optional next-cycle start must be unique and ordered before admission.
func (protocol *workflowContextManagedRPCProtocol) awaitProviderBoundaryState(
	ctx context.Context, id string, allowNextCompaction bool,
) (bool, error) {
	accepted, started, turned, ended, nextStarted := false, false, false, false, false
	for !(accepted && started && turned && ended) {
		frame, err := protocol.next(ctx)
		if err != nil {
			return false, fmt.Errorf("await managed OMP provider boundary: %w", err)
		}
		if frame.Type == "extension_error" {
			return false, errors.New("managed OMP extension activity interrupted provider admission")
		}
		if frame.Type == "extension_ui_request" {
			if workflowContextManagedProductNotification(frame.Method) {
				continue
			}
			if frame.Method == "confirm" && allowNextCompaction && accepted && started && turned && !ended {
				if err := protocol.putBack(frame); err != nil {
					return false, err
				}
				return true, nil
			}
			return false, fmt.Errorf("managed OMP UI activity interrupted provider admission: %s", frame.Method)
		}
		if frame.Type == "auto_compaction_end" {
			return false, errors.New("managed OMP native completion arrived during provider admission")
		}
		if frame.Type == "auto_compaction_start" {
			if !allowNextCompaction || nextStarted || !accepted || !started || !turned || ended ||
				frame.Reason != "threshold" || frame.Action != "snapcompact" {
				return false, errors.New("managed OMP next compaction start is out of order")
			}
			nextStarted = true
			continue
		}
		if frame.Type == "response" && frame.ID == id {
			if accepted || started || turned || ended || frame.Success == nil || !*frame.Success || frame.Command != "prompt" {
				return false, errors.New("managed OMP admission prompt was rejected")
			}
			accepted = true
		}
		switch frame.Type {
		case "agent_start":
			if !accepted || started || turned || ended {
				return false, errors.New("managed OMP admission agent start is out of order")
			}
			started = true
		case "turn_end":
			if !started || turned || ended {
				return false, errors.New("managed OMP admission turn end is out of order")
			}
			turned = true
		case "agent_end":
			if !turned || ended || frame.IsTerminal != nil && !*frame.IsTerminal {
				return false, errors.New("managed OMP admission agent end is out of order")
			}
			ended = true
		}
	}
	return nextStarted, nil
}
