package setup

import (
	"path/filepath"
	"sort"

	"github.com/insajin/autopus-adk/pkg/config"
)

const defaultDocsDir = ".autopus/docs"

// GenerateOptions holds options for document generation.
type GenerateOptions struct {
	OutputDir string
	Force     bool
	Render    *RenderOptions
	Config    *config.HarnessConfig // optional; controls sigmap generation
}

// Generate creates all documentation files for the project.
func Generate(projectDir string, opts *GenerateOptions) (*DocSet, error) {
	plan, err := BuildGeneratePlan(projectDir, opts)
	if err != nil {
		return nil, err
	}
	result, err := ApplyChangePlan(plan)
	if err != nil {
		return nil, err
	}
	return result.DocSet, nil
}

// Update regenerates only documents whose source data has changed.
func Update(projectDir string, outputDir string) ([]string, error) {
	plan, err := BuildUpdatePlan(projectDir, outputDir)
	if err != nil {
		return nil, err
	}
	result, err := ApplyChangePlan(plan)
	if err != nil {
		return nil, err
	}
	return legacyUpdatedFiles(plan, result), nil
}

func resolveDocsDir(projectDir, outputDir string) string {
	if outputDir != "" {
		if filepath.IsAbs(outputDir) {
			return outputDir
		}
		return filepath.Join(projectDir, outputDir)
	}
	return filepath.Join(projectDir, defaultDocsDir)
}

func legacyUpdatedFiles(plan *ChangePlan, result *ApplyResult) []string {
	if plan.FullRegeneration {
		return []string{"all (full regeneration due to missing/corrupted .meta.yaml)"}
	}
	if result == nil || len(result.ChangedPaths) == 0 {
		return nil
	}

	updated := make([]string, 0, len(result.ChangedPaths))
	for _, path := range result.ChangedPaths {
		base := filepath.Base(path)
		if base == metaFileName {
			continue
		}
		updated = append(updated, base)
	}
	sort.Strings(updated)
	return updated
}
