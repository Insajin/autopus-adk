package codex

import (
	"fmt"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

// prepareFiles prepares files without writing to disk.
func (a *Adapter) prepareFiles(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	if codexOwnsSharedSurface(cfg) {
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

	extSkillFiles, err := a.renderExtendedSkills()
	if err != nil {
		return nil, fmt.Errorf("extended skill 준비 실패: %w", err)
	}
	files = append(files, extSkillFiles...)

	if codexOwnsSharedSurface(cfg) {
		standardSkillFiles, err := a.prepareStandardSkillMappings(cfg)
		if err != nil {
			return nil, fmt.Errorf("표준 codex skill 준비 실패: %w", err)
		}
		files = append(files, standardSkillFiles...)
	}

	promptFiles, err := a.preparePromptFiles(cfg)
	if err != nil {
		return nil, fmt.Errorf("codex prompt 템플릿 준비 실패: %w", err)
	}
	files = append(files, promptFiles...)

	pluginFiles, err := a.preparePluginMappings(cfg)
	if err != nil {
		return nil, fmt.Errorf("codex plugin 준비 실패: %w", err)
	}
	files = append(files, pluginFiles...)

	rulePrepFiles, err := a.prepareRuleMappings(cfg)
	if err != nil {
		return nil, fmt.Errorf("codex rule 준비 실패: %w", err)
	}
	files = append(files, rulePrepFiles...)

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

	configPrepFiles, err := a.prepareConfigFile(cfg)
	if err != nil {
		return nil, fmt.Errorf("config 준비 실패: %w", err)
	}
	files = append(files, configPrepFiles...)

	gitHookFiles, err := a.prepareGitHookFiles(cfg)
	if err != nil {
		return nil, fmt.Errorf("git hooks 준비 실패: %w", err)
	}
	files = append(files, gitHookFiles...)

	return files, nil
}
