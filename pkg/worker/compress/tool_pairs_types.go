package compress

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	toolPairIDPattern  = regexp.MustCompile(`"pair_id"\s*:\s*"([^"]+)"`)
	toolOrdinalPattern = regexp.MustCompile(`"ordinal"\s*:\s*([0-9]+)`)
)

type toolBlock struct {
	kind    string
	start   int
	end     int
	text    string
	pairID  string
	ordinal string
	key     string
}

type toolPair struct {
	key     string
	pairID  string
	ordinal string
	call    *toolBlock
	result  *toolBlock
	start   int
	end     int
}

type replacement struct {
	start, end int
	text       string
}

func findToolBlocks(text string) []toolBlock {
	var blocks []toolBlock
	for _, match := range toolCallPattern.FindAllStringIndex(text, -1) {
		blocks = append(blocks, newToolBlock("call", text, match))
	}
	for _, match := range toolResultPattern.FindAllStringIndex(text, -1) {
		blocks = append(blocks, newToolBlock("result", text, match))
	}
	for _, match := range findJSONToolBlockIndices(text) {
		blocks = append(blocks, newToolBlock(match.kind, text, []int{match.start, match.end}))
	}
	blocks = dedupeToolBlocks(blocks)
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].start < blocks[j].start
	})
	assignToolKeys(blocks)
	return blocks
}

type jsonToolBlockIndex struct {
	kind  string
	start int
	end   int
}

func findJSONToolBlockIndices(text string) []jsonToolBlockIndex {
	var matches []jsonToolBlockIndex
	for offset := 0; offset < len(text); offset++ {
		if text[offset] != '{' {
			continue
		}
		var payload map[string]any
		dec := json.NewDecoder(bytes.NewReader([]byte(text[offset:])))
		if err := dec.Decode(&payload); err != nil {
			continue
		}
		kind, ok := payload["type"].(string)
		if !ok {
			continue
		}
		if kind != "tool_call" && kind != "tool_result" {
			continue
		}
		matches = append(matches, jsonToolBlockIndex{
			kind:  strings.TrimPrefix(kind, "tool_"),
			start: offset,
			end:   offset + int(dec.InputOffset()),
		})
	}
	return matches
}

func dedupeToolBlocks(blocks []toolBlock) []toolBlock {
	seen := map[string]bool{}
	var out []toolBlock
	for _, block := range blocks {
		key := fmt.Sprintf("%d:%d:%s", block.start, block.end, block.kind)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, block)
	}
	return out
}

func newToolBlock(kind, text string, match []int) toolBlock {
	blockText := text[match[0]:match[1]]
	pairID := firstSubmatch(toolPairIDPattern, blockText)
	ordinal := firstSubmatch(toolOrdinalPattern, blockText)
	key := ""
	if pairID != "" {
		key = "pair:" + pairID
	} else if ordinal != "" {
		key = "ordinal:" + ordinal
	}
	return toolBlock{
		kind:    kind,
		start:   match[0],
		end:     match[1],
		text:    blockText,
		pairID:  pairID,
		ordinal: ordinal,
		key:     key,
	}
}

func firstSubmatch(pattern *regexp.Regexp, text string) string {
	match := pattern.FindStringSubmatch(text)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

// @AX:NOTE: [AUTO] @AX:SPEC: SPEC-CONTEXT-COMPRESS-001: fallback sequence keys pair unlabelled calls with following results; do not prune them independently
func assignToolKeys(blocks []toolBlock) {
	pendingCall := -1
	sequence := 1
	for i := range blocks {
		if blocks[i].kind == "call" {
			if blocks[i].key == "" {
				blocks[i].key = fmt.Sprintf("sequence:%d", sequence)
				sequence++
			}
			pendingCall = i
			continue
		}
		if blocks[i].key == "" && pendingCall >= 0 {
			blocks[i].key = blocks[pendingCall].key
			pendingCall = -1
			continue
		}
		if blocks[i].key == "" {
			blocks[i].key = fmt.Sprintf("standalone:%d", sequence)
			sequence++
		}
	}
}

func hasToolKinds(blocks []toolBlock) (bool, bool) {
	hasCalls := false
	hasResults := false
	for _, block := range blocks {
		hasCalls = hasCalls || block.kind == "call"
		hasResults = hasResults || block.kind == "result"
	}
	return hasCalls, hasResults
}

func successfulToolPair(pair *toolPair) bool {
	if pair == nil || pair.call == nil || pair.result == nil {
		return false
	}
	start := strings.IndexByte(pair.result.text, '{')
	end := strings.LastIndexByte(pair.result.text, '}')
	if start < 0 || end <= start {
		return false
	}
	var payload struct {
		Status  string `json:"status"`
		Success *bool  `json:"success"`
		IsError *bool  `json:"is_error"`
	}
	if err := json.Unmarshal([]byte(pair.result.text[start:end+1]), &payload); err != nil {
		return false
	}
	if payload.IsError != nil && *payload.IsError {
		return false
	}
	if payload.Success != nil {
		return *payload.Success
	}
	switch strings.ToLower(strings.TrimSpace(payload.Status)) {
	case "success", "succeeded", "completed":
		return true
	default:
		return false
	}
}
