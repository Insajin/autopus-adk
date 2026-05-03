package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
		OutputRoot:                 opts.Output,
		RunIndexPreviewPath:        filepath.Join(opts.Output, "<run-id>", "run-index.json"),
		ManifestOutputPreviewPaths: []string{},
		SetupGaps:                  []SetupGap{},
		Deferred:                   []SetupGap{},
		AdapterMetadata:            adapter.WithSetupGaps(),
		CandidateJourneys:          candidatePayloads(candidates),
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
	}
	for _, detection := range detections {
		plan.DetectedAdapters = append(plan.DetectedAdapters, detection.AdapterID)
		if detection.SetupGapReason != "" {
			plan.SetupGaps = append(plan.SetupGaps, SetupGap{Adapter: detection.AdapterID, Reason: detection.SetupGapReason})
		}
	}
	if len(plan.SelectedJourneys) == 0 && opts.JourneyID == "" {
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
	return plan, nil
}

func normalizeOptions(opts Options) Options {
	if strings.TrimSpace(opts.ProjectDir) == "" {
		opts.ProjectDir = "."
	}
	if strings.TrimSpace(opts.Profile) == "" {
		opts.Profile = "standalone"
	}
	if strings.TrimSpace(opts.Lane) == "" {
		opts.Lane = "fast"
	}
	if strings.TrimSpace(opts.Output) == "" {
		opts.Output = filepath.Join(opts.ProjectDir, ".autopus", "qa", "runs")
	}
	return opts
}

func includePack(pack journey.Pack, opts Options) bool {
	if opts.JourneyID != "" && pack.ID != opts.JourneyID {
		return false
	}
	if opts.AdapterID != "" && pack.Adapter.ID != opts.AdapterID {
		return false
	}
	return journey.HasLane(pack, opts.Lane)
}

func candidatePayloads(candidates []qacompile.Candidate) []CandidateJourney {
	out := make([]CandidateJourney, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, CandidateJourney{
			JourneyID:         candidate.JourneyID,
			StepID:            candidate.StepID,
			Adapter:           candidate.Adapter,
			Command:           candidate.Command,
			CWD:               candidate.CWD,
			Timeout:           candidate.Timeout,
			EnvAllowlist:      candidate.EnvAllowlist,
			Artifacts:         candidate.Artifacts,
			AcceptanceRefs:    candidate.AcceptanceRefs,
			Source:            candidate.Source,
			InputSource:       candidate.InputSource,
			PassFailAuthority: candidate.PassFailAuthority,
			OracleThresholds:  candidate.OracleThresholds,
			ManualOrDeferred:  candidate.ManualOrDeferred,
			ErrorCode:         candidate.ErrorCode,
		})
	}
	return out
}

func includeCandidate(candidate qacompile.Candidate, opts Options) bool {
	if candidate.ManualOrDeferred || candidate.JourneyID == "" || candidate.Adapter == "" {
		return false
	}
	if deferredCandidateReason(candidate) != "" {
		return false
	}
	return candidateMatchesFilters(candidate, opts)
}

func candidateMatchesFilters(candidate qacompile.Candidate, opts Options) bool {
	if opts.JourneyID != "" && candidate.JourneyID != opts.JourneyID {
		return false
	}
	if opts.AdapterID != "" && candidate.Adapter != opts.AdapterID {
		return false
	}
	return true
}

func deferredPackReason(pack journey.Pack) string {
	surface := strings.ToLower(strings.TrimSpace(pack.Surface))
	adapterID := strings.ToLower(strings.TrimSpace(pack.Adapter.ID))
	inputSource := strings.ToLower(strings.TrimSpace(pack.InputSource))
	authority := strings.ToLower(strings.TrimSpace(pack.PassFailAuthority))
	if surface == "mobile" || surface == "production_replay" || inputSource == "production_session" || authority == "ai" || deferredAdapter(adapterID) {
		return "deferred to SPEC-QAMESH-003"
	}
	return ""
}

func deferredCandidateReason(candidate qacompile.Candidate) string {
	if deferredAdapter(strings.ToLower(strings.TrimSpace(candidate.Adapter))) {
		return "deferred to SPEC-QAMESH-003"
	}
	if strings.EqualFold(strings.TrimSpace(candidate.InputSource), "production_session") || strings.EqualFold(strings.TrimSpace(candidate.PassFailAuthority), "ai") {
		return "deferred to SPEC-QAMESH-003"
	}
	if candidate.ManualOrDeferred && strings.Contains(candidate.ErrorCode, "SPEC-QAMESH-003") {
		return "deferred to SPEC-QAMESH-003"
	}
	return ""
}

func deferredAdapter(adapterID string) bool {
	switch adapterID {
	case "browserstack", "firebase-test-lab", "maestro", "detox", "session-replay":
		return true
	default:
		return false
	}
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func safeSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "item"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", " ", "-")
	return replacer.Replace(value)
}

func validateOutputRoot(projectDir, output string) error {
	root, err := filepath.Abs(projectDir)
	if err != nil {
		return err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	target, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	target, err = resolvePathForCreate(target)
	if err != nil {
		return err
	}
	if !pathWithin(root, target) {
		return fmt.Errorf("qa output must be inside project root")
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	if denied, ok := generatedSurfaceIn(rel); ok {
		return fmt.Errorf("qa output may not target generated surface %s", denied)
	}
	return nil
}

func resolvePathForCreate(path string) (string, error) {
	path = filepath.Clean(path)
	missing := []string{}
	current := path
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func generatedSurfaceIn(rel string) (string, bool) {
	parts := strings.Split(strings.ToLower(filepath.ToSlash(filepath.Clean(rel))), "/")
	for index, part := range parts {
		switch part {
		case ".codex", ".claude", ".gemini", ".opencode":
			return part, true
		case ".autopus":
			if index+1 < len(parts) && parts[index+1] == "plugins" {
				return ".autopus/plugins", true
			}
		}
	}
	return "", false
}

func pathWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
