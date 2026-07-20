package cli

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
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

func applyReviewProviderPolicy(
	providers []orchestra.ProviderConfig,
	commandName string,
	tier reviewRiskTier,
	inputs []string,
	explicit bool,
	w io.Writer,
) ([]orchestra.ProviderConfig, bool) {
	if commandName != "review" || tier == "" {
		return providers, false
	}
	adjusted, degraded := providers, tierRequiresMultipleProviders(tier) && len(providers) < 2
	if !explicit {
		adjusted, degraded = applyReviewRiskTierProviders(providers, tier)
	}
	if len(adjusted) != len(providers) || degraded {
		fmt.Fprintf(w, "리스크 티어: %s", tier)
		if len(inputs) > 0 {
			fmt.Fprintf(w, " (signals: %s)", strings.Join(inputs, ", "))
		}
		if degraded {
			fmt.Fprintln(w, " — multi-provider 대상이지만 provider가 1개라 단일 provider로 폴백")
		} else {
			fmt.Fprintf(w, " — provider fan-out %d → %d\n", len(providers), len(adjusted))
		}
	}
	single := len(adjusted) == 1 && (degraded || tier == reviewRiskTierLow || tier == reviewRiskTierMedium)
	return adjusted, single
}

func reviewRiskMinimumProviders(commandName string, tier reviewRiskTier) int {
	if commandName == "review" && tierRequiresMultipleProviders(tier) {
		return 2
	}
	return 0
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

// @AX:NOTE: [AUTO] @AX:SPEC: SPEC-ADK-ULTRA-EFFICIENCY-001 — any sensitive-domain substring forces the critical/full-review path.
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
	base := strings.ToLower(filepath.Base(lower))
	if strings.HasSuffix(lower, ".proto") || strings.HasSuffix(lower, ".graphql") || strings.HasSuffix(lower, ".gql") ||
		strings.HasPrefix(base, "openapi.") || strings.HasPrefix(base, "swagger.") {
		return true
	}
	highPrefixes := []string{
		"api/",
		"proto/",
		"openapi/",
		"swagger/",
		"graphql/",
		"internal/api/",
		"pkg/api/",
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
	files, _ := discoverChangedFilesForRiskTier()
	return files
}

// discoverChangedFilesForRiskTier preserves discovery failures for callers
// that must fail closed. changedFilesForRiskTier retains the legacy best-effort
// behavior used by the review command.
func discoverChangedFilesForRiskTier() ([]string, error) {
	return discoverChangedFilesForRiskTierIn("")
}

func discoverChangedFilesForRiskTierIn(dir string) ([]string, error) {
	commands := [][]string{
		{"diff", "--name-only", "--diff-filter=ACMRD"},
		{"diff", "--cached", "--name-only", "--diff-filter=ACMRD"},
		{"ls-files", "--others", "--exclude-standard"},
	}
	seen := make(map[string]struct{})
	for _, args := range commands {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			return nil, err
		}
		for _, file := range normalizeRiskTierFiles(strings.Split(string(out), "\n")) {
			seen[file] = struct{}{}
		}
	}
	files := make([]string, 0, len(seen))
	for file := range seen {
		files = append(files, file)
	}
	sort.Strings(files)
	return files, nil
}
