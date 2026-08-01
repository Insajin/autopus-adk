package detect

// DetectInstalledPlatforms returns supported coding CLIs found in PATH.
// Results preserve knownCLIs order.
//
// CONTRACT CHANGE: this function used to promise it never executed provider
// binaries. It no longer does. Detection is presence-only for every platform
// EXCEPT omp, whose binary is executed with --version.
//
// The exception is deliberate: `auto init` and `auto update` activate platforms
// from this list, `omp` is a short and commonly used name, and adopting an
// unrelated binary would point the adapter at a `.omp/` directory it does not
// own (REQ-019). Callers on an untrusted PATH should know this executes.
//
// Mitigation that is doing real work here: exec.LookPath returns ErrDot for a
// relative PATH entry, so a binary dropped in the current directory is not
// picked up. The version check itself is accident prevention rather than a
// security control — see platformVersionMatchesIdentity.
func DetectInstalledPlatforms() []Platform {
	platforms := make([]Platform, 0, len(knownCLIs))
	for _, cli := range knownCLIs {
		if !IsInstalled(cli.binary) {
			continue
		}
		if platformNeedsIdentityProbe(cli.name) {
			version, ok := detectBinary(cli.binary, cli.versionArg)
			if !ok || !platformVersionMatchesIdentity(cli.name, version) {
				continue
			}
		}
		platforms = append(platforms, Platform{Name: cli.name, Binary: cli.binary})
	}
	return platforms
}
