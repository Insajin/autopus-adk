package capture

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePolicy_AcceptsUndeclaredAndWellFormed(t *testing.T) {
	t.Parallel()

	// Packs written before this contract leave the block empty and must keep working.
	require.NoError(t, ValidatePolicy(Policy{}))
	assert.False(t, Policy{}.Declared())
	assert.False(t, Policy{}.Enabled())

	policy := validPolicy()
	require.NoError(t, ValidatePolicy(policy))
	assert.True(t, policy.Enabled())

	off := Policy{Mode: ModeOff}
	require.NoError(t, ValidatePolicy(off))
	assert.True(t, off.Declared())
	assert.False(t, off.Enabled())
}

// TestValidatePolicy_RejectsUnknownFields is the whole point of the inline map:
// a misspelled capture key used to silently disable evidence collection.
func TestValidatePolicy_RejectsUnknownFields(t *testing.T) {
	t.Parallel()

	policy := validPolicy()
	policy.Unknown = map[string]any{"screenshots": "per-step", "modee": "always"}
	err := ValidatePolicy(policy)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown fields")
	assert.Contains(t, err.Error(), "modee")
	assert.Contains(t, err.Error(), "screenshots")
}

func TestValidatePolicy_RejectsContradictions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		mutate func(*Policy)
		want   string
	}{
		{"mode", func(p *Policy) { p.Mode = "maybe" }, "must be off, on-failure, or always"},
		{"stream", func(p *Policy) { p.Streams = []string{"keystrokes"} }, "unsupported stream"},
		{"duplicate stream", func(p *Policy) { p.Streams = append(p.Streams, StreamConsole) }, "repeats"},
		{"off with streams", func(p *Policy) { p.Mode = ModeOff }, "off may not declare streams"},
		{"cadence", func(p *Policy) { p.Screenshot = "sometimes" }, "must be off, on-failure, or per-step"},
		{"severity", func(p *Policy) { p.ConsoleSeverity = "verbose" }, "must be info, warning, or error"},
		{"replay", func(p *Policy) { p.ReplayScript = "maybe" }, "must be off, optional, or required"},
		{
			"trace without local retention",
			func(p *Policy) { p.RetainLocal = false; p.Screenshot = ScreenshotOff },
			`"trace" requires gui.capture.retain_local`,
		},
		{
			"screenshot cadence without stream",
			func(p *Policy) { p.Streams = []string{StreamConsole}; p.Screenshot = ScreenshotPerStep },
			"requires the screenshot stream",
		},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			policy := validPolicy()
			testCase.mutate(&policy)
			err := ValidatePolicy(policy)
			require.Error(t, err)
			assert.Contains(t, err.Error(), testCase.want)
		})
	}
}

func TestConform_AcceptsMatchingEvidence(t *testing.T) {
	t.Parallel()

	index := validIndex(t, t.TempDir())
	assert.Empty(t, Conform(index, validPolicy()))
}

func TestConform_ReportsMissingEvidence(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		mutate func(*Index, *Policy)
		want   string
	}{
		{
			"mode mismatch",
			func(i *Index, _ *Policy) { i.Mode = ModeOnFailure },
			"does not match policy",
		},
		{
			"undeclared stream",
			func(i *Index, _ *Policy) { i.Streams = append(i.Streams, StreamVideo) },
			`stream "video" was not declared`,
		},
		{
			"missing console",
			func(i *Index, _ *Policy) { i.Steps[0].Console = nil },
			"missing console evidence",
		},
		{
			"missing network",
			func(i *Index, _ *Policy) { i.Steps[1].Network = nil },
			"missing network evidence",
		},
		{
			"missing per-step screenshot",
			func(i *Index, p *Policy) { p.Screenshot = ScreenshotPerStep; i.Steps[0].Screenshot = nil },
			"missing a screenshot",
		},
		{
			"missing trace media",
			func(i *Index, _ *Policy) { i.Media = nil },
			"no trace media was captured",
		},
		{
			"missing required replay",
			func(i *Index, _ *Policy) { i.Replay = nil },
			"requires a replay reference",
		},
		{
			"console below threshold",
			func(i *Index, p *Policy) {
				p.ConsoleSeverity = SeverityError
				i.Steps[0].Console.Messages[0].Severity = SeverityInfo
			},
			"below the error threshold",
		},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			index := validIndex(t, t.TempDir())
			policy := validPolicy()
			testCase.mutate(&index, &policy)
			findings := Conform(index, policy)
			require.NotEmpty(t, findings)
			assert.Contains(t, joined(findings), testCase.want)
		})
	}
}

// TestConform_OnFailureModeOnlyRequiresFailedSteps keeps a green run cheap: a
// passing step does not have to carry every declared stream.
func TestConform_OnFailureModeOnlyRequiresFailedSteps(t *testing.T) {
	t.Parallel()

	index := validIndex(t, t.TempDir())
	index.Mode = ModeOnFailure
	index.Steps[0].Console = nil
	index.Steps[0].Network = nil
	index.Media = nil

	policy := validPolicy()
	policy.Mode = ModeOnFailure
	assert.Empty(t, Conform(index, policy))

	// The failed step still must carry its evidence.
	index.Steps[1].Console = nil
	assert.Contains(t, joined(Conform(index, policy)), `step "submit-order" is missing console evidence`)
}

func joined(values []string) string {
	out := ""
	for _, value := range values {
		out += value + "\n"
	}
	return out
}
