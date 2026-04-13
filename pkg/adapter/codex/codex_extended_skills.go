package codex

import (
	"fmt"
	"path/filepath"

	"github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
)

// renderExtendedSkills transforms embedded content skills for the Codex platform
// and returns file mappings for .codex/skills/{skill-name}.md.
func (a *Adapter) renderExtendedSkills() ([]adapter.FileMapping, error) {
	transformer, err := pkgcontent.NewSkillTransformerFromFS(content.FS, "skills")
	if err != nil {
		return nil, fmt.Errorf("skill transformer init: %w", err)
	}

	skills, report, err := transformer.TransformForPlatform("codex")
	if err != nil {
		return nil, fmt.Errorf("skill transform for codex: %w", err)
	}

	logTransformReport("codex", report)

	var files []adapter.FileMapping
	for _, s := range skills {
		content := normalizeCodexInvocationBody(s.Content)
		content = normalizeCodexHelperPaths(content)
		content = normalizeCodexToolingBody(content)
		content = normalizeCodexExtendedSkill(s.Name, content)
		relPath := filepath.Join(".codex", "skills", s.Name+".md")
		files = append(files, adapter.FileMapping{
			TargetPath:      relPath,
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(content),
			Content:         []byte(content),
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
