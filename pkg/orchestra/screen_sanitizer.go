package orchestra

import (
	"regexp"
	"strings"
)

// Compiled regex patterns for ANSI escape sequence stripping.
// @AX:NOTE: [AUTO] magic constants — compiled regexes encode terminal escape grammar; update when adding new escape types
var (
	// csiRe matches CSI sequences: \x1b[ followed by params and a final letter.
	csiRe = regexp.MustCompile(`\x1b\[\??[0-9;]*[a-zA-Z]`)

	// oscBelRe matches OSC sequences terminated by BEL (\x07).
	oscBelRe = regexp.MustCompile(`\x1b\][^\x07]*\x07`)

	// oscStRe matches OSC sequences terminated by ST (\x1b\\).
	oscStRe = regexp.MustCompile(`\x1b\].*?\x1b\\`)

	// dcsRe matches DCS sequences: \x1bP ... \x1b\\.
	dcsRe = regexp.MustCompile(`\x1bP[^\x1b]*\x1b\\`)

	// cursorSaveRestoreRe matches cursor save/restore escapes.
	cursorSaveRestoreRe = regexp.MustCompile(`\x1b[78]`)

	// statusBarRe matches tmux-style status bar lines.
	statusBarRe = regexp.MustCompile(`(?m)^\[\d+\]\s+\d+:.*$`)

	// cliBannerRe matches CLI banner lines containing Unicode block characters
	// (e.g., ▐▛███▜▌ Claude Code, ▝▜▄ Gemini CLI, etc.).
	cliBannerRe = regexp.MustCompile(`(?m)^[^\n]*[▀▁▂▃▄▅▆▇█▉▊▋▌▍▎▏▐░▒▓▔▕▖▗▘▙▚▛▜▝▞▟]+[^\n]*$`)

	// multiBlankRe matches 3+ consecutive newlines.
	multiBlankRe = regexp.MustCompile(`\n{3,}`)
)

// SanitizeScreenOutput applies all output sanitization steps:
// 1. Strip extended ANSI escape sequences (CSI, OSC, DCS)
// 2. Strip terminal status bar lines
// 3. Trim trailing whitespace per line
// 4. Collapse consecutive blank lines to at most one
func SanitizeScreenOutput(raw string) string {
	if raw == "" {
		return ""
	}
	s := stripANSIExtended(raw)
	s = stripStatusBar(s)
	s = stripCLIBanners(s)
	s = trimTrailingWhitespace(s)
	s = collapseBlankLines(s)
	return s
}

// stripANSIExtended removes all ANSI escape sequences including CSI, OSC, DCS,
// and cursor save/restore escapes.
func stripANSIExtended(s string) string {
	s = csiRe.ReplaceAllString(s, "")
	s = oscBelRe.ReplaceAllString(s, "")
	s = oscStRe.ReplaceAllString(s, "")
	s = dcsRe.ReplaceAllString(s, "")
	s = cursorSaveRestoreRe.ReplaceAllString(s, "")
	return s
}

// stripStatusBar removes tmux/terminal status bar lines.
func stripStatusBar(s string) string {
	lines := strings.Split(s, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if statusBarRe.MatchString(line) {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}

// stripCLIBanners removes lines containing Unicode block characters used in
// CLI tool banners (Claude Code, Gemini CLI, etc.).
func stripCLIBanners(s string) string {
	lines := strings.Split(s, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if cliBannerRe.MatchString(line) {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}

// collapseBlankLines reduces consecutive blank lines to at most one.
func collapseBlankLines(s string) string {
	return multiBlankRe.ReplaceAllString(s, "\n\n")
}

// trimTrailingWhitespace removes trailing whitespace from each line.
func trimTrailingWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t\r")
	}
	return strings.Join(lines, "\n")
}
