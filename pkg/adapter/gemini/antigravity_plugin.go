package gemini

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

const antigravityPluginDir = ".agents/plugins/autopus"

func prepareAntigravityPluginJSON() ([]adapter.FileMapping, error) {
	body, err := json.MarshalIndent(map[string]string{"name": "autopus"}, "", "  ")
	if err != nil {
		return nil, err
	}
	body = append(body, '\n')
	return []adapter.FileMapping{{
		TargetPath:      filepath.Join(antigravityPluginDir, "plugin.json"),
		OverwritePolicy: adapter.OverwriteAlways,
		Checksum:        checksum(string(body)),
		Content:         body,
	}}, nil
}

func mirrorAntigravityPluginMappings(files []adapter.FileMapping) []adapter.FileMapping {
	mirrored := make([]adapter.FileMapping, 0, len(files))
	for _, file := range files {
		target, ok := antigravityPluginTarget(file.TargetPath)
		if !ok {
			continue
		}
		content := rewriteAntigravityPluginContent(string(file.Content))
		mirrored = append(mirrored, adapter.FileMapping{
			TargetPath:      target,
			OverwritePolicy: file.OverwritePolicy,
			Checksum:        checksum(content),
			Content:         []byte(content),
		})
	}
	return mirrored
}

func antigravityPluginTarget(path string) (string, bool) {
	path = filepath.ToSlash(path)
	switch {
	case strings.HasPrefix(path, ".gemini/skills/autopus/"):
		return strings.Replace(path, ".gemini/skills/autopus/", antigravityPluginDir+"/skills/", 1), true
	case path == ".gemini/skills/auto/SKILL.md":
		return filepath.ToSlash(filepath.Join(antigravityPluginDir, "skills", "auto", "SKILL.md")), true
	case strings.HasPrefix(path, ".gemini/rules/autopus/"):
		return strings.Replace(path, ".gemini/rules/autopus/", antigravityPluginDir+"/rules/", 1), true
	case strings.HasPrefix(path, ".gemini/agents/autopus/"):
		return strings.Replace(path, ".gemini/agents/autopus/", antigravityPluginDir+"/agents/", 1), true
	case strings.HasPrefix(path, ".gemini/commands/auto/"):
		return strings.Replace(path, ".gemini/commands/auto/", antigravityPluginDir+"/commands/auto/", 1), true
	default:
		return "", false
	}
}

func rewriteAntigravityPluginContent(content string) string {
	replacer := strings.NewReplacer(
		".gemini/skills/autopus/", antigravityPluginDir+"/skills/",
		".gemini/skills/auto/", antigravityPluginDir+"/skills/auto/",
		".gemini/rules/autopus/", antigravityPluginDir+"/rules/",
		".gemini/agents/autopus/", antigravityPluginDir+"/agents/",
		".gemini/commands/auto/", antigravityPluginDir+"/commands/auto/",
	)
	return replacer.Replace(content)
}
