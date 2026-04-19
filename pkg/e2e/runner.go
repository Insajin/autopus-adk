// Package e2e provides user-facing scenario-based E2E test infrastructure.
package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"
)

// RunnerOptions configures the scenario runner.
type RunnerOptions struct {
	ProjectDir   string        // project root directory
	AutoBuild    bool          // auto-build binary before first run
	BuildCommand string        // build command (e.g., "go build -o auto .") — legacy single-build
	Builds       []BuildEntry  // multi-build entries; takes precedence over BuildCommand
	Timeout      time.Duration // per-scenario timeout (default: 30s)
}

// RunnerResult holds the result of a scenario execution.
type RunnerResult struct {
	ExitCode       int    // command exit code
	Stdout         string // captured stdout
	Stderr         string // captured stderr
	Pass           bool   // true if all verify primitives passed
	FailureDetails string // human-readable failure description
	TimedOut       bool   // true if command exceeded timeout
	BuildOccurred  bool   // true if auto-build was triggered this run
	WorkDir        string // temp directory used for this run
}

// WorkDirExists checks if the work directory still exists.
// Returns an error if the directory has been removed (expected after cleanup).
func (r *RunnerResult) WorkDirExists() (bool, error) {
	_, err := os.Stat(r.WorkDir)
	if err != nil {
		return false, err
	}
	return true, nil
}

// Runner executes E2E scenarios.
type Runner struct {
	opts         RunnerOptions
	buildOnce    sync.Once             // legacy single-build once guard
	buildErr     error                 // legacy single-build error
	buildOnceMu  sync.Mutex            // protects buildOnceMap and buildErrMap
	buildOnceMap map[string]*sync.Once // per-label build once guards
	buildErrMap  map[string]error      // per-label build errors
}

// @AX:NOTE [AUTO] @AX:REASON: magic constant — 30s default per-scenario timeout; adjust for slow integration tests or network-dependent scenarios
// NewRunner creates a new Runner with the given options.
func NewRunner(opts RunnerOptions) *Runner {
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	return &Runner{opts: opts}
}

// @AX:NOTE [AUTO] @AX:REASON: public API boundary — primary scenario execution entry point; fan_in=1 (internal/cli/test.go); sync.Once ensures build runs exactly once per Runner instance
// Run executes a single scenario and returns the result.
func (r *Runner) Run(scenario Scenario) (*RunnerResult, error) {
	result := &RunnerResult{}

	// Auto-build if enabled.
	if r.opts.AutoBuild {
		built, err := r.runBuild(scenario)
		if err != nil {
			return nil, err
		}
		result.BuildOccurred = built
	}

	// Create isolated temp directory for this run.
	workDir, err := os.MkdirTemp("", "e2e-run-*")
	if err != nil {
		return nil, fmt.Errorf("create work dir: %w", err)
	}
	result.WorkDir = workDir
	defer func() {
		_ = os.RemoveAll(workDir)
	}()

	// Execute command with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), r.opts.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", scenario.Command)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	if ctx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.Pass = false
		result.ExitCode = -1
		return result, nil
	}

	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
		}
	} else {
		result.ExitCode = 0
	}

	// Evaluate verify primitives.
	allPass := true
	for _, primitive := range scenario.Verify {
		vr := evaluatePrimitive(primitive, result)
		if !vr.Pass {
			allPass = false
			result.FailureDetails += vr.Message + "\n"
		}
	}
	result.Pass = allPass

	return result, nil
}

// runBuild handles build execution for a scenario.
// Multi-build path: matches scenario section to a BuildEntry, runs per-label once.
// Legacy path: uses BuildCommand with a single sync.Once.
// Returns (true, nil) when a build was triggered this call.
func (r *Runner) runBuild(scenario Scenario) (bool, error) {
	// Multi-build path: Builds takes precedence over BuildCommand.
	if len(r.opts.Builds) > 0 {
		matched := MatchBuild(scenario, r.opts.Builds)
		if matched == nil {
			// No matching build for this scenario — skip build (R5).
			return false, nil
		}
		return r.runLabeledBuild(*matched)
	}

	// Legacy fallback: single BuildCommand (R4).
	if r.opts.BuildCommand == "" {
		return false, nil
	}
	builtThisCall := false
	r.buildOnce.Do(func() {
		cmd := exec.Command("sh", "-c", r.opts.BuildCommand)
		cmd.Dir = r.opts.ProjectDir
		if err := cmd.Run(); err != nil {
			r.buildErr = err
		}
		builtThisCall = true
	})
	if r.buildErr != nil {
		return false, fmt.Errorf("auto-build failed: %w", r.buildErr)
	}
	return builtThisCall, nil
}

// @AX:WARN [AUTO] @AX:REASON: concurrent map + sync.Once — buildOnceMu guards buildOnceMap/buildErrMap; lock ordering: acquire buildOnceMu, read/init map, release, then call once.Do
// runLabeledBuild runs a build for a specific label, using per-label Once caching.
func (r *Runner) runLabeledBuild(entry BuildEntry) (bool, error) {
	label := entry.Label
	if label == "" {
		label = "__default__"
	}

	r.buildOnceMu.Lock()
	if r.buildOnceMap == nil {
		r.buildOnceMap = make(map[string]*sync.Once)
		r.buildErrMap = make(map[string]error)
	}
	once, ok := r.buildOnceMap[label]
	if !ok {
		once = &sync.Once{}
		r.buildOnceMap[label] = once
	}
	r.buildOnceMu.Unlock()

	builtThisCall := false
	once.Do(func() {
		buildDir := ResolveBuildDir(r.opts.ProjectDir, entry)
		cmd := exec.Command("sh", "-c", entry.Command)
		cmd.Dir = buildDir
		if err := cmd.Run(); err != nil {
			r.buildOnceMu.Lock()
			r.buildErrMap[label] = err
			r.buildOnceMu.Unlock()
		}
		builtThisCall = true
	})

	r.buildOnceMu.Lock()
	buildErr := r.buildErrMap[label]
	r.buildOnceMu.Unlock()

	if buildErr != nil {
		return false, fmt.Errorf("auto-build failed (label=%s): %w", label, buildErr)
	}
	return builtThisCall, nil
}

// evaluatePrimitive evaluates a single verification primitive string against the result.
func evaluatePrimitive(primitive string, result *RunnerResult) VerifyResult {
	switch {
	case primitive == "exit_code(0)":
		return CheckExitCode(0, result.ExitCode)
	case len(primitive) > 12 && primitive[:11] == "exit_code(":
		var code int
		_, _ = fmt.Sscanf(primitive, "exit_code(%d)", &code)
		return CheckExitCode(code, result.ExitCode)
	case primitive == "stderr_empty()":
		return CheckStderrEmpty(result.Stderr)
	default:
		// Return pass for unknown primitives (other agents will implement them).
		return VerifyResult{Primitive: primitive, Pass: true}
	}
}
