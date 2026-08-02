package omp

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

type ompConfigLayout struct{ root, skillsKey, skillsValue, customKey, customValue *yaml.Node }
type ompMarkerSpan struct{ beginLine, endLine, start, end, indent int }
type ompRawLine struct {
	start, end int
	text       string
}

// mergeOMPConfigDocument owns skills.customDirectories and preserves outside bytes.
// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-002: downgraded from ANCHOR because fan-in is below 3.
// The marker span is the complete OMP config ownership boundary; generation and removal fail closed so user-owned
// comments, sibling keys, and bytes outside skills.customDirectories are never rewritten.
func mergeOMPConfigDocument(existing string) (string, error) {
	layout, err := parseOMPConfigLayout(existing)
	if err != nil {
		return "", err
	}
	lines := splitOMPRawLines(existing)
	span, hasMarkers, err := findOMPMarkerSpan(existing, lines)
	if err != nil {
		return "", err
	}

	if hasMarkers {
		if err := validateManagedOMPSpan(layout, span); err != nil {
			return "", err
		}
		section := renderOMPManagedSection(span.indent, newlineFor(existing))
		if span.end == len(existing) && !hasTrailingNewline(existing) {
			section = strings.TrimSuffix(section, newlineFor(existing))
		}
		return existing[:span.start] + section + existing[span.end:], nil
	}

	if layout.customKey != nil {
		return "", fmt.Errorf(
			"%s의 skills.customDirectories가 Autopus 관리 마커 밖에 있어 재작성을 중단합니다", configFile)
	}
	if layout.skillsKey == nil {
		return appendRootOMPSection(existing), nil
	}
	return insertNestedOMPSection(existing, lines, layout)
}
func parseOMPConfigLayout(existing string) (ompConfigLayout, error) {
	var layout ompConfigLayout
	if strings.TrimSpace(existing) == "" {
		return layout, nil
	}

	decoder := yaml.NewDecoder(strings.NewReader(existing))
	var doc yaml.Node
	if err := decoder.Decode(&doc); err != nil {
		return layout, fmt.Errorf("%s를 YAML로 읽을 수 없어 재작성을 중단합니다: %w", configFile, err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return layout, fmt.Errorf("%s를 YAML로 읽을 수 없어 재작성을 중단합니다: %w", configFile, err)
		}
		return layout, fmt.Errorf("%s에 YAML 문서가 여러 개 있어 재작성을 중단합니다", configFile)
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return layout, fmt.Errorf("%s의 최상위 값은 mapping이어야 합니다", configFile)
	}

	root := doc.Content[0]
	layout.root = root
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "<<" {
			return layout, fmt.Errorf("%s의 top-level YAML merge key 때문에 skills 소유권이 모호합니다", configFile)
		}
		if root.Content[i].Value != "skills" {
			continue
		}
		if layout.skillsKey != nil {
			return layout, fmt.Errorf("%s에 top-level skills key가 중복되어 있습니다", configFile)
		}
		layout.skillsKey, layout.skillsValue = root.Content[i], root.Content[i+1]
	}
	if layout.skillsKey == nil {
		return layout, nil
	}
	if layout.skillsValue.Kind != yaml.MappingNode {
		return layout, fmt.Errorf("%s의 skills 값은 mapping이어야 합니다", configFile)
	}
	for i := 0; i+1 < len(layout.skillsValue.Content); i += 2 {
		if layout.skillsValue.Content[i].Value == "<<" {
			return layout, fmt.Errorf("%s의 skills YAML merge key 때문에 customDirectories 소유권이 모호합니다", configFile)
		}
		if layout.skillsValue.Content[i].Value != "customDirectories" {
			continue
		}
		if layout.customKey != nil {
			return layout, fmt.Errorf("%s에 skills.customDirectories key가 중복되어 있습니다", configFile)
		}
		layout.customKey, layout.customValue = layout.skillsValue.Content[i], layout.skillsValue.Content[i+1]
	}
	return layout, nil
}
func findOMPMarkerSpan(existing string, lines []ompRawLine) (ompMarkerSpan, bool, error) {
	var begins, ends []int
	for i, line := range lines {
		trimmed := strings.TrimSpace(line.text)
		switch trimmed {
		case markerBeginYml:
			begins = append(begins, i)
		case markerEndYml:
			ends = append(ends, i)
		}
	}
	if strings.Count(existing, markerBeginYml) != len(begins) ||
		strings.Count(existing, markerEndYml) != len(ends) {
		return ompMarkerSpan{}, false, fmt.Errorf("%s의 관리 마커 경계가 모호합니다", configFile)
	}
	if len(begins) == 0 && len(ends) == 0 {
		return ompMarkerSpan{}, false, nil
	}
	if len(begins) != 1 || len(ends) != 1 || begins[0] >= ends[0] {
		return ompMarkerSpan{}, false, fmt.Errorf(
			"%s의 관리 구간 마커가 정확한 한 쌍이 아닙니다(BEGIN %d, END %d)",
			configFile, len(begins), len(ends))
	}
	begin, end := lines[begins[0]], lines[ends[0]]
	beginIndent, okBegin := markerIndent(begin.text)
	endIndent, okEnd := markerIndent(end.text)
	if !okBegin || !okEnd || beginIndent != endIndent {
		return ompMarkerSpan{}, false, fmt.Errorf("%s의 관리 마커 indentation이 일치하지 않습니다", configFile)
	}
	return ompMarkerSpan{begins[0], ends[0], begin.start, end.end, beginIndent}, true, nil
}

