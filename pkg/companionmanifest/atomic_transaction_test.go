package companionmanifest

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteSignedFiles_SecondPublishFailure_RestoresExistingPair(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	signaturePath := filepath.Join(dir, "manifest.sig")
	writeSignedPairFixture(t, manifestPath, signaturePath, "old-manifest", "old-signature")

	err := writeSignedFilesWithFault(
		manifestPath,
		signaturePath,
		[]byte("new-manifest"),
		[]byte("new-signature"),
		func(step string) error {
			if step == "manifest_published" {
				return errors.New("injected second publish failure")
			}
			return nil
		},
	)
	if err == nil {
		t.Fatal("writeSignedFilesWithFault() error = nil, want rollback")
	}
	assertSignedPair(t, manifestPath, signaturePath, "old-manifest", "old-signature")
	assertNoSignedPairTransactions(t, dir)
}

func TestWriteSignedFiles_BeforePreparedFault_ReleasesTransactionForRetry(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	signaturePath := filepath.Join(dir, "manifest.sig")
	writeSignedPairFixture(t, manifestPath, signaturePath, "old-manifest", "old-signature")
	err := writeSignedFilesWithFault(
		manifestPath,
		signaturePath,
		[]byte("failed-manifest"),
		[]byte("failed-signature"),
		func(step string) error {
			if step == "before_prepared" {
				return errors.New("injected preparation fault")
			}
			return nil
		},
	)
	if err == nil {
		t.Fatal("preparation fault was ignored")
	}
	assertSignedPair(t, manifestPath, signaturePath, "old-manifest", "old-signature")
	assertNoSignedPairTransactions(t, dir)
	if err := WriteSignedFiles(
		manifestPath,
		signaturePath,
		[]byte("retry-manifest"),
		[]byte("retry-signature"),
	); err != nil {
		t.Fatalf("retry WriteSignedFiles() error = %v", err)
	}
	assertSignedPair(t, manifestPath, signaturePath, "retry-manifest", "retry-signature")
}

func TestWriteSignedFiles_ParentReplacement_FailsWithoutSplittingPair(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not permit renaming an open transaction directory")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "release")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	signaturePath := filepath.Join(dir, "manifest.sig")
	writeSignedPairFixture(t, manifestPath, signaturePath, "old-manifest", "old-signature")
	movedDir := filepath.Join(root, "release.original")

	err := writeSignedFilesWithFault(
		manifestPath,
		signaturePath,
		[]byte("new-manifest"),
		[]byte("new-signature"),
		func(step string) error {
			if step != "staged" {
				return nil
			}
			if err := os.Rename(dir, movedDir); err != nil {
				return err
			}
			return os.Mkdir(dir, 0o700)
		},
	)
	if err == nil {
		t.Fatal("writeSignedFilesWithFault() error = nil, want parent replacement rejection")
	}
	assertSignedPair(
		t,
		filepath.Join(movedDir, "manifest.json"),
		filepath.Join(movedDir, "manifest.sig"),
		"old-manifest",
		"old-signature",
	)
	for _, path := range []string{manifestPath, signaturePath} {
		if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("replacement parent output %q stat error = %v, want absent", path, statErr)
		}
	}
	assertNoSignedPairTransactions(t, movedDir)
}

func TestWriteSignedFiles_CrashBoundaries_AreRecoveredOnRestart(t *testing.T) {
	tests := []struct {
		step          string
		wantManifest  string
		wantSignature string
	}{
		{step: "before_prepared", wantManifest: "old-manifest", wantSignature: "old-signature"},
		{step: "manifest_published", wantManifest: "old-manifest", wantSignature: "old-signature"},
		{step: "pair_committed", wantManifest: "new-manifest", wantSignature: "new-signature"},
		{step: "transaction_detached", wantManifest: "new-manifest", wantSignature: "new-signature"},
	}
	for _, test := range tests {
		t.Run(test.step, func(t *testing.T) {
			dir := t.TempDir()
			manifestPath := filepath.Join(dir, "manifest.json")
			signaturePath := filepath.Join(dir, "manifest.sig")
			writeSignedPairFixture(t, manifestPath, signaturePath, "old-manifest", "old-signature")
			command := exec.Command(os.Args[0], "-test.run=TestWriteSignedFilesCrashHelper$")
			command.Env = append(
				os.Environ(),
				"GO_WANT_SIGNED_PAIR_CRASH=1",
				"SIGNED_PAIR_CRASH_STEP="+test.step,
				"SIGNED_PAIR_CRASH_MANIFEST="+manifestPath,
				"SIGNED_PAIR_CRASH_SIGNATURE="+signaturePath,
			)
			err := command.Run()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 91 {
				t.Fatalf("crash helper result = %v", err)
			}

			if err := recoverSignedFileTransactions(dir); err != nil {
				t.Fatalf("recoverSignedFileTransactions() error = %v", err)
			}
			assertSignedPair(
				t,
				manifestPath,
				signaturePath,
				test.wantManifest,
				test.wantSignature,
			)
			assertNoSignedPairTransactions(t, dir)
		})
	}
}

func TestWriteSignedFilesCrashHelper(t *testing.T) {
	if os.Getenv("GO_WANT_SIGNED_PAIR_CRASH") != "1" {
		return
	}
	err := writeSignedFilesWithFault(
		os.Getenv("SIGNED_PAIR_CRASH_MANIFEST"),
		os.Getenv("SIGNED_PAIR_CRASH_SIGNATURE"),
		[]byte("new-manifest"),
		[]byte("new-signature"),
		func(step string) error {
			if step == os.Getenv("SIGNED_PAIR_CRASH_STEP") {
				os.Exit(91)
			}
			return nil
		},
	)
	if err != nil {
		os.Exit(92)
	}
	os.Exit(0)
}

func writeSignedPairFixture(
	t *testing.T,
	manifestPath, signaturePath, manifest, signature string,
) {
	t.Helper()
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(signaturePath, []byte(signature), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertSignedPair(
	t *testing.T,
	manifestPath, signaturePath, manifest, signature string,
) {
	t.Helper()
	for path, want := range map[string]string{
		manifestPath:  manifest,
		signaturePath: signature,
	} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("ReadFile(%q) = %q, %v; want %q", path, got, err, want)
		}
	}
}

func assertNoSignedPairTransactions(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), signedPairTransactionPrefix) ||
			strings.HasPrefix(entry.Name(), signedPairCleanupPrefix) {
			t.Fatalf("transaction residue retained: %q", entry.Name())
		}
	}
}
