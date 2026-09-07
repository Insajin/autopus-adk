package cli

// SPEC-SPECREV-002 S11 integration wiring oracle: prove that the value produced
// by resolveSpecReviewMaxRevisions(gate, flags.LoopMode, singlePass) is actually
// consumed as the upper bound of the REVISE loop, not merely computed and
// discarded. The helper-level unit tests (spec_review_revisions_test.go) pin the
// arithmetic; these tests pin the wiring by counting real orchestra invocations
// driven through runSpecReviewWithOptions for LoopMode, --single-pass and an
// explicitly configured max_revisions: 0 (issue #187).

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/orchestra"
)

// wiringInitialFindings is the number of open findings emitted in the discover
// pass (revision 0). It must exceed the LoopMode call budget so that resolving
// one additional finding per verify revision never drives the active count to
// zero before the revision bound is reached (which would trip the PASS exit).
const wiringInitialFindings = 8

// wiringReviewOutput builds the fake orchestra output for a given 1-based call
// number, following the parser contract in pkg/spec/reviewer.go:
//
//   - Call 1 is the discover pass (priorFindings empty): emit
//     wiringInitialFindings distinct open major/correctness findings. The merge
//     pipeline assigns them sequential IDs F-001..F-00N.
//   - Calls >= 2 are verify passes (priorFindings non-empty): keep the verdict
//     REVISE and resolve a strictly growing prefix of prior findings via
//     FINDING_STATUS lines (F-001..F-(call-1) resolved). The remaining findings
//     stay open, so the merged active-finding count is
//     wiringInitialFindings-(call-1): strictly decreasing, but > 0 within the
//     LoopMode budget.
//
// Strictly decreasing active counts keep ShouldTripCircuitBreaker
// (trips when curr >= prev) from firing, and the still-open findings keep the
// verdict REVISE, so the only loop exit reached is the revision bound.
func wiringReviewOutput(call int) string {
	var b strings.Builder
	b.WriteString("VERDICT: REVISE\n")
	if call == 1 {
		for i := 0; i < wiringInitialFindings; i++ {
			fmt.Fprintf(&b, "FINDING: [major] [correctness] pkg/spec/reviewer.go:%d issue number %d\n", 100+i, i)
		}
		return b.String()
	}
	// Verify pass: resolve the first (call-1) prior findings, leaving the rest open.
	resolved := call - 1
	for i := 1; i <= resolved; i++ {
		fmt.Fprintf(&b, "FINDING_STATUS: F-%03d | resolved | addressed in revision %d\n", i, call-1)
	}
	return b.String()
}

// runWiringReviewCountingCalls drives runSpecReviewWithOptions with the given
// LoopMode and returns how many times orchestra was invoked. With the
// strictly-decreasing-but-nonzero active-finding schedule above, the loop runs
// to its revision bound, so the call count equals maxRevisions + 1 (the loop is
// `for revision := 0; revision <= max`).
//
// The fake also appends a line to spec.md on every call, standing in for the
// author edit each revision is supposed to review. Without it the loop stops on
// unchanged input after the first round (issue #187), which would mask the
// revision-bound wiring this oracle measures.
func runWiringReviewCountingCalls(t *testing.T, loopMode bool, specID string, opts specReviewOptions) int {
	t.Helper()

	dir := t.TempDir()
	specDir := scaffoldReviewSpec(t, dir, specID)
	setFakeProviderOnPath(t, dir, "claude")

	origWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origWD) }()
	require.NoError(t, os.Chdir(dir))

	callCount := 0
	origRunner := specReviewRunOrchestra
	specReviewRunOrchestra = func(_ context.Context, _ orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		callCount++
		appendWiringSpecEdit(t, specDir, callCount)
		return &orchestra.OrchestraResult{
			Responses: []orchestra.ProviderResponse{
				{Provider: "claude", Output: wiringReviewOutput(callCount)},
			},
		}, nil
	}
	defer func() { specReviewRunOrchestra = origRunner }()

	ctx := withGlobalFlags(context.Background(), globalFlags{LoopMode: loopMode})
	require.NoError(t, runSpecReviewWithOptions(ctx, specID, "consensus", 10, opts))
	return callCount
}

// appendWiringSpecEdit simulates the author revising spec.md between rounds so
// each revision reviews genuinely different input.
func appendWiringSpecEdit(t *testing.T, specDir string, call int) {
	t.Helper()
	path := filepath.Join(specDir, "spec.md")
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	body = append(body, []byte(fmt.Sprintf("\n<!-- author revision %d -->\n", call))...)
	require.NoError(t, os.WriteFile(path, body, 0o600))
}

