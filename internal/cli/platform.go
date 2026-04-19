// Package cli는 platform 커맨드를 구현한다.
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/adapter/gemini"
	"github.com/insajin/autopus-adk/pkg/adapter/opencode"
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

	return cmd
}

func newPlatformListCmd(dir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured and detected platforms",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := resolveDir(*dir)
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
	return &cobra.Command{
		Use:   "add <platform>",
		Short: "Add a platform to autopus.yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := resolveDir(*dir)
			if err != nil {
				return err
			}

			platform := strings.TrimSpace(args[0])

			// 유효한 플랫폼 확인 (임시 config로 검증)
			testCfg := &config.HarnessConfig{
				Mode:        config.ModeFull,
				ProjectName: "test",
				Platforms:   []string{platform},
			}
			if err := testCfg.Validate(); err != nil {
				return fmt.Errorf("잘못된 플랫폼 %q: %w", platform, err)
			}

			cfg, err := config.Load(d)
			if err != nil {
				return fmt.Errorf("설정 로드 실패: %w", err)
			}

			// 이미 존재하는지 확인
			for _, p := range cfg.Platforms {
				if p == platform {
					fmt.Fprintf(cmd.OutOrStdout(), "플랫폼 %q는 이미 추가되어 있습니다\n", platform)
					return nil
				}
			}

			cfg.Platforms = append(cfg.Platforms, platform)

			// Update orchestra config for the new platform
			providerName := config.PlatformToProvider(platform)
			if providerName == "" {
				providerName = platform
			}
			if err := config.EnsureOrchestraProvider(cfg, providerName); err != nil {
				return fmt.Errorf("orchestra 설정 갱신 실패: %w", err)
			}

			if err := config.Save(d, cfg); err != nil {
				return fmt.Errorf("설정 저장 실패: %w", err)
			}

			// 플랫폼 파일 생성
			ctx := context.Background()
			effectiveCfg := applyFlagCC21Overrides(cfg, globalFlagsFromContext(cmd.Context()))
			switch platform {
			case "claude-code":
				a := claude.NewWithRoot(d)
				if _, err := a.Generate(ctx, effectiveCfg); err != nil {
					return fmt.Errorf("claude-code 파일 생성 실패: %w", err)
				}
			case "codex":
				a := codex.NewWithRoot(d)
				if _, err := a.Generate(ctx, effectiveCfg); err != nil {
					return fmt.Errorf("codex 파일 생성 실패: %w", err)
				}
			case "gemini-cli":
				a := gemini.NewWithRoot(d)
				if _, err := a.Generate(ctx, effectiveCfg); err != nil {
					return fmt.Errorf("gemini-cli 파일 생성 실패: %w", err)
				}
			case "opencode":
				a := opencode.NewWithRoot(d)
				if _, err := a.Generate(ctx, effectiveCfg); err != nil {
					return fmt.Errorf("opencode 파일 생성 실패: %w", err)
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✓ Platform %q added\n", platform)
			return nil
		},
	}
}

func newPlatformRemoveCmd(dir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <platform>",
		Short: "Remove a platform from autopus.yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := resolveDir(*dir)
			if err != nil {
				return err
			}

			platform := strings.TrimSpace(args[0])

			cfg, err := config.Load(d)
			if err != nil {
				return fmt.Errorf("설정 로드 실패: %w", err)
			}

			// 플랫폼 제거
			var newPlatforms []string
			found := false
			for _, p := range cfg.Platforms {
				if p == platform {
					found = true
					continue
				}
				newPlatforms = append(newPlatforms, p)
			}

			if !found {
				fmt.Fprintf(cmd.OutOrStdout(), "플랫폼 %q를 찾을 수 없습니다\n", platform)
				return nil
			}

			if len(newPlatforms) == 0 {
				return fmt.Errorf("최소 하나의 플랫폼이 필요합니다")
			}

			cfg.Platforms = newPlatforms
			if err := config.Save(d, cfg); err != nil {
				return fmt.Errorf("설정 저장 실패: %w", err)
			}

			// 플랫폼 파일 정리
			ctx := context.Background()
			switch platform {
			case "claude-code":
				a := claude.NewWithRoot(d)
				_ = a.Clean(ctx) // 정리 실패는 무시
			case "codex":
				a := codex.NewWithRoot(d)
				_ = a.Clean(ctx)
			case "gemini-cli":
				a := gemini.NewWithRoot(d)
				_ = a.Clean(ctx)
			case "opencode":
				a := opencode.NewWithRoot(d)
				_ = a.Clean(ctx)
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
