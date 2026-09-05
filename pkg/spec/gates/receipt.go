package gates

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	applicabilityFile = "gate-applicability.json"
	evidenceDir       = "gates"
)

// ApplicabilityPath returns {SPEC_DIR}/gate-applicability.json.
func ApplicabilityPath(specDir string) string {
	return filepath.Join(specDir, applicabilityFile)
}

// EvidencePath returns {SPEC_DIR}/gates/evidence-<gate>.json.
func EvidencePath(specDir string, gate GateID) string {
	return filepath.Join(specDir, evidenceDir, "evidence-"+string(gate)+".json")
}

// WriteApplicability persists receipt with decisions in catalog order; the
// caller's slice is left untouched.
func WriteApplicability(specDir string, receipt ApplicabilityReceipt) (string, error) {
	decisions := append([]GateDecision(nil), receipt.Decisions...)
	sort.SliceStable(decisions, func(i, j int) bool {
		return catalogIndex(decisions[i].Gate) < catalogIndex(decisions[j].Gate)
	})
	receipt.Decisions = decisions
	path := ApplicabilityPath(specDir)
	return path, writeJSON(path, receipt)
}

// ReadApplicability loads a previously written applicability receipt.
func ReadApplicability(specDir string) (ApplicabilityReceipt, error) {
	var receipt ApplicabilityReceipt
	if err := readJSON(ApplicabilityPath(specDir), &receipt); err != nil {
		return ApplicabilityReceipt{}, err
	}
	if receipt.Schema != ApplicabilitySchema {
		return ApplicabilityReceipt{}, fmt.Errorf("unexpected applicability schema %q", receipt.Schema)
	}
	return receipt, nil
}

// WriteEvidence persists an evidence receipt under {SPEC_DIR}/gates/.
func WriteEvidence(specDir string, receipt EvidenceReceipt) (string, error) {
	if _, ok := LookupGate(receipt.Gate); !ok {
		return "", fmt.Errorf("unknown gate %q", receipt.Gate)
	}
	if err := os.MkdirAll(filepath.Join(specDir, evidenceDir), 0o700); err != nil {
		return "", fmt.Errorf("create evidence dir: %w", err)
	}
	path := EvidencePath(specDir, receipt.Gate)
	return path, writeJSON(path, receipt)
}

// ReadEvidence loads the evidence receipt for gate; ok is false when none
// has been recorded.
func ReadEvidence(specDir string, gate GateID) (receipt EvidenceReceipt, ok bool, err error) {
	err = readJSON(EvidencePath(specDir, gate), &receipt)
	if errors.Is(err, fs.ErrNotExist) {
		return EvidenceReceipt{}, false, nil
	}
	if err != nil {
		return EvidenceReceipt{}, false, err
	}
	if receipt.Schema != EvidenceSchema {
		return EvidenceReceipt{}, false, fmt.Errorf("unexpected evidence schema %q in %s", receipt.Schema, EvidencePath(specDir, gate))
	}
	if receipt.Gate != gate {
		return EvidenceReceipt{}, false, fmt.Errorf("evidence receipt %s records gate %q", EvidencePath(specDir, gate), receipt.Gate)
	}
	return receipt, true, nil
}

// LoadPriorEvidence reads every recorded evidence receipt and recomputes its
// input closure against root, ready for Decide. Receipt paths are recorded
// relative to root so the applicability receipt does not depend on how the
// SPEC was addressed.
func LoadPriorEvidence(root, specDir string) (map[GateID]PriorEvidence, error) {
	prior := map[GateID]PriorEvidence{}
	for _, entry := range Catalog {
		receipt, ok, err := ReadEvidence(specDir, entry.ID)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		closure, missing, err := RecomputeClosure(root, receipt)
		if err != nil {
			return nil, fmt.Errorf("recompute closure for %s: %w", entry.ID, err)
		}
		prior[entry.ID] = PriorEvidence{
			Receipt:        receipt,
			Path:           rootRelative(root, EvidencePath(specDir, entry.ID)),
			CurrentClosure: closure,
			MissingInputs:  missing,
		}
	}
	return prior, nil
}

// rootRelative returns path relative to root in slash form when both resolve
// under the same tree; otherwise path is returned unchanged.
func rootRelative(root, path string) string {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return path
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}
	return filepath.ToSlash(rel)
}

func catalogIndex(id GateID) int {
	for i, entry := range Catalog {
		if entry.ID == id {
			return i
		}
	}
	return len(Catalog)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}
