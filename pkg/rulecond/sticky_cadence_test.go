package rulecond_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/rulecond"
)

// injectingIndexes drives count sequential prompts for one session at cadence
// and returns the 1-based prompt indexes that produced structured output.
func injectingIndexes(t *testing.T, f *stickyFixture, session string, cadence, count int) []int {
	t.Helper()
	var fired []int
	for index := 1; index <= count; index++ {
		stdout, stderr := f.submitAt(session, cadence)
		assert.Empty(t, stderr, "a healthy sticky set writes no stderr at index %d", index)
		if strings.TrimSpace(stdout) != "" {
			fired = append(fired, index)
		}
	}
	return fired
}

// TestStickyFire_CadenceEmitsAtFixedIndexes is the S1 oracle for
// REQ-STICKYRULE-FIRE-01: 17 prompts at cadence 8 inject at 1, 9, and 17 only.
func TestStickyFire_CadenceEmitsAtFixedIndexes(t *testing.T) {
	f := newStickyFixture(t)
	f.writeShippedStickyPair()

	var contexts []string
	var fired []int
	for index := 1; index <= 17; index++ {
		stdout, stderr := f.submit("S-alpha")
		require.Empty(t, stderr, "index %d must produce no diagnostic", index)
		if ctx := stickyContext(t, stdout); ctx != "" {
			fired = append(fired, index)
			contexts = append(contexts, ctx)
		}
		for _, quiet := range []int{2, 8, 10, 16} {
			if index == quiet {
				assert.Empty(t, stdout, "index %d is off-cadence and must write empty stdout", index)
			}
		}
	}

	assert.Equal(t, []int{1, 9, 17}, fired,
		"cadence 8 injects at index 1 and every index where (index-1) mod 8 == 0")
	require.Len(t, contexts, 3)
	for i, ctx := range contexts {
		assert.Contains(t, ctx, ruleLanguagePolicy, "injection %d must name language-policy", i)
		assert.Contains(t, ctx, ruleObjectiveReasoning, "injection %d must name objective-reasoning", i)
		assert.Contains(t, ctx, injectableBody(t, ruleLanguagePolicy),
			"injection %d must carry the language-policy injectable body", i)
	}
	assert.Equal(t, 17, f.counter("S-alpha"))
}

// TestStickyFire_SessionsKeepIndependentCounters is the S2 oracle for
// REQ-STICKYRULE-FIRE-02. The 12-step order is fixed, not alternating, so each
// session's own prompt index is what decides injection.
func TestStickyFire_SessionsKeepIndependentCounters(t *testing.T) {
	f := newStickyFixture(t)
	f.writeShippedStickyPair()

	order := []string{
		"alpha", "beta", "alpha", "alpha", "beta", "alpha",
		"alpha", "beta", "alpha", "alpha", "alpha", "alpha",
	}

	var alphaSteps, betaSteps []int
	calls := map[string]int{}
	for i, session := range order {
		step := i + 1
		calls[session]++
		stdout, stderr := f.submit(session)
		assert.Empty(t, stderr, "step %d", step)
		if strings.TrimSpace(stdout) == "" {
			continue
		}
		assert.Contains(t, stickyContext(t, stdout), ruleLanguagePolicy, "step %d", step)
		if session == "alpha" {
			alphaSteps = append(alphaSteps, step)
		} else {
			betaSteps = append(betaSteps, step)
		}
	}

	assert.Equal(t, 9, calls["alpha"], "alpha is invoked 9 times")
	assert.Equal(t, 3, calls["beta"], "beta is invoked 3 times")
	assert.Equal(t, []int{1, 12}, alphaSteps,
		"alpha injects on its own indexes 1 and 9, which are steps 1 and 12")
	assert.Equal(t, []int{2}, betaSteps,
		"beta injects on its own index 1 only, which is step 2")
	assert.Equal(t, 9, f.counter("alpha"))
	assert.Equal(t, 3, f.counter("beta"))
}

// TestResolveStickyCadence_ConfiguredValues is the S14 resolution oracle for
// REQ-STICKYRULE-MAP-01. A non-integer scalar never reaches this function: the
// typed yaml.Unmarshal in pkg/config/loader.go rejects it at config load.
func TestResolveStickyCadence_ConfiguredValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		configured int
		want       int
	}{
		{name: "absent key decodes to the zero value", configured: 0, want: 8},
		{name: "explicit zero", configured: 0, want: 8},
		{name: "negative", configured: -3, want: 8},
		{name: "positive is honored", configured: 2, want: 2},
		{name: "one injects on every prompt", configured: 1, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, rulecond.ResolveStickyCadence(tt.configured))
		})
	}
	assert.Equal(t, 8, rulecond.DefaultStickyCadence,
		"the documented fallback cadence is 8")
}

// TestStickyFire_ResolvedCadenceDrivesInjection is the S14 behavioral half:
// each configured value must produce the exact injecting index set over 1..9.
func TestStickyFire_ResolvedCadenceDrivesInjection(t *testing.T) {
	tests := []struct {
		name       string
		configured int
		want       []int
	}{
		{name: "absent key falls back to 8", configured: 0, want: []int{1, 9}},
		{name: "negative falls back to 8", configured: -3, want: []int{1, 9}},
		{name: "two", configured: 2, want: []int{1, 3, 5, 7, 9}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newStickyFixture(t)
			f.writeShippedStickyPair()
			cadence := rulecond.ResolveStickyCadence(tt.configured)
			assert.Equal(t, tt.want, injectingIndexes(t, f, "S-cadence", cadence, 9))
		})
	}
}
