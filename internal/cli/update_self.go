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

// @AX:WARN: [AUTO] Self-update tests replace the package-level temporary-directory factory.
// @AX:REASON: [AUTO] The mutable seam must remain serialized and be restored with t.Cleanup so update staging cannot cross test boundaries.
var makeSelfUpdateTempDir = os.MkdirTemp

type selfUpdateVersionProbe func(string) (string, error)
type selfUpdateBinaryReplace func(string, string) error
type selfUpdateReleaseOperation func(*selfupdate.ReleaseInfo, string) error

// @AX:WARN: [AUTO] runSelfUpdate has at least eight if branches.
// @AX:REASON: [AUTO] Stable-version checks, force and check-only behavior, binary ownership, and signed replacement must fail closed in order.
func runSelfUpdate(cmd *cobra.Command, checkOnly, force bool, targetVersion string) error {
	if targetVersion != "" {
		return unsupportedUpdateVersionError()
	}
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

	if info == nil {
		info, err = checker.FetchLatest(runtime.GOOS, runtime.GOARCH)
		if err != nil {
			return fmt.Errorf("최신 릴리즈 정보 조회 실패: %w", err)
		}
	}
	err = installSelfUpdateRelease(cmd, currentVer, info, pathInfo)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "하네스 파일도 업데이트하려면: auto update\n")
	return nil
}

func installSelfUpdateRelease(
	cmd *cobra.Command,
	currentVersion string,
	info *selfupdate.ReleaseInfo,
	pathInfo binaryPathInfo,
) error {
	return installSelfUpdateReleaseWith(cmd, currentVersion, info, pathInfo)
}

func installFreshUpdateRelease(
	cmd *cobra.Command,
	currentVersion string,
	info *selfupdate.ReleaseInfo,
	pathInfo binaryPathInfo,
) error {
	return installSelfUpdateReleaseWith(cmd, currentVersion, info, pathInfo)
}

func installSelfUpdateReleaseWith(
	cmd *cobra.Command,
	currentVersion string,
	info *selfupdate.ReleaseInfo,
	pathInfo binaryPathInfo,
) error {
	return installSelfUpdateReleaseWithOperation(
		cmd,
		currentVersion,
		info,
		pathInfo,
		downloadVerifyAndReplaceSelfUpdate,
	)
}

func installSelfUpdateReleaseWithOperation(
	cmd *cobra.Command,
	currentVersion string,
	info *selfupdate.ReleaseInfo,
	pathInfo binaryPathInfo,
	replace selfUpdateReleaseOperation,
) error {
	if pathInfo.IsManagerOwned() {
		return managerRequiredUpdateError(pathInfo.ManagedPath())
	}
	execPath := pathInfo.ManagedPath()
	if !isWritable(filepath.Dir(execPath)) {
		return fmt.Errorf(
			"privileged-update-required: %q은 현재 사용자가 안전하게 교체할 수 없습니다; 설치에 사용한 시스템 패키지 관리자나 관리자 관리 도구로 업데이트하세요",
			execPath,
		)
	}
	if info == nil || info.ArchiveURL == "" || info.ChecksumURL == "" {
		tag := ""
		if info != nil {
			tag = info.TagName
		}
		return fmt.Errorf("다운로드 URL을 찾을 수 없음 (tag: %s)", tag)
	}
	if replace == nil {
		return fmt.Errorf("self-update replacement operation is unavailable")
	}

	if err := replace(info, execPath); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "v%s → %s 업데이트 완료\n", currentVersion, info.TagName)
	return nil
}

func downloadVerifyAndReplaceSelfUpdate(info *selfupdate.ReleaseInfo, execPath string) error {
	targetVersion := strings.TrimPrefix(info.TagName, "v")
	archiveName := selfupdate.ArchiveName(runtime.GOOS, runtime.GOARCH, targetVersion)
	err := withSelfUpdateTempDir(func(tmpDir string) error {
		downloader := selfupdate.NewDownloader()
		binaryPath, err := downloader.DownloadAndVerifyWithSignature(
			info.ArchiveURL,
			info.ChecksumURL,
			info.SignatureURL,
			archiveName,
			tmpDir,
		)
		if err != nil {
			return fmt.Errorf("다운로드/검증 실패: %w", err)
		}
		return verifyAndReplaceSelfUpdate(binaryPath, execPath, targetVersion)
	})
	if err != nil {
		return err
	}
	return nil
}

func unsupportedUpdateVersionError() error {
	return fmt.Errorf("auto update --version은 아직 지원되지 않습니다; 최신 stable 릴리스만 사용할 수 있습니다")
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
