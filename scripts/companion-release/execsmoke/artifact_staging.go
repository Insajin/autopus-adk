package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	productionInstallPath = "/usr/bin/install"
	productionRemovePath  = "/bin/rm"
)

type stagedSignedArtifact struct {
	source, path, digest string
	info                 os.FileInfo
	policy               canaryUIDIsolationPolicy
}

func stageSignedArtifact(source string, isolation *canaryUIDIsolation) (*stagedSignedArtifact, error) {
	if isolation == nil || source == "" || !filepath.IsAbs(source) || filepath.Clean(source) != source {
		return nil, errors.New("signed artifact execution staging inputs are invalid")
	}
	destination := filepath.Join(isolation.root, "auto")
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		return nil, errors.New("signed artifact execution staging path already exists or is unavailable")
	}
	digest, _, err := stableSignedArtifactSHA256(source)
	if err != nil {
		return nil, err
	}
	if isolation.policy.requireProductionRunner {
		err = newSignedArtifactInstallCommand(source, destination, isolation.policy).Run()
	} else {
		err = copySignedArtifact(source, destination)
	}
	if err != nil {
		cleanupErr := removeSignedArtifactPath(destination, isolation.policy)
		if cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
			return nil, fmt.Errorf("signed artifact execution staging failed; cleanup failed: %v", cleanupErr)
		}
		return nil, errors.New("signed artifact execution staging failed")
	}

	stagedDigest, info, validationErr := stableSignedArtifactSHA256(destination)
	if validationErr == nil {
		validationErr = validateCanaryPath(destination, false, 0o555, isolation.policy.runnerOwnerUID)
	}
	if validationErr == nil && stagedDigest != digest {
		validationErr = errors.New("signed artifact execution staging digest differs")
	}
	if validationErr != nil {
		cleanupErr := removeSignedArtifactPath(destination, isolation.policy)
		if cleanupErr != nil {
			return nil, fmt.Errorf("signed artifact execution staging identity differs; cleanup failed: %v", cleanupErr)
		}
		return nil, errors.New("signed artifact execution staging identity differs")
	}
	staged := &stagedSignedArtifact{
		source: source, path: destination, digest: digest, info: info, policy: isolation.policy,
	}
	if err := staged.verify(); err != nil {
		return nil, staged.finish(err)
	}
	return staged, nil
}

func copySignedArtifact(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o555)
	if err != nil {
		_ = input.Close()
		return err
	}
	_, copyErr := io.Copy(output, input)
	inputCloseErr := input.Close()
	outputCloseErr := output.Close()
	if copyErr != nil || inputCloseErr != nil || outputCloseErr != nil {
		_ = os.Remove(destination)
		return errors.New("copy signed artifact")
	}
	if err := os.Chmod(destination, 0o555); err != nil {
		_ = os.Remove(destination)
		return err
	}
	return nil
}

func stableSignedArtifactSHA256(path string) (string, os.FileInfo, error) {
	before, err := os.Lstat(path)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return "", nil, errors.New("signed artifact identity is unsafe")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", nil, errors.New("signed artifact is unreadable")
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	opened, statErr := file.Stat()
	closeErr := file.Close()
	after, pathStatErr := os.Lstat(path)
	if copyErr != nil || statErr != nil || closeErr != nil || pathStatErr != nil ||
		!os.SameFile(before, opened) || !os.SameFile(opened, after) || before.Size() != after.Size() ||
		before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
		return "", nil, errors.New("signed artifact changed while hashing")
	}
	return "sha256:" + fmtDigest(hash.Sum(nil)), after, nil
}

func newSignedArtifactInstallCommand(source, destination string, policy canaryUIDIsolationPolicy) *exec.Cmd {
	return exec.Command(policy.runnerPath, "-n", productionInstallPath,
		"-m", "0555", "-o", "root", "-g", "wheel", source, destination)
}

func removeSignedArtifactPath(path string, policy canaryUIDIsolationPolicy) error {
	if policy.requireProductionRunner {
		if err := exec.Command(policy.runnerPath, "-n", productionRemovePath, "-f", "--", path).Run(); err != nil {
			return errors.New("remove staged signed artifact")
		}
		return nil
	}
	return os.Remove(path)
}

func (staged *stagedSignedArtifact) verify() error {
	sourceDigest, _, sourceErr := stableSignedArtifactSHA256(staged.source)
	stagedDigest, currentInfo, stagedErr := stableSignedArtifactSHA256(staged.path)
	if sourceErr != nil || stagedErr != nil || sourceDigest != staged.digest || stagedDigest != staged.digest ||
		!os.SameFile(staged.info, currentInfo) ||
		validateCanaryPath(staged.path, false, 0o555, staged.policy.runnerOwnerUID) != nil {
		return errors.New("staged signed artifact changed during execution smoke")
	}
	return nil
}

func (staged *stagedSignedArtifact) finish(smokeErr error) error {
	verificationErr := staged.verify()
	cleanupErr := removeSignedArtifactPath(staged.path, staged.policy)
	if smokeErr != nil {
		if verificationErr != nil || cleanupErr != nil {
			return fmt.Errorf("%w; staged artifact verification or cleanup failed", smokeErr)
		}
		return smokeErr
	}
	if verificationErr != nil {
		if cleanupErr != nil {
			return fmt.Errorf("%w; cleanup failed", verificationErr)
		}
		return verificationErr
	}
	if cleanupErr != nil {
		return errors.New("signed artifact execution staging cleanup failed")
	}
	if _, err := os.Lstat(staged.path); !os.IsNotExist(err) {
		return errors.New("signed artifact execution staging cleanup is incomplete")
	}
	return nil
}
