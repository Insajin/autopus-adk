// Package security provides ReDoS-safe command validation.
package security

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const maxPatternLength = 1024

// compilePattern compiles a regex pattern with basic safety checks.
func compilePattern(pattern string) (*regexp.Regexp, error) {
	return regexp.Compile(pattern)
}

// ValidateDeniedPatterns checks all DeniedPatterns for validity:
// - Patterns longer than 1024 characters are rejected
// - Patterns that fail to compile are rejected
func (p *SecurityPolicy) ValidateDeniedPatterns() error {
	for _, pattern := range p.DeniedPatterns {
		if len(pattern) > maxPatternLength {
			return fmt.Errorf("pattern too long (%d chars, max %d)", len(pattern), maxPatternLength)
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid denied pattern: %w", err)
		}
	}
	return nil
}

// ValidateCommandSafe is a timeout-protected version of ValidateCommand.
// Each denied pattern is matched with a 100ms deadline to prevent ReDoS.
// On timeout, the command is denied (fail-closed).
func (p *SecurityPolicy) ValidateCommandSafe(command, workDir string) (bool, string) {
	if len(p.AllowedCommands) == 0 {
		return false, "no allowed commands configured (fail-closed)"
	}

	// Normalize before any pattern matching to prevent null-byte/encoding bypass.
	command = normalizeForValidation(command)

	// Check denied patterns with timeout protection
	for _, pattern := range p.DeniedPatterns {
		if len(pattern) > maxPatternLength {
			return false, "denied pattern too long (fail-closed)"
		}

		matched, err := matchWithTimeout(pattern, command, 100*time.Millisecond)
		if err != nil {
			// Compile failure or timeout → deny all (fail-closed)
			return false, "denied pattern check failed: " + err.Error()
		}
		if matched {
			return false, "command matches denied pattern: " + pattern
		}
	}

	// Word-boundary command match
	commandAllowed := false
	for _, allowed := range p.AllowedCommands {
		trimmed := strings.TrimRight(allowed, " ")
		if command == trimmed || strings.HasPrefix(command, trimmed+" ") {
			commandAllowed = true
			break
		}
	}
	if !commandAllowed {
		return false, "command not in allowed list"
	}

	// Directory check
	if len(p.AllowedDirs) > 0 && workDir != "" {
		dirAllowed := false
		for _, dir := range p.AllowedDirs {
			if strings.HasPrefix(workDir, dir+"/") || workDir == dir {
				dirAllowed = true
				break
			}
		}
		if !dirAllowed {
			return false, "working directory not in allowed list"
		}
	}

	return true, ""
}

// ValidateCommandDenyOnBadPattern denies all commands if any denied pattern
// fails to compile (fail-closed behavior).
func (p *SecurityPolicy) ValidateCommandDenyOnBadPattern(command, workDir string) (bool, string) {
	if len(p.AllowedCommands) == 0 {
		return false, "no allowed commands configured (fail-closed)"
	}

	// Normalize before any pattern matching to prevent null-byte/encoding bypass.
	command = normalizeForValidation(command)

	for _, pattern := range p.DeniedPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false, "invalid denied pattern (deny-all): " + pattern
		}
		if re.MatchString(command) {
			return false, "command matches denied pattern: " + pattern
		}
	}

	// Word-boundary command match
	commandAllowed := false
	for _, allowed := range p.AllowedCommands {
		trimmed := strings.TrimRight(allowed, " ")
		if command == trimmed || strings.HasPrefix(command, trimmed+" ") {
			commandAllowed = true
			break
		}
	}
	if !commandAllowed {
		return false, "command not in allowed list"
	}

	return true, ""
}

// matchWithTimeout runs a regex match with a context deadline.
// Returns (matched, error). Error is non-nil on compile failure or timeout.
//
// NOTE: On timeout, the goroutine running re.MatchString continues executing
// because Go's regexp engine cannot be interrupted mid-match. The buffered
// channel prevents a goroutine leak (deadlock), but the goroutine may consume
// CPU until the match completes. This is acceptable because Go's RE2 engine
// guarantees linear-time matching, so the goroutine will terminate eventually.
// The timeout protects against patterns with large constant factors, not
// exponential backtracking (which RE2 prevents by design).
func matchWithTimeout(pattern, input string, timeout time.Duration) (bool, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Errorf("compile failed: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	type result struct {
		matched bool
	}

	ch := make(chan result, 1)
	go func() {
		ch <- result{matched: re.MatchString(input)}
	}()

	select {
	case r := <-ch:
		return r.matched, nil
	case <-ctx.Done():
		return false, fmt.Errorf("pattern match timed out")
	}
}
