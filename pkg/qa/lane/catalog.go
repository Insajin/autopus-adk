// Package lane holds the canonical QA lane vocabulary.
//
// It deliberately has no dependencies. pkg/qa/release already imports
// pkg/qa/run, so the run planner cannot ask the release catalog whether a lane
// name is real without an import cycle; both sides resolve lane identity here
// instead of keeping two drifting copies.
package lane

import "slices"

// Release returns the canonical release-gate lane IDs in gate order. The order
// is the execution order of `auto qa release` and is part of the JSON contract.
func Release() []string {
	return []string{
		"fast",
		"browser-staging",
		"desktop-native",
		"gui-explore",
		"mobile-readiness",
		"canary-explicit",
		"evidence-dashboard",
	}
}

// IsRelease reports whether id is a canonical release-gate lane.
func IsRelease(id string) bool {
	return slices.Contains(Release(), id)
}

// WithoutExecutor returns release lanes the gate advertises but that no adapter
// in pkg/qa/adapter and no harness code path can execute. They stay in the
// catalog so the gate keeps reporting them as an open setup gap; what they must
// never do is report success. A project's auto-detected test suite is not an
// evidence dashboard, and calling it one is how a lane with no implementation
// ends up scoring a silent pass.
func WithoutExecutor() []string {
	return []string{"evidence-dashboard"}
}

// HasExecutor reports whether the harness ships an execution path for id.
// Unknown ids are reported as executable: only lanes this package knows to be
// unimplemented are held back.
func HasExecutor(id string) bool {
	return !slices.Contains(WithoutExecutor(), id)
}
