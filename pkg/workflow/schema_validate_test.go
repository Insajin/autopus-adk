package workflow

import "testing"

// TestParseSchema_RejectsInjectionModel locks S6: a model string carrying JS
// breakout characters is rejected at the parse boundary before it can reach the
// generated workflow JS.
func TestParseSchema_RejectsInjectionModel(t *testing.T) {
	t.Parallel()
	data := []byte(`{"phases":[{"id":"planning","model":"claude-opus-4-8\");evil(("}]}`)
	if _, err := ParseSchema(data); err == nil {
		t.Fatal("expected unsafe-model rejection")
	}
}

// TestParseSchema_RejectsNonEnumEffort locks the effort whitelist boundary.
func TestParseSchema_RejectsNonEnumEffort(t *testing.T) {
	t.Parallel()
	data := []byte(`{"phases":[{"id":"planning","effort":"ultra"}]}`)
	if _, err := ParseSchema(data); err == nil {
		t.Fatal("expected non-enum effort rejection")
	}
}

func TestIsSafeAgentModel(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"":                          true,
		"claude-opus-4-8":           true,
		"claude-sonnet-4-6":         true,
		"claude-haiku-4-5":          true,
		"gpt-4":                     false,
		"claude-opus-4-8\");evil((": false,
	}
	for in, want := range cases {
		if got := isSafeAgentModel(in); got != want {
			t.Errorf("isSafeAgentModel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIsSafeEffort(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"":       true,
		"low":    true,
		"medium": true,
		"high":   true,
		"xhigh":  true,
		"max":    true,
		"ultra":  false,
		"HIGH":   false,
	}
	for in, want := range cases {
		if got := isSafeEffort(in); got != want {
			t.Errorf("isSafeEffort(%q) = %v, want %v", in, got, want)
		}
	}
}
