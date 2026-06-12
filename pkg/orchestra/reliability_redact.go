package orchestra

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// reliability_redact.go holds the secret-redaction and preview helpers used when
// building sanitized reliability artifacts. Split out of reliability_bundle.go to
// keep both files within the source-file line limit.

// @AX:NOTE [AUTO]: hardcoded sensitive-pattern list — extend here when new secret formats (e.g. new token prefixes) are identified; order is irrelevant (all patterns applied)
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(authorization\s*:\s*bearer\s+)[^\s]+`),
	regexp.MustCompile(`(?i)([A-Z0-9_]*API[_-]?KEY\s*=\s*)[^\s]+`),
	regexp.MustCompile(`(?i)(authorization|token|secret|password|cookie)\s*[:=]\s*[^\s]+`),
	regexp.MustCompile(`(?i)bearer\s+[a-z0-9._-]+`),
	regexp.MustCompile(`sk-[A-Za-z0-9_-]+`),
	regexp.MustCompile(`eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`),
}

func sanitizeArtifact(text string) SanitizedArtifact {
	redacted := redactSensitiveText(text)
	return SanitizedArtifact{
		ByteLength: len(text),
		Hash:       hashString(text),
		Preview:    safePreview(redacted, 120),
	}
}

func redactSensitiveText(text string) string {
	redacted := text
	for _, pattern := range sensitivePatterns {
		redacted = pattern.ReplaceAllStringFunc(redacted, func(s string) string {
			if groups := pattern.FindStringSubmatch(s); len(groups) == 2 {
				return groups[1] + "***"
			}
			parts := strings.SplitN(s, "=", 2)
			if len(parts) == 2 {
				return parts[0] + "=***"
			}
			parts = strings.SplitN(s, ":", 2)
			if len(parts) == 2 {
				return parts[0] + ": ***"
			}
			if strings.HasPrefix(strings.ToLower(s), "bearer ") {
				return "Bearer ***"
			}
			return "***"
		})
	}
	return redacted
}

func safePreview(text string, max int) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if len(normalized) <= max {
		return normalized
	}
	return normalized[:max] + "..."
}

func hashString(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
