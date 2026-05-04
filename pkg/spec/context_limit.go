package spec

// AdaptiveContextLimit returns the SPEC review context line budget based on the
// number of unique cited files. The mapping is:
//
//   - 0..2 cited files  -> 500 lines
//   - 3..5 cited files  -> 1500 lines
//   - 6+ cited files    -> 3000 lines (hard cap)
//
// When ceiling > 0, the result is capped at ceiling (min(mapped, ceiling)).
// When ceiling <= 0, the ceiling is ignored and the mapped value is returned.
//
// Negative citedFileCount is treated as 0.
func AdaptiveContextLimit(citedFileCount, ceiling int) int {
	mapped := mapCitedFilesToLimit(citedFileCount)
	if ceiling > 0 && ceiling < mapped {
		return ceiling
	}
	return mapped
}

func mapCitedFilesToLimit(citedFileCount int) int {
	switch {
	case citedFileCount <= 2:
		return 500
	case citedFileCount <= 5:
		return 1500
	default:
		return 3000
	}
}

// ExtractSpecContextTargetsForCLI exposes extractSpecContextTargets for callers
// outside the spec package (e.g. internal/cli) that need the same set of cited
// file paths CollectContextForSpec uses internally.
func ExtractSpecContextTargetsForCLI(specDir string) []string {
	return extractSpecContextTargets(specDir)
}

// ResolveSpecTargetPathForCLI exposes resolveSpecTargetPath for callers outside
// the spec package. It returns the resolved absolute path or "" if the target
// cannot be located on disk.
func ResolveSpecTargetPathForCLI(projectRoot, moduleRoot, target string) string {
	return resolveSpecTargetPath(projectRoot, moduleRoot, target)
}
