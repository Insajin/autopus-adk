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

func TestWorkflowBindingSurface_AutoGoUsesCurrentWorkflowArgsShape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if _, err := claude.NewWithRoot(root).Generate(context.Background(), config.DefaultFullConfig("workflow-binding")); err != nil {
		t.Fatal(err)
	}
	detailPath := filepath.Join(root, ".claude", "skills", "auto-go", "SKILL.md")
	body, err := os.ReadFile(detailPath)
	if err != nil {
		t.Fatal(err)
	}
	contract := string(body)

	for _, want := range []string{
		"Workflow({",
		`scriptPath: ".claude/workflows/route_a.workflow.js"`,
		"args: {",
		"planning result handed into implementation",
	} {
		if !strings.Contains(contract, want) {
			t.Errorf("auto-go current Workflow contract missing %q", want)
		}
	}
	if strings.Contains(contract, "team Workflow substrate") {
		t.Fatal("--workflow must not route through the retained team workflow")
	}
}
