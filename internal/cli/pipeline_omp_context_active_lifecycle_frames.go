package cli

// Frame-shape admission for the managed active lifecycle. These validators live
// beside the lifecycle loop that calls them so both files stay inside the
// source-size limit; the package and the behaviour are unchanged.

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

func validPipelineOMPActivePromptResponseData(data json.RawMessage) bool {
	body := bytes.TrimSpace(data)
	if len(body) == 0 || bytes.Equal(body, []byte("null")) {
		return true
	}
	if rejectDuplicatePipelineOMPJSON(body) != nil {
		return false
	}
	var exact map[string]json.RawMessage
	if json.Unmarshal(body, &exact) != nil || len(exact) != 1 {
		return false
	}
	var invoked bool
	value, ok := exact["agentInvoked"]
	return ok && json.Unmarshal(value, &invoked) == nil && invoked
}

func (protocol *pipelineOMPRPCProtocol) confirmPipelineOMPActiveBridge(id string) error {
	confirmed := true
	return protocol.process.send(pipelineOMPRPCCommand{
		ID: id, Type: "extension_ui_response", Confirmed: &confirmed,
	})
}

func validPipelineOMPActiveManualResult(data json.RawMessage) bool {
	var result struct {
		Summary string `json:"summary"`
	}
	return json.Unmarshal(data, &result) == nil && strings.TrimSpace(result.Summary) != ""
}

// pipelineOMPActiveNativeEndShape checks a native completion frame apart from
// the method it names. The method itself is decided by
// pipelineOMPActiveProbe.validNativeEnd, which admits snapcompact everywhere
// and remote only while a probe is watching.
func pipelineOMPActiveNativeEndShape(frame pipelineOMPRPCFrame) bool {
	result := bytes.TrimSpace(frame.Result)
	return frame.Type == "auto_compaction_end" && !frame.Aborted && !frame.Skipped &&
		frame.ErrorMessage == "" && len(result) > 0 && !bytes.Equal(result, []byte("null"))
}

func validatePipelineOMPActiveBridgeFrame(
	frame pipelineOMPRPCFrame,
	binding WorkflowContextBridgeBinding,
) (string, error) {
	if frame.Type != "extension_ui_request" || frame.Method != "confirm" || frame.ID == "" {
		return "", errors.New("managed active OMP emitted unsupported UI activity")
	}
	var message string
	var envelope workflowContextManagedBridgeEnvelope
	var exact map[string]any
	if json.Unmarshal(frame.Message, &message) != nil || json.Unmarshal([]byte(message), &envelope) != nil ||
		json.Unmarshal([]byte(message), &exact) != nil || len(exact) != 6 ||
		envelope.SchemaVersion != binding.SchemaVersion || frame.Title != "Autopus context "+envelope.Event ||
		!workflowContextSecureEqual(envelope.BindingHash, binding.BindingHash) ||
		!workflowContextSecureEqual(envelope.OptionsHash, binding.OptionsHash) ||
		!workflowContextSecureEqual(envelope.SessionHash, binding.SessionHash) ||
		!workflowContextSecureEqual(envelope.NonceHash, binding.NonceHash) ||
		(envelope.Event != WorkflowContextEventPreCompaction && envelope.Event != WorkflowContextEventPostCompaction) {
		return "", errors.New("managed active OMP bridge authority mismatch")
	}
	return envelope.Event, nil
}
