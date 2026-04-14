package content

import "testing"

func TestEmbeddedPath_NormalizesWindowsSeparators(t *testing.T) {
	t.Parallel()

	got := EmbeddedPath(`rules\branding.md`)
	if got != "rules/branding.md" {
		t.Fatalf("EmbeddedPath should normalize Windows separators: got %q", got)
	}
}

func TestEmbeddedPath_JoinsSegmentsWithSlash(t *testing.T) {
	t.Parallel()

	got := EmbeddedPath("rules", "branding.md")
	if got != "rules/branding.md" {
		t.Fatalf("EmbeddedPath should join with slash: got %q", got)
	}
}
