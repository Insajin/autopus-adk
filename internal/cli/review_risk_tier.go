package cli

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

type reviewRiskTier string

const (
	reviewRiskTierAuto     reviewRiskTier = "auto"
	reviewRiskTierLow      reviewRiskTier = "low"
	reviewRiskTierMedium   reviewRiskTier = "medium"
	reviewRiskTierHigh     reviewRiskTier = "high"
	reviewRiskTierCritical reviewRiskTier = "critical"
)

func resolveReviewRiskTier(value string, files []string) (reviewRiskTier, []string, error) {
	tier, err := parseReviewRiskTier(value)
	if err != nil {
		return "", nil, err
	}
	inputs := normalizeRiskTierFiles(files)
	if tier != reviewRiskTierAuto {
		return tier, inputs, nil
	}
	if len(inputs) == 0 {
		inputs = changedFilesForRiskTier()
	}
	return inferReviewRiskTier(inputs), inputs, nil
}

func parseReviewRiskTier(value string) (reviewRiskTier, error) {
	switch reviewRiskTier(strings.ToLower(strings.TrimSpace(value))) {
	case "", reviewRiskTierAuto:
		return reviewRiskTierAuto, nil
	case reviewRiskTierLow:
		return reviewRiskTierLow, nil
	case reviewRiskTierMedium:
		return reviewRiskTierMedium, nil
	case reviewRiskTierHigh:
		return reviewRiskTierHigh, nil
	case reviewRiskTierCritical:
		return reviewRiskTierCritical, nil
	default:
		return "", fmt.Errorf("invalid risk tier %q: expected auto, low, medium, high, or critical", value)
	}
}

func inferReviewRiskTier(files []string) reviewRiskTier {
	files = normalizeRiskTierFiles(files)
	if len(files) == 0 {
		return reviewRiskTierMedium
	}
	if allDocumentationFiles(files) {
		return reviewRiskTierLow
	}

	sourceCount := 0
	for _, file := range files {
		if isCriticalReviewPath(file) {
			return reviewRiskTierCritical
		}
		if isHighReviewPath(file) {
			return reviewRiskTierHigh
		}
		if isSourceReviewPath(file) {
			sourceCount++
		}
	}
	if len(files) >= 10 || sourceCount >= 5 {
		return reviewRiskTierHigh
	}
	if sourceCount > 0 {
		return reviewRiskTierMedium
	}
	return reviewRiskTierLow
}

func applyReviewRiskTierProviders(providers []orchestra.ProviderConfig, tier reviewRiskTier) ([]orchestra.ProviderConfig, bool) {
	if len(providers) <= 1 {
		return providers, tierRequiresMultipleProviders(tier) && len(providers) == 1
	}
	if tier == reviewRiskTierLow || tier == reviewRiskTierMedium {
		return providers[:1], false
	}
	return providers, false
}

func tierRequiresMultipleProviders(tier reviewRiskTier) bool {
	return tier == reviewRiskTierHigh || tier == reviewRiskTierCritical
}

func normalizeRiskTierFiles(files []string) []string {
	normalized := make([]string, 0, len(files))
	for _, file := range files {
		path := filepath.ToSlash(strings.TrimSpace(file))
		path = strings.TrimPrefix(path, "./")
		if path == "" || path == "." {
			continue
		}
		normalized = append(normalized, path)
	}
	return normalized
}

func allDocumentationFiles(files []string) bool {
	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file))
		switch ext {
		case ".md", ".mdx", ".txt", ".rst", ".adoc":
			continue
		default:
			return false
		}
	}
	return true
}

func isCriticalReviewPath(file string) bool {
	lower := strings.ToLower(file)
	criticalTokens := []string{
		"auth", "oauth", "jwt", "credential", "secret", "token",
		"billing", "payment", "stripe", "invoice", "finance",
		"security", "iam", "permission", "access", "policy",
		"deploy", "release", "production", "mutation",
		"migration", "migrations/", ".sql",
		"gdpr", "legal", "compliance", "encryption", "crypto",
	}
	for _, token := range criticalTokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func isHighReviewPath(file string) bool {
	lower := strings.ToLower(file)
	highPrefixes := []string{
		"internal/services/",
		"internal/handlers/",
		"internal/workers/",
		"internal/middleware/",
		"pkg/pipeline/",
		"pkg/orchestra/",
		"pkg/qa/",
		"pkg/worker/",
		"src-tauri/",
		".github/workflows/",
	}
	for _, prefix := range highPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func isSourceReviewPath(file string) bool {
	switch strings.ToLower(filepath.Ext(file)) {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".rs", ".py", ".java", ".kt", ".swift", ".c", ".cc", ".cpp", ".h", ".hpp", ".sql":
		return true
	default:
		return false
	}
}

func changedFilesForRiskTier() []string {
	out, err := exec.Command("git", "diff", "--name-only", "--diff-filter=ACMR").Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(string(out), "\n")
	return normalizeRiskTierFiles(lines)
}
