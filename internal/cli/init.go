// Package cli는 init 커맨드를 구현한다.
package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/adapter/gemini"
	"github.com/insajin/autopus-adk/pkg/config"
)

// gitignorePatterns는 autopus 관련 .gitignore 패턴 목록이다.
var gitignorePatterns = []string{
	".claude/rules/autopus/",
	".claude/skills/autopus/",
	".claude/commands/auto.md",
	".claude/agents/autopus/",
	".codex/skills/",
	".gemini/skills/autopus/",
	".agents/skills/",
	".autopus/manifest.json",
	".autopus/*-manifest.json",
	".autopus/backup/",
}

func newInitCmd() *cobra.Command {
	var (
		fullMode  bool
		liteMode  bool
		dir       string
		project   string
		platforms string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize autopus harness in the current project",
		Long:  "코딩 CLI에 Autopus 하네스를 설치합니다. autopus.yaml을 생성하고 플랫폼 파일을 설치합니다.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 디렉터리 결정
			if dir == "" {
				var err error
				dir, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("현재 디렉터리를 가져올 수 없음: %w", err)
				}
			}

			// 모드 결정 (기본값: lite)
			mode := config.ModeLite
			if fullMode {
				mode = config.ModeFull
			}

			// 프로젝트 이름 결정
			if project == "" {
				project = filepath.Base(dir)
			}

			// 플랫폼 목록 파싱
			var platformList []string
			if platforms != "" {
				for _, p := range strings.Split(platforms, ",") {
					p = strings.TrimSpace(p)
					if p != "" {
						platformList = append(platformList, p)
					}
				}
			}
			if len(platformList) == 0 {
				platformList = []string{"claude-code"}
			}

			// 설정 생성
			var cfg *config.HarnessConfig
			if mode == config.ModeFull {
				cfg = config.DefaultFullConfig(project)
			} else {
				cfg = config.DefaultLiteConfig(project)
			}
			cfg.Platforms = platformList

			// autopus.yaml 저장
			if err := config.Save(dir, cfg); err != nil {
				return fmt.Errorf("autopus.yaml 저장 실패: %w", err)
			}

			// 플랫폼별 파일 생성
			ctx := context.Background()
			if err := generatePlatformFiles(ctx, dir, cfg, cmd); err != nil {
				return err
			}

			// .gitignore 업데이트
			if err := updateGitignore(dir); err != nil {
				return fmt.Errorf(".gitignore 업데이트 실패: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✓ Autopus harness initialized (%s mode)\n", mode)
			fmt.Fprintf(cmd.OutOrStdout(), "  Platforms: %s\n", strings.Join(platformList, ", "))
			fmt.Fprintf(cmd.OutOrStdout(), "\n  PATH 확인: auto 바이너리가 PATH에 포함되어야 합니다.\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  코딩 CLI에서 /plan, /go 등의 명령어가 auto CLI를 호출합니다.\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  확인: which auto\n")
			return nil
		},
	}

	cmd.Flags().BoolVar(&fullMode, "full", false, "Full 모드로 초기화")
	cmd.Flags().BoolVar(&liteMode, "lite", false, "Lite 모드로 초기화 (기본값)")
	cmd.Flags().StringVar(&dir, "dir", "", "프로젝트 루트 디렉터리 (기본값: 현재 디렉터리)")
	cmd.Flags().StringVar(&project, "project", "", "프로젝트 이름")
	cmd.Flags().StringVar(&platforms, "platforms", "", "설치할 플랫폼 목록 (쉼표 구분, 예: claude-code,codex)")

	_ = liteMode // --lite 플래그는 참고용 (기본값이 lite이므로)

	return cmd
}

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
			fmt.Fprintf(cmd.OutOrStdout(), "  경고: 알 수 없는 플랫폼 %q, 건너뜀\n", p)
			continue
		}
		if err != nil {
			return fmt.Errorf("플랫폼 %q 파일 생성 실패: %w", p, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s\n", p)
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
