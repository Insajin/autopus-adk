package design

import (
	"strings"

	"gopkg.in/yaml.v3"
)

type designFrontmatter struct {
	SourceOfTruth []string `yaml:"source_of_truth"`
}

func parseSourceOfTruth(content string) []string {
	if !strings.HasPrefix(content, "---\n") {
		return nil
	}
	rest := strings.TrimPrefix(content, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil
	}
	var fm designFrontmatter
	if err := yaml.Unmarshal([]byte(rest[:end]), &fm); err != nil {
		return nil
	}
	return fm.SourceOfTruth
}
