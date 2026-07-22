package gemini

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter"
)

const geminiCompletionHookAssetName = "hook-gemini-afteragent.sh"

var (
	geminiManagedHookDirectory = filepath.Join(".gemini", "hooks", "autopus")
	geminiCompletionHookTarget = filepath.Join(
		geminiManagedHookDirectory, geminiCompletionHookAssetName,
	)
	geminiCompletionHookAssetNames = []string{
		geminiCompletionHookAssetName,
		"hook-gemini-stop.sh",
	}
)

func prepareGeminiCompletionHookAssets() ([]adapter.FileMapping, error) {
	assets := make([]adapter.FileMapping, 0, len(geminiCompletionHookAssetNames))
	for _, name := range geminiCompletionHookAssetNames {
		data, err := contentfs.FS.ReadFile("hooks/" + name)
		if err != nil {
			return nil, fmt.Errorf("Gemini completion hook asset 읽기 실패 %s: %w", name, err)
		}
		assets = append(assets, adapter.FileMapping{
			TargetPath:      filepath.Join(geminiManagedHookDirectory, name),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(string(data)),
			Content:         data,
		})
	}
	return assets, nil
}

// writeGeminiCompletionHookAsset confines the managed executable to the
// project root and refuses symlinked parents or non-regular targets.
func writeGeminiCompletionHookAsset(rootPath string, asset adapter.FileMapping) error {
	return writeGeminiManagedHookAsset(rootPath, asset, 0o755)
}

func writeGeminiManagedHookAsset(rootPath string, asset adapter.FileMapping, mode os.FileMode) error {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return fmt.Errorf("Gemini project root 열기 실패: %w", err)
	}
	defer root.Close()

	target, err := cleanGeminiManagedHookTarget(asset.TargetPath)
	if err != nil {
		return err
	}
	if err := ensureGeminiHookDirectories(root, filepath.Dir(target), true); err != nil {
		return err
	}
	if info, statErr := root.Lstat(target); statErr == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Gemini completion hook target must be a regular file: %s", target)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("Gemini completion hook target 검사 실패: %w", statErr)
	}

	temporary, file, err := createGeminiHookTemporary(root, filepath.Dir(target))
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		_ = file.Close()
		if !committed {
			_ = root.Remove(temporary)
		}
	}()
	if _, err := io.Copy(file, bytes.NewReader(asset.Content)); err != nil {
		return fmt.Errorf("Gemini completion hook 임시 파일 쓰기 실패: %w", err)
	}
	if err := file.Chmod(mode.Perm()); err != nil {
		return fmt.Errorf("Gemini completion hook 실행 권한 설정 실패: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("Gemini completion hook 동기화 실패: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("Gemini completion hook 닫기 실패: %w", err)
	}
	if err := root.Rename(temporary, target); err != nil {
		return fmt.Errorf("Gemini completion hook 교체 실패: %w", err)
	}
	committed = true
	return nil
}

func ensureGeminiHookDirectories(root *os.Root, dir string, create bool) error {
	current := ""
	for _, part := range strings.Split(filepath.Clean(dir), string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := root.Lstat(current)
		if errors.Is(err, os.ErrNotExist) && create {
			if err := root.Mkdir(current, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
				return fmt.Errorf("Gemini completion hook 디렉터리 생성 실패 %s: %w", current, err)
			}
			info, err = root.Lstat(current)
		}
		if err != nil {
			return fmt.Errorf("Gemini completion hook 디렉터리 검사 실패 %s: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("Gemini completion hook parent must be a directory: %s", current)
		}
	}
	return nil
}

func createGeminiHookTemporary(root *os.Root, dir string) (string, *os.File, error) {
	for attempt := 0; attempt < 16; attempt++ {
		random := make([]byte, 8)
		if _, err := rand.Read(random); err != nil {
			return "", nil, fmt.Errorf("Gemini completion hook 임시 이름 생성 실패: %w", err)
		}
		path := filepath.Join(dir, ".autopus-hook.tmp-"+hex.EncodeToString(random))
		file, err := root.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", nil, fmt.Errorf("Gemini completion hook 임시 파일 생성 실패: %w", err)
		}
		return path, file, nil
	}
	return "", nil, errors.New("Gemini completion hook 임시 이름 생성 한도 초과")
}

