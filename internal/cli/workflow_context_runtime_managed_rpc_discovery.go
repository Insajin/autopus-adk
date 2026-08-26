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
	const requestID = "managed-product-commands"
	if err := protocol.send(map[string]any{
		"id": requestID, "type": "get_available_commands",
	}); err != nil {
		return err
	}
	ready, responseAccepted := false, false
	discovered := map[string]bool{}
	for !ready || !responseAccepted || !discovered[want[0]] || !discovered[want[1]] {
		frame, nextErr := protocol.next(ctx)
		if nextErr != nil {
			return fmt.Errorf("await managed OMP product discovery: %w", nextErr)
		}
		switch {
		case frame.Type == "extension_error":
			return errors.New("managed OMP extension failed during product startup")
		case frame.Type == "ready":
			ready = true
		case frame.Type == "available_commands_update":
			recordWorkflowContextManagedCommands(discovered, frame.Commands)
		case frame.Type == "response" && frame.ID == requestID:
			if frame.Success == nil || !*frame.Success || frame.Command != "get_available_commands" {
				return errors.New("managed OMP product command discovery failed")
			}
			var payload struct {
				Commands []json.RawMessage `json:"commands"`
			}
			if json.Unmarshal(frame.Data, &payload) != nil {
				return errors.New("managed OMP product command discovery response is invalid")
			}
			recordWorkflowContextManagedCommands(discovered, payload.Commands)
			responseAccepted = true
		}
	}
	return nil
}

func recordWorkflowContextManagedCommands(discovered map[string]bool, commands []json.RawMessage) {
	for _, raw := range commands {
		var name string
		if json.Unmarshal(raw, &name) != nil {
			var command struct {
				Name string `json:"name"`
			}
			_ = json.Unmarshal(raw, &command)
			name = command.Name
		}
		if name != "" {
			discovered[name] = true
		}
	}
}
