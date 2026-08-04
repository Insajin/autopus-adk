package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	pipelineOMPContextObservationSchema   = "autopus.omp_context_observation.v1"
	pipelineOMPContextObservationMaxBytes = 4 << 20
	pipelineOMPContextObservationMaxRun   = 2 * time.Hour
)

var pipelineOMPContextObservationRunID = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)

func loadPipelineOMPContextObservation(path string) (pipelineOMPContextObservationV1, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		info.Size() <= 0 || info.Size() > pipelineOMPContextObservationMaxBytes {
		return pipelineOMPContextObservationV1{}, errors.New("OMP context observation manifest is unavailable")
	}
	body, err := os.ReadFile(path)
	if err != nil || len(body) > pipelineOMPContextObservationMaxBytes {
		return pipelineOMPContextObservationV1{}, errors.New("OMP context observation manifest is unavailable")
	}
	if err := rejectDuplicatePipelineOMPJSON(body); err != nil {
		return pipelineOMPContextObservationV1{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var result pipelineOMPContextObservationV1
	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("decode OMP context observation: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return result, errors.New("OMP context observation contains trailing JSON")
	}
	return result, validatePipelineOMPContextObservation(result)
}

func validatePipelineOMPContextObservation(value pipelineOMPContextObservationV1) error {
	if value.SchemaVersion != pipelineOMPContextObservationSchema ||
		!pipelineOMPContextObservationRunID.MatchString(value.RunID) || value.Attempt < 1 ||
		value.TrustLane == "" || value.ProducerRepository == "" || value.ProducerWorkflowRef == "" ||
		!validPipelineOMPActiveHash(value.EvidenceID) || !validPipelineOMPActiveHash(value.ChallengeDigest) ||
		!pipelineOMPContextCohortLocatorPattern.MatchString(value.CredentialLocator) ||
		len(value.Tasks) != 20 || len(value.Calls) != 40 {
		return errors.New("OMP context observation identity is invalid")
	}
	if err := validatePipelineOMPContextObservationCoordinates(value); err != nil {
		return err
	}
	ab, ba, seen, previous := 0, 0, map[string]bool{}, time.Time{}
	for index, task := range value.Tasks {
		if !validPipelineOMPActiveHash(task.TaskIDDigest) || seen[task.TaskIDDigest] ||
			(task.Order != "AB" && task.Order != "BA") {
			return errors.New("OMP context observation task is invalid")
		}
		seen[task.TaskIDDigest] = true
		if task.Order == "AB" {
			ab++
		} else {
			ba++
		}
		for pairIndex := 0; pairIndex < 2; pairIndex++ {
			call := value.Calls[index*2+pairIndex]
			wantVariant := string(task.Order[pairIndex])
			completed, err := validatePipelineOMPContextObservationCall(value, call, task.TaskIDDigest,
				wantVariant, index*2+pairIndex+1, pairIndex+1, previous)
			if err != nil {
				return err
			}
			previous = completed
		}
	}
	if ab != 10 || ba != 10 {
		return errors.New("OMP context observation order must be AB/BA 10:10")
	}
	first, _ := time.Parse(time.RFC3339Nano, value.Calls[0].StartedAt)
	if previous.Sub(first) > pipelineOMPContextObservationMaxRun {
		return errors.New("OMP context observation exceeded its time budget")
	}
	return nil
}

func validatePipelineOMPContextObservationCoordinates(value pipelineOMPContextObservationV1) error {
	stringsRequired := []string{
		value.Candidate.Repository, value.Candidate.Revision, value.Candidate.TreeSHA, value.Policy.PolicyID,
		value.Runtime.OMPVersion, value.Runtime.AutoVersion, value.CohortDigest, value.OrderSeed,
		value.Provider, value.Model, value.ProducerRepository, value.ProducerWorkflowRef,
	}
	for _, item := range stringsRequired {
		if strings.TrimSpace(item) == "" || strings.ContainsRune(item, 0) {
			return errors.New("OMP context observation coordinates are invalid")
		}
	}
	for _, hash := range []string{value.Candidate.ArtifactSHA256, value.Policy.PolicyDigest,
		value.Runtime.OMPExecutableSHA256, value.Runtime.AutoBinarySHA256, value.ModelScopeDigest,
		value.Runtime.PipelineImplementationDigest, value.CohortDigest, value.OrderSeed, value.OraclePolicyDigest} {
		if !validPipelineOMPActiveHash(hash) {
			return errors.New("OMP context observation coordinates are invalid")
		}
	}
	if !validPipelineOMPActiveGitHash(value.Candidate.Revision) || !validPipelineOMPActiveGitHash(value.Candidate.TreeSHA) ||
		value.Policy.HistoryMode != "active" || value.Policy.MemoryMode != "off" ||
		value.Policy.MinPairCount != 20 || value.Policy.MinReductionBasisPoints != 2000 ||
		value.Runtime.ExecutionClass != "external-live" || value.Runtime.RuntimeKind != "omp-pipeline-managed-rpc" ||
		!value.Runtime.ProductionPathEquivalent ||
		value.Runtime.PipelineImplementationDigest != pipelineOMPActiveImplementationDigest() {
		return errors.New("OMP context observation coordinates are invalid")
	}
	return nil
}

func validatePipelineOMPContextObservationCall(value pipelineOMPContextObservationV1,
	call pipelineOMPContextObservationCall, taskDigest, variant string, sequence, pairSequence int,
	previous time.Time,
) (time.Time, error) {
	started, startErr := time.Parse(time.RFC3339Nano, call.StartedAt)
	completed, completeErr := time.Parse(time.RFC3339Nano, call.CompletedAt)
	validTime := startErr == nil && completeErr == nil && started.UTC().Format(time.RFC3339Nano) == call.StartedAt &&
		completed.UTC().Format(time.RFC3339Nano) == call.CompletedAt && started.Before(completed) &&
		(previous.IsZero() || !started.Before(previous)) && completed.Sub(started) <= 10*time.Minute
	if !validTime || call.Sequence != sequence || call.PairSequence != pairSequence || call.TaskIDDigest != taskDigest ||
		call.Variant != variant || call.EndpointClass != "external-provider" || call.Transport != "provider-api" ||
		call.CredentialMode != "locator-only" || call.ExecutionMode != "external-live" || call.Provider != value.Provider ||
		call.Model != value.Model || call.ModelScopeDigest != value.ModelScopeDigest ||
		call.RetryCount != 0 || call.MaxConcurrency != 1 || !validPipelineOMPContextObservationFacts(call) {
		return time.Time{}, fmt.Errorf("OMP context observation call %s is invalid", strconv.Itoa(sequence))
	}
	for _, digest := range []string{call.TaskDigest, call.OutputDigest, call.OracleDigest} {
		if !validPipelineOMPActiveHash(digest) {
			return time.Time{}, fmt.Errorf("OMP context observation call %d digest is invalid", sequence)
		}
	}
	return completed, nil
}

func validPipelineOMPContextObservationFacts(call pipelineOMPContextObservationCall) bool {
	usage := call.TokenUsage.PrimaryInputTokens > 0 && call.TokenUsage.PrimaryOutputTokens > 0 &&
		call.TokenUsage.MaintenanceInputTokens >= 0 && call.TokenUsage.MaintenanceOutputTokens >= 0 &&
		call.TokenUsage.TotalTokens == call.TokenUsage.PrimaryInputTokens+call.TokenUsage.PrimaryOutputTokens+
			call.TokenUsage.MaintenanceInputTokens+call.TokenUsage.MaintenanceOutputTokens
	integrity := call.IntegrityFacts
	maintenanceCountsValid := integrity.SetupProviderRequests >= 0 && integrity.CompactionProviderCalls >= 0
	integrityValid := integrity.RequiredDocumentRefs > 0 &&
		integrity.MatchedDocumentRefs == integrity.RequiredDocumentRefs &&
		integrity.RequiredEphemeralFields > 0 && integrity.MatchedEphemeralFields == integrity.RequiredEphemeralFields &&
		integrity.HashMismatches == 0 && integrity.DocumentOmissions == 0 && integrity.MemoryInjections == 0 &&
		integrity.PrimaryProviderRequests == 1 && maintenanceCountsValid &&
		integrity.TotalProviderRequests == integrity.SetupProviderRequests+
			integrity.PrimaryProviderRequests+integrity.CompactionProviderCalls &&
		integrity.UnexpectedProviderCalls == 0
	security := call.SecurityFacts
	securityValid := security.AuthorizedGatewayRequests == integrity.TotalProviderRequests &&
		security.UnexpectedLoopbackRequests == 0 && security.ChildCredentialExposures == 0 &&
		security.ChildEndpointExposures == 0 && security.RawBodyPersistedFields == 0 &&
		security.SecretMatches == 0 && security.AbsolutePathLeaks == 0 &&
		security.PromptInjectionExecutions == 0 && security.SecurityFindings == 0
	quality := call.QualityFacts.Assertions > 0 && call.QualityFacts.PassedAssertions == call.QualityFacts.Assertions
	fallback := call.FallbackFacts.Trials > 0 &&
		call.FallbackFacts.CanonicalFullRestarts == call.FallbackFacts.Trials &&
		call.FallbackFacts.ExactMatches == call.FallbackFacts.Trials &&
		call.FallbackFacts.OptimizedProviderCallsAfterMismatch == 0
	rollback := call.RollbackFacts.Trials > 0 &&
		call.RollbackFacts.EffectiveShadowReadbacks == call.RollbackFacts.Trials &&
		call.RollbackFacts.ReadbackMismatches == 0 && call.RollbackFacts.InheritedOptimizedSessions == 0
	cleanup := call.CleanupFacts.OwnedRootsCreated > 0 &&
		call.CleanupFacts.OwnedRootsRemoved == call.CleanupFacts.OwnedRootsCreated &&
		call.CleanupFacts.OwnedRootsRemain == 0 && call.CleanupFacts.UserRootsAccessed == 0 &&
		call.CleanupFacts.PathEscapeEvents == 0 && call.CleanupFacts.ModeViolations == 0
	return usage && integrityValid && securityValid && quality && fallback && rollback && cleanup
}
