package orchestra

import "testing"

// TestSanitizeScreenOutput verifies screen output sanitization for ANSI codes,
// OSC sequences, status bars, markdown preservation, and blank line collapsing.
func TestSanitizeScreenOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// S4: ANSI color codes
		{"strips ANSI color codes", "\x1b[31mError\x1b[0m", "Error"},
		// S5: OSC sequences
		{"strips OSC window title", "\x1b]0;window title\x07real content", "real content"},
		// S6: status bar lines
		{"strips tmux status bar", "content\n[0] 0:bash* 1:vim\nmore content", "content\nmore content"},
		// S7: preserves markdown quotes
		{"preserves markdown quotes", "> This is a quote\n> Another line", "> This is a quote\n> Another line"},
		// S8: collapses blank lines
		{"collapses consecutive blank lines", "line1\n\n\n\nline2", "line1\n\nline2"},
		// Edge cases
		{"empty input returns empty", "", ""},
		{"plain text unchanged", "hello world", "hello world"},
		// Extended ANSI: CSI cursor movement
		{"strips CSI cursor movement", "\x1b[2J\x1b[H content here", " content here"},
		// DCS sequences
		{"strips DCS sequences", "\x1bPtest\x1b\\content", "content"},
		// Composite: ANSI + OSC + status bar in single input
		{"composite ANSI OSC and status bar", "\x1b[31mheader\x1b[0m\n\x1b]0;title\x07body\n[0] 0:bash* 1:vim\nfooter", "header\nbody\nfooter"},
		// Multi-line status bars
		{"multiple status bar lines", "top\n[0] 0:bash*\nmiddle\n[1] 1:vim*\nbottom", "top\nmiddle\nbottom"},
		// Long input with mixed escapes
		{"long input with trailing whitespace", "line1   \n\x1b[32mline2\x1b[0m  \n\n\n\nline3", "line1\nline2\n\nline3"},
		// Cursor save/restore sequences
		{"strips cursor save restore", "\x1b7saved\x1b8restored", "savedrestored"},
		// OSC terminated by ST
		{"strips OSC with ST terminator", "\x1b]52;c;data\x1b\\visible", "visible"},
		// CLI banner stripping
		{"strips Claude banner", "▐▛███▜▌   Claude Code v2.1.87\ncontent here", "content here"},
		{"strips Gemini banner", "▝▜▄     Gemini CLI v0.35.3     ▝▜▄\ncontent here", "content here"},
		{"strips multi-line banners", "▐▛███▜▌   Claude Code\n▝▜█████▛▘  Opus 4.6\ncontent", "content"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SanitizeScreenOutput(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeScreenOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}
