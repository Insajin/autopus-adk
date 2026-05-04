package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/spec"
)

// resolveSpecReviewContextLimit derives the per-run SPEC review context line
// budget by counting cited files, applying the adaptive mapping, honoring an
// optional frontmatter override, and finally capping by the configured ceiling.
//
// Returns:
//   - cited: number of unique resolved files referenced from plan/research docs
//   - applied: final line budget passed to spec.CollectContextForSpec
//   - override: "frontmatter" when a frontmatter override was consumed, else ""
//   - err: non-nil only on unrecoverable I/O errors; invalid frontmatter values
//     are reported via stderr and the function falls back to the adaptive mapping.
// @AX:WARN: [AUTO] multi-stage decision chain — adaptive → frontmatter override → ceiling cap; @AX:REASON: three independent override paths interact; incorrect ordering silently ignores a higher-priority source
func resolveSpecReviewContextLimit(projectRoot, specDir string, ceiling int, stderr io.Writer) (cited, applied int, override string, err error) {
	cited = countSpecCitedFiles(projectRoot, specDir)
	mapped := mapAdaptiveLimit(cited)

	overrideValue, hasOverride := readFrontmatterOverride(specDir, stderr)

	base := mapped
	if hasOverride {
		base = overrideValue
		override = "frontmatter"
	}

	applied = base
	appliedCeiling := 0
	if ceiling > 0 && ceiling < base {
		applied = ceiling
		appliedCeiling = ceiling
	}

	emitContextLine(stderr, cited, applied, override, appliedCeiling)
	return cited, applied, override, nil
}

// countSpecCitedFiles counts unique resolved source files referenced by the
// SPEC plan/research docs. Mirrors the resolution logic in CollectContextForSpec.
func countSpecCitedFiles(projectRoot, specDir string) int {
	targets := extractSpecTargetsForCount(specDir)
	if len(targets) == 0 {
		return 0
	}
	moduleRoot := filepath.Dir(filepath.Dir(filepath.Dir(specDir)))
	seen := make(map[string]bool, len(targets))
	count := 0
	for _, target := range targets {
		resolved := spec.ResolveSpecTargetPathForCLI(projectRoot, moduleRoot, target)
		if resolved == "" || seen[resolved] {
			continue
		}
		seen[resolved] = true
		count++
	}
	return count
}

func extractSpecTargetsForCount(specDir string) []string {
	return spec.ExtractSpecContextTargetsForCLI(specDir)
}

// mapAdaptiveLimit applies the SPEC review adaptive mapping without ceiling cap.
func mapAdaptiveLimit(cited int) int {
	// Use the public AdaptiveContextLimit with ceiling=0 to obtain the raw mapped value.
	return spec.AdaptiveContextLimit(cited, 0)
}

// readFrontmatterOverride reads spec.md and parses the optional
// review_context_lines frontmatter override. Invalid values are logged as a
// reject line and treated as absent.
func readFrontmatterOverride(specDir string, stderr io.Writer) (int, bool) {
	data, err := os.ReadFile(filepath.Join(specDir, "spec.md"))
	if err != nil {
		return 0, false
	}
	value, present, parseErr := spec.ParseReviewContextOverride(string(data))
	if !present {
		return 0, false
	}
	if parseErr != nil {
		reason := "invalid integer"
		if errors.Is(parseErr, spec.ErrInvalidContextOverride) {
			reason = "must be >0 and <=10000"
		}
		fmt.Fprintf(stderr, "경고: review_context_lines 무시 (값=%d, 사유=%s)\n", value, reason)
		return 0, false
	}
	return value, true
}

// emitContextLine writes the structured stderr summary line. ceiling is the
// actual ceiling value applied (0 when ceiling did not cap the budget).
func emitContextLine(stderr io.Writer, cited, applied int, override string, ceiling int) {
	line := fmt.Sprintf("SPEC review context: cited=%d applied=%d", cited, applied)
	if override != "" {
		line += " override=" + override
	}
	if ceiling > 0 {
		line += fmt.Sprintf(" ceiling=%d", ceiling)
	}
	fmt.Fprintln(stderr, line)
}
