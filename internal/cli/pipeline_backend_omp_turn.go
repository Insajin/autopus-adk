package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// pipelineOMPTurnIdentity is the provider/model the session reports for the
// assistant message that settled the turn; empty when the runtime omitted it.
type pipelineOMPTurnIdentity struct {
	Provider string
	Model    string
}

// pipelineOMPTurnError is a turn that the provider ended with an error instead
// of a reply. Status carries the upstream HTTP status when the runtime reported
// one, so callers can separate transient overload from permanent rejection.
type pipelineOMPTurnError struct {
	Status  int
	Message string
}

func (e *pipelineOMPTurnError) Error() string {
	if e.Status > 0 {
		return fmt.Sprintf("OMP pipeline turn failed with provider error status %d: %s", e.Status, e.Message)
	}
	return "OMP pipeline turn failed with provider error: " + e.Message
}

// Transient reports whether the upstream status is worth one more attempt.
func (e *pipelineOMPTurnError) Transient() bool {
	switch e.Status {
	case 408, 409, 425, 429, 500, 502, 503, 504, 529:
		return true
	}
	return false
}

const pipelineOMPTurnErrorPreview = 240

// settlePipelineOMPTurn inspects the agent_end message list. A missing list is
// accepted for older runtimes; a present list must end in an assistant message
// that stopped normally, otherwise the turn is a failure rather than "empty".
func settlePipelineOMPTurn(raw json.RawMessage) (pipelineOMPTurnIdentity, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return pipelineOMPTurnIdentity{}, nil
	}
	var messages []struct {
		Role         string `json:"role"`
		Provider     string `json:"provider"`
		Model        string `json:"model"`
		StopReason   string `json:"stopReason"`
		ErrorStatus  int    `json:"errorStatus"`
		ErrorMessage string `json:"errorMessage"`
	}
	if err := json.Unmarshal(raw, &messages); err != nil {
		return pipelineOMPTurnIdentity{}, errors.New("OMP pipeline RPC agent_end messages are malformed")
	}
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if message.Role != "assistant" {
			continue
		}
		identity := pipelineOMPTurnIdentity{Provider: message.Provider, Model: message.Model}
		switch message.StopReason {
		case "error":
			preview := strings.TrimSpace(message.ErrorMessage)
			if preview == "" {
				preview = "provider reported no error message"
			}
			if runes := []rune(preview); len(runes) > pipelineOMPTurnErrorPreview {
				// The whole preview, ellipsis included, stays within the cap and
				// never splits a multi-byte character.
				preview = string(runes[:pipelineOMPTurnErrorPreview-3]) + "..."
			}
			return identity, &pipelineOMPTurnError{Status: message.ErrorStatus, Message: preview}
		case "aborted":
			return identity, errors.New("OMP pipeline turn was aborted before the assistant replied")
		}
		return identity, nil
	}
	return pipelineOMPTurnIdentity{}, nil
}
