package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/insajin/autopus-adk/pkg/config"
)

type workflowContextProductOverlay struct {
	mu            sync.Mutex
	path          string
	memoryMode    string
	activeBody    []byte
	activeHash    string
	activeInfo    fs.FileInfo
	effectiveMode string
}

func newWorkflowContextProductOverlay(
	runtimeRoot string,
	memoryMode string,
) (WorkflowContextOverlayController, string, error) {
	path, body, activeInfo, err := prepareWorkflowContextProductOverlay(runtimeRoot, memoryMode, true)
	if err != nil {
		return nil, "", err
	}
	return &workflowContextProductOverlay{
		path: path, memoryMode: memoryMode, activeBody: body,
		activeHash: workflowContextProductOverlayHash(body), activeInfo: activeInfo,
		effectiveMode: config.OMPContextHistoryActive,
	}, path, nil
}

func newWorkflowContextManagedManualCompactionOverlay(runtimeRoot, memoryMode string) (string, error) {
	path, _, _, err := prepareWorkflowContextProductOverlay(runtimeRoot, memoryMode, false)
	return path, err
}

func prepareWorkflowContextProductOverlay(
	runtimeRoot, memoryMode string,
	automaticCompaction bool,
) (string, []byte, fs.FileInfo, error) {
	if memoryMode != config.OMPContextMemoryOff && memoryMode != config.OMPContextMemoryShadow {
		return "", nil, nil, errors.New("product OMP overlay memory mode is invalid")
	}
	root, err := filepath.Abs(runtimeRoot)
	if err != nil {
		return "", nil, nil, fmt.Errorf("resolve product OMP overlay root: %w", err)
	}
	root = filepath.Clean(root)
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", nil, nil, errors.New("product OMP overlay root is unsafe")
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return "", nil, nil, fmt.Errorf("secure product OMP overlay root: %w", err)
	}
	path := filepath.Join(root, "context-product.yml")
	body := workflowContextProductOverlayBody(automaticCompaction)
	if err := writeWorkflowContextProductOverlay(path, body); err != nil {
		return "", nil, nil, err
	}
	activeInfo, err := os.Lstat(path)
	if err != nil {
		return "", nil, nil, fmt.Errorf("inspect product OMP overlay: %w", err)
	}
	return path, body, activeInfo, nil
}

func (overlay *workflowContextProductOverlay) Apply(
	ctx context.Context,
	request WorkflowContextOverlayRequest,
) (WorkflowContextOverlayReadback, error) {
	if ctx == nil {
		return WorkflowContextOverlayReadback{}, errors.New("product OMP overlay context is required")
	}
	select {
	case <-ctx.Done():
		return WorkflowContextOverlayReadback{}, ctx.Err()
	default:
	}
	if request.MemoryMode != overlay.memoryMode {
		return WorkflowContextOverlayReadback{}, errors.New("product OMP overlay memory mode changed")
	}
	overlay.mu.Lock()
	defer overlay.mu.Unlock()
	previous := overlay.effectiveMode
	switch request.HistoryMode {
	case config.OMPContextHistoryActive:
		if previous != config.OMPContextHistoryActive {
			return WorkflowContextOverlayReadback{}, errors.New("product OMP overlay cannot reactivate a rolled back session")
		}
		if err := overlay.verifyActive(); err != nil {
			return WorkflowContextOverlayReadback{}, err
		}
		return overlay.readback(request, config.OMPContextHistoryShadow, overlay.activeHash), nil
	case config.OMPContextHistoryShadow, config.OMPContextHistoryOff:
		body := workflowContextProductOverlayBody(false)
		if err := writeWorkflowContextProductOverlay(overlay.path, body); err != nil {
			return WorkflowContextOverlayReadback{}, err
		}
		hash := workflowContextProductOverlayHash(body)
		if err := verifyWorkflowContextProductOverlay(overlay.path, body, hash); err != nil {
			return WorkflowContextOverlayReadback{}, err
		}
		overlay.effectiveMode = request.HistoryMode
		return overlay.readback(request, previous, hash), nil
	default:
		return WorkflowContextOverlayReadback{}, errors.New("product OMP overlay history mode is invalid")
	}
}

func (overlay *workflowContextProductOverlay) readback(
	request WorkflowContextOverlayRequest,
	previous string,
	hash string,
) WorkflowContextOverlayReadback {
	return WorkflowContextOverlayReadback{
		RequestedHistoryMode: request.HistoryMode, EffectiveHistoryMode: request.HistoryMode,
		EffectiveMemoryMode: overlay.memoryMode, PreviousHistoryMode: previous,
		OverlayHash: hash, ReadbackHash: hash, Reason: request.Reason,
	}
}

func (overlay *workflowContextProductOverlay) verifyActive() error {
	info, err := os.Lstat(overlay.path)
	if err != nil || !os.SameFile(overlay.activeInfo, info) {
		return errors.New("prepared product OMP overlay identity changed")
	}
	return verifyWorkflowContextProductOverlay(overlay.path, overlay.activeBody, overlay.activeHash)
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: 100k threshold and 128/256-token windows bound local compaction; rollback only disables it.
func workflowContextProductOverlayBody(active bool) []byte {
	enabled := "false"
	if active {
		enabled = "true"
	}
	return []byte("contextPromotion:\n  enabled: false\n" +
		"extensions: []\nmcp:\n  enableProjectConfig: false\n" +
		"commands:\n  enableClaudeProject: false\n  enableClaudeUser: false\n" +
		"  enableOpencodeProject: false\n  enableOpencodeUser: false\n" +
		"compaction:\n  enabled: " + enabled + "\n  methodOrder: [snapcompact]\n  midTurnEnabled: false\n" +
		"  thresholdTokens: 100000\n  thresholdPercent: -1\n  reserveTokens: 128\n" +
		"  keepRecentTokens: 256\n  autoContinue: false\n" +
		"memory:\n  backend: off\nskills:\n  enableCodexUser: false\n  enableClaudeUser: false\n" +
		"  enableClaudeProject: false\n  enablePiUser: false\n  enablePiProject: true\n" +
		"  enableAgentsUser: false\n  enableAgentsProject: false\n  enableSkillCommands: true\n" +
		"  customDirectories: []\n")
}

func writeWorkflowContextProductOverlay(path string, body []byte) (resultErr error) {
	directory := filepath.Dir(path)
	temp, err := os.CreateTemp(directory, ".context-product-*.tmp")
	if err != nil {
		return fmt.Errorf("create product OMP overlay: %w", err)
	}
	tempPath := temp.Name()
	defer func() {
		if removeErr := os.Remove(tempPath); !errors.Is(removeErr, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, removeErr)
		}
	}()
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(body); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("install product OMP overlay: %w", err)
	}
	return nil
}

func verifyWorkflowContextProductOverlay(path string, expected []byte, expectedHash string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 {
		return errors.New("product OMP overlay identity is invalid")
	}
	body, err := os.ReadFile(path)
	if err != nil || string(body) != string(expected) || workflowContextProductOverlayHash(body) != expectedHash {
		return errors.New("product OMP overlay readback mismatch")
	}
	return nil
}

func workflowContextProductOverlayHash(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}
