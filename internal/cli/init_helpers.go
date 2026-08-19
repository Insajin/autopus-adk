// Package cli provides helper functions for the init command.
package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/internal/cli/tui"
	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/adapter/gemini"
	"github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/adapter/opencode"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/detect"
)

var initSupportedPlatforms = map[string]bool{
	"claude-code":     true,
	"codex":           true,
	"antigravity-cli": true,
	"opencode":        true,
	"omp":             true,
}

var legacyRootGeneratedGitignorePatterns = map[string]string{
	".claude/":     "/.claude/",
	".claude.json": "/.claude.json",
	".codex/":      "/.codex/",
	".gemini/":     "/.gemini/",
	".mcp.json":    "/.mcp.json",
	".opencode/":   "/.opencode/",
	"config.toml":  "/config.toml",
}

// generatePlatformFiles는 플랫폼별 파일을 생성한다.
func generatePlatformFiles(ctx context.Context, dir string, cfg *config.HarnessConfig, cmd *cobra.Command) error {
	effectiveCfg := applyFlagCC21Overrides(cfg, globalFlagsFromContext(cmd.Context()))

	for _, p := range cfg.Platforms {
		var err error
		switch p {
		case "claude-code":
			a := claude.NewWithRoot(dir)
			_, err = a.Generate(ctx, effectiveCfg)
		case "codex":
			a := codex.NewWithRoot(dir)
			_, err = a.Generate(ctx, effectiveCfg)
		case "antigravity-cli":
			a := gemini.NewWithRoot(dir)
			_, err = a.Generate(ctx, effectiveCfg)
		case "opencode":
			a := opencode.NewWithRoot(dir)
			_, err = a.Generate(ctx, effectiveCfg)
		case "omp":
			a := omp.NewWithRoot(dir)
			_, err = a.Generate(ctx, effectiveCfg)
		default:
			tui.Warnf(cmd.OutOrStdout(), "알 수 없는 플랫폼 %q, 건너뜀", p)
			continue
		}
		if err != nil {
			return fmt.Errorf("플랫폼 %q 파일 생성 실패: %w", p, err)
		}
		tui.Success(cmd.OutOrStdout(), p)
	}
	return nil
}

// detectDefaultPlatforms returns installed, ADK-supported platforms in a stable order.
// Falls back to Claude Code when no supported CLI is found in PATH.
func detectDefaultPlatforms() []string {
	detected := detectInstalledPlatforms()
	platforms := make([]string, 0, len(detected))
	platforms = append(platforms, detected...)

	if len(platforms) == 0 {
		return []string{"claude-code"}
	}
	return platforms
}

// detectInstalledPlatforms returns installed, ADK-supported platforms in a stable order.
// Unlike detectDefaultPlatforms, it does not add any fallback platform.
//
// Exec surface: this is not purely passive. detect.DetectInstalledPlatforms runs
// `omp --version` to satisfy the REQ-019 identity gate, so calling it executes
// one binary from PATH. Every other platform is decided by presence alone.
func detectInstalledPlatforms() []string {
	detected := detect.DetectInstalledPlatforms()
	platforms := make([]string, 0, len(detected))
	seen := make(map[string]bool, len(detected))

	for _, p := range detected {
		if !initSupportedPlatforms[p.Name] || seen[p.Name] {
			continue
		}
		platforms = append(platforms, p.Name)
		seen[p.Name] = true
	}
	return platforms
}

// gitignorePatterns는 autopus 관련 .gitignore 패턴 목록이다.
var gitignorePatterns = []string{
	".autopus/*-manifest.json",
	".autopus/context/signatures.md",
	".autopus/plugins/",
	".autopus/orchestra/",
	".autopus/brainstorms/",
	".autopus/txns/",
	".autopus/design/imports/",
	".autopus/design/verify/",
	".autopus/canary/",
	".autopus/backup/",
	".autopus/cache/",
	".autopus/docs/",
	".autopus/qa/runs/",
	".autopus/qa/cache/",
	".autopus/qa/gui/",
	".autopus/qa/feedback/",
	".autopus/qa/evidence/",
	".autopus/qa/releases/",
	".autopus/runtime/",
	".autopus/omp-model-resolution-v1.json",
	".autopus/state.json",
	".autopus/telemetry/",
	".autopus/audit.jsonl",
	"**/.autopus/specs/**/review.md",
	"**/.autopus/specs/**/review-findings.json",
	"**/.autopus/specs/**/.self-verify.log",
	"/.claude/",
	"/.claude.json",
	"/.codex/",
	"/.gemini/",
	"/.opencode/",
	".agents/skills/",
	".agents/plugins/",
	".agents/commands/",
	".agents/hooks.json",
	// Directory form, never a file-name glob. omp discovers rules with a
	// gitignore-aware glob, and measured on omp 17.3.5 a `.omp/rules/autopus-*.md`
	// entry drops all 14 generated rules from the session (domain rules 9 -> 0,
	// TTSR 3 -> 0) while `.omp/rules/` keeps all 14 discoverable. Omitting the
	// entry is not an option either: doctor's hygiene check fails on untracked
	// runtime/generated files left visible to git.
	//
	// A user who wants their own rule in this tree tracked runs
	// `git add -f .omp/rules/mine.md` or keeps the rule outside `.omp/rules/`.
	// A `!` negation cannot re-include a file whose parent directory is excluded.
	".omp/rules/",
	".omp/agents/",
	".omp/config.yml",
	".symphony/artifacts/",
	"/.mcp.json",
	"/config.toml",
}

// updateGitignore는 .gitignore에 autopus 패턴을 추가한다.
func updateGitignore(dir string) error {
	gitignorePath := filepath.Join(dir, ".gitignore")

	var existing string
	if data, err := os.ReadFile(gitignorePath); err == nil {
		existing = string(data)
	}
	existing, migrated := migrateLegacyRootGeneratedGitignorePatterns(existing)
	existingLines := make(map[string]bool)
	for _, line := range strings.Split(existing, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		existingLines[line] = true
	}

	var toAdd []string
	for _, pattern := range gitignorePatterns {
		if !existingLines[pattern] {
			toAdd = append(toAdd, pattern)
		}
	}

	if len(toAdd) == 0 && !migrated {
		return nil
	}

	var sb strings.Builder
	sb.WriteString(existing)
	if len(toAdd) > 0 {
		if existing != "" && !strings.HasSuffix(existing, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("\n# Autopus-ADK generated files\n")
		for _, p := range toAdd {
			sb.WriteString(p)
			sb.WriteString("\n")
		}
	}

	return os.WriteFile(gitignorePath, []byte(sb.String()), 0644)
}

func migrateLegacyRootGeneratedGitignorePatterns(existing string) (string, bool) {
	if existing == "" {
		return existing, false
	}
	lines := strings.Split(existing, "\n")
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if replacement, ok := legacyRootGeneratedGitignorePatterns[trimmed]; ok {
			lines[i] = replacement
			changed = true
		}
	}
	if !changed {
		return existing, false
	}
	return strings.Join(lines, "\n"), true
}
