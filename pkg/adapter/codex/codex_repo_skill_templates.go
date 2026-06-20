package codex

import (
	"path/filepath"
	"strings"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/config"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
)

func shouldEmitCodexRepoSkillTemplate(skillFile string, cfg *config.HarnessConfig) (bool, error) {
	if cfg == nil || cfg.Skills.Compiler.EffectiveMode() == config.SkillCompilerModeFull {
		return true, nil
	}

	name := strings.TrimSuffix(skillFile, ".md")
	if name == "" || strings.HasPrefix(name, "auto-") {
		return true, nil
	}

	catalog, err := pkgcontent.LoadSkillCatalogFromFS(contentfs.FS, "skills")
	if err != nil {
		return false, err
	}
	entry, ok := catalog.Get(name)
	if !ok {
		return true, nil
	}

	state := pkgcontent.ResolveCatalogSkillState(entry, "codex", cfg)
	return filepath.ToSlash(state.TargetPath) == filepath.ToSlash(filepath.Join(".codex", "skills", name+".md")), nil
}
