package cli

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"time"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

const pipelineOMPActiveLeaseMaxValidity = 5 * time.Minute

// pipelineOMPActiveLeaseBinding contains only body-free, exact active-session
// coordinates. Prompt bodies remain in the sealed execution view.
type pipelineOMPActiveLeaseBinding struct {
	GrantDigest          string
	PolicyDigest         string
	WorkspaceID          string
	SpecID               string
	TaskID               string
	Phase                string
	SessionID            string
	BindingHash          string
	OptionsHash          string
	SnapshotHash         string
	GitCommitHash        string
	OriginalTaskHash     string
	DecisionDeltaHash    string
	RuntimeVersion       string
	ExecutableSHA256     string
	AutoVersion          string
	AutoExecutableSHA256 string
	AutoSourceCommit     string
	AutoSourceTree       string
	ModelScopeDigest     string
	// Provider and Model are retained for in-package compatibility only. They
	// are deliberately excluded from validation and the authority digest.
	Provider            string
	Model               string
	CohortDigest        string
	OracleDigest        string
	EligibleHistoryHash string
}

// pipelineOMPActiveLease is a process-private, one-use capability. It has no
// decoder or persistence representation by design.
type pipelineOMPActiveLease struct {
	bindingDigest string
	nonceHash     string
	issuedAt      time.Time
	expiresAt     time.Time
	consumed      atomic.Bool
}

func newPipelineOMPActiveLease(
	binding pipelineOMPActiveLeaseBinding,
	now time.Time,
	validFor time.Duration,
) (*pipelineOMPActiveLease, error) {
	if err := validatePipelineOMPActiveLeaseBinding(binding); err != nil {
		return nil, err
	}
	if now.IsZero() || validFor <= 0 || validFor > pipelineOMPActiveLeaseMaxValidity {
		return nil, errors.New("OMP active lease validity is invalid")
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, errors.New("OMP active lease nonce is unavailable")
	}
	issuedAt := now.UTC()
	return &pipelineOMPActiveLease{
		bindingDigest: hashPipelineOMPActiveLeaseBinding(binding),
		nonceHash:     pipelineOMPActiveHash(nonce),
		issuedAt:      issuedAt,
		expiresAt:     issuedAt.Add(validFor),
	}, nil
}

// Consume validates the complete coordinate set and atomically spends the
// lease. Callers invoke it immediately before the managed child spawn.
func (lease *pipelineOMPActiveLease) Consume(binding pipelineOMPActiveLeaseBinding, now time.Time) error {
	if lease == nil || lease.bindingDigest == "" || lease.nonceHash == "" {
		return errors.New("OMP active lease is unavailable")
	}
	if err := validatePipelineOMPActiveLeaseBinding(binding); err != nil ||
		!workflowContextSecureEqual(lease.bindingDigest, hashPipelineOMPActiveLeaseBinding(binding)) {
		return errors.New("OMP active lease binding mismatch")
	}
	checkedAt := now.UTC()
	if checkedAt.Before(lease.issuedAt) || !checkedAt.Before(lease.expiresAt) {
		return errors.New("OMP active lease expired")
	}
	if !lease.consumed.CompareAndSwap(false, true) {
		return errors.New("OMP active lease already consumed")
	}
	return nil
}

func (lease *pipelineOMPActiveLease) Digest() string {
	if lease == nil {
		return ""
	}
	return pipelineOMPActiveHash([]byte(lease.bindingDigest + "\x00" + lease.nonceHash))
}

func (lease *pipelineOMPActiveLease) NonceHash() string {
	if lease == nil {
		return ""
	}
	return lease.nonceHash
}

func (*pipelineOMPActiveLease) MarshalJSON() ([]byte, error) {
	return nil, errors.New("OMP active lease cannot be serialized")
}

var _ json.Marshaler = (*pipelineOMPActiveLease)(nil)

func validatePipelineOMPActiveLeaseBinding(binding pipelineOMPActiveLeaseBinding) error {
	if err := pipeline.ValidateSpecID(binding.SpecID); err != nil {
		return errors.New("OMP active lease binding is invalid")
	}
	for _, value := range []string{
		binding.GrantDigest, binding.PolicyDigest, binding.BindingHash, binding.OptionsHash, binding.SnapshotHash,
		binding.OriginalTaskHash, binding.DecisionDeltaHash,
		binding.ExecutableSHA256, binding.AutoExecutableSHA256,
		binding.ModelScopeDigest, binding.CohortDigest, binding.OracleDigest, binding.EligibleHistoryHash,
	} {
		if !validPipelineOMPActiveHash(value) {
			return errors.New("OMP active lease binding is invalid")
		}
	}
	for _, value := range []string{
		binding.WorkspaceID, binding.TaskID, binding.Phase, binding.SessionID,
		binding.RuntimeVersion, binding.AutoVersion,
	} {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value || strings.ContainsRune(value, 0) {
			return errors.New("OMP active lease binding is invalid")
		}
	}
	if !validPipelineOMPActiveGitHash(binding.GitCommitHash) ||
		!validPipelineOMPActiveSourceCoordinates(binding.AutoSourceCommit, binding.AutoSourceTree) {
		return errors.New("OMP active lease binding is invalid")
	}
	return nil
}

func validPipelineOMPActiveSourceCoordinates(commit, tree string) bool {
	// Empty coordinates are admitted only for package-private runner seams. The
	// production coordinator rejects them before it loads a signed grant.
	return commit == "" && tree == "" ||
		validPipelineOMPActiveGitHash(commit) && validPipelineOMPActiveGitHash(tree)
}

func hashPipelineOMPActiveLeaseBinding(binding pipelineOMPActiveLeaseBinding) string {
	parts := []string{
		binding.GrantDigest, binding.PolicyDigest, binding.WorkspaceID, binding.SpecID, binding.TaskID, binding.Phase,
		binding.SessionID, binding.BindingHash, binding.OptionsHash, binding.SnapshotHash,
		binding.GitCommitHash, binding.OriginalTaskHash, binding.DecisionDeltaHash,
		binding.RuntimeVersion, binding.ExecutableSHA256, binding.AutoVersion, binding.AutoExecutableSHA256,
		binding.AutoSourceCommit, binding.AutoSourceTree,
		binding.ModelScopeDigest, binding.CohortDigest, binding.OracleDigest, binding.EligibleHistoryHash,
	}
	return pipelineOMPActiveHash([]byte(strings.Join(parts, "\x00")))
}

func pipelineOMPActiveHash(value []byte) string {
	sum := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func validPipelineOMPActiveHash(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil && strings.ToLower(value) == value
}

func validPipelineOMPActiveGitHash(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}
