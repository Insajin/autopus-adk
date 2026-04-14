// Package gemini provides skill template rendering for Gemini CLI.
package gemini

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/templates"
)

// renderSkillTemplates reads Gemini skill templates from the embedded FS,
// renders them, writes to .gemini/skills/autopus/{skill}/SKILL.md, and
// returns file mappings.
func (a *Adapter) renderSkillTemplates(cfg *config.HarnessConfig, geminiSkillBaseDir string) ([]adapter.FileMapping, error) {
	mappings, err := a.prepareSkillMappings(cfg)
	if err != nil {
		return nil, err
	}

	for _, m := range mappings {
		destPath := filepath.Join(a.root, m.TargetPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return nil, fmt.Errorf("제미니 스킬 디렉터리 생성 실패 %s: %w", filepath.Dir(destPath), err)
		}
		if err := os.WriteFile(destPath, m.Content, 0644); err != nil {
			return nil, fmt.Errorf("제미니 SKILL.md 쓰기 실패 %s: %w", destPath, err)
		}
	}

	// Extended skills from content/skills/ via transformer
	extFiles, err := a.renderExtendedSkills()
	if err != nil {
		return nil, fmt.Errorf("extended skill rendering failed: %w", err)
	}
	for _, ef := range extFiles {
		destPath := filepath.Join(a.root, ef.TargetPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return nil, fmt.Errorf("extended skill dir creation failed %s: %w", filepath.Dir(destPath), err)
		}
		if err := os.WriteFile(destPath, ef.Content, 0644); err != nil {
			return nil, fmt.Errorf("extended skill write failed %s: %w", destPath, err)
		}
	}
	mappings = append(mappings, extFiles...)

	return mappings, nil
}

// prepareSkillMappings renders skill templates and returns file mappings
// without writing to disk. Used by both renderSkillTemplates and prepareFiles.
func (a *Adapter) prepareSkillMappings(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	entries, err := templates.FS.ReadDir("gemini/skills")
	if err != nil {
		return nil, fmt.Errorf("제미니 스킬 템플릿 디렉터리 읽기 실패: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillName := entry.Name()

		tmplPath := "gemini/skills/" + skillName + "/SKILL.md.tmpl"
		tmplContent, err := templates.FS.ReadFile(tmplPath)
		if err != nil {
			return nil, fmt.Errorf("제미니 스킬 템플릿 읽기 실패 %s: %w", tmplPath, err)
		}

		rendered, err := a.engine.RenderString(string(tmplContent), cfg)
		if err != nil {
			if hasAutoPrefix(skillName) {
				return nil, fmt.Errorf("제미니 스킬 템플릿 렌더링 실패 %s: %w", skillName, err)
			}
			rendered = string(tmplContent)
		}

		relPath := filepath.Join(".gemini", "skills", "autopus", skillName, "SKILL.md")
		files = append(files, adapter.FileMapping{
			TargetPath:      relPath,
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(rendered),
			Content:         []byte(rendered),
		})
	}

	return files, nil
}

func hasAutoPrefix(name string) bool {
	return len(name) >= 5 && name[:5] == "auto-"
}
