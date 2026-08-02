package rulecond

import (
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"
)

// blockSeparator joins injected rule blocks and the truncation notice.
const blockSeparator = "\n\n"

// hookOutput is the advisory dispatcher response. It carries rule text only:
// no permissionDecision and no continue field (REQ-CONDRULE-FIRE-06).
type hookOutput struct {
	HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
}

type hookSpecificOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext"`
}

// injected is one contained rule body ready for injection.
type injected struct {
	name string
	body string
}

// maxNoticeBytes bounds the truncation notice. The notice is built from rule
// names, which are untrusted manifest input, so without a bound the notice
// itself can carry the injected context past MaxContextBytes.
const maxNoticeBytes = 512

// assembleContext renders the injected context: rule names and rule bodies
// only, never the matched command text or any other tool input
// (REQ-CONDRULE-FIRE-04). Once the aggregate cap is reached, whole rules are
// dropped in reverse rule-name order and named in one truncation notice
// (REQ-CONDRULE-FIRE-05).
//
// The final string is truncated to MaxContextBytes unconditionally. The drop
// loop alone is not sufficient: it stops at kept == 0 without re-testing the
// cap, so a single block larger than the cap, or a notice large enough on its
// own, would otherwise be emitted whole.
//
// @AX:NOTE [AUTO] @AX:SPEC: SPEC-STICKYRULE-001: this function now only binds MaxContextBytes; the aggregate cap itself moved to assembleWithCap, which is where the ANCHOR for REQ-CONDRULE-FIRE-05 lives.
func assembleContext(entries []injected) string {
	return assembleWithCap(entries, MaxContextBytes)
}

// assembleWithCap is assembleContext with the aggregate cap as a parameter, so
// the SPEC-CONDRULE-001 dispatcher and the SPEC-STICKYRULE-001 sticky runtime
// share one drop-and-truncate implementation under their own caps. A second copy
// of this loop is what would let one cap keep the closing truncateBytes call and
// the other lose it.
//
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-CONDRULE-001, SPEC-STICKYRULE-001: REQ-CONDRULE-FIRE-05 aggregate cap — this is the only place the injected context length is bounded, now for both the conditional dispatcher and the sticky runtime.
// @AX:REASON: Removing the closing truncateBytes call restores an unbounded injection whenever the drop loop cannot get the total under the cap, and it would do so on both injection paths at once.
func assembleWithCap(entries []injected, maxContext int) string {
	blocks := injectionBlocks(entries)

	kept := len(blocks)
	var dropped []string
	for kept > 0 {
		total := joinedLen(blocks[:kept])
		if len(dropped) > 0 {
			total += len(blockSeparator) + len(truncationNotice(dropped))
		}
		if total <= maxContext {
			break
		}
		kept--
		dropped = append([]string{entries[kept].name}, dropped...)
	}

	parts := make([]string, 0, kept+1)
	parts = append(parts, blocks[:kept]...)
	if len(dropped) > 0 {
		parts = append(parts, truncationNotice(dropped))
	}
	return truncateBytes(strings.Join(parts, blockSeparator), maxContext)
}

// injectionBlocks renders one header-and-body block per entry. The compile-time
// aggregate check measures exactly these blocks, so the compiler refuses the
// same set the runtime would have dropped from.
func injectionBlocks(entries []injected) []string {
	blocks := make([]string, len(entries))
	for i, entry := range entries {
		blocks[i] = RuleHeaderPrefix + entry.name + "\n" + entry.body
	}
	return blocks
}

// truncationNotice names the dropped rules on one line and carries no tool
// input. The line is bounded by maxNoticeBytes.
func truncationNotice(dropped []string) string {
	return truncateBytes(TruncationPrefix+strings.Join(dropped, ", "), maxNoticeBytes)
}

// truncateBytes caps a string at max bytes, backing off to the last complete
// UTF-8 rune so a cut never emits a partial code point.
func truncateBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// joinedLen is the byte length of blocks joined by blockSeparator.
func joinedLen(blocks []string) int {
	if len(blocks) == 0 {
		return 0
	}
	total := len(blockSeparator) * (len(blocks) - 1)
	for _, block := range blocks {
		total += len(block)
	}
	return total
}

// writeOutput emits the advisory hook payload.
func writeOutput(stdout io.Writer, event, context string) error {
	if context == "" || stdout == nil {
		return nil
	}
	blob, err := json.Marshal(hookOutput{
		HookSpecificOutput: hookSpecificOutput{HookEventName: event, AdditionalContext: context},
	})
	if err != nil {
		return nil
	}
	_, err = stdout.Write(append(blob, '\n'))
	return err
}
