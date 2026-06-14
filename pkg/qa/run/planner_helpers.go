package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	qacompile "github.com/insajin/autopus-adk/pkg/qa/compile"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

func allowDetectedFallback(opts Options) bool {
	if strings.EqualFold(opts.Lane, "gui-explore") || opts.AdapterID == "gui-explore" {
		return false
	}
	return !strings.EqualFold(opts.Lane, "mobile-readiness") && !strings.EqualFold(opts.Lane, laneMobileScripted)
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
	if legacyDeferredMobile(surface, adapterID) ||
		surface == "production_replay" || inputSource == "production_session" || authority == "ai" {
		return "deferred to SPEC-QAMESH-003"
	}
	return ""
}

func deferredCandidateReason(candidate qacompile.Candidate) string {
	if deferredAdapter(strings.ToLower(strings.TrimSpace(candidate.Adapter))) {
		return "deferred to SPEC-QAMESH-003"
	}
	if strings.EqualFold(strings.TrimSpace(candidate.InputSource), "production_session") ||
		strings.EqualFold(strings.TrimSpace(candidate.PassFailAuthority), "ai") {
		return "deferred to SPEC-QAMESH-003"
	}
	if candidate.ManualOrDeferred && strings.Contains(candidate.ErrorCode, "SPEC-QAMESH-003") {
		return "deferred to SPEC-QAMESH-003"
	}
	return ""
}

func legacyDeferredMobile(surface, adapterID string) bool {
	return surface == "mobile" && adapterID != "maestro-scripted" && adapterID != "appium-mobile-explore"
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
