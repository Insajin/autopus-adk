package skillevolve

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type qameshRunIndex struct {
	Status        string           `json:"status"`
	ManifestPaths []string         `json:"manifest_paths"`
	Checks        []qameshRunCheck `json:"checks"`
}

type qameshRunCheck struct {
	ID      string `json:"id"`
	Adapter string `json:"adapter"`
	Status  string `json:"status"`
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-SKILL-EVOLVE-001: deterministic QAMESH replay is the promotion-readiness authority.
// @AX:REASON: LLM scores stay advisory; only passed deterministic must-check evidence can move a candidate toward promotion.
func ReplayCandidate(ctx context.Context, opts ReplayOptions) (ReplayResult, error) {
	if err := ctx.Err(); err != nil {
		return ReplayResult{}, err
	}
	result := ReplayResult{
		LLMScore: normalizeLLMScore(opts.Candidate.LLMScore),
	}
	safety, err := EvaluateSafety(ctx, opts.Candidate, SafetyOptions{})
	if err != nil {
		return ReplayResult{}, err
	}
	if !safety.ReplayAllowed {
		result.FailureReasons = appendFailureReasons(result.FailureReasons, safety.ReasonCodes)
		return result, nil
	}
	if len(opts.Candidate.ReplayPlan.MustChecks) == 0 {
		result.FailureReasons = append(result.FailureReasons, "deterministic_must_checks_missing")
		return result, nil
	}
	if opts.Candidate.ReplayPlan.RunIndexPath == "" {
		result.FailureReasons = append(result.FailureReasons, "replay_run_index_missing")
		return result, nil
	}

	runIndexPath := resolveWorkspacePath(opts.ProjectDir, opts.Candidate.ReplayPlan.RunIndexPath)
	if !pathWithinProjectForRead(opts.ProjectDir, runIndexPath) {
		result.FailureReasons = append(result.FailureReasons, "replay_run_index_outside_project")
		return result, nil
	}
	runIndex, err := readRunIndex(runIndexPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			result.FailureReasons = append(result.FailureReasons, "replay_run_index_missing")
			return result, nil
		}
		return ReplayResult{}, err
	}
	result.FailureReasons = appendFailureReasons(result.FailureReasons, replayEvidenceBindingFailures(opts.ProjectDir, opts.Candidate, runIndexPath))
	if runIndex.Status != "passed" {
		result.FailureReasons = appendReason(result.FailureReasons, "replay_run_index_not_passed:"+statusOrUnknown(runIndex.Status))
	}
	expectedCommand := expectedReplayCommand(opts.Candidate)
	if expectedCommand == "" {
		result.FailureReasons = appendReason(result.FailureReasons, "replay_command_missing")
	}
	result.Evidence.Commands, result.FailureReasons = replayCommandEvidence(
		runIndexPath,
		runIndex,
		opts.ProjectDir,
		expectedCommand,
		result.FailureReasons,
	)
	checks, checkFailures := evaluateMustChecks(opts.Candidate.ReplayPlan.MustChecks, runIndex)
	result.Evidence.Checks = checks
	result.FailureReasons = appendFailureReasons(result.FailureReasons, checkFailures)
	if !hasReplayAcceptanceMapping(opts.Candidate) {
		result.FailureReasons = appendReason(result.FailureReasons, "missing_acceptance_mapping")
	}
	result.PromotionReady = len(result.FailureReasons) == 0
	if result.PromotionReady && opts.Candidate.BundlePath != "" {
		updated := opts.Candidate
		updated.Status = "promotion_ready"
		updated.PromotionReady = true
		updated.ReplayEvidenceRefs = appendUnique(updated.ReplayEvidenceRefs, opts.Candidate.ReplayPlan.RunIndexPath)
		updated.ReplayEvidenceRefs = appendUniqueMany(updated.ReplayEvidenceRefs, runIndex.ManifestPaths)
		if err := writeCandidateBundlePath(updated.BundlePath, updated); err != nil {
			return ReplayResult{}, err
		}
	}
	return result, nil
}

func readRunIndex(path string) (qameshRunIndex, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return qameshRunIndex{}, err
	}
	var runIndex qameshRunIndex
	if err := json.Unmarshal(body, &runIndex); err != nil {
		return qameshRunIndex{}, err
	}
	return runIndex, nil
}

func evaluateMustChecks(mustChecks []ReplayCheckRef, runIndex qameshRunIndex) ([]ReplayCheckEvidence, []string) {
	checksByID := make(map[string]qameshRunCheck, len(runIndex.Checks))
	for _, check := range runIndex.Checks {
		checksByID[check.ID] = check
	}
	evidence := make([]ReplayCheckEvidence, 0, len(mustChecks))
	var failures []string
	for _, must := range mustChecks {
		check, found := checksByID[must.ID]
		status := "missing"
		if found {
			status = check.Status
		}
		deterministic := found && check.Status == "passed" && isDeterministicAdapter(check.Adapter)
		evidence = append(evidence, ReplayCheckEvidence{
			ID:            must.ID,
			Status:        status,
			Deterministic: deterministic,
			AcceptanceRef: must.AcceptanceRef,
			Source:        must.Source,
		})
		if !deterministic {
			failures = append(failures, "deterministic_must_check_failed:"+must.ID)
		}
	}
	return evidence, failures
}

func isDeterministicAdapter(adapter string) bool {
	switch adapter {
	case "go-test", "unit-test", "qamesh", "":
		return true
	default:
		return false
	}
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-SKILL-EVOLVE-001: LLM confidence is normalized to advisory so it cannot satisfy promotion readiness alone.
func normalizeLLMScore(score LLMScore) LLMScore {
	score.Advisory = true
	score.Authority = "advisory"
	return score
}

func resolveWorkspacePath(projectDir, rel string) string {
	if projectDir == "" || filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(projectDir, filepath.FromSlash(rel))
}

func writeCandidateBundlePath(path string, candidate CandidateBundle) error {
	body, err := json.MarshalIndent(candidate, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o644)
}
