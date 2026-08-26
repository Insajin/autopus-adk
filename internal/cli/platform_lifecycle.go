package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/config"
)

type platformConfigSaver func(string, *config.HarnessConfig) error
type platformLifecycleStage string

const (
	platformStageGenerate platformLifecycleStage = "generate"
	platformStageUpdate   platformLifecycleStage = "update"
	platformStageClean    platformLifecycleStage = "clean"
	platformStageSave     platformLifecycleStage = "save"
)

type platformLifecycleError struct {
	stage      platformLifecycleStage
	err        error
	rolledBack bool
}

func (e *platformLifecycleError) Error() string { return e.err.Error() }
func (e *platformLifecycleError) Unwrap() error { return e.err }

func generatePlatformThenSave(
	ctx context.Context,
	root string,
	descriptor platformDescriptor,
	generateCfg *config.HarnessConfig,
	saveCfg *config.HarnessConfig,
	save platformConfigSaver,
) error {
	snapshots, err := captureDescriptorLifecycleSnapshots(root, descriptor)
	if err != nil {
		return &platformLifecycleError{stage: platformStageGenerate, err: err}
	}
	files, err := descriptor.Generate(ctx, root, generateCfg)
	if err != nil {
		rollbackErr := restoreDescriptorLifecycleSnapshots(root, descriptor, snapshots, nil)
		return &platformLifecycleError{
			stage: platformStageGenerate, err: lifecycleOperationError(err, rollbackErr), rolledBack: rollbackErr == nil,
		}
	}
	if err := save(root, saveCfg); err != nil {
		rollbackErr := restoreDescriptorLifecycleSnapshots(root, descriptor, snapshots, platformFilePaths(files))
		return &platformLifecycleError{
			stage: platformStageSave, err: lifecycleOperationError(err, rollbackErr), rolledBack: rollbackErr == nil,
		}
	}
	return nil
}

func updatePlatformThenSave(
	ctx context.Context,
	root string,
	descriptor platformDescriptor,
	updateCfg *config.HarnessConfig,
	saveCfg *config.HarnessConfig,
	save platformConfigSaver,
) error {
	snapshots, err := captureDescriptorLifecycleSnapshots(root, descriptor)
	if err != nil {
		return &platformLifecycleError{stage: platformStageUpdate, err: err}
	}
	files, err := descriptor.Update(ctx, root, updateCfg)
	if err != nil {
		rollbackErr := restoreDescriptorLifecycleSnapshots(root, descriptor, snapshots, nil)
		return &platformLifecycleError{
			stage: platformStageUpdate, err: lifecycleOperationError(err, rollbackErr), rolledBack: rollbackErr == nil,
		}
	}
	if err := save(root, saveCfg); err != nil {
		rollbackErr := restoreDescriptorLifecycleSnapshots(root, descriptor, snapshots, platformFilePaths(files))
		return &platformLifecycleError{
			stage: platformStageSave, err: lifecycleOperationError(err, rollbackErr), rolledBack: rollbackErr == nil,
		}
	}
	return nil
}

func cleanPlatformThenSave(
	ctx context.Context,
	root string,
	descriptor platformDescriptor,
	cfg *config.HarnessConfig,
	save platformConfigSaver,
) (platformCleanReceipt, error) {
	snapshots, err := captureDescriptorLifecycleSnapshots(root, descriptor)
	if err != nil {
		return platformCleanReceipt{}, &platformLifecycleError{stage: platformStageClean, err: err}
	}
	receipt, err := descriptor.Clean(ctx, root)
	if err != nil {
		rollbackErr := restoreDescriptorLifecycleSnapshots(root, descriptor, snapshots, receipt.changedPaths)
		if rollbackErr == nil {
			receipt = platformCleanReceipt{}
		}
		return receipt, &platformLifecycleError{
			stage: platformStageClean, err: lifecycleOperationError(err, rollbackErr), rolledBack: rollbackErr == nil,
		}
	}
	if err := save(root, cfg); err != nil {
		rollbackErr := restoreDescriptorLifecycleSnapshots(root, descriptor, snapshots, receipt.changedPaths)
		rolledBack := rollbackErr == nil
		if rolledBack {
			receipt = platformCleanReceipt{}
		}
		return receipt, &platformLifecycleError{
			stage: platformStageSave, err: lifecycleOperationError(err, rollbackErr), rolledBack: rolledBack,
		}
	}
	return receipt, nil
}

type harnessConfigSnapshot struct {
	data   []byte
	mode   os.FileMode
	exists bool
}

func saveConfigWithGenerationRollback(
	root string,
	cfg *config.HarnessConfig,
	generate func() error,
) error {
	snapshot, err := captureHarnessConfigSnapshot(root)
	if err != nil {
		return fmt.Errorf("autopus.yaml snapshot failed: %w", err)
	}
	platformSnapshots, err := captureNamedLifecycleSnapshots(root, cfg.Platforms)
	if err != nil {
		return err
	}
	if err := config.Save(root, cfg); err != nil {
		return err
	}
	if err := generate(); err != nil {
		platformRollbackErr := restoreNamedLifecycleSnapshots(root, platformSnapshots)
		configRollbackErr := restoreHarnessConfigSnapshot(root, snapshot)
		return lifecycleOperationError(err, errors.Join(platformRollbackErr, configRollbackErr))
	}
	return nil
}

func captureHarnessConfigSnapshot(root string) (harnessConfigSnapshot, error) {
	path := filepath.Join(root, "autopus.yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return harnessConfigSnapshot{}, nil
	}
	if err != nil {
		return harnessConfigSnapshot{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return harnessConfigSnapshot{}, err
	}
	return harnessConfigSnapshot{data: data, mode: info.Mode().Perm(), exists: true}, nil
}

func restoreHarnessConfigSnapshot(root string, snapshot harnessConfigSnapshot) error {
	path := filepath.Join(root, "autopus.yaml")
	if !snapshot.exists {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return os.WriteFile(path, snapshot.data, snapshot.mode)
}
