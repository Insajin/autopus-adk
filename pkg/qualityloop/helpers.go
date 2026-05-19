package qualityloop

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

func stableCandidateID(input FailureInput, kind string, reasons []string) string {
	seed := strings.Join([]string{input.SourceArtifactType, input.SourceID, input.FailureFingerprint, kind, strings.Join(reasons, ",")}, "|")
	sum := sha256.Sum256([]byte(seed))
	return "ic-" + hex.EncodeToString(sum[:])[:16]
}

func sourceFailureRefs(input FailureInput) []string {
	if input.SourceID == "" && input.SourceArtifactType == "" {
		return nil
	}
	return []string{strings.Trim(strings.Join([]string{input.SourceArtifactType, input.SourceID}, ":"), ":")}
}

func routeTargets(input FailureInput, kind string) []string {
	targets := make([]string, 0)
	if input.TargetArtifact != "" {
		targets = append(targets, input.TargetArtifact)
	}
	targets = append(targets, input.OwnedPaths...)
	if len(targets) == 0 {
		targets = append(targets, kind)
	}
	return uniqueStrings(targets)
}

func generatedSurfaceValidation(target string) string {
	if isGeneratedSurfacePath(target) {
		return "rejected"
	}
	if strings.TrimSpace(target) == "" {
		return "not_applicable"
	}
	return "source_owned"
}

func ownerFor(kind string) string {
	switch kind {
	case KindQAMESHRepairHandoff, KindSkillEvolveCandidate, KindPromptLayerUpdate:
		return "autopus-adk"
	case KindSourceSetupMission, KindWorkspaceEvolutionSignal, KindOperatingPackCandidate, KindLaunchGateBlocker, KindAgentEvalRemediation:
		return "Autopus/backend"
	default:
		return "quality-loop"
	}
}

func ownerRepoFor(input FailureInput, kind string) string {
	if input.TargetArtifact != "" {
		switch {
		case strings.HasPrefix(input.TargetArtifact, "autopus-adk/"):
			return "autopus-adk"
		case strings.HasPrefix(input.TargetArtifact, "Autopus/"):
			return "Autopus"
		case strings.HasPrefix(input.TargetArtifact, "autopus-desktop/"):
			return "autopus-desktop"
		}
	}
	return ownerFor(kind)
}

func severityFor(status, taxonomy string) string {
	if status == StatusRejected || taxonomy == TaxonomyUnsafeMutationBoundary || taxonomy == TaxonomySafetyPolicyGap {
		return "critical"
	}
	if status == StatusQuarantined || status == StatusBlocked || status == StatusAwaitingReplay {
		return "high"
	}
	return "medium"
}

func riskFor(taxonomy string) string {
	switch taxonomy {
	case TaxonomyUnsafeMutationBoundary, TaxonomySafetyPolicyGap:
		return "critical"
	case TaxonomyProductBug, TaxonomyStaleOrMissingEvidence:
		return "high"
	default:
		return "medium"
	}
}

func expectedValidationFor(kind string) string {
	switch kind {
	case KindQAMESHRepairHandoff:
		return "auto qa feedback and deterministic replay"
	case KindEvalCalibrationTask:
		return "eval calibration governance evidence"
	default:
		return "deterministic replay or post-apply evidence"
	}
}

func proposedActionFor(input FailureInput, kind string) string {
	if input.ProposedActionKind != "" {
		return input.ProposedActionKind
	}
	if kind == KindQAMESHRepairHandoff && len(input.OwnedPaths) > 0 && len(input.EvidenceRefs) > 0 {
		return "auto qa feedback --to " + input.OwnedPaths[0] + " --evidence " + input.EvidenceRefs[0]
	}
	return ""
}

func rollbackFor(kind string) string {
	if kind == KindSourceSetupMission || kind == KindWorkspaceEvolutionSignal {
		return "archive candidate without provider writes"
	}
	return "revert target artifact after approval audit"
}

func safetyReasonCodes(decision SafetyDecision) []string {
	if decision.Accepted {
		return nil
	}
	return append([]string{}, decision.ReasonCodes...)
}

func clampConfidence(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func appendUniqueMany(values, next []string) []string {
	for _, value := range next {
		values = appendUnique(values, value)
	}
	return values
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsAny(values []string, targets ...string) bool {
	for _, target := range targets {
		if contains(values, target) {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		out = appendUnique(out, value)
	}
	return out
}

var unsafeTextPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:file://)?/(?:Users|home|private|tmp|var|etc)/`),
	regexp.MustCompile(`(?i)[?&](?:x-amz-signature|x-goog-signature|signature|token|secret|password|passwd|api[_-]?key|credential|cookie|session|auth)=`),
	regexp.MustCompile(`(?i)\b(?:Authorization:\s*)?Bearer\s+[A-Za-z0-9._~+/=-]{8,}`),
	regexp.MustCompile(`(?i)\b(?:sk-|gh[pousr]_|github_pat_)[A-Za-z0-9._-]{8,}`),
	regexp.MustCompile(`(?i)\.(?:codex|opencode|claude|gemini)/`),
	regexp.MustCompile(`(?i)\.autopus/(?:runtime|plugins|brainstorms|orchestra)/`),
	regexp.MustCompile(`(?i)\.agents/plugins/marketplace\.json`),
}

func unsafeText(value string) bool {
	for _, pattern := range unsafeTextPatterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	return false
}

func digestStrings(groups ...[]string) string {
	hash := sha256.New()
	for _, group := range groups {
		for _, value := range group {
			_, _ = hash.Write([]byte(value))
			_, _ = hash.Write([]byte{0})
		}
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}
