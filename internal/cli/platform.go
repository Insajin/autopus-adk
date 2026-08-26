// Package cli는 platform 커맨드를 구현한다.
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/detect"
)

func newPlatformCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "platform",
		Short: "Manage platforms",
		Long:  "플랫폼 목록을 관리합니다: 목록 조회, 추가, 제거.",
	}

	cmd.PersistentFlags().StringVar(&dir, "dir", "", "프로젝트 루트 디렉터리 (기본값: 현재 디렉터리)")

	cmd.AddCommand(newPlatformListCmd(&dir))
	cmd.AddCommand(newPlatformAddCmd(&dir))
	cmd.AddCommand(newPlatformRemoveCmd(&dir))
	cmd.AddCommand(newPlatformOMPCmd(&dir))

	return cmd
}

func newPlatformListCmd(dir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured and detected platforms",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := resolveProjectDir(*dir)
			if err != nil {
				return err
			}

			cfg, err := config.Load(d)
			if err != nil {
				return fmt.Errorf("설정 로드 실패: %w", err)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "Configured platforms:")
			for _, p := range cfg.Platforms {
				fmt.Fprintf(out, "  - %s\n", p)
			}

			fmt.Fprintln(out, "\nDetected CLIs (in PATH):")
			detected := detect.DetectPlatforms()
			if len(detected) == 0 {
				fmt.Fprintln(out, "  (none)")
			} else {
				for _, p := range detected {
					fmt.Fprintf(out, "  - %s (%s)\n", p.Name, p.Version)
				}
			}

			return nil
		},
	}
}

func newPlatformAddCmd(dir *string) *cobra.Command {
	return newPlatformAddCmdWithLookup(dir, lookupPlatformDescriptor)
}

// @AX:WARN [AUTO]: platform-add contains more than eight conditional branches.
// @AX:REASON [AUTO]: platform normalization, catalog admission, config cloning, provider migration, generation, persistence, and lifecycle-stage errors converge here.
func newPlatformAddCmdWithLookup(dir *string, lookup platformDescriptorLookup) *cobra.Command {
	return &cobra.Command{
		Use:   "add <platform>",
		Short: "Add a platform to autopus.yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := resolveProjectDir(*dir)
			if err != nil {
				return err
			}

			platform := strings.TrimSpace(args[0])
			if normalized := config.ProviderToPlatform(platform); normalized != "" {
				platform = normalized
			}
			descriptor, ok := lookup(platform)
			if !ok {
				return fmt.Errorf("잘못된 플랫폼 %q", platform)
			}

			cfg, err := config.LoadPreview(d)
			if err != nil {
				return fmt.Errorf("설정 로드 실패: %w", err)
			}
			if containsPlatform(cfg.Platforms, platform) {
				fmt.Fprintf(cmd.OutOrStdout(), "플랫폼 %q는 이미 추가되어 있습니다\n", platform)
				return nil
			}

			candidateCfg, err := cloneHarnessConfig(cfg)
			if err != nil {
				return fmt.Errorf("설정 준비 실패: %w", err)
			}
			candidateCfg.Platforms = append(candidateCfg.Platforms, platform)
			providerName := config.PlatformToProvider(platform)
			if providerName != "" {
				if err := config.EnsureOrchestraProvider(candidateCfg, providerName); err != nil {
					return fmt.Errorf("orchestra 설정 갱신 실패: %w", err)
				}
			}

			effectiveCfg := applyFlagCC21Overrides(candidateCfg, globalFlagsFromContext(cmd.Context()))
			descriptor = withPlatformAddOwnershipHandoff(descriptor, effectiveCfg, lookup)
			err = generatePlatformThenSave(
				context.Background(), d, descriptor, effectiveCfg, candidateCfg, config.Save,
			)
			if err != nil {
				if lifecycleErr, ok := err.(*platformLifecycleError); ok && lifecycleErr.stage == platformStageSave {
					return fmt.Errorf("설정 저장 실패: %w", err)
				}
				return fmt.Errorf("%s 파일 생성 실패: %w", platform, err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✓ Platform %q added\n", platform)
			return nil
		},
	}
}

func newPlatformRemoveCmd(dir *string) *cobra.Command {
	return newPlatformRemoveCmdWithLookup(dir, lookupPlatformDescriptor)
}

// @AX:WARN [AUTO]: platform-remove contains more than eight conditional branches.
// @AX:REASON [AUTO]: platform admission, config cloning, manifest cleanup, persistence, and repair-required receipt handling converge here.
func newPlatformRemoveCmdWithLookup(dir *string, lookup platformDescriptorLookup) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <platform>",
		Short: "Remove a platform from autopus.yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := resolveProjectDir(*dir)
			if err != nil {
				return err
			}

			platform := strings.TrimSpace(args[0])
			if normalized := config.ProviderToPlatform(platform); normalized != "" {
				platform = normalized
			}
			descriptor, ok := lookup(platform)
			if !ok {
				return fmt.Errorf("잘못된 플랫폼 %q", platform)
			}

			cfg, err := config.LoadPreview(d)
			if err != nil {
				return fmt.Errorf("설정 로드 실패: %w", err)
			}

			newPlatforms := make([]string, 0, len(cfg.Platforms)-1)
			found := false
			for _, configured := range cfg.Platforms {
				if configured == platform {
					found = true
					continue
				}
				newPlatforms = append(newPlatforms, configured)
			}
			if !found {
				fmt.Fprintf(cmd.OutOrStdout(), "플랫폼 %q를 찾을 수 없습니다\n", platform)
				return nil
			}
			if len(newPlatforms) == 0 {
				return fmt.Errorf("최소 하나의 플랫폼이 필요합니다")
			}

			candidateCfg, err := cloneHarnessConfig(cfg)
			if err != nil {
				return fmt.Errorf("설정 준비 실패: %w", err)
			}
			candidateCfg.Platforms = newPlatforms
			descriptor = withPlatformRemoveOwnershipHandoff(descriptor, candidateCfg, lookup)
			receipt, lifecycleErr := cleanPlatformThenSave(
				context.Background(), d, descriptor, candidateCfg, config.Save,
			)
			if lifecycleErr != nil {
				stage := platformStageClean
				detail, hasDetail := lifecycleErr.(*platformLifecycleError)
				if hasDetail {
					stage = detail.stage
				}
				if stage == platformStageSave && hasDetail && detail.rolledBack {
					return fmt.Errorf("설정 저장 실패(플랫폼 변경 rollback 완료): %w", lifecycleErr)
				}
				if descriptor.repairDetail {
					paths := strings.Join(receipt.changedPaths, ",")
					if stage == platformStageSave || len(receipt.changedPaths) > 0 {
						return fmt.Errorf("%s 파일 정리 실패: %w; repair_required=true changed_paths=[%s]",
							platform, lifecycleErr, paths)
					}
					return fmt.Errorf("%s 파일 정리 사전 검증 실패: %w", platform, lifecycleErr)
				}
				if stage == platformStageSave {
					return fmt.Errorf("설정 저장 실패: %w", lifecycleErr)
				}
				return fmt.Errorf("%s 파일 정리 실패: %w", platform, lifecycleErr)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✓ Platform %q removed\n", platform)
			return nil
		},
	}
}

// resolveDir는 디렉터리 경로를 반환한다. 비어있으면 현재 디렉터리를 반환한다.
func resolveDir(dir string) (string, error) {
	if dir != "" {
		return dir, nil
	}
	d, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("현재 디렉터리를 가져올 수 없음: %w", err)
	}
	return d, nil
}
