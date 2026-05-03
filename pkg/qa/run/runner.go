package run

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

func Execute(opts Options) (Result, error) {
	opts = normalizeOptions(opts)
	plan, err := BuildPlan(opts)
	if err != nil {
		return Result{}, err
	}
	if opts.FeedbackTo != "" && !validFeedbackTarget(opts.FeedbackTo) {
		return Result{}, fmt.Errorf("unsupported feedback target %q", opts.FeedbackTo)
	}
	if opts.DryRun {
		return dryRunResult(plan), nil
	}
	started := time.Now().UTC()
	runID := "qa-" + uuid.NewString()
	runDir := filepath.Join(opts.Output, runID)
	packs, err := executionPacks(opts, plan)
	if err != nil {
		return Result{}, err
	}
	result := initialResult(opts, plan, runID, runDir)
	for _, pack := range packs {
		adapterResult, manifestPath, check := executePack(opts, pack, filepath.Join(runDir, "_raw"), runDir)
		result.AdapterResults = append(result.AdapterResults, adapterResult)
		if adapterResult.SetupGap != nil {
			result.SetupGaps = append(result.SetupGaps, *adapterResult.SetupGap)
		}
		if manifestPath != "" {
			result.ManifestPaths = append(result.ManifestPaths, manifestPath)
		}
		result.Checks = append(result.Checks, check)
		if check.Status == "failed" || check.Status == "blocked" {
			result.FailedChecks = append(result.FailedChecks, check.ID)
		}
	}
	result.Status = aggregateStatus(result)
	if opts.FeedbackTo != "" {
		paths, err := writeFeedbackBundles(opts, result.ManifestPaths)
		if err != nil {
			return result, err
		}
		result.FeedbackBundlePaths = paths
		result.FeedbackAvailable = len(result.FeedbackBundlePaths) > 0
	}
	if err := writeIndex(result, opts, started, time.Now().UTC()); err != nil {
		return result, err
	}
	if result.Status == "failed" || result.Status == "blocked" {
		return result, fmt.Errorf("qa run %s", result.Status)
	}
	return result, nil
}

func validFeedbackTarget(target string) bool {
	switch target {
	case "codex", "claude", "gemini", "opencode":
		return true
	default:
		return false
	}
}

func dryRunResult(plan Plan) Result {
	return Result{
		Status:           "passed",
		DryRun:           true,
		SelectedJourneys: plan.SelectedJourneys,
		SelectedAdapters: plan.SelectedAdapters,
		OutputRoot:       plan.OutputRoot,
		ManifestPaths:    []string{},
		FailedChecks:     []string{},
		Checks:           []IndexCheck{},
		AdapterResults:   []AdapterResult{},
		SetupGaps:        plan.SetupGaps,
		RedactionStatus:  RedactionStatus{Status: "passed"},
	}
}

func initialResult(opts Options, plan Plan, runID, runDir string) Result {
	return Result{
		RunID:             runID,
		Status:            "passed",
		SelectedJourneys:  plan.SelectedJourneys,
		SelectedAdapters:  plan.SelectedAdapters,
		OutputRoot:        opts.Output,
		RunIndexPath:      filepath.Join(runDir, "run-index.json"),
		ManifestPaths:     []string{},
		FailedChecks:      []string{},
		Checks:            []IndexCheck{},
		AdapterResults:    []AdapterResult{},
		SetupGaps:         plan.SetupGaps,
		RedactionStatus:   RedactionStatus{Status: "passed"},
		FeedbackAvailable: false,
	}
}

func executionPacks(opts Options, plan Plan) ([]journey.Pack, error) {
	packs, err := selectedPacks(opts)
	if err != nil {
		return nil, err
	}
	if len(packs) > 0 {
		return packs, nil
	}
	if candidates := candidatePacks(plan); len(candidates) > 0 {
		return candidates, nil
	}
	return detectedPacks(plan), nil
}

func selectedPacks(opts Options) ([]journey.Pack, error) {
	packs, err := journey.LoadDir(opts.ProjectDir)
	if err != nil {
		return nil, err
	}
	out := make([]journey.Pack, 0, len(packs))
	for _, pack := range packs {
		if deferredPackReason(pack) != "" {
			continue
		}
		if includePack(pack, opts) {
			out = append(out, pack)
		}
	}
	return out, nil
}

func executePack(opts Options, pack journey.Pack, rawRoot, runDir string) (AdapterResult, string, IndexCheck) {
	check := IndexCheck{ID: firstCheckID(pack), JourneyID: pack.ID, Adapter: pack.Adapter.ID, Expected: "exit_code=0"}
	if gap := setupGapFor(opts, pack); gap != nil {
		check.Status = "skipped"
		return AdapterResult{Adapter: pack.Adapter.ID, JourneyID: pack.ID, Status: "skipped", SetupGap: gap}, "", check
	}
	cmdResult := runCommand(opts.ProjectDir, pack, filepath.Join(rawRoot, safeSegment(pack.ID)))
	applyOracle(opts.ProjectDir, pack, &cmdResult, &check)
	manifest := buildManifest(opts, pack, cmdResult, check)
	manifestPath, err := qaevidence.WriteFinalManifest(manifest, manifestOutputDir(runDir, pack.ID))
	if err != nil {
		check.Status = "blocked"
		check.FailureSummary = err.Error()
		return AdapterResult{
			Adapter:               pack.Adapter.ID,
			JourneyID:             pack.ID,
			Status:                "blocked",
			RepairPromptAvailable: false,
			FailureSummary:        err.Error(),
		}, "", check
	}
	return AdapterResult{
		Adapter:               pack.Adapter.ID,
		JourneyID:             pack.ID,
		Status:                cmdResult.Status,
		QAMESHManifestPath:    manifestPath,
		RepairPromptAvailable: cmdResult.Status == "failed" || cmdResult.Status == "blocked",
		FailureSummary:        cmdResult.FailureSummary,
	}, manifestPath, check
}

func writeFeedbackBundles(opts Options, manifestPaths []string) ([]string, error) {
	paths := []string{}
	for _, path := range manifestPaths {
		manifest, err := qaevidence.LoadManifest(path)
		if err != nil || manifest.Status != "failed" {
			continue
		}
		output := filepath.Join(filepath.Dir(filepath.Dir(path)), "feedback")
		result, err := qaevidence.WriteFeedbackBundle(manifest, opts.FeedbackTo, output)
		if err != nil {
			return paths, err
		}
		paths = append(paths, result.BundlePath)
	}
	return paths, nil
}
