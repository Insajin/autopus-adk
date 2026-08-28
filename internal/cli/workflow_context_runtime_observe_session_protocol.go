package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/insajin/autopus-adk/pkg/version"
)

func workflowContextObserveSessionBaseResponse(
	typeName string,
	modelScope string,
) workflowContextObserveSessionResponse {
	return workflowContextObserveSessionResponse{
		SchemaVersion: workflowContextObserveSessionResponseSchema, Type: typeName,
		ExecutionClass: "external-live", RuntimeKind: "omp-pipeline-managed-rpc",
		ProductionPathEquivalent: true, ImplementationDigest: pipelineOMPActiveImplementationDigest(),
		ModelScopeDigest: modelScope, SourceCommit: version.SourceCommit(), SourceTree: version.SourceTree(),
	}
}

func nextWorkflowContextObserveSessionCommand(
	scanner *bufio.Scanner,
) (workflowContextObserveSessionCommand, error) {
	if !scanner.Scan() {
		if scanner.Err() != nil {
			return workflowContextObserveSessionCommand{}, scanner.Err()
		}
		return workflowContextObserveSessionCommand{}, io.EOF
	}
	body := bytes.TrimSpace(scanner.Bytes())
	if len(body) == 0 || len(body) > workflowContextObserveSessionMaxLine || body[0] != '{' ||
		rejectDuplicatePipelineOMPJSON(body) != nil {
		return workflowContextObserveSessionCommand{}, errors.New("invalid observe-session JSONL frame")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var command workflowContextObserveSessionCommand
	if err := decoder.Decode(&command); err != nil {
		return command, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return command, errors.New("observe-session frame contains trailing JSON")
	}
	return command, nil
}

func validWorkflowContextObserveHandshake(command workflowContextObserveSessionCommand) bool {
	return command.SchemaVersion == workflowContextObserveSessionCommandSchema && command.Type == "handshake" &&
		validPipelineOMPActiveHash(command.ChallengeDigest) && command.Sequence == 0 && command.PairSequence == 0 &&
		command.TaskIDDigest == "" && command.Variant == "" && command.Prompt == ""
}

func validateWorkflowContextObserveSessionCall(command workflowContextObserveSessionCommand, sequence int) error {
	wantPair := 1 + (sequence-1)%2
	taskIndex := (sequence - 1) / 2
	wantVariant := "A"
	if taskIndex%2 == 0 && wantPair == 2 || taskIndex%2 == 1 && wantPair == 1 {
		wantVariant = "B"
	}
	if command.SchemaVersion != workflowContextObserveSessionCommandSchema || command.Type != "call" ||
		command.ChallengeDigest != "" || command.Sequence != sequence || command.PairSequence != wantPair ||
		command.Variant != wantVariant || !validPipelineOMPActiveHash(command.TaskIDDigest) ||
		strings.TrimSpace(command.Prompt) == "" || len(command.Prompt) > workflowContextObserveSessionMaxLine-4096 ||
		strings.ContainsRune(command.Prompt, 0) || strings.HasPrefix(strings.TrimSpace(command.Prompt), "/auto") ||
		strings.Contains(command.Prompt, "AUTOPUS_PROVIDER_PHASE=") {
		return errors.New("invalid call")
	}
	return nil
}

func validWorkflowContextObserveShutdown(command workflowContextObserveSessionCommand) bool {
	return command.SchemaVersion == workflowContextObserveSessionCommandSchema && command.Type == "shutdown" &&
		command.ChallengeDigest == "" && command.Sequence == 0 && command.PairSequence == 0 &&
		command.TaskIDDigest == "" && command.Variant == "" && command.Prompt == ""
}
