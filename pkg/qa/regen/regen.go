package regen

// BuildResult composes the Unit 1 pipeline over a project directory: detect
// present surfaces, synthesize+evaluate one pack per surface, load existing
// packs (parse-only so an invalid existing pack does not abort), and compute
// the deterministic diff over accepted packs. The returned Diff is NOT yet
// redacted; callers that serialize or persist it MUST run RedactDiff and gate
// on AssertDiffSafe. Unit 2 consumes RegenResult to drive approval and apply.
func BuildResult(projectDir string) (RegenResult, error) {
	surfaces := PresentSurfaces(projectDir)
	synthesized := Synthesize(projectDir, surfaces)
	existing, err := LoadExistingPacks(projectDir)
	if err != nil {
		return RegenResult{}, err
	}
	return RegenResult{
		Surfaces:    surfaces,
		Synthesized: synthesized,
		Diff:        ComputeDiff(synthesized, existing),
	}, nil
}
