package release

import (
	"fmt"
	"os"
	"path/filepath"

	qarun "github.com/insajin/autopus-adk/pkg/qa/run"
)

type defaultLaneRunner struct{}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-QAMESH-004: release gate execution aggregates deterministic lane rows into the persisted release index.
// @AX:REASON: CLI callers and tests depend on fail-closed blocking, skipped-after-block behavior, redaction merging, and AI advisory refs in one contract.
// @AX:WARN [AUTO] @AX:SPEC: SPEC-QAMESH-004: release execution has 8+ branch points across preflight, lane selection, redaction, and failure status.
// @AX:REASON: Fail-closed ordering, block short-circuiting, and sanitized-payload persistence can regress if branches are reordered.
func Execute(opts Options) (ExecutionPayload, error) {
	opts = normalizeOptions(opts)
	plan, err := BuildPlan(opts)
	if err != nil {
		return ExecutionPayload{}, err
	}
	runner := opts.Runner
	if runner == nil {
		runner = defaultLaneRunner{}
	}
	started := opts.Now()
	releaseID := opts.NewID()
	index := Index{
		SchemaVersion:          IndexSchemaVersion,
		ReleaseID:              releaseID,
		Workspace:              workspaceRef(opts.ProjectDir),
		Profile:                opts.Profile,
		StartedAt:              started.Format(timeFormat),
		SelectedLanes:          ReleaseLanes(),
		OutputPaths:            executionOutputPaths(opts, releaseID),
		SiblingSpecs:           SiblingSpecs(),
		DeterministicAuthority: true,
		RedactionStatus:        plan.RedactionStatus,
		Blockers:               []Blocker{},
		FeedbackRefs:           []string{},
		AIAnalysisRefs:         []AIAnalysisRef{},
		SourceRefs:             planSourceRefs(opts.ProjectDir, plan),
	}
	policy, _ := profilePolicy(opts.Profile)
	policy = adaptPolicyToProject(opts.ProjectDir, policy, plan.JourneyPacks)
	gaps := gapsByLane(plan.SetupGaps)
	blocked := false
	for _, lane := range ReleaseLanes() {
		var row LaneRow
		var aiRefs []AIAnalysisRef
		laneRedaction := RedactionClean
		if blocked {
			row = skippedAfterBlockRow(policy, lane)
		} else if gap, ok := gaps[lane]; ok {
			row = setupGapLaneRow(policy, gap)
		} else if lanePolicy(policy, lane) == LanePolicyDeferred {
			row = deferredLaneRow(policy, lane)
		} else {
			result, runErr := runner.RunLane(opts, lane)
			aiRefs = result.AIAnalysisRefs
			laneRedaction = result.RedactionStatus
			row = runResultLaneRow(policy, lane, result, runErr)
		}
		row = NormalizeLaneRow(row)
		index.LaneRows = append(index.LaneRows, row)
		index.Blockers = append(index.Blockers, row.Blockers...)
		index.FeedbackRefs = append(index.FeedbackRefs, row.FeedbackRefs...)
		index.AIAnalysisRefs = append(index.AIAnalysisRefs, trustedFalseRefs(aiRefs)...)
		index.RedactionStatus = mergeRedaction(index.RedactionStatus, laneRedaction)
		if row.LaneVerdict == LaneVerdictBlock {
			blocked = true
		}
	}
	index.SetupGaps = plan.SetupGaps
	index.Status = AggregateGateStatus(index.LaneRows)
	index.EndedAt = opts.Now().Format(timeFormat)
	index.RedactionStatus = mergeRedaction(index.RedactionStatus)
	actualIndexPath := index.OutputPaths.ReleaseIndexPath
	sanitizedIndex, redactionState := sanitizeIndex(index)
	sanitizedIndex.RedactionStatus = mergeRedaction(sanitizedIndex.RedactionStatus, redactionState)
	releaseIndexPath, pathChanged := redactReleaseString(actualIndexPath)
	if pathChanged {
		sanitizedIndex.RedactionStatus = mergeRedaction(sanitizedIndex.RedactionStatus, RedactionRedacted)
	}
	payload := ExecutionPayload{Index: sanitizedIndex, ReleaseIndexPath: releaseIndexPath}
	if err := writeIndex(payload.Index, actualIndexPath); err != nil {
		return payload, err
	}
	if payload.Status == GateStatusBlocked {
		return payload, ErrReleaseBlocked
	}
	return payload, nil
}

const timeFormat = "2006-01-02T15:04:05.999999999Z07:00"

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-004: default release execution delegates lanes to QAMESH run and emits codex feedback bundle refs.
func (defaultLaneRunner) RunLane(opts Options, lane string) (LaneRunResult, error) {
	result, err := qarun.Execute(qarunOptionsForLane(opts, lane))
	if result.RunID != "" {
		_ = os.RemoveAll(filepath.Join(opts.RunOutputRoot, result.RunID, "_raw"))
	}
	failedJourneyID, failureSummary := firstRunFailure(result)
	return LaneRunResult{
		Status:          mapRunStatus(result.Status),
		RunIndexPath:    result.RunIndexPath,
		ManifestPaths:   result.ManifestPaths,
		FeedbackRefs:    result.FeedbackBundlePaths,
		FailedJourneyID: failedJourneyID,
		FailureSummary:  failureSummary,
		RedactionStatus: mapRunRedaction(result.RedactionStatus.Status),
	}, err
}

