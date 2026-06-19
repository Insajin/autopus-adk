package workflow

import (
	"path/filepath"
	"runtime"
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
