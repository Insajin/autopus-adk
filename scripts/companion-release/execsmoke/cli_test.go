//go:build darwin || linux

package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestValidateMachOArchitecture_CurrentTestBinary_MatchesHost(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Mach-O execution contract is Darwin-only")
	}
	testBinary := testArtifact(t)

	if err := validateMachOArchitecture(testBinary, runtime.GOARCH); err != nil {
		t.Fatalf("validateMachOArchitecture() error = %v", err)
	}
}

func TestValidateMachOArchitecture_WrongArchitecture_Fails(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Mach-O execution contract is Darwin-only")
	}
	wrongArchitecture := "arm64"
	if runtime.GOARCH == "arm64" {
		wrongArchitecture = "amd64"
	}

	err := validateMachOArchitecture(testArtifact(t), wrongArchitecture)

	if !errors.Is(err, errArchitectureMismatch) {
		t.Fatalf("validateMachOArchitecture() error = %v, want errArchitectureMismatch", err)
	}
}

func TestValidateMachOArchitecture_InvalidOrUnsupportedArtifact_Fails(t *testing.T) {
	t.Parallel()
	notMachO := filepath.Join(t.TempDir(), "not-macho")
	if err := os.WriteFile(notMachO, []byte("not a Mach-O\n"), 0o700); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	if err := validateMachOArchitecture(notMachO, "arm64"); err == nil {
		t.Fatal("validateMachOArchitecture() error = nil, want malformed artifact failure")
	}
	if err := validateMachOArchitecture(notMachO, "mips64"); err == nil {
		t.Fatal("validateMachOArchitecture() error = nil, want unsupported architecture failure")
	}
}

func TestValidatePinnedOMPExecutable_WrongDigestFails(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("pinned arm64 OMP digest validation is Darwin arm64 only")
	}
	executable := linkTestArtifact(t, "omp")
	_, err := validatePinnedOMPExecutable(executable, ompCanaryPolicy{
		version: pinnedOMPVersion,
		sha256:  "sha256:" + strings.Repeat("0", 64),
	})
	if err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("validatePinnedOMPExecutable() error = %v, want SHA-256 mismatch", err)
	}
}

func TestRunCLI_ValidMachOAndExactVersion_Succeeds(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("execution smoke CLI is Darwin-only")
	}
	artifact := linkTestArtifact(t, "auto")
	isolation, ompExecutable := testCanaryUIDIsolation(t)
	info, err := os.Lstat(ompExecutable)
	if err != nil {
		t.Fatalf("os.Lstat() error = %v", err)
	}
	digest, err := stableExecutableSHA256(ompExecutable, info)
	if err != nil {
		t.Fatalf("stableExecutableSHA256() error = %v", err)
	}

	args := validCLIArgs(artifact, runtime.GOARCH)
	if runtime.GOARCH == "arm64" {
		t.Setenv("OMP_CONTEXT_RELEASE_CANARY_EXECUTABLE", ompExecutable)
		t.Setenv("OMP_CONTEXT_RELEASE_CANARY_ROOT", isolation.root)
	}
	err = runCLIWithPolicy(
		args, io.Discard,
		ompCanaryPolicy{version: pinnedOMPVersion, sha256: digest, isolation: isolation.policy},
	)

	if err != nil {
		t.Fatalf("runCLI() error = %v", err)
	}
}

func TestRunCLI_InvalidInputs_FailClosed(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("execution smoke CLI is Darwin-only")
	}
	validArtifact := linkTestArtifact(t, "auto")
	notExecutable := filepath.Join(t.TempDir(), "auto")
	if err := os.WriteFile(notExecutable, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	wrongName := linkTestArtifact(t, "not-auto")
	symlink := filepath.Join(t.TempDir(), "auto")
	if err := os.Symlink(validArtifact, symlink); err != nil {
		t.Fatalf("os.Symlink() error = %v", err)
	}
	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown flag", args: []string{"--unknown"}},
		{name: "positional argument", args: []string{"extra"}},
		{name: "missing artifact", args: []string{"--expected-version", expectedFixtureVersion}},
		{name: "invalid version", args: []string{"--expected-version", "bad version"}},
		{name: "zero timeout", args: []string{"--expected-version", expectedFixtureVersion, "--timeout", "0s"}},
		{name: "excessive timeout", args: []string{"--expected-version", expectedFixtureVersion, "--timeout", "61s"}},
		{name: "not executable", args: validCLIArgs(notExecutable, runtime.GOARCH)},
		{name: "wrong artifact name", args: validCLIArgs(wrongName, runtime.GOARCH)},
		{name: "symlink artifact", args: validCLIArgs(symlink, runtime.GOARCH)},
		{name: "unsupported architecture", args: validCLIArgs(validArtifact, "mips64")},
		{name: "missing arm64 OMP", args: validCLIArgs(validArtifact, "arm64")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := runCLI(test.args, io.Discard); err == nil {
				t.Fatal("runCLI() error = nil, want fail-closed validation")
			}
		})
	}
}

func TestLimitedBuffer_Overflow_IsBounded(t *testing.T) {
	t.Parallel()
	buffer := &limitedBuffer{limit: 4}

	written, err := buffer.Write([]byte("123456"))

	if !errors.Is(err, errOutputLimit) {
		t.Fatalf("limitedBuffer.Write() error = %v, want errOutputLimit", err)
	}
	if written != 4 || buffer.String() != "1234" {
		t.Fatalf("limitedBuffer.Write() = (%d, %q), want (4, %q)", written, buffer.String(), "1234")
	}
	if written, err = buffer.Write([]byte("7")); !errors.Is(err, errOutputLimit) || written != 0 {
		t.Fatalf("second limitedBuffer.Write() = (%d, %v), want (0, errOutputLimit)", written, err)
	}
}

func TestStderrDiagnostic_MultilineAndLong_IsSingleLineAndBounded(t *testing.T) {
	t.Parallel()
	diagnostic := stderrDiagnostic(strings.Repeat("x", 600) + "\r\nsecret")

	if strings.ContainsAny(diagnostic, "\r\n") {
		t.Fatalf("stderrDiagnostic() contains a line break: %q", diagnostic)
	}
	if len(diagnostic) > 530 {
		t.Fatalf("stderrDiagnostic() length = %d, want bounded diagnostic", len(diagnostic))
	}
	if stderrDiagnostic("") != "" {
		t.Fatal("stderrDiagnostic(\"\") must be empty")
	}
}

func linkTestArtifact(t *testing.T, name string) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), name)
	if err := os.Link(testArtifact(t), target); err != nil {
		t.Fatalf("os.Link() error = %v", err)
	}
	return target
}

func validCLIArgs(artifact, architecture string) []string {
	return []string{
		"--artifact", artifact,
		"--expected-version", expectedFixtureVersion,
		"--architecture", architecture,
		"--timeout", "10s",
	}
}
