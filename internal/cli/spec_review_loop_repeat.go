package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/spec"
)

// specReviewRepeatTracker carries the cross-revision state the review loop needs
// to tell convergence from spinning: whether this revision re-reviews unchanged
// input, and which findings restate ground the review already covered.
type specReviewRepeatTracker struct {
	specDir      string
	prevSnapshot string
	sameInput    bool
}

// beginRevision snapshots the SPEC inputs and records whether they are
// byte-identical to the previous revision's. An unreadable spec dir yields an
// empty snapshot, which never claims sameness.
func (t *specReviewRepeatTracker) beginRevision() {
	snapshot := specReviewInputSnapshot(t.specDir)
	t.sameInput = snapshot != "" && snapshot == t.prevSnapshot
	t.prevSnapshot = snapshot
}

// apply classifies verify-mode restatements on the merged result and folds in
// the repeats the verify merge already absorbed, so a duplicate dropped during
// merge is still counted instead of vanishing.
func (t *specReviewRepeatTracker) apply(
	merged *spec.ReviewResult,
	priorFindings []spec.ReviewFinding,
	revision int,
	mergeRepeats []spec.RepeatDiscovery,
) {
	merged.SameInputReReview = t.sameInput
	if len(priorFindings) == 0 {
		merged.RepeatDiscoveries = spec.MergeRepeatDiscoveries(mergeRepeats)
		return
	}
	classified, repeats := spec.ApplyRepeatDiscoveries(merged.Findings, priorFindings, revision)
	merged.Findings = classified
	merged.RepeatDiscoveries = spec.MergeRepeatDiscoveries(mergeRepeats, repeats)
}

// specReviewInputSnapshot hashes the SPEC documents the reviewers are shown so
// the loop can distinguish a genuine revision from a re-review of identical
// input. review.md is excluded because the loop rewrites it every revision,
// which would make every snapshot differ.
func specReviewInputSnapshot(specDir string) string {
	entries, err := os.ReadDir(specDir)
	if err != nil {
		return ""
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || entry.Name() == "review.md" {
			continue
		}
		names = append(names, entry.Name())
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)

	digest := sha256.New()
	for _, name := range names {
		data, readErr := os.ReadFile(filepath.Join(specDir, name))
		if readErr != nil {
			return ""
		}
		fmt.Fprintf(digest, "%s\x00%x\n", name, sha256.Sum256(data))
	}
	return hex.EncodeToString(digest.Sum(nil))
}
