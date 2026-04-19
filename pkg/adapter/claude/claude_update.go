package claude

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

// Update는 매니페스트 기반으로 파일을 업데이트한다.
// 사용자가 수정한 파일은 백업 후 덮어쓰고, 삭제한 파일은 재생성하지 않는다.
func (a *Adapter) Update(ctx context.Context, cfg *config.HarnessConfig) (*adapter.PlatformFiles, error) {
	oldManifest, err := adapter.LoadManifest(a.root, adapterName)
	if err != nil {
		return nil, fmt.Errorf("매니페스트 로드 실패: %w", err)
	}

	if oldManifest == nil {
		pf, err := a.Generate(ctx, cfg)
		if err != nil {
			return nil, err
		}
		m := adapter.ManifestFromFiles(adapterName, pf)
		if saveErr := m.Save(a.root); saveErr != nil {
			return nil, fmt.Errorf("매니페스트 저장 실패: %w", saveErr)
		}
		return pf, nil
	}

	newFiles, err := a.prepareFiles(cfg)
	if err != nil {
		return nil, err
	}

	var backupDir string
	var results []adapter.UpdateResult
	var finalFiles []adapter.FileMapping

	for _, f := range newFiles {
		action := adapter.ResolveAction(a.root, f.TargetPath, f.OverwritePolicy, oldManifest)

		switch action {
		case adapter.ActionSkip:
			results = append(results, adapter.UpdateResult{
				Path:   f.TargetPath,
				Action: adapter.ActionSkip,
			})
			continue
		case adapter.ActionBackup:
			if backupDir == "" {
				backupDir, err = adapter.CreateBackupDir(a.root)
				if err != nil {
					return nil, err
				}
			}
			backupPath, backupErr := adapter.BackupFile(a.root, f.TargetPath, backupDir)
			if backupErr != nil {
				return nil, backupErr
			}
			results = append(results, adapter.UpdateResult{
				Path:       f.TargetPath,
				Action:     adapter.ActionBackup,
				BackupPath: backupPath,
			})
		}

		targetPath := filepath.Join(a.root, f.TargetPath)
		targetDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return nil, fmt.Errorf("디렉터리 생성 실패 %s: %w", targetDir, err)
		}

		perm := os.FileMode(0644)
		if filepath.Ext(f.TargetPath) == ".sh" {
			perm = 0755
		}
		if err := os.WriteFile(targetPath, f.Content, perm); err != nil {
			return nil, fmt.Errorf("파일 쓰기 실패 %s: %w", f.TargetPath, err)
		}
		finalFiles = append(finalFiles, f)
	}

	if err := a.applyHooksAndPermissions(ctx, cfg); err != nil {
		return nil, err
	}

	pf := &adapter.PlatformFiles{
		Files:    finalFiles,
		Checksum: checksum(fmt.Sprintf("%d", len(finalFiles))),
	}

	m := adapter.ManifestFromFiles(adapterName, pf)
	for _, r := range results {
		if r.Action != adapter.ActionSkip {
			continue
		}
		if prev, ok := oldManifest.Files[r.Path]; ok {
			m.Files[r.Path] = prev
		}
	}
	if saveErr := m.Save(a.root); saveErr != nil {
		return nil, fmt.Errorf("매니페스트 저장 실패: %w", saveErr)
	}

	if backupDir != "" {
		fmt.Fprintf(os.Stderr, "  백업됨: %s\n", backupDir)
	}

	return pf, nil
}
