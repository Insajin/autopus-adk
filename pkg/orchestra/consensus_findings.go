package orchestra

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ConsensusMetrics reports typed claim counts independently from the rendered
// markdown summary.
type ConsensusMetrics struct {
	TotalClaims   int                     `json:"total_claims"`
	AgreedClaims  int                     `json:"agreed_claims"`
	DissentClaims int                     `json:"dissent_claims"`
	CriticalVeto  bool                    `json:"critical_veto"`
	FindingClaims []FindingConsensusClaim `json:"finding_claims,omitempty"`
}

// FindingClaimEvidence preserves one provider's typed view of a clustered finding.
type FindingClaimEvidence struct {
	Provider    string `json:"provider"`
	Severity    string `json:"severity"`
	Category    string `json:"category,omitempty"`
	ScopeRef    string `json:"scope_ref,omitempty"`
	Location    string `json:"location,omitempty"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// FindingConsensusClaim is the typed, order-independent projection of one finding cluster.
type FindingConsensusClaim struct {
	Identity     string                 `json:"identity"`
	Finding      Finding                `json:"finding"`
	Evidence     []FindingClaimEvidence `json:"evidence"`
	CriticalVeto bool                   `json:"critical_veto"`
}

type findingClaim struct {
	identity             string
	finding              Finding
	representativeStatus string
	providers            []string
	evidence             []FindingClaimEvidence
}

func mergeReviewerFindingConsensus(responses []ProviderResponse, threshold float64) (string, string, bool) {
	claims, ok := collectReviewerFindingClaims(responses)
	if !ok {
		return "", "", false
	}

	identities := make([]string, 0, len(claims))
	for identity := range claims {
		identities = append(identities, identity)
	}
	sort.Strings(identities)
	totalProviders := len(responses)
	var agreed, dissent []string
	veto := false
	for _, identity := range identities {
		claim := claims[identity]
		count := len(claim.providers)
		line := formatFindingClaim(*claim, count, totalProviders)
		if float64(count)/float64(totalProviders) >= threshold {
			agreed = append(agreed, "✓ "+line)
		} else {
			dissent = append(dissent, "△ "+line)
		}
		if findingClaimHasCriticalVeto(*claim) {
			veto = true
		}
	}

	var body strings.Builder
	if len(agreed) > 0 {
		body.WriteString("## 합의된 내용\n")
		body.WriteString(strings.Join(agreed, "\n"))
		body.WriteString("\n")
	}
	if len(dissent) > 0 {
		body.WriteString("\n## 이견이 있는 내용\n")
		body.WriteString(strings.Join(dissent, "\n"))
		body.WriteString("\n")
	}
	if veto {
		body.WriteString("\n## Critical veto\nUnresolved critical security/correctness finding blocks the gate.\n")
	}
	summary := fmt.Sprintf("합의율: %d/%d (%.0f%%)", len(agreed), len(claims), float64(len(agreed))/float64(max1(len(claims)))*100)
	if veto {
		summary += "; veto=critical"
	}
	return body.String(), summary, true
}

func collectReviewerFindingClaims(responses []ProviderResponse) (map[string]*findingClaim, bool) {
	claims := make(map[string]*findingClaim)
	for _, response := range responses {
		var review ReviewerOutput
		if err := json.Unmarshal([]byte(response.Output), &review); err != nil {
			return nil, false
		}
		seen := make(map[string]bool)
		statuses := findingStatusMap(review.FindingStatus)
		for _, finding := range review.Findings {
			identity := stableFindingIdentity(finding)
			if identity == "" || seen[identity] {
				continue
			}
			seen[identity] = true
			status := findingEvidenceStatus(finding, identity, statuses)
			entry := claims[identity]
			if entry == nil {
				entry = &findingClaim{identity: identity, finding: finding, representativeStatus: status}
				claims[identity] = entry
			}
			entry.providers = append(entry.providers, response.Provider)
			entry.evidence = append(entry.evidence, FindingClaimEvidence{
				Provider: response.Provider, Severity: finding.Severity,
				Category: finding.Category, ScopeRef: finding.ScopeRef, Location: finding.Location,
				Description: finding.Description, Suggestion: finding.Suggestion, Status: status,
			})
			entry.finding, entry.representativeStatus = preferredFinding(
				entry.finding, entry.representativeStatus, finding, status,
			)
		}
	}
	return claims, len(claims) > 0
}

func deriveConsensusMetrics(responses []ProviderResponse, threshold float64) *ConsensusMetrics {
	if len(responses) == 0 {
		return nil
	}
	if threshold <= 0 {
		threshold = 0.66
	}
	criticalVeto := structuredCriticalVeto(responses)
	if claims, ok := collectReviewerFindingClaims(responses); ok {
		metrics := metricsFromFindingClaims(claims, len(responses), threshold)
		metrics.CriticalVeto = metrics.CriticalVeto || criticalVeto
		return &metrics
	}
	claims := collectTextConsensusClaims(responses)
	if len(claims) == 0 {
		if !criticalVeto {
			return nil
		}
		return &ConsensusMetrics{CriticalVeto: true}
	}
	metrics := metricsFromClaimCounts(claims, len(responses), threshold)
	metrics.CriticalVeto = criticalVeto
	return &metrics
}

func structuredCriticalVeto(responses []ProviderResponse) bool {
	for _, response := range responses {
		var review ReviewerOutput
		if err := json.Unmarshal([]byte(response.Output), &review); err != nil {
			continue
		}
		statuses := findingStatusMap(review.FindingStatus)
		for _, finding := range review.Findings {
			identity := stableFindingIdentity(finding)
			if isCriticalVetoFinding(finding) && findingEvidenceStatus(finding, identity, statuses) != "resolved" {
				return true
			}
		}
	}
	return false
}

func metricsFromFindingClaims(claims map[string]*findingClaim, providers int, threshold float64) ConsensusMetrics {
	metrics := ConsensusMetrics{TotalClaims: len(claims)}
	identities := make([]string, 0, len(claims))
	for identity := range claims {
		identities = append(identities, identity)
	}
	sort.Strings(identities)
	for _, identity := range identities {
		claim := claims[identity]
		if float64(len(claim.providers))/float64(providers) >= threshold {
			metrics.AgreedClaims++
		} else {
			metrics.DissentClaims++
		}
		if findingClaimHasCriticalVeto(*claim) {
			metrics.CriticalVeto = true
		}
		metrics.FindingClaims = append(metrics.FindingClaims, FindingConsensusClaim{
			Identity: claim.identity, Finding: claim.finding,
			Evidence:     append([]FindingClaimEvidence(nil), claim.evidence...),
			CriticalVeto: findingClaimHasCriticalVeto(*claim),
		})
	}
	return metrics
}

func collectTextConsensusClaims(responses []ProviderResponse) map[string]int {
	claims := make(map[string]int)
	for _, response := range responses {
		seen := make(map[string]bool)
		if items, err := parseStructuredResponse(response.Output); err == nil {
			for _, item := range items {
				identity := normalizeLine(item)
				if identity != "" {
					seen[identity] = true
				}
			}
		} else {
			for _, line := range splitLines(response.Output) {
				identity := normalizeLine(line)
				if identity != "" {
					seen[identity] = true
				}
			}
		}
		for identity := range seen {
			claims[identity]++
		}
	}
	return claims
}

func metricsFromClaimCounts(claims map[string]int, providers int, threshold float64) ConsensusMetrics {
	metrics := ConsensusMetrics{TotalClaims: len(claims)}
	for _, count := range claims {
		if float64(count)/float64(providers) >= threshold {
			metrics.AgreedClaims++
		} else {
			metrics.DissentClaims++
		}
	}
	return metrics
}

var (
	lineColumnSuffix = regexp.MustCompile(`(?i)(?::\d+(?::\d+)?(?:-\d+)?)$`)
	githubLineSuffix = regexp.MustCompile(`(?i)#L\d+(?:-L?\d+)?$`)
	wordLineSuffix   = regexp.MustCompile(`(?i)\s*\(?lines?\s+\d+(?:-\d+)?\)?$`)
)

