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

	// Parse every response before merging. A single unstructured response keeps
	// the legacy line-based fallback available to callers.
	parsed := make([]map[int]string, len(responses))
	for i, r := range responses {
		items, err := parseStructuredResponse(r.Output)
		if err != nil {
			return "", ""
		}
		parsed[i] = items
	}

	total := len(responses)
	type claim struct {
		index     int
		text      string
		providers []string
	}
	claims := make(map[string]*claim)
	var claimOrder []string
	for providerIndex, items := range parsed {
		keys := make([]int, 0, len(items))
		for key := range items {
			keys = append(keys, key)
		}
		sort.Ints(keys)
		seenByProvider := make(map[string]bool, len(keys))
		for _, key := range keys {
			text := strings.TrimSpace(items[key])
			identity := normalizeLine(text)
			if identity == "" || seenByProvider[identity] {
				continue
			}
			seenByProvider[identity] = true
			entry, exists := claims[identity]
			if !exists {
				entry = &claim{index: key, text: text}
				claims[identity] = entry
				claimOrder = append(claimOrder, identity)
			}
			entry.providers = append(entry.providers, responses[providerIndex].Provider)
		}
	}

	var agreedLines []string
	var disputedLines []string
	agreedCount := 0
	for _, identity := range claimOrder {
		entry := claims[identity]
		count := len(entry.providers)
		ratio := float64(count) / float64(total)
		line := fmt.Sprintf("%d. %s [%d/%d]", entry.index, entry.text, count, total)
		if ratio >= threshold {
			agreedLines = append(agreedLines, "✓ "+line)
			agreedCount++
		} else {
			disputedLines = append(disputedLines, "△ "+line)
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

	totalClaims := len(claimOrder)
	summary := fmt.Sprintf("합의율: %d/%d (%.0f%%)",
		agreedCount, totalClaims, float64(agreedCount)/float64(max1(totalClaims))*100)

	return sb.String(), summary
}
