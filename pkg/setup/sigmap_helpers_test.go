//go:build integration

package setup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/config"
)

func generateSignatureMap(projectDir string, cfg *config.HarnessConfig) error {
	content, enabled, err := renderSignatureMapContent(projectDir, cfg)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	return writeGeneratedFile(filepath.Join(projectDir, signaturesDir, signaturesFile), content)
}

func updateSignatureMap(projectDir string, cfg *config.HarnessConfig) (bool, error) {
	content, enabled, err := renderSignatureMapContent(projectDir, cfg)
	if err != nil {
		return false, err
	}
	if !enabled {
		return false, nil
	}

	outPath := filepath.Join(projectDir, signaturesDir, signaturesFile)
	oldContent, _ := os.ReadFile(outPath)
	if string(oldContent) == string(content) {
		return false, nil
	}
	if err := writeGeneratedFile(outPath, content); err != nil {
		return false, fmt.Errorf("write signatures: %w", err)
	}

	return true, nil
}
