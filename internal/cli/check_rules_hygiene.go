package cli

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/internal/cli/tui"
	"github.com/insajin/autopus-adk/pkg/workflow"
)

var hygieneManifestSourcePrefixes = []string{
	"content/",
	"pkg/adapter/",
	"pkg/content/",
	"pkg/setup/",
	"templates/",
}

var hygieneAlwaysBlockPrefixes = []string{
	".autopus/brainstorms/",
	".autopus/orchestra/",
	".autopus/txns/",
}

var hygieneAlwaysBlockExactPaths = map[string]bool{
	".autopus/context/signatures.md": true,
	"config.toml":                    true,
}

func checkHygiene(dir string, out io.Writer, quiet bool) bool {
	if !quiet {
		tui.SectionHeader(out, "hygiene: generated/runtime drift")
	}

	staged, err := stagedGitPaths(dir, "ACMRD")
	if err != nil {
		if !quiet {
			tui.Info(out, "not a git worktree or no staged index, skipping hygiene check")
		}
		return true
	}
	if len(staged) == 0 {
		if !quiet {
			tui.OK(out, "no staged files")
		}
		return true
	}

	drift := blockedGeneratedDrift(staged)
	if len(drift) == 0 {
		if !quiet {
			tui.OK(out, "staged generated/runtime drift clear")
		}
		return true
	}

	for _, rel := range drift {
		tui.FAIL(out, fmt.Sprintf("%s (generated/runtime drift without source-of-truth change)", rel))
	}
	return false
}

func blockedGeneratedDrift(staged []string) []string {
	candidates := workflow.DetectGeneratedDrift(staged, false)
	var blocked []string
	for _, rel := range candidates {
		if !hasMatchingStagedSourceOfTruth(rel, staged) {
			blocked = append(blocked, rel)
		}
	}
	return blocked
}

func stagedGitPaths(dir, diffFilter string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--name-only", "--diff-filter="+diffFilter)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var paths []string
	for _, rel := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		rel = strings.TrimSpace(rel)
		if rel != "" {
			paths = append(paths, filepath.ToSlash(rel))
		}
	}
	return paths, nil
}

func hasMatchingStagedSourceOfTruth(generated string, staged []string) bool {
	cleanGenerated := normalizeGitRel(generated)
	if hygieneAlwaysBlockExactPaths[cleanGenerated] {
		return false
	}
	for _, prefix := range hygieneAlwaysBlockPrefixes {
		if strings.HasPrefix(cleanGenerated, prefix) {
			return false
		}
	}

	for _, rel := range staged {
		cleanSource := normalizeGitRel(rel)
		if cleanSource == cleanGenerated {
			continue
		}
		if sourceMatchesGenerated(cleanGenerated, cleanSource) {
			return true
		}
	}
	return false
}

func sourceMatchesGenerated(generated, source string) bool {
	if isRootAutopusManifestPath(generated) {
		return hasAnyPrefix(source, hygieneManifestSourcePrefixes)
	}
	return hasAnyPrefix(source, sourcePrefixesForGenerated(generated))
}

func sourcePrefixesForGenerated(generated string) []string {
	switch {
	case strings.HasPrefix(generated, ".codex/agents/"):
		return []string{"content/agents/", "pkg/adapter/codex/codex_agents", "pkg/content/agent_transformer", "templates/codex/agents/"}
	case strings.HasPrefix(generated, ".codex/skills/"):
		return []string{"content/skills/", "pkg/adapter/codex/codex_skill", "pkg/content/skill_", "templates/codex/skills/"}
	case strings.HasPrefix(generated, ".codex/rules/"):
		return []string{"content/rules/", "pkg/adapter/codex/codex_rules", "templates/codex/rules/"}
	case strings.HasPrefix(generated, ".codex/prompts/"):
		return []string{"templates/codex/prompts/"}
	case generated == ".codex/hooks.json":
		return []string{"content/hooks/", "pkg/content/hooks", "templates/codex/hooks"}
	case generated == ".codex/config.toml":
		return []string{"pkg/adapter/codex/", "templates/codex/config.toml.tmpl"}
	case strings.HasPrefix(generated, ".claude/agents/"):
		return []string{"content/agents/", "pkg/adapter/claude/", "pkg/content/agent_transformer", "templates/claude/agents/"}
	case strings.HasPrefix(generated, ".claude/skills/"):
		return []string{"content/skills/", "pkg/adapter/claude/", "pkg/content/skill_", "templates/claude/skills/"}
	case strings.HasPrefix(generated, ".claude/rules/"):
		return []string{"content/rules/", "pkg/adapter/claude/", "templates/claude/rules/"}
	case strings.HasPrefix(generated, ".claude/commands/"):
		return []string{"templates/claude/commands/"}
	case strings.HasPrefix(generated, ".claude/workflows/"):
		return []string{"content/workflows/", "pkg/content/workflow_", "templates/claude/workflows/"}
	case strings.HasPrefix(generated, ".claude/hooks/"):
		return []string{"content/hooks/", "pkg/content/hooks", "templates/hooks/"}
	case strings.HasPrefix(generated, ".gemini/agents/"):
		return []string{"content/agents/", "pkg/adapter/gemini/gemini_agents", "pkg/content/agent_transformer", "templates/gemini/agents/"}
	case strings.HasPrefix(generated, ".gemini/skills/"):
		return []string{"content/skills/", "pkg/adapter/gemini/gemini_skills", "pkg/content/skill_", "templates/gemini/skills/"}
	case strings.HasPrefix(generated, ".gemini/rules/"):
		return []string{"content/rules/", "pkg/adapter/gemini/gemini_rules", "templates/gemini/rules/"}
	case strings.HasPrefix(generated, ".gemini/commands/"):
		return []string{"templates/gemini/commands/"}
	case generated == ".gemini/settings.json":
		return []string{"pkg/adapter/gemini/", "templates/gemini/settings/"}
	case strings.HasPrefix(generated, ".opencode/agents/"):
		return []string{"content/agents/", "pkg/adapter/opencode/", "pkg/content/agent_transformer", "templates/opencode/agents/"}
	case strings.HasPrefix(generated, ".opencode/skills/") || strings.HasPrefix(generated, ".opencode/commands/"):
		return []string{"content/skills/", "pkg/adapter/opencode/", "pkg/content/skill_", "templates/opencode/"}
	case strings.HasPrefix(generated, ".opencode/rules/"):
		return []string{"content/rules/", "pkg/adapter/opencode/", "templates/opencode/rules/"}
	case strings.HasPrefix(generated, ".opencode/plugins/"):
		return []string{"content/hooks/", "pkg/adapter/opencode/", "pkg/content/hooks", "templates/opencode/plugins/"}
	case strings.HasPrefix(generated, ".agents/plugins/") || strings.HasPrefix(generated, ".autopus/plugins/"):
		return []string{"content/", "pkg/adapter/", "pkg/content/", "pkg/setup/", "templates/"}
	default:
		return nil
	}
}

func hasAnyPrefix(rel string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(rel, prefix) {
			return true
		}
	}
	return false
}

func isRootAutopusManifestPath(rel string) bool {
	return strings.HasPrefix(rel, ".autopus/") &&
		path.Dir(rel) == ".autopus" &&
		strings.HasSuffix(path.Base(rel), "-manifest.json")
}

func normalizeGitRel(rel string) string {
	return strings.TrimPrefix(path.Clean(filepath.ToSlash(rel)), "./")
}
