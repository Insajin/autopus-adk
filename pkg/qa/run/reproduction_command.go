package run

import "strings"

// renderReproductionCommand renders argv as a single POSIX shell line that a
// reader can paste and get the exact execution back.
//
// The manifest field this feeds is the only instruction a reviewer has for
// reproducing a failure, and packs are executed through exec without a shell,
// so argv legitimately carries bytes a shell would act on. A regex is the
// ordinary case: `go test -skip '^(TestA|TestB)$'` joined naively becomes a
// pipeline, so the pasted line runs something the harness never ran. Quoting
// here is what lets argv validation stop banning those bytes.
func renderReproductionCommand(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, quoteShellArgument(arg))
	}
	return strings.Join(quoted, " ")
}

// shellSafeArgument reports whether an argument survives a shell unquoted.
//
// The allowlist is deliberately narrow. Anything outside it is quoted rather
// than inspected for danger, so a byte nobody considered is quoted by default
// instead of passed through.
func shellSafeArgument(arg string) bool {
	if arg == "" {
		return false
	}
	for _, char := range arg {
		switch {
		case char >= 'a' && char <= 'z':
		case char >= 'A' && char <= 'Z':
		case char >= '0' && char <= '9':
		case char == '_' || char == '-' || char == '.' || char == '/' ||
			char == ':' || char == '=' || char == ',' || char == '+' ||
			char == '@' || char == '%':
		default:
			return false
		}
	}
	return true
}

// quoteShellArgument wraps an argument in single quotes, which suppress every
// shell expansion. An embedded single quote is closed, escaped, and reopened —
// the standard POSIX idiom, because there is no escape for a single quote
// inside single quotes.
func quoteShellArgument(arg string) string {
	if shellSafeArgument(arg) {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
}
