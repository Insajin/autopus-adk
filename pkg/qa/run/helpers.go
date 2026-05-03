package run

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/qa/adapter"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

func detectedPacks(plan Plan) []journey.Pack {
	packs := make([]journey.Pack, 0, len(plan.SelectedAdapters))
	for _, id := range plan.SelectedAdapters {
		packs = append(packs, journey.Pack{
			ID:      "detected-" + id,
			Surface: surfaceForAdapter(id),
			Lanes:   []string{plan.SelectedLane},
			Adapter: journey.AdapterRef{ID: id},
			Command: defaultCommand(id),
			Checks:  []journey.Check{{ID: id, Type: "unit_test"}},
			SourceRefs: journey.SourceRefs{
				SourceSpec:       "SPEC-QAMESH-002",
				AcceptanceRefs:   []string{"AC-QAMESH2-005"},
				OwnedPaths:       []string{"."},
				DoNotModifyPaths: []string{".codex/**", ".opencode/**", ".autopus/plugins/**"},
			},
		})
	}
	return packs
}

func candidatePacks(plan Plan) []journey.Pack {
	selected := map[string]bool{}
	for _, id := range plan.SelectedJourneys {
		selected[id] = true
	}
	packs := make([]journey.Pack, 0, len(plan.CandidateJourneys))
	for _, candidate := range plan.CandidateJourneys {
		if candidate.ManualOrDeferred || candidate.JourneyID == "" || candidate.Adapter == "" {
			continue
		}
		if deferredAdapter(strings.ToLower(strings.TrimSpace(candidate.Adapter))) ||
			strings.EqualFold(strings.TrimSpace(candidate.InputSource), "production_session") ||
			strings.EqualFold(strings.TrimSpace(candidate.PassFailAuthority), "ai") {
			continue
		}
		if !selected[candidate.JourneyID] {
			continue
		}
		command := journey.Command{Argv: candidate.Command, CWD: candidate.CWD, Timeout: candidate.Timeout, EnvAllowlist: candidate.EnvAllowlist}
		if command.CWD == "" {
			command.CWD = "."
		}
		if command.Timeout == "" {
			command.Timeout = "60s"
		}
		packs = append(packs, journey.Pack{
			ID:      candidate.JourneyID,
			Surface: surfaceForAdapter(candidate.Adapter),
			Lanes:   []string{plan.SelectedLane},
			Adapter: journey.AdapterRef{ID: candidate.Adapter},
			Command: command,
			Checks: []journey.Check{{
				ID:       candidate.StepID,
				Type:     "compiled_check",
				Expected: candidate.OracleThresholds,
			}},
			Artifacts: artifactRefs(candidate.Artifacts),
			SourceRefs: journey.SourceRefs{
				SourceSpec:       "SPEC-QAMESH-002",
				AcceptanceRefs:   candidate.AcceptanceRefs,
				OwnedPaths:       []string{"."},
				DoNotModifyPaths: []string{".codex/**", ".opencode/**", ".autopus/plugins/**"},
			},
		})
	}
	return packs
}

func artifactRefs(values []string) []journey.Artifact {
	out := make([]journey.Artifact, 0, len(values))
	for _, value := range values {
		out = append(out, journey.Artifact{Path: value})
	}
	return out
}

func setupGapFor(opts Options, pack journey.Pack) *SetupGap {
	if missing := missingCapabilities(opts.Profile, pack.ProfileRequirements.Capabilities); len(missing) > 0 {
		return &SetupGap{Adapter: pack.Adapter.ID, JourneyID: pack.ID, Reason: "missing profile capability: " + missing[0]}
	}
	item, ok := adapter.ByID(pack.Adapter.ID)
	if !ok {
		return &SetupGap{Adapter: pack.Adapter.ID, JourneyID: pack.ID, Reason: "unknown adapter"}
	}
	for _, binary := range item.RequiredBinaries {
		if _, err := exec.LookPath(binary); err != nil {
			return &SetupGap{Adapter: pack.Adapter.ID, JourneyID: pack.ID, Reason: "missing required binary: " + binary}
		}
	}
	if pack.Adapter.ID == "canary-template" && len(commandArgs(pack)) == 0 {
		return &SetupGap{Adapter: pack.Adapter.ID, JourneyID: pack.ID, Reason: "explicit safe canary command is required"}
	}
	if len(commandArgs(pack)) == 0 && pack.Adapter.ID != "go-test" && pack.Adapter.ID != "node-script" && pack.Adapter.ID != "pytest" && pack.Adapter.ID != "cargo-test" {
		return &SetupGap{Adapter: pack.Adapter.ID, JourneyID: pack.ID, Reason: "explicit safe command is required"}
	}
	return nil
}

func missingCapabilities(profile string, required []string) []string {
	if len(required) == 0 {
		return nil
	}
	available := map[string]bool{}
	for _, capability := range config.DefaultTestProfileCapabilities(profile) {
		available[capability] = true
	}
	var missing []string
	for _, capability := range required {
		if !available[capability] {
			missing = append(missing, capability)
		}
	}
	return missing
}

func aggregateStatus(result Result) string {
	if len(result.FailedChecks) > 0 {
		return "failed"
	}
	for _, adapterResult := range result.AdapterResults {
		if adapterResult.Status == "blocked" {
			return "blocked"
		}
	}
	if len(result.SetupGaps) > 0 {
		return "warning"
	}
	return "passed"
}

func surfaceForAdapter(id string) string {
	switch id {
	case "node-script", "vitest", "jest":
		return "package"
	case "playwright", "auto-verify":
		return "frontend"
	case "custom-command":
		return "custom"
	case "auto-test-run", "canary-template":
		return "multi"
	default:
		return "cli"
	}
}

func manifestOutputDir(runDir, journeyID string) string {
	return filepath.Join(runDir, safeSegment(journeyID))
}
