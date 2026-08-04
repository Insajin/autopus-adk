package promptlayer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	ompContextPromotionReportRelativePathV2      = ".autopus/runtime/omp-context/promotion-report-v1.json"
	ompContextPromotionAttestationRelativePathV2 = ".autopus/runtime/omp-context/promotion-attestation-v2.json"
)

func LoadVerifiedOMPContextPromotionV2(root string, now time.Time,
	expected OMPContextPromotionExpectationV2) (VerifiedOMPContextPromotion, error) {
	reportPath, attestationPath, err := resolveOMPContextPromotionArtifactPathsV2(root)
	if err != nil {
		return VerifiedOMPContextPromotion{}, err
	}
	reportBytes, err := readOMPContextPromotionArtifactFileV2(reportPath, ompContextPromotionReportMaxBytesV1)
	if err != nil {
		return VerifiedOMPContextPromotion{}, err
	}
	attestationBytes, err := readOMPContextPromotionArtifactFileV2(attestationPath, ompContextPromotionAttestationMaxBytesV2)
	if err != nil {
		return VerifiedOMPContextPromotion{}, err
	}
	return VerifyOMPContextPromotionArtifactV2(reportBytes, attestationBytes, now, expected)
}

func resolveOMPContextPromotionArtifactPathsV2(root string) (string, string, error) {
	if root == "" || filepath.Clean(root) == "." {
		return "", "", fmt.Errorf("OMP context promotion root is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	rootInfo, err := os.Lstat(absolute)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return "", "", fmt.Errorf("OMP context promotion root must be a regular directory")
	}
	resolvedRoot, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", "", fmt.Errorf("OMP context promotion root identity is unsafe")
	}
	current := absolute
	for index, part := range []string{".autopus", "runtime", "omp-context"} {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", "", fmt.Errorf("OMP context promotion artifact directory is unsafe")
		}
		if (index == 0 && info.Mode().Perm()&0o022 != 0) || (index > 0 && info.Mode().Perm() != 0o700) {
			return "", "", fmt.Errorf("OMP context promotion artifact directory permissions are unsafe")
		}
		resolved, resolveErr := filepath.EvalSymlinks(current)
		if resolveErr != nil || !containedOMPContextPromotionPathV2(resolvedRoot, resolved) {
			return "", "", fmt.Errorf("OMP context promotion artifact directory escapes root")
		}
	}
	return filepath.Join(absolute, ompContextPromotionReportRelativePathV2),
		filepath.Join(absolute, ompContextPromotionAttestationRelativePathV2), nil
}

func readOMPContextPromotionArtifactFileV2(path string, limit int64) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Mode().Perm() != 0o600 ||
		before.Size() <= 0 || before.Size() > limit {
		return nil, fmt.Errorf("OMP context promotion artifact file is unsafe")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open OMP context promotion artifact: %w", err)
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !after.Mode().IsRegular() || after.Mode().Perm() != 0o600 || !os.SameFile(before, after) {
		return nil, fmt.Errorf("OMP context promotion artifact identity changed")
	}
	body, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || len(body) == 0 || int64(len(body)) > limit {
		return nil, fmt.Errorf("read OMP context promotion artifact")
	}
	return body, nil
}

func containedOMPContextPromotionPathV2(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && !filepath.IsAbs(relative) && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
