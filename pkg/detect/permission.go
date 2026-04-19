package detect

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// PermissionResult holds the detected permission mode and metadata.
type PermissionResult struct {
	Mode      string `json:"mode"`       // "bypass" or "safe"
	ParentPID int    `json:"parent_pid"` // parent process PID
	FlagFound bool   `json:"flag_found"` // whether the flag was found
}

const (
	modeBypass = "bypass"
	modeSafe   = "safe"

	permissionFlag = "--dangerously-skip-permissions"
	envKey         = "AUTOPUS_PERMISSION_MODE"
	// @AX:NOTE [AUTO] magic constant — max process tree traversal depth, prevents infinite loops on circular PID references
	maxTreeDepth = 20
)

// checkProcessTreeFn is the process tree checker used by DetectPermissionMode.
// Package-level indirection so tests can swap in a deterministic stub — the
// default checker walks the real parent process tree, which is unpredictable
// when tests run inside a Claude Code session launched with
// --dangerously-skip-permissions.
var checkProcessTreeFn = checkParentProcessTree

// DetectPermissionMode checks the parent process tree for the
// --dangerously-skip-permissions flag and returns the permission mode.
// Priority: AUTOPUS_PERMISSION_MODE env > process tree scan > cmux heuristic.
// cmux launches Claude Code without visible CLI flags, so process tree scan
// always returns "safe". When cmux is detected, assume bypass mode.
func DetectPermissionMode() PermissionResult {
	if env := os.Getenv(envKey); env == modeBypass || env == modeSafe {
		return PermissionResult{Mode: env}
	}
	result := detectPermissionModeWith(checkProcessTreeFn)
	if result.FlagFound {
		return result
	}
	// cmux heuristic: cmux runs Claude in bypass mode but does not pass the
	// flag as a visible CLI argument. Detect via CMUX_CLAUDE_PID env var.
	if os.Getenv("CMUX_CLAUDE_PID") != "" {
		return PermissionResult{Mode: modeBypass, ParentPID: result.ParentPID, FlagFound: true}
	}
	return result
}

// detectPermissionModeWith uses an injectable checker for testability.
func detectPermissionModeWith(checker func() (bool, int, error)) PermissionResult {
	flagFound, parentPID, err := checker()
	if err != nil {
		return PermissionResult{Mode: modeSafe}
	}
	if flagFound {
		return PermissionResult{Mode: modeBypass, ParentPID: parentPID, FlagFound: true}
	}
	return PermissionResult{Mode: modeSafe, ParentPID: parentPID}
}

// checkParentProcessTree walks up the process tree looking for the
// --dangerously-skip-permissions flag in command arguments.
// Individual process lookup failures are skipped so that transient
// errors (exited intermediary, permission denied) do not abort the
// entire tree walk before reaching the ancestor that carries the flag.
func checkParentProcessTree() (flagFound bool, parentPID int, err error) {
	pid := os.Getppid()
	parentPID = pid

	for i := 0; i < maxTreeDepth && pid > 1; i++ {
		args, argsErr := processArgs(pid)
		if argsErr == nil && strings.Contains(args, permissionFlag) {
			return true, parentPID, nil
		}
		// On processArgs error, skip this PID and continue to its parent.

		nextPID, ppidErr := parentPIDOf(pid)
		if ppidErr != nil {
			// Cannot determine next ancestor — stop walking.
			break
		}
		pid = nextPID
	}
	return false, parentPID, nil
}

// processArgs returns the command-line arguments for a given PID.
func processArgs(pid int) (string, error) {
	out, err := exec.Command("ps", "-o", "args=", "-p", fmt.Sprintf("%d", pid)).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// parentPIDOf returns the parent PID of the given PID.
func parentPIDOf(pid int) (int, error) {
	out, err := exec.Command("ps", "-o", "ppid=", "-p", fmt.Sprintf("%d", pid)).Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}