// TestRunSpecReview_LoopModeFloorBoundsRevisionLoop is the S11 wiring oracle.
//
// Expected-value derivation (semantics read from spec_review_loop.go and
// spec_review.go):
//   - The loop is `for revision := 0; revision <= p.maxRevisions; revision++`,
//     and orchestra is invoked exactly once per iteration, so the number of
//     orchestra calls == maxRevisions + 1 when the loop runs to its bound.
//   - The default harness config (pkg/config/defaults.go) sets
//     ReviewGate.MaxRevisions = 2. In a scaffolded temp dir with no autopus.yaml,
//     the harness config loader returns these defaults.
//   - resolveSpecReviewMaxRevisions(gate{MaxRevisions:2}, false, false) = 2 -> 3 calls.
//   - resolveSpecReviewMaxRevisions(gate{MaxRevisions:2}, true, false)  = 5
//     (loopModeMinRevisions floor; configured 2 < 5)                     -> 6 calls.
//   - The fake resolves one additional prior finding per verify revision, so the
//     active-finding count strictly decreases (ShouldTripCircuitBreaker never
//     trips) while staying > 0 (PASS exit never fires), and it edits spec.md so
//     the unchanged-input stop never fires either, leaving only the
//     revision-bound exit. The call-count gap (6 vs 3) is therefore caused
//     solely by the resolved maxRevisions value flowing into the loop bound —
//     the wiring under test.
func TestRunSpecReview_LoopModeFloorBoundsRevisionLoop(t *testing.T) {
	// Not parallel: uses os.Chdir and the package-level specReviewRunOrchestra seam.

	loopCalls := runWiringReviewCountingCalls(t, true, "SPEC-WIRING-LOOP-001", specReviewOptions{})
	noLoopCalls := runWiringReviewCountingCalls(t, false, "SPEC-WIRING-NOLOOP-001", specReviewOptions{})

	// maxRevisions + 1, with maxRevisions resolved from the loop floor / config.
	assert.Equal(t, 6, loopCalls,
		"LoopMode=true: floor loopModeMinRevisions=5 -> loop bound 5 -> 6 orchestra calls")
	assert.Equal(t, 3, noLoopCalls,
		"LoopMode=false: default config MaxRevisions=2 -> loop bound 2 -> 3 orchestra calls")
	assert.Greater(t, loopCalls, noLoopCalls,
		"the --loop floor must raise the revision budget actually consumed by the loop")
}

// Issue #187: --single-pass means exactly one provider round even when the
// config budget and the --loop floor would both allow more.
func TestRunSpecReview_SinglePassRunsExactlyOneRound(t *testing.T) {
	// Not parallel: uses os.Chdir and the package-level specReviewRunOrchestra seam.

	calls := runWiringReviewCountingCalls(
		t, false, "SPEC-WIRING-SINGLE-001", specReviewOptions{singlePass: true},
	)
	assert.Equal(t, 1, calls,
		"--single-pass with default MaxRevisions=2 must still dispatch exactly once")

	loopCalls := runWiringReviewCountingCalls(
		t, true, "SPEC-WIRING-SINGLE-LOOP-001", specReviewOptions{singlePass: true},
	)
	assert.Equal(t, 1, loopCalls, "--single-pass overrides the --loop floor")
}

// An explicitly configured max_revisions: 0 must mean one round, not the
// unset-vs-zero fallback to the default budget (issue #187).
func TestRunSpecReview_ExplicitZeroMaxRevisionsRunsExactlyOneRound(t *testing.T) {
	// Not parallel: uses os.Chdir and the package-level specReviewRunOrchestra seam.

	dir := t.TempDir()
	specID := "SPEC-WIRING-ZERO-001"
	specDir := scaffoldReviewSpec(t, dir, specID)
	setFakeProviderOnPath(t, dir, "claude")
	cfg := config.DefaultFullConfig("wiring-zero")
	cfg.Spec.ReviewGate.Providers = []string{"claude"}
	cfg.Spec.ReviewGate.Judge = ""
	cfg.Spec.ReviewGate.MaxRevisions = new(0)
	cfg.Spec.ReviewGate.AutoCollectContext = false
	require.NoError(t, config.Save(dir, cfg))

	origWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origWD) }()
	require.NoError(t, os.Chdir(dir))

	callCount := 0
	origRunner := specReviewRunOrchestra
	specReviewRunOrchestra = func(_ context.Context, _ orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		callCount++
		appendWiringSpecEdit(t, specDir, callCount)
		return &orchestra.OrchestraResult{
			Responses: []orchestra.ProviderResponse{
				{Provider: "claude", Output: wiringReviewOutput(callCount)},
			},
		}, nil
	}
	defer func() { specReviewRunOrchestra = origRunner }()

	ctx := withGlobalFlags(context.Background(), globalFlags{})
	require.NoError(t, runSpecReviewWithOptions(ctx, specID, "consensus", 10, specReviewOptions{}))
	assert.Equal(t, 1, callCount,
		"explicit max_revisions: 0 must not re-apply the default budget of 3")
}

// The --single-pass flag must be discoverable in the command help.
func TestSpecReviewCommand_DocumentsSinglePassFlag(t *testing.T) {
	t.Parallel()

	flag := newSpecReviewCmd().Flags().Lookup("single-pass")
	require.NotNil(t, flag, "--single-pass must be registered on spec review")
	assert.Contains(t, flag.Usage, "one provider round")
	assert.Contains(t, flag.Usage, "--loop")
}
