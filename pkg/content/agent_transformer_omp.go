package content

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"

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

// OMPAgentModelSelection is the validated model tuple rendered for an opt-in
// OMP role-model policy. Model must be a native @role alias.
type OMPAgentModelSelection struct {
	Model    string
	Thinking string
}

// TransformAgentForOMP produces an OMP markdown template without selecting a
// child model. Legacy Claude sonnet/opus labels are not OMP selectors, so their
// omission intentionally inherits the parent session model.
func TransformAgentForOMP(src AgentSource) string {
	model := src.Meta.Model
	if model == "sonnet" || model == "opus" {
		model = ""
	}
	return renderAgentForOMP(src, model, "")
}

// TransformAgentForOMPWithModel produces an OMP agent using an explicitly
// compiled native role alias. Only a validated opt-in selection emits both the
// @role model and thinking fields.
func TransformAgentForOMPWithModel(src AgentSource, selection OMPAgentModelSelection) (string, error) {
	if err := validateOMPAgentModelSelection(selection); err != nil {
		return "", err
	}
	return renderAgentForOMP(src, selection.Model, selection.Thinking), nil
}

// @AX:WARN [AUTO]: OMP agent rendering contains 10 if branches.
// @AX:REASON [AUTO]: optional metadata, tools, permissions, model, thinking, and body sections jointly define emitted frontmatter.
func renderAgentForOMP(src AgentSource, model, thinking string) string {
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
	if model != "" {
		fmt.Fprintf(&sb, "model: %s\n", OMPYAMLScalar(model))
	}
	if thinking != "" {
		fmt.Fprintf(&sb, "thinking: %s\n", OMPYAMLScalar(thinking))
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

func validateOMPAgentModelSelection(selection OMPAgentModelSelection) error {
	role := strings.TrimPrefix(selection.Model, "@")
	if selection.Model == "" || role == selection.Model || !isOMPSafeIdentifier(role) {
		return errors.New("OMP agent model must be a safe native @role alias")
	}
	if !isOMPThinkingLevel(selection.Thinking) {
		return fmt.Errorf("unsupported OMP thinking level %q", selection.Thinking)
	}
	return nil
}

func isOMPSafeIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for i, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') ||
			(i > 0 && (char == '-' || char == '_')) {
			continue
		}
		return false
	}
	return true
}

func isOMPThinkingLevel(value string) bool {
	return config.IsOMPNativeThinkingLevel(value)
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
