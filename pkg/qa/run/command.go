package run

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

type commandResult struct {
	Status            string
	ExitCode          int
	FailureSummary    string
	StdoutText        string
	StdoutPath        string
	StderrPath        string
	StartedAt         time.Time
	EndedAt           time.Time
	DurationMS        int64
	Command           string
	GUIGuardReadyPath string
}

func runCommand(projectDir string, pack journey.Pack, artifactDir string) commandResult {
	return runCommandWithEnv(projectDir, pack, artifactDir, nil)
}

// runCommandWithEnv runs the pack command through the single shared exec engine,
// optionally injecting extra environment entries (used by the mobile lane to
// pass the runtime device handle). It reuses the same engine as runCommand so
// there is no duplicate execution path (REQ-EXEC-01).
func runCommandWithEnv(projectDir string, pack journey.Pack, artifactDir string, extraEnv []string) commandResult {
	started := time.Now().UTC()
	_ = os.MkdirAll(artifactDir, 0o755)
	args := commandArgs(pack)
	result := commandResult{Status: "passed", StartedAt: started, Command: strings.Join(args, " ")}
	if len(args) == 0 {
		result.Status = "blocked"
		result.FailureSummary = "empty command"
		return finishCommandResult(result, artifactDir, nil, nil)
	}
	guiInput, err := prepareGUIPolicyInput(pack, artifactDir)
	if err != nil {
		result.Status = "blocked"
		result.ExitCode = -1
		result.FailureSummary = err.Error()
		return finishCommandResult(result, artifactDir, nil, nil)
	}
	result.GUIGuardReadyPath = guiInput.GuardReadyPath
	commandCache, err := prepareCommandGoCache(projectDir)
	if err != nil {
		result.Status = "blocked"
		result.ExitCode = -1
		result.FailureSummary = "qa go cache setup failed: " + err.Error()
		return finishCommandResult(result, artifactDir, nil, nil)
	}
	defer commandCache.Cleanup()
	timeout := commandTimeout(pack.Command.Timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = filepath.Join(projectDir, pack.Command.CWD)
	cmd.Env = authoritativeCommandEnv(commandCache.Paths, pack.Command.EnvAllowlist, append(append([]string{}, guiInput.Env...), extraEnv...))
	if err := verifyGUIGuardPreflight(ctx, cmd.Dir, cmd.Env, guiInput, args); err != nil {
		result.Status = "blocked"
		result.ExitCode = -1
		result.FailureSummary = err.Error()
		return finishCommandResult(result, artifactDir, nil, nil)
	}
	stdout, stderr := strings.Builder{}, strings.Builder{}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		result.Status = "blocked"
		result.ExitCode = -1
		result.FailureSummary = "timeout after " + timeout.String()
		return finishCommandResult(result, artifactDir, []byte(stdout.String()), []byte(stderr.String()))
	}
	if err != nil {
		result.Status = "failed"
		result.ExitCode = exitCode(err)
		result.FailureSummary = err.Error()
	}
	return finishCommandResult(result, artifactDir, []byte(stdout.String()), []byte(stderr.String()))
}

func commandTimeout(value string) time.Duration {
	timeout := 60 * time.Second
	if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
		timeout = parsed
	}
	if timeout > journey.MaxCommandTimeout {
		return journey.MaxCommandTimeout
	}
	return timeout
}

func allowedEnv(paths goCachePaths, allowlist []string) []string {
	projectDir := paths.ProjectDir
	home := projectDir
	if envNameAllowed(allowlist, "HOME") {
		if value := os.Getenv("HOME"); value != "" {
			home = value
		}
	}
	env := []string{
		"HOME=" + home,
		"TMPDIR=" + os.TempDir(),
	}
	if path := os.Getenv("PATH"); path != "" {
		env = append(env, "PATH="+path)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		env = appendDefaultEnv(env, "CARGO_HOME", filepath.Join(home, ".cargo"))
		env = appendDefaultEnv(env, "RUSTUP_HOME", filepath.Join(home, ".rustup"))
		env = appendDefaultEnv(env, "PLAYWRIGHT_BROWSERS_PATH", defaultPlaywrightBrowsersPath(home))
	}
	for _, name := range allowlist {
		if strings.EqualFold(name, "HOME") || isManagedGoEnvName(name) {
			continue
		}
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	return appendEnvOverrides(env, managedGoEnv(paths))
}

func authoritativeCommandEnv(paths goCachePaths, allowlist, overrides []string) []string {
	env := appendEnvOverrides(allowedEnv(paths, allowlist), overrides)
	return appendEnvOverrides(env, managedGoEnv(paths))
}

func managedGoEnv(paths goCachePaths) []string {
	return []string{
		"GOCACHE=" + paths.GoBuild,
		"GOMODCACHE=" + paths.GoMod,
		"GOPATH=" + paths.GoPath,
	}
}

func isManagedGoEnvName(name string) bool {
	switch {
	case strings.EqualFold(name, "GOCACHE"):
		return true
	case strings.EqualFold(name, "GOMODCACHE"):
		return true
	case strings.EqualFold(name, "GOPATH"):
		return true
	default:
		return false
	}
}

func envNameAllowed(allowlist []string, target string) bool {
	for _, name := range allowlist {
		if name == target {
			return true
		}
	}
	return false
}

func appendDefaultEnv(env []string, name, fallback string) []string {
	if value := os.Getenv(name); value != "" {
		return append(env, name+"="+value)
	}
	return append(env, name+"="+fallback)
}

func defaultPlaywrightBrowsersPath(home string) string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Caches", "ms-playwright")
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(home, "AppData", "Local", "ms-playwright")
	}
	return filepath.Join(home, ".cache", "ms-playwright")
}

func finishCommandResult(result commandResult, artifactDir string, stdout, stderr []byte) commandResult {
	result.EndedAt = time.Now().UTC()
	result.DurationMS = result.EndedAt.Sub(result.StartedAt).Milliseconds()
	result.StdoutText = string(stdout)
	result.StdoutPath = filepath.Join(artifactDir, "stdout.log")
	result.StderrPath = filepath.Join(artifactDir, "stderr.log")
	_ = os.WriteFile(result.StdoutPath, stdout, 0o644)
	_ = os.WriteFile(result.StderrPath, stderr, 0o644)
	return result
}

func commandArgs(pack journey.Pack) []string {
	if len(pack.Command.Argv) > 0 {
		return pack.Command.Argv
	}
	if strings.TrimSpace(pack.Command.Run) != "" {
		return strings.Fields(pack.Command.Run)
	}
	defaultCommand := defaultCommand(pack.Adapter.ID)
	if strings.TrimSpace(defaultCommand.Run) == "" && len(defaultCommand.Argv) == 0 {
		return nil
	}
	return commandArgs(journey.Pack{Adapter: pack.Adapter, Command: defaultCommand})
}

func defaultCommand(id string) journey.Command {
	switch id {
	case "go-test":
		return journey.Command{Run: "go test ./...", CWD: ".", Timeout: "60s"}
	case "node-script":
		return journey.Command{Run: "npm test", CWD: ".", Timeout: "60s"}
	case "pytest":
		return journey.Command{Run: "pytest", CWD: ".", Timeout: "60s"}
	case "cargo-test":
		return journey.Command{Run: "cargo test", CWD: ".", Timeout: "60s"}
	default:
		return journey.Command{CWD: ".", Timeout: "60s"}
	}
}

func exitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}
