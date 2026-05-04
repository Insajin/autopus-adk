package spec

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidContextOverride is returned by ParseReviewContextOverride when the
// review_context_lines frontmatter value is outside the accepted range
// (must satisfy 0 < value <= 10000).
// @AX:NOTE: [AUTO] magic constant — 10000 is the upper bound for review_context_lines override (REQ-CTX-3); matches CLI warning message
var ErrInvalidContextOverride = errors.New("review_context_lines must be >0 and <=10000")

// ParseReviewContextOverride scans the SPEC frontmatter for the
// "review_context_lines" key and returns the parsed override.
//
// Return semantics:
//   - present=false, value=0,  err=nil                          : key absent
//   - present=true,  value=N,  err=nil                          : valid (0 < N <= 10000)
//   - present=true,  value=N,  err=ErrInvalidContextOverride    : out of range; N is echoed for logging
//   - present=true,  value=0,  err=<strconv error>              : non-integer value
func ParseReviewContextOverride(content string) (int, bool, error) {
	lines := strings.Split(content, "\n")
	frontmatter := parseSpecFrontmatter(lines)
	raw, ok := frontmatter["review_context_lines"]
	if !ok {
		return 0, false, nil
	}
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, true, fmt.Errorf("parse review_context_lines: %w", err)
	}
	if value <= 0 || value > 10000 {
		return value, true, ErrInvalidContextOverride
	}
	return value, true, nil
}

// @AX:NOTE: [AUTO] magic constants — 500/1500/3000 are calibrated budget tiers (REQ-CTX-1); adjust only with SPEC update
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
