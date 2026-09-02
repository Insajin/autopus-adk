package codex

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

func (a *Adapter) validateNativeCodexSurface(errs *[]adapter.ValidationError) {
	manifest, _ := adapter.LoadManifest(a.root, adapterName)
	expectedSkills := []string{codexProjectSkillPath("auto")}
	if manifest != nil {
		expectedSkills = expectedSkills[:0]
		for path := range manifest.Files {
			if strings.HasPrefix(filepath.ToSlash(path), ".codex/skills/") {
				expectedSkills = append(expectedSkills, path)
			}
		}
	}
	for _, path := range expectedSkills {
		validateNativeCodexSkill(a.root, path, errs)
	}

	for _, path := range []string{
		filepath.Join(".codex", "agents", "executor.toml"),
		filepath.Join(".codex", "hooks.json"),
		filepath.Join(".autopus", "plugins", "auto", ".codex-plugin", "plugin.json"),
	} {
		if info, err := os.Stat(filepath.Join(a.root, path)); err != nil || !info.Mode().IsRegular() {
			*errs = append(*errs, adapter.ValidationError{
				File: path, Message: "Codex native managed surface가 없거나 regular file이 아님", Level: "error",
			})
		}
	}
	validateObsoleteCodexSurface(a.root, a.openCodeOwnsRootDoc(), errs)
}

func validateNativeCodexSkill(root, path string, errs *[]adapter.ValidationError) {
	clean := filepath.ToSlash(path)
	parts := strings.Split(clean, "/")
	if len(parts) != 4 || parts[0] != ".codex" || parts[1] != "skills" ||
		!strings.HasPrefix(parts[2], "codex-") || parts[3] != "SKILL.md" {
		*errs = append(*errs, adapter.ValidationError{
			File: path, Message: "Codex skill이 native codex-*/SKILL.md layout이 아님", Level: "error",
		})
		return
	}
	data, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		*errs = append(*errs, adapter.ValidationError{
			File: path, Message: "Codex native skill을 읽을 수 없음", Level: "error",
		})
		return
	}
	if !strings.Contains(string(data), "name: "+parts[2]) {
		*errs = append(*errs, adapter.ValidationError{
			File: path, Message: "Codex native skill name이 directory와 일치하지 않음", Level: "error",
		})
	}
}

// validateObsoleteCodexSurface reports every retired Codex surface the detector
// names. Detection itself lives in obsoleteCodexSurfacePaths so the update
// prune removes exactly what this report claims is obsolete.
func validateObsoleteCodexSurface(root string, openCodeOwnsSharedSkills bool, errs *[]adapter.ValidationError) {
	for _, path := range obsoleteCodexSurfacePaths(root, openCodeOwnsSharedSkills) {
		*errs = append(*errs, adapter.ValidationError{
			File: path, Message: "obsolete Codex managed surface가 남아 있음", Level: "error",
		})
	}
}
