package spec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// findingsSidecar is the object shape of review-findings.json introduced by
// SPEC-ADK-REVIEW-INTEGRITY-001. Prior artifacts are a top-level array; loaders
// sniff the first non-space byte to accept both shapes (REQ-RINT-COMPAT-10).
// @AX:ANCHOR: [AUTO] on-disk backward-compat contract — every review-findings.json written before this SPEC is a bare array; changing the '{' sniff in LoadFindingsWithCoverage or this struct's JSON tags breaks parsing of all pre-existing sidecars
// @AX:REASON: REQ-RINT-COMPAT-10 requires prior-schema files to keep loading without error; this is the sole shape-detection point
type findingsSidecar struct {
	Findings     []ReviewFinding `json:"findings"`
	DocCoverages []DocCoverage   `json:"doc_coverages"`
}

// PersistFindings writes the current findings state to review-findings.json.
// A nil findings slice is normalized to an empty slice so the sidecar always
// holds a valid JSON array `[]` instead of the literal `null` (issue #58):
// `json.MarshalIndent` emits `null` for a nil slice, which makes the file look
// corrupt to downstream tooling even when review.md still carries the findings.
func PersistFindings(dir string, findings []ReviewFinding) error {
	if findings == nil {
		findings = []ReviewFinding{}
	}
	data, err := json.MarshalIndent(findings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal findings: %w", err)
	}
	path := filepath.Join(dir, "review-findings.json")
	return os.WriteFile(path, data, 0o644)
}

// PersistFindingsWithCoverage writes findings and per-document observation
// coverage as the object-shaped review-findings.json (SPEC-ADK-REVIEW-INTEGRITY-001
// REQ-RINT-COV-01). Nil slices normalize to empty arrays so the sidecar never
// serializes the literal null (issue #58). LoadFindings reads this shape and the
// legacy array shape interchangeably.
func PersistFindingsWithCoverage(dir string, findings []ReviewFinding, coverages []DocCoverage) error {
	if findings == nil {
		findings = []ReviewFinding{}
	}
	if coverages == nil {
		coverages = []DocCoverage{}
	}
	data, err := json.MarshalIndent(findingsSidecar{Findings: findings, DocCoverages: coverages}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal findings sidecar: %w", err)
	}
	path := filepath.Join(dir, "review-findings.json")
	return os.WriteFile(path, data, 0o644)
}

// NormalizeAdvisoryFindings keeps suggestion-level feedback visible without
// letting it block review-gate convergence.
func NormalizeAdvisoryFindings(findings []ReviewFinding) []ReviewFinding {
	if len(findings) == 0 {
		return nil
	}

	updated := make([]ReviewFinding, len(findings))
	for i, f := range findings {
		updated[i] = f
		if IsAdvisoryFinding(f) && isOpenOrRegressed(f.Status) {
			updated[i].Status = FindingStatusDeferred
		}
	}
	return updated
}

// IsAdvisoryFinding reports whether a finding is non-blocking review feedback.
func IsAdvisoryFinding(f ReviewFinding) bool {
	return strings.EqualFold(strings.TrimSpace(f.Severity), "suggestion") &&
		f.Category != FindingCategorySecurity &&
		!f.EscapeHatch
}

// IsActiveBlockingFinding reports whether a finding should block PASS.
func IsActiveBlockingFinding(f ReviewFinding) bool {
	if !isOpenOrRegressed(f.Status) {
		return false
	}
	return !IsAdvisoryFinding(f)
}

func isOpenOrRegressed(status FindingStatus) bool {
	return status == FindingStatusOpen || status == FindingStatusRegressed
}

// LoadFindings reads prior findings from review-findings.json.
// Returns empty slice (not error) if file doesn't exist.
// Returns error if file exists but is corrupted.
// Accepts both the legacy top-level array and the object shape written by
// PersistFindingsWithCoverage (REQ-RINT-COMPAT-10).
func LoadFindings(dir string) ([]ReviewFinding, error) {
	findings, _, err := LoadFindingsWithCoverage(dir)
	return findings, err
}

// LoadFindingsWithCoverage reads findings and any recorded observation coverage
// from review-findings.json. Legacy array-shaped sidecars load with an empty
// coverage set, treating the coverage fields as additive and optional
// (REQ-RINT-COMPAT-10). A missing file is not an error.
func LoadFindingsWithCoverage(dir string) ([]ReviewFinding, []DocCoverage, error) {
	path := filepath.Join(dir, "review-findings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []ReviewFinding{}, nil, nil
		}
		return nil, nil, fmt.Errorf("read findings file: %w", err)
	}

	// Sniff the first non-space byte: '{' is the object shape, otherwise the
	// legacy top-level array.
	if trimmed := bytes.TrimLeft(data, " \t\r\n"); len(trimmed) > 0 && trimmed[0] == '{' {
		var sidecar findingsSidecar
		if err := json.Unmarshal(data, &sidecar); err != nil {
			return nil, nil, fmt.Errorf("unmarshal findings sidecar: %w", err)
		}
		return sidecar.Findings, sidecar.DocCoverages, nil
	}

	var findings []ReviewFinding
	if err := json.Unmarshal(data, &findings); err != nil {
		return nil, nil, fmt.Errorf("unmarshal findings: %w", err)
	}
	return findings, nil, nil
}

