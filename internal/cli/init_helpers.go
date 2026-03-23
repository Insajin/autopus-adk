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
	"github.com/insajin/autopus-adk/pkg/config"
)

// generatePlatformFiles는 플랫폼별 파일을 생성한다.
func generatePlatformFiles(ctx context.Context, dir string, cfg *config.HarnessConfig, cmd *cobra.Command) error {
	for _, p := range cfg.Platforms {
		var err error
		switch p {
		case "claude-code":
			a := claude.NewWithRoot(dir)
			_, err = a.Generate(ctx, cfg)
		case "codex":
			a := codex.NewWithRoot(dir)
			_, err = a.Generate(ctx, cfg)
		case "gemini-cli":
			a := gemini.NewWithRoot(dir)
			_, err = a.Generate(ctx, cfg)
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

// updateGitignore는 .gitignore에 autopus 패턴을 추가한다.
func updateGitignore(dir string) error {
	gitignorePath := filepath.Join(dir, ".gitignore")

	var existing string
	if data, err := os.ReadFile(gitignorePath); err == nil {
		existing = string(data)
	}

	var toAdd []string
	for _, pattern := range gitignorePatterns {
		if !strings.Contains(existing, pattern) {
			toAdd = append(toAdd, pattern)
		}
	}

	if len(toAdd) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString(existing)
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString("\n# Autopus-ADK generated files\n")
	for _, p := range toAdd {
		sb.WriteString(p)
		sb.WriteString("\n")
	}

	return os.WriteFile(gitignorePath, []byte(sb.String()), 0644)
}
