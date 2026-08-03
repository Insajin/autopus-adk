package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
)

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: these four ordered documents define the canonical pipeline snapshot hash.
var pipelineSpecSnapshotDocuments = []string{"spec.md", "plan.md", "acceptance.md", "research.md"}

// SpecSnapshotHash returns the canonical hash used to bind a pipeline run to its SPEC package.
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: public snapshot contract shared by pipeline resolution and frozen prompt verification.
// @AX:REASON [AUTO]: Filename order, optional-document handling, and byte framing must remain identical for every run authority check.
func SpecSnapshotHash(specDir string) (string, error) {
	documents := make(map[string][]byte, len(pipelineSpecSnapshotDocuments))
	for _, name := range pipelineSpecSnapshotDocuments {
		body, err := os.ReadFile(filepath.Join(specDir, name))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) && name != "spec.md" {
				continue
			}
			return "", err
		}
		documents[name] = body
	}
	return specSnapshotHashFromDocuments(documents), nil
}

func specSnapshotHashFromDocuments(documents map[string][]byte) string {
	hash := sha256.New()
	for _, name := range pipelineSpecSnapshotDocuments {
		body, exists := documents[name]
		if !exists {
			continue
		}
		_, _ = hash.Write([]byte(name))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write(body)
		_, _ = hash.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}
