package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

// requireCompleteGPTReviewDocuments keeps the new full-document admission
// path scoped to GPT/Codex-only review runs. Mixed or non-GPT provider sets
// retain their existing prompt behavior.
func requireCompleteGPTReviewDocuments(providers []orchestra.ProviderConfig) bool {
	if len(providers) == 0 {
		return false
	}
	for _, provider := range providers {
		identity := strings.ToLower(strings.TrimSpace(provider.Name))
		if identity == "" {
			identity = strings.ToLower(filepath.Base(strings.TrimSpace(provider.Binary)))
		}
		switch identity {
		case "codex", "openai", "gpt":
			continue
		default:
			return false
		}
	}
	return true
}

type specReviewContextScope struct {
	projectRoot string
	specDir     string
}

// resolveSpecReviewContextScope anchors a resolved SPEC under its owning
// project root and returns the SPEC directory as a root-relative reference.
func resolveSpecReviewContextScope(resolvedSpecDir string) (specReviewContextScope, error) {
	absSpecDir, err := filepath.Abs(strings.TrimSpace(resolvedSpecDir))
	if err != nil {
		return specReviewContextScope{}, fmt.Errorf("resolve review SPEC directory: %w", err)
	}
	canonicalSpecDir, err := filepath.EvalSymlinks(absSpecDir)
	if err != nil {
		return specReviewContextScope{}, fmt.Errorf("resolve review SPEC directory links: %w", err)
	}
	info, err := os.Stat(canonicalSpecDir)
	if err != nil || !info.IsDir() {
		return specReviewContextScope{}, fmt.Errorf("review SPEC directory unavailable: %s", resolvedSpecDir)
	}
	specsDir := filepath.Dir(canonicalSpecDir)
	autopusDir := filepath.Dir(specsDir)
	if filepath.Base(specsDir) != "specs" || filepath.Base(autopusDir) != ".autopus" {
		return specReviewContextScope{}, fmt.Errorf("review SPEC directory is outside .autopus/specs: %s", resolvedSpecDir)
	}
	projectRoot := filepath.Dir(autopusDir)
	relativeSpecDir, err := filepath.Rel(projectRoot, canonicalSpecDir)
	if err != nil || relativeSpecDir == "." || relativeSpecDir == ".." || strings.HasPrefix(relativeSpecDir, ".."+string(filepath.Separator)) {
		return specReviewContextScope{}, fmt.Errorf("review SPEC directory escapes project root: %s", resolvedSpecDir)
	}
	return specReviewContextScope{
		projectRoot: projectRoot,
		specDir:     filepath.ToSlash(relativeSpecDir),
	}, nil
}
