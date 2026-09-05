package spec

import (
	"strconv"
	"strings"
	"unicode"
)

// RepeatDiscovery records a verify-mode finding that restates an issue the
// review already reported in an earlier revision. FindingID is the id the
// current revision assigned, MatchedPriorID the earlier finding it duplicates.
// It is the machine-readable evidence behind the review receipt's
// repeat-discovery counters: a loop that keeps rediscovering the same issues is
// not converging, it is spinning.
type RepeatDiscovery struct {
	FindingID      string `json:"finding_id"`
	MatchedPriorID string `json:"matched_prior_id"`
	Revision       int    `json:"revision"`
}

// NormalizeFindingKey renders a finding title into the canonical form used to
// recognize restatements: lower-cased, punctuation reduced to separators and
// whitespace collapsed. "Missing REQ-001 coverage." and "missing  req 001
// coverage" therefore share a key.
func NormalizeFindingKey(f ReviewFinding) string {
	return normalizeFindingTitle(f.Description)
}

func normalizeFindingTitle(title string) string {
	var b strings.Builder
	b.Grow(len(title))
	separated := false
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if separated && b.Len() > 0 {
				b.WriteByte(' ')
			}
			separated = false
			b.WriteRune(r)
			continue
		}
		separated = true
	}
	return b.String()
}

// FindingLocationKey returns the "file:line" coordinate a finding points at, or
// "" when the scope reference is a requirement id or carries no line number.
// Two findings at the same coordinate are the same discovery even when the
// reviewers worded them differently; a bare file path is deliberately not a
// location key because unrelated issues share a file.
func FindingLocationKey(f ReviewFinding) string {
	ref := NormalizeScopeRef(f.ScopeRef, "")
	if ref == "" || strings.HasPrefix(ref, "REQ-") {
		return ""
	}
	parts := strings.Split(ref, ":")
	if len(parts) < 2 {
		return ""
	}
	file := strings.TrimSpace(parts[0])
	line := strings.TrimSpace(parts[1])
	if file == "" || line == "" {
		return ""
	}
	if _, err := strconv.Atoi(line); err != nil {
		return ""
	}
	return file + ":" + line
}

// repeatIndex resolves an incoming finding to the prior finding it restates.
// Prior findings of every status participate — a resolved or out_of_scope
// finding being re-reported is exactly the signal this detection exists for.
type repeatIndex struct {
	knownIDs   map[string]struct{}
	byTitle    map[string]ReviewFinding
	byLocation map[string]ReviewFinding
}

func newRepeatIndex(prior []ReviewFinding) *repeatIndex {
	ix := &repeatIndex{
		knownIDs:   make(map[string]struct{}, len(prior)),
		byTitle:    make(map[string]ReviewFinding, len(prior)),
		byLocation: make(map[string]ReviewFinding, len(prior)),
	}
	for _, f := range prior {
		if f.ID != "" {
			ix.knownIDs[f.ID] = struct{}{}
		}
		if key := NormalizeFindingKey(f); key != "" {
			if _, seen := ix.byTitle[key]; !seen {
				ix.byTitle[key] = f
			}
		}
		if key := FindingLocationKey(f); key != "" {
			if _, seen := ix.byLocation[key]; !seen {
				ix.byLocation[key] = f
			}
		}
	}
	return ix
}

func (ix *repeatIndex) match(f ReviewFinding) (ReviewFinding, bool) {
	if f.ID != "" {
		if _, known := ix.knownIDs[f.ID]; known {
			return ReviewFinding{}, false
		}
	}
	if key := NormalizeFindingKey(f); key != "" {
		if prior, ok := ix.byTitle[key]; ok {
			return prior, true
		}
	}
	if key := FindingLocationKey(f); key != "" {
		if prior, ok := ix.byLocation[key]; ok {
			return prior, true
		}
	}
	return ReviewFinding{}, false
}

// walkRepeats reports every incoming finding that restates a prior one and
// hands each match to onMatch with its index, so the query and the classifier
// share one traversal and one matching rule.
func walkRepeats(
	prior, incoming []ReviewFinding,
	revision int,
	onMatch func(index int, matched ReviewFinding),
) []RepeatDiscovery {
	if len(prior) == 0 || len(incoming) == 0 {
		return nil
	}
	ix := newRepeatIndex(prior)
	var repeats []RepeatDiscovery
	for i, f := range incoming {
		matched, ok := ix.match(f)
		if !ok {
			continue
		}
		if onMatch != nil {
			onMatch(i, matched)
		}
		repeats = append(repeats, RepeatDiscovery{
			FindingID: f.ID, MatchedPriorID: matched.ID, Revision: revision,
		})
	}
	return MergeRepeatDiscoveries(repeats)
}

// DetectRepeatDiscoveries reports which incoming verify-mode findings restate a
// prior finding. A finding already tracked by id is a status update, not a
// repeat; anything else matching a prior normalized title or file:line location
// is a rediscovery of known ground.
func DetectRepeatDiscoveries(prior, incoming []ReviewFinding, revision int) []RepeatDiscovery {
	return walkRepeats(prior, incoming, revision, nil)
}

// ApplyRepeatDiscoveries classifies restatements before scope lock runs.
// A repeat is scoped out and tagged with the finding it duplicates, so it never
// re-opens work the review already closed. Critical and security repeats keep
// their escape hatch: restating a resolved critical finding is a regression,
// restating a still-open one stays open.
func ApplyRepeatDiscoveries(incoming, prior []ReviewFinding, revision int) ([]ReviewFinding, []RepeatDiscovery) {
	if len(prior) == 0 || len(incoming) == 0 {
		return incoming, nil
	}
	result := make([]ReviewFinding, len(incoming))
	copy(result, incoming)
	repeats := walkRepeats(prior, incoming, revision, func(index int, matched ReviewFinding) {
		result[index].RepeatOf = matched.ID
		result[index].Status = repeatFindingStatus(result[index], matched)
		if isEscapeHatchFinding(result[index]) {
			result[index].EscapeHatch = true
		}
	})
	return result, repeats
}

func repeatFindingStatus(incoming, matched ReviewFinding) FindingStatus {
	if !isEscapeHatchFinding(incoming) {
		return FindingStatusOutOfScope
	}
	if matched.Status == FindingStatusResolved {
		return FindingStatusRegressed
	}
	return FindingStatusOpen
}

func isEscapeHatchFinding(f ReviewFinding) bool {
	return strings.EqualFold(strings.TrimSpace(f.Severity), "critical") ||
		f.Category == FindingCategorySecurity
}

// MergeRepeatDiscoveries concatenates repeat groups, dropping entries that
// duplicate an earlier one. Providers restating the same issue must not inflate
// the repeat count; the first occurrence wins and input order is preserved.
func MergeRepeatDiscoveries(groups ...[]RepeatDiscovery) []RepeatDiscovery {
	type key struct {
		finding  string
		prior    string
		revision int
	}
	seen := map[key]struct{}{}
	var merged []RepeatDiscovery
	for _, group := range groups {
		for _, r := range group {
			k := key{finding: r.FindingID, prior: r.MatchedPriorID, revision: r.Revision}
			if _, dup := seen[k]; dup {
				continue
			}
			seen[k] = struct{}{}
			merged = append(merged, r)
		}
	}
	return merged
}
