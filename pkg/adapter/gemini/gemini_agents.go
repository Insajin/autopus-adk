// Package gemini provides agent content file management for Gemini CLI.
package gemini

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/templates"
)

const agentsTemplateDir = "gemini/agents"

// renderAgentFiles renders transformed agent templates from templates/gemini/agents/
// to .gemini/agents/autopus/ and returns file mappings.
func (a *Adapter) renderAgentFiles() ([]adapter.FileMapping, error) {
	targetRelDir := filepath.Join(".gemini", "agents", "autopus")
	absTargetDir := filepath.Join(a.root, targetRelDir)
	if err := os.MkdirAll(absTargetDir, 0755); err != nil {
		return nil, fmt.Errorf("gemini agents directory creation failed: %w", err)
	}

	mappings, err := a.prepareAgentMappings()
	if err != nil {
		return nil, err
	}

	for _, m := range mappings {
		destPath := filepath.Join(a.root, m.TargetPath)
		if err := os.WriteFile(destPath, m.Content, 0644); err != nil {
			return nil, fmt.Errorf("gemini agent file write failed %s: %w", destPath, err)
		}
	}

	return mappings, nil
}

// prepareAgentMappings reads pre-transformed agent templates and returns file mappings
// without writing to disk. Uses templates/gemini/agents/*.md.tmpl which have
// tool references already mapped (Agent() → @agent, .claude/ → .gemini/, etc).
func (a *Adapter) prepareAgentMappings() ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	entries, err := templates.FS.ReadDir(agentsTemplateDir)
	if err != nil {
		return nil, fmt.Errorf("gemini agent template directory read failed: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tmpl") {
			continue
		}

		data, err := fs.ReadFile(templates.FS, agentsTemplateDir+"/"+entry.Name())
		if err != nil {
			return nil, fmt.Errorf("gemini agent template read failed %s: %w", entry.Name(), err)
		}

		// Strip .tmpl extension for the output filename
		outputName := strings.TrimSuffix(entry.Name(), ".tmpl")
		relPath := filepath.Join(".gemini", "agents", "autopus", outputName)
		files = append(files, adapter.FileMapping{
			TargetPath:      relPath,
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(string(data)),
			Content:         data,
		})
	}

	return files, nil
}
