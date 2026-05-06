package skillevolve

import (
	"path"
	"path/filepath"
	"strings"
)

func replayEvidenceBindingFailures(projectDir string, candidate CandidateBundle, runIndexPath string) []string {
	if candidateHasSourceHash(candidate) && candidateEvidenceReferencesPath(projectDir, candidate, runIndexPath) {
		return nil
	}
	return []string{"replay_evidence_not_in_provenance"}
}

func candidateHasSourceHash(candidate CandidateBundle) bool {
	for _, hash := range candidate.SourceHashes {
		if strings.HasPrefix(hash, "sha256:") {
			return true
		}
	}
	for _, hash := range candidate.Provenance.SourceHashes {
		if strings.HasPrefix(hash, "sha256:") {
			return true
		}
	}
	for _, failure := range candidate.SourceFailures {
		if strings.HasPrefix(failure.Hash, "sha256:") {
			return true
		}
	}
	return false
}

func candidateEvidenceReferencesPath(projectDir string, candidate CandidateBundle, target string) bool {
	refs := append([]string{}, candidate.ReplayEvidenceRefs...)
	refs = append(refs, candidate.Provenance.EvidenceRefs...)
	for _, failure := range candidate.SourceFailures {
		refs = append(refs, failure.EvidenceRef)
	}
	for _, ref := range refs {
		if evidenceRefMatchesProjectPath(projectDir, ref, target) {
			return true
		}
	}
	return false
}

func evidenceRefMatchesProjectPath(projectDir, ref, target string) bool {
	refPath := stripEvidenceFragment(ref)
	targetPath := stripEvidenceFragment(target)
	if refPath == "" || targetPath == "" {
		return false
	}
	if cleanEvidencePath(refPath) == cleanEvidencePath(targetPath) {
		return true
	}
	refAbs, refOK := canonicalEvidencePath(resolveWorkspacePath(projectDir, refPath))
	targetAbs, targetOK := canonicalEvidencePath(resolveWorkspacePath(projectDir, targetPath))
	return refOK && targetOK && refAbs == targetAbs
}

func stripEvidenceFragment(value string) string {
	value = strings.TrimSpace(value)
	if idx := strings.Index(value, "#"); idx >= 0 {
		return value[:idx]
	}
	return value
}

func cleanEvidencePath(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	if value == "" {
		return ""
	}
	if path.IsAbs(value) {
		return path.Clean(value)
	}
	return cleanRelPath(value)
}

func canonicalEvidencePath(value string) (string, bool) {
	if value == "" {
		return "", false
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		return "", false
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return filepath.Clean(abs), true
}

func expectedReplayCommand(candidate CandidateBundle) string {
	for _, command := range candidate.ReplayPlan.Commands {
		if strings.TrimSpace(command.Command) != "" {
			return strings.TrimSpace(command.Command)
		}
	}
	return ""
}

func hasReplayAcceptanceMapping(candidate CandidateBundle) bool {
	return hasAffectedOutputMapping(candidate) && hasAffectedAcceptanceMapping(candidate)
}

func hasAffectedOutputMapping(candidate CandidateBundle) bool {
	for _, ref := range candidate.AffectedRefs {
		if cleanRelPath(ref) != "" {
			return true
		}
	}
	for _, file := range candidate.ProposedFiles {
		if cleanRelPath(file.Path) != "" {
			return true
		}
	}
	for _, ref := range candidate.Provenance.AffectedSourceOfTruths {
		if cleanRelPath(ref) != "" {
			return true
		}
	}
	return false
}

func hasAffectedAcceptanceMapping(candidate CandidateBundle) bool {
	for _, id := range candidate.AffectedAcceptanceIDs {
		if strings.TrimSpace(id) != "" {
			return true
		}
	}
	for _, id := range candidate.ReplayPlan.AcceptanceRefs {
		if strings.TrimSpace(id) != "" {
			return true
		}
	}
	for _, id := range candidate.Provenance.AffectedAcceptanceIDs {
		if strings.TrimSpace(id) != "" {
			return true
		}
	}
	for _, check := range candidate.ReplayPlan.MustChecks {
		if strings.TrimSpace(check.AcceptanceRef) != "" {
			return true
		}
	}
	return false
}

func statusOrUnknown(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "unknown"
	}
	return status
}

func appendFailureReasons(reasons []string, additions []string) []string {
	for _, addition := range additions {
		reasons = appendReason(reasons, addition)
	}
	return reasons
}
