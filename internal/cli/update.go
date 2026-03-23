// Package cli는 update 커맨드를 구현한다.
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/adapter/gemini"
	"github.com/insajin/autopus-adk/pkg/config"
)

func newUpdateCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update autopus harness files",
		Long:  "설치된 하네스 파일을 업데이트합니다. 사용자 수정 사항을 보존합니다.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				var err error
				dir, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("현재 디렉터리를 가져올 수 없음: %w", err)
				}
			}

			// 설정 로드
			cfg, err := config.Load(dir)
			if err != nil {
				return fmt.Errorf("설정 로드 실패: %w", err)
			}

			// Orchestra config migration
			if changed, migrateErr := config.MigrateOrchestraConfig(cfg); migrateErr != nil {
				return fmt.Errorf("orchestra 마이그레이션 실패: %w", migrateErr)
			} else if changed {
				if saveErr := config.Save(dir, cfg); saveErr != nil {
					return fmt.Errorf("마이그레이션 설정 저장 실패: %w", saveErr)
				}
			}

			// 프로젝트 설정 프롬프트 (미설정 항목만)
			promptLanguageSettings(cmd, dir, cfg)
			warnParentRuleConflicts(cmd, dir, cfg)

			ctx := context.Background()
			updated := 0

			for _, p := range cfg.Platforms {
				var updateErr error
				switch p {
				case "claude-code":
					a := claude.NewWithRoot(dir)
					_, updateErr = a.Update(ctx, cfg)
				case "codex":
					a := codex.NewWithRoot(dir)
					_, updateErr = a.Update(ctx, cfg)
				case "gemini-cli":
					a := gemini.NewWithRoot(dir)
					_, updateErr = a.Update(ctx, cfg)
				default:
					fmt.Fprintf(cmd.OutOrStdout(), "  경고: 알 수 없는 플랫폼 %q, 건너뜀\n", p)
					continue
				}
				if updateErr != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %s: %v\n", p, updateErr)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s updated\n", p)
					updated++
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Update complete: %d platform(s) updated\n", updated)
			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "프로젝트 루트 디렉터리 (기본값: 현재 디렉터리)")
	return cmd
}
