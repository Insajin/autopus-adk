package codex

import (
	"fmt"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

// prepareFiles prepares files without writing to disk.
func (a *Adapter) prepareFiles(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	return a.prepareFilesWithManifest(cfg, nil)
}

// @AX:WARN [AUTO]: Codex surface preparation contains more than eight conditional branches.
// @AX:REASON [AUTO]: shared root ownership, native skills, plugins, agents, hooks, structural config merge, Git hooks, and mapping deduplication converge here.
func (a *Adapter) prepareFilesWithManifest(
	cfg *config.HarnessConfig,
	oldManifest *adapter.Manifest,
) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	if codexOwnsRootDoc(cfg) {
		agentsMD, err := a.injectMarkerSection(cfg)
		if err != nil {
			return nil, fmt.Errorf("AGENTS.md 마커 주입 실패: %w", err)
		}
		files = append(files, adapter.FileMapping{
			TargetPath:      "AGENTS.md",
			OverwritePolicy: adapter.OverwriteMarker,
			Checksum:        checksum(agentsMD),
			Content:         []byte(agentsMD),
		})
	}

	skillMappings, err := a.prepareSkillTemplateMappings(cfg)
	if err != nil {
		return nil, fmt.Errorf("codex skill 템플릿 준비 실패: %w", err)
	}
	files = append(files, skillMappings...)

	extSkillFiles, err := a.renderExtendedSkills(cfg)
	if err != nil {
		return nil, fmt.Errorf("extended skill 준비 실패: %w", err)
	}
	files = append(files, extSkillFiles...)

	standardSkillFiles, err := a.prepareStandardSkillMappings(cfg)
	if err != nil {
		return nil, fmt.Errorf("표준 codex skill 준비 실패: %w", err)
	}
	files = append(files, standardSkillFiles...)

	pluginFiles, err := a.preparePluginMappings(cfg)
	if err != nil {
		return nil, fmt.Errorf("codex plugin 준비 실패: %w", err)
	}
	files = append(files, pluginFiles...)

	agentPrepFiles, err := a.prepareAgentFiles(cfg)
	if err != nil {
		return nil, fmt.Errorf("agent 준비 실패: %w", err)
	}
	files = append(files, agentPrepFiles...)

	hooksPrepFiles, err := a.prepareHooksFile(cfg)
	if err != nil {
		return nil, fmt.Errorf("hooks 준비 실패: %w", err)
	}
	files = append(files, hooksPrepFiles...)

	configPrepFiles, err := a.prepareConfigFileWithManifest(cfg, oldManifest)
	if err != nil {
		return nil, fmt.Errorf("config 준비 실패: %w", err)
	}
	files = append(files, configPrepFiles...)

	gitHookFiles, err := a.prepareGitHookFiles(cfg)
	if err != nil {
		return nil, fmt.Errorf("git hooks 준비 실패: %w", err)
	}
	files = append(files, gitHookFiles...)

	return uniqueCodexMappings(files), nil
}

func uniqueCodexMappings(files []adapter.FileMapping) []adapter.FileMapping {
	result := make([]adapter.FileMapping, 0, len(files))
	indices := make(map[string]int, len(files))
	for _, file := range files {
		path := filepath.ToSlash(file.TargetPath)
		if index, ok := indices[path]; ok {
			result[index] = file
			continue
		}
		indices[path] = len(result)
		result = append(result, file)
	}
	return result
}
