package cli

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter/omp"
)

type ompOperatorExecRunner struct {
	process *omp.OMPModelProbeProcess
	pinErr  error
}

func newOMPOperatorExecRunner() *ompOperatorExecRunner {
	process, err := omp.NewOMPInstalledModelProbeProcess("omp", ompModelDoctorProbeOutput)
	return &ompOperatorExecRunner{process: process, pinErr: err}
}

func (runner ompOperatorExecRunner) Run(
	ctx context.Context,
	executable string,
	args ...string,
) ([]byte, error) {
	if executable != "omp" || !safeOMPOperatorProbeArgs(args) {
		return nil, errors.New("unsafe OMP operator command")
	}
	if runner.pinErr != nil {
		return nil, runner.pinErr
	}
	return runner.process.Run(ctx, args...)
}

func safeOMPOperatorProbeArgs(args []string) bool {
	joined := strings.Join(args, " ")
	if joined == "--version" || joined == "models --json --no-extensions" ||
		joined == "config list --json" {
		return true
	}
	keyIndex := 2
	if len(args) == 6 && args[0] == "--config" && filepath.IsAbs(args[1]) &&
		!strings.ContainsAny(args[1], "\x00\r\n") {
		keyIndex = 4
	}
	if len(args) != keyIndex+2 || args[keyIndex-2] != "config" ||
		args[keyIndex-1] != "get" || args[keyIndex+1] != "--json" {
		return false
	}
	allowed := map[string]bool{
		"modelRoles": true, "retry.fallbackChains": true, "retry.modelFallback": true,
		"tools.approvalMode": true, "task.isolation.mode": true,
	}
	return allowed[args[keyIndex]]
}
