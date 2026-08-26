package content

import "testing"

func TestResolveCatalogSkillOutputName(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		wantName string
		wantPath string
	}{
		{name: "planning", platform: "claude", wantName: "planning", wantPath: ".claude/skills/planning/SKILL.md"},
		{name: "planning", platform: "claude-code", wantName: "planning", wantPath: ".claude/skills/planning/SKILL.md"},
		{name: "planning", platform: "codex", wantName: "codex-planning", wantPath: ".codex/skills/codex-planning/SKILL.md"},
		{name: "codex-planning", platform: "codex", wantName: "codex-planning", wantPath: ".codex/skills/codex-planning/SKILL.md"},
		{name: "planning", platform: "gemini", wantName: "planning", wantPath: ".gemini/skills/autopus/planning/SKILL.md"},
		{name: "planning", platform: "antigravity-cli", wantName: "planning", wantPath: ".gemini/skills/autopus/planning/SKILL.md"},
		{name: "planning", platform: "opencode", wantName: "planning", wantPath: ".agents/skills/planning/SKILL.md"},
		{name: "planning", platform: "omp", wantName: "planning", wantPath: ".omp/skills/planning/SKILL.md"},
		{name: "planning", platform: "unknown", wantName: "planning", wantPath: ""},
	}

	for _, test := range tests {
		t.Run(test.platform+"/"+test.name, func(t *testing.T) {
			if got := ResolveCatalogSkillOutputName(test.name, test.platform); got != test.wantName {
				t.Errorf("ResolveCatalogSkillOutputName(%q, %q) = %q, want %q", test.name, test.platform, got, test.wantName)
			}
			if got := resolveDefaultSkillTarget(test.name, test.platform); got != test.wantPath {
				t.Errorf("resolveDefaultSkillTarget(%q, %q) = %q, want %q", test.name, test.platform, got, test.wantPath)
			}
		})
	}
}

func TestResolveCatalogSkillStateAndRefPath_UseNativeTargets(t *testing.T) {
	skill := CatalogSkill{
		Name:           "planning",
		CompileTargets: []string{"claude", "codex", "gemini", "opencode", "omp"},
		Visibility:     SkillVisibilityShared,
	}
	catalog := &SkillCatalog{skills: map[string]CatalogSkill{"planning": skill}}
	tests := []struct {
		platform string
		wantPath string
	}{
		{platform: "claude", wantPath: ".claude/skills/planning/SKILL.md"},
		{platform: "codex", wantPath: ".codex/skills/codex-planning/SKILL.md"},
		{platform: "gemini", wantPath: ".gemini/skills/autopus/planning/SKILL.md"},
		{platform: "opencode", wantPath: ".agents/skills/planning/SKILL.md"},
		{platform: "omp", wantPath: ".omp/skills/planning/SKILL.md"},
	}

	for _, test := range tests {
		t.Run(test.platform, func(t *testing.T) {
			cfg := cfgFull(test.platform)
			state := ResolveCatalogSkillState(skill, test.platform, cfg)
			if !state.Compiled || !state.Visible || state.TargetPath != test.wantPath {
				t.Errorf("ResolveCatalogSkillState(%q) = %+v, want visible target %q", test.platform, state, test.wantPath)
			}
			if got := ResolveCatalogSkillRefPath(catalog, skill.Name, test.platform, cfg); got != test.wantPath {
				t.Errorf("ResolveCatalogSkillRefPath(%q) = %q, want %q", test.platform, got, test.wantPath)
			}
		})
	}
}
