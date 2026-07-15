package design

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
)

type preparedVisualJSON struct {
	root       visualReportRoot
	targetPath string
	tempPath   string
}

// WriteVisualGateReportBundle publishes v1 first and v2 as its digest-bound commit marker.
func WriteVisualGateReportBundle(root string, legacy VisualGateReport, evidence VisualGateReportV2) error {
	rootFS, _, err := openVisualReportRootForWrite(root)
	if err != nil {
		return err
	}
	defer func() { _ = rootFS.Close() }()
	return writeVisualGateReportBundleRoot(rootFS, legacy, evidence)
}

func writeVisualGateReportBundleRoot(root visualReportRoot, legacy VisualGateReport, evidence VisualGateReportV2) error {
	if legacy.Version != 1 {
		return fmt.Errorf("visual gate v1 version must be 1, got %d", legacy.Version)
	}
	if evidence.Version != 2 {
		return fmt.Errorf("visual gate v2 version must be 2, got %d", evidence.Version)
	}
	if err := ensureVisualReportDir(root); err != nil {
		return err
	}
	legacyRaw, err := marshalVisualJSON(legacy)
	if err != nil {
		return fmt.Errorf("marshal visual gate v1: %w", err)
	}
	if err := validateVisualReportSize(legacyRaw); err != nil {
		return fmt.Errorf("marshal visual gate v1: %w", err)
	}
	digest := sha256.Sum256(legacyRaw)
	evidence.LegacySHA256 = hex.EncodeToString(digest[:])
	evidenceRaw, err := marshalVisualJSON(evidence)
	if err != nil {
		return fmt.Errorf("marshal visual gate v2: %w", err)
	}
	if err := validateVisualReportSize(evidenceRaw); err != nil {
		return fmt.Errorf("marshal visual gate v2: %w", err)
	}
	v1, err := prepareVisualJSON(root, visualReportV1Path, legacyRaw)
	if err != nil {
		return fmt.Errorf("write visual gate v1: %w", err)
	}
	defer v1.cleanup()
	v2, err := prepareVisualJSON(root, visualReportV2Path, evidenceRaw)
	if err != nil {
		return fmt.Errorf("write visual gate v2: %w", err)
	}
	defer v2.cleanup()
	if err := validateVisualReportTarget(root, visualReportV1Path); err != nil {
		return fmt.Errorf("validate visual gate v1: %w", err)
	}
	if err := validateVisualReportTarget(root, visualReportV2Path); err != nil {
		return fmt.Errorf("validate visual gate v2: %w", err)
	}
	if err := v1.publish(); err != nil {
		return fmt.Errorf("write visual gate v1: %w", err)
	}
	if err := v2.publish(); err != nil {
		return fmt.Errorf("write visual gate v2: %w", err)
	}
	return nil
}

func writeVisualGateReport(root string, report VisualGateReport) (string, error) {
	rootFS, _, err := openVisualReportRootForWrite(root)
	if err != nil {
		return "", err
	}
	defer func() { _ = rootFS.Close() }()
	if err := ensureVisualReportDir(rootFS); err != nil {
		return "", err
	}
	raw, err := marshalVisualJSON(report)
	if err != nil {
		return "", err
	}
	if err := validateVisualReportSize(raw); err != nil {
		return "", err
	}
	prepared, err := prepareVisualJSON(rootFS, visualReportV1Path, raw)
	if err != nil {
		return "", err
	}
	defer prepared.cleanup()
	if err := prepared.publish(); err != nil {
		return "", err
	}
	return filepath.Join(root, filepath.FromSlash(visualReportV1Path)), nil
}

func marshalVisualJSON(value any) ([]byte, error) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func prepareVisualJSON(root visualReportRoot, path string, raw []byte) (_ *preparedVisualJSON, resultErr error) {
	path = filepath.FromSlash(path)
	if err := validateVisualReportTarget(root, path); err != nil {
		return nil, err
	}
	temp, tempPath, err := createVisualReportTemp(root)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = temp.Close()
		if resultErr != nil {
			_ = root.Remove(tempPath)
		}
	}()
	if _, err := temp.Write(raw); err != nil {
		return nil, err
	}
	if err := temp.Sync(); err != nil {
		return nil, err
	}
	if err := temp.Close(); err != nil {
		return nil, err
	}
	return &preparedVisualJSON{root: root, targetPath: path, tempPath: tempPath}, nil
}

func (prepared *preparedVisualJSON) publish() error {
	if err := validateVisualReportTarget(prepared.root, prepared.targetPath); err != nil {
		return err
	}
	if err := prepared.root.Rename(prepared.tempPath, prepared.targetPath); err != nil {
		return err
	}
	prepared.tempPath = ""
	return nil
}

func (prepared *preparedVisualJSON) cleanup() {
	if prepared != nil && prepared.tempPath != "" {
		_ = prepared.root.Remove(prepared.tempPath)
		prepared.tempPath = ""
	}
}
