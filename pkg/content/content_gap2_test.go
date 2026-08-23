package content

import (
	"strings"
	"testing"
	"testing/fstest"
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
