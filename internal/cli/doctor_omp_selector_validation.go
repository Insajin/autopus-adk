package cli

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

const ompDoctorAgentMaxSize = 64 * 1024

type ompDoctorSelectorCollection struct {
	selectors []string
	findings  []adapter.ValidationError
}

func collectOMPDoctorSelectors(root string) ompDoctorSelectorCollection {
	var result ompDoctorSelectorCollection
	dir := filepath.Join(root, ".omp", "agents")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return result
	}
	if err != nil {
		result.findings = append(result.findings,
			ompDoctorSelectorFinding(filepath.Join(".omp", "agents"), "agent_directory_unreadable"))
		return result
	}

	seen := make(map[string]bool)
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		rel := filepath.Join(".omp", "agents", entry.Name())
		data, reason := readOMPDoctorAgent(filepath.Join(root, rel), entry)
		if reason != "" {
			result.findings = append(result.findings, ompDoctorSelectorFinding(rel, reason))
			continue
		}
		selector, reason := parseOMPDoctorFrontmatterModel(data)
		if reason != "" {
			result.findings = append(result.findings, ompDoctorSelectorFinding(rel, reason))
			continue
		}
		if selector != "" {
			seen[selector] = true
		}
	}
	for selector := range seen {
		result.selectors = append(result.selectors, selector)
	}
	sort.Strings(result.selectors)
	sort.SliceStable(result.findings, func(i, j int) bool {
		if result.findings[i].File == result.findings[j].File {
			return result.findings[i].Message < result.findings[j].Message
		}
		return result.findings[i].File < result.findings[j].File
	})
	return result
}

func readOMPDoctorAgent(path string, entry os.DirEntry) ([]byte, string) {
	info, err := entry.Info()
	if err != nil {
		return nil, "agent_stat_failed"
	}
	if !info.Mode().IsRegular() {
		return nil, "agent_non_regular"
	}
	if info.Size() > ompDoctorAgentMaxSize {
		return nil, "agent_oversized"
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, "agent_unreadable"
	}
	data, readErr := io.ReadAll(io.LimitReader(file, ompDoctorAgentMaxSize+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return nil, "agent_unreadable"
	}
	if len(data) > ompDoctorAgentMaxSize {
		return nil, "agent_oversized"
	}
	return data, ""
}

func parseOMPDoctorFrontmatterModel(data []byte) (string, string) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return "", "frontmatter_missing"
	}
	end := strings.Index(text[4:], "\n---\n")
	if end < 0 {
		return "", "frontmatter_malformed"
	}
	var frontmatter struct {
		Model string `yaml:"model"`
	}
	if yaml.Unmarshal([]byte(text[4:4+end]), &frontmatter) != nil {
		return "", "frontmatter_malformed"
	}
	selector := strings.TrimSpace(frontmatter.Model)
	if selector == "" {
		return "", ""
	}
	if !ompDoctorSafeToken.MatchString(selector) || strings.Contains(selector, "//") ||
		strings.HasSuffix(selector, "/") {
		return "", "model_malformed"
	}
	return selector, ""
}

func ompDoctorSelectorFinding(path, reason string) adapter.ValidationError {
	return adapter.ValidationError{
		File: filepath.ToSlash(path), Message: "agent selector validation failed reason=" + reason,
		Level: "error",
	}
}
