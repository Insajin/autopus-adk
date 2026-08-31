package capture

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// validIndex builds a two-step index that passes Validate. Tests mutate one
// field at a time so a failure names exactly one broken rule.
func validIndex(t *testing.T, dir string) Index {
	t.Helper()
	shot := writeMedia(t, dir, filepath.Join("screenshots", "open.png"), "png-bytes-open")
	shot.Width, shot.Height = 1280, 720
	failShot := writeMedia(t, dir, filepath.Join("screenshots", "submit.png"), "png-bytes-submit")
	failShot.Width, failShot.Height = 1280, 720
	trace := writeMedia(t, dir, filepath.Join("traces", "checkout.zip"), "trace-bytes")
	spec := writeMedia(t, dir, filepath.Join("specs", "checkout.spec.ts"), "test('open', async () => {})")

	index := Index{
		SchemaVersion: IndexSchemaVersion,
		JourneyID:     "gui-checkout",
		Mode:          ModeAlways,
		Streams:       []string{StreamScreenshot, StreamConsole, StreamNetwork, StreamTrace},
		StartedAt:     "2026-01-02T03:00:00Z",
		EndedAt:       "2026-01-02T03:00:20Z",
		Steps: []Step{
			{
				StepID: "open-checkout", Order: 1, Title: "open checkout", Status: StatusPassed,
				StartedAt: "2026-01-02T03:00:00Z", EndedAt: "2026-01-02T03:00:05Z", DurationMS: 5000,
				Actions:    []Action{{API: "goto", TargetRef: "origin:0/checkout", DurationMS: 300}},
				Screenshot: &shot,
				Console:    &ConsoleSummary{Errors: 0, Warnings: 1, Messages: []ConsoleMessage{{Severity: SeverityWarning, Text: "slow hydration"}}},
				Network:    &NetworkSummary{Requests: 3, Failures: 0, Entries: []NetworkEntry{{Method: "GET", URLRef: "/api/cart", Status: 200, DurationMS: 40}}},
			},
			{
				StepID: "submit-order", Order: 2, Status: StatusFailed, FailureSummary: "expired session stayed on checkout",
				StartedAt: "2026-01-02T03:00:05Z", EndedAt: "2026-01-02T03:00:20Z", DurationMS: 15000,
				Screenshot: &failShot,
				Console:    &ConsoleSummary{Errors: 1, Messages: []ConsoleMessage{{Severity: SeverityError, Text: "session expired"}}},
				Network:    &NetworkSummary{Requests: 2, Failures: 1, Entries: []NetworkEntry{{Method: "POST", URLRef: "origin:0/api/order", Status: 500}}},
			},
		},
		Media:  []Media{{MediaRef: trace, Kind: StreamTrace, StepID: "submit-order"}},
		Replay: &Replay{Kind: ReplayKindPlaywrightGrep, Command: []string{"npm", "exec", "playwright", "test", "--", "--grep", "^checkout"}, SpecRefs: []string{"specs/checkout.spec.ts"}, Digest: spec.Digest, StepCount: 2},
	}
	index.Totals = ComputeTotals(index)
	return index
}

// validPolicy mirrors validIndex so Conform passes without findings.
func validPolicy() Policy {
	return Policy{
		Mode:            ModeAlways,
		Streams:         []string{StreamScreenshot, StreamConsole, StreamNetwork, StreamTrace},
		Screenshot:      ScreenshotOnFailure,
		ConsoleSeverity: SeverityWarning,
		RetainLocal:     true,
		ReplayScript:    ReplayRequired,
	}
}

// writeMedia writes a real file and returns a reference whose digest and size
// match the bytes on disk, so VerifyLocalMedia has something truthful to check.
func writeMedia(t *testing.T, dir, rel, body string) MediaRef {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	sum := sha256.Sum256([]byte(body))
	return MediaRef{
		Ref:       filepath.ToSlash(rel),
		Digest:    "sha256:" + hex.EncodeToString(sum[:]),
		Bytes:     int64(len(body)),
		Retention: RetentionLocalOnly,
	}
}
