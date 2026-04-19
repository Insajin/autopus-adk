package orchestra

import (
	"regexp"
	"strings"
)

// validProviderName matches safe provider names (alphanumeric, hyphens, underscores).
var validProviderName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// sanitizeProviderName returns a safe provider name for use in file paths.
// Rejects names containing path separators or special characters.
func sanitizeProviderName(name string) string {
	if validProviderName.MatchString(name) {
		return name
	}
	// Strip everything except safe chars
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			sb.WriteRune(r)
		}
	}
	if sb.Len() == 0 {
		return "unknown"
	}
	return sb.String()
}

// shellEscapeArg wraps a string in single quotes for safe shell interpolation.
// Any embedded single quotes use the standard POSIX quote-break pattern:
// close quote, emit an escaped quote, then reopen the quote.
func shellEscapeArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// shellEscapeArgs applies shellEscapeArg to each element and joins with spaces.
func shellEscapeArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	escaped := make([]string, len(args))
	for i, a := range args {
		escaped[i] = shellEscapeArg(a)
	}
	return strings.Join(escaped, " ")
}

// uniqueHeredocDelimiter returns a heredoc delimiter that does not appear in content.
// Falls back to appending a random suffix if the base delimiter is found in content.
func uniqueHeredocDelimiter(base, content, randomSuffix string) string {
	delim := base
	if strings.Contains(content, delim) {
		delim = base + "_" + randomSuffix
	}
	return delim
}
