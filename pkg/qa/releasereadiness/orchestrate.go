package releasereadiness

import (
	"sort"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
	"github.com/insajin/autopus-adk/pkg/qa/regen"
	"github.com/insajin/autopus-adk/pkg/qa/release"
	qarun "github.com/insajin/autopus-adk/pkg/qa/run"
)

// Orchestrate runs the release-readiness flow over a project directory. It is
// the only entry point; no scheduler, hook, or CI trigger invokes it.
//
// Flow: present surfaces -> regen.BuildResult -> RedactDiff + AssertDiffSafe ->
// build payload. When approval is not granted (or is declined) it returns the
// redacted diff with files_written=0 and lanes_executed=0 and performs no write
// or execution. When approved it persists the accepted packs via
// regen.ApplyPacks, dispatches each eligible synthesized or configured pack's
// lane, aggregates a deterministic verdict, and attaches a sanitized v2 evidence summary.
func Orchestrate(opts Options) (Payload, error) {
	return orchestrateWith(opts, func(o qarun.Options) (qarun.Result, error) {
		return qarun.Execute(o)
	})
}

// orchestrateWith is the testable core: runFn is the qarun.Execute seam so
// tests drive deterministic exit-derived statuses without the real (unexported)
// mobile device runner.
func orchestrateWith(opts Options, runFn runFunc) (Payload, error) {
	surfaces := regen.PresentSurfaces(opts.ProjectDir)

	result, err := regen.BuildResult(opts.ProjectDir)
	if err != nil {
		return Payload{}, err
	}
	redacted := regen.RedactDiff(result.Diff)
	if err := regen.AssertDiffSafe(redacted); err != nil {
		return Payload{}, err
	}

	payload := Payload{
		SchemaVersion:    SchemaVersion,
		AnalyzedSurfaces: surfaces,
		Diff:             redacted,
		LaneRows:         []LaneRow{},
		Verdict:          Verdict{Status: string(release.GateStatusPassed), DeterministicAuthority: true},
	}

	// Decline takes precedence over approve so a declined run never writes or
	// executes anything (AC-011).
	if opts.Decline {
		payload.Phase = string(PhaseDeclined)
		return payload, nil
	}
	if !opts.Approve {
		payload.Phase = string(PhaseDiffPresented)
		// No surfaces means nothing to regenerate; report analyzed and make no
		// regenerated claim (AC-013). Diff counts are already 0 in that case.
		if len(surfaces) == 0 {
			payload.Phase = string(PhaseAnalyzed)
		}
		return payload, nil
	}

	// Approved path: persist accepted packs, then dispatch their lanes.
	accepted := result.AcceptedPacks()
	dispatchPacks, err := readinessDispatchPacks(opts.ProjectDir, accepted)
	if err != nil {
		return Payload{}, err
	}
	if err := validateReadinessRuntimeProvider(opts.RuntimeProvider, dispatchPacks); err != nil {
		return Payload{}, err
	}
	applied, err := regen.ApplyPacks(opts.ProjectDir, accepted)
	if err != nil {
		return Payload{}, err
	}
	payload.FilesWritten = len(applied.Written)

	rows := dispatchAccepted(opts, dispatchPacks, surfaces, runFn)
	payload.LaneRows = rows
	payload.LanesExecuted = len(rows)
	payload.Verdict = aggregateVerdict(rows)
	payload.Phase = string(PhaseExecuted)

	summary, err := buildEvidenceSummary(payload)
	if err != nil {
		return Payload{}, err
	}
	payload.EvidenceSummary = summary
	return payload, nil
}

func readinessDispatchPacks(projectDir string, accepted []journey.Pack) ([]journey.Pack, error) {
	existing, err := regen.LoadExistingPacks(projectDir)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(accepted))
	packs := append([]journey.Pack(nil), accepted...)
	for _, pack := range accepted {
		seen[pack.ID] = true
	}
	ids := make([]string, 0, len(existing))
	for id := range existing {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		pack := existing[id]
		if seen[id] || pack.Adapter.ID != "desktop-accessibility-observe" {
			continue
		}
		if evaluated := regen.EvaluatePack(projectDir, pack.Surface, pack); evaluated.Excluded {
			continue
		}
		packs = append(packs, pack)
		seen[id] = true
	}
	return packs, nil
}

func validateReadinessRuntimeProvider(provider desktopobserve.RuntimeProvider, packs []journey.Pack) error {
	if provider != "" && provider != desktopobserve.RuntimeProviderLocal && provider != desktopobserve.RuntimeProviderOrca {
		return desktopobserve.ErrRuntimeProviderInvalid
	}
	for _, pack := range packs {
		if pack.Adapter.ID == "desktop-accessibility-observe" && provider == "" {
			return desktopobserve.ErrRuntimeProviderRequired
		}
	}
	return nil
}

// dispatchAccepted resolves a LaneRow for every eligible pack. Each pack maps to
// exactly one lane; callers provide accepted synthesis results plus validated
// additive desktop observation packs (AC-012).
func dispatchAccepted(opts Options, packs []journey.Pack, present []string, runFn runFunc) []LaneRow {
	rows := make([]LaneRow, 0, len(packs))
	for _, pack := range packs {
		rows = append(rows, dispatchLane(opts, pack, present, runFn))
	}
	return rows
}
