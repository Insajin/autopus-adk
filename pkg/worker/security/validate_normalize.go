// Package security provides input normalization for command validation.
package security

import (
	"path/filepath"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// NormalizeCommand sanitizes a command string before validation:
// 1. Strips null bytes (injection prevention)
// 2. Applies Unicode NFC normalization
// 3. Canonicalizes path segments (collapses double slashes)
// 4. Trims leading/trailing whitespace
func NormalizeCommand(cmd string) (string, error) {
	// Strip null bytes
	cmd = strings.ReplaceAll(cmd, "\x00", "")

	// Unicode NFC normalization
	cmd = norm.NFC.String(cmd)

	// Canonicalize path-like segments (collapse double slashes, resolve . and ..)
	parts := strings.Fields(cmd)
	for i, part := range parts {
		if strings.Contains(part, "/") {
			parts[i] = filepath.ToSlash(filepath.Clean(part))
		}
	}
	cmd = strings.Join(parts, " ")

	// Trim whitespace
	cmd = strings.TrimSpace(cmd)

	return cmd, nil
}

// ValidateCommandWordBoundary checks whether a command matches an allowed
// command with exact word-boundary semantics:
// - "go" matches only the bare "go" command, not "gobuster" or "go build"
func (p *SecurityPolicy) ValidateCommandWordBoundary(command, workDir string) (bool, string) {
	if len(p.AllowedCommands) == 0 {
		return false, "no allowed commands configured (fail-closed)"
	}

	// Check denied patterns
	for _, pattern := range p.DeniedPatterns {
		re, err := compilePattern(pattern)
		if err != nil {
			return false, "invalid denied pattern: " + pattern
		}
		if re.MatchString(command) {
			return false, "command matches denied pattern: " + pattern
		}
	}

	// Exact word-boundary match: allowed must match the full command exactly
	for _, allowed := range p.AllowedCommands {
		if command == allowed {
			return true, ""
		}
	}

	return false, "command not in allowed list"
}
