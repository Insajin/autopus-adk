//go:build darwin || linux

package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/companionmanifest"
)

func executePublicKeyReceiptBundleCommand(
	t *testing.T,
	keyPath, bundlePath string,
	faultHook func(string) error,
) publicKeyReceiptCommandResult {
	t.Helper()
	command := newCompanionPublicKeyReceiptCmdWithFaultHook(faultHook)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{
		"--key-file", keyPath,
		"--bundle-output", bundlePath,
		"--key-id", "rfc8032-vector-1",
		"--issued-at", "2026-07-14T00:00:00Z",
		"--expires-at", "2026-07-21T00:00:00Z",
		"--handoff", "v1",
		"--minimum-rollback-floor", "5069",
	})
	err := command.Execute()
	return publicKeyReceiptCommandResult{
		stdout: stdout.String(), stderr: stderr.String(), err: err,
	}
}

func newReceiptBundleFixture(t *testing.T) (string, string, []byte) {
	t.Helper()
	root := t.TempDir()
	keyPath, privateKey := writePublicKeyReceiptTestKey(t, root)
	outputParent := filepath.Join(root, "output")
	if err := os.Mkdir(outputParent, 0o700); err != nil {
		t.Fatal(err)
	}
	return keyPath, filepath.Join(outputParent, "receipt-v1.bundle"), privateKey
}

func assertReceiptBundleAbsent(t *testing.T, bundlePath string) {
	t.Helper()
	if _, err := os.Lstat(bundlePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("false bundle publication at %s: %v", bundlePath, err)
	}
}

func assertExactReceiptBundle(t *testing.T, bundlePath string) {
	t.Helper()
	entries, err := os.ReadDir(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name() != publicKeyReceiptEntryName ||
		entries[1].Name() != publicKeyReceiptSignatureEntryName {
		t.Fatalf("bundle entries = %v", entries)
	}
	bundleInfo, err := os.Lstat(bundlePath)
	if err != nil || !bundleInfo.IsDir() || bundleInfo.Mode().Perm() != 0o700 {
		t.Fatalf("bundle mode/type = %v, error = %v", bundleInfo, err)
	}
	receiptBytes, err := os.ReadFile(filepath.Join(bundlePath, publicKeyReceiptEntryName))
	if err != nil {
		t.Fatal(err)
	}
	signature, err := os.ReadFile(filepath.Join(bundlePath, publicKeyReceiptSignatureEntryName))
	if err != nil {
		t.Fatal(err)
	}
	policy := companionmanifest.PublicKeyReceiptPolicy{
		Now:           time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC),
		ExpectedKeyID: "rfc8032-vector-1", ExpectedHandoff: "v1",
		MinimumRollbackFloor: 5069,
	}
	if err := companionmanifest.CheckPublicKeyReceiptSelfConsistency(
		receiptBytes,
		signature,
		policy,
	); err != nil {
		t.Fatalf("published bundle self-check: %v", err)
	}
	for _, entry := range entries {
		info, statErr := entry.Info()
		if statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
			t.Fatalf("entry %s mode/type = %v, error = %v", entry.Name(), info, statErr)
		}
	}
}

func TestCompanionPublicKeyReceiptBundle_ValidInput_PublishesOneExactBundle(t *testing.T) {
	keyPath, bundlePath, privateKey := newReceiptBundleFixture(t)
	result := executePublicKeyReceiptBundleCommand(t, keyPath, bundlePath, nil)
	if result.err != nil {
		t.Fatal(result.err)
	}
	assertExactReceiptBundle(t, bundlePath)
	assertPublicKeyReceiptCommandNoLeak(t, result, privateKey, "")
}