func validateManagedOMPSpan(layout ompConfigLayout, span ompMarkerSpan) error {
	if layout.skillsKey == nil || layout.customKey == nil {
		return fmt.Errorf("%s의 관리 구간에 skills.customDirectories가 없습니다", configFile)
	}
	skillsLine := layout.skillsKey.Line - 1
	customLine := layout.customKey.Line - 1
	inside := func(line int) bool { return span.beginLine < line && line < span.endLine }

	if span.indent == 0 {
		if !inside(skillsLine) || !inside(customLine) {
			return fmt.Errorf("%s의 top-level 관리 마커가 skills subtree를 감싸지 않습니다", configFile)
		}
		managedKeys := 0
		for i := 0; i+1 < len(layout.root.Content); i += 2 {
			key := layout.root.Content[i]
			if inside(key.Line - 1) {
				managedKeys++
				if key != layout.skillsKey {
					return fmt.Errorf("%s의 top-level 관리 구간이 skills 외 key를 포함합니다", configFile)
				}
			}
		}
		if managedKeys != 1 || !ompNodeInsideSpan(layout.skillsValue, inside) {
			return fmt.Errorf("%s의 top-level skills subtree가 관리 구간 밖까지 이어집니다", configFile)
		}
		return nil
	}
	if skillsLine >= span.beginLine || !inside(customLine) {
		return fmt.Errorf("%s의 indented 관리 마커가 skills direct child에 있지 않습니다", configFile)
	}
	if span.indent != layout.customKey.Column-1 {
		return fmt.Errorf("%s의 관리 마커와 customDirectories indentation이 다릅니다", configFile)
	}
	if !ompNodeInsideSpan(layout.customValue, inside) {
		return fmt.Errorf("%s의 customDirectories 값이 관리 구간 밖까지 이어집니다", configFile)
	}
	for i := 0; i+1 < len(layout.skillsValue.Content); i += 2 {
		key := layout.skillsValue.Content[i]
		if inside(key.Line-1) && key != layout.customKey {
			return fmt.Errorf("%s의 indented 관리 구간이 customDirectories 외 skills key를 포함합니다", configFile)
		}
	}
	for i := 0; i+1 < len(layout.root.Content); i += 2 {
		key := layout.root.Content[i]
		if inside(key.Line - 1) {
			return fmt.Errorf("%s의 indented 관리 구간이 skills 밖의 top-level key를 포함합니다", configFile)
		}
	}
	return nil
}

func ompNodeInsideSpan(node *yaml.Node, inside func(int) bool) bool {
	if node == nil || node.Line < 1 || !inside(node.Line-1) {
		return false
	}
	for _, child := range node.Content {
		if !ompNodeInsideSpan(child, inside) {
			return false
		}
	}
	return true
}

func appendRootOMPSection(existing string) string {
	nl := newlineFor(existing)
	section := renderOMPManagedSection(0, nl)
	if existing == "" {
		return section
	}
	separator := nl + nl
	if hasTrailingNewline(existing) {
		separator = nl
		if strings.HasSuffix(existing, nl+nl) {
			separator = ""
		}
	}
	return existing + separator + section
}

func insertNestedOMPSection(existing string, lines []ompRawLine, layout ompConfigLayout) (string, error) {
	if layout.skillsValue.Style&yaml.FlowStyle != 0 || len(layout.skillsValue.Content) == 0 {
		return "", fmt.Errorf("%s의 skills mapping 형식에는 안전하게 관리 entry를 삽입할 수 없습니다", configFile)
	}
	indent := layout.skillsValue.Content[0].Column - 1
	if indent <= layout.skillsKey.Column-1 {
		return "", fmt.Errorf("%s의 skills child indentation이 올바르지 않습니다", configFile)
	}

	insertAt := len(existing)
	rootIndent := layout.skillsKey.Column - 1
	for i := layout.skillsKey.Line; i < len(lines); i++ {
		text := strings.TrimRight(lines[i].text, "\r\n")
		trimmed := strings.TrimSpace(text)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		lineIndent := len(text) - len(strings.TrimLeft(text, " "))
		if lineIndent <= rootIndent {
			insertAt = lines[i].start
			break
		}
	}
	nl := newlineFor(existing)
	section := renderOMPManagedSection(indent, nl)
	prefix := ""
	if insertAt > 0 && existing[insertAt-1] != '\n' {
		prefix = nl
	}
	return existing[:insertAt] + prefix + section + existing[insertAt:], nil
}

func renderOMPManagedSection(indent int, newline string) string {
	pad := strings.Repeat(" ", indent)
	childPad := strings.Repeat(" ", indent+2)
	if indent == 0 {
		return markerBeginYml + newline + "skills:" + newline +
			"  customDirectories:" + newline + "    - .agents/skills" + newline +
			markerEndYml + newline
	}
	return pad + markerBeginYml + newline + pad + "customDirectories:" + newline +
		childPad + "- .agents/skills" + newline + pad + markerEndYml + newline
}

func splitOMPRawLines(value string) []ompRawLine {
	if value == "" {
		return nil
	}
	var lines []ompRawLine
	for start := 0; start < len(value); {
		end := strings.IndexByte(value[start:], '\n')
		if end < 0 {
			end = len(value)
		} else {
			end += start + 1
		}
		lines = append(lines, ompRawLine{start: start, end: end, text: value[start:end]})
		start = end
	}
	return lines
}

func markerIndent(line string) (int, bool) {
	text := strings.TrimRight(line, "\r\n")
	prefix := text[:len(text)-len(strings.TrimLeft(text, " \t"))]
	if strings.Contains(prefix, "\t") {
		return 0, false
	}
	return len(prefix), true
}

func newlineFor(value string) string {
	if strings.Contains(value, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

func hasTrailingNewline(value string) bool { return strings.HasSuffix(value, "\n") }
