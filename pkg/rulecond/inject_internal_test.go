package rulecond

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestAssembleContext_NeverExceedsAggregateCap is the white-box H2 oracle.
//
// The manifest loader now refuses a rule name outside ruleNamePattern, so the
// reported proof-of-concept shape cannot reach this function through Fire any
// more. This test drives assembleContext directly so the cap stays enforced at
// the point of assembly, independently of that upstream check.
func TestAssembleContext_NeverExceedsAggregateCap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []injected
	}{
		{
			// The reported PoC: one rule whose name alone dwarfs the cap. The
			// drop loop reduces kept to 0 and then stops without re-testing,
			// so before the fix the notice was emitted whole at 20037 bytes.
			name: "single huge name",
			entries: []injected{
				{name: strings.Repeat("A", 20000), body: "# body\n"},
			},
		},
		{
			name: "huge name alongside a legitimate rule",
			entries: []injected{
				{name: "lore-commit", body: strings.Repeat("x", 3000)},
				{name: strings.Repeat("B", 20000), body: strings.Repeat("y", 3000)},
			},
		},
		{
			// Every rule dropped means the notice is the entire output, which
			// is the other way the pre-fix loop escaped the cap.
			name: "many huge names all dropped",
			entries: []injected{
				{name: strings.Repeat("C", 5000), body: strings.Repeat("x", 3900)},
				{name: strings.Repeat("D", 5000), body: strings.Repeat("y", 3900)},
				{name: strings.Repeat("E", 5000), body: strings.Repeat("z", 3900)},
			},
		},
		{
			name: "multibyte name truncated on a rune boundary",
			entries: []injected{
				{name: strings.Repeat("한", 9000), body: "# body\n"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := assembleContext(tt.entries)
			if len(got) > MaxContextBytes {
				t.Fatalf("assembleContext emitted %d bytes, above the %d byte cap",
					len(got), MaxContextBytes)
			}
			if !utf8.ValidString(got) {
				t.Fatal("truncation split a UTF-8 rune")
			}
		})
	}
}

// TestTruncationNotice_IsBounded pins the notice bound on its own, since the
// notice is built from untrusted rule names.
func TestTruncationNotice_IsBounded(t *testing.T) {
	t.Parallel()

	notice := truncationNotice([]string{strings.Repeat("A", 20000), strings.Repeat("B", 20000)})
	if len(notice) > maxNoticeBytes {
		t.Fatalf("notice is %d bytes, above the %d byte bound", len(notice), maxNoticeBytes)
	}
	if !strings.HasPrefix(notice, TruncationPrefix) {
		t.Fatal("notice must keep its prefix so it stays recognizable as one line")
	}
	if strings.Contains(notice, "\n") {
		t.Fatal("the notice must stay a single line")
	}
}

// TestSanitizeName_ProducesOneSafeStderrToken covers the H1 diagnostic path: a
// hostile name must not be able to forge extra "<rule> <reason>" lines.
func TestSanitizeName_ProducesOneSafeStderrToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "newline injection", in: "evil\nlore-commit absolute_path", want: "evil?lore-commit?absolute_path"},
		{name: "space injection", in: "a b", want: "a?b"},
		{name: "path separator", in: "../../etc/passwd", want: "..?..?etc?passwd"},
		{name: "empty", in: "", want: "?"},
		{name: "conforming", in: "lore-commit", want: "lore-commit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sanitizeName(tt.in)
			if got != tt.want {
				t.Fatalf("sanitizeName(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if strings.ContainsAny(got, "\n\r ") {
				t.Fatalf("sanitized name %q still carries a line or field separator", got)
			}
		})
	}
}

// TestSanitizeName_IsBounded keeps a huge name from flooding stderr.
func TestSanitizeName_IsBounded(t *testing.T) {
	t.Parallel()

	if got := sanitizeName(strings.Repeat("A", 20000)); len(got) != 64 {
		t.Fatalf("sanitizeName truncated to %d bytes, want 64", len(got))
	}
}
