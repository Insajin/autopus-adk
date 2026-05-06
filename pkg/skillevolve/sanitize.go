package skillevolve

func SanitizeGenerationResult(result CandidateGenerationResult) CandidateGenerationResult {
	sanitized := CandidateGenerationResult{
		Candidates: make([]CandidateBundle, 0, len(result.Candidates)),
	}
	for _, candidate := range result.Candidates {
		sanitized.Candidates = append(sanitized.Candidates, SanitizeCandidate(candidate))
	}
	return sanitized
}

func SanitizeCandidate(candidate CandidateBundle) CandidateBundle {
	candidate.ProposedFiles = append([]ProposedFile{}, candidate.ProposedFiles...)
	for i := range candidate.ProposedFiles {
		candidate.ProposedFiles[i].Content = ""
	}
	return candidate
}
