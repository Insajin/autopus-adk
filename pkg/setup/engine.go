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

	// Load existing meta
	meta, err := LoadMeta(docsDir)
	if err != nil {
		// Corrupted or missing meta — full regeneration
		_, genErr := Generate(projectDir, &GenerateOptions{
			OutputDir: outputDir,
			Force:     true,
		})
		if genErr != nil {
			return nil, genErr
		}
		return []string{"all (full regeneration due to missing/corrupted .meta.yaml)"}, nil
	}

	// Scan current project state
	info, err := Scan(projectDir)
	if err != nil {
		return nil, fmt.Errorf("scan project: %w", err)
	}

	// Check which documents need updating
	docSet := Render(info, nil)
	var updated []string

	docContents := map[string]string{
		"index.md":        docSet.Index,
		"commands.md":     docSet.Commands,
		"structure.md":    docSet.Structure,
		"conventions.md":  docSet.Conventions,
		"boundaries.md":   docSet.Boundaries,
		"architecture.md": docSet.Architecture,
		"testing.md":      docSet.Testing,
	}

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

	// Update scenarios.md (incremental sync — non-fatal on error)
	_ = generateScenarios(projectDir, info)

	// Update signature map
	sigUpdated, sigErr := updateSignatureMap(projectDir, nil)
	if sigErr != nil {
		return nil, fmt.Errorf("update signature map: %w", sigErr)
	}
	if sigUpdated {
		updated = append(updated, signaturesFile)
	}

	// Update meta timestamp
	if len(updated) > 0 {
		meta.GeneratedAt = time.Now().UTC()
		meta.ProjectHash = hashProjectStructure(projectDir)
		if err := SaveMeta(docsDir, meta); err != nil {
			return nil, fmt.Errorf("save meta: %w", err)
		}
	}

	return updated, nil
}

// Status returns the documentation status.
type Status struct {
	Exists       bool
	GeneratedAt  time.Time
	FileStatuses map[string]FileStatus
	DriftScore   float64
}

// FileStatus represents the status of a single documentation file.
type FileStatus struct {
	Exists  bool
	Fresh   bool // Content matches current project state
	ModTime time.Time
}

// GetStatus returns the current documentation status.
func GetStatus(projectDir string, outputDir string) (*Status, error) {
	docsDir := resolveDocsDir(projectDir, outputDir)

	status := &Status{
		FileStatuses: make(map[string]FileStatus),
	}

	if _, err := os.Stat(docsDir); err != nil {
		return status, nil
	}
	status.Exists = true

	// Load meta
	meta, err := LoadMeta(docsDir)
	if err != nil {
		// Docs exist but meta is broken
		for _, fileName := range DocFiles {
			docPath := filepath.Join(docsDir, fileName)
			fi, err := os.Stat(docPath)
			status.FileStatuses[fileName] = FileStatus{
				Exists: err == nil,
				Fresh:  false,
			}
			if err == nil {
				status.FileStatuses[fileName] = FileStatus{
					Exists:  true,
					Fresh:   false,
					ModTime: fi.ModTime(),
				}
			}
		}
		return status, nil
	}

	status.GeneratedAt = meta.GeneratedAt

	// Check freshness of each file
	info, err := Scan(projectDir)
	if err != nil {
		return status, nil
	}

	docSet := Render(info, nil)
	docContents := map[string]string{
		"index.md":        docSet.Index,
		"commands.md":     docSet.Commands,
		"structure.md":    docSet.Structure,
		"conventions.md":  docSet.Conventions,
		"boundaries.md":   docSet.Boundaries,
		"architecture.md": docSet.Architecture,
		"testing.md":      docSet.Testing,
	}

	staleCount := 0
	for fileName, content := range docContents {
		docPath := filepath.Join(docsDir, fileName)
		fi, err := os.Stat(docPath)
		fresh := !meta.HasContentChanged(fileName, content)
		if !fresh {
			staleCount++
		}

		fs := FileStatus{
			Exists: err == nil,
			Fresh:  fresh,
		}
		if err == nil {
			fs.ModTime = fi.ModTime()
		}
		status.FileStatuses[fileName] = fs
	}

	if len(docContents) > 0 {
		status.DriftScore = float64(staleCount) / float64(len(docContents))
	}

	return status, nil
}

func writeDocSet(docsDir, projectDir string, ds *DocSet, meta *Meta, info *ProjectInfo) error {
	docs := map[string]struct {
		content     string
		sourceFiles []string
	}{
		"index.md":        {ds.Index, docSourceFiles(info, "index")},
		"commands.md":     {ds.Commands, docSourceFiles(info, "commands")},
		"structure.md":    {ds.Structure, docSourceFiles(info, "structure")},
		"conventions.md":  {ds.Conventions, docSourceFiles(info, "conventions")},
		"boundaries.md":   {ds.Boundaries, docSourceFiles(info, "boundaries")},
		"architecture.md": {ds.Architecture, docSourceFiles(info, "architecture")},
		"testing.md":      {ds.Testing, docSourceFiles(info, "testing")},
	}

	for fileName, doc := range docs {
		path := filepath.Join(docsDir, fileName)
		if err := os.WriteFile(path, []byte(doc.content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", fileName, err)
		}
		meta.SetFileMeta(fileName, doc.content, doc.sourceFiles, projectDir)
	}

	return nil
}

func docSourceFiles(info *ProjectInfo, docType string) []string {
	var files []string
	switch docType {
	case "index", "structure":
		// All build files contribute to index
		for _, bf := range info.BuildFiles {
			files = append(files, bf.Path)
		}
	case "commands":
		for _, bf := range info.BuildFiles {
			files = append(files, bf.Path)
		}
	case "conventions", "boundaries", "architecture", "testing":
		for _, bf := range info.BuildFiles {
			files = append(files, bf.Path)
		}
	}
	return files
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
