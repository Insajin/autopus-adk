package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

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
	if opts == nil {
		opts = &GenerateOptions{}
	}

	docsDir := resolveDocsDir(projectDir, opts.OutputDir)

	// Check if docs already exist
	if !opts.Force {
		if _, err := os.Stat(docsDir); err == nil {
			return nil, fmt.Errorf("documentation already exists at %s. Use --force to overwrite", docsDir)
		}
	}

	// Scan project
	info, err := Scan(projectDir)
	if err != nil {
		return nil, fmt.Errorf("scan project: %w", err)
	}

	// Render documents
	docSet := Render(info, opts.Render)

	// Create docs directory
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		return nil, fmt.Errorf("create docs directory: %w", err)
	}

	// Write all documents
	meta := NewMeta(projectDir)
	if err := writeDocSet(docsDir, projectDir, docSet, meta, info); err != nil {
		return nil, fmt.Errorf("write documents: %w", err)
	}

	// Generate scenarios.md
	if err := generateScenarios(projectDir, info); err != nil {
		return nil, fmt.Errorf("generate scenarios: %w", err)
	}

	// Generate signature map
	if err := generateSignatureMap(projectDir, opts.Config); err != nil {
		return nil, fmt.Errorf("generate signature map: %w", err)
	}

	// Save meta
	if err := SaveMeta(docsDir, meta); err != nil {
		return nil, fmt.Errorf("save meta: %w", err)
	}

	return docSet, nil
}

// Update regenerates only documents whose source data has changed.
func Update(projectDir string, outputDir string) ([]string, error) {
	docsDir := resolveDocsDir(projectDir, outputDir)

	meta, err := LoadMeta(docsDir)
	if err != nil {
		_, genErr := Generate(projectDir, &GenerateOptions{
			OutputDir: outputDir,
			Force:     true,
		})
		if genErr != nil {
			return nil, genErr
		}
		return []string{"all (full regeneration due to missing/corrupted .meta.yaml)"}, nil
	}

	info, err := Scan(projectDir)
	if err != nil {
		return nil, fmt.Errorf("scan project: %w", err)
	}

	docSet := Render(info, nil)
	docContents := renderDocContents(docSet)
	var updated []string

	if meta.Files == nil {
		meta.Files = make(map[string]FileMeta)
	}

	for fileName, content := range docContents {
		if meta.HasContentChanged(fileName, content) {
			if err := os.WriteFile(filepath.Join(docsDir, fileName), []byte(content), 0644); err != nil {
				return nil, fmt.Errorf("write %s: %w", fileName, err)
			}
			meta.Files[fileName] = FileMeta{
				ContentHash: hashString(content),
			}
			updated = append(updated, fileName)
		}
	}

	_ = generateScenarios(projectDir, info)

	sigUpdated, sigErr := updateSignatureMap(projectDir, nil)
	if sigErr != nil {
		return nil, fmt.Errorf("update signature map: %w", sigErr)
	}
	if sigUpdated {
		updated = append(updated, signaturesFile)
	}

	if len(updated) > 0 {
		meta.GeneratedAt = time.Now().UTC()
		meta.ProjectHash = hashProjectStructure(projectDir)
		if err := SaveMeta(docsDir, meta); err != nil {
			return nil, fmt.Errorf("save meta: %w", err)
		}
	}

	return updated, nil
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
