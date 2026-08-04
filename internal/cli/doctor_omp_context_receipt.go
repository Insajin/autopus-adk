package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

const (
	ompContextDoctorMaxReceiptBytes = 1 << 20
	ompContextDoctorMaxReceipts     = 256
	ompContextDoctorFreshness       = 24 * time.Hour
	ompContextDoctorFutureSkew      = 5 * time.Minute
)

var ompContextDoctorHash = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type ompContextDoctorReceiptCandidate struct {
	rel     string
	receipt WorkflowContextRuntimeReceipt
}

// @AX:WARN [AUTO]: receipt discovery and trust validation has cyclomatic complexity 27.
// @AX:REASON [AUTO]: gocyclo reports 27 across rooted-path, permission, freshness, and candidate-selection checks.
func readOMPContextDoctorReceipt(root string, now time.Time) ompContextDoctorReceiptState {
	const relRoot = ".autopus/runtime/omp-context"
	rootInfo, err := os.Lstat(root)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return ompContextDoctorReceiptState{Status: "invalid", Freshness: "invalid"}
	}
	path, err := adapter.SafeWorkspacePath(root, relRoot)
	if err != nil {
		return ompContextDoctorReceiptState{Status: "invalid", Freshness: "invalid"}
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return ompContextDoctorReceiptState{Status: "missing", Freshness: "missing"}
	}
	if err != nil || !safeOMPContextDoctorDir(info) {
		return ompContextDoctorReceiptState{Status: "invalid", Freshness: "invalid"}
	}
	var receipts []ompContextDoctorReceiptCandidate
	walkErr := filepath.WalkDir(path, func(candidate string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		entryInfo, err := entry.Info()
		if err != nil || entryInfo.Mode()&os.ModeSymlink != 0 {
			return errors.New("unsafe OMP context receipt path")
		}
		if entry.IsDir() {
			if !safeOMPContextDoctorDir(entryInfo) {
				return errors.New("unsafe OMP context receipt directory mode")
			}
			return nil
		}
		if entry.Name() != "receipt.json" {
			return nil
		}
		if len(receipts) >= ompContextDoctorMaxReceipts {
			return errors.New("too many OMP context receipts")
		}
		rel, err := filepath.Rel(root, candidate)
		if err != nil {
			return err
		}
		safePath, err := adapter.SafeWorkspacePath(root, rel)
		if err != nil || safePath != candidate || !entryInfo.Mode().IsRegular() || entryInfo.Mode().Perm() != 0o600 {
			return errors.New("unsafe OMP context receipt file")
		}
		receipt, err := decodeOMPContextDoctorReceipt(safePath)
		if err != nil {
			return err
		}
		receipts = append(receipts, ompContextDoctorReceiptCandidate{rel: filepath.ToSlash(rel), receipt: receipt})
		return nil
	})
	if walkErr != nil {
		return ompContextDoctorReceiptState{Status: "invalid", Freshness: "invalid"}
	}
	if len(receipts) == 0 {
		return ompContextDoctorReceiptState{Status: "missing", Freshness: "missing"}
	}
	sort.Slice(receipts, func(i, j int) bool {
		left, right := receipts[i], receipts[j]
		if left.receipt.Capabilities.CheckedAt.Equal(right.receipt.Capabilities.CheckedAt) {
			return left.rel < right.rel
		}
		return left.receipt.Capabilities.CheckedAt.After(right.receipt.Capabilities.CheckedAt)
	})
	receipt := receipts[0].receipt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if receipt.Capabilities.CheckedAt.After(now.Add(ompContextDoctorFutureSkew)) {
		return ompContextDoctorReceiptState{Status: "invalid", Freshness: "invalid"}
	}
	freshness := "fresh"
	if now.Sub(receipt.Capabilities.CheckedAt) > ompContextDoctorFreshness {
		freshness = "stale"
	}
	return ompContextDoctorReceiptState{Status: "valid", Freshness: freshness, Receipt: receipt}
}

func safeOMPContextDoctorDir(info os.FileInfo) bool {
	return info.IsDir() && info.Mode()&os.ModeSymlink == 0 && info.Mode().Perm()&0o077 == 0
}

func decodeOMPContextDoctorReceipt(path string) (WorkflowContextRuntimeReceipt, error) {
	file, err := os.Open(path)
	if err != nil {
		return WorkflowContextRuntimeReceipt{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, ompContextDoctorMaxReceiptBytes+1))
	if err != nil || len(data) > ompContextDoctorMaxReceiptBytes {
		return WorkflowContextRuntimeReceipt{}, errors.New("invalid OMP context receipt size")
	}
	if err := validateWorkflowContextReceiptValues(data); err != nil {
		return WorkflowContextRuntimeReceipt{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var receipt WorkflowContextRuntimeReceipt
	if err := decoder.Decode(&receipt); err != nil || requireOMPContextDoctorJSONEOF(decoder) != nil {
		return WorkflowContextRuntimeReceipt{}, errors.New("invalid OMP context receipt JSON")
	}
	if !validOMPContextDoctorReceipt(receipt) {
		return WorkflowContextRuntimeReceipt{}, errors.New("invalid OMP context receipt schema")
	}
	return receipt, nil
}

func requireOMPContextDoctorJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return errors.New("unexpected trailing JSON value")
	}
	return err
}

