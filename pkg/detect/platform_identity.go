package detect

import "strings"

// OMPVersionPrefix is the oh-my-pi identity marker (REQ-019). `omp` is a short,
// common binary name, so presence on PATH alone does not establish the
// platform: an unrelated binary would otherwise auto-activate omp and let the
// adapter write into a `.omp/` directory that belongs to something else.
const OMPVersionPrefix = "omp/"

// platformNeedsIdentityProbe reports whether presence on PATH is insufficient
// to conclude the platform is installed, so callers must run a version probe
// even on the presence-only detection path.
func platformNeedsIdentityProbe(name string) bool {
	return name == "omp"
}

// platformVersionMatchesIdentity checks a version string against the platform's
// identity marker. Platforms with no ambiguous binary name always match.
//
// This prefix check is ACCIDENT PREVENTION, NOT A SECURITY CONTROL. It stops an
// unrelated tool that happens to be named omp from being adopted by mistake; it
// does nothing against a hostile binary, which can print any version string it
// likes. Later work must not stack trust decisions on a passing check here —
// treat a confirmed platform as "probably the tool the user meant", never as
// "verified safe to hand privileges to".
func platformVersionMatchesIdentity(name, version string) bool {
	if name == "omp" {
		return strings.HasPrefix(version, OMPVersionPrefix)
	}
	return true
}
