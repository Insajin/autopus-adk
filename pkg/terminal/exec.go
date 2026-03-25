// Package terminal provides exec abstraction for testability.
package terminal

import "os/exec"

// execCommand is a mockable function variable for creating exec.Cmd instances.
// Tests can replace this variable to intercept terminal commands.
// @AX:WARN [AUTO] global state mutation — execCommand is a mutable package-level variable replaced by tests
// @AX:REASON: concurrent test execution may cause data races when multiple tests replace this variable simultaneously; use t.Parallel() guards or per-instance injection
var execCommand = func(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}
