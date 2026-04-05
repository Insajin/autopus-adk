package content

import "strings"

// condenseBody strips code blocks and reduces markdown body to key content.
// Keeps H2/H3 headers and their first few content lines, drops code fences.
func condenseBody(body string) string {
	lines := strings.Split(body, "\n")
	var result []string
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Toggle code block state
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}

		// Skip empty lines and H1 headers (title already in description)
		if trimmed == "" || strings.HasPrefix(trimmed, "# ") {
			continue
		}

		result = append(result, trimmed)
	}

	return strings.Join(result, " ")
}