// DeduplicateFindings removes duplicate findings based on normalized ScopeRef + Category + Description.
// Assigns sequential IDs (F-001, F-002, ...) to the deduplicated set.
func DeduplicateFindings(findings []ReviewFinding) []ReviewFinding {
	if len(findings) == 0 {
		return nil
	}

	type key struct {
		scopeRef    string
		category    FindingCategory
		description string
	}

	seen := make(map[key]bool)
	result := make([]ReviewFinding, 0, len(findings))

	for _, f := range findings {
		k := key{
			scopeRef:    NormalizeScopeRef(f.ScopeRef, ""),
			category:    f.Category,
			description: f.Description,
		}
		if seen[k] {
			continue
		}
		seen[k] = true
		result = append(result, f)
	}

	// Assign sequential IDs to deduplicated findings.
	for i := range result {
		result[i].ID = fmt.Sprintf("F-%03d", i+1)
	}

	return result
}

// ApplyScopeLock filters findings based on mode and prior scope.
// In verify mode: new non-critical, non-security findings are tagged out_of_scope.
// Critical/security findings get EscapeHatch=true.
// In discover mode: all findings pass through unchanged.
func ApplyScopeLock(incoming, prior []ReviewFinding, mode ReviewMode) []ReviewFinding {
	if mode != ReviewModeVerify {
		return incoming
	}

	// Build set of known IDs from prior findings.
	knownIDs := make(map[string]bool, len(prior))
	for _, f := range prior {
		if f.ID != "" {
			knownIDs[f.ID] = true
		}
	}

	result := make([]ReviewFinding, 0, len(incoming))
	for _, f := range incoming {
		if knownIDs[f.ID] {
			result = append(result, f)
			continue
		}
		// New finding in verify mode: apply scope lock
		if f.Severity == "critical" || f.Category == FindingCategorySecurity {
			f.EscapeHatch = true
			f.Status = FindingStatusOpen
		} else {
			f.Status = FindingStatusOutOfScope
		}
		result = append(result, f)
	}
	return result
}

// MergeSupermajority merges findings from multiple providers using a supermajority threshold.
// totalProviders: total number of providers that participated.
// threshold: fraction required for consensus (e.g., 0.67 for 2/3).
// Critical/security findings bypass the threshold and are always kept.
func MergeSupermajority(findings []ReviewFinding, totalProviders int, threshold float64) []ReviewFinding {
	if len(findings) == 0 {
		return nil
	}

	// Group findings by normalized key: category + scopeRef + description-prefix.
	type groupKey struct {
		category          FindingCategory
		scopeRef          string
		descriptionPrefix string
	}

	groups := make(map[groupKey][]ReviewFinding)
	var keyOrder []groupKey

	for _, f := range findings {
		k := groupKey{
			category:          f.Category,
			scopeRef:          NormalizeScopeRef(f.ScopeRef, ""),
			descriptionPrefix: normalizedDescriptionPrefix(f.Description),
		}
		if _, exists := groups[k]; !exists {
			keyOrder = append(keyOrder, k)
		}
		groups[k] = append(groups[k], f)
	}

	var merged []ReviewFinding
	for _, k := range keyOrder {
		group := groups[k]
		count := len(group)

		// Critical/security findings bypass supermajority.
		if k.category == FindingCategorySecurity {
			merged = append(merged, group[0])
			continue
		}

		// Use a small tolerance so that e.g. 2/3=0.6667 qualifies for threshold=0.67.
		if float64(count)/float64(totalProviders)+0.005 >= threshold {
			merged = append(merged, group[0])
		}
	}

	return merged
}

func normalizedDescriptionPrefix(description string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(description), " "))
	runes := []rune(normalized)
	if len(runes) > 120 {
		return string(runes[:120])
	}
	return normalized
}

// NormalizeScopeRef normalizes a scope reference for comparison (REQ-012):
// 1. Strip leading "./"
// 2. Strip basePath prefix for absolute paths (if provided)
// 3. Lowercase for file paths
// 4. Requirement refs (REQ-xxx) are kept as-is
// 5. Line number info is preserved
func NormalizeScopeRef(ref, basePath string) string {
	if ref == "" {
		return ref
	}

	ref = strings.TrimPrefix(ref, "./")

	// Requirement references are exact match — no normalization.
	if strings.HasPrefix(strings.ToUpper(ref), "REQ-") {
		return strings.ToUpper(ref)
	}

	// Strip absolute basePath prefix to get relative path.
	if basePath != "" {
		prefix := strings.TrimSuffix(basePath, "/") + "/"
		ref = strings.TrimPrefix(ref, prefix)
	}

	return strings.ToLower(ref)
}
