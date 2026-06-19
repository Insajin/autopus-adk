package claude_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/config"
)

// TestGenerateProducesRouteAWorkflow verifies the S1 adapter half + REQ-004:
// full-mode Generate emits the generated Route A workflow JS with its
// edit-forbidden warning, the four phase ids, and a manifest registration.
func TestGenerateProducesRouteAWorkflow(t *testing.T) {
	dir := t.TempDir()
	a := claude.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	if _, err := a.Generate(context.Background(), cfg); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	wfPath := filepath.Join(dir, ".claude", "workflows", "route_a.workflow.js")
	data, err := os.ReadFile(wfPath)
	if err != nil {
		t.Fatalf("workflow file not found at %s: %v", wfPath, err)
	}
	content := string(data)

	// First line MUST carry the generated / edit-forbidden warning.
	firstLine := content
	if idx := strings.IndexByte(content, '\n'); idx >= 0 {
		firstLine = content[:idx]
	}
	if !strings.Contains(firstLine, "GENERATED") || !strings.Contains(firstLine, "DO NOT EDIT") {
		t.Errorf("first line missing generated/edit-forbidden warning: %q", firstLine)
	}

	// All four phase-id tokens must be present.
	for _, phase := range []string{"planning", "implementation", "gate_build_test", "release_hygiene"} {
		if !strings.Contains(content, phase) {
			t.Errorf("workflow content missing phase id %q", phase)
		}
	}

	// Manifest (REQ-004) must register the workflow target path.
	manifestPath := filepath.Join(dir, ".autopus", "claude-code-manifest.json")
	mdata, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest not found at %s: %v", manifestPath, err)
	}
	if !strings.Contains(string(mdata), ".claude/workflows/route_a.workflow.js") {
		t.Errorf("manifest does not register the workflow path:\n%s", string(mdata))
	}
}
