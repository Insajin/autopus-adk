package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/processprobe"
	"github.com/insajin/autopus-adk/pkg/selfupdate"
	"github.com/insajin/autopus-adk/pkg/version"
)

const (
	selfUpdateVersionProbeTimeout   = 15 * time.Second
	selfUpdateVersionOutputMaxBytes = 4 << 10
)

var makeSelfUpdateTempDir = os.MkdirTemp

type selfUpdateVersionProbe func(string) (string, error)
type selfUpdateBinaryReplace func(string, string) error

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
	err = withSelfUpdateTempDir(func(tmpDir string) error {
		dl := selfupdate.NewDownloader()
		binaryPath, err := dl.DownloadAndVerifyWithSignature(
			info.ArchiveURL,
			info.ChecksumURL,
			info.SignatureURL,
			archiveName,
			tmpDir,
		)
		if err != nil {
			return fmt.Errorf("다운로드/검증 실패: %w", err)
		}
		return verifyAndReplaceSelfUpdate(binaryPath, execPath, ver)
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "v%s → %s 업데이트 완료\n", currentVer, info.TagName)
	fmt.Fprintf(cmd.OutOrStdout(), "하네스 파일도 업데이트하려면: auto update\n")
	return nil
}

func withSelfUpdateTempDir(run func(string) error) error {
	tmpDir, err := makeSelfUpdateTempDir("", "autopus-update-*")
	if err != nil {
		return fmt.Errorf("self-update 임시 디렉터리 생성 실패: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Printf("[update] cleanup tmp dir failed: %v", err)
		}
	}()
	return run(tmpDir)
}

func verifyAndReplaceSelfUpdate(binaryPath, targetPath, targetVersion string) error {
	return verifyAndReplaceSelfUpdateWith(
		binaryPath,
		targetPath,
		targetVersion,
		probeStagedSelfUpdateVersion,
		func(source, target string) error {
			return selfupdate.NewReplacer().Replace(source, target)
		},
	)
}

func verifyAndReplaceSelfUpdateWith(
	binaryPath, targetPath, targetVersion string,
	probe selfUpdateVersionProbe,
	replace selfUpdateBinaryReplace,
) error {
	actualVersion, err := probe(binaryPath)
	if err != nil {
		return fmt.Errorf("새 바이너리 실행 검증 실패: %w", err)
	}
	if actualVersion != targetVersion {
		return fmt.Errorf(
			"새 바이너리 버전 불일치: expected %q, got %q",
			targetVersion,
			actualVersion,
		)
	}
	return replace(binaryPath, targetPath)
}

func probeStagedSelfUpdateVersion(binaryPath string) (string, error) {
	return probeStagedSelfUpdateVersionWithTimeout(binaryPath, selfUpdateVersionProbeTimeout)
}

func probeStagedSelfUpdateVersionWithTimeout(binaryPath string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath, "version", "--short") //nolint:gosec // verified archive path, passed without a shell
	out, err := processprobe.OutputLimited(cmd, selfUpdateVersionOutputMaxBytes)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", err
	}
	return normalizeSelfUpdateVersionOutput(out)
}

func normalizeSelfUpdateVersionOutput(output []byte) (string, error) {
	value := string(output)
	if strings.HasSuffix(value, "\r\n") {
		value = strings.TrimSuffix(value, "\r\n")
	} else {
		value = strings.TrimSuffix(value, "\n")
	}
	if value == "" || strings.ContainsAny(value, "\r\n") || strings.TrimSpace(value) != value {
		return "", fmt.Errorf("새 바이너리가 유효한 단일 버전을 출력하지 않음")
	}
	return value, nil
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
