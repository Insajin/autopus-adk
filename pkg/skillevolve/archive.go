package skillevolve

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type archiveRecord struct {
	Status             string              `json:"status"`
	ReasonCode         string              `json:"reason_code"`
	CandidateID        string              `json:"candidate_id"`
	Provenance         CandidateProvenance `json:"provenance"`
	ReplayEvidenceRefs []string            `json:"replay_evidence_refs,omitempty"`
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-SKILL-EVOLVE-001: archive records preserve rejection reason, provenance, and replay evidence.
// @AX:REASON: Auditability depends on stale or rejected candidates retaining the same source failure and evidence references after quarantine.
func ArchiveCandidate(ctx context.Context, opts ArchiveOptions) (ArchiveResult, error) {
	if err := ctx.Err(); err != nil {
		return ArchiveResult{}, err
	}
	if opts.Reason == "" {
		return ArchiveResult{}, errors.New("archive reason is required")
	}
	quarantineDir := opts.QuarantineDir
	if quarantineDir == "" {
		quarantineDir = filepath.Join(".autopus", "skill-evolve", "quarantine")
	}
	archiveDir := filepath.Join(quarantineDir, "archive")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return ArchiveResult{}, err
	}
	candidateID := opts.Candidate.ID
	if candidateID == "" {
		candidateID = "cand-" + shortDigest(hashJSON(opts.Candidate))
	}
	result := ArchiveResult{
		Status:             "archived",
		ReasonCode:         opts.Reason,
		Provenance:         provenanceFor(opts.Candidate),
		ReplayEvidenceRefs: append([]string{}, opts.Candidate.ReplayEvidenceRefs...),
		ArchivePath:        filepath.Join(archiveDir, safeFileName(candidateID)+".json"),
	}
	record := archiveRecord{
		Status:             result.Status,
		ReasonCode:         result.ReasonCode,
		CandidateID:        candidateID,
		Provenance:         result.Provenance,
		ReplayEvidenceRefs: result.ReplayEvidenceRefs,
	}
	body, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return ArchiveResult{}, err
	}
	if err := os.WriteFile(result.ArchivePath, append(body, '\n'), 0o644); err != nil {
		return ArchiveResult{}, err
	}
	return result, nil
}

func provenanceFor(candidate CandidateBundle) CandidateProvenance {
	provenance := candidate.Provenance
	if len(provenance.SourceFailureRefs) == 0 {
		for _, failure := range candidate.SourceFailures {
			provenance.SourceFailureRefs = append(provenance.SourceFailureRefs, failure.Ref)
			provenance.EvidenceRefs = appendUnique(provenance.EvidenceRefs, failure.EvidenceRef)
		}
	}
	if len(provenance.SourceHashes) == 0 {
		provenance.SourceHashes = append([]string{}, candidate.SourceHashes...)
	}
	if provenance.GenerationPromptDigest == "" {
		provenance.GenerationPromptDigest = candidate.GenerationPromptDigest
	}
	if provenance.RedactionStatus == "" {
		provenance.RedactionStatus = candidate.RedactionStatus
	}
	if provenance.Creator == "" {
		provenance.Creator = candidate.Creator
	}
	if len(provenance.AffectedAcceptanceIDs) == 0 {
		provenance.AffectedAcceptanceIDs = append([]string{}, candidate.AffectedAcceptanceIDs...)
	}
	if len(provenance.AffectedSourceOfTruths) == 0 {
		provenance.AffectedSourceOfTruths = append([]string{}, candidate.AffectedRefs...)
	}
	if len(provenance.AffectedGeneratedSurfaces) == 0 {
		provenance.AffectedGeneratedSurfaces = append([]string{}, candidate.AffectedGeneratedSurfaces...)
	}
	return provenance
}
