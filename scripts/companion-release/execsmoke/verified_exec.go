package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const verifiedExecSmokeSchemaV1 = "workflow-context-verified-exec-smoke/v1"

var errVerifiedExecOutput = errors.New("verified OMP execution smoke output mismatch")

type verifiedExecSmokeConfig struct {
	artifact         string
	ompExecutable    string
	canaryRoot       string
	policy           ompCanaryPolicy
	timeout          time.Duration
	pipeWait         time.Duration
	extraEnvironment []string
	isolation        *canaryUIDIsolation
}

type verifiedExecSmokeOutput struct {
	SchemaVersion       string `json:"schema_version"`
	OMPVersion          string `json:"omp_version"`
	OMPExecutableSHA256 string `json:"omp_executable_sha256"`
	RPCReady            bool   `json:"rpc_ready"`
	ProviderCalls       int64  `json:"provider_calls"`
	UIDIsolated         bool   `json:"uid_isolated"`
	EffectiveUser       string `json:"effective_user"`
}

func runVerifiedExecSmoke(config verifiedExecSmokeConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), config.timeout)
	defer cancel()
	command, err := newCanaryCommand(ctx, config.artifact, config.isolation,
		"workflow", "context-runtime", "verified-exec-smoke",
		"--omp-executable", config.ompExecutable, "--canary-root", config.canaryRoot, "--format", "json")
	if err != nil {
		return err
	}
	command.Env = append([]string{"LANG=C", "LC_ALL=C", "PATH=/usr/bin:/bin"}, config.extraEnvironment...)
	command.Stdin = strings.NewReader("")
	command.Dir = filepath.Dir(config.artifact)
	if err := configureProcessGroup(command); err != nil {
		return fmt.Errorf("configure verified OMP smoke process group: %w", err)
	}
	stdout := &limitedBuffer{limit: maximumOutput}
	stderr := &limitedBuffer{limit: maximumOutput}
	command.Stdout, command.Stderr, command.WaitDelay = stdout, stderr, config.pipeWait
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		return killProcessGroup(command.Process.Pid)
	}
	runErr := command.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("%w after %s", errExecutionTimeout, config.timeout)
	}
	if command.Process != nil {
		if cleanupErr := killProcessGroup(command.Process.Pid); cleanupErr != nil {
			return fmt.Errorf("clean verified OMP smoke process group: %w", cleanupErr)
		}
	}
	if errors.Is(runErr, exec.ErrWaitDelay) {
		return fmt.Errorf("%w after %s", errInheritedPipe, config.pipeWait)
	}
	if stdout.exceeded || stderr.exceeded {
		return errOutputLimit
	}
	if runErr != nil {
		return fmt.Errorf("verified OMP smoke command failed: %w%s", runErr, stderrDiagnostic(stderr.String()))
	}
	return validateVerifiedExecSmokeOutput([]byte(stdout.String()), config.policy)
}

func validateVerifiedExecSmokeOutput(data []byte, policy ompCanaryPolicy) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var output verifiedExecSmokeOutput
	if err := decoder.Decode(&output); err != nil {
		return fmt.Errorf("%w: invalid JSON", errVerifiedExecOutput)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("%w: trailing output", errVerifiedExecOutput)
	}
	if output.SchemaVersion != verifiedExecSmokeSchemaV1 || output.OMPVersion != policy.version ||
		output.OMPExecutableSHA256 != policy.sha256 || !output.RPCReady || output.ProviderCalls != 0 ||
		!output.UIDIsolated || output.EffectiveUser != "nobody" {
		return errVerifiedExecOutput
	}
	canonical, err := json.Marshal(output)
	if err != nil || !bytes.Equal(data, append(canonical, '\n')) {
		return fmt.Errorf("%w: non-canonical JSON", errVerifiedExecOutput)
	}
	return nil
}
