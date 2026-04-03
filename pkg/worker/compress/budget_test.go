package compress

import (
	"strings"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{"empty", "", 0},
		{"short", "hello world", 2},
		{"exact multiple", "abcdefgh", 2},
		{"long", strings.Repeat("a", 400), 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EstimateTokens(tt.text); got != tt.want {
				t.Errorf("EstimateTokens() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestWindowSize(t *testing.T) {
	tests := []struct {
		provider string
		want     int
	}{
		{"claude", 200000},
		{"codex", 128000},
		{"gemini", 1000000},
		{"unknown", 128000},
		{"default", 128000},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			if got := WindowSize(tt.provider); got != tt.want {
				t.Errorf("WindowSize(%q) = %d, want %d", tt.provider, got, tt.want)
			}
		})
	}
}

func TestSummaryBudget(t *testing.T) {
	tests := []struct {
		provider string
		want     int
	}{
		{"claude", 10000},   // 200000 * 0.05 = 10000
		{"codex", 6400},     // 128000 * 0.05 = 6400
		{"gemini", 12288},   // 1000000 * 0.05 = 50000 → capped at 12288
		{"unknown", 6400},   // default 128000 * 0.05 = 6400
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			if got := SummaryBudget(tt.provider); got != tt.want {
				t.Errorf("SummaryBudget(%q) = %d, want %d", tt.provider, got, tt.want)
			}
		})
	}
}

func TestShouldCompress(t *testing.T) {
	// claude window = 200000 tokens, threshold = 100000 tokens = 400000 chars
	belowThreshold := strings.Repeat("a", 399999)
	aboveThreshold := strings.Repeat("a", 400005)

	if ShouldCompress(belowThreshold, "claude") {
		t.Error("expected no compression below threshold")
	}
	if !ShouldCompress(aboveThreshold, "claude") {
		t.Error("expected compression above threshold")
	}
}
