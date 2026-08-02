package rulecond

import (
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
)

// stickyPayload is the subset of the UserPromptSubmit payload the runtime reads.
//
// Only session_id is decoded. The payload also carries `prompt`,
// `transcript_path`, and `cwd`, and REQ-STICKYRULE-FIRE-04 forbids emitting,
// logging, or persisting any of them, so none of them has a field here to be
// accidentally threaded into output.
//
// @AX:WARN [AUTO] @AX:SPEC: SPEC-STICKYRULE-001: the redaction boundary is this struct's field list, not a downstream filter.
// @AX:REASON: REQ-STICKYRULE-FIRE-04 holds because `prompt`, `transcript_path`, and `cwd` have nowhere to be decoded into; adding a field for any of them makes the leak reachable from stderr and from the injected context in one edit.
type stickyPayload struct {
	SessionID string `json:"session_id"`
}

// decodeStickyPayload reads a bounded stdin payload and decodes it as JSON. An
// unreadable or unparseable payload is whole-run benign.
func decodeStickyPayload(stdin io.Reader) (stickyPayload, error) {
	var payload stickyPayload
	if stdin == nil {
		return payload, errors.New("no stdin")
	}
	data, err := io.ReadAll(io.LimitReader(stdin, maxStdinBytes))
	if err != nil {
		return payload, err
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return payload, err
	}
	return payload, nil
}

// readStickyBodies reads each sticky rule body under the sticky body root,
// separating the two per-rule cases of REQ-STICKYRULE-FIRE-08: a body that is
// simply gone is skipped silently, while a containment or size violation is
// reported as the rule name and the reason code and nothing else.
//
// The root is resolved once. A relocated sticky root compromises the frame every
// rule is measured against, so each rule is suppressed and named rather than
// measured against a frame an attacker moved.
func readStickyBodies(root string, rules []StickyRule, stderr io.Writer) []injected {
	bodyRoot, err := ResolveStickyBodyRoot(root)
	if err != nil {
		for _, rule := range rules {
			reportViolation(stderr, rule.Name, err)
		}
		return nil
	}

	entries := make([]injected, 0, len(rules))
	for _, rule := range rules {
		raw, readErr := ReadBody(bodyRoot, rule.Body)
		if readErr != nil {
			reportViolation(stderr, rule.Name, readErr)
			continue
		}
		body := injectableBodyOf(raw)
		// REQ-STICKYRULE-FIRE-07: the per-body cap is applied here, before the
		// aggregate assembly, so an oversize body is refused outright instead of
		// counted toward the aggregate and then dropped without a diagnostic.
		if len(body) > MaxStickyBodyBytes {
			reportViolation(stderr, rule.Name, violation(ReasonBodyTooLarge))
			continue
		}
		if strings.TrimSpace(body) == "" {
			continue
		}
		entries = append(entries, injected{name: rule.Name, body: body})
	}
	return entries
}

// injectableBodyOf strips the YAML frontmatter from a rule file, which is the
// only form this SPEC injects and the only form its byte accounting counts.
//
// The computation is deliberately identical to the one ParseRule performs, so
// the bytes the runtime injects are the same bytes the compiler measured.
func injectableBodyOf(raw []byte) string {
	_, body := splitFrontmatter(string(raw))
	return strings.TrimLeft(body, "\n")
}

// assembleStickyContext renders the injected context under the sticky aggregate
// cap: rule names and injectable bodies only, never a byte of the prompt, the
// session identifier, the transcript path, or the working directory
// (REQ-STICKYRULE-FIRE-04). Rules dropped by the cap are named on one bounded
// notice that counts against the cap (REQ-STICKYRULE-FIRE-10).
func assembleStickyContext(entries []injected) string {
	if stickyPanicHook != nil {
		stickyPanicHook()
	}
	return assembleWithCap(sortInjectedByName(entries), MaxStickyContextBytes)
}

// sortInjectedByName orders entries by rule name. The order is load-bearing
// rather than cosmetic: assembleWithCap drops from the tail, so name ordering is
// what makes "drop whole rules in reverse rule-name order" hold for a manifest
// whose own array order is untrusted.
//
// @AX:NOTE [AUTO] @AX:SPEC: SPEC-STICKYRULE-001: the sort is load-bearing, not cosmetic — compileSticky measures the same name-ordered blocks, so removing it makes the compile-time aggregate check disagree with what the runtime actually drops.
func sortInjectedByName(entries []injected) []injected {
	sorted := make([]injected, len(entries))
	copy(sorted, entries)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].name < sorted[j].name })
	return sorted
}
