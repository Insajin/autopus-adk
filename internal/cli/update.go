// Package cli는 update 커맨드를 구현한다.
package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/adapter/gemini"
	"github.com/insajin/autopus-adk/pkg/adapter/opencode"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/selfupdate"
	"github.com/insajin/autopus-adk/pkg/version"
)

func newUpdateCmd() *cobra.Command {
	var dir string
	var selfFlag, checkOnly, force, yesFlag bool
	var targetVersion string

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

			// 설정 로드
			cfg, err := config.Load(dir)
			if err != nil {
				return fmt.Errorf("설정 로드 실패: %w", err)
			}

			addedPlatforms := appendDetectedPlatforms(cfg)

			// Orchestra config migration
			if changed, migrateErr := config.MigrateOrchestraConfig(cfg); migrateErr != nil {
				return fmt.Errorf("orchestra 마이그레이션 실패: %w", migrateErr)
			} else if changed || len(addedPlatforms) > 0 {
				if saveErr := config.Save(dir, cfg); saveErr != nil {
					return fmt.Errorf("마이그레이션 설정 저장 실패: %w", saveErr)
				}
			}
			if len(addedPlatforms) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  + 새 플랫폼 감지: %s\n", strings.Join(addedPlatforms, ", "))
			}

			// 프로젝트 설정 프롬프트 (미설정 항목만, --yes 시 스킵)
			if !yesFlag {
				promptLanguageSettings(cmd, dir, cfg)
			}
			warnParentRuleConflicts(cmd, dir, cfg, yesFlag)

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
				case "opencode":
					a := opencode.NewWithRoot(dir)
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
	cmd.Flags().BoolVar(&selfFlag, "self", false, "CLI 바이너리 자체 업데이트")
	cmd.Flags().BoolVar(&checkOnly, "check", false, "업데이트 가능 여부만 확인 (다운로드하지 않음)")
	cmd.Flags().BoolVar(&force, "force", false, "같은 버전이라도 재설치 또는 개발 빌드 업데이트 강제")
	cmd.Flags().StringVar(&targetVersion, "version", "", "특정 버전 설치 (기본값: 최신 버전)")
	cmd.Flags().BoolVarP(&yesFlag, "yes", "y", false, "모든 프롬프트를 기본값으로 자동 수락")
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

// @AX:NOTE: [AUTO] linear guard-clause pattern with 7 steps (R2-R12) — complexity is managed via early returns; no refactor needed unless new steps are added
// targetVersion is accepted for future P2 use (pinned version install); currently unused — checker always fetches latest.
func runSelfUpdate(cmd *cobra.Command, checkOnly, force bool, targetVersion string) error {
	_ = targetVersion // P2: reserved for pinned version install via --version flag
	rawVer := strings.TrimPrefix(version.Version(), "v")
	currentCommit := version.Commit()

	// R12: Dev build guard
	if (rawVer == "dev" || currentCommit == "none") && !force {
		return fmt.Errorf("개발 빌드에서는 --force 플래그가 필요합니다")
	}

	// Strip pseudo-version suffixes for clean semver comparison.
	// e.g., "0.21.2-0.20260328130835-dd328b13c758+dirty" → "0.21.2"
	currentVer := rawVer
	if idx := strings.IndexByte(currentVer, '-'); idx != -1 {
		currentVer = currentVer[:idx]
	}
	if idx := strings.IndexByte(currentVer, '+'); idx != -1 {
		currentVer = currentVer[:idx]
	}

	// Pseudo-version or dirty build: always force update to get clean goreleaser binary
	isPseudo := rawVer != currentVer
	if isPseudo {
		force = true
	}

	// Check latest
	checker := selfupdate.NewChecker()
	info, err := checker.CheckLatest(currentVer, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return fmt.Errorf("업데이트 확인 실패: %w", err)
	}

	// R7: Already up to date
	if info == nil && !force {
		fmt.Fprintf(cmd.OutOrStdout(), "이미 최신 버전입니다 (v%s)\n", currentVer)
		return nil
	}

	// R10: Check-only mode
	if checkOnly {
		if info != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "업데이트 가능: v%s → %s\n", currentVer, info.TagName)
		}
		return nil
	}

	pathInfo, err := resolveCurrentBinaryPath()
	if err != nil {
		return err
	}
	execPath := pathInfo.ManagedPath()

	// R13: Check write permission — re-exec with sudo if needed
	if !isWritable(filepath.Dir(execPath)) {
		return reExecWithSudo()
	}

	// Force mode with no newer version: re-fetch latest release info
	if info == nil {
		info, err = checker.FetchLatest(runtime.GOOS, runtime.GOARCH)
		if err != nil {
			return fmt.Errorf("최신 릴리즈 정보 조회 실패: %w", err)
		}
	}

	// Validate download URLs before proceeding
	if info.ArchiveURL == "" || info.ChecksumURL == "" {
		return fmt.Errorf("다운로드 URL을 찾을 수 없음 (tag: %s)", info.TagName)
	}

	// R2: Download archive
	ver := strings.TrimPrefix(info.TagName, "v")
	archiveName := selfupdate.ArchiveName(runtime.GOOS, runtime.GOARCH, ver)
	dl := selfupdate.NewDownloader()
	tmpDir, _ := os.MkdirTemp("", "autopus-update-*")
	defer os.RemoveAll(tmpDir)

	// R3: Download and verify checksum
	binaryPath, err := dl.DownloadAndVerify(info.ArchiveURL, info.ChecksumURL, archiveName, tmpDir)
	if err != nil {
		return fmt.Errorf("다운로드/검증 실패: %w", err)
	}

	// R4: Atomic replace
	replacer := selfupdate.NewReplacer()
	if err := replacer.Replace(binaryPath, execPath); err != nil {
		return err
	}

	// R5: Display result
	fmt.Fprintf(cmd.OutOrStdout(), "v%s → %s 업데이트 완료\n", currentVer, info.TagName)
	fmt.Fprintf(cmd.OutOrStdout(), "하네스 파일도 업데이트하려면: auto update\n")
	return nil
}
