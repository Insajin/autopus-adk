package content

import "strings"

const codexSkillNamePrefix = "codex-"

// ResolveCatalogSkillOutputName returns the platform-native frontmatter and directory name.
// @AX:ANCHOR [AUTO]: preserve platform-native skill naming as a single catalog boundary.
// @AX:REASON [AUTO]: Claude, Codex, OMP, and catalog target resolution share this name for both frontmatter and output directories.
func ResolveCatalogSkillOutputName(name, platform string) string {
	if normalizeCatalogPlatform(platform) != "codex" || strings.HasPrefix(name, codexSkillNamePrefix) {
		return name
	}
	return codexSkillNamePrefix + name
}
