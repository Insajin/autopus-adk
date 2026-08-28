package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
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
	errorModelScope := ""
	errorStage := "handshake"
	errorSequence := 0
	defer func() {
		if runErr == nil {
			return
		}
		response := workflowContextObserveSessionBaseResponse("error", errorModelScope)
		response.ErrorCode = workflowContextObserveSessionErrorCode(runErr)
		response.ErrorStage = errorStage
		response.FailedSequence = errorSequence
		if err := encoder.Encode(response); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("observe-session error response: %w", err))
		}
	}()
	first, err := nextWorkflowContextObserveSessionCommand(scanner)
	if err != nil || !validWorkflowContextObserveHandshake(first) {
		return errors.New("observe-session handshake is invalid")
	}
	errorStage = "setup"
	setup, err := prepareWorkflowContextObserveSessionForRun(ctx, options, first.ChallengeDigest)
	if err != nil {
		return err
	}
	errorModelScope = setup.candidate.ModelScopeDigest
	defer func() { runErr = errors.Join(runErr, setup.close()) }()
	errorStage = "startup"
	if err := setup.start(ctx); err != nil {
		return err
	}
	fullPID, optimizedPID := setup.full.PID(), setup.optimized.PID()
	if fullPID <= 0 || optimizedPID <= 0 || fullPID == optimizedPID {
		return errors.New("observe-session persistent process pair is invalid")
	}
	handshake := workflowContextObserveSessionBaseResponse("handshake", setup.candidate.ModelScopeDigest)
	handshake.OMPVersion, handshake.OMPExecutableSHA256 = setup.ompVersion, setup.ompExecutableSHA256
	handshake.ProviderAuthorityDigest = setup.providerAuthorityDigest
	if err := encoder.Encode(handshake); err != nil {
		return err
	}
	errorStage = "call"
	seenTasks := make(map[string]struct{}, workflowContextObserveSessionPairCount)
	variantCalls := map[string]int{"A": 0, "B": 0}
	segmentVariantCalls := map[string]int{"A": 0, "B": 0}
	sessionBindings := map[string]string{"A": "", "B": ""}
	allSessionBindings := make(map[string]struct{}, workflowContextObserveSessionSegmentCount*2)
	compactionCycles, preCompactionACKs := 0, 0
	postCompactionACKs, canonicalReadmissions, ephemeralReadmissions := 0, 0, 0
	providerAuthority := setup.providerAuthorityDigest
	if !validPipelineOMPActiveHash(providerAuthority) {
		return errors.New("observe-session provider authority binding is unstable")
	}
	calls := make([]workflowContextObserveSessionCallEvidence, 0, workflowContextObserveSessionPairCount*2)
	var pairTask, pairPromptHash, pairOutputDigest string
	var lastCompleted time.Time
	for index := range workflowContextObserveSessionPairCount * 2 {
		sequence := index + 1
		errorSequence = sequence
		command, err := nextWorkflowContextObserveSessionCommand(scanner)
		if err != nil || validateWorkflowContextObserveSessionCall(command, sequence) != nil {
			return fmt.Errorf("observe-session call %d is invalid", sequence)
		}
		if command.PairSequence == 1 && len(seenTasks) > 0 &&
			len(seenTasks)%workflowContextObserveSessionSegmentPairs == 0 {
			if err := setup.rotate(ctx); err != nil {
				return fmt.Errorf("observe-session call %d segment rotation failed: %w", sequence, err)
			}
			fullPID, optimizedPID = setup.full.PID(), setup.optimized.PID()
			if fullPID <= 0 || optimizedPID <= 0 || fullPID == optimizedPID {
				return errors.New("observe-session persistent process pair is invalid")
			}
			segmentVariantCalls = map[string]int{"A": 0, "B": 0}
			sessionBindings = map[string]string{"A": "", "B": ""}
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
		providerPrompt, providerPromptHash, err := setup.sealPrompt(command.Prompt)
		if err != nil {
			return fmt.Errorf("observe-session call %d canonical admission failed: %w", sequence, err)
		}
		startedAt := time.Now().UTC()
		if !lastCompleted.IsZero() && !startedAt.After(lastCompleted) {
			startedAt = lastCompleted.Add(time.Nanosecond)
		}
		assistant, receipt, err := session.Execute(ctx, providerPrompt)
		endedAt := time.Now().UTC()
		if !endedAt.After(startedAt) {
			endedAt = startedAt.Add(time.Nanosecond)
		}
		if err != nil {
			return fmt.Errorf("observe-session call %d failed closed: %w", sequence, err)
		}
		if !safeWorkflowContextObserveSessionOutput(setup, assistant) {
			return fmt.Errorf("observe-session call %d returned private or unsafe output", sequence)
		}
		response := workflowContextObserveSessionBaseResponse("call", setup.candidate.ModelScopeDigest)
		response.Sequence, response.PairSequence = command.Sequence, command.PairSequence
		response.TaskIDDigest, response.Variant = command.TaskIDDigest, command.Variant
		response.AssistantText, response.OutputDigest = assistant, workflowContextRuntimeHash(assistant)
		if command.PairSequence == 1 {
			pairOutputDigest = response.OutputDigest
		} else if response.OutputDigest != pairOutputDigest {
			return fmt.Errorf("observe-session call %d quality oracle changed across the pair", sequence)
		}
		response.SessionDigest = receipt.SessionBindingHash
		response.ProviderAuthorityDigest = providerAuthority
		response.ProcessReused = segmentVariantCalls[command.Variant] > 0
		response.CompactionCycles = receipt.CompactionCycles
		response.PreCompactionACKs = receipt.PreCompactionACKs
		response.PostCompactionACKs = receipt.PostCompactionACKs
		response.CanonicalReadmissions = receipt.CanonicalReadmissions
		response.EphemeralReadmissions = receipt.EphemeralReadmissions
		response.Usage = &workflowContextObserveSessionUsage{
			PrimaryInputTokens: receipt.InputTokens, PrimaryOutputTokens: receipt.OutputTokens,
			MaintenanceInputTokens:  receipt.MaintenanceInputTokens,
			MaintenanceOutputTokens: receipt.MaintenanceOutputTokens, TotalTokens: receipt.TotalTokens,
		}
		lifecycleValid := session.PID() == expectedPID && receipt.SameProcess && receipt.SameSession &&
			receipt.TerminalIdle && receipt.SessionBindingHash != "" &&
			receipt.BridgeBindingHash == session.binding.BindingHash
		if prior := sessionBindings[command.Variant]; prior == "" {
			if _, duplicate := allSessionBindings[receipt.SessionBindingHash]; duplicate {
				lifecycleValid = false
			} else {
				sessionBindings[command.Variant] = receipt.SessionBindingHash
				allSessionBindings[receipt.SessionBindingHash] = struct{}{}
			}
		} else if prior != receipt.SessionBindingHash {
			lifecycleValid = false
		}
		expectedCompactions := response.CompactionCycles
		freshOptimized := command.Variant == "B" && segmentVariantCalls["B"] == 0
		variantCalls[command.Variant]++
		segmentVariantCalls[command.Variant]++
		fullValid := command.Variant == "A" && expectedCompactions == 0 &&
			response.PreCompactionACKs == 0 && response.PostCompactionACKs == 0 &&
			response.CanonicalReadmissions == 0 && response.EphemeralReadmissions == 0
		optimizedValid := command.Variant == "B" && expectedCompactions >= 0 && expectedCompactions <= 1 &&
			(!freshOptimized || expectedCompactions == 0) &&
			response.PreCompactionACKs == expectedCompactions &&
			response.PostCompactionACKs == expectedCompactions &&
			response.CanonicalReadmissions == expectedCompactions &&
			response.EphemeralReadmissions == expectedCompactions
		if !lifecycleValid || !fullValid && !optimizedValid {
			return fmt.Errorf("observe-session call %d process lifecycle changed", sequence)
		}
		compactionCycles += response.CompactionCycles
		preCompactionACKs += response.PreCompactionACKs
		postCompactionACKs += response.PostCompactionACKs
		canonicalReadmissions += response.CanonicalReadmissions
		ephemeralReadmissions += response.EphemeralReadmissions
		calls = append(calls, workflowContextObserveSessionCallEvidence{
			command: command, response: response, providerPromptHash: providerPromptHash,
			startedAt: startedAt, endedAt: endedAt,
		})
		lastCompleted = endedAt
		if command.PairSequence == 2 {
			if err := verifyWorkflowContextObserveSessionReadback(
				ctx, &setup, variantCalls["A"], variantCalls["B"],
			); err != nil {
				return fmt.Errorf("observe-session pair %d failed readback: %w", (sequence+1)/2, err)
			}
		}
		if err := encoder.Encode(response); err != nil {
			return err
		}
	}
	errorSequence = 0
	errorStage = "cohort"
	if len(seenTasks) != workflowContextObserveSessionPairCount ||
		variantCalls["A"] != workflowContextObserveSessionPairCount ||
		variantCalls["B"] != workflowContextObserveSessionPairCount ||
		setup.segmentsStarted != workflowContextObserveSessionSegmentCount ||
		len(allSessionBindings) != workflowContextObserveSessionSegmentCount*2 ||
		compactionCycles < 2 || sessionBindings["A"] == "" || sessionBindings["B"] == "" ||
		sessionBindings["A"] == sessionBindings["B"] {
		return errors.New("observe-session task or reusable-session cardinality is invalid")
	}
	errorStage = "shutdown"
	shutdown, err := nextWorkflowContextObserveSessionCommand(scanner)
	if err != nil || !validWorkflowContextObserveShutdown(shutdown) {
		return errors.New("observe-session shutdown is invalid")
	}
	if scanner.Scan() || scanner.Err() != nil {
		return errors.New("observe-session input continued after shutdown")
	}
	errorStage = "cleanup"
	evidenceSetup := setup
	if err := setup.close(); err != nil {
		return err
	}
	setup.taskRoot = ""
	errorStage = "evidence"
	checkedAt := time.Now().UTC()
	if !checkedAt.After(lastCompleted) {
		checkedAt = lastCompleted.Add(time.Nanosecond)
	}
	evidenceID, reportDigest, err := writeWorkflowContextObserveSessionEvidence(
		options, evidenceSetup, first.ChallengeDigest, calls, checkedAt,
	)
	if err != nil {
		return err
	}
	errorStage = "response"
	response := workflowContextObserveSessionBaseResponse("shutdown", setup.candidate.ModelScopeDigest)
	response.CallsCompleted = 40
	response.ProviderAuthorityDigest = providerAuthority
	response.EvidenceID, response.ReportDigest = evidenceID, reportDigest
	response.CleanupVerified = true
	response.CompactionCycles = compactionCycles
	response.PreCompactionACKs = preCompactionACKs
	response.PostCompactionACKs = postCompactionACKs
	response.CanonicalReadmissions = canonicalReadmissions
	response.EphemeralReadmissions = ephemeralReadmissions
	return encoder.Encode(response)
}
