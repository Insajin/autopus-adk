package setup

import (
	"fmt"
	"strings"
)

func writeTreeEntry(b *strings.Builder, entry DirEntry, depth int) {
	prefix := strings.Repeat("    ", depth)
	suffix := "/"
	if entry.Description != "" {
		suffix += "  # " + entry.Description
	}
	b.WriteString(fmt.Sprintf("%s%s%s\n", prefix, entry.Name, suffix))
}

func writeDetailedTreeEntry(b *strings.Builder, entry DirEntry, depth int) {
	prefix := strings.Repeat("    ", depth)
	b.WriteString(fmt.Sprintf("%s%s/\n", prefix, entry.Name))
	for _, child := range entry.Children {
		writeDetailedTreeEntry(b, child, depth+1)
	}
}

func truncateLines(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines-2], "\n") + "\n\n<!-- Truncated: exceeded line limit -->\n"
}
