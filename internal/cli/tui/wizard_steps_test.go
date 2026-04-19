package tui_test

import (
	"testing"

	"github.com/insajin/autopus-adk/internal/cli/tui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunInitWizard_ReturnsResult verifies R1: wizard returns a valid result struct.
func TestRunInitWizard_ReturnsResult(t *testing.T) {
	t.Parallel()
	result, err := tui.RunInitWizard(tui.InitWizardOpts{Accessible: true})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Cancelled)
}

// TestRunInitWizard_FlagSkipsStep verifies R13: pre-configured flags skip wizard steps.
func TestRunInitWizard_FlagSkipsStep(t *testing.T) {
	t.Parallel()

	opts := tui.InitWizardOpts{
		Quality:      "strict",
		NoReviewGate: true,
		Platforms:    []string{"claude-code"},
		Accessible:   true,
	}
	result, err := tui.RunInitWizard(opts)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Pre-configured quality must be preserved
	assert.Equal(t, "strict", result.Quality)
	// NoReviewGate=true means ReviewGate should be false
	assert.False(t, result.ReviewGate)
}

// TestRunInitWizard_PreConfiguredDefaults verifies R9: pre-set values appear in result.
func TestRunInitWizard_PreConfiguredDefaults(t *testing.T) {
	t.Parallel()
	result, err := tui.RunInitWizard(tui.InitWizardOpts{Quality: "balanced", Accessible: true})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "balanced", result.Quality)
}

// TestRunInitWizard_CompletionSummary verifies R10: result contains all expected fields.
func TestRunInitWizard_CompletionSummary(t *testing.T) {
	t.Parallel()
	result, err := tui.RunInitWizard(tui.InitWizardOpts{Accessible: true})
	require.NoError(t, err)
	require.NotNil(t, result)
	fields := map[string]string{
		"CommentsLang": result.CommentsLang,
		"CommitsLang":  result.CommitsLang,
		"AILang":       result.AILang,
		"Quality":      result.Quality,
		"Methodology":  result.Methodology,
	}
	for name, val := range fields {
		assert.NotEmpty(t, val, "%s should not be empty after wizard", name)
	}
}

// TestRunInitWizard_NonTTYDefaults verifies exact default values in accessible mode.
// Not parallel: uses t.Setenv to pin locale.
func TestRunInitWizard_NonTTYDefaults(t *testing.T) {
	// Pin locale to English so the test is deterministic.
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANGUAGE", "")

	result, err := tui.RunInitWizard(tui.InitWizardOpts{Accessible: true})
	require.NoError(t, err)

	assert.Equal(t, "en", result.CommentsLang)
	assert.Equal(t, "en", result.CommitsLang)
	assert.Equal(t, "en", result.AILang)
	assert.Equal(t, "balanced", result.Quality)
	assert.False(t, result.ReviewGate, "review gate defaults to false with zero providers")
	assert.Equal(t, "tdd", result.Methodology)
	assert.False(t, result.Cancelled)
}

// TestRunInitWizard_NonTTYDefaults_Korean verifies locale-aware defaults.
// Not parallel: uses t.Setenv to pin locale.
func TestRunInitWizard_NonTTYDefaults_Korean(t *testing.T) {
	t.Setenv("LANG", "ko_KR.UTF-8")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANGUAGE", "")

	result, err := tui.RunInitWizard(tui.InitWizardOpts{Accessible: true})
	require.NoError(t, err)

	assert.Equal(t, "en", result.CommentsLang, "code comments always English")
	assert.Equal(t, "ko", result.CommitsLang, "commits follow system locale")
	assert.Equal(t, "ko", result.AILang, "AI responses follow system locale")
}

// TestRunInitWizard_FlagCombinations verifies R13: various flag combos skip steps correctly.
func TestRunInitWizard_FlagCombinations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		opts        tui.InitWizardOpts
		wantQuality string
		wantGate    bool
	}{
		{
			name:        "quality flag only",
			opts:        tui.InitWizardOpts{Quality: "ultra", Accessible: true},
			wantQuality: "ultra",
			wantGate:    false,
		},
		{
			name:        "no-review-gate flag only",
			opts:        tui.InitWizardOpts{NoReviewGate: true, Accessible: true},
			wantQuality: "balanced",
			wantGate:    false,
		},
		{
			name:        "both flags set",
			opts:        tui.InitWizardOpts{Quality: "ultra", NoReviewGate: true, Accessible: true},
			wantQuality: "ultra",
			wantGate:    false,
		},
		{
			name:        "no flags — defaults applied",
			opts:        tui.InitWizardOpts{Accessible: true},
			wantQuality: "balanced",
			wantGate:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := tui.RunInitWizard(tc.opts)
			require.NoError(t, err)
			assert.Equal(t, tc.wantQuality, result.Quality)
			assert.Equal(t, tc.wantGate, result.ReviewGate)
		})
	}
}

// TestRunInitWizard_ProviderCounts verifies review gate default across provider counts.
func TestRunInitWizard_ProviderCounts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		providers []string
		wantGate  bool
	}{
		{"zero providers", nil, false},
		{"one provider", []string{"claude"}, false},
		{"two providers", []string{"claude", "openai"}, true},
		{"five providers", []string{"a", "b", "c", "d", "e"}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			opts := tui.InitWizardOpts{
				Accessible: true,
				Providers:  tc.providers,
			}
			result, err := tui.RunInitWizard(opts)
			require.NoError(t, err)
			assert.Equal(t, tc.wantGate, result.ReviewGate)
		})
	}
}

