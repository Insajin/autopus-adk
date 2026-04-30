package cli

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/selfupdate"
	"github.com/insajin/autopus-adk/pkg/version"
)

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

	currentVer := trimPseudoVersion(rawVer)
	if rawVer != currentVer {
		force = true
	}

	checker := selfupdate.NewChecker()
	info, err := checker.CheckLatest(currentVer, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return fmt.Errorf("업데이트 확인 실패: %w", err)
	}

	if info == nil && !force {
		fmt.Fprintf(cmd.OutOrStdout(), "이미 최신 버전입니다 (v%s)\n", currentVer)
		return nil
	}

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
	if !isWritable(filepath.Dir(execPath)) {
		return reExecWithSudo()
	}

	if info == nil {
		info, err = checker.FetchLatest(runtime.GOOS, runtime.GOARCH)
		if err != nil {
			return fmt.Errorf("최신 릴리즈 정보 조회 실패: %w", err)
		}
	}
	if info.ArchiveURL == "" || info.ChecksumURL == "" {
		return fmt.Errorf("다운로드 URL을 찾을 수 없음 (tag: %s)", info.TagName)
	}

	ver := strings.TrimPrefix(info.TagName, "v")
	archiveName := selfupdate.ArchiveName(runtime.GOOS, runtime.GOARCH, ver)
	dl := selfupdate.NewDownloader()
	tmpDir, _ := os.MkdirTemp("", "autopus-update-*")
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Printf("[update] cleanup tmp dir failed: %v", err)
		}
	}()

	binaryPath, err := dl.DownloadAndVerify(info.ArchiveURL, info.ChecksumURL, archiveName, tmpDir)
	if err != nil {
		return fmt.Errorf("다운로드/검증 실패: %w", err)
	}

	replacer := selfupdate.NewReplacer()
	if err := replacer.Replace(binaryPath, execPath); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "v%s → %s 업데이트 완료\n", currentVer, info.TagName)
	fmt.Fprintf(cmd.OutOrStdout(), "하네스 파일도 업데이트하려면: auto update\n")
	return nil
}

func trimPseudoVersion(rawVer string) string {
	currentVer := rawVer
	if idx := strings.IndexByte(currentVer, '-'); idx != -1 {
		currentVer = currentVer[:idx]
	}
	if idx := strings.IndexByte(currentVer, '+'); idx != -1 {
		currentVer = currentVer[:idx]
	}
	return currentVer
}
