package orchestra

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// numberedItemRe matches numbered list items like "1. item", "1) item".
var numberedItemRe = regexp.MustCompile(`^\s*(\d+)[.)]\s+(.+)`)

// bulletItemRe matches bullet list items like "- item".
var bulletItemRe = regexp.MustCompile(`^\s*[-*]\s+(.+)`)

// buildStructuredPromptPrefix returns a prompt prefix requesting structured output.
func buildStructuredPromptPrefix() string {
	return "Please respond with a numbered list (e.g., 1. item, 2. item). " +
		"Each point on its own line.\n\n"
}

// parseStructuredResponse extracts numbered items from a response.
// Supports "1. item", "1) item", "- item" formats.
// Returns a map from index (1-based) to item text, or error if no items found.
func parseStructuredResponse(output string) (map[int]string, error) {
	lines := strings.Split(output, "\n")
	result := make(map[int]string)
	bulletIdx := 1

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if m := numberedItemRe.FindStringSubmatch(line); m != nil {
			idx, _ := strconv.Atoi(m[1])
			result[idx] = strings.TrimSpace(m[2])
			continue
		}
		if m := bulletItemRe.FindStringSubmatch(line); m != nil {
			result[bulletIdx] = strings.TrimSpace(m[1])
			bulletIdx++
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no structured items found in response")
	}
	return result, nil
}

// MergeStructuredConsensus attempts structured comparison of provider responses.
// Returns (merged output, summary). Falls back to line-based if parsing fails.
func MergeStructuredConsensus(responses []ProviderResponse, threshold float64) (string, string) {
	if len(responses) == 0 {
		return "", ""
	}

	// Try to parse all responses as structured
	parsed := make([]map[int]string, len(responses))
	for i, r := range responses {
		items, err := parseStructuredResponse(r.Output)
		if err != nil {
			// Any failure → fall back to line-based
			return "", ""
		}
		parsed[i] = items
	}

	// Collect all unique keys across all responses
	keySet := make(map[int]bool)
	for _, items := range parsed {
		for k := range items {
			keySet[k] = true
		}
	}

	total := len(responses)
	var agreedLines []string
	var disputedLines []string
	agreedCount := 0

	// Sort keys for deterministic output order
	keys := make([]int, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	for _, key := range keys {
		// Count how many providers have this key
		count := 0
		var texts []string
		for _, items := range parsed {
			if v, ok := items[key]; ok {
				count++
				texts = append(texts, v)
			}
		}

		ratio := float64(count) / float64(total)
		if ratio >= threshold && count > 0 {
			// Use the first occurrence as canonical text
			agreedLines = append(agreedLines, fmt.Sprintf("✓ %d. %s", key, texts[0]))
			agreedCount++
		} else {
			disputedLines = append(disputedLines, fmt.Sprintf("△ %d. %s [%d/%d]", key, texts[0], count, total))
		}
	}

	var sb strings.Builder
	if len(agreedLines) > 0 {
		sb.WriteString("## 합의된 내용\n")
		sb.WriteString(strings.Join(agreedLines, "\n"))
		sb.WriteString("\n")
	}
	if len(disputedLines) > 0 {
		sb.WriteString("\n## 이견이 있는 내용\n")
		sb.WriteString(strings.Join(disputedLines, "\n"))
	}

	allKeys := len(keySet)
	summary := fmt.Sprintf("합의율: %d/%d (%.0f%%)",
		agreedCount, allKeys, float64(agreedCount)/float64(max1(allKeys))*100)

	return sb.String(), summary
}
