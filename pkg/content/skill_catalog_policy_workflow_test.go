package content

import "testing"

// TestPolicy_HarnessWorkflowIsClaudeOnly asserts the harness-workflow skill is
// scoped to the claude-code platform via both visibility and compile targets
// (SPEC-HARNESS-WORKFLOW-001 REQ-005 / S3).
func TestPolicy_HarnessWorkflowIsClaudeOnly(t *testing.T) {
	if got := visibilityForSkill("harness-workflow"); got != SkillVisibilityClaudeOnly {
		t.Errorf("visibilityForSkill(harness-workflow) = %q, want %q", got, SkillVisibilityClaudeOnly)
	}

	targets := compileTargetsForSkill("harness-workflow")
	if len(targets) != 1 || targets[0] != "claude" {
		t.Errorf("compileTargetsForSkill(harness-workflow) = %v, want [claude]", targets)
	}
}

// TestPolicy_NonWorkflowSkillStaysShared confirms the claude-only carve-out does
// not regress the default shared/all-platform behavior for other skills.
func TestPolicy_NonWorkflowSkillStaysShared(t *testing.T) {
	if got := visibilityForSkill("metrics"); got != SkillVisibilityShared {
		t.Errorf("visibilityForSkill(metrics) = %q, want %q", got, SkillVisibilityShared)
	}

	targets := compileTargetsForSkill("metrics")
	want := []string{"claude", "codex", "gemini", "opencode"}
	if len(targets) != len(want) {
		t.Fatalf("compileTargetsForSkill(metrics) = %v, want %v", targets, want)
	}
	for i, p := range want {
		if targets[i] != p {
			t.Errorf("compileTargetsForSkill(metrics)[%d] = %q, want %q", i, targets[i], p)
		}
	}
}