func cleanGeminiManagedHookTarget(path string) (string, error) {
	clean := filepath.Clean(path)
	relative, err := filepath.Rel(geminiManagedHookDirectory, clean)
	if err != nil || clean == "." || filepath.IsAbs(clean) || !filepath.IsLocal(clean) ||
		relative == "." || !filepath.IsLocal(relative) || filepath.Dir(relative) != "." {
		return "", fmt.Errorf("invalid Gemini completion hook target: %s", path)
	}
	return clean, nil
}

func isGeminiManagedHookAsset(asset adapter.FileMapping) bool {
	_, err := cleanGeminiManagedHookTarget(asset.TargetPath)
	return err == nil
}

type geminiManagedHookSnapshot struct {
	asset   adapter.FileMapping
	exists  bool
	content []byte
	mode    os.FileMode
}

func applyGeminiManagedHookAssets(
	rootPath string,
	assets []adapter.FileMapping,
) (func() error, error) {
	snapshots := make([]geminiManagedHookSnapshot, 0, len(assets))
	for _, asset := range assets {
		snapshot, err := snapshotGeminiManagedHookAsset(rootPath, asset)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}

	for index, asset := range assets {
		if err := writeGeminiCompletionHookAsset(rootPath, asset); err != nil {
			restoreErr := restoreGeminiManagedHookAssets(rootPath, snapshots[:index])
			return nil, errors.Join(err, restoreErr)
		}
	}
	return func() error {
		return restoreGeminiManagedHookAssets(rootPath, snapshots)
	}, nil
}

func snapshotGeminiManagedHookAsset(
	rootPath string,
	asset adapter.FileMapping,
) (geminiManagedHookSnapshot, error) {
	target, err := cleanGeminiManagedHookTarget(asset.TargetPath)
	if err != nil {
		return geminiManagedHookSnapshot{}, err
	}
	snapshot := geminiManagedHookSnapshot{asset: asset}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return snapshot, fmt.Errorf("Gemini project root 열기 실패: %w", err)
	}
	defer root.Close()

	if err := ensureGeminiHookDirectories(root, filepath.Dir(target), false); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return snapshot, nil
		}
		return snapshot, err
	}
	info, err := root.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return snapshot, nil
	}
	if err != nil {
		return snapshot, fmt.Errorf("Gemini completion hook target 검사 실패: %w", err)
	}
	if !info.Mode().IsRegular() {
		return snapshot, fmt.Errorf("Gemini completion hook target must be a regular file: %s", target)
	}
	file, err := root.Open(target)
	if err != nil {
		return snapshot, fmt.Errorf("Gemini completion hook snapshot 열기 실패: %w", err)
	}
	content, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return snapshot, errors.Join(readErr, closeErr)
	}
	snapshot.exists = true
	snapshot.content = content
	snapshot.mode = info.Mode().Perm()
	return snapshot, nil
}

func restoreGeminiManagedHookAssets(rootPath string, snapshots []geminiManagedHookSnapshot) error {
	var restoreErr error
	for index := len(snapshots) - 1; index >= 0; index-- {
		snapshot := snapshots[index]
		if snapshot.exists {
			snapshot.asset.Content = snapshot.content
			restoreErr = errors.Join(
				restoreErr,
				writeGeminiManagedHookAsset(rootPath, snapshot.asset, snapshot.mode),
			)
			continue
		}
		restoreErr = errors.Join(restoreErr, removeGeminiManagedHookAsset(rootPath, snapshot.asset))
	}
	return restoreErr
}

func removeGeminiManagedHookAsset(rootPath string, asset adapter.FileMapping) error {
	target, err := cleanGeminiManagedHookTarget(asset.TargetPath)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return fmt.Errorf("Gemini project root 열기 실패: %w", err)
	}
	defer root.Close()
	if err := ensureGeminiHookDirectories(root, filepath.Dir(target), false); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := root.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("Gemini completion hook rollback 제거 실패: %w", err)
	}
	// Remove only empty shells created by the managed writer. User-owned files
	// keep their parent directories intact.
	_ = root.Remove(geminiManagedHookDirectory)
	_ = root.Remove(filepath.Dir(geminiManagedHookDirectory))
	return nil
}
