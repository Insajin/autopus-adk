//go:build darwin || linux

package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestCompanionPublicKeyReceiptKey_UnsafeTypes_FailWithoutBlockingOrPublishing(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*testing.T, string, string) string
	}{
		{name: "mode", setup: func(t *testing.T, _ string, keyPath string) string {
			if err := os.Chmod(keyPath, 0o640); err != nil {
				t.Fatal(err)
			}
			return keyPath
		}},
		{name: "symlink", setup: func(t *testing.T, root, keyPath string) string {
			path := filepath.Join(root, "key-link")
			if err := os.Symlink(keyPath, path); err != nil {
				t.Fatal(err)
			}
			return path
		}},
		{name: "hardlink", setup: func(t *testing.T, root, keyPath string) string {
			path := filepath.Join(root, "key-hardlink")
			if err := os.Link(keyPath, path); err != nil {
				t.Fatal(err)
			}
			return path
		}},
		{name: "directory", setup: func(t *testing.T, root, _ string) string {
			path := filepath.Join(root, "key-directory")
			if err := os.Mkdir(path, 0o600); err != nil {
				t.Fatal(err)
			}
			return path
		}},
		{name: "fifo", setup: func(t *testing.T, root, _ string) string {
			path := filepath.Join(root, "key-fifo")
			if err := unix.Mkfifo(path, 0o600); err != nil {
				t.Fatal(err)
			}
			return path
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			keyPath, bundlePath, privateKey := newReceiptBundleFixture(t)
			keyPath = tc.setup(t, filepath.Dir(keyPath), keyPath)
			started := time.Now()
			result := executePublicKeyReceiptBundleCommand(t, keyPath, bundlePath, nil)
			if result.err == nil {
				t.Fatal("unsafe key was accepted")
			}
			if elapsed := time.Since(started); elapsed > time.Second {
				t.Fatalf("unsafe key open blocked for %s", elapsed)
			}
			assertReceiptBundleAbsent(t, bundlePath)
			assertPublicKeyReceiptCommandNoLeak(t, result, privateKey, "")
		})
	}
}

func TestCompanionPublicKeyReceiptKey_PathReplacementAfterOpen_FailsClosed(t *testing.T) {
	keyPath, bundlePath, privateKey := newReceiptBundleFixture(t)
	originalPath := keyPath + ".original"
	replaced := false
	result := executePublicKeyReceiptBundleCommand(t, keyPath, bundlePath, func(step string) error {
		if step != "key_opened" {
			return nil
		}
		if err := os.Rename(keyPath, originalPath); err != nil {
			return err
		}
		if err := os.WriteFile(keyPath, []byte("attacker-key"), 0o600); err != nil {
			return err
		}
		replaced = true
		return nil
	})
	if result.err == nil || !replaced {
		t.Fatalf("replacement result/attempted = %v / %v", result.err, replaced)
	}
	assertReceiptBundleAbsent(t, bundlePath)
	if got, err := os.ReadFile(originalPath); err != nil || len(got) == 0 {
		t.Fatalf("original key was not preserved: %v / %d", err, len(got))
	}
	assertPublicKeyReceiptCommandNoLeak(t, result, privateKey, "")
}

func TestCompanionPublicKeyReceiptKey_UnsupportedOutputCollision_FailsBeforeRead(t *testing.T) {
	keyPath, _, _ := newReceiptBundleFixture(t)
	result := executePublicKeyReceiptBundleCommand(t, keyPath, keyPath, nil)
	if result.err == nil {
		t.Fatalf("key/output collision result = %v", result.err)
	}
}
