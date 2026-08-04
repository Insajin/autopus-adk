package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/insajin/autopus-adk/pkg/version"
)

const workflowContextObserveSessionMaxLine = 1 << 20

func RunWorkflowContextObserveSession(
	ctx context.Context,
	input io.Reader,
	output io.Writer,
	options workflowContextObserveSessionOptions,
) (runErr error) {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64<<10), workflowContextObserveSessionMaxLine)
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	first, err := nextWorkflowContextObserveSessionCommand(scanner)
	if err != nil || !validWorkflowContextObserveHandshake(first) {
		return errors.New("observe-session handshake is invalid")
	}
	setup, err := prepareWorkflowContextObserveSession(ctx, options, first.ChallengeDigest)
	if err != nil {
		return err
	}
	defer func() { runErr = errors.Join(runErr, setup.close()) }()
	if err := setup.start(ctx); err != nil {
		return err
	}
	fullPID, optimizedPID := setup.full.PID(), setup.optimized.PID()
	if fullPID <= 0 || optimizedPID <= 0 || fullPID == optimizedPID {
		return errors.New("observe-session persistent process pair is invalid")
	}
	handshake := workflowContextObserveSessionBaseResponse("handshake", setup.candidate.ModelScopeDigest)
	handshake.OMPVersion, handshake.OMPExecutableSHA256 = setup.ompVersion, setup.ompExecutableSHA256
	if err := encoder.Encode(handshake); err != nil {
		return err
	}
	seenTasks := make(map[string]struct{}, 20)
	var pairTask, pairPromptHash string
	for sequence := 1; sequence <= 40; sequence++ {
		command, err := nextWorkflowContextObserveSessionCommand(scanner)
		if err != nil || validateWorkflowContextObserveSessionCall(command, sequence) != nil {
			return fmt.Errorf("observe-session call %d is invalid", sequence)
		}
		if command.PairSequence == 1 {
			if _, duplicate := seenTasks[command.TaskIDDigest]; duplicate {
				return fmt.Errorf("observe-session call %d repeats a task", sequence)
			}
			seenTasks[command.TaskIDDigest] = struct{}{}
			pairTask, pairPromptHash = command.TaskIDDigest, workflowContextRuntimeHash(command.Prompt)
		} else if command.TaskIDDigest != pairTask || workflowContextRuntimeHash(command.Prompt) != pairPromptHash {
			return fmt.Errorf("observe-session call %d pair authority changed", sequence)
		}
		session, expectedPID := setup.full, fullPID
		if command.Variant == "B" {
			session, expectedPID = setup.optimized, optimizedPID
		}
		assistant, receipt, err := session.Execute(ctx, command.Prompt)
		if err != nil {
			return fmt.Errorf("observe-session call %d failed closed: %w", sequence, err)
		}
		response := workflowContextObserveSessionBaseResponse("call", setup.candidate.ModelScopeDigest)
		response.Sequence, response.PairSequence = command.Sequence, command.PairSequence
		response.TaskIDDigest, response.Variant = command.TaskIDDigest, command.Variant
		response.AssistantText, response.OutputDigest = assistant, workflowContextRuntimeHash(assistant)
		response.SessionDigest = workflowContextRuntimeHash(receipt.SessionID)
		response.ProcessReused = session.PID() == expectedPID && receipt.SameProcess && receipt.SameSession
		response.CompactionCycles = receipt.CompactionCycles
		response.Usage = &workflowContextObserveSessionUsage{
			PrimaryInputTokens: receipt.InputTokens, PrimaryOutputTokens: receipt.OutputTokens,
			MaintenanceInputTokens:  receipt.MaintenanceInputTokens,
			MaintenanceOutputTokens: receipt.MaintenanceOutputTokens, TotalTokens: receipt.TotalTokens,
		}
		if !response.ProcessReused || (command.Variant == "A" && response.CompactionCycles != 0) {
			return fmt.Errorf("observe-session call %d process lifecycle changed", sequence)
		}
		if err := encoder.Encode(response); err != nil {
			return err
		}
	}
	if len(seenTasks) != 20 {
		return errors.New("observe-session task cardinality is invalid")
	}
	shutdown, err := nextWorkflowContextObserveSessionCommand(scanner)
	if err != nil || !validWorkflowContextObserveShutdown(shutdown) {
		return errors.New("observe-session shutdown is invalid")
	}
	if scanner.Scan() || scanner.Err() != nil {
		return errors.New("observe-session input continued after shutdown")
	}
	if err := setup.close(); err != nil {
		return err
	}
	setup.taskRoot = ""
	response := workflowContextObserveSessionBaseResponse("shutdown", setup.candidate.ModelScopeDigest)
	response.CallsCompleted = 40
	return encoder.Encode(response)
}

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
