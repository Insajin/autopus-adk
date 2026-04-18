// Package gemini implements the Gemini CLI platform adapter.
package gemini

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

// Update applies incremental changes to an existing installation.
// Falls back to Generate when no manifest exists.
func (a *Adapter) Update(ctx context.Context, cfg *config.HarnessConfig) (*adapter.PlatformFiles, error) {
	oldManifest, err := adapter.LoadManifest(a.root, adapterName)
	if err != nil {
		return nil, fmt.Errorf("매니페스트 로드 실패: %w", err)
	}

	if oldManifest == nil {
		return a.Generate(ctx, cfg)
	}

	newFiles, err := a.prepareFiles(cfg)
	if err != nil {
		return nil, err
	}

	var backupDir string
	var finalFiles []adapter.FileMapping

	for _, f := range newFiles {
		action := adapter.ResolveAction(a.root, f.TargetPath, f.OverwritePolicy, oldManifest)

		if action == adapter.ActionSkip {
			continue
		}
		if action == adapter.ActionBackup {
			if backupDir == "" {
				backupDir, err = adapter.CreateBackupDir(a.root)
				if err != nil {
					return nil, err
				}
			}
			if _, backupErr := adapter.BackupFile(a.root, f.TargetPath, backupDir); backupErr != nil {
				return nil, backupErr
			}
		}

		targetPath := filepath.Join(a.root, f.TargetPath)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return nil, fmt.Errorf("디렉터리 생성 실패: %w", err)
		}
		if err := os.WriteFile(targetPath, f.Content, 0644); err != nil {
			return nil, fmt.Errorf("파일 쓰기 실패 %s: %w", f.TargetPath, err)
		}
		finalFiles = append(finalFiles, f)
	}

	pf := &adapter.PlatformFiles{
		Files:    finalFiles,
		Checksum: checksum(fmt.Sprintf("%d", len(finalFiles))),
	}

	m := adapter.ManifestFromFiles(adapterName, pf)
	if saveErr := m.Save(a.root); saveErr != nil {
		return nil, fmt.Errorf("매니페스트 저장 실패: %w", saveErr)
	}

	if backupDir != "" {
		fmt.Fprintf(os.Stderr, "  백업됨: %s\n", backupDir)
	}

	// Hooks and permissions are applied to .gemini/settings.json after the
	// template-based file writes above — Update was previously skipping this
	// step, leaving stale Claude Code event names (PreToolUse/PostToolUse)
	// in place after the event-name translation was added.
	if err := a.applyHooksAndPermissions(ctx, cfg); err != nil {
		return nil, fmt.Errorf("제미니 훅/권한 설치 실패: %w", err)
	}

	return pf, nil
}

// prepareFiles prepares the same files as Generate but without writing to disk.
func (a *Adapter) prepareFiles(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	geminiMD, err := a.injectMarkerSection(cfg)
	if err != nil {
		return nil, fmt.Errorf("GEMINI.md 마커 주입 실패: %w", err)
	}
	files = append(files, adapter.FileMapping{
		TargetPath:      "GEMINI.md",
		OverwritePolicy: adapter.OverwriteMarker,
		Checksum:        checksum(geminiMD),
		Content:         []byte(geminiMD),
	})

	skillMappings, err := a.prepareSkillMappings(cfg)
	if err != nil {
		return nil, err
	}
	files = append(files, skillMappings...)

	// Extended skills from content/skills/ via transformer
	extSkillMappings, err := a.renderExtendedSkills()
	if err != nil {
		return nil, fmt.Errorf("extended skill 준비 실패: %w", err)
	}
	files = append(files, extSkillMappings...)

	cmdMappings, err := a.prepareCommandMappings(cfg)
	if err != nil {
		return nil, err
	}
	files = append(files, cmdMappings...)

	ruleMappings, err := a.prepareRuleMappings(cfg)
	if err != nil {
		return nil, err
	}
	files = append(files, ruleMappings...)

	if cfg.IsFullMode() {
		agentMappings, err := a.prepareAgentMappings()
		if err != nil {
			return nil, err
		}
		files = append(files, agentMappings...)
	}

	settingsMappings, err := a.generateSettings(cfg)
	if err != nil {
		return nil, err
	}
	files = append(files, settingsMappings...)

	routerMappings, err := a.prepareRouterCommand(cfg)
	if err != nil {
		return nil, err
	}
	files = append(files, routerMappings...)

	statusFiles, err := a.prepareStatusline()
	if err != nil {
		return nil, err
	}
	files = append(files, statusFiles...)

	return files, nil
}
