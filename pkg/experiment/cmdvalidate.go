package experiment

import (
	"fmt"
	"strings"
)

// validateConfig holds configuration for command validation.
type validateConfig struct {
	allowShellMeta bool
}

// ValidateOption configures command validation behavior.
type ValidateOption func(*validateConfig)

// AllowShellMeta returns an option that bypasses shell metacharacter validation.
func AllowShellMeta() ValidateOption {
	return func(c *validateConfig) {
		c.allowShellMeta = true
	}
}

// @AX:NOTE [AUTO]: security-critical allow/deny lists — changes here directly affect shell injection protection surface
// disallowedMultiChar lists multi-character shell metacharacter patterns.
var disallowedMultiChar = []string{"&&", "||", "$("}

// disallowedSingleChar lists single-character shell metacharacters.
// Braces and angle brackets inside single-quoted strings are safe,
// but we validate at the raw command level for defense in depth.
var disallowedSingleChar = ";&|`(){}<>\n\r"

// ValidateCommand checks cmd for disallowed shell metacharacters.
// Returns an error if any are found, unless AllowShellMeta() is passed.
func ValidateCommand(cmd string, opts ...ValidateOption) error {
	var cfg validateConfig
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.allowShellMeta {
		return nil
	}

	// Strip single-quoted segments to allow JSON like '{"metric": 1.5}'.
	stripped := stripSingleQuoted(cmd)

	// Check multi-char patterns first.
	for _, pat := range disallowedMultiChar {
		if strings.Contains(stripped, pat) {
			return fmt.Errorf("disallowed shell metacharacter %q in command", pat)
		}
	}

	// Check single-char metacharacters.
	for _, ch := range disallowedSingleChar {
		if strings.ContainsRune(stripped, ch) {
			return fmt.Errorf("disallowed shell metacharacter %q in command", string(ch))
		}
	}

	return nil
}

// stripSingleQuoted removes content inside single quotes from the command
// so that JSON payloads like '{"metric": 1.5}' are not flagged.
// If a trailing quote is unmatched, returns the original string unmodified
// to prevent metacharacters from being hidden by an open quote.
func stripSingleQuoted(s string) string {
	var b strings.Builder
	inQuote := false
	for _, ch := range s {
		if ch == '\'' {
			inQuote = !inQuote
			continue
		}
		if !inQuote {
			b.WriteRune(ch)
		}
	}
	if inQuote {
		return s
	}
	return b.String()
}
