//go:build darwin || linux

package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func testCanaryUIDIsolation(t *testing.T) (*canaryUIDIsolation, string) {
	t.Helper()
	uid, err := canaryFileUID(mustLstat(t, testArtifact(t)))
	if err != nil {
		t.Fatalf("canaryFileUID() error = %v", err)
	}
	root := filepath.Join(t.TempDir(), "canary")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatalf("os.Mkdir() error = %v", err)
	}
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatalf("os.Chmod(root) error = %v", err)
	}
	for _, name := range []string{"home", "tmp"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o700); err != nil {
			t.Fatalf("os.Mkdir(%q) error = %v", name, err)
		}
	}
	omp := filepath.Join(root, "omp-darwin-arm64")
	body, err := os.ReadFile(testArtifact(t))
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	if err := os.WriteFile(omp, body, 0o555); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.Chmod(omp, 0o555); err != nil {
		t.Fatalf("os.Chmod() error = %v", err)
	}
	runner := filepath.Join(t.TempDir(), "sudo")
	environment := filepath.Join(t.TempDir(), "env")
	writeUIDIsolationScript(t, environment, "#!/bin/sh\nexec /usr/bin/env \"$@\"\n", 0o700)
	runnerBody := "#!/bin/sh\n[ \"$1\" = -n ] && [ \"$2\" = -u ] && [ \"$3\" = nobody ] || exit 71\nshift 3\nexec \"$@\"\n"
	writeUIDIsolationScript(t, runner, runnerBody, os.ModeSetuid|0o700)
	policy := canaryUIDIsolationPolicy{
		runnerPath: runner, envPath: environment, user: productionUser,
		expectedUID: uid, runnerOwnerUID: uid,
	}
	return &canaryUIDIsolation{
		root: root, home: filepath.Join(root, "home"), temporary: filepath.Join(root, "tmp"), policy: policy,
	}, omp
}

