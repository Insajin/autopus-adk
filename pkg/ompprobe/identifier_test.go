package ompprobe

import (
	"strings"
	"testing"
)

func TestValidIdentifier_MatchesTheDocumentedRangeExactly(t *testing.T) {
	accepted := []string{
		"openaiResponsesHistory", "compactionSummary", "toolCall", "toolResult",
		"inputTokens", "outputTokens", "totalTokens", "_private", "a", "A1_9",
		"a" + strings.Repeat("b", 63),
	}
	for _, name := range accepted {
		if !ValidIdentifier(name) {
			t.Fatalf("identifier inside %s rejected: %q", IdentifierPattern, name)
		}
	}
	rejected := []string{
		"", "9lives", "-lead", "role.name", "Opaque-Item!", "with space",
		"réle", "tab\tname", "new\nline", "a" + strings.Repeat("b", 64),
	}
	for _, name := range rejected {
		if ValidIdentifier(name) {
			t.Fatalf("identifier outside %s accepted: %q", IdentifierPattern, name)
		}
	}
}

func TestHistogram_PreservesIdentifierCaseAndNeverAliasesRejections(t *testing.T) {
	var histogram Histogram
	preserved := []string{
		"openaiResponsesHistory", "openairesponseshistory", "compactionSummary",
		"toolCall", "toolResult", "toolCall",
	}
	for _, name := range preserved {
		if !histogram.Observe(name) {
			t.Fatalf("preservable identifier rejected: %q", name)
		}
	}
	dropped := []string{"Opaque-Item!", "", "9lives", "content type", "unprintable\x01"}
	for _, name := range dropped {
		if histogram.Observe(name) {
			t.Fatalf("out-of-range identifier preserved: %q", name)
		}
	}
	counts := histogram.Counts()
	if counts["openaiResponsesHistory"] != 1 || counts["openairesponseshistory"] != 1 {
		t.Fatal("case-differing identifiers must stay distinct allowlist candidates")
	}
	if counts["toolCall"] != 2 {
		t.Fatalf("repeated identifier counted %d times", counts["toolCall"])
	}
	if len(counts) != 5 {
		t.Fatalf("histogram holds %d keys, want only the preserved identifiers", len(counts))
	}
	for name := range counts {
		if !ValidIdentifier(name) {
			t.Fatalf("histogram key outside the preserved range: %q", name)
		}
	}
	if histogram.Rejected() != len(dropped) {
		t.Fatalf("rejected count is %d, want %d", histogram.Rejected(), len(dropped))
	}
}

func TestHistogram_BoundsKeyCardinalityWithoutAliasing(t *testing.T) {
	var histogram Histogram
	for index := range maxHistogramKeys {
		if !histogram.Observe("key_" + string(rune('a'+index%26)) + string(rune('a'+index/26))) {
			t.Fatalf("identifier %d rejected below the key bound", index)
		}
	}
	if histogram.Observe("overflowKey") {
		t.Fatal("identifier accepted past the key bound")
	}
	if histogram.Rejected() != 1 || len(histogram.Counts()) != maxHistogramKeys {
		t.Fatalf("bound overflow changed the histogram: keys=%d rejected=%d",
			len(histogram.Counts()), histogram.Rejected())
	}
}

func TestIdentifierSet_DeduplicatesInFirstSeenOrderAndCountsRejections(t *testing.T) {
	var set IdentifierSet
	for _, name := range []string{"notify", "requestInput", "notify", "Not-A-Method", ""} {
		set.Observe(name)
	}
	names := set.Names()
	if len(names) != 2 || names[0] != "notify" || names[1] != "requestInput" {
		t.Fatalf("identifier order is %v, want [notify requestInput]", names)
	}
	if set.Rejected() != 2 {
		t.Fatalf("rejected count is %d, want 2", set.Rejected())
	}
	var empty IdentifierSet
	if empty.Names() != nil {
		t.Fatal("empty set must serialize as an omitted field")
	}
}

func TestValidReasonToken_AcceptsOnlyBodyFreeLowercaseTokens(t *testing.T) {
	for _, reason := range []string{"probe_interrupted", "handshake_failed", "a", "x9_z"} {
		if !validReasonToken(reason) {
			t.Fatalf("body-free reason rejected: %q", reason)
		}
	}
	for _, reason := range []string{
		"", "Probe_Interrupted", "9lives", "_lead", "reason with words",
		"reason-dash", "compact failed: model said no",
	} {
		if validReasonToken(reason) {
			t.Fatalf("unsafe reason accepted: %q", reason)
		}
	}
}