// @AX:WARN [AUTO]: terminal receipt validation has cyclomatic complexity 26.
// @AX:REASON [AUTO]: gocyclo reports 26 because every schema, capability, phase, projection, and reference invariant is fail-closed.
func validOMPContextDoctorReceipt(receipt WorkflowContextRuntimeReceipt) bool {
	if receipt.SchemaVersion != WorkflowContextRuntimeReceiptSchemaVersion || receipt.Event != "terminal" ||
		!ompDoctorVersion.MatchString(receipt.Capabilities.Version) || receipt.Capabilities.CheckedAt.IsZero() ||
		!validOMPContextDoctorOutcome(receipt.Outcome) || !validOMPContextDoctorRoot(receipt.RootClass) ||
		len(receipt.DocumentOmissions) != 0 || len(receipt.MemoryInjections) != 0 ||
		receipt.Cleanup.UserRootAccessCount != 0 || receipt.ArtifactCounts.Before < 0 || receipt.ArtifactCounts.AfterCleanup < 0 {
		return false
	}
	if receipt.Fallback.Mode != WorkflowContextFallbackNone && receipt.Fallback.Mode != config.OMPContextFallbackBlock &&
		receipt.Fallback.Mode != config.OMPContextFallbackCanonicalFull {
		return false
	}
	for _, value := range []string{receipt.WorkspaceID, receipt.SpecID, receipt.TaskID, receipt.Phase, receipt.SessionID} {
		if validateWorkflowContextReceiptID(value) != nil {
			return false
		}
	}
	for _, hash := range []string{receipt.BindingHash, receipt.OptionsHash, receipt.SnapshotHash, receipt.PromptManifestHash,
		receipt.Mode.OverlayHash, receipt.Mode.ReadbackHash} {
		if !ompContextDoctorHash.MatchString(hash) {
			return false
		}
	}
	if receipt.Mode.OverlayHash != receipt.Mode.ReadbackHash || !validOMPContextDoctorModes(receipt.Mode) {
		return false
	}
	if !validOMPContextDoctorReferences(receipt) {
		return false
	}
	if receipt.Fallback.SnapshotHash != "" && !ompContextDoctorHash.MatchString(receipt.Fallback.SnapshotHash) ||
		receipt.Fallback.PromptManifestHash != "" && !ompContextDoctorHash.MatchString(receipt.Fallback.PromptManifestHash) {
		return false
	}
	return true
}

func validOMPContextDoctorOutcome(value string) bool {
	return value == WorkflowContextOutcomeAdmitted || value == WorkflowContextOutcomeFallback || value == WorkflowContextOutcomeBlocked
}

func validOMPContextDoctorRoot(value string) bool {
	return value == config.OMPContextRuntimeNoSession || value == config.OMPContextRuntimeIsolatedTaskOwned
}

func validOMPContextDoctorModes(mode WorkflowContextModeReceipt) bool {
	history := func(value string) bool {
		return value == config.OMPContextHistoryOff || value == config.OMPContextHistoryShadow || value == config.OMPContextHistoryActive
	}
	memory := mode.EffectiveMemoryMode == config.OMPContextMemoryOff || mode.EffectiveMemoryMode == config.OMPContextMemoryShadow
	return history(mode.RequestedHistoryMode) && history(mode.EffectiveHistoryMode) && history(mode.PreviousHistoryMode) && memory
}

// @AX:WARN [AUTO]: document-reference validation has cyclomatic complexity 19.
// @AX:REASON [AUTO]: gocyclo reports 19 across full-document, frozen-finding, ownership, forbidden-path, and schema references.
func validOMPContextDoctorReferences(receipt WorkflowContextRuntimeReceipt) bool {
	for _, ref := range receipt.FullDocumentRefs {
		if !validOMPContextDoctorDocumentRef(ref.SourceRef, ref.SourceHash, ref.PromptHash, ref.Complete) {
			return false
		}
	}
	for _, ref := range receipt.RequiredEphemeralRefs {
		if validateWorkflowContextReceiptID(ref.ID) != nil || !ompContextDoctorHash.MatchString(ref.Hash) {
			return false
		}
	}
	for _, row := range receipt.HistoryCreditRows {
		if validateWorkflowContextReceiptID(row.ID) != nil || !safeOMPContextDoctorRef(row.SourceRef) ||
			!ompContextDoctorHash.MatchString(row.PriorHash) || row.TokenBefore < 0 || row.TokenAfter < 0 {
			return false
		}
	}
	for _, ref := range receipt.ShadowCandidateRefs {
		if !safeOMPContextDoctorRef(ref.SourceRef) || !ompContextDoctorHash.MatchString(ref.SourceHash) {
			return false
		}
	}
	for _, ref := range receipt.Fallback.FullDocumentRefs {
		if !validOMPContextDoctorDocumentRef(ref.SourceRef, ref.SourceHash, ref.PromptHash, ref.Complete) {
			return false
		}
	}
	for _, value := range append(append([]string(nil), receipt.FrozenFindingIDs...), receipt.WorkerResultFields...) {
		if validateWorkflowContextReceiptID(value) != nil {
			return false
		}
	}
	return true
}

func validOMPContextDoctorDocumentRef(source, sourceHash, promptHash string, complete bool) bool {
	return safeOMPContextDoctorRef(source) && ompContextDoctorHash.MatchString(sourceHash) &&
		ompContextDoctorHash.MatchString(promptHash) && complete
}

func safeOMPContextDoctorRef(value string) bool {
	clean := filepath.Clean(strings.TrimSpace(value))
	return clean != "." && !filepath.IsAbs(clean) && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}
