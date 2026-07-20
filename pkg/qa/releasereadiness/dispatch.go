package releasereadiness

import (
	"os/exec"

	"github.com/insajin/autopus-adk/pkg/qa/adapter"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
	qarun "github.com/insajin/autopus-adk/pkg/qa/run"
)

// laneForPack maps a journey pack to its release-readiness lane. Synthesized
// packs declare a single lane (browser-staging, desktop-native, mobile-scripted)
// mirroring the scaffold starters; we honor the declared lane and fall back to a
// surface-derived lane only when none is declared.
func laneForPack(pack journey.Pack) string {
	if len(pack.Lanes) > 0 && pack.Lanes[0] != "" {
		return pack.Lanes[0]
	}
	switch normalizeSurface(pack.Surface) {
	case surfaceWeb:
		return "browser-staging"
	case surfaceDesktop:
		return "desktop-native"
	case surfaceMobile:
		return "mobile-scripted"
	default:
		return pack.Surface
	}
}

// Canonical surface tokens used by regen.PresentSurfaces.
const (
	surfaceWeb     = "web"
	surfaceDesktop = "desktop"
	surfaceMobile  = "mobile"
)

// normalizeSurface maps a journey pack surface token to a present-surface token.
// Journey packs use "frontend" for the web surface; PresentSurfaces uses "web".
func normalizeSurface(surface string) string {
	switch surface {
	case "frontend", "web":
		return surfaceWeb
	case "desktop":
		return surfaceDesktop
	case "mobile":
		return surfaceMobile
	default:
		return surface
	}
}

// surfacePresent reports whether the pack's surface is in the present-surface
// set returned by regen.PresentSurfaces (web|desktop|mobile).
func surfacePresent(packSurface string, present []string) bool {
	want := normalizeSurface(packSurface)
	for _, s := range present {
		if s == want {
			return true
		}
	}
	return false
}

// missingBinary returns the first RequiredBinary of the pack's adapter that is
// not resolvable via exec.LookPath, or "" when every binary is present (or the
// adapter declares none). It uses exec.LookPath exclusively: no GNU timeout
// wrapper, no subprocess spawn for the probe (AC-008).
func missingBinary(adapterID string) (string, bool) {
	meta, ok := adapter.ByID(adapterID)
	if !ok {
		return "", false
	}
	for _, bin := range meta.RequiredBinaries {
		if _, err := exec.LookPath(bin); err != nil {
			return bin, true
		}
	}
	return "", false
}

// dispatchLane resolves one accepted pack into a LaneRow. It first fails closed
// on an absent surface, then on a missing surface tool, and only otherwise runs
// the lane through qarun.Execute and maps the run status to a lane status.
//
// Reason codes are set directly here from structured signals (surface presence,
// exec.LookPath); qarun setup-gap free text is never parsed for reason codes.
func dispatchLane(opts Options, pack journey.Pack, present []string, runFn runFunc) LaneRow {
	lane := laneForPack(pack)
	row := LaneRow{Lane: lane, DeterministicAuthority: true, adapterID: pack.Adapter.ID}

	if !surfacePresent(pack.Surface, present) {
		row.Status = statusSetupGap
		row.ReasonCode = adapter.ReasonSurfaceAbsent
		return row
	}
	if bin, missing := missingBinary(pack.Adapter.ID); missing {
		row.Status = statusSetupGap
		row.ReasonCode = adapter.ReasonSurfaceToolUnavailable
		row.FailureSummary = "required surface tool not on PATH: " + bin
		return row
	}

	result, err := runFn(qarun.Options{
		ProjectDir:      opts.ProjectDir,
		Lane:            lane,
		JourneyID:       pack.ID,
		AdapterID:       pack.Adapter.ID,
		Output:          runOutputDir(opts.ProjectDir),
		RuntimeProvider: opts.RuntimeProvider,
	})
	return laneRowFromRun(row, result, err)
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-011: statusSetupGap must match the release.LaneStatus "setup_gap" string value; aggregateVerdict in execute.go maps it via SetupGapToolUnavailable, so any rename is a two-file change.
const statusSetupGap = "setup_gap"

// runFunc is the injectable qarun.Execute seam so tests can drive deterministic
// exit-derived statuses without spawning the real mobile device runner.
type runFunc func(qarun.Options) (qarun.Result, error)
