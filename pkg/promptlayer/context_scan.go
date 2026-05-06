package promptlayer

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-AGENT-PROMPT-001: 32 KiB cap bounds optional context before prompt-layer hashing.
const defaultContextMaxBytes = 32 * 1024

type ContextOptions struct {
	MaxBytes int
}

type SanitizedContent struct {
	Content            string
	RedactionStatus    string
	InvalidationReason string
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-AGENT-PROMPT-001: security-sensitive marker lists define context redaction and injection detection coverage.
var contextSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?is)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`),
	regexp.MustCompile(`(?i)\b[A-Z0-9_]*(API[_-]?KEY|ACCESS[_-]?TOKEN|AUTH[_-]?TOKEN|TOKEN|SECRET|PASSWORD|PASSWD|PRIVATE[_-]?KEY|CLIENT[_-]?SECRET)[A-Z0-9_]*\b\s*[:=]\s*["']?[^"'\s]{8,}["']?`),
	regexp.MustCompile(`(?i)\bAuthorization\s*:\s*Bearer\s+[A-Za-z0-9._~+/=-]{8,}`),
	regexp.MustCompile(`\b(sk-(proj-)?[A-Za-z0-9_-]{8,}|gh[pousr]_[A-Za-z0-9_]{20,}|github_pat_[A-Za-z0-9_]{20,}|AKIA[0-9A-Z]{16}|ASIA[0-9A-Z]{16})\b`),
}

var injectionMarkers = []string{
	"ignore previous instructions",
	"disregard previous instructions",
	"reveal the system prompt",
	"developer message",
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-AGENT-PROMPT-001: LoadContextLayer is the root-relative context ingestion and redaction boundary.
func LoadContextLayer(root, relPath string, opts ContextOptions) (Layer, error) {
	if filepath.IsAbs(relPath) {
		return Layer{}, fmt.Errorf("context path must be relative: %s", relPath)
	}
	clean := filepath.Clean(relPath)
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return Layer{}, fmt.Errorf("context path escapes root: %s", relPath)
	}
	path, err := resolveContextPath(root, clean)
	if err != nil {
		return Layer{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Layer{
			ID:                 "context:" + filepath.ToSlash(clean),
			Kind:               KindStable,
			Group:              GroupProjectContext,
			SourceRef:          filepath.ToSlash(clean),
			RedactionStatus:    RedactionSkipped,
			InvalidationReason: InvalidationMissingOptionalContext,
		}, nil
	}
	if err != nil {
		return Layer{}, err
	}

	sanitized := SanitizeContent(string(data), opts)

	return Layer{
		ID:                 "context:" + filepath.ToSlash(clean),
		Kind:               KindStable,
		Group:              GroupProjectContext,
		SourceRef:          filepath.ToSlash(clean),
		Content:            sanitized.Content,
		CacheEligible:      sanitized.RedactionStatus == RedactionPassed,
		RedactionStatus:    sanitized.RedactionStatus,
		InvalidationReason: sanitized.InvalidationReason,
	}, nil
}

func SanitizeContent(raw string, opts ContextOptions) SanitizedContent {
	content, reasons, redacted := sanitizeContextContent(raw, maxContextBytes(opts))
	status := RedactionPassed
	if redacted || len(reasons) > 0 {
		status = RedactionRedacted
	}
	return SanitizedContent{
		Content:            content,
		RedactionStatus:    status,
		InvalidationReason: joinReasons(reasons),
	}
}

func sanitizeContextContent(raw string, maxBytes int) (string, []string, bool) {
	var out []string
	reasons := map[string]bool{}
	redacted := false

	cleanedRaw := raw
	for _, pattern := range contextSecretPatterns {
		if pattern.MatchString(cleanedRaw) {
			reasons[InvalidationSecretRisk] = true
			redacted = true
			cleanedRaw = pattern.ReplaceAllString(cleanedRaw, "[REDACTED_SECRET]")
		}
	}

	for _, line := range strings.Split(cleanedRaw, "\n") {
		if hasInjectionMarker(line) {
			reasons[InvalidationInjectionRisk] = true
			redacted = true
			continue
		}
		out = append(out, line)
	}

	content := strings.TrimSpace(strings.Join(out, "\n"))
	if len(raw) > maxBytes || len(content) > maxBytes {
		reasons[InvalidationSizeCap] = true
		redacted = true
		if len(content) > maxBytes {
			content = strings.TrimSpace(truncateUTF8(content, maxBytes))
		}
	}

	return content, sortedReasonList(reasons), redacted
}

func resolveContextPath(root, clean string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	evaluatedRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", fmt.Errorf("context root unavailable: %w", err)
	}
	target := filepath.Join(evaluatedRoot, clean)
	evaluatedTarget, err := filepath.EvalSymlinks(target)
	if err == nil {
		if !pathWithinRoot(evaluatedRoot, evaluatedTarget) {
			return "", fmt.Errorf("context path escapes root: %s", clean)
		}
		return evaluatedTarget, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	if info, lstatErr := os.Lstat(target); lstatErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("context path symlink is unavailable: %s", clean)
	}
	parent := filepath.Dir(target)
	evaluatedParent, parentErr := filepath.EvalSymlinks(parent)
	if parentErr != nil {
		return "", parentErr
	}
	if !pathWithinRoot(evaluatedRoot, evaluatedParent) {
		return "", fmt.Errorf("context path escapes root: %s", clean)
	}
	return target, nil
}

func pathWithinRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func hasInjectionMarker(line string) bool {
	lower := strings.ToLower(line)
	for _, marker := range injectionMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func maxContextBytes(opts ContextOptions) int {
	if opts.MaxBytes > 0 {
		return opts.MaxBytes
	}
	return defaultContextMaxBytes
}

func sortedReasonList(reasons map[string]bool) []string {
	ordered := []string{InvalidationInjectionRisk, InvalidationSecretRisk, InvalidationSizeCap, InvalidationMissingOptionalContext}
	out := make([]string, 0, len(reasons))
	for _, reason := range ordered {
		if reasons[reason] {
			out = append(out, reason)
		}
	}
	if len(out) != len(reasons) {
		extras := make([]string, 0, len(reasons)-len(out))
		for reason := range reasons {
			if !containsString(out, reason) {
				extras = append(extras, reason)
			}
		}
		sort.Strings(extras)
		out = append(out, extras...)
	}
	return out
}

func joinReasons(reasons []string) string {
	if len(reasons) == 0 {
		return InvalidationNone
	}
	return strings.Join(reasons, ",")
}

func truncateUTF8(content string, maxBytes int) string {
	if maxBytes <= 0 || len(content) <= maxBytes {
		return content
	}
	for maxBytes > 0 && !utf8.ValidString(content[:maxBytes]) {
		maxBytes--
	}
	return content[:maxBytes]
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
