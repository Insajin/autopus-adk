package companionmanifest

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func resolveSignedPairPaths(manifestPath, signaturePath string) (string, string, string, error) {
	manifestAbs, err := filepath.Abs(filepath.Clean(manifestPath))
	if err != nil {
		return "", "", "", errors.New("resolve manifest output")
	}
	signatureAbs, err := filepath.Abs(filepath.Clean(signaturePath))
	if err != nil {
		return "", "", "", errors.New("resolve signature output")
	}
	manifestName, signatureName := filepath.Base(manifestAbs), filepath.Base(signatureAbs)
	if manifestAbs == signatureAbs || filepath.Dir(manifestAbs) != filepath.Dir(signatureAbs) ||
		!validSignedPairOutputName(manifestName) ||
		!validSignedPairOutputName(signatureName) {
		return "", "", "", errors.New("signed outputs must be distinct files in one directory")
	}
	return filepath.Dir(manifestAbs), manifestName, signatureName, nil
}

func validSignedPairOutputName(name string) bool {
	return name != "" && name != "." && filepath.Base(name) == name &&
		!strings.HasPrefix(name, signedPairTransactionPrefix) &&
		!strings.HasPrefix(name, signedPairCleanupPrefix)
}

func openSignedPairRoot(path string) (*os.Root, os.FileInfo, error) {
	pathInfo, err := os.Lstat(path)
	if err != nil || !pathInfo.IsDir() || pathInfo.Mode()&os.ModeSymlink != 0 {
		return nil, nil, errors.New("signed output directory must be a regular directory")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, nil, errors.New("open signed output directory")
	}
	rootInfo, err := root.Stat(".")
	if err != nil || !os.SameFile(pathInfo, rootInfo) {
		closeErr := root.Close()
		return nil, nil, errors.Join(
			errors.New("signed output directory identity changed"),
			closeErr,
		)
	}
	return root, rootInfo, nil
}

func inspectSignedPairOutput(root *os.Root, name string) (bool, os.FileInfo, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil, nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return false, nil, errors.New("signed output must be a regular file")
	}
	return true, info, nil
}

func createSignedPairTransactionDirectory(
	root *os.Root,
	state signedPairState,
) (string, error) {
	digest := sha256.Sum256(
		[]byte(state.ManifestName + "\x00" + state.SignatureName),
	)
	name := signedPairTransactionPrefix + hex.EncodeToString(digest[:16])
	if err := root.Mkdir(name, 0o700); err != nil {
		if errors.Is(err, os.ErrExist) {
			return "", errors.New("signed pair transaction already exists")
		}
		return "", errors.New("create signed pair transaction directory")
	}
	return name, nil
}

func detachSignedPairTransactionDirectory(root *os.Root, name string) (string, error) {
	for attempt := 0; attempt < 16; attempt++ {
		random := make([]byte, 16)
		if _, err := rand.Read(random); err != nil {
			return "", errors.New("generate signed pair cleanup name")
		}
		cleanupName := signedPairCleanupPrefix + hex.EncodeToString(random)
		if _, err := root.Lstat(cleanupName); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", errors.New("inspect signed pair cleanup path")
		}
		if err := root.Rename(name, cleanupName); err != nil {
			return "", errors.New("detach signed pair transaction directory")
		}
		return cleanupName, nil
	}
	return "", errors.New("allocate signed pair cleanup path")
}

func writeSignedPairRootFile(root *os.Root, name string, data []byte) error {
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	writeErr := writeAll(file, data)
	syncErr := file.Sync()
	closeErr := file.Close()
	return errors.Join(writeErr, syncErr, closeErr)
}

func verifySignedPairOutput(root *os.Root, name string, want []byte) error {
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return errors.New("published signed output is not a secure regular file")
	}
	got, err := root.ReadFile(name)
	if err != nil || !bytes.Equal(got, want) {
		return errors.New("published signed output bytes changed")
	}
	return nil
}

func runSignedPairFault(hook func(string) error, step string) error {
	if hook != nil && hook(step) != nil {
		return errors.New("signed pair transaction fault")
	}
	return nil
}
