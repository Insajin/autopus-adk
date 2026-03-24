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
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/selfupdate"
	"github.com/insajin/autopus-adk/pkg/version"
)

func newUpdateCmd() *cobra.Command {
	var dir string
	var selfFlag, checkOnly, force bool
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
	cmd.Flags().BoolVar(&selfFlag, "self", false, "CLI 바이너리 자체 업데이트")
	cmd.Flags().BoolVar(&checkOnly, "check", false, "업데이트 가능 여부만 확인 (다운로드하지 않음)")
	cmd.Flags().BoolVar(&force, "force", false, "같은 버전이라도 재설치 또는 개발 빌드 업데이트 강제")
	cmd.Flags().StringVar(&targetVersion, "version", "", "특정 버전 설치 (기본값: 최신 버전)")
	return cmd
}

// @AX:WARN: [AUTO] high branch complexity — orchestrates check/download/verify/replace with 7+ conditional paths
// @AX:REASON: consolidates R2–R12 self-update requirements; refactor into sub-steps if complexity grows further
// targetVersion is accepted for future P2 use (pinned version install); currently unused — checker always fetches latest.
func runSelfUpdate(cmd *cobra.Command, checkOnly, force bool, targetVersion string) error {
	_ = targetVersion // P2: reserved for pinned version install via --version flag
	currentVer := version.Version()
	currentCommit := version.Commit()

	// R12: Dev build guard
	if (currentVer == "dev" || currentCommit == "none") && !force {
		return fmt.Errorf("개발 빌드에서는 --force 플래그가 필요합니다")
	}

	// Check latest
	checker := selfupdate.NewChecker()
	info, err := checker.CheckLatest(currentVer)
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

	// Get current binary path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("현재 바이너리 경로를 가져올 수 없음: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("심볼릭 링크 해결 실패: %w", err)
	}

	// Use info from checker, or construct one if targetVersion is set
	if info == nil {
		info = &selfupdate.ReleaseInfo{}
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