func qarunOptionsForLane(opts Options, lane string) qarun.Options {
	return qarun.Options{
		ProjectDir:      opts.ProjectDir,
		Profile:         opts.Profile,
		Lane:            lane,
		Output:          opts.RunOutputRoot,
		FeedbackTo:      "codex",
		RuntimeProvider: opts.RuntimeProvider,
	}
}

func executionOutputPaths(opts Options, releaseID string) OutputPaths {
	return OutputPaths{
		ReleaseIndexPath: filepath.Join(opts.Output, releaseID, "release-index.json"),
		RunIndexRoot:     opts.RunOutputRoot,
		EvidenceRoot:     filepath.Join(opts.ProjectDir, ".autopus", "qa", "evidence"),
		FeedbackRoot:     filepath.Join(opts.ProjectDir, ".autopus", "qa", "feedback"),
	}
}

func setupGapLaneRow(policy ProfilePolicy, gap SetupGapRow) LaneRow {
	status := LaneStatusSetupGap
	if lanePolicy(policy, gap.Lane) == LanePolicyDeferred {
		status = LaneStatusDeferred
	}
	catalog := laneByID(gap.Lane)
	return LaneRow{
		Lane:                   gap.Lane,
		LanePolicy:             lanePolicy(policy, gap.Lane),
		OwnerSpec:              catalog.OwnerSpec,
		OwnerRepo:              catalog.OwnerRepo,
		Status:                 status,
		SetupGapClass:          gap.SetupGapClass,
		Severity:               gap.Severity,
		ManifestPaths:          []string{},
		FeedbackRefs:           []string{},
		DeterministicAuthority: true,
	}
}

func deferredLaneRow(policy ProfilePolicy, lane string) LaneRow {
	catalog := laneByID(lane)
	return LaneRow{
		Lane:                   lane,
		LanePolicy:             lanePolicy(policy, lane),
		OwnerSpec:              catalog.OwnerSpec,
		OwnerRepo:              catalog.OwnerRepo,
		Status:                 LaneStatusDeferred,
		SetupGapClass:          SetupGapNone,
		Severity:               SeverityNone,
		ManifestPaths:          []string{},
		FeedbackRefs:           []string{},
		DeterministicAuthority: true,
	}
}

func skippedAfterBlockRow(policy ProfilePolicy, lane string) LaneRow {
	row := deferredLaneRow(policy, lane)
	row.Status = LaneStatusSkipped
	row.SkippedReason = "not_started_after_block"
	return row
}

func runResultLaneRow(policy ProfilePolicy, lane string, result LaneRunResult, err error) LaneRow {
	catalog := laneByID(lane)
	status := result.Status
	if status == "" {
		status = LaneStatusBlocked
	}
	if result.RedactionStatus == RedactionBlocked {
		status = LaneStatusBlocked
	}
	row := LaneRow{
		Lane:                   lane,
		LanePolicy:             lanePolicy(policy, lane),
		OwnerSpec:              catalog.OwnerSpec,
		OwnerRepo:              catalog.OwnerRepo,
		Status:                 status,
		SetupGapClass:          SetupGapNone,
		Severity:               SeverityNone,
		RunIndexPath:           result.RunIndexPath,
		ManifestPaths:          result.ManifestPaths,
		FeedbackRefs:           result.FeedbackRefs,
		FailedJourneyID:        result.FailedJourneyID,
		FailureSummary:         result.FailureSummary,
		DeterministicAuthority: true,
	}
	if err != nil && status != LaneStatusFailed && status != LaneStatusBlocked {
		row.Status = LaneStatusBlocked
		row.Blockers = []Blocker{{Lane: lane, Reason: fmt.Sprintf("lane_runner_error: %s", err.Error())}}
	}
	return row
}

func gapsByLane(gaps []SetupGapRow) map[string]SetupGapRow {
	out := map[string]SetupGapRow{}
	for _, gap := range gaps {
		out[gap.Lane] = gap
	}
	return out
}

func mapRunStatus(status string) LaneStatus {
	switch status {
	case "warning":
		return LaneStatusWarn
	case "passed", "failed", "blocked":
		return LaneStatus(status)
	default:
		return LaneStatusBlocked
	}
}

func mapRunRedaction(status string) RedactionState {
	if status == "blocked" {
		return RedactionBlocked
	}
	return RedactionClean
}

func trustedFalseRefs(refs []AIAnalysisRef) []AIAnalysisRef {
	out := make([]AIAnalysisRef, 0, len(refs))
	for _, ref := range refs {
		ref.TrustedForVerdict = false
		out = append(out, ref)
	}
	return out
}
