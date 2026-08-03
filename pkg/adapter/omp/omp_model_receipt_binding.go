package omp

import "fmt"

func validateOMPModelReceiptWorkspaceBinding(
	workspace *ompRootedWorkspace,
	receipt OMPModelResolutionReceipt,
) error {
	switch receipt.ConfigSource {
	case "overlay":
		data, _, err := workspace.readFile(DefaultOMPModelOverlayPath, 4<<20)
		if err != nil || OMPModelSHA256(data) != receipt.Activation.ConfigHash {
			return fmt.Errorf("OMP model receipt overlay binding mismatch")
		}
	case "project-managed":
		ownership, exists, err := readOMPModelProjectOwnershipAt(workspace)
		if err != nil || !exists || ownership.LedgerDigest != receipt.ProjectOwnershipDigest {
			return fmt.Errorf("OMP model receipt project ownership mismatch")
		}
		config, err := validateCurrentOMPModelProjectConfigAt(workspace, ownership)
		if err != nil || OMPModelSHA256(config) != receipt.Activation.ConfigHash {
			return fmt.Errorf("OMP model receipt project config mismatch")
		}
	default:
		return fmt.Errorf("OMP model receipt config source is unsupported")
	}
	return nil
}
