package gemini

import (
	"fmt"
	"path/filepath"

	"github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
)

// renderExtendedSkills transforms embedded content skills for the Gemini platform
// and returns file mappings for .gemini/skills/autopus/{skill-name}/SKILL.md.
func (a *Adapter) renderExtendedSkills() ([]adapter.FileMapping, error) {
	transformer, err := pkgcontent.NewSkillTransformerFromFS(content.FS, "skills")
	if err != nil {
		return nil, fmt.Errorf("skill transformer init: %w", err)
	}

	skills, report, err := transformer.TransformForPlatform("gemini")
	if err != nil {
		return nil, fmt.Errorf("skill transform for gemini: %w", err)
	}

	logTransformReport("gemini", report)

	var files []adapter.FileMapping
	for _, s := range skills {
		// Gemini convention: each skill gets its own subdirectory
		relPath := filepath.Join(".gemini", "skills", "autopus", s.Name, "SKILL.md")
		files = append(files, adapter.FileMapping{
			TargetPath:      relPath,
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(s.Content),
			Content:         []byte(s.Content),
		})
	}

	return files, nil
}

// logTransformReport prints a summary of skill transformation results.
func logTransformReport(platform string, report *pkgcontent.TransformReport) {
	if report == nil {
		return
	}
	fmt.Printf("  [%s] extended skills: %d compatible, %d incompatible\n",
		platform, len(report.Compatible), len(report.Incompatible))
}
