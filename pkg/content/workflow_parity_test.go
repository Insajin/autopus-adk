package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTempManifest builds a temp content dir containing workflows/route_a.md
// and workflows/route_a.schema.json with the provided bytes.
func writeTempManifest(t *testing.T, schemaJSON, md string) string {
	t.Helper()
	contentDir := t.TempDir()
	wfDir := filepath.Join(contentDir, "workflows")
	if err := os.MkdirAll(wfDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "route_a.schema.json"), []byte(schemaJSON), 0644); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "route_a.md"), []byte(md), 0644); err != nil {
		t.Fatalf("write md: %v", err)
	}
	return contentDir
}

const alignedSchema = `{"phases":[
	{"id":"planning","retry":0,"budget":60000},
	{"id":"implementation","retry":0,"budget":120000},
	{"id":"gate_build_test","retry":0,"budget":40000,"verdict_source":"exit_code"},
	{"id":"release_hygiene","retry":0,"budget":30000}
]}`

// driftedSchema removes gate_build_test, diverging from the markdown.
const driftedSchema = `{"phases":[
	{"id":"planning","retry":0,"budget":60000},
	{"id":"implementation","retry":0,"budget":120000},
	{"id":"release_hygiene","retry":0,"budget":30000}
]}`

const alignedMD = `# Route A

### planning
plan.

### implementation
impl.

### gate_build_test
gate with verdict_source exit_code.

### release_hygiene
hygiene.
`

// TestS2_ParityFailClosed verifies that a manifest where the schema is missing
// gate_build_test fails generation closed, names the diverging phase, and does
// NOT write the js.tmpl.
func TestS2_ParityFailClosed(t *testing.T) {
	contentDir := writeTempManifest(t, driftedSchema, alignedMD)
	tmplDir := t.TempDir()

	err := generateWorkflowTemplates(contentDir, tmplDir)
	if err == nil {
		t.Fatal("expected parity drift error, got nil")
	}
	if !strings.Contains(err.Error(), "gate_build_test") {
		t.Errorf("error must name diverging phase gate_build_test, got: %v", err)
	}

	// Fail-closed: js.tmpl must NOT be written.
	jsPath := filepath.Join(tmplDir, "claude", "workflows", "route_a.workflow.js.tmpl")
	if _, statErr := os.Stat(jsPath); !os.IsNotExist(statErr) {
		t.Errorf("js.tmpl must not be written on parity failure, stat err = %v", statErr)
	}
}

// TestS2_ParityFailClosed_MDMissingPhase verifies the reverse drift direction:
// schema declares a phase absent from the markdown.
func TestS2_ParityFailClosed_MDMissingPhase(t *testing.T) {
	mdMissingGate := strings.Replace(alignedMD, "### gate_build_test\ngate with verdict_source exit_code.\n\n", "", 1)
	contentDir := writeTempManifest(t, alignedSchema, mdMissingGate)
	tmplDir := t.TempDir()

	err := generateWorkflowTemplates(contentDir, tmplDir)
	if err == nil {
		t.Fatal("expected parity drift error, got nil")
	}
	if !strings.Contains(err.Error(), "gate_build_test") {
		t.Errorf("error must name diverging phase gate_build_test, got: %v", err)
	}
	jsPath := filepath.Join(tmplDir, "claude", "workflows", "route_a.workflow.js.tmpl")
	if _, statErr := os.Stat(jsPath); !os.IsNotExist(statErr) {
		t.Errorf("js.tmpl must not be written on parity failure, stat err = %v", statErr)
	}
}

// TestS2_ParityPass_AlignedManifest verifies the positive case: an aligned
// manifest generates without error and writes the js.tmpl.
func TestS2_ParityPass_AlignedManifest(t *testing.T) {
	contentDir := writeTempManifest(t, alignedSchema, alignedMD)
	tmplDir := t.TempDir()

	if err := generateWorkflowTemplates(contentDir, tmplDir); err != nil {
		t.Fatalf("aligned manifest must not error: %v", err)
	}
	jsPath := filepath.Join(tmplDir, "claude", "workflows", "route_a.workflow.js.tmpl")
	if _, err := os.Stat(jsPath); err != nil {
		t.Errorf("js.tmpl must be written on parity pass: %v", err)
	}
}
