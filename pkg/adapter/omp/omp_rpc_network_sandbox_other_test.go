//go:build !darwin

package omp

import (
	"context"
	"fmt"
	"os/exec"
)

func configureOMPRPCNetworkSandbox(_ *exec.Cmd, _ string) error {
	return fmt.Errorf("network sandbox unsupported on this platform")
}

func probeOMPRPCNetworkSandbox(
	_ context.Context,
	_ string,
) (ompRPCNetworkSandboxEvidence, error) {
	return ompRPCNetworkSandboxEvidence{}, fmt.Errorf("network sandbox unsupported on this platform")
}
