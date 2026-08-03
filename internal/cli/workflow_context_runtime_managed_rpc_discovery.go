package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

func (protocol *workflowContextManagedRPCProtocol) awaitProductReady(
	ctx context.Context, firstPrompt string,
) error {
	want, err := workflowContextManagedProductCommandNames(firstPrompt)
	if err != nil {
		return err
	}
	ready := false
	discovered := map[string]bool{}
	for !ready || !discovered[want[0]] || !discovered[want[1]] {
		frame, nextErr := protocol.next(ctx)
		if nextErr != nil {
			return fmt.Errorf("await managed OMP product discovery: %w", nextErr)
		}
		if frame.Type == "extension_error" {
			return errors.New("managed OMP extension failed during product startup")
		}
		if frame.Type == "ready" {
			ready = true
		}
		if frame.Type == "available_commands_update" {
			for _, raw := range frame.Commands {
				var name string
				if json.Unmarshal(raw, &name) != nil {
					var command struct {
						Name string `json:"name"`
					}
					_ = json.Unmarshal(raw, &command)
					name = command.Name
				}
				discovered[name] = true
			}
		}
	}
	return nil
}