func TestValidateCanaryUIDIsolationRejectsUnsafeInputs(t *testing.T) {
	isolation, omp := testCanaryUIDIsolation(t)
	artifact := linkTestArtifact(t, "auto")
	if _, err := validateCanaryUIDIsolation(isolation.root, omp, artifact, isolation.policy); err != nil {
		t.Fatalf("validateCanaryUIDIsolation() error = %v", err)
	}
	missingRunner := isolation.policy
	missingRunner.runnerPath = filepath.Join(t.TempDir(), "missing-sudo")
	if _, err := validateCanaryUIDIsolation(isolation.root, omp, artifact, missingRunner); err == nil {
		t.Fatal("missing sudo runner passed isolation validation")
	}
	if err := os.Chmod(isolation.root, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := validateCanaryUIDIsolation(isolation.root, omp, artifact, isolation.policy); err == nil {
		t.Fatal("unsafe canary root mode passed isolation validation")
	}
	if err := os.Chmod(isolation.root, 0o755); err != nil {
		t.Fatal(err)
	}
	wrongOwner := isolation.policy
	wrongOwner.runnerOwnerUID++
	if _, err := validateCanaryUIDIsolation(isolation.root, omp, artifact, wrongOwner); err == nil {
		t.Fatal("wrong canary root owner passed isolation validation")
	}
	if err := os.Chmod(omp, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := validateCanaryUIDIsolation(isolation.root, omp, artifact, isolation.policy); err == nil {
		t.Fatal("writable canary OMP passed isolation validation")
	}
}

func TestProductionCanaryUIDIsolationPolicyUsesPinnedRunner(t *testing.T) {
	policy, err := productionCanaryUIDIsolationPolicy()
	if err != nil {
		t.Fatalf("productionCanaryUIDIsolationPolicy() error = %v", err)
	}
	if policy.runnerPath != productionSudoPath || policy.envPath != productionEnvPath ||
		policy.user != productionUser || !policy.requireProductionRunner || !policy.requireArtifactOtherExec {
		t.Fatalf("productionCanaryUIDIsolationPolicy() = %#v", policy)
	}
}

func TestValidateArtifactUIDAccessRejectsTargetOwnerAndWritablePaths(t *testing.T) {
	uid, err := canaryFileUID(mustLstat(t, testArtifact(t)))
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(".", ".uid-artifact-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(root, "auto")
	body, err := os.ReadFile(testArtifact(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact, body, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(artifact, 0o755); err != nil {
		t.Fatal(err)
	}
	targetUID := uid + 1
	if err := validateArtifactUIDAccess(artifact, uid, nil, uid); err == nil {
		t.Fatal("target-owned artifact passed access validation")
	}
	if err := os.Chmod(root, 0o757); err != nil {
		t.Fatal(err)
	}
	if !canaryTargetMode(mustLstat(t, root), targetUID, nil, 0o200, 0o020, 0o002) {
		t.Fatal("target-writable parent mode was not detected")
	}
	if err := validateArtifactUIDAccess(artifact, targetUID, nil, uid); err == nil {
		t.Fatal("target-writable artifact parent passed access validation")
	}
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(artifact, 0o757); err != nil {
		t.Fatal(err)
	}
	if !canaryTargetMode(mustLstat(t, artifact), targetUID, nil, 0o200, 0o020, 0o002) {
		t.Fatal("target-writable artifact mode was not detected")
	}
	if err := validateArtifactUIDAccess(artifact, targetUID, nil, uid); err == nil {
		t.Fatal("target-writable artifact passed access validation")
	}
}

func TestValidateArtifactUIDAccessAcceptsTrustedStickyAncestor(t *testing.T) {
	uid, gid, err := canaryFileIdentity(mustLstat(t, testArtifact(t)))
	if err != nil {
		t.Fatal(err)
	}
	ancestor, err := os.MkdirTemp(".", ".uid-sticky-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(ancestor) })
	if err := os.Chmod(ancestor, 0o777|os.ModeSticky); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(ancestor, "root")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(root, "auto")
	body, err := os.ReadFile(testArtifact(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact, body, 0o555); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(artifact, 0o555); err != nil {
		t.Fatal(err)
	}
	targetUID := uid + 1
	targetGIDs := map[uint32]struct{}{gid: {}}
	if err := validateArtifactUIDAccess(artifact, targetUID, targetGIDs, uid+2); err == nil {
		t.Fatal("sticky ancestor owned by an untrusted identity passed access validation")
	}
	if err := validateArtifactUIDAccess(artifact, targetUID, targetGIDs, uid); err != nil {
		t.Fatalf("trusted sticky ancestor failed access validation: %v", err)
	}
	if err := os.Chmod(ancestor, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := validateArtifactUIDAccess(artifact, targetUID, targetGIDs, uid); err == nil {
		t.Fatal("trusted writable ancestor without sticky mode passed access validation")
	}
}

func TestValidateArtifactAncestorUIDAccessRejectsTargetOwnedReadOnlyDirectory(t *testing.T) {
	uid, err := canaryFileUID(mustLstat(t, testArtifact(t)))
	if err != nil {
		t.Fatal(err)
	}
	ancestor := t.TempDir()
	if err := os.Chmod(ancestor, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ancestor, 0o700) })
	if err := validateArtifactAncestorUIDAccess(ancestor, uid, nil, uid); err == nil {
		t.Fatal("target-owned 0555 artifact ancestor passed access validation")
	}
}

func TestNewCanaryCommandUsesDirectEmptyEnvironmentRunner(t *testing.T) {
	isolation, _ := testCanaryUIDIsolation(t)
	command, err := newCanaryCommand(context.Background(), "/public/auto", isolation, "version", "--short")
	if err != nil {
		t.Fatalf("newCanaryCommand() error = %v", err)
	}
	want := []string{isolation.policy.runnerPath, "-n", "-u", productionUser,
		isolation.policy.envPath, "-i", "HOME=" + isolation.home, "TMPDIR=" + isolation.temporary,
		"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "/public/auto", "version", "--short"}
	if command.Path != isolation.policy.runnerPath || !reflect.DeepEqual(command.Args, want) {
		t.Fatalf("newCanaryCommand() = (%q, %q), want (%q, %q)",
			command.Path, command.Args, isolation.policy.runnerPath, want)
	}
}

func writeUIDIsolationScript(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("os.Chmod() error = %v", err)
	}
}

func mustLstat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("os.Lstat() error = %v", err)
	}
	return info
}
