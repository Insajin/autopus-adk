package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type binaryPathInfo struct {
	ExecutablePath string
	ResolvedPath   string
}

// @AX:WARN: [AUTO] Binary-path tests replace these package-level resolution hooks.
// @AX:REASON: [AUTO] The mutable seams must remain serialized and be restored with t.Cleanup to prevent cross-test ownership leakage.
var (
	currentExecutablePath = os.Executable
	evalBinarySymlinks    = filepath.EvalSymlinks
)

// @AX:ANCHOR: [AUTO] canonical current-binary ownership boundary.
// @AX:REASON: [AUTO] Root commands, worker lifecycle, self-update, and ownership tests share the resolved executable identity returned here.
func resolveCurrentBinaryPath() (binaryPathInfo, error) {
	execPath, err := currentExecutablePath()
	if err != nil {
		return binaryPathInfo{}, fmt.Errorf("현재 바이너리 경로를 가져올 수 없음: %w", err)
	}

	resolvedPath, err := evalBinarySymlinks(execPath)
	if err != nil {
		return binaryPathInfo{}, fmt.Errorf("현재 바이너리 경로를 안전하게 해석할 수 없음: %w", err)
	}

	return binaryPathInfo{
		ExecutablePath: execPath,
		ResolvedPath:   resolvedPath,
	}, nil
}

func (info binaryPathInfo) ManagedPath() string {
	if info.ResolvedPath != "" {
		return info.ResolvedPath
	}
	return info.ExecutablePath
}

func (info binaryPathInfo) IsSymlinked() bool {
	return info.ExecutablePath != "" && info.ExecutablePath != info.ManagedPath()
}

func (info binaryPathInfo) IsManagerOwned() bool {
	return isManagerOwnedBinaryPath(info.ExecutablePath) ||
		isManagerOwnedBinaryPath(info.ResolvedPath)
}

func isManagerOwnedBinaryPath(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == "/.vol" || strings.HasPrefix(clean, "/.vol/") {
		return true
	}
	components := strings.Split(clean, "/")
	for _, component := range components {
		if strings.EqualFold(component, "managed-adk") {
			return true
		}
	}
	return isHomebrewManagedBinaryPath(clean)
}

func isHomebrewManagedBinaryPath(clean string) bool {
	lower := strings.ToLower(filepath.ToSlash(filepath.Clean(clean)))
	roots := []string{
		"/homebrew/",
		"/.linuxbrew/",
		"/usr/local/",
	}
	for _, root := range roots {
		for _, formula := range []string{"auto", "autopus-adk"} {
			if strings.Contains(lower, root+"cellar/"+formula+"/") ||
				strings.Contains(lower, root+"opt/"+formula+"/") {
				return true
			}
		}
		if strings.Contains(lower, root+"caskroom/auto/") {
			return true
		}
	}
	return false
}

func managerRequiredUpdateError(path string) error {
	return fmt.Errorf(
		"manager-required: 설치 관리자가 소유한 auto 바이너리 %q은 CLI가 직접 교체할 수 없습니다; Homebrew/Linuxbrew 또는 Autopus Desktop 등 설치에 사용한 관리자로 업데이트하세요",
		path,
	)
}
