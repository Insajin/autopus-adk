package orchestra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- R4: DefaultCompletionPatterns provider-specific matching ---

// TestDefaultCompletionPatterns_ProviderSpecificMatching verifies each provider's actual
// TUI output matches its pattern in DefaultCompletionPatterns and does NOT match other providers.
func TestDefaultCompletionPatterns_ProviderSpecificMatching(t *testing.T) {
	t.Parallel()

	patterns := DefaultCompletionPatterns()

	tests := []struct {
		name             string
		screen           string
		expectedProvider string
	}{
		{
			name:             "claude: unicode heavy right-pointing angle",
			screen:           "some output\n❯ ",
			expectedProvider: "claude",
		},
		{
			name:             "claude: bare prompt on own line",
			screen:           "❯",
			expectedProvider: "claude",
		},
		{
			name:             "gemini: Type your message prompt",
			screen:           "> Type your message...",
			expectedProvider: "gemini",
		},
		{
			name:             "gemini: @ mention prompt",
			screen:           "  > @user",
			expectedProvider: "gemini",
		},
		{
			name:             "codex: codex> prompt",
			screen:           "codex> ",
			expectedProvider: "codex",
		},
		{
			name:             "opencode: Ask anything prompt",
			screen:           "Ask anything",
			expectedProvider: "opencode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Verify the expected provider's pattern matches
			matched := false
			for _, cp := range patterns {
				if cp.Provider == tt.expectedProvider && cp.Pattern.MatchString(tt.screen) {
					matched = true
					break
				}
			}
			assert.True(t, matched, "pattern for %s must match screen %q", tt.expectedProvider, tt.screen)

			// Verify no OTHER provider's pattern matches (cross-match check)
			for _, cp := range patterns {
				if cp.Provider != tt.expectedProvider && cp.Pattern.MatchString(tt.screen) {
					t.Errorf("pattern for %s should NOT match screen intended for %s: %q",
						cp.Provider, tt.expectedProvider, tt.screen)
				}
			}
		})
	}
}

// --- R1: ANSI-wrapped prompt detection ---

// TestIsPromptVisible_ANSIWrappedPrompts verifies that ANSI escape codes are stripped
// before prompt pattern matching, ensuring colored prompts are correctly detected.
func TestIsPromptVisible_ANSIWrappedPrompts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		screen  string
		matched bool
	}{
		{
			name:    "claude prompt with green ANSI color",
			screen:  "\x1b[32m❯\x1b[0m ",
			matched: true,
		},
		{
			name:    "claude prompt with bold+green ANSI",
			screen:  "output done\n\x1b[1;32m❯\x1b[0m",
			matched: true,
		},
		{
			name:    "codex prompt with cyan ANSI color",
			screen:  "\x1b[36mcodex>\x1b[0m ",
			matched: true,
		},
		{
			name:    "gemini prompt with ANSI wrapping",
			screen:  "\x1b[34m> Type your message...\x1b[0m",
			matched: true,
		},
		{
			name:    "shell $ prompt with ANSI",
			screen:  "\x1b[33m$\x1b[0m ",
			matched: true,
		},
		{
			name:    "multiple ANSI codes around prompt",
			screen:  "\x1b[38;5;82m\x1b[1m❯\x1b[0m\x1b[0m",
			matched: true,
		},
		{
			name:    "ANSI codes but no prompt pattern",
			screen:  "\x1b[31mprocessing...\x1b[0m",
			matched: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isPromptVisible(tt.screen, nil)
			assert.Equal(t, tt.matched, got)
		})
	}
}

// --- R5: SessionReadyPatterns excludes shell prompts ---

// TestSessionReadyPatterns_ShellDollarNotMatched verifies that shell $ does NOT match
// SessionReadyPatterns, preventing false session-ready detection on bare shell prompts.
func TestSessionReadyPatterns_ShellDollarNotMatched(t *testing.T) {
	t.Parallel()

	patterns := SessionReadyPatterns()
	shellScreens := []string{
		"$ ",
		"# ",
		"user@host $ ",
	}

	for _, screen := range shellScreens {
		assert.False(t, isSessionReady(screen, patterns),
			"shell prompt %q must NOT match SessionReadyPatterns", screen)
	}
}

// TestSessionReadyPatterns_CLIPromptsMatch verifies that CLI prompts DO match
// SessionReadyPatterns.
func TestSessionReadyPatterns_CLIPromptsMatch(t *testing.T) {
	t.Parallel()

	patterns := SessionReadyPatterns()
	tests := []struct {
		name   string
		screen string
	}{
		{"claude prompt", "❯"},
		{"gemini prompt", "> Type your message..."},
		{"codex prompt", "codex> "},
		{"opencode prompt", "Ask anything"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.True(t, isSessionReady(tt.screen, patterns),
				"CLI prompt %q must match SessionReadyPatterns", tt.screen)
		})
	}
}

// TestIsSessionReady_ANSIStripping verifies ANSI codes are stripped before matching.
func TestIsSessionReady_ANSIStripping(t *testing.T) {
	t.Parallel()

	patterns := SessionReadyPatterns()
	assert.True(t, isSessionReady("\x1b[32m❯\x1b[0m", patterns),
		"ANSI-wrapped claude prompt must match after stripping")
	assert.False(t, isSessionReady("\x1b[33m$\x1b[0m ", patterns),
		"ANSI-wrapped shell $ must NOT match SessionReadyPatterns")
}
