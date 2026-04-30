// Package cli는 update 커맨드를 구현한다.
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
	"github.com/insajin/autopus-adk/pkg/adapter/opencode"
	"github.com/insajin/autopus-adk/pkg/config"
)

func newUpdateCmd() *cobra.Command {
	var dir string
	var selfFlag, checkOnly, force, yesFlag, previewMode bool
	var targetVersion, statusLine string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update autopus harness files",
		Long:  "설치된 하네스 파일을 업데이트합니다. 사용자 수정 사항을 보존합니다.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// R9: Self-update branch
			if selfFlag {
				return runSelfUpdate(cmd, checkOnly, force, targetVersion)
			}

			if dir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("현재 디렉터리를 가져올 수 없음: %w", err)
				}
				dir = cwd
			}

			var (
				cfg                   *config.HarnessConfig
				err                   error
				platformNamesMigrated bool
			)
			if previewMode {
				cfg, platformNamesMigrated, err = loadConfigForUpdatePreview(dir)
			} else {
				cfg, err = config.Load(dir)
			}
			if err != nil {
				return fmt.Errorf("설정 로드 실패: %w", err)
			}
			configExists := true
			if _, statErr := os.Stat(filepath.Join(dir, "autopus.yaml")); os.IsNotExist(statErr) {
				configExists = false
			} else if statErr != nil {
				return fmt.Errorf("autopus.yaml 확인 실패: %w", statErr)
			}
			designConfigMissing := false
			if configExists {
				var missingErr error
				designConfigMissing, missingErr = config.MissingTopLevelKey(dir, "design")
				if missingErr != nil {
					return fmt.Errorf("design 설정 확인 실패: %w", missingErr)
				}
			}

			if err := validateStatusLineMode(statusLine); err != nil {
				return err
			}

			if previewMode {
				previewCfg, configReasons, cloneErr := prepareUpdatePreviewConfig(
					dir,
					cfg,
					platformNamesMigrated,
					designConfigMissing,
					yesFlag,
					isStdinTTY(),
				)
				if cloneErr != nil {
					return cloneErr
				}

				addedPlatforms := appendDetectedPlatforms(previewCfg)
				if len(addedPlatforms) > 0 {
					configReasons = appendConfigPreviewReason(
						configReasons,
						"detected platforms: "+strings.Join(addedPlatforms, ", "),
					)
				}

				migrated, migrateErr := config.MigrateOrchestraConfig(previewCfg)
				if migrateErr != nil {
					return fmt.Errorf("orchestra 마이그레이션 실패: %w", migrateErr)
				}
				if migrated {
					configReasons = appendConfigPreviewReason(
						configReasons,
						"orchestra config migration would be persisted",
					)
				}

				effectiveCfg := applyFlagCC21Overrides(previewCfg, globalFlagsFromContext(cmd.Context()))
				if _, modeErr := applyStatusLineMode(cmd, dir, effectiveCfg, statusLine, false); modeErr != nil {
					return modeErr
				}
				preview, previewErr := buildUpdatePreview(cmd.Context(), dir, effectiveCfg)
				if previewErr != nil {
					return previewErr
				}
				preview.Items = appendConfigPreviewItem(preview.Items, dir, configReasons)

				printPreview(cmd.OutOrStdout(), "auto update", preview.Hint, preview.Items)
				return nil
			}

			addedPlatforms := appendDetectedPlatforms(cfg)

			// Orchestra config migration
			if changed, migrateErr := config.MigrateOrchestraConfig(cfg); migrateErr != nil {
				return fmt.Errorf("orchestra 마이그레이션 실패: %w", migrateErr)
			} else if changed || len(addedPlatforms) > 0 || (designConfigMissing && cfg.Design.Enabled) {
				if saveErr := config.Save(dir, cfg); saveErr != nil {
					return fmt.Errorf("마이그레이션 설정 저장 실패: %w", saveErr)
				}
			}
			if len(addedPlatforms) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  + 새 플랫폼 감지: %s\n", strings.Join(addedPlatforms, ", "))
			}
			if designConfigMissing && cfg.Design.Enabled {
				fmt.Fprintln(cmd.OutOrStdout(), "  + design defaults added to autopus.yaml")
			}

			// 프로젝트 설정 프롬프트 (미설정 항목만, --yes 시 스킵)
			if !yesFlag {
				promptLanguageSettings(cmd, dir, cfg)
			}
			statusLineSummary, statusLineErr := applyStatusLineMode(cmd, dir, cfg, statusLine, isStdinTTY() && !yesFlag)
			if statusLineErr != nil {
				return statusLineErr
			}
			warnParentRuleConflicts(cmd, dir, cfg, yesFlag)
			if configExists && cfg.Design.Enabled {
				designPath, created, designErr := ensureStarterDesignFile(dir)
				if designErr != nil {
					return fmt.Errorf("DESIGN.md 생성 실패: %w", designErr)
				}
				if created {
					fmt.Fprintf(cmd.OutOrStdout(), "  + created %s\n", filepath.Base(designPath))
				}
			}

			ctx := context.Background()
			effectiveCfg := applyFlagCC21Overrides(cfg, globalFlagsFromContext(cmd.Context()))
			updated := 0

			for _, p := range cfg.Platforms {
				var updateErr error
				switch p {
				case "claude-code":
					a := claude.NewWithRoot(dir)
					_, updateErr = a.Update(ctx, effectiveCfg)
				case "codex":
					a := codex.NewWithRoot(dir)
					_, updateErr = a.Update(ctx, effectiveCfg)
				case "gemini-cli":
					a := gemini.NewWithRoot(dir)
					_, updateErr = a.Update(ctx, effectiveCfg)
				case "opencode":
					a := opencode.NewWithRoot(dir)
					_, updateErr = a.Update(ctx, effectiveCfg)
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

			if statusLineSummary != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  • %s\n", statusLineSummary)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Update complete: %d platform(s) updated\n", updated)
			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "프로젝트 루트 디렉터리 (기본값: 현재 디렉터리)")
	cmd.Flags().BoolVar(&selfFlag, "self", false, "CLI 바이너리 자체 업데이트")
	cmd.Flags().BoolVar(&checkOnly, "check", false, "업데이트 가능 여부만 확인 (다운로드하지 않음)")
	cmd.Flags().BoolVar(&force, "force", false, "같은 버전이라도 재설치 또는 개발 빌드 업데이트 강제")
	cmd.Flags().StringVar(&targetVersion, "version", "", "특정 버전 설치 (기본값: 최신 버전)")
	cmd.Flags().BoolVarP(&yesFlag, "yes", "y", false, "모든 프롬프트를 기본값으로 자동 수락")
	cmd.Flags().StringVar(&statusLine, "statusline-mode", "", "Claude statusLine handling: keep, merge, replace")
	cmd.Flags().BoolVar(&previewMode, "plan", false, "변경 예정 파일만 계산하고 쓰지 않음")
	cmd.Flags().BoolVar(&previewMode, "preview", false, "변경 예정 파일만 계산하고 쓰지 않음")
	cmd.Flags().BoolVar(&previewMode, "dry-run", false, "변경 예정 파일만 계산하고 쓰지 않음")
	return cmd
}

func appendDetectedPlatforms(cfg *config.HarnessConfig) []string {
	var added []string
	for _, platform := range detectInstalledPlatforms() {
		if containsPlatform(cfg.Platforms, platform) {
			continue
		}
		cfg.Platforms = append(cfg.Platforms, platform)
		added = append(added, platform)
	}
	return added
}
