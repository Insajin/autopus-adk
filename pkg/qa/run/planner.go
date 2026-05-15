package run

import (
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/qa/adapter"
	qacompile "github.com/insajin/autopus-adk/pkg/qa/compile"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

func BuildPlan(opts Options) (Plan, error) {
	opts = normalizeOptions(opts)
	if err := validateOutputRoot(opts.ProjectDir, opts.Output); err != nil {
		return Plan{}, err
	}
	packs, err := journey.LoadDir(opts.ProjectDir)
	if err != nil {
		return Plan{}, err
	}
	detections := adapter.Detect(opts.ProjectDir)
	candidates := qacompile.FromProject(opts.ProjectDir)
	plan := Plan{
		SelectedLane:               opts.Lane,
		HarnessContract:            harnessContract(opts.ProjectDir),
		OutputRoot:                 opts.Output,
		RunIndexPreviewPath:        filepath.Join(opts.Output, "<run-id>", "run-index.json"),
		ManifestOutputPreviewPaths: []string{},
		SetupGaps:                  []SetupGap{},
		Deferred:                   []SetupGap{},
		AdapterMetadata:            adapter.WithSetupGaps(),
		CandidateJourneys:          candidatePayloads(candidates),
		ArtifactPreviewRefs:        []ArtifactPreview{},
	}
	for _, pack := range packs {
		plan.ConfiguredJourneys = append(plan.ConfiguredJourneys, pack.ID)
		if reason := deferredPackReason(pack); reason != "" {
			if includePack(pack, opts) {
				plan.Deferred = append(plan.Deferred, SetupGap{Adapter: pack.Adapter.ID, JourneyID: pack.ID, Reason: reason})
			}
			continue
		}
		if includePack(pack, opts) {
			plan.SelectedJourneys = append(plan.SelectedJourneys, pack.ID)
			plan.SelectedAdapters = appendUnique(plan.SelectedAdapters, pack.Adapter.ID)
			plan.ManifestOutputPreviewPaths = append(plan.ManifestOutputPreviewPaths, filepath.Join(opts.Output, "<run-id>", safeSegment(pack.ID), "manifest.json"))
			plan.ArtifactPreviewRefs = append(plan.ArtifactPreviewRefs, artifactPreviewsForPack(pack)...)
		}
	}
	for _, candidate := range candidates {
		if reason := deferredCandidateReason(candidate); reason != "" {
			if candidateMatchesFilters(candidate, opts) {
				plan.Deferred = append(plan.Deferred, SetupGap{Adapter: candidate.Adapter, JourneyID: candidate.JourneyID, Reason: reason})
			}
			continue
		}
		if !includeCandidate(candidate, opts) {
			continue
		}
		plan.SelectedJourneys = appendUnique(plan.SelectedJourneys, candidate.JourneyID)
		plan.SelectedAdapters = appendUnique(plan.SelectedAdapters, candidate.Adapter)
		plan.ManifestOutputPreviewPaths = append(plan.ManifestOutputPreviewPaths, filepath.Join(opts.Output, "<run-id>", safeSegment(candidate.JourneyID), "manifest.json"))
		plan.ArtifactPreviewRefs = append(plan.ArtifactPreviewRefs, artifactPreviewsForCandidate(candidate)...)
	}
	for _, detection := range detections {
		plan.DetectedAdapters = append(plan.DetectedAdapters, detection.AdapterID)
		if detection.SetupGapReason != "" {
			plan.SetupGaps = append(plan.SetupGaps, SetupGap{Adapter: detection.AdapterID, Reason: detection.SetupGapReason})
		}
	}
	applyMobileReadiness(&plan, opts)
	plan.ProjectHints = append(plan.ProjectHints, projectLocalJourneyHints(opts, packs)...)
	plan.SetupGaps = append(plan.SetupGaps, projectLocalJourneySetupGaps(opts, packs)...)
	if len(plan.SelectedJourneys) == 0 && opts.JourneyID == "" && allowDetectedFallback(opts) {
		for _, detection := range detections {
			if opts.AdapterID != "" && opts.AdapterID != detection.AdapterID {
				continue
			}
			id := "detected-" + detection.AdapterID
			plan.SelectedJourneys = append(plan.SelectedJourneys, id)
			plan.SelectedAdapters = appendUnique(plan.SelectedAdapters, detection.AdapterID)
			plan.ManifestOutputPreviewPaths = append(plan.ManifestOutputPreviewPaths, filepath.Join(opts.Output, "<run-id>", safeSegment(id), "manifest.json"))
		}
	}
	if len(plan.ConfiguredJourneys) == 0 {
		plan.ConfiguredJourneys = []string{}
	}
	if len(plan.DetectedAdapters) == 0 {
		plan.DetectedAdapters = []string{}
	}
	return publicPlan(plan, opts.ProjectDir), nil
}
