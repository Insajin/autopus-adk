package issue

import (
	"os"
	"regexp"
	"strings"
)

var (
	// reToken matches common API token patterns.
	reToken = regexp.MustCompile(`(?i)(ghp_|gho_|github_pat_|sk-|AKIA)[a-zA-Z0-9_-]+`)

	// reEnvSecret matches environment variable assignments containing sensitive names.
	// Capture groups: 1=name, 2=separator+whitespace, 3=value.
	reEnvSecret = regexp.MustCompile(`(?i)([A-Z_]*(TOKEN|KEY|SECRET|PASSWORD|CREDENTIAL)[A-Z_]*)(\s*[=:]\s*)\S+`)

	// reURLCred matches HTTP/HTTPS URLs with embedded credentials.
	reURLCred = regexp.MustCompile(`(https?://)([^@\s]+@)`)
)

// SanitizePath replaces the user's home directory prefix with $HOME.
func SanitizePath(s string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return s
	}
	return strings.ReplaceAll(s, home, "$HOME")
}

// SanitizeSecrets redacts API keys, tokens, and secret environment variables.
func SanitizeSecrets(s string) string {
	// Apply env var pattern first: replace value but keep name and separator.
	// Groups: $1=name, $3=separator+whitespace, value is replaced.
	s = reEnvSecret.ReplaceAllString(s, "${1}${3}[REDACTED]")
	// Apply token pattern for standalone tokens not caught by env var pattern.
	s = reToken.ReplaceAllString(s, "[REDACTED]")
	return s
}

// SanitizeGitURL strips embedded credentials from HTTP/HTTPS git URLs.
func SanitizeGitURL(s string) string {
	return reURLCred.ReplaceAllString(s, "$1")
}

// Sanitize applies SanitizePath, SanitizeSecrets, and SanitizeGitURL in sequence.
func Sanitize(s string) string {
	s = SanitizePath(s)
	s = SanitizeSecrets(s)
	s = SanitizeGitURL(s)
	return s
}
