//go:build darwin || linux

package companionmanifest

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteSignedFiles_ActiveProcessTransactionIsNotRecovered(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	signaturePath := filepath.Join(dir, "manifest.sig")
	readyPath := filepath.Join(dir, "child.ready")
	releasePath := filepath.Join(dir, "child.release")
	writeSignedPairFixture(t, manifestPath, signaturePath, "old-manifest", "old-signature")
	command := exec.Command(os.Args[0], "-test.run=TestWriteSignedFilesActiveProcessHelper$")
	command.Env = append(
		os.Environ(),
		"GO_WANT_ACTIVE_SIGNED_PAIR=1",
		"ACTIVE_SIGNED_PAIR_MANIFEST="+manifestPath,
		"ACTIVE_SIGNED_PAIR_SIGNATURE="+signaturePath,
		"ACTIVE_SIGNED_PAIR_BLOCK_STEP=staged",
		"ACTIVE_SIGNED_PAIR_READY="+readyPath,
		"ACTIVE_SIGNED_PAIR_RELEASE="+releasePath,
	)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.WriteFile(releasePath, []byte("release"), 0o600)
		if command.ProcessState == nil {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	}()
	waitForSignedPairTestPath(t, readyPath)

	parentErr := WriteSignedFiles(
		manifestPath,
		signaturePath,
		[]byte("parent-manifest"),
		[]byte("parent-signature"),
	)
	if err := os.WriteFile(releasePath, []byte("release"), 0o600); err != nil {
		t.Fatal(err)
	}
	childErr := command.Wait()
	if parentErr == nil {
		t.Fatal("second process recovered and replaced a live signed-pair transaction")
	}
	if childErr != nil {
		t.Fatalf("active transaction child error = %v", childErr)
	}
	assertSignedPair(t, manifestPath, signaturePath, "child-manifest", "child-signature")
	assertNoSignedPairTransactions(t, dir)
}

func TestWriteSignedFiles_DetachedCleanupCannotDeleteNextTransaction(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	signaturePath := filepath.Join(dir, "manifest.sig")
	readyPath := filepath.Join(dir, "child.ready")
	releasePath := filepath.Join(dir, "child.release")
	writeSignedPairFixture(t, manifestPath, signaturePath, "old-manifest", "old-signature")
	command := exec.Command(os.Args[0], "-test.run=TestWriteSignedFilesActiveProcessHelper$")
	command.Env = append(
		os.Environ(),
		"GO_WANT_ACTIVE_SIGNED_PAIR=1",
		"ACTIVE_SIGNED_PAIR_MANIFEST="+manifestPath,
		"ACTIVE_SIGNED_PAIR_SIGNATURE="+signaturePath,
		"ACTIVE_SIGNED_PAIR_BLOCK_STEP=transaction_detached",
		"ACTIVE_SIGNED_PAIR_READY="+readyPath,
		"ACTIVE_SIGNED_PAIR_RELEASE="+releasePath,
	)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.WriteFile(releasePath, []byte("release"), 0o600)
		if command.ProcessState == nil {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	}()
	waitForSignedPairTestPath(t, readyPath)

	parentErr := WriteSignedFiles(
		manifestPath,
		signaturePath,
		[]byte("parent-manifest"),
		[]byte("parent-signature"),
	)
	if err := os.WriteFile(releasePath, []byte("release"), 0o600); err != nil {
		t.Fatal(err)
	}
	childErr := command.Wait()
	if parentErr != nil {
		t.Fatalf("next transaction error = %v", parentErr)
	}
	if childErr != nil {
		t.Fatalf("detached cleanup child error = %v", childErr)
	}
	assertSignedPair(t, manifestPath, signaturePath, "parent-manifest", "parent-signature")
	assertNoSignedPairTransactions(t, dir)
}

func TestWriteSignedFilesActiveProcessHelper(t *testing.T) {
	if os.Getenv("GO_WANT_ACTIVE_SIGNED_PAIR") != "1" {
		return
	}
	err := writeSignedFilesWithFault(
		os.Getenv("ACTIVE_SIGNED_PAIR_MANIFEST"),
		os.Getenv("ACTIVE_SIGNED_PAIR_SIGNATURE"),
		[]byte("child-manifest"),
		[]byte("child-signature"),
		func(step string) error {
			if step != os.Getenv("ACTIVE_SIGNED_PAIR_BLOCK_STEP") {
				return nil
			}
			if err := os.WriteFile(
				os.Getenv("ACTIVE_SIGNED_PAIR_READY"),
				[]byte("ready"),
				0o600,
			); err != nil {
				return err
			}
			deadline := time.Now().Add(10 * time.Second)
			for time.Now().Before(deadline) {
				_, err := os.Lstat(os.Getenv("ACTIVE_SIGNED_PAIR_RELEASE"))
				if err == nil {
					return nil
				}
				if !errors.Is(err, os.ErrNotExist) {
					return err
				}
				time.Sleep(10 * time.Millisecond)
			}
			return errors.New("timed out waiting to release active transaction")
		},
	)
	if err != nil {
		os.Exit(92)
	}
	os.Exit(0)
}

func waitForSignedPairTestPath(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		_, err := os.Lstat(path)
		if err == nil {
			return
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q", path)
}
