package content

import (
	"path"
	"strings"
)

// EmbeddedPath builds slash-separated paths for embed.FS and normalizes
// Windows-style separators so the same lookup works cross-platform.
func EmbeddedPath(parts ...string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized = append(normalized, strings.ReplaceAll(part, `\`, "/"))
	}
	return path.Clean(strings.Join(normalized, "/"))
}
