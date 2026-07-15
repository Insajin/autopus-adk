package design

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ErrVisualGateBundleUncommitted means v2 cannot be trusted as evidence for the current v1 bytes.
var ErrVisualGateBundleUncommitted = errors.New("visual gate bundle is not committed")

const maxVisualReportBytes int64 = 64 << 20

// ReadVisualGateReportBundle returns only a v2 report bound to the exact current v1 bytes.
func ReadVisualGateReportBundle(root string) (VisualGateReport, VisualGateReportV2, error) {
	rootFS, _, err := openVisualReportRoot(root)
	if err != nil {
		return VisualGateReport{}, VisualGateReportV2{}, err
	}
	defer func() { _ = rootFS.Close() }()
	if err := validateVisualReportDir(rootFS); err != nil {
		return VisualGateReport{}, VisualGateReportV2{}, uncommittedVisualBundleError("validate report directory", err)
	}
	legacyRaw, err := readVisualReportFile(rootFS, visualReportV1Path)
	if err != nil {
		return VisualGateReport{}, VisualGateReportV2{}, uncommittedVisualBundleError("read v1", err)
	}
	var legacy VisualGateReport
	if err := json.Unmarshal(legacyRaw, &legacy); err != nil {
		return VisualGateReport{}, VisualGateReportV2{}, uncommittedVisualBundleError("decode v1", err)
	}
	if legacy.Version != 1 {
		return legacy, VisualGateReportV2{}, uncommittedVisualBundleError("validate v1", fmt.Errorf("unexpected version %d", legacy.Version))
	}
	evidenceRaw, err := readVisualReportFile(rootFS, visualReportV2Path)
	if err != nil {
		return legacy, VisualGateReportV2{}, uncommittedVisualBundleError("read v2", err)
	}
	var evidence VisualGateReportV2
	if err := json.Unmarshal(evidenceRaw, &evidence); err != nil {
		return legacy, VisualGateReportV2{}, uncommittedVisualBundleError("decode v2", err)
	}
	if evidence.Version != 2 {
		return legacy, VisualGateReportV2{}, uncommittedVisualBundleError("validate v2", fmt.Errorf("unexpected version %d", evidence.Version))
	}
	legacyConfirm, err := readVisualReportFile(rootFS, visualReportV1Path)
	if err != nil {
		return legacy, VisualGateReportV2{}, uncommittedVisualBundleError("confirm v1", err)
	}
	if !bytes.Equal(legacyRaw, legacyConfirm) {
		return legacy, VisualGateReportV2{}, uncommittedVisualBundleError("confirm v1", errors.New("v1 changed during read"))
	}
	if err := validateLegacyDigest(legacyRaw, evidence.LegacySHA256); err != nil {
		return legacy, VisualGateReportV2{}, uncommittedVisualBundleError("validate legacy_sha256", err)
	}
	return legacy, evidence, nil
}

func validateVisualReportDir(root *os.Root) error {
	for _, component := range []string{".autopus", ".autopus/design", visualReportDir} {
		info, err := root.Lstat(filepath.FromSlash(component))
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("visual report path contains symlink: %s", component)
		}
		if !info.IsDir() {
			return fmt.Errorf("visual report path component is not a directory: %s", component)
		}
	}
	return nil
}

func readVisualReportFile(root *os.Root, path string) ([]byte, error) {
	path = filepath.FromSlash(path)
	info, err := root.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("visual report is not a regular file")
	}
	if err := validateVisualReportFileSize(info); err != nil {
		return nil, err
	}
	file, err := root.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return nil, fmt.Errorf("visual report changed while opening")
	}
	if err := validateVisualReportFileSize(openedInfo); err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(io.LimitReader(file, maxVisualReportBytes+1))
	if err != nil {
		return nil, err
	}
	if err := validateVisualReportSize(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func validateVisualReportFileSize(info os.FileInfo) error {
	if info.Size() > maxVisualReportBytes {
		return fmt.Errorf("visual report size limit exceeded: %d > %d", info.Size(), maxVisualReportBytes)
	}
	return nil
}

func validateVisualReportSize(raw []byte) error {
	if int64(len(raw)) > maxVisualReportBytes {
		return fmt.Errorf("visual report size limit exceeded: %d > %d", len(raw), maxVisualReportBytes)
	}
	return nil
}

func validateLegacyDigest(raw []byte, encoded string) error {
	claimed, err := hex.DecodeString(encoded)
	if err != nil || len(claimed) != sha256.Size {
		return fmt.Errorf("invalid SHA-256 digest")
	}
	actual := sha256.Sum256(raw)
	if subtle.ConstantTimeCompare(claimed, actual[:]) != 1 {
		return fmt.Errorf("SHA-256 digest mismatch")
	}
	return nil
}

func uncommittedVisualBundleError(operation string, err error) error {
	return fmt.Errorf("%w: %s: %v", ErrVisualGateBundleUncommitted, operation, err)
}
