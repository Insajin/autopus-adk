package releasereadiness

import (
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
// regen.ApplyPacks, dispatches each persisted pack's lane, aggregates a
// deterministic verdict, and attaches a sanitized v2 evidence summary.
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
	applied, err := regen.ApplyPacks(opts.ProjectDir, accepted)
	if err != nil {
		return Payload{}, err
	}
	payload.FilesWritten = len(applied.Written)

	rows := dispatchAccepted(opts, accepted, surfaces, runFn)
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

// dispatchAccepted resolves a LaneRow for every accepted pack. Each pack maps to
// exactly one lane; ApplyPacks already excluded invalid/AI-authority packs, so
// only persisted-eligible packs are dispatched (AC-012).
func dispatchAccepted(opts Options, packs []journey.Pack, present []string, runFn runFunc) []LaneRow {
	rows := make([]LaneRow, 0, len(packs))
	for _, pack := range packs {
		rows = append(rows, dispatchLane(opts, pack, present, runFn))
	}
	return rows
}
