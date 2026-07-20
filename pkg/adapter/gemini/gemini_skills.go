// Package gemini provides skill template rendering for Antigravity CLI.
package gemini

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
	"github.com/insajin/autopus-adk/templates"
)

// renderSkillTemplates reads Gemini skill templates from the embedded FS,
// renders them, writes to .gemini/skills/autopus/{skill}/SKILL.md, and
// returns file mappings.
func (a *Adapter) renderSkillTemplates(cfg *config.HarnessConfig, geminiSkillBaseDir string) ([]adapter.FileMapping, error) {
	platformMappings, err := a.prepareSkillMappings(cfg)
	if err != nil {
		return nil, err
	}

	// Extended skills from content/skills/ via transformer
	extFiles, err := a.renderExtendedSkills(cfg)
	if err != nil {
		return nil, fmt.Errorf("extended skill rendering failed: %w", err)
	}
	extMirrors := mirrorAntigravityPluginMappings(extFiles)
	files := mergeSkillMappings(platformMappings, append(extFiles, extMirrors...))
	for _, mapping := range files {
		destPath := filepath.Join(a.root, mapping.TargetPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return nil, fmt.Errorf("extended skill dir creation failed %s: %w", filepath.Dir(destPath), err)
		}
		if err := os.WriteFile(destPath, mapping.Content, 0644); err != nil {
			return nil, fmt.Errorf("extended skill write failed %s: %w", destPath, err)
		}
	}
	return files, nil
}

// prepareSkillMappings renders skill templates and returns file mappings
// without writing to disk. Used by both renderSkillTemplates and prepareFiles.
func (a *Adapter) prepareSkillMappings(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping
	catalog, err := pkgcontent.LoadSkillCatalogFromFS(contentfs.FS, "skills")
	if err != nil {
		return nil, fmt.Errorf("skill catalog init: %w", err)
	}

	entries, err := templates.FS.ReadDir("gemini/skills")
	if err != nil {
		return nil, fmt.Errorf("제미니 스킬 템플릿 디렉터리 읽기 실패: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillName := entry.Name()
		if skill, ok := catalog.Get(skillName); ok &&
			!pkgcontent.ResolveCatalogSkillState(skill, "gemini", cfg).Compiled {
			continue
		}

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

	return append(files, mirrorAntigravityPluginMappings(files)...), nil
}

// @AX:NOTE: [AUTO] magic constant — "auto-" prefix is 5 chars; hardcoded length check must be updated if the prefix ever changes
func hasAutoPrefix(name string) bool {
	return len(name) >= 5 && name[:5] == "auto-"
}

// mergeSkillMappings selects one canonical owner for each generated target.
// Platform templates own auto-* route contracts; transformed catalog skills
// own shared non-route collisions. Sorted targets keep writes and manifests
// deterministic across Generate and Update, including Antigravity mirrors.
func mergeSkillMappings(platform, transformed []adapter.FileMapping) []adapter.FileMapping {
	byTarget := make(map[string]adapter.FileMapping, len(platform)+len(transformed))
	for _, mapping := range platform {
		byTarget[filepath.Clean(mapping.TargetPath)] = mapping
	}
	for _, mapping := range transformed {
		target := filepath.Clean(mapping.TargetPath)
		if _, exists := byTarget[target]; exists && isAutoRouteSkillTarget(target) {
			continue
		}
		byTarget[target] = mapping
	}

	targets := make([]string, 0, len(byTarget))
	for target := range byTarget {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	merged := make([]adapter.FileMapping, 0, len(targets))
	for _, target := range targets {
		merged = append(merged, byTarget[target])
	}
	return merged
}

func isAutoRouteSkillTarget(target string) bool {
	if filepath.Base(target) != "SKILL.md" {
		return false
	}
	return hasAutoPrefix(filepath.Base(filepath.Dir(target)))
}
