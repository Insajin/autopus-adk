package compress

import (
	"fmt"
	"sort"
)

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
