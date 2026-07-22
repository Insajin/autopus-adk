// Package gemini implements the Antigravity CLI platform adapter.
package gemini

import (
	"context"
	"errors"
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
		oldManifest, err = adapter.LoadManifest(a.root, legacyAdapterName)
		if err != nil {
			return nil, fmt.Errorf("legacy 매니페스트 로드 실패: %w", err)
		}
	}

	newFiles, err := a.prepareFiles(cfg)
	if err != nil {
		return nil, err
	}

	plan, pf := a.buildUpdateTransactionPlan(oldManifest, newFiles)
	rollbackHooks, err := applyGeminiManagedHookAssets(a.root, geminiManagedHookAssets(pf.Files))
	if err != nil {
		return nil, err
	}
	if _, err := adapter.ApplyTransaction(a.root, adapterName, plan); err != nil {
		return nil, errors.Join(err, rollbackHooks())
	}
	a.installAntigravityPluginIfAvailable(ctx)

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

	pluginFiles, err := prepareAntigravityPluginJSON()
	if err != nil {
		return nil, fmt.Errorf("antigravity plugin manifest 생성 실패: %w", err)
	}
	files = append(files, pluginFiles...)

	skillMappings, err := a.prepareSkillMappings(cfg)
	if err != nil {
		return nil, err
	}

	// Extended skills from content/skills/ via transformer
	extSkillMappings, err := a.renderExtendedSkills(cfg)
	if err != nil {
		return nil, fmt.Errorf("extended skill 준비 실패: %w", err)
	}
	extMirrors := mirrorAntigravityPluginMappings(extSkillMappings)
	files = append(files, mergeSkillMappings(skillMappings, append(extSkillMappings, extMirrors...))...)

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

	completionHookAssets, err := prepareGeminiCompletionHookAssets()
	if err != nil {
		return nil, err
	}
	files = append(files, completionHookAssets...)

	settingsMappings, err := a.generateSettingsWithHooks(cfg)
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

	hookMappings, err := a.prepareAntigravityHooksJSON(a.configuredAntigravityHooks(cfg))
	if err != nil {
		return nil, err
	}
	files = append(files, hookMappings...)

	return sanitizeUnsupportedClaudeTeamMappings(files), nil
}

func (a *Adapter) buildUpdateTransactionPlan(
	oldManifest *adapter.Manifest,
	newFiles []adapter.FileMapping,
) (adapter.TransactionPlan, *adapter.PlatformFiles) {
	finalFiles := make([]adapter.FileMapping, 0, len(newFiles))
	writes := make([]adapter.TransactionWrite, 0, len(newFiles))
	for _, file := range newFiles {
		if !isGeminiManagedHookAsset(file) {
			action := adapter.ResolveAction(a.root, file.TargetPath, file.OverwritePolicy, oldManifest)
			if action == adapter.ActionSkip {
				continue
			}
			writes = append(writes, adapter.TransactionWrite{
				Path:    file.TargetPath,
				Content: file.Content,
				Perm:    geminiFileMode(file.TargetPath),
			})
		}
		finalFiles = append(finalFiles, file)
	}

	// @AX:NOTE: [AUTO] file-count-only checksum — manifest integrity reflects file count only, not content hash; not a tamper-detection mechanism
	pf := &adapter.PlatformFiles{
		Files:    finalFiles,
		Checksum: checksum(fmt.Sprintf("%d", len(finalFiles))),
	}

	return adapter.TransactionPlan{
		Writes:   writes,
		Manifest: adapter.ManifestFromFiles(adapterName, pf),
	}, pf
}

func geminiManagedHookAssets(files []adapter.FileMapping) []adapter.FileMapping {
	assets := make([]adapter.FileMapping, 0, len(files))
	for _, file := range files {
		if isGeminiManagedHookAsset(file) {
			assets = append(assets, file)
		}
	}
	return assets
}

func geminiFileMode(path string) os.FileMode {
	clean := filepath.ToSlash(path)
	if filepath.Ext(clean) == ".sh" {
		return 0755
	}
	return 0644
}
