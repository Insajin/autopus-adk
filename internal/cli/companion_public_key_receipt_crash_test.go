//go:build darwin || linux

package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompanionPublicKeyReceiptBundle_CrashBoundaries_ExposeOnlyAbsentOrComplete(t *testing.T) {
	steps := []struct {
		name          string
		publishedMust bool
	}{
		{name: "key_opened"},
		{name: "receipt_file_synced"},
		{name: "signature_file_synced"},
		{name: "staging_dir_synced"},
		{name: "publish_ready"},
		{name: "bundle_published", publishedMust: true},
		{name: "parent_dir_synced", publishedMust: true},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			keyPath, bundlePath, _ := newReceiptBundleFixture(t)
			command := exec.Command(
				os.Args[0],
				"-test.run=TestCompanionPublicKeyReceiptBundleCrashHelper$",
			)
			command.Env = append(
				os.Environ(),
				"GO_WANT_PUBLIC_KEY_RECEIPT_CRASH=1",
				"PUBLIC_KEY_RECEIPT_CRASH_STEP="+step.name,
				"PUBLIC_KEY_RECEIPT_CRASH_KEY="+keyPath,
				"PUBLIC_KEY_RECEIPT_CRASH_BUNDLE="+bundlePath,
			)
			err := command.Run()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 91 {
				t.Fatalf("crash helper result = %v", err)
			}
			_, statErr := os.Lstat(bundlePath)
			if errors.Is(statErr, os.ErrNotExist) {
				if step.publishedMust {
					t.Fatal("post-publish crash lost the complete visible bundle")
				}
				if step.name == "key_opened" {
					assertNoPublicKeyReceiptStages(t, filepath.Dir(bundlePath))
					return
				}
				restart := executePublicKeyReceiptBundleCommand(
					t,
					keyPath,
					bundlePath,
					nil,
				)
				if restart.err != nil {
					t.Fatalf("restart recovery error = %v", restart.err)
				}
				assertExactReceiptBundle(t, bundlePath)
				assertNoPublicKeyReceiptStages(t, filepath.Dir(bundlePath))
				return
			}
			if statErr != nil {
				t.Fatal(statErr)
			}
			assertExactReceiptBundle(t, bundlePath)
		})
	}
}

func TestCompanionPublicKeyReceiptBundleCrashHelper(t *testing.T) {
	if os.Getenv("GO_WANT_PUBLIC_KEY_RECEIPT_CRASH") != "1" {
		return
	}
	crashStep := os.Getenv("PUBLIC_KEY_RECEIPT_CRASH_STEP")
	result := executePublicKeyReceiptBundleCommand(
		t,
		os.Getenv("PUBLIC_KEY_RECEIPT_CRASH_KEY"),
		os.Getenv("PUBLIC_KEY_RECEIPT_CRASH_BUNDLE"),
		func(step string) error {
			if step == crashStep {
				os.Exit(91)
			}
			return nil
		},
	)
	if result.err != nil {
		os.Exit(92)
	}
	os.Exit(0)
}

func TestCompanionPublicKeyReceiptBundle_ActiveStageIsNotRecovered(t *testing.T) {
	root := t.TempDir()
	firstOutput, err := resolvePublicKeyReceiptBundle(filepath.Join(root, "first.bundle"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := beginPublicKeyReceiptBundleTransaction(firstOutput)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := first.finish(); err != nil {
			t.Errorf("finish first transaction: %v", err)
		}
	}()
	secondOutput, err := resolvePublicKeyReceiptBundle(filepath.Join(root, "second.bundle"))
	if err != nil {
		t.Fatal(err)
	}
	if second, err := beginPublicKeyReceiptBundleTransaction(secondOutput); err == nil {
		_ = second.finish()
		t.Fatal("concurrent receipt transaction was accepted")
	}
	if !first.stageNameMatchesDescriptor() {
		t.Fatal("active receipt stage was removed or replaced")
	}
}

func assertNoPublicKeyReceiptStages(t *testing.T, parent string) {
	t.Helper()
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), publicKeyReceiptStagePrefix) {
			t.Fatalf("stale receipt stage retained: %q", entry.Name())
		}
	}
}
