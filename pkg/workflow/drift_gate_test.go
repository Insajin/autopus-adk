package workflow

import (
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/lore"
)

// validLoreMessage is a commit message satisfying a Constraint-required config.
const validLoreMessage = "feat(workflow): add gate\n\nbody\n\nConstraint: gate verdict derives from exit codes\n"

// nonLoreMessage lacks the required Constraint trailer.
const nonLoreMessage = "add a thing\n"

func loreCfg() lore.LoreConfig {
	return lore.LoreConfig{RequiredTrailers: []string{"Constraint"}}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// S6: a staged generated surface with no source-of-truth change is detected and
// blocks the run.
func TestDetectGeneratedDrift_BlocksStagedGeneratedSurface(t *testing.T) {
	staged := []string{".claude/workflows/route_a.workflow.js"}

	drift := DetectGeneratedDrift(staged, false)
	if !contains(drift, ".claude/workflows/route_a.workflow.js") {
		t.Fatalf("drift = %v, want the staged generated path", drift)
	}

	// With a valid Lore message and no oversized files, the only block reason
	// is the generated-surface drift.
	report := Hygiene(staged, false, nil, DefaultSourceLimit, validLoreMessage, loreCfg())
	if !report.Blocked {
		t.Fatal("Hygiene must block on generated-surface drift")
	}
	if !contains(report.BlockedPaths, ".claude/workflows/route_a.workflow.js") {
		t.Fatalf("blocked paths = %v, want generated path", report.BlockedPaths)
	}
}

// A staged generated surface accompanied by a source-of-truth change is allowed.
func TestDetectGeneratedDrift_AllowsWhenSotChanged(t *testing.T) {
	staged := []string{".claude/workflows/route_a.workflow.js"}
	if drift := DetectGeneratedDrift(staged, true); len(drift) != 0 {
		t.Fatalf("drift = %v, want none when SoT changed", drift)
	}
}

// TestDetectGeneratedDrift_NormalizesPath locks the path-normalization hardening:
// non-canonical inputs cannot bypass the generated-surface block-list.
func TestDetectGeneratedDrift_NormalizesPath(t *testing.T) {
	staged := []string{"a/../.claude/workflows/x.js", ".//.codex/skills/y.md"}
	drift := DetectGeneratedDrift(staged, false)
	if len(drift) != 2 {
		t.Fatalf("drift = %v, want both non-canonical generated paths caught", drift)
	}
}

// S13: a 301-line source file plus a non-Lore message both block, with the
// oversized path in BlockedPaths and a Lore violation in Reasons.
func TestHygiene_BlocksOversizeAndLore(t *testing.T) {
	sizes := map[string]int{"new.go": 301, "ok.go": 200}

	report := Hygiene(nil, false, sizes, DefaultSourceLimit, nonLoreMessage, loreCfg())

	if !report.Blocked {
		t.Fatal("Hygiene must block on oversize + lore violation")
	}
	if !contains(report.BlockedPaths, "new.go") {
		t.Fatalf("blocked paths = %v, want new.go (301 lines)", report.BlockedPaths)
	}
	if contains(report.BlockedPaths, "ok.go") {
		t.Fatalf("ok.go (200 lines) must not be blocked: %v", report.BlockedPaths)
	}
	hasLore := false
	for _, r := range report.Reasons {
		if strings.Contains(r, "lore") {
			hasLore = true
		}
	}
	if !hasLore {
		t.Fatalf("reasons = %v, want a lore violation", report.Reasons)
	}
}
