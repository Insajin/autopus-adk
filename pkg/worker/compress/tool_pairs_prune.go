package compress

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

// @AX:NOTE: [AUTO] @AX:SPEC: SPEC-ADK-ULTRA-EFFICIENCY-001 — bounded excerpts retain evidence while preventing pruned tool bodies from regrowing context.
const softPruneExcerptLimit = 160

func softPruneToolPairs(text string, blocks []toolBlock, keepRecent int) pruneDetails {
	pairs := collectToolPairs(blocks)
	eligible := make([]*toolPair, 0, len(pairs))
	for _, pair := range pairs {
		if successfulToolPair(pair) {
			eligible = append(eligible, pair)
		}
	}
	incomplete := 0
	for _, pair := range pairs {
		if pair.call == nil || pair.result == nil {
			incomplete++
		}
	}
	if incomplete > 0 {
		return pruneDetails{
			Text:                text,
			IncompletePairCount: incomplete,
			ReasonCodes:         []string{ReasonIncompleteToolPair},
		}
	}
	sort.Slice(eligible, func(i, j int) bool { return eligible[i].end < eligible[j].end })
	if len(eligible) <= keepRecent {
		return pruneDetails{Text: text}
	}

	artifactDigest := sha256.Sum256([]byte(text))
	toPrune := eligible[:len(eligible)-keepRecent]
	replacements := make([]replacement, 0, len(toPrune))
	for _, pair := range toPrune {
		replacements = append(replacements, replacement{
			start: pair.start,
			end:   pair.end,
			text:  softPrunedPairRecord(pair, artifactDigest),
		})
	}
	return pruneDetails{
		Text:            applyReplacements(text, replacements),
		PrunedPairCount: len(toPrune),
		ReasonCodes:     []string{ReasonToolPairPruned},
	}
}

func softPrunedPairRecord(pair *toolPair, artifactDigest [sha256.Size]byte) string {
	pairText := pair.call.text + "\n" + pair.result.text
	pairDigest := sha256.Sum256([]byte(pairText))
	safeText, _ := redactUnsafeContext(pairText)
	refs := extractSourceRefs(safeText)
	if len(refs) == 0 {
		refs = []string{"none"}
	}
	excerptText, _ := omitToolPayloadBodies(safeText)
	return fmt.Sprintf(
		`[tool_pair pruned: status=success pair=%s ordinal=%s digest=sha256:%x artifact_ref=phase-output:sha256:%x source_refs=%s excerpt="%s"]`,
		safeEvidenceRef(pairRef(pair)), ordinalRef(pair), pairDigest, artifactDigest, strings.Join(refs, ","), boundedEvidenceExcerpt(excerptText),
	)
}

func safeEvidenceRef(value string) string {
	redacted, reasons := redactUnsafeContext(value)
	if len(reasons) == 0 && redacted == value && isSimpleEvidenceRef(value) {
		return value
	}
	digest := sha256.Sum256([]byte(value))
	return fmt.Sprintf("redacted-ref-%x", digest[:6])
}

func isSimpleEvidenceRef(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || strings.ContainsRune("._:-", r) {
			continue
		}
		return false
	}
	return true
}

func boundedEvidenceExcerpt(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	text = strings.ReplaceAll(text, `"`, `'`)
	if len(text) > softPruneExcerptLimit {
		text = text[:softPruneExcerptLimit]
	}
	return text
}

func pruneToolPairs(text string, blocks []toolBlock, keepRecent int) pruneDetails {
	pairs := collectToolPairs(blocks)
	completePairs := completeToolPairs(pairs)
	sort.Slice(completePairs, func(i, j int) bool {
		return completePairs[i].end < completePairs[j].end
	})

	kept := map[string]bool{}
	for i := max(0, len(completePairs)-keepRecent); i < len(completePairs); i++ {
		kept[completePairs[i].key] = true
	}

	var replacements []replacement
	details := pruneDetails{Text: text}
	for _, pair := range pairs {
		if pair.call != nil && pair.result != nil {
			if !kept[pair.key] {
				replacements = append(replacements, replacement{
					start: pair.start,
					end:   pair.end,
					text:  prunedPairPlaceholder(pair),
				})
				details.PrunedPairCount++
			} else {
				replacements = append(replacements,
					replacement{start: pair.call.start, end: pair.call.end, text: preservedToolBlock(pair.call)},
					replacement{start: pair.result.start, end: pair.result.end, text: preservedToolBlock(pair.result)},
				)
			}
			continue
		}
		replacements = append(replacements, replacement{
			start: pair.start,
			end:   pair.end,
			text:  incompletePairPlaceholder(pair),
		})
		details.IncompletePairCount++
	}
	details.Text = applyReplacements(text, replacements)
	if details.PrunedPairCount > 0 {
		details.ReasonCodes = append(details.ReasonCodes, ReasonToolPairPruned)
	}
	if details.IncompletePairCount > 0 {
		details.ReasonCodes = append(details.ReasonCodes, ReasonIncompleteToolPair)
	}
	return details
}

func collectToolPairs(blocks []toolBlock) map[string]*toolPair {
	pairs := map[string]*toolPair{}
	for i := range blocks {
		block := &blocks[i]
		pair := pairs[block.key]
		if pair == nil {
			pair = &toolPair{key: block.key, start: block.start, end: block.end}
			pairs[block.key] = pair
		}
		pair.start = min(pair.start, block.start)
		pair.end = max(pair.end, block.end)
		if block.pairID != "" {
			pair.pairID = block.pairID
		}
		if block.ordinal != "" {
			pair.ordinal = block.ordinal
		}
		if block.kind == "call" {
			pair.call = block
		} else {
			pair.result = block
		}
	}
	return pairs
}

func completeToolPairs(pairs map[string]*toolPair) []*toolPair {
	var complete []*toolPair
	for _, pair := range pairs {
		if pair.call != nil && pair.result != nil {
			complete = append(complete, pair)
		}
	}
	return complete
}

func prunedPairPlaceholder(pair *toolPair) string {
	return fmt.Sprintf("[tool_pair pruned: pair=%s ordinal=%s]", pairRef(pair), ordinalRef(pair))
}

func incompletePairPlaceholder(pair *toolPair) string {
	reason := "missing_result"
	if pair.call == nil {
		reason = "missing_call"
	}
	return fmt.Sprintf("[tool_pair incomplete: pair=%s ordinal=%s reason=%s]", pairRef(pair), ordinalRef(pair), reason)
}

func preservedToolBlock(block *toolBlock) string {
	return fmt.Sprintf("<tool_%s>{\"pair_id\":\"%s\",\"ordinal\":\"%s\",\"body\":\"[omitted]\",\"reason\":\"provider_payload_omitted\"}</tool_%s>",
		block.kind,
		pairRefFromBlock(*block),
		ordinalRefFromBlock(*block),
		block.kind,
	)
}

func pairRef(pair *toolPair) string {
	if pair.pairID != "" {
		return pair.pairID
	}
	return pair.key
}

func ordinalRef(pair *toolPair) string {
	if pair.ordinal != "" {
		return pair.ordinal
	}
	return "unknown"
}

func applyReplacements(text string, replacements []replacement) string {
	sort.Slice(replacements, func(i, j int) bool {
		return replacements[i].start > replacements[j].start
	})
	result := text
	for _, repl := range replacements {
		result = result[:repl.start] + repl.text + result[repl.end:]
	}
	return result
}
