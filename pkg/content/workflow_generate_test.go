package content

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/workflow"
)

// repoContentDir resolves the real module-root content/ directory relative to
// this test file (pkg/content/workflow_generate_test.go).
func repoContentDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller path")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "content")
}

var s1CanonicalPhases = []string{"gate_build_test", "implementation", "planning", "release_hygiene"} // sorted

func sortedSet(ss []string) []string {
	out := append([]string(nil), ss...)
	sort.Strings(out)
	return out
}

// TestS1_DeterministicGeneration verifies that generating the workflow JS twice
// from the same content yields byte-identical output, and that the phase-id set
// extracted from the generated JS, the schema, and the markdown all equal the
// canonical four phases.
func TestS1_DeterministicGeneration(t *testing.T) {
	contentDir := repoContentDir(t)

	tmpl1 := t.TempDir()
	tmpl2 := t.TempDir()

	if err := generateWorkflowTemplates(contentDir, tmpl1); err != nil {
		t.Fatalf("generate run 1: %v", err)
	}
	if err := generateWorkflowTemplates(contentDir, tmpl2); err != nil {
		t.Fatalf("generate run 2: %v", err)
	}

	js1 := readGeneratedJS(t, tmpl1)
	js2 := readGeneratedJS(t, tmpl2)
	if js1 != js2 {
		t.Fatalf("generated js.tmpl not byte-identical across runs")
	}

	// Line 1 is the generated warning.
	if firstLine := strings.SplitN(js1, "\n", 2)[0]; !strings.Contains(firstLine, "GENERATED") || !strings.Contains(firstLine, "DO NOT EDIT") {
		t.Errorf("first line missing generated warning: %q", firstLine)
	}

	// Phase-id set from generated JS.
	jsIDs := extractPhaseIDsFromJS(js1)
	jsSet := make([]string, 0, len(jsIDs))
	for id := range jsIDs {
		jsSet = append(jsSet, id)
	}

	// Phase-id set from schema.
	schema, err := workflow.LoadSchema(filepath.Join(contentDir, "workflows", "route_a.schema.json"))
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}
	schemaSet := schema.PhaseIDs()

	// Phase-id set from markdown (presence of each canonical phase token).
	mdBytes, err := os.ReadFile(filepath.Join(contentDir, "workflows", "route_a.md"))
	if err != nil {
		t.Fatalf("read md: %v", err)
	}
	var mdSet []string
	for _, want := range []string{"planning", "implementation", "gate_build_test", "release_hygiene"} {
		if strings.Contains(string(mdBytes), want) {
			mdSet = append(mdSet, want)
		}
	}

	if got := sortedSet(jsSet); !equalStrs(got, s1CanonicalPhases) {
		t.Errorf("js phase set = %v, want %v", got, s1CanonicalPhases)
	}
	if got := sortedSet(schemaSet); !equalStrs(got, s1CanonicalPhases) {
		t.Errorf("schema phase set = %v, want %v", got, s1CanonicalPhases)
	}
	if got := sortedSet(mdSet); !equalStrs(got, s1CanonicalPhases) {
		t.Errorf("md phase set = %v, want %v", got, s1CanonicalPhases)
	}
}

func readGeneratedJS(t *testing.T, templateDir string) string {
	t.Helper()
	path := filepath.Join(templateDir, "claude", "workflows", "route_a.workflow.js.tmpl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated js %q: %v", path, err)
	}
	return string(data)
}

func equalStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
