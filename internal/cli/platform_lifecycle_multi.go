package cli

import (
	"errors"
	"fmt"
)

type platformLifecycleSnapshots struct {
	order      []string
	byPlatform map[string]platformLifecycleSnapshot
}

func captureDescriptorLifecycleSnapshots(
	root string,
	descriptor platformDescriptor,
) (platformLifecycleSnapshots, error) {
	names := append([]string{descriptor.name}, descriptor.rollbackPlatforms...)
	return captureNamedLifecycleSnapshots(root, names)
}

func captureNamedLifecycleSnapshots(root string, names []string) (platformLifecycleSnapshots, error) {
	result := platformLifecycleSnapshots{byPlatform: make(map[string]platformLifecycleSnapshot)}
	seen := make(map[string]bool)
	for _, name := range names {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		snapshot, err := capturePlatformLifecycleSnapshot(root, name)
		if err != nil {
			return platformLifecycleSnapshots{}, fmt.Errorf("capture %s lifecycle: %w", name, err)
		}
		result.order = append(result.order, name)
		result.byPlatform[name] = snapshot
	}
	return result, nil
}

func restoreDescriptorLifecycleSnapshots(
	root string,
	descriptor platformDescriptor,
	snapshots platformLifecycleSnapshots,
	primaryPaths []string,
) error {
	var result error
	for index := len(snapshots.order) - 1; index >= 0; index-- {
		name := snapshots.order[index]
		paths := []string(nil)
		if name == descriptor.name {
			paths = primaryPaths
		}
		if err := restorePlatformLifecycleSnapshot(root, name, snapshots.byPlatform[name], paths); err != nil {
			result = errors.Join(result, fmt.Errorf("restore %s lifecycle: %w", name, err))
		}
	}
	return result
}

func restoreNamedLifecycleSnapshots(root string, snapshots platformLifecycleSnapshots) error {
	var result error
	for index := len(snapshots.order) - 1; index >= 0; index-- {
		name := snapshots.order[index]
		if err := restorePlatformLifecycleSnapshot(root, name, snapshots.byPlatform[name], nil); err != nil {
			result = errors.Join(result, fmt.Errorf("restore %s lifecycle: %w", name, err))
		}
	}
	return result
}

func lifecycleOperationError(primary, rollback error) error {
	if rollback == nil {
		return primary
	}
	return fmt.Errorf("%w; platform lifecycle rollback failed: %v", primary, rollback)
}
