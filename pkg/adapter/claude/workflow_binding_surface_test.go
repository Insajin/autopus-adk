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

func TestWorkflowBindingSurface_AutoGoResolvesOnceAndReusesBareQualityForAllSegments(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if _, err := claude.NewWithRoot(root).Generate(context.Background(), config.DefaultFullConfig("workflow-binding")); err != nil {
		t.Fatal(err)
	}
	detailPath := filepath.Join(root, ".claude", "skills", "autopus", "auto-go.md")
	body, err := os.ReadFile(detailPath)
	if err != nil {
		t.Fatal(err)
	}
	contract := string(body)

	if strings.Contains(contract, `run("auto workflow binding`) {
		t.Fatal("auto-go must not interpolate a shell command string")
	}
	if got := strings.Count(contract, `"auto", "workflow", "binding"`); got != 1 {
		t.Fatalf("auto-go argv binding command count = %d, want exactly 1", got)
	}
	for _, flag := range []string{`"--quality"`, `"--risk-tier", "auto"`, `"--files-file"`, `"--rollout-receipt"`, `"--format", "json"`} {
		if !strings.Contains(contract, flag) {
			t.Errorf("auto-go binding command is missing %q", flag)
		}
	}
	if got := strings.Count(contract, "quality: binding.quality"); got != 4 {
		t.Fatalf("binding.quality handoff count = %d, want 4 segment launches", got)
	}
	for _, segment := range []string{"A", "B", "C", "D"} {
		want := "quality: binding.quality, segment:'" + segment + "'"
		if !strings.Contains(contract, want) {
			t.Errorf("auto-go does not pass the same bare quality map to segment %s: missing %q", segment, want)
		}
	}
	if strings.Contains(contract, "quality: binding, segment:") {
		t.Fatal("auto-go must pass binding.quality, not the receipt wrapper")
	}
}
