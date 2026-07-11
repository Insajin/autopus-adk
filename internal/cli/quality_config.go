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

var renameQualityConfig = os.Rename

func saveQualityDefault(dir string, cfg *config.HarnessConfig, preset string) error {
	cfg.Quality.Default = preset
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	return saveQualityScalar(dir, cfg, "default", preset)
}

func saveQualitySupervisorPolicy(dir string, cfg *config.HarnessConfig, policy string) error {
	cfg.Quality.SupervisorModelPolicy = policy
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	return saveQualityScalar(dir, cfg, "supervisor_model_policy", policy)
}

func saveQualityScalar(dir string, cfg *config.HarnessConfig, key, value string) error {
	path := filepath.Join(dir, "autopus.yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		data, marshalErr := yaml.Marshal(cfg)
		if marshalErr != nil {
			return fmt.Errorf("marshal config: %w", marshalErr)
		}
		if writeErr := atomicWriteQualityConfig(path, data); writeErr != nil {
			return fmt.Errorf("write config: %w", writeErr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	updated, err := updateQualityScalar(data, key, value)
	if err != nil {
		return err
	}
	if err := validateQualityYAML(updated); err != nil {
		return err
	}
	if err := atomicWriteQualityConfig(path, updated); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func updateQualityScalar(data []byte, key, value string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("config root must be a YAML mapping")
	}

	root := doc.Content[0]
	qualityKey, quality := yamlMappingPair(root, "quality")
	if quality == nil {
		return appendQualitySection(data, key, value), nil
	}
	if quality.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("quality must be a YAML mapping")
	}
	if fieldKey, field := yamlMappingPair(quality, key); field != nil {
		line := field.Line
		if fieldKey.Line > 0 {
			line = fieldKey.Line
		}
		return replaceQualityScalarLine(data, line-1, key, value)
	}

	lines := strings.SplitAfter(string(data), "\n")
	newline := detectConfigNewline(data)
	indent := strings.Repeat(" ", max(qualityKey.Column-1, 0)+2)
	insertAt := qualityKey.Line
	if len(quality.Content) > 0 {
		firstKey := quality.Content[0]
		insertAt = firstKey.Line - 1
		indent = lineIndent(lines, insertAt, indent)
	}
	if key != "default" {
		if defaultKey, defaultValue := yamlMappingPair(quality, "default"); defaultValue != nil {
			insertAt = defaultKey.Line
			indent = lineIndent(lines, defaultKey.Line-1, indent)
		}
	}
	return insertConfigLine(lines, insertAt, indent+key+": "+value+newline), nil
}

func findQualityDefaultLine(data []byte) (int, bool) {
	line, ok := findQualityScalarLine(data, "default")
	return line, ok
}

func findQualityScalarLine(data []byte, key string) (int, bool) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || len(doc.Content) == 0 {
		return 0, false
	}
	_, quality := yamlMappingPair(doc.Content[0], "quality")
	_, field := yamlMappingPair(quality, key)
	if field == nil || field.Line <= 0 {
		return 0, false
	}
	return field.Line - 1, true
}

func yamlMappingPair(node *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil, nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i], node.Content[i+1]
		}
	}
	return nil, nil
}

func replaceQualityDefaultLine(data []byte, lineIdx int, preset string) ([]byte, error) {
	return replaceQualityScalarLine(data, lineIdx, "default", preset)
}

func replaceQualityScalarLine(data []byte, lineIdx int, key, value string) ([]byte, error) {
	lines := strings.SplitAfter(string(data), "\n")
	if lineIdx < 0 || lineIdx >= len(lines) {
		return nil, fmt.Errorf("quality.%s line index out of range", key)
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
	quotedKey := regexp.QuoteMeta(key)
	lineRE := regexp.MustCompile(`^(\s*(?:` + quotedKey + `|"` + quotedKey + `"|'` + quotedKey + `')\s*:\s*)([^#\r\n]*?)(\s*(?:#.*)?)$`)
	matches := lineRE.FindStringSubmatch(line)
	if matches == nil {
		return nil, fmt.Errorf("quality.%s line has unsupported format", key)
	}
	lines[lineIdx] = matches[1] + value + matches[3] + carriage + newline
	return []byte(strings.Join(lines, "")), nil
}

func appendQualitySection(data []byte, key, value string) []byte {
	content := string(data)
	newline := detectConfigNewline(data)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += newline
	}
	return []byte(content + "quality:" + newline + "  " + key + ": " + value + newline)
}

func insertConfigLine(lines []string, index int, line string) []byte {
	if index < 0 {
		index = 0
	}
	if index > len(lines) {
		index = len(lines)
	}
	if index > 0 && !strings.HasSuffix(lines[index-1], "\n") {
		lines[index-1] += detectConfigNewline([]byte(strings.Join(lines, "")))
	}
	updated := make([]string, 0, len(lines)+1)
	updated = append(updated, lines[:index]...)
	updated = append(updated, line)
	updated = append(updated, lines[index:]...)
	return []byte(strings.Join(updated, ""))
}

func lineIndent(lines []string, index int, fallback string) string {
	if index < 0 || index >= len(lines) {
		return fallback
	}
	line := strings.TrimSuffix(strings.TrimSuffix(lines[index], "\n"), "\r")
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}

func detectConfigNewline(data []byte) string {
	if strings.Contains(string(data), "\r\n") {
		return "\r\n"
	}
	return "\n"
}

func validateQualityYAML(data []byte) error {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("validate written config: %w", err)
	}
	return nil
}

func atomicWriteQualityConfig(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".autopus-quality-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := renameQualityConfig(tmpPath, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}
