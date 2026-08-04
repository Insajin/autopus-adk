package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/memindex"
)

func TestPhasePromptBuilder_InitialSnapshotRejectsFirstPassMutation(t *testing.T) {
	const specID = "SPEC-PIPELINE-COHERENCE-001"
	dir := filepath.Join(t.TempDir(), specID)
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	originalPlan := []byte("original plan snapshot")
	documents := map[string][]byte{
		"spec.md":       []byte("# " + specID + ": coherence test\nrequired outcome"),
		"plan.md":       originalPlan,
		"acceptance.md": []byte("Given a coherent snapshot\nThen no mixed identity is published"),
	}
	for name, content := range documents {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	builder := NewPhasePromptBuilder(dir)
	var mutationErr error
	builder.requiredSnapshotFirstPassHook = func() {
		mutationErr = os.WriteFile(filepath.Join(dir, "plan.md"), []byte("mutated between passes"), 0o600)
	}
	prompt, manifest, err := builder.BuildPromptWithManifest(PhasePlan, PhaseContext{
		ContextResult: &memindex.ContextResult{Prompt: "receipt"},
	})
	if mutationErr != nil {
		t.Fatal(mutationErr)
	}
	if err == nil || !strings.Contains(err.Error(), "plan.md changed while snapshot was built") {
		t.Fatalf("expected coherent snapshot failure, got %v", err)
	}
	if prompt != "" || len(manifest.Entries) != 0 {
		t.Fatalf("mixed snapshot must not publish prompt or manifest: %q %+v", prompt, manifest)
	}

	if err := os.WriteFile(filepath.Join(dir, "plan.md"), originalPlan, 0o600); err != nil {
		t.Fatal(err)
	}
	prompt, manifest, err = builder.BuildPromptWithManifest(PhaseReview, PhaseContext{
		ContextResult: &memindex.ContextResult{Prompt: "receipt"},
	})
	if err == nil || prompt != "" || len(manifest.Entries) != 0 {
		t.Fatalf("failed initial freeze must remain fail-closed: err=%v prompt=%q manifest=%+v", err, prompt, manifest)
	}
}

func TestPhasePromptBuilder_RejectsDocumentsThatDriftedAfterRunResolution(t *testing.T) {
	const specID = "SPEC-PIPELINE-SNAPSHOT-001"
	dir := filepath.Join(t.TempDir(), specID)
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"spec.md":       "# " + specID + ": snapshot test\nrequired outcome",
		"plan.md":       "original plan",
		"acceptance.md": "Given the original plan\nThen the binding matches",
		"research.md":   "original research",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := SpecSnapshotHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plan.md"), []byte("mutated plan"), 0o600); err != nil {
		t.Fatal(err)
	}
	builder := NewPhasePromptBuilder(dir)
	builder.expectedSnapshotHash = expected
	prompt, manifest, err := builder.BuildPromptWithManifest(PhasePlan, PhaseContext{FrozenRequiredDocuments: true})
	if err == nil || !strings.Contains(err.Error(), "resolved SPEC snapshot") {
		t.Fatalf("expected resolved snapshot mismatch, got %v", err)
	}
	if prompt != "" || len(manifest.Entries) != 0 {
		t.Fatalf("drifted snapshot must not publish prompt or manifest: %q %+v", prompt, manifest)
	}
}
