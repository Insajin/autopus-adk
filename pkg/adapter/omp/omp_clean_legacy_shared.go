package omp

import (
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

func (a *Adapter) shouldCleanLegacySharedPath(
	path string,
	previous adapter.ManifestFile,
	data []byte,
) bool {
	clean := filepath.ToSlash(path)
	legacyShared := strings.HasPrefix(clean, ".agents/skills/") ||
		strings.HasPrefix(clean, ".agents/commands/")
	if !legacyShared {
		return true
	}
	if adapter.Checksum(string(data)) != previous.Checksum {
		return false
	}
	cfg, err := config.LoadPreview(a.root)
	if err != nil {
		return false
	}
	for _, platform := range cfg.Platforms {
		if platform != "omp" {
			return false
		}
	}
	return true
}
