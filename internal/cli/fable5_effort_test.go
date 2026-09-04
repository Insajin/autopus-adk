package cli

import (
	"strings"
	"testing"
)

func TestResolveEffort_FableModelsUseMaxInEveryQualityMode(t *testing.T) {
	t.Parallel()

	for _, quality := range []string{"ultra", "balanced"} {
		quality := quality
		t.Run(quality, func(t *testing.T) {
			t.Parallel()
			for _, model := range []string{
				"claude-fable-5-1", "fable-5-1", "claude-fable-5", "fable", "best",
			} {
				result, err := ResolveEffort(EffortResolveInput{
					FlagQuality: quality,
					Model:       model,
				})
				if err != nil {
					t.Fatalf("ResolveEffort(%s, %s): %v", quality, model, err)
				}
				if result.Effort != EffortMax {
					t.Errorf("ResolveEffort(%s, %s) = %q, want %q", quality, model, result.Effort, EffortMax)
				}
				if result.Source != EffortSourceQualityMode {
					t.Errorf("source = %q, want %q", result.Source, EffortSourceQualityMode)
				}
			}
		})
	}
}

func TestNormalizeModelID_FableKeys(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"claude-fable-5-1": "fable-5-1",
		"fable-5-1":        "fable-5-1",
		"claude-fable-5":   "fable-5",
		"fable":            "fable",
		"best":             "best",
	}
	for input, want := range cases {
		if got := normalizeModelID(input); got != want {
			t.Errorf("normalizeModelID(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestEffortDetectHelp_DistinguishesModelAndSessionEffort(t *testing.T) {
	t.Parallel()

	help := newEffortDetectCmd().Long
	for _, want := range []string{
		"low | medium | high | xhigh | max",
		"session-only",
		"ultracode",
		"claude-fable-5-1",
		"fable-5-1",
		"claude-fable-5",
		"fable",
		"best",
		"2.1.170",
		"2.1.203",
		"2.1.210",
	} {
		if !strings.Contains(help, want) {
			t.Errorf("help missing %q:\n%s", want, help)
		}
	}
}
