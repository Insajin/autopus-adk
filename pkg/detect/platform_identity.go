package detect

import "regexp"

// OMPVersionPrefix is the oh-my-pi identity marker (REQ-019). `omp` is a short,
// common binary name, so presence on PATH alone does not establish the
// platform: an unrelated binary would otherwise auto-activate omp and let the
// adapter write into a `.omp/` directory that belongs to something else.
const OMPVersionPrefix = "omp/"

var ompVersionIdentity = regexp.MustCompile(`^omp/[0-9]+\.[0-9]+\.[0-9]+$`)

// OMPVersionMatchesIdentity accepts only a complete oh-my-pi release identity.
// Pre-release suffixes and extra output are intentionally rejected because this
// gate prevents an unrelated executable named omp from being auto-activated.
func OMPVersionMatchesIdentity(version string) bool {
	return ompVersionIdentity.MatchString(version)
}
