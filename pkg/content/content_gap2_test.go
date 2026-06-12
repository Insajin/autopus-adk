package content

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/insajin/autopus-adk/pkg/config"
)

// --- skills.go: convertSkillCodex / convertSkillGemini ---

func TestConvertSkillCodex_WithAndWithoutTriggers(t *testing.T) {
	skill := SkillDefinition{
		Name:        "planning",
		Description: "Planning skill",
		Triggers:    []string{"/plan", "plan this"},
		Level2Body:  "## Body\nContent here.",
	}
	out := convertSkillCodex(skill)
	if !strings.HasPrefix(out, "# auto-planning") {
		t.Errorf("codex prefix missing: %q", out[:40])
	}
	if !strings.Contains(out, "Planning skill") {
		t.Errorf("description missing: %q", out)
	}
	if !strings.Contains(out, "/plan") {
		t.Errorf("trigger missing: %q", out)
	}
	if !strings.Contains(out, "## Body") {
		t.Errorf("level2 body missing: %q", out)
	}
	// No triggers path.
	noTrig := SkillDefinition{Name: "x", Description: "d"}
	if strings.Contains(convertSkillCodex(noTrig), "Triggers") {
		t.Error("no-trigger codex must omit Triggers line")
	}
}

func TestConvertSkillGemini_WithAndWithoutTriggers(t *testing.T) {
	skill := SkillDefinition{
		Name:        "review",
		Description: "Review skill",
		Triggers:    []string{"/review"},
		Level2Body:  "## Instructions\nDo review.",
	}
	out := convertSkillGemini(skill)
	if !strings.HasPrefix(out, "---\n") {
		t.Errorf("gemini must start with YAML frontmatter: %q", out[:20])
	}
	if !strings.Contains(out, "name: auto-review") {
		t.Errorf("gemini name missing: %q", out)
	}
	if !strings.Contains(out, "triggers:\n") || !strings.Contains(out, "  - /review") {
		t.Errorf("gemini trigger missing: %q", out)
	}
	if !strings.Contains(out, "## Instructions") {
		t.Errorf("gemini body missing: %q", out)
	}
	// No triggers path.
	noTrig := SkillDefinition{Name: "y", Description: "d"}
	out2 := convertSkillGemini(noTrig)
	if strings.Contains(out2, "triggers:") {
		t.Error("no-trigger gemini must omit triggers section")
	}
}

// --- skill_catalog.go: LoadSkillCatalogFromFS error paths ---

func TestLoadSkillCatalogFromFS_ParseError(t *testing.T) {
	fsys := fstest.MapFS{
		"skills/bad.md": &fstest.MapFile{
			// Invalid YAML inside valid frontmatter delimiters triggers yaml parse error.
			Data: []byte("---\nname: [\nbad yaml\n---\n# body"),
		},
	}
	_, err := LoadSkillCatalogFromFS(fsys, "skills")
	if err == nil {
		t.Error("malformed YAML must return error")
	}
}

func TestLoadSkillCatalogFromFS_ReadError(t *testing.T) {
	// Non-existent directory returns an error.
	_, err := LoadSkillCatalogFromFS(fstest.MapFS{}, "nonexistent")
	if err == nil {
		t.Error("missing directory must return error")
	}
}

// --- workflow.go: GenerateWorkflow ---

func TestGenerateWorkflow_NilConfig(t *testing.T) {
	_, err := GenerateWorkflow(nil)
	if err == nil {
		t.Error("nil config must return error")
	}
}

func TestGenerateWorkflow_TDDMode(t *testing.T) {
	cfg := &config.HarnessConfig{ProjectName: "TestProject"}
	cfg.Methodology.Mode = "tdd"
	cfg.Methodology.Enforce = true
	cfg.Hooks.PreCommitLore = true
	cfg.Hooks.PreCommitArch = true
	cfg.Methodology.ReviewGate = true
	cfg.Platforms = []string{"claude-code", "codex"}

	out, err := GenerateWorkflow(cfg)
	if err != nil {
		t.Fatalf("GenerateWorkflow: %v", err)
	}
	if !strings.Contains(out, "# TestProject Workflow") {
		t.Errorf("title missing: %q", out[:60])
	}
	if !strings.Contains(out, "tdd") {
		t.Errorf("methodology missing: %q", out)
	}
	if !strings.Contains(out, "Phase 1: Red") {
		t.Errorf("TDD phase missing: %q", out)
	}
	if !strings.Contains(out, "Phase 3: Refactor") {
		t.Errorf("TDD refactor phase missing: %q", out)
	}
	if !strings.Contains(out, "Lore") {
		t.Errorf("lore policy missing: %q", out)
	}
	if !strings.Contains(out, "아키텍처") {
		t.Errorf("arch policy missing: %q", out)
	}
	if !strings.Contains(out, "리뷰 게이트") {
		t.Errorf("review gate missing: %q", out)
	}
	if !strings.Contains(out, "claude-code") {
		t.Errorf("platform list missing: %q", out)
	}
}

func TestGenerateWorkflow_DDDMode(t *testing.T) {
	cfg := &config.HarnessConfig{ProjectName: "P"}
	cfg.Methodology.Mode = "ddd"
	out, err := GenerateWorkflow(cfg)
	if err != nil {
		t.Fatalf("GenerateWorkflow ddd: %v", err)
	}
	if !strings.Contains(out, "Phase 1: Analyze") {
		t.Errorf("DDD analyze phase missing: %q", out)
	}
	if !strings.Contains(out, "Phase 3: Improve") {
		t.Errorf("DDD improve phase missing: %q", out)
	}
}

func TestGenerateWorkflow_DefaultMode(t *testing.T) {
	cfg := &config.HarnessConfig{ProjectName: "P"}
	out, err := GenerateWorkflow(cfg)
	if err != nil {
		t.Fatalf("GenerateWorkflow default: %v", err)
	}
	if !strings.Contains(out, "Phase 1: Planning") {
		t.Errorf("default planning phase missing: %q", out)
	}
}

func TestGenerateWorkflow_NoPlatforms(t *testing.T) {
	cfg := &config.HarnessConfig{ProjectName: "P"}
	out, err := GenerateWorkflow(cfg)
	if err != nil {
		t.Fatalf("GenerateWorkflow no platforms: %v", err)
	}
	// No platforms section when platforms list is empty.
	if strings.Contains(out, "Supported Platforms") {
		t.Errorf("empty platforms should omit section: %q", out)
	}
}
