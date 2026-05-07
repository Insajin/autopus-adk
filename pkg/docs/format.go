package docs

import (
	"errors"
	"fmt"
	"strings"
)

// sourceLabel maps internal source identifiers to display labels.
func sourceLabel(source string) string {
	switch source {
	case "context7":
		return "Context7"
	case "scraper":
		return "WebSearch"
	case "cache":
		return "Cache"
	default:
		if len(source) == 0 {
			return source
		}
		return strings.ToUpper(source[:1]) + source[1:]
	}
}

// FormatPromptInjection formats documentation results as a prompt injection section.
// Returns an error if results is nil or empty.
func FormatPromptInjection(results []*DocResult) (string, error) {
	if len(results) == 0 {
		return "", errors.New("no documentation results to format")
	}

	var sb strings.Builder
	sb.WriteString("## Reference Documentation\n\n")
	sb.WriteString("The following documentation was fetched from documentation sources for libraries used in this task.\n")

	for _, r := range results {
		sb.WriteString(fmt.Sprintf("\n### %s (via %s)\n", r.LibraryName, sourceLabel(r.Source)))
		meta := formatMetadata(r)
		if meta != "" {
			sb.WriteString(meta)
			sb.WriteString("\n\n")
		}
		sb.WriteString(r.Content)
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func formatMetadata(r *DocResult) string {
	var parts []string
	if r.Version != "" {
		parts = append(parts, "version="+r.Version)
	}
	if r.SourceRef != "" {
		parts = append(parts, "source_ref="+r.SourceRef)
	}
	if !r.CheckedAt.IsZero() {
		parts = append(parts, "checked_at="+r.CheckedAt.UTC().Format("2006-01-02"))
	}
	if len(parts) == 0 {
		return ""
	}
	return "_Metadata: " + strings.Join(parts, " | ") + "_"
}
