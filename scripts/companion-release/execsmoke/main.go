package main

import (
	"bytes"
	"context"
	"debug/macho"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	defaultTimeout  = 15 * time.Second
	maximumTimeout  = 60 * time.Second
	defaultPipeWait = 250 * time.Millisecond
	maximumOutput   = 4096
)

var (
	errArchitectureMismatch = errors.New("artifact architecture mismatch")
	errExecutionTimeout     = errors.New("artifact execution timed out")
	errInheritedPipe        = errors.New("artifact descendant retained an output pipe")
	errVersionMismatch      = errors.New("artifact version output mismatch")
	errOutputLimit          = errors.New("artifact output exceeded limit")
	versionPattern          = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,255}$`)
)

type smokeConfig struct {
	artifact         string
	expectedVersion  string
	timeout          time.Duration
	pipeWait         time.Duration
	extraEnvironment []string
}

type limitedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	remaining := b.limit - b.buffer.Len()
	if remaining <= 0 {
		return 0, errOutputLimit
	}
	if len(data) > remaining {
		written, _ := b.buffer.Write(data[:remaining])
		return written, errOutputLimit
	}
	return b.buffer.Write(data)
}

func (b *limitedBuffer) String() string {
	return b.buffer.String()
}

func runVersionSmoke(config smokeConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), config.timeout)
	defer cancel()

	command := exec.CommandContext(ctx, config.artifact, "version", "--short")
	command.Env = append([]string{"LANG=C", "LC_ALL=C", "PATH=/usr/bin:/bin"},
		config.extraEnvironment...)
	command.Stdin = strings.NewReader("")
	command.Dir = filepath.Dir(config.artifact)
	if err := configureProcessGroup(command); err != nil {
		return fmt.Errorf("configure isolated process group: %w", err)
	}
	stdout := &limitedBuffer{limit: maximumOutput}
	stderr := &limitedBuffer{limit: maximumOutput}
	command.Stdout = stdout
	command.Stderr = stderr
	command.WaitDelay = config.pipeWait
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		return killProcessGroup(command.Process.Pid)
	}

	runErr := command.Run()
	if command.Process != nil {
		if cleanupErr := killProcessGroup(command.Process.Pid); cleanupErr != nil {
			return fmt.Errorf("clean artifact process group: %w", cleanupErr)
		}
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("%w after %s", errExecutionTimeout, config.timeout)
	}
	if errors.Is(runErr, exec.ErrWaitDelay) {
		return fmt.Errorf("%w after %s", errInheritedPipe, config.pipeWait)
	}
	if runErr != nil {
		return fmt.Errorf("artifact command failed: %w%s", runErr, stderrDiagnostic(stderr.String()))
	}
	expectedOutput := config.expectedVersion + "\n"
	if stdout.String() != expectedOutput {
		return fmt.Errorf("%w: expected exactly %q", errVersionMismatch, expectedOutput)
	}
	return nil
}

func stderrDiagnostic(value string) string {
	if value == "" {
		return ""
	}
	value = strings.NewReplacer("\r", " ", "\n", " ").Replace(value)
	if len(value) > 512 {
		value = value[:512]
	}
	return fmt.Sprintf(" (stderr: %q)", value)
}

func validateMachOArchitecture(path, expectedArchitecture string) error {
	expectedCPU, ok := map[string]macho.Cpu{
		"amd64": macho.CpuAmd64,
		"arm64": macho.CpuArm64,
	}[expectedArchitecture]
	if !ok {
		return fmt.Errorf("unsupported Darwin architecture %q", expectedArchitecture)
	}
	artifact, err := macho.Open(path)
	if err != nil {
		return fmt.Errorf("open thin Mach-O artifact: %w", err)
	}
	defer artifact.Close()
	if artifact.Cpu != expectedCPU {
		return fmt.Errorf("%w: expected %s, found %s", errArchitectureMismatch,
			expectedArchitecture, artifact.Cpu.String())
	}
	return nil
}

func validateArtifact(path string) (string, error) {
	if path == "" {
		return "", errors.New("artifact path is required")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("inspect artifact: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return "", errors.New("artifact must be a non-symlink regular executable")
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve artifact path: %w", err)
	}
	if filepath.Base(absolutePath) != "auto" {
		return "", errors.New("Darwin companion artifact must be named auto")
	}
	return absolutePath, nil
}

func runCLI(args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("companion-release-exec-smoke", flag.ContinueOnError)
	flags.SetOutput(stderr)
	artifactFlag := flags.String("artifact", "", "final signed auto artifact")
	versionFlag := flags.String("expected-version", "", "exact expected version")
	architectureFlag := flags.String("architecture", "", "Darwin artifact architecture")
	timeoutFlag := flags.Duration("timeout", defaultTimeout, "execution deadline")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("positional arguments are forbidden")
	}
	if runtime.GOOS != "darwin" {
		return errors.New("Darwin release execution smoke gate requires macOS")
	}
	if !versionPattern.MatchString(*versionFlag) {
		return errors.New("expected version is invalid")
	}
	if *timeoutFlag <= 0 || *timeoutFlag > maximumTimeout {
		return fmt.Errorf("timeout must be greater than zero and at most %s", maximumTimeout)
	}
	artifact, err := validateArtifact(*artifactFlag)
	if err != nil {
		return err
	}
	if err := validateMachOArchitecture(artifact, *architectureFlag); err != nil {
		return err
	}
	return runVersionSmoke(smokeConfig{
		artifact:        artifact,
		expectedVersion: *versionFlag,
		timeout:         *timeoutFlag,
		pipeWait:        defaultPipeWait,
	})
}

func main() {
	if err := runCLI(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "companion release execution smoke: %v\n", err)
		os.Exit(1)
	}
}
