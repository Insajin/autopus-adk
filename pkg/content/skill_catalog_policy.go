package content

var coreSkillSet = map[string]bool{
	"adaptive-quality":            true,
	"agent-pipeline":              true,
	"agent-presets":               true,
	"agent-teams":                 true,
	"ax-annotation":               true,
	"debugging":                   true,
	"ddd":                         true,
	"frontend-verify":             true,
	"hash-anchored-edit":          true,
	"make-interfaces-feel-better": true,
	"planning":                    true,
	"review":                      true,
	"spec-review":                 true,
	"subagent-dev":                true,
	"tdd":                         true,
	"testing-strategy":            true,
	"using-autopus":               true,
	"verification":                true,
	"worktree-isolation":          true,
}

var bundleOverrides = map[string][]string{
	"brainstorming":               {"product", "research"},
	"browser-automation":          {"frontend", "ops"},
	"ci-cd":                       {"ops"},
	"competitive-analysis":        {"product", "research"},
	"context-search":              {"research"},
	"database":                    {"product"},
	"docker":                      {"ops"},
	"double-diamond":              {"product", "research"},
	"entropy-scan":                {"ops"},
	"experiment":                  {"research", "ops"},
	"frontend-skill":              {"frontend"},
	"frontend-verify":             {"core", "frontend"},
	"git-worktrees":               {"ops"},
	"idea":                        {"product", "research"},
	"lore-commit":                 {"research"},
	"make-interfaces-feel-better": {"core", "frontend"},
	"metrics":                     {"product"},
	"migration":                   {"ops", "product"},
	"monitor-patterns":            {"ops"},
	"performance":                 {"ops"},
	"playwright-cli":              {"frontend"},
	"prd":                         {"product"},
	"product-discovery":           {"product", "research"},
	"security-audit":              {"ops"},
	"writing-skills":              {"research"},
}

func bundlesForSkill(name, category string) []string {
	if bundles, ok := bundleOverrides[name]; ok {
		return bundles
	}
	if coreSkillSet[name] {
		return []string{"core"}
	}

	switch category {
	case "agentic", "methodology", "quality", "testing":
		return []string{"core"}
	case "development", "devops", "security":
		return []string{"ops"}
	case "documentation", "strategy":
		return []string{"research"}
	case "workflow":
		return []string{"product"}
	default:
		return []string{"product"}
	}
}

// claudeOnlySkillSet lists skills that are scoped to the claude-code platform
// only. Their documentation and capabilities (e.g. the deterministic
// `--workflow` route) rely on claude-code-exclusive primitives, so they MUST
// NOT be compiled into codex/gemini/opencode surfaces.
var claudeOnlySkillSet = map[string]bool{
	"harness-workflow": true,
}

func visibilityForSkill(name string) string {
	if claudeOnlySkillSet[name] {
		return SkillVisibilityClaudeOnly
	}
	return SkillVisibilityShared
}

func compileTargetsForSkill(name string) []string {
	if claudeOnlySkillSet[name] {
		return []string{"claude"}
	}
	return []string{"claude", "codex", "gemini", "opencode"}
}

// IsCoreSkill reports whether the canonical skill should remain in shared/core surfaces.
func IsCoreSkill(name string) bool {
	return coreSkillSet[name]
}
