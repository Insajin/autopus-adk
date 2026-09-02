package cli

// doctorRemediationAdvice names the command that actually repairs what failed.
//
// The banner used to say "review warnings or run 'auto doctor --fix' where
// offered" for every failure. `--fix` installs missing dependencies and nothing
// else, so on a repo whose harness is gitignored -- the layout autopus-adk
// itself uses -- a fresh clone fails every platform check and the advice sends
// the operator to a command that installs zero files and reprints the same
// advice. `auto update` is the installer.
func doctorRemediationAdvice(platformFailed, depsMissing bool) string {
	switch {
	case platformFailed && depsMissing:
		return "Issues found — run 'auto update' to install the managed surface, then 'auto doctor --fix' for missing dependencies"
	case platformFailed:
		return "Issues found — the managed surface is missing or stale; run 'auto update'"
	case depsMissing:
		return "Issues found — run 'auto doctor --fix' to install missing dependencies"
	default:
		return "Issues found — review the warnings above"
	}
}
