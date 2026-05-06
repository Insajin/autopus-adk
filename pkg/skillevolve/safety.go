package skillevolve

import (
	"context"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-SKILL-EVOLVE-001: 64KiB bounds retained candidate content before replay or promotion.
const defaultMaxCandidateBytes = 64 * 1024

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-SKILL-EVOLVE-001: hardcoded secret signatures define local redaction policy for retained candidate metadata.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`sk-(?:proj-)?[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`(?i)(api[_-]?key|token|secret|password)\s*[:=]\s*[A-Za-z0-9_\-./+=]{16,}`),
	regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{20,}`),
}

// @AX:WARN [AUTO] @AX:SPEC: SPEC-SKILL-EVOLVE-001: static admission gate combines path, ownership, frontmatter, prompt-injection, secret, and size checks.
// @AX:REASON: The function has 8+ safety branches and promotion/replay safety depends on preserving all reason-code paths.
func EvaluateSafety(ctx context.Context, candidate CandidateBundle, opts SafetyOptions) (SafetyResult, error) {
	if err := ctx.Err(); err != nil {
		return SafetyResult{}, err
	}
	maxBytes := opts.MaxCandidateBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxCandidateBytes
	}

	ownedPaths := append([]string{}, candidate.OwnedPaths...)
	ownedPaths = append(ownedPaths, opts.OwnedPaths...)
	reasons := make([]string, 0)
	totalBytes := 0
	secretSeen := false

	for _, file := range candidate.ProposedFiles {
		if isGeneratedSurfacePath(file.Path) {
			reasons = appendReason(reasons, "generated_surface_mutation_forbidden")
		}
		if len(ownedPaths) > 0 && !pathWithinOwnedPaths(file.Path, ownedPaths) {
			reasons = appendReason(reasons, "affected_file_outside_owned_paths")
		}
		totalBytes += len([]byte(file.Content))
		if requiresSkillFrontmatter(file.Path) && !hasValidSkillFrontmatter(file.Content) {
			reasons = appendReason(reasons, "invalid_frontmatter")
		}
		if containsForbiddenInstruction(file.Content) {
			reasons = appendReason(reasons, "forbidden_instruction")
		}
		if containsSecret(file.Content) {
			secretSeen = true
			reasons = appendReason(reasons, "secret_risk")
		}
	}
	if totalBytes > maxBytes {
		reasons = appendReason(reasons, "candidate_too_large")
	}

	allowed := len(reasons) == 0
	retained := retainedSafetyMetadata(candidate, reasons)
	if secretSeen {
		retained["redaction_sample"] = "[REDACTED_SECRET]"
	}
	return SafetyResult{
		Allowed:          allowed,
		ReplayAllowed:    allowed,
		PromotionAllowed: allowed,
		ReasonCodes:      reasons,
		RetainedMetadata: retained,
	}, nil
}

func requiresSkillFrontmatter(rel string) bool {
	rel = cleanRelPath(rel)
	return strings.HasSuffix(rel, ".md") &&
		(strings.Contains(rel, "/content/skills/") || strings.HasSuffix(rel, "/SKILL.md"))
}

func hasValidSkillFrontmatter(content string) bool {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return false
	}
	rest := strings.TrimPrefix(normalized, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return false
	}
	var frontmatter map[string]any
	if err := yaml.Unmarshal([]byte(rest[:end]), &frontmatter); err != nil {
		return false
	}
	return nonEmptyString(frontmatter["name"]) && nonEmptyString(frontmatter["description"])
}

func nonEmptyString(value any) bool {
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) != ""
}

func containsForbiddenInstruction(content string) bool {
	lower := strings.ToLower(content)
	phrases := []string{
		"ignore previous instructions",
		"bypass review",
		"disable safety",
		"override system instructions",
		"exfiltrate",
	}
	for _, phrase := range phrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

func containsSecret(content string) bool {
	for _, pattern := range secretPatterns {
		if pattern.MatchString(content) {
			return true
		}
	}
	return false
}

func redactSecrets(content string) string {
	redacted := content
	for _, pattern := range secretPatterns {
		redacted = pattern.ReplaceAllString(redacted, "[REDACTED_SECRET]")
	}
	return redacted
}

func retainedSafetyMetadata(candidate CandidateBundle, reasons []string) map[string]any {
	files := make([]map[string]string, 0, len(candidate.ProposedFiles))
	for _, file := range candidate.ProposedFiles {
		snippet := redactSecrets(file.Content)
		if len(snippet) > 256 {
			snippet = snippet[:256]
		}
		files = append(files, map[string]string{
			"path":    cleanRelPath(file.Path),
			"snippet": snippet,
		})
	}
	return map[string]any{
		"candidate_id": candidate.ID,
		"reason_codes": reasons,
		"files":        files,
	}
}

func appendReason(reasons []string, reason string) []string {
	for _, existing := range reasons {
		if existing == reason {
			return reasons
		}
	}
	return append(reasons, reason)
}
