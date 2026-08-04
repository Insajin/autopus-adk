package promptlayer

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var (
	ompContextMemoryWindowsAbsV1 = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
	ompContextMemorySecretV1     = regexp.MustCompile(`(?i)(sk-(proj-)?[A-Za-z0-9_-]{8,}|gh[pousr]_[A-Za-z0-9_]{20,}|github_pat_[A-Za-z0-9_]{20,}|Authorization[[:space:]]*:[[:space:]]*Bearer|API[_-]?KEY[[:space:]]*[:=]|PASSWORD[[:space:]]*[:=]|SECRET[[:space:]]*[:=])`)
)

func safeOMPContextMemoryMetadataV1(value string) bool {
	if value == "" || strings.TrimSpace(value) != value || len(value) > 256 || filepath.IsAbs(value) || ompContextMemoryWindowsAbsV1.MatchString(value) || strings.HasPrefix(value, `\\`) {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	return !hasOMPContextMemorySecretV1(value) && !hasOMPContextMemoryPromptInjectionV1(value)
}

func safeOMPContextMemoryRefV1(value string) bool {
	if !safeOMPContextMemoryMetadataV1(value) || filepath.IsAbs(value) {
		return false
	}
	clean := filepath.Clean(value)
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator)) && filepath.ToSlash(clean) == value
}

func validOMPContextMemoryHashV1(value string) bool {
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, char := range value[len("sha256:"):] {
		if !strings.ContainsRune("0123456789abcdef", char) {
			return false
		}
	}
	return true
}

func hasOMPContextMemorySecretV1(value string) bool {
	return ompContextMemorySecretV1.MatchString(value)
}

func hasOMPContextMemoryPromptInjectionV1(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{
		"ignore previous instructions", "disregard previous instructions", "reveal the system prompt",
		"developer message", "drop acceptance.md", "delete acceptance.md",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func hasOMPContextMemoryAbsolutePathV1(value string) bool {
	for _, token := range strings.FieldsFunc(value, func(char rune) bool {
		return unicode.IsSpace(char) || strings.ContainsRune(`"'()[]{}<>=,;`, char)
	}) {
		trimmed := strings.TrimRight(token, ".:!?")
		if filepath.IsAbs(trimmed) || ompContextMemoryWindowsAbsV1.MatchString(trimmed) || strings.HasPrefix(trimmed, `\\`) {
			return true
		}
	}
	return false
}

func safeOMPContextMemoryOutputIDV1(candidateID string, index int) string {
	if safeOMPContextMemoryMetadataV1(candidateID) {
		return candidateID
	}
	return "candidate-" + fixedOMPContextMemoryIndexV1(index)
}

func fixedOMPContextMemoryIndexV1(index int) string {
	const digits = "0123456789"
	buffer := []byte("000000")
	for position := len(buffer) - 1; position >= 0 && index > 0; position-- {
		buffer[position] = digits[index%10]
		index /= 10
	}
	return string(buffer)
}
