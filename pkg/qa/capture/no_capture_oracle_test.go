package capture

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConform_NoCaptureRequiresEveryOracle is the whole point of the mode: a run
// that took no screenshot must still deliver every substitute observation, and a
// gap must name the kind the producer has to add next.
func TestConform_NoCaptureRequiresEveryOracle(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		drop     string
		want     string
		findings int
	}{
		{"complete", "", "", 0},
		{"missing geometry", OracleDOMGeometry, "missing_no_capture_oracle:dom_geometry", 1},
		{"missing accessibility tree", OracleAccessibilityTree, "missing_no_capture_oracle:accessibility_tree", 1},
		{"missing keyboard navigation", OracleKeyboardNavigation, "missing_no_capture_oracle:keyboard_navigation", 1},
		{"missing state transition", OracleStateTransition, "missing_no_capture_oracle:state_transition", 1},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			index := noCaptureIndex()
			index.Oracles = oraclesExcept(testCase.drop)
			index.Totals = ComputeTotals(index)
			require.NoError(t, Validate(index))

			findings := Conform(index, noCapturePolicy())
			require.Len(t, findings, testCase.findings)
			if testCase.want != "" {
				assert.Equal(t, testCase.want, findings[0])
			}
		})
	}
}

// TestConform_NoCaptureReportsFirstMissingKindOnly pins the fixed order: with
// three kinds missing the producer is told about dom_geometry, not handed a set
// whose order it would have to guess at.
func TestConform_NoCaptureReportsFirstMissingKindOnly(t *testing.T) {
	t.Parallel()

	index := noCaptureIndex()
	index.Oracles = []Oracle{{Kind: OracleStateTransition, StepID: "logout", Assertions: 2}}
	index.Totals = ComputeTotals(index)

	assert.Equal(t, []string{"missing_no_capture_oracle:dom_geometry"}, Conform(index, noCapturePolicy()))
}

// TestConform_NoCaptureRejectsSmuggledScreenshot keeps the mode honest: opting
// out of pixels and then storing them anyway is the failure the mode exists to
// prevent.
func TestConform_NoCaptureRejectsSmuggledScreenshot(t *testing.T) {
	t.Parallel()

	index := noCaptureIndex()
	index.Oracles = oraclesExcept("")
	index.Steps[0].Screenshot = &MediaRef{
		Ref: "screenshots/home.png", Digest: "sha256:" + shortDigest(), Bytes: 12, Retention: RetentionLocalOnly,
	}
	index.Totals = ComputeTotals(index)

	assert.Equal(t,
		[]string{`no-capture mode forbids screenshots but step "open-home" carries one`},
		Conform(index, noCapturePolicy()))
}

// TestConform_ScreenshotModeIgnoresOracles keeps the oracle set optional for a
// pixel run: a project that adds semantic evidence on top of screenshots must
// not be forced to add all four, and one that adds none must not start failing.
func TestConform_ScreenshotModeIgnoresOracles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	index := validIndex(t, dir)
	assert.Empty(t, Conform(index, validPolicy()))

	index.Oracles = []Oracle{{Kind: OracleKeyboardNavigation, StepID: "open-checkout", Assertions: 4}}
	index.Totals = ComputeTotals(index)
	require.NoError(t, Validate(index))
	assert.Empty(t, Conform(index, validPolicy()))
}

// TestValidate_RejectsUnknownOracleKind fails closed on a kind nothing produces:
// a misspelled oracle used to look exactly like a missing one.
func TestValidate_RejectsUnknownOracleKind(t *testing.T) {
	t.Parallel()

	index := noCaptureIndex()
	index.Oracles = []Oracle{{Kind: "dom-geometry"}}
	err := Validate(index)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `oracles[0].kind "dom-geometry" is unsupported`)

	index.Oracles = []Oracle{{Kind: OracleDOMGeometry, StepID: "../escape"}}
	err = Validate(index)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oracles[0].step_id must match")
}

// TestPolicyFromVerifyConfig maps the config surface onto the capture contract.
func TestPolicyFromVerifyConfig(t *testing.T) {
	t.Parallel()

	noCapture := PolicyFromVerifyConfig(VerifyCaptureNoCapture)
	require.NoError(t, ValidatePolicy(noCapture))
	assert.Equal(t, ModeOff, noCapture.Mode)
	assert.False(t, noCapture.Enabled())
	assert.True(t, noCapture.RequiresNoCaptureOracles())

	for _, capture := range []string{"", VerifyCaptureScreenshot, " no-capture ", "anything-else"} {
		policy := PolicyFromVerifyConfig(capture)
		require.NoError(t, ValidatePolicy(policy), capture)
		if capture == " no-capture " {
			assert.True(t, policy.RequiresNoCaptureOracles(), "surrounding whitespace must not change the mode")
			continue
		}
		assert.Equal(t, ModeAlways, policy.Mode, capture)
		assert.True(t, policy.HasStream(StreamScreenshot), capture)
		assert.False(t, policy.RequiresNoCaptureOracles(), capture)
	}

	// An undeclared policy is not a no-capture policy: packs written before the
	// capture contract existed must keep passing without oracles.
	assert.False(t, Policy{}.RequiresNoCaptureOracles())
	assert.Empty(t, Conform(Index{}, Policy{}))
}

func noCapturePolicy() Policy {
	return Policy{Mode: ModeOff}
}

// noCaptureIndex builds the index a no-capture producer emits: real steps, real
// statuses, no media at all.
func noCaptureIndex() Index {
	index := Index{
		SchemaVersion: IndexSchemaVersion,
		JourneyID:     "verify-home",
		Mode:          ModeOff,
		StartedAt:     "2026-01-02T03:00:00Z",
		EndedAt:       "2026-01-02T03:00:09Z",
		Steps: []Step{
			{StepID: "open-home", Order: 1, Title: "open home", Status: StatusPassed, DurationMS: 4000},
			{StepID: "logout", Order: 2, Title: "log out", Status: StatusPassed, DurationMS: 5000},
		},
	}
	index.Totals = ComputeTotals(index)
	return index
}

func oraclesExcept(drop string) []Oracle {
	oracles := make([]Oracle, 0, len(NoCaptureOracleKinds))
	for _, kind := range NoCaptureOracleKinds {
		if kind == drop {
			continue
		}
		oracles = append(oracles, Oracle{Kind: kind, StepID: "open-home", Assertions: 3})
	}
	return oracles
}

func shortDigest() string {
	return "0000000000000000000000000000000000000000000000000000000000000000"
}
