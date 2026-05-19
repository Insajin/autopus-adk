package memindex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
	"github.com/insajin/autopus-adk/pkg/qualityloop"
)

func scanImprovementCandidates(projectDir string) ([]Record, []Skip, error) {
	root := filepath.Join(projectDir, ".autopus", "quality-loop", "candidates")
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	records := make([]Record, 0)
	skips := make([]Skip, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		record, ok, skip, err := improvementCandidateRecord(projectDir, path)
		if err != nil {
			return err
		}
		if !ok {
			skips = append(skips, skip)
			return nil
		}
		records = append(records, record)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return records, skips, nil
}

func improvementCandidateRecord(projectDir, path string) (Record, bool, Skip, error) {
	rel := slashRel(projectDir, path)
	body, err := os.ReadFile(path)
	if err != nil {
		return Record{}, false, Skip{}, err
	}
	var candidate qualityloop.ImprovementCandidate
	if err := json.Unmarshal(body, &candidate); err != nil {
		return Record{}, false, Skip{Path: rel, Reason: "invalid_improvement_candidate"}, nil
	}
	if candidate.SchemaVersion != qualityloop.SchemaVersion {
		return Record{}, false, Skip{Path: rel, Reason: "invalid_improvement_candidate"}, nil
	}
	if candidate.RawPayloadPresent || candidate.ProviderWriteCallCount != 0 {
		return Record{}, false, Skip{Path: rel, Reason: "unsafe_improvement_candidate"}, nil
	}
	if !improvementCandidateRedactionReady(candidate.RedactionStatus) {
		return Record{}, false, Skip{Path: rel, Reason: "unredacted_improvement_candidate"}, nil
	}
	content := improvementCandidateText(candidate)
	if findings := qaevidence.FindUnsafeText(content, rel); len(findings) > 0 {
		return Record{}, false, Skip{Path: rel, Reason: "unsafe_source_text"}, nil
	}
	if !safeImprovementCandidateProjection(candidate, rel) {
		return Record{}, false, Skip{Path: rel, Reason: "unsafe_source_text"}, nil
	}
	hash, err := hashFile(path)
	if err != nil {
		return Record{}, false, Skip{}, err
	}
	severity := candidate.Severity
	if severity == "" {
		severity = improvementCandidateSeverity(candidate)
	}
	return Record{
		SourceType:      "improvement_candidate",
		SourceRef:       rel,
		SourceHash:      hash,
		Title:           safeText("Improvement candidate " + candidate.CandidateID),
		Summary:         safeText(content),
		Tags:            improvementCandidateTags(candidate),
		SpecID:          "SPEC-QUALITY-IMPROVEMENT-LOOP-001",
		AcceptanceIDs:   candidate.AffectedAcceptanceIDs,
		FileRefs:        candidate.AffectedRefs,
		PackageRefs:     []string{candidate.OwningRepo},
		Severity:        severity,
		RedactionStatus: candidate.RedactionStatus,
		Content:         safeText(content),
		SourceMetadata: map[string]any{
			"candidate_id":           candidate.CandidateID,
			"candidate_kind":         candidate.CandidateKind,
			"status":                 candidate.Status,
			"active":                 candidate.Active,
			"source_hashes":          candidate.SourceHashes,
			"taxonomy":               candidate.FailureTaxonomy,
			"confidence_band":        candidate.ConfidenceBand,
			"recommended_route":      candidate.RecommendedRoute,
			"last_replay_status":     candidate.LastReplayStatus,
			"repair_action_policy":   candidate.RepairActionPolicy,
			"provider_write_count":   candidate.ProviderWriteCallCount,
			"raw_payload_present":    candidate.RawPayloadPresent,
			"audit_refs":             candidate.AuditRefs,
			"replay_evidence_refs":   candidate.ReplayEvidenceRefs,
			"quality_index_refs":     candidate.QualityIndexRefs,
			"non_convergence_reason": candidate.NonConvergenceReason,
		},
	}, true, Skip{}, nil
}

func improvementCandidateSeverity(candidate qualityloop.ImprovementCandidate) string {
	switch candidate.Status {
	case qualityloop.StatusRejected, qualityloop.StatusQuarantined, qualityloop.StatusAwaitingReplay, qualityloop.StatusBlocked:
		return "high"
	case qualityloop.StatusVerified:
		return "low"
	default:
		return "medium"
	}
}

func improvementCandidateRedactionReady(status string) bool {
	switch status {
	case qualityloop.RedactionRedacted, qualityloop.RedactionMetadataOnly, "not_required":
		return true
	default:
		return false
	}
}

func improvementCandidateText(candidate qualityloop.ImprovementCandidate) string {
	parts := []string{
		fmt.Sprintf("candidate %s", candidate.CandidateID),
		fmt.Sprintf("kind %s", candidate.CandidateKind),
		fmt.Sprintf("status %s", candidate.Status),
		fmt.Sprintf("taxonomy %s", candidate.FailureTaxonomy),
		fmt.Sprintf("reason_codes %s", strings.Join(candidate.ReasonCodes, " ")),
		fmt.Sprintf("confidence %s %.2f", candidate.ConfidenceBand, candidate.ClassificationConfidence),
		fmt.Sprintf("route %s", candidate.RecommendedRoute),
		fmt.Sprintf("owner %s %s", candidate.Owner, candidate.OwningRepo),
		fmt.Sprintf("target %s", candidate.TargetArtifact),
		fmt.Sprintf("expected_validation %s", candidate.ExpectedValidation),
		fmt.Sprintf("rollback %s", candidate.RollbackPath),
		fmt.Sprintf("replay %s", candidate.LastReplayStatus),
		fmt.Sprintf("next_action %s", candidate.RepairActionPolicy),
	}
	return strings.Join(parts, "\n")
}

func improvementCandidateTags(candidate qualityloop.ImprovementCandidate) []string {
	tags := []string{
		"quality-loop",
		candidate.CandidateKind,
		candidate.Status,
		candidate.FailureTaxonomy,
		candidate.ConfidenceBand,
	}
	tags = append(tags, candidate.ReasonCodes...)
	return uniqueStrings(tags)
}

func safeImprovementCandidateProjection(candidate qualityloop.ImprovementCandidate, rel string) bool {
	refs := make([]string, 0)
	refs = append(refs, candidate.AffectedRefs...)
	refs = append(refs, candidate.AuditRefs...)
	refs = append(refs, candidate.ReplayEvidenceRefs...)
	refs = append(refs, candidate.QualityIndexRefs...)
	refs = append(refs, candidate.SourceHashes...)
	refs = append(refs, candidate.RouteTargets...)
	if findings := qaevidence.FindUnsafeText(strings.Join(refs, "\n"), rel); len(findings) > 0 {
		return false
	}
	for _, ref := range refs {
		if memindexCrossWorkspaceRef(candidate.WorkspaceID, ref) {
			return false
		}
	}
	return true
}

func memindexCrossWorkspaceRef(workspaceID, ref string) bool {
	if strings.TrimSpace(workspaceID) == "" {
		return false
	}
	for _, id := range memindexWorkspaceRefs(ref) {
		if id != workspaceID {
			return true
		}
	}
	return false
}

func memindexWorkspaceRefs(ref string) []string {
	const marker = "workspace:"
	var ids []string
	for start := strings.Index(ref, marker); start >= 0; start = strings.Index(ref, marker) {
		ref = ref[start+len(marker):]
		end := strings.IndexAny(ref, ":/?&#| ,")
		if end < 0 {
			end = len(ref)
		}
		if end > 0 {
			ids = append(ids, ref[:end])
		}
		if end >= len(ref) {
			break
		}
		ref = ref[end:]
	}
	return ids
}
