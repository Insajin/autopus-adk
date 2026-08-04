package omp

import (
	"fmt"
	"os"
)

type ompProjectCleanState struct {
	preimage []byte
	missing  bool
	mode     os.FileMode
}

func (a *Adapter) prepareOMPProjectCleanStateAt(workspace *ompRootedWorkspace) (*ompProjectCleanState, error) {
	ownership, exists, err := readOMPModelProjectOwnershipAt(workspace)
	if err != nil || !exists {
		return nil, err
	}
	if _, err := validateCurrentOMPModelProjectConfigAt(workspace, ownership); err != nil {
		return nil, err
	}
	receipt, reason := readOMPModelDoctorReceiptAt(workspace)
	if reason != "" || receipt.ConfigSource != "project-managed" ||
		receipt.ProjectOwnershipDigest != ownership.LedgerDigest {
		return nil, fmt.Errorf("managed_key_conflict: project ownership receipt mismatch")
	}
	preimage, missing, mode, err := loadOMPModelProjectOriginalPreimageAt(workspace, ownership)
	if err != nil {
		return nil, err
	}
	return &ompProjectCleanState{preimage: preimage, missing: missing, mode: mode}, nil
}
