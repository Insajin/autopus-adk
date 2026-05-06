package skillevolve

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type qualityIndex struct {
	Failures []qualityIndexFailure `json:"failures"`
}

type qualityIndexFailure struct {
	Ref             string   `json:"ref"`
	Fingerprint     string   `json:"fingerprint"`
	SourceHash      string   `json:"source_hash"`
	EvidenceRef     string   `json:"evidence_ref"`
	AffectedRefs    []string `json:"affected_refs"`
	AcceptanceRefs  []string `json:"acceptance_refs"`
	Expected        string   `json:"expected"`
	Actual          string   `json:"actual"`
	FailureSeverity string   `json:"failure_severity"`
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-SKILL-EVOLVE-001: repeated failure fingerprints generate inactive quarantined bundles only.
// @AX:REASON: The generator must not mutate canonical source or generated harness surfaces before replay and promotion gates run.
func GenerateCandidates(ctx context.Context, opts CandidateGenerationOptions) (CandidateGenerationResult, error) {
	if err := ctx.Err(); err != nil {
		return CandidateGenerationResult{}, err
	}
	if opts.QualityIndexPath == "" {
		return CandidateGenerationResult{}, errors.New("quality index path is required")
	}
	if opts.MinCount <= 0 {
		opts.MinCount = 2
	}
	if opts.QuarantineDir == "" {
		root := opts.ProjectDir
		if root == "" {
			root = "."
		}
		opts.QuarantineDir = filepath.Join(root, ".autopus", "skill-evolve", "quarantine")
	}

	index, err := readQualityIndex(opts.QualityIndexPath)
	if err != nil {
		return CandidateGenerationResult{}, err
	}

	groups := groupFailures(index.Failures)
	fingerprints := make([]string, 0, len(groups))
	for fingerprint, failures := range groups {
		if len(failures) >= opts.MinCount {
			fingerprints = append(fingerprints, fingerprint)
		}
	}
	sort.Strings(fingerprints)

	result := CandidateGenerationResult{Candidates: make([]CandidateBundle, 0, len(fingerprints))}
	for _, fingerprint := range fingerprints {
		if err := ctx.Err(); err != nil {
			return CandidateGenerationResult{}, err
		}
		candidate := buildCandidateBundle(fingerprint, groups[fingerprint], opts)
		safety, err := EvaluateSafety(ctx, candidate, SafetyOptions{})
		if err != nil {
			return CandidateGenerationResult{}, err
		}
		candidate.SafetyReasonCodes = append([]string{}, safety.ReasonCodes...)
		if !safety.Allowed {
			candidate.Status = "rejected"
			candidate.Active = false
			candidate.RedactionStatus = "failed"
			candidate.Provenance.RedactionStatus = "failed"
		}
		if err := writeCandidateBundle(&candidate, opts.QuarantineDir); err != nil {
			return CandidateGenerationResult{}, err
		}
		result.Candidates = append(result.Candidates, candidate)
	}
	return result, nil
}

func readQualityIndex(path string) (qualityIndex, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return qualityIndex{}, err
	}
	var index qualityIndex
	if err := json.Unmarshal(body, &index); err != nil {
		return qualityIndex{}, err
	}
	return index, nil
}

func groupFailures(failures []qualityIndexFailure) map[string][]qualityIndexFailure {
	groups := make(map[string][]qualityIndexFailure)
	for _, failure := range failures {
		if failure.Fingerprint == "" {
			continue
		}
		groups[failure.Fingerprint] = append(groups[failure.Fingerprint], failure)
	}
	for fingerprint := range groups {
		sort.SliceStable(groups[fingerprint], func(i, j int) bool {
			return groups[fingerprint][i].Ref < groups[fingerprint][j].Ref
		})
	}
	return groups
}

