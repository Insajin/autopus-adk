package cli

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const maxPlaywrightDiagnosticBytes = 24 << 10

var (
	playwrightANSI        = regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07]*(?:\x07|\x1b\\))`)
	playwrightAuth        = regexp.MustCompile(`(?i)\bAuthorization\s*[:=]\s*[^\r\n]*`)
	playwrightBearer      = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+`)
	playwrightSecret      = regexp.MustCompile(`(?i)\b([A-Za-z0-9_]*(?:TOKEN|SECRET|PASSWORD))\s*[:=]\s*(?:"[^"\r\n]*"|'[^'\r\n]*'|[^\s,;]+)`)
	playwrightUNCPath     = regexp.MustCompile(`(?m)(^|[[:space:]("'=:])(\\\\[^\s"'<>]+)`)
	playwrightUnixPath    = regexp.MustCompile(`(?m)(^|[[:space:]("'=:])(/[^\s"'<>]+)`)
	playwrightWindowsPath = regexp.MustCompile(`(?im)(^|[[:space:]("'=])([a-z]:[\\/][^\s"'<>]+)`)
)

type sanitizedPlaywrightError struct {
	cause   error
	message string
}

func (err *sanitizedPlaywrightError) Error() string { return err.message }

func (err *sanitizedPlaywrightError) Unwrap() error { return err.cause }

func sanitizePlaywrightStderr(raw []byte) string {
	diagnostic := playwrightANSI.ReplaceAllString(string(raw), "")
	diagnostic = playwrightAuth.ReplaceAllString(diagnostic, "Authorization=[REDACTED]")
	diagnostic = playwrightBearer.ReplaceAllString(diagnostic, "Bearer [REDACTED]")
	diagnostic = playwrightSecret.ReplaceAllString(diagnostic, "$1=[REDACTED]")
	diagnostic = playwrightUNCPath.ReplaceAllString(diagnostic, "$1[REDACTED_PATH]")
	diagnostic = playwrightWindowsPath.ReplaceAllString(diagnostic, "$1[REDACTED_PATH]")
	diagnostic = playwrightUnixPath.ReplaceAllString(diagnostic, "$1[REDACTED_PATH]")
	diagnostic = strings.Map(func(value rune) rune {
		if value == '\n' || value == '\t' || value >= 0x20 && value != 0x7f {
			return value
		}
		return -1
	}, diagnostic)
	diagnostic = strings.TrimSpace(diagnostic)
	if len(diagnostic) <= maxPlaywrightDiagnosticBytes {
		return diagnostic
	}
	limit := maxPlaywrightDiagnosticBytes - len("\n…[truncated]")
	for limit > 0 && !utf8.RuneStart(diagnostic[limit]) {
		limit--
	}
	return diagnostic[:limit] + "\n…[truncated]"
}

func publicPlaywrightError(err error) string {
	if err == nil {
		return ""
	}
	return sanitizePlaywrightStderr([]byte(err.Error()))
}

func wrapSanitizedPlaywrightError(err error) error {
	if err == nil {
		return nil
	}
	message := publicPlaywrightError(err)
	if message == "" {
		message = "playwright process error was redacted"
	}
	return &sanitizedPlaywrightError{cause: err, message: message}
}
