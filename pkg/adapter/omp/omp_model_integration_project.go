package omp

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

// @AX:WARN [AUTO]: project config merge has cyclomatic complexity 20.
// @AX:REASON [AUTO]: gocyclo reports 20 across ownership, journal, YAML merge, conflict, backup, and transaction preparation paths.
func (i *ompModelIntegration) mergeOMPIntegratedProjectConfig(
	workspace *ompRootedWorkspace,
	mapping adapter.FileMapping,
	values map[string]any,
) (adapter.FileMapping, adapter.FileMapping, string, error) {
	if len(i.profile.ManagedKeys) != len(values) {
		return mapping, adapter.FileMapping{}, "", fmt.Errorf(
			"managed_key_claim_required: expected=%d actual=%d", len(values), len(i.profile.ManagedKeys),
		)
	}
	ownership, managed, err := readOMPModelProjectOwnershipAt(workspace)
	if err != nil {
		return mapping, adapter.FileMapping{}, "", err
	}
	original, originalMissing, currentMode, err := readOMPModelProjectConfigAt(workspace)
	if err != nil {
		return mapping, adapter.FileMapping{}, "", err
	}
	base := append([]byte(nil), original...)
	if originalMissing {
		base = nil
	}
	if managed {
		if _, err := validateCurrentOMPModelProjectConfigAt(workspace, ownership); err != nil {
			return mapping, adapter.FileMapping{}, "", err
		}
		original, originalMissing, _, err = loadOMPModelProjectOriginalPreimageAt(workspace, ownership)
		if err != nil {
			return mapping, adapter.FileMapping{}, "", err
		}
		base = append([]byte(nil), original...)
		if originalMissing {
			base = nil
		}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	claims := make([]OMPManagedKeyClaim, 0, len(keys))
	for _, key := range keys {
		claim, ok := i.profile.ManagedKeys[key]
		if !ok {
			return mapping, adapter.FileMapping{}, "", fmt.Errorf("managed_key_claim_required: %s", key)
		}
		prior := claim.PriorFingerprint
		if managed && ompModelProjectOwnedPath(ownership.ManagedKeys, key) {
			prior, err = fingerprintOMPManagedPath(base, key)
			if err != nil {
				return mapping, adapter.FileMapping{}, "", err
			}
		}
		claims = append(claims, OMPManagedKeyClaim{
			Path: key, Value: values[key], Complete: claim.Complete,
			FullArrayOwnership: claim.FullArrayOwnership,
			PriorFingerprint:   prior,
		})
	}
	mode := os.FileMode(0o600)
	if currentMode != 0 {
		mode = currentMode
	}
	result, err := MergeOMPProjectManagedConfig(OMPProjectManagedInput{
		Existing: base, Mode: mode, Claims: claims,
	})
	if err != nil {
		return mapping, adapter.FileMapping{}, "", err
	}
	mergedDocument, err := mergeOMPConfigDocument(string(result.Bytes))
	if err != nil {
		return mapping, adapter.FileMapping{}, "", err
	}
	mapping.Content = []byte(mergedDocument)
	mapping.Checksum = adapter.Checksum(string(mapping.Content))
	var ownershipData []byte
	if managed {
		ownership, ownershipData, err = updateOMPModelProjectOwnership(
			ownership, mapping.Content, result.ManagedFingerprints,
		)
	} else {
		ownership, ownershipData, err = newOMPModelProjectOwnership(
			original, originalMissing, mapping.Content, result.ManagedFingerprints,
		)
	}
	if err != nil {
		return mapping, adapter.FileMapping{}, "", err
	}
	ledger := ompIntegratedMapping(OMPModelProjectOwnershipRelativePath, ownershipData)
	return mapping, ledger, ownership.LedgerDigest, nil
}

func fingerprintOMPManagedPath(data []byte, path string) (string, error) {
	document, err := parseOMPManagedDocument(data)
	if err != nil {
		return "", err
	}
	if err := rejectDuplicateOMPManagedKeys(document); err != nil {
		return "", err
	}
	location, err := locateOMPManagedPath(document, strings.Split(path, "."))
	if err != nil {
		return "", err
	}
	if !location.exists {
		return OMPMissingManagedValueFingerprint(), nil
	}
	value, err := decodeOMPManagedNode(location.value)
	if err != nil {
		return "", err
	}
	return FingerprintOMPManagedValue(value)
}

func ompModelProjectOwnedPath(keys []ompModelProjectOwnedKey, path string) bool {
	for _, key := range keys {
		if key.Path == path {
			return true
		}
	}
	return false
}