func buildCandidateBundle(fingerprint string, failures []qualityIndexFailure, opts CandidateGenerationOptions) CandidateBundle {
	sourceFailures := make([]SourceFailure, 0, len(failures))
	sourceRefs := make([]string, 0, len(failures))
	evidenceRefs := make([]string, 0, len(failures))
	sourceHashes := make([]string, 0, len(failures))
	var affectedRefs []string
	var acceptanceRefs []string

	for _, failure := range failures {
		hash := failure.SourceHash
		if hash == "" {
			hash = hashJSON(failure)
		}
		sourceFailures = append(sourceFailures, SourceFailure{
			Ref:         failure.Ref,
			Hash:        hash,
			EvidenceRef: failure.EvidenceRef,
		})
		sourceRefs = append(sourceRefs, failure.Ref)
		evidenceRefs = appendUnique(evidenceRefs, failure.EvidenceRef)
		sourceHashes = appendUnique(sourceHashes, hash)
		affectedRefs = appendUniqueMany(affectedRefs, failure.AffectedRefs)
		acceptanceRefs = appendUniqueMany(acceptanceRefs, failure.AcceptanceRefs)
	}
	sort.Strings(sourceHashes)
	sort.Strings(affectedRefs)
	sort.Strings(acceptanceRefs)

	proposedFiles := proposedFilesFor(fingerprint, affectedRefs, acceptanceRefs)
	proposedDigest := hashJSON(proposedFiles)
	promptDigest := hashJSON(map[string]any{
		"fingerprint":     fingerprint,
		"affected_refs":   affectedRefs,
		"acceptance_refs": acceptanceRefs,
		"min_count":       opts.MinCount,
	})
	id := "cand-" + safeFileName(fingerprint) + "-" + shortDigest(promptDigest)
	provenance := CandidateProvenance{
		SourceFailureRefs:      sourceRefs,
		SourceHashes:           sourceHashes,
		EvidenceRefs:           evidenceRefs,
		GenerationPromptDigest: promptDigest,
		RedactionStatus:        "passed",
		Creator:                opts.Creator,
		AffectedAcceptanceIDs:  acceptanceRefs,
		AffectedSourceOfTruths: affectedRefs,
	}
	return CandidateBundle{
		ID:                     id,
		Fingerprint:            fingerprint,
		Status:                 "quarantined",
		Active:                 false,
		Creator:                opts.Creator,
		RedactionStatus:        "passed",
		SourceFailures:         sourceFailures,
		SourceHashes:           sourceHashes,
		AffectedRefs:           affectedRefs,
		AffectedAcceptanceIDs:  acceptanceRefs,
		ProposedDigest:         proposedDigest,
		GenerationPromptDigest: promptDigest,
		ReplayPlan: ReplayPlan{
			RunIndexPath:   firstNonEmpty(evidenceRefs),
			Commands:       []ReplayCommand{{Command: "go test ./pkg/skillevolve -run Replay -count=1"}},
			MustChecks:     defaultMustChecks(fingerprint),
			AcceptanceRefs: []string{"AC-SEVOLVE-001", "AC-SEVOLVE-003"},
		},
		OwnedPaths:         affectedRefs,
		ProposedFiles:      proposedFiles,
		Provenance:         provenance,
		ReplayEvidenceRefs: evidenceRefs,
	}
}

func firstNonEmpty(values []string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func defaultMustChecks(fingerprint string) []ReplayCheckRef {
	if fingerprint == "oracle.structural_only.missing_semantic_output" {
		return []ReplayCheckRef{{
			ID:            "must-semantic-output",
			AcceptanceRef: "AC-SEVOLVE-003",
			Source:        "qamesh-run-1/run-index.json",
		}}
	}
	return []ReplayCheckRef{{
		ID:            safeFileName(fingerprint),
		AcceptanceRef: "AC-SEVOLVE-003",
		Source:        "quality-index",
	}}
}

func proposedFilesFor(fingerprint string, affectedRefs, acceptanceRefs []string) []ProposedFile {
	// @AX:NOTE [AUTO] @AX:SPEC: SPEC-SKILL-EVOLVE-001: fallback proposal target is canonical ADK content, not a generated skill surface.
	target := "autopus-adk/content/skills/testing-strategy.md"
	for _, ref := range affectedRefs {
		if isADKSourceOfTruthPath(ref) && filepath.Ext(ref) == ".md" {
			target = ref
			break
		}
	}
	content := "---\nname: skill-evolve-candidate\ndescription: Candidate skill improvement\n---\n"
	content += "# Skill Evolution Candidate\n\n"
	content += "Fingerprint: " + fingerprint + "\n\n"
	content += "Acceptance refs: " + joinComma(acceptanceRefs) + "\n"
	return []ProposedFile{{Path: target, Content: content}}
}

func writeCandidateBundle(candidate *CandidateBundle, quarantineDir string) error {
	if err := os.MkdirAll(quarantineDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(quarantineDir, candidate.ID+".json")
	candidate.BundlePath = path
	body, err := json.MarshalIndent(candidate, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o644)
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func appendUniqueMany(values []string, additions []string) []string {
	for _, addition := range additions {
		values = appendUnique(values, addition)
	}
	return values
}

func joinComma(values []string) string {
	return strings.Join(values, ",")
}
