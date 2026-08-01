package content

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ompToolMap maps ADK tools to omp canonical tool names.
var ompToolMap = map[string]string{
	"Read":      "read",
	"Write":     "write",
	"Edit":      "edit",
	"Grep":      "grep",
	"Glob":      "glob",
	"Bash":      "bash",
	"TodoWrite": "todo",
	"WebSearch": "web_search",
	"WebFetch":  "web_search",
}

// TransformAgentForOMP produces an OMP markdown template from an agent source.
func TransformAgentForOMP(src AgentSource) string {
	var sb strings.Builder

	body := NormalizeAgentReferences(src.Body, "omp")

	// tools mapping
	var tools []string
	if src.Meta.Tools != "" {
		seen := make(map[string]bool)
		for _, t := range strings.Split(src.Meta.Tools, ",") {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			if strings.HasPrefix(t, "mcp__") {
				if !seen[t] {
					seen[t] = true
					tools = append(tools, t)
				}
				continue
			}
			if mapped, ok := ompToolMap[t]; ok {
				if !seen[mapped] {
					seen[mapped] = true
					tools = append(tools, mapped)
				}
			}
		}
		sort.Strings(tools)
	}

	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "name: %s\n", OMPYAMLScalar(src.Meta.Name))
	if src.Meta.Description != "" {
		fmt.Fprintf(&sb, "description: %s\n", OMPYAMLScalar(src.Meta.Description))
	}
	if src.Meta.Model != "" {
		fmt.Fprintf(&sb, "model: %s\n", OMPYAMLScalar(src.Meta.Model))
	}
	if len(tools) > 0 {
		sb.WriteString("tools:\n")
		for _, tool := range tools {
			fmt.Fprintf(&sb, "  - %s\n", tool)
		}
	}
	sb.WriteString("---\n\n")
	sb.WriteString(body)
	sb.WriteString("\n")

	return sb.String()
}

// OMPYAMLScalar renders a value as a YAML scalar for omp frontmatter. Emitting the raw
// string let a value containing ": ", a newline, or a leading "tools:" line
// close the field early and inject sibling frontmatter keys that omp would then
// honor. Marshalling quotes only when the value would otherwise change the
// document structure, so ordinary values keep their unquoted form.
func OMPYAMLScalar(value string) string {
	encoded, err := yaml.Marshal(value)
	if err != nil {
		return strconv.Quote(value)
	}
	return strings.TrimRight(string(encoded), "\n")
}