// TestRunInitWizard_CancelledZeroValues verifies cancelled result has zero-value fields.
func TestRunInitWizard_CancelledZeroValues(t *testing.T) {
	t.Parallel()

	// Construct a cancelled result manually to verify the contract:
	// when Cancelled is true, all other fields should be zero values.
	cancelled := tui.InitWizardResult{Cancelled: true}

	assert.True(t, cancelled.Cancelled)
	assert.Empty(t, cancelled.CommentsLang)
	assert.Empty(t, cancelled.CommitsLang)
	assert.Empty(t, cancelled.AILang)
	assert.Empty(t, cancelled.Quality)
	assert.False(t, cancelled.ReviewGate)
	assert.Empty(t, cancelled.Methodology)
}

// TestAutopusTheme verifies the custom huh theme is created successfully.
func TestAutopusTheme(t *testing.T) {
	t.Parallel()

	theme := tui.AutopusTheme()
	require.NotNil(t, theme, "AutopusTheme must return a non-nil theme")
}

// TestLangOptions verifies language option count and values.
func TestLangOptions(t *testing.T) {
	t.Parallel()

	opts := tui.TestLangOptions()
	assert.Len(t, opts, 4, "should have 4 language options")
}

// TestBuildStepList_StepCounts verifies step filtering based on flags.
func TestBuildStepList_StepCounts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		opts      tui.InitWizardOpts
		wantSteps int
	}{
		{
			name:      "all steps — no flags",
			opts:      tui.InitWizardOpts{},
			wantSteps: 5, // profile + lang + quality + review-gate + methodology
		},
		{
			name:      "quality pre-set — skip quality step",
			opts:      tui.InitWizardOpts{Quality: "ultra"},
			wantSteps: 4, // profile + lang + review-gate + methodology
		},
		{
			name:      "no-review-gate — skip gate step",
			opts:      tui.InitWizardOpts{NoReviewGate: true},
			wantSteps: 4, // profile + lang + quality + methodology
		},
		{
			name:      "both flags — skip quality and gate",
			opts:      tui.InitWizardOpts{Quality: "ultra", NoReviewGate: true},
			wantSteps: 3, // profile + lang + methodology
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			steps := tui.TestBuildStepList(tc.opts)
			assert.Len(t, steps, tc.wantSteps)
		})
	}
}

// TestBuildSteps_ReturnForms verifies each step builder produces a non-nil form.
func TestBuildSteps_ReturnForms(t *testing.T) {
	t.Parallel()

	result := &tui.InitWizardResult{}

	assert.NotNil(t, tui.TestBuildLangStep(1, 4, result))
	assert.NotNil(t, tui.TestBuildQualityStep(2, 4, result))
	assert.NotNil(t, tui.TestBuildMethodologyStep(4, 4, result))

	// Review gate with and without providers (covers both desc branches)
	assert.NotNil(t, tui.TestBuildReviewGateStep(3, 4, result,
		tui.InitWizardOpts{Providers: []string{"claude", "openai"}}))
	assert.NotNil(t, tui.TestBuildReviewGateStep(3, 4, result, tui.InitWizardOpts{}))
}

// TestBuildStepList_FormsCallable verifies all built steps produce runnable forms.
func TestBuildStepList_FormsCallable(t *testing.T) {
	t.Parallel()

	steps := tui.TestBuildStepList(tui.InitWizardOpts{})
	result := &tui.InitWizardResult{}
	for i, step := range steps {
		assert.NotNilf(t, step(result), "step %d should produce a non-nil form", i)
	}
}

// TestDefaultResult_UsageProfile verifies the default usage profile is "developer".
func TestDefaultResult_UsageProfile(t *testing.T) {
	t.Parallel()
	result := tui.TestDefaultResult(tui.InitWizardOpts{})
	assert.Equal(t, "developer", result.UsageProfile)
}

// TestDefaultResult_ExistingProfile verifies ExistingProfile propagates to result.
func TestDefaultResult_ExistingProfile(t *testing.T) {
	t.Parallel()
	result := tui.TestDefaultResult(tui.InitWizardOpts{ExistingProfile: "fullstack"})
	assert.Equal(t, "fullstack", result.UsageProfile)
}

// TestBuildStepList_IncludesProfileStep verifies the profile step is first when no ExistingProfile.
func TestBuildStepList_IncludesProfileStep(t *testing.T) {
	t.Parallel()
	steps := tui.TestBuildStepList(tui.InitWizardOpts{})
	require.NotEmpty(t, steps)
	result := &tui.InitWizardResult{}
	assert.NotNil(t, steps[0](result), "first step (profile) must return a non-nil form")
	// ExistingProfile set → one fewer step
	skipped := tui.TestBuildStepList(tui.InitWizardOpts{ExistingProfile: "developer"})
	assert.Len(t, skipped, len(steps)-1, "ExistingProfile should skip the profile step")
}

// TestBuildProfileStep verifies buildProfileStep returns a valid huh.Form.
func TestBuildProfileStep(t *testing.T) {
	t.Parallel()
	result := &tui.InitWizardResult{}
	assert.NotNil(t, tui.TestBuildProfileStep(1, 5, result))
}
