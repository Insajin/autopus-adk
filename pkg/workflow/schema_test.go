package workflow

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoSchemaPath resolves the real content/workflows/route_a.schema.json from
// the module root relative to this test file.
func repoSchemaPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller path")
	}
	// pkg/workflow/schema_test.go -> module root is two dirs up.
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")
	return filepath.Join(root, "content", "workflows", "route_a.schema.json")
}

var canonicalPhases = []string{"planning", "implementation", "gate_build_test", "release_hygiene"}

func TestLoadSchema_PhaseIDsOrdered(t *testing.T) {
	s, err := LoadSchema(repoSchemaPath(t))
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	got := s.PhaseIDs()
	if len(got) != len(canonicalPhases) {
		t.Fatalf("phase count = %d, want %d (%v)", len(got), len(canonicalPhases), got)
	}
	for i, want := range canonicalPhases {
		if got[i] != want {
			t.Errorf("phase[%d] = %q, want %q", i, got[i], want)
		}
	}
}

func TestSchema_GateResultTypeIsExitCode(t *testing.T) {
	s, err := LoadSchema(repoSchemaPath(t))
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	rt := s.ResultTypeSet()
	if rt["gate_build_test"] != "exit_code" {
		t.Errorf("gate_build_test result-type = %q, want %q", rt["gate_build_test"], "exit_code")
	}
	// Non-gate phases carry no result-type.
	for _, id := range []string{"planning", "implementation", "release_hygiene"} {
		if rt[id] != "" {
			t.Errorf("phase %q result-type = %q, want empty", id, rt[id])
		}
	}
}

func TestParseSchema_RoundTripSets(t *testing.T) {
	data := []byte(`{"phases":[
		{"id":"planning","retry":1,"budget":100},
		{"id":"gate_build_test","retry":2,"budget":200,"verdict_source":"exit_code"}
	]}`)
	s, err := ParseSchema(data)
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	if got := s.RetrySet(); got["planning"] != 1 || got["gate_build_test"] != 2 {
		t.Errorf("RetrySet = %v", got)
	}
	if got := s.BudgetSet(); got["planning"] != 100 || got["gate_build_test"] != 200 {
		t.Errorf("BudgetSet = %v", got)
	}
	if got := s.ResultTypeSet(); got["gate_build_test"] != "exit_code" {
		t.Errorf("ResultTypeSet = %v", got)
	}
}

func TestParseSchema_RejectsEmpty(t *testing.T) {
	if _, err := ParseSchema([]byte(`{"phases":[]}`)); err == nil {
		t.Fatal("expected error for empty phases")
	}
}

// TestParseSchema_RejectsUnsafePhaseID locks the Q-SEC-01 hardening: ids with
// characters that could break/inject the generated workflow JS are fail-closed
// at the parse boundary.
func TestParseSchema_RejectsUnsafePhaseID(t *testing.T) {
	unsafe := []string{
		`{"phases":[{"id":"gate'); evil("}]}`,
		`{"phases":[{"id":"a\nb"}]}`,
		`{"phases":[{"id":"a b"}]}`,
		`{"phases":[{"id":"{title}"}]}`,
	}
	for _, data := range unsafe {
		if _, err := ParseSchema([]byte(data)); err == nil {
			t.Errorf("expected unsafe-id rejection for %q", data)
		}
	}
	// snake_case and hyphenated ids remain accepted.
	if _, err := ParseSchema([]byte(`{"phases":[{"id":"gate_build_test"},{"id":"release-hygiene"}]}`)); err != nil {
		t.Errorf("safe ids rejected: %v", err)
	}
}

// TestParseSchema_RoundTripQualitySets round-trips the model/effort/depth fields
// and asserts the new accessor maps surface them.
func TestParseSchema_RoundTripQualitySets(t *testing.T) {
	t.Parallel()
	data := []byte(`{"phases":[
		{"id":"planning","model":"claude-opus-4-8","effort":"medium"},
		{"id":"implementation","model":"claude-sonnet-4-6","effort":"high","fan_out_cap":5},
		{"id":"review","verify_votes":3,"synthesis":true,"retry":2}
	]}`)
	s, err := ParseSchema(data)
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	if got := s.ModelSet(); got["planning"] != "claude-opus-4-8" || got["implementation"] != "claude-sonnet-4-6" {
		t.Errorf("ModelSet = %v", got)
	}
	if got := s.EffortSet(); got["planning"] != "medium" || got["implementation"] != "high" {
		t.Errorf("EffortSet = %v", got)
	}
	d := s.DepthSet()
	if d["implementation"].FanOutCap != 5 {
		t.Errorf("implementation FanOutCap = %d, want 5", d["implementation"].FanOutCap)
	}
	if d["review"].VerifyVotes != 3 || !d["review"].Synthesis || d["review"].Retry != 2 {
		t.Errorf("review depth = %+v", d["review"])
	}
}

// TestParseSchema_RejectsVerifyVotesOverCap locks S4: a verify_votes above the
// cap is rejected fail-closed and the error names the cap.
func TestParseSchema_RejectsVerifyVotesOverCap(t *testing.T) {
	t.Parallel()
	_, err := ParseSchema([]byte(`{"phases":[{"id":"review","verify_votes":4}]}`))
	if err == nil {
		t.Fatal("expected verify_votes over-cap rejection")
	}
	if !strings.Contains(err.Error(), "cap") {
		t.Errorf("error %q should mention the cap", err)
	}
}

// TestParseSchema_RetryCap locks S13: retry above the cap is rejected; a
// within-cap retry is preserved in RetrySet.
func TestParseSchema_RetryCap(t *testing.T) {
	t.Parallel()
	if _, err := ParseSchema([]byte(`{"phases":[{"id":"planning","retry":5}]}`)); err == nil {
		t.Fatal("expected retry over-cap rejection")
	}
	s, err := ParseSchema([]byte(`{"phases":[{"id":"planning","retry":2}]}`))
	if err != nil {
		t.Fatalf("ParseSchema retry=2: %v", err)
	}
	if got := s.RetrySet(); got["planning"] != 2 {
		t.Errorf("RetrySet[planning] = %d, want 2", got["planning"])
	}
}