func stableFindingIdentity(finding Finding) string {
	if id := normalizeLine(finding.ID); id != "" {
		return "id|" + id
	}
	scope := finding.ScopeRef
	if strings.TrimSpace(scope) == "" {
		scope = finding.Location
	}
	parts := []string{
		normalizeLine(finding.Category),
		normalizeStableScope(scope),
		normalizeLine(finding.Description),
	}
	return strings.Join(parts, "|")
}

func normalizeStableScope(scope string) string {
	scope = strings.TrimSpace(scope)
	scope = lineColumnSuffix.ReplaceAllString(scope, "")
	scope = githubLineSuffix.ReplaceAllString(scope, "")
	scope = wordLineSuffix.ReplaceAllString(scope, "")
	return normalizeLine(scope)
}

func formatFindingClaim(claim findingClaim, count, total int) string {
	finding := claim.finding
	location := strings.TrimSpace(finding.ScopeRef)
	if location == "" {
		location = strings.TrimSpace(finding.Location)
	}
	if location != "" {
		location = " @ " + location
	}
	line := fmt.Sprintf("[%s/%s] %s%s [%d/%d]", strings.ToLower(finding.Severity), strings.ToLower(finding.Category), strings.TrimSpace(finding.Description), location, count, total)
	return line + formatFindingEvidence(claim.evidence)
}

func isCriticalVetoFinding(finding Finding) bool {
	if !strings.EqualFold(strings.TrimSpace(finding.Severity), "critical") {
		return false
	}
	category := strings.ToLower(strings.TrimSpace(finding.Category))
	return category == "security" || category == "correctness"
}
