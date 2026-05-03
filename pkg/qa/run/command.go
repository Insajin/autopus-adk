package run

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

type commandResult struct {
	Status         string
	ExitCode       int
	FailureSummary string
	StdoutText     string
	StdoutPath     string
	StderrPath     string
	StartedAt      time.Time
	EndedAt        time.Time
	DurationMS     int64
	Command        string
}

func runCommand(projectDir string, pack journey.Pack, artifactDir string) commandResult {
	started := time.Now().UTC()
	_ = os.MkdirAll(artifactDir, 0o755)
	args := commandArgs(pack)
	result := commandResult{Status: "passed", StartedAt: started, Command: strings.Join(args, " ")}
	if len(args) == 0 {
		result.Status = "blocked"
		result.FailureSummary = "empty command"
		return finishCommandResult(result, artifactDir, nil, nil)
	}
	timeout := 60 * time.Second
	if parsed, err := time.ParseDuration(pack.Command.Timeout); err == nil && parsed > 0 {
		timeout = parsed
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = filepath.Join(projectDir, pack.Command.CWD)
	cmd.Env = allowedEnv(projectDir, pack.Command.EnvAllowlist)
	stdout, stderr := strings.Builder{}, strings.Builder{}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		result.Status = "blocked"
		result.ExitCode = -1
		result.FailureSummary = "timeout"
		return finishCommandResult(result, artifactDir, []byte(stdout.String()), []byte(stderr.String()))
	}
	if err != nil {
		result.Status = "failed"
		result.ExitCode = exitCode(err)
		result.FailureSummary = err.Error()
	}
	return finishCommandResult(result, artifactDir, []byte(stdout.String()), []byte(stderr.String()))
}

func allowedEnv(projectDir string, allowlist []string) []string {
	cacheDir := filepath.Join(projectDir, ".autopus", "qa", "cache", "go")
	_ = os.MkdirAll(cacheDir, 0o755)
	env := []string{
		"HOME=" + projectDir,
		"GOCACHE=" + cacheDir,
		"TMPDIR=" + os.TempDir(),
	}
	if path := os.Getenv("PATH"); path != "" {
		env = append(env, "PATH="+path)
	}
	for _, name := range allowlist {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	return env
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
