package compress

import (
	"fmt"
	"regexp"
	"strings"
)

// Section headers recognized in phase output.
var sectionPatterns = map[string]*regexp.Regexp{
	"goal":     regexp.MustCompile(`(?i)^##?\s*(goal|objective|task)\s*$`),
	"progress": regexp.MustCompile(`(?i)^##?\s*(progress|actions?|done|completed|changes?)\s*$`),
	"decision": regexp.MustCompile(`(?i)^##?\s*(decision|design|architecture)\s*$`),
	"files":    regexp.MustCompile(`(?i)^##?\s*(files?\s*(modified|changed)?|modified\s*files?)\s*$`),
	"next":     regexp.MustCompile(`(?i)^##?\s*(next\s*steps?|todo|remaining)\s*$`),
}

// filePattern matches file paths in text (e.g., "pkg/worker/compress/budget.go").
var filePattern = regexp.MustCompile(`(?:^|\s)([\w./\-]+\.\w{1,5})`)

// Summarize generates a structured summary from phase output text.
// The summary follows the template: Goal / Progress / Decisions / Files / Next Steps.
func Summarize(phaseName, text string, maxTokens int) string {
	sections := extractSections(text)
	files := extractFiles(text)

	var sb strings.Builder
	fmt.Fprintf(&sb, "## Phase Summary: %s\n\n", phaseName)

	writeSection(&sb, "Goal", sections["goal"], "Phase output analysis")
	writeSection(&sb, "Progress", sections["progress"], "Completed phase tasks")
	writeSection(&sb, "Decisions", sections["decision"], "No explicit decisions recorded")
	writeFileSection(&sb, files)
	writeSection(&sb, "Next Steps", sections["next"], "Continue to next phase")

	result := sb.String()
	return truncateToTokens(result, maxTokens)
}

// extractSections parses text looking for known markdown section headers
// and captures content under each.
func extractSections(text string) map[string]string {
	lines := strings.Split(text, "\n")
	sections := make(map[string]string)
	currentSection := ""
	var content []string

	for _, line := range lines {
		matched := false
		for name, pat := range sectionPatterns {
			if pat.MatchString(strings.TrimSpace(line)) {
				if currentSection != "" && len(content) > 0 {
					sections[currentSection] = strings.TrimSpace(strings.Join(content, "\n"))
				}
				currentSection = name
				content = nil
				matched = true
				break
			}
		}
		if !matched && currentSection != "" {
			content = append(content, line)
		}
	}

	if currentSection != "" && len(content) > 0 {
		sections[currentSection] = strings.TrimSpace(strings.Join(content, "\n"))
	}
	return sections
}

// extractFiles finds file paths mentioned in the text.
func extractFiles(text string) []string {
	matches := filePattern.FindAllStringSubmatch(text, -1)
	seen := make(map[string]bool)
	var files []string
	for _, m := range matches {
		path := m[1]
		if !strings.Contains(path, "/") {
			continue // skip bare filenames without directory
		}
		if !seen[path] {
			seen[path] = true
			files = append(files, path)
		}
	}
	return files
}

func writeSection(sb *strings.Builder, title, content, fallback string) {
	fmt.Fprintf(sb, "### %s\n", title)
	if content != "" {
		fmt.Fprintf(sb, "%s\n\n", content)
	} else {
		fmt.Fprintf(sb, "%s\n\n", fallback)
	}
}

func writeFileSection(sb *strings.Builder, files []string) {
	sb.WriteString("### Files Modified\n")
	if len(files) == 0 {
		sb.WriteString("No file paths detected\n\n")
		return
	}
	for _, f := range files {
		fmt.Fprintf(sb, "- %s\n", f)
	}
	sb.WriteString("\n")
}

// truncateToTokens truncates text to fit within token budget.
func truncateToTokens(text string, maxTokens int) string {
	if maxTokens <= 0 || EstimateTokens(text) <= maxTokens {
		return text
	}
	maxChars := maxTokens * 4
	if maxChars >= len(text) {
		return text
	}
	return text[:maxChars] + "\n\n[...truncated to fit summary budget]\n"
}