func TestCompanionPublicKeyReceiptBundle_ExistingOrWritableNamespace_PreservesBytes(t *testing.T) {
	for _, scenario := range []string{"existing", "writable parent"} {
		t.Run(scenario, func(t *testing.T) {
			keyPath, bundlePath, privateKey := newReceiptBundleFixture(t)
			parent := filepath.Dir(bundlePath)
			protected := filepath.Join(parent, "protected")
			if err := os.WriteFile(protected, []byte("original-bytes"), 0o600); err != nil {
				t.Fatal(err)
			}
			if scenario == "existing" {
				if err := os.Mkdir(bundlePath, 0o700); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Chmod(parent, 0o770); err != nil {
				t.Fatal(err)
			}
			result := executePublicKeyReceiptBundleCommand(t, keyPath, bundlePath, nil)
			got, err := os.ReadFile(protected)
			if result.err == nil || err != nil || string(got) != "original-bytes" {
				t.Fatalf("result/protected = %v / %q / %v", result.err, got, err)
			}
			assertPublicKeyReceiptCommandNoLeak(t, result, privateKey, "")
		})
	}
}

func TestCompanionPublicKeyReceiptBundle_InjectedFault_RollsBackOwnedPublication(t *testing.T) {
	steps := []string{
		"key_opened", "receipt_file_synced", "signature_file_synced",
		"staging_dir_synced", "publish_ready", "bundle_published", "parent_dir_synced",
	}
	for _, failStep := range steps {
		t.Run(failStep, func(t *testing.T) {
			keyPath, bundlePath, privateKey := newReceiptBundleFixture(t)
			seen := false
			result := executePublicKeyReceiptBundleCommand(t, keyPath, bundlePath, func(step string) error {
				if step == failStep {
					seen = true
					return errors.New("injected bundle fault")
				}
				return nil
			})
			if result.err == nil || !seen {
				t.Fatalf("result/seen = %v / %v", result.err, seen)
			}
			assertReceiptBundleAbsent(t, bundlePath)
			entries, err := os.ReadDir(filepath.Dir(bundlePath))
			if err != nil || len(entries) != 0 {
				t.Fatalf("rollback leftovers = %v, error = %v", entries, err)
			}
			assertPublicKeyReceiptCommandNoLeak(t, result, privateKey, "")
		})
	}
}

func TestCompanionPublicKeyReceiptBundle_ParentReplacement_FailsClosed(t *testing.T) {
	keyPath, bundlePath, _ := newReceiptBundleFixture(t)
	parent := filepath.Dir(bundlePath)
	originalParent := parent + ".original"
	replaced := false
	result := executePublicKeyReceiptBundleCommand(t, keyPath, bundlePath, func(step string) error {
		if step != "staging_dir_synced" {
			return nil
		}
		if err := os.Rename(parent, originalParent); err != nil {
			return err
		}
		if err := os.Mkdir(parent, 0o700); err != nil {
			return err
		}
		replaced = true
		return nil
	})
	if result.err == nil || !replaced {
		t.Fatalf("replacement result/attempted = %v / %v", result.err, replaced)
	}
	assertReceiptBundleAbsent(t, bundlePath)
	assertReceiptBundleAbsent(t, filepath.Join(originalParent, filepath.Base(bundlePath)))
}

func TestCompanionPublicKeyReceiptBundle_PostPublishTamper_FailsAndUnpublishes(t *testing.T) {
	keyPath, bundlePath, _ := newReceiptBundleFixture(t)
	tampered := false
	result := executePublicKeyReceiptBundleCommand(t, keyPath, bundlePath, func(step string) error {
		if step != "bundle_published" {
			return nil
		}
		path := filepath.Join(bundlePath, publicKeyReceiptEntryName)
		if err := os.WriteFile(path, []byte(strings.Repeat("x", 64)), 0o600); err != nil {
			return err
		}
		tampered = true
		return nil
	})
	if result.err == nil || !tampered {
		t.Fatalf("tamper result/attempted = %v / %v", result.err, tampered)
	}
	assertReceiptBundleAbsent(t, bundlePath)
}

func TestCompanionPublicKeyReceiptBundle_StagingSubstitution_FailsBeforePublication(t *testing.T) {
	keyPath, bundlePath, _ := newReceiptBundleFixture(t)
	parent := filepath.Dir(bundlePath)
	var originalReceipt []byte
	substituted := false
	result := executePublicKeyReceiptBundleCommand(t, keyPath, bundlePath, func(step string) error {
		if step != "before_publish" {
			return nil
		}
		entries, err := os.ReadDir(parent)
		if err != nil {
			return err
		}
		var stagePath string
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), publicKeyReceiptStagePrefix) {
				stagePath = filepath.Join(parent, entry.Name())
				break
			}
		}
		if stagePath == "" {
			return errors.New("staging directory not observed")
		}
		originalReceipt, err = os.ReadFile(filepath.Join(stagePath, publicKeyReceiptEntryName))
		if err != nil {
			return err
		}
		if err := os.Rename(stagePath, stagePath+".saved"); err != nil {
			return err
		}
		if err := os.Mkdir(stagePath, 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(
			filepath.Join(stagePath, publicKeyReceiptEntryName),
			[]byte("attacker-receipt"),
			0o600,
		); err != nil {
			return err
		}
		substituted = true
		return nil
	})
	if result.err == nil || !substituted {
		t.Fatalf("substitution result/attempted = %v / %v", result.err, substituted)
	}
	assertReceiptBundleAbsent(t, bundlePath)
	if len(originalReceipt) == 0 || bytes.Equal(originalReceipt, []byte("attacker-receipt")) {
		t.Fatal("original staged receipt was not preserved")
	}
}
