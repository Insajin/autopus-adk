package cli

import "fmt"

// bothBackendsUnavailableError returns an actionable error (REQ-009) for the
// case where both execution paths have failed:
//
//   - Interactive hook path: hook subsystem unavailable or provider panes timed out.
//   - Subprocess fallback path: provider CLI exited non-zero (typically missing API key).
//
// The message embeds three recovery keywords that acceptance oracle S9 asserts:
// "auto init", "cmux", and "API".
//
// Pass a non-empty detail to append provider-specific failure context.
func bothBackendsUnavailableError(detail string) error {
	base := "multi-provider execution failed: " +
		"interactive hook path unavailable " +
		"(run `auto init` to reinstall hooks, verify cmux/tmux is installed) " +
		"and subprocess -p fallback also failed " +
		"(check that an API key is configured for each provider)"
	if detail != "" {
		return fmt.Errorf("%s — %s", base, detail)
	}
	return fmt.Errorf("%s", base) //nolint:err113
}
