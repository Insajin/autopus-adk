package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/insajin/autopus-adk/pkg/config"
)

var qualityDefaultLineRe = regexp.MustCompile(`^(\s*default\s*:\s*)([^#\r\n]*?)(\s*(?:#.*)?)$`)

func saveQualityDefault(dir string, cfg *config.HarnessConfig, preset string) error {
	cfg.Quality.Default = preset
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	path := filepath.Join(dir, "autopus.yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return config.Save(dir, cfg)
	}
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	line, ok := findQualityDefaultLine(data)
	if !ok {
		return config.Save(dir, cfg)
	}
	updated, err := replaceQualityDefaultLine(data, line, preset)
	if err != nil {
		return err
	}

	mode := os.FileMode(0644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode()
	}
	if err := os.WriteFile(path, updated, mode); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if _, err := config.LoadPreview(dir); err != nil {
		return fmt.Errorf("validate written config: %w", err)
	}
	return nil
}

func findQualityDefaultLine(data []byte) (int, bool) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || len(doc.Content) == 0 {
		return 0, false
	}
	root := doc.Content[0]
	quality := yamlMappingValue(root, "quality")
	if quality == nil {
		return 0, false
	}
	defaultNode := yamlMappingValue(quality, "default")
	if defaultNode == nil || defaultNode.Line <= 0 {
		return 0, false
	}
	return defaultNode.Line - 1, true
}

func yamlMappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func replaceQualityDefaultLine(data []byte, lineIdx int, preset string) ([]byte, error) {
	lines := strings.SplitAfter(string(data), "\n")
	if lineIdx < 0 || lineIdx >= len(lines) {
		return nil, fmt.Errorf("quality.default line index out of range")
	}
	line := lines[lineIdx]
	newline := ""
	if strings.HasSuffix(line, "\n") {
		newline = "\n"
		line = strings.TrimSuffix(line, "\n")
	}
	carriage := ""
	if strings.HasSuffix(line, "\r") {
		carriage = "\r"
		line = strings.TrimSuffix(line, "\r")
	}
	matches := qualityDefaultLineRe.FindStringSubmatch(line)
	if matches == nil {
		return nil, fmt.Errorf("quality.default line has unsupported format")
	}
	lines[lineIdx] = matches[1] + preset + matches[3] + carriage + newline
	return []byte(strings.Join(lines, "")), nil
}
