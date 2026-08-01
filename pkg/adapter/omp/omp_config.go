package omp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

const (
	markerBeginYml = "# AUTOPUS:BEGIN"
	markerEndYml   = "# AUTOPUS:END"
	configFile     = ".omp/config.yml"
)

func (a *Adapter) prepareConfigMapping(cfg *config.HarnessConfig) (adapter.FileMapping, error) {
	configDoc, err := a.renderConfigDocument(cfg)
	if err != nil {
		return adapter.FileMapping{}, err
	}
	return adapter.FileMapping{
		TargetPath:      configFile,
		OverwritePolicy: adapter.OverwriteMarker,
		Checksum:        adapter.Checksum(configDoc),
		Content:         []byte(configDoc),
	}, nil
}

func (a *Adapter) renderConfigDocument(cfg *config.HarnessConfig) (string, error) {
	// This read feeds the document that gets written back and checksummed, so a
	// link here would merge an unrelated file's contents into managed output.
	// Fail closed rather than letting the later write guard catch it, so the
	// error names the real problem and the foreign file is never read at all.
	if err := adapter.RejectSymlinkComponents(a.root, configFile); err != nil {
		return "", fmt.Errorf("%s 경로가 심링크를 지나갑니다: %w", configFile, err)
	}

	path := filepath.Join(a.root, configFile)
	var existing string
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	}

	if err := validateMarkerBalance(existing); err != nil {
		return "", err
	}

	sectionContent := `skills:
  customDirectories:
    - .agents/skills`

	newSection := markerBeginYml + "\n" + sectionContent + "\n" + markerEndYml

	if markerReYml.MatchString(existing) {
		return strings.TrimRight(markerReYml.ReplaceAllString(existing, newSection+"\n"), "\n") + "\n", nil
	}

	if existing == "" {
		return newSection + "\n", nil
	}
	return strings.TrimSpace(existing) + "\n\n" + newSection + "\n", nil
}
