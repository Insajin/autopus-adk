package companionmanifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenSignedPairRecoveryRoot_DirectoryReplacementIsRejected(t *testing.T) {
	parent := t.TempDir()
	name := signedPairTransactionPrefix + "replacement-test"
	path := filepath.Join(parent, name)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Errorf("close signed-pair recovery test root: %v", err)
		}
	}()

	recoveryRoot, err := openSignedPairRecoveryRootWithHook(root, name, func() {
		if renameErr := os.Rename(path, path+".original"); renameErr != nil {
			t.Fatal(renameErr)
		}
		if mkdirErr := os.Mkdir(path, 0o700); mkdirErr != nil {
			t.Fatal(mkdirErr)
		}
	})
	if recoveryRoot != nil {
		_ = recoveryRoot.Close()
	}
	if err == nil {
		t.Fatal("replacement recovery directory was accepted")
	}
}
