package execplane

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/insajin/autopus-adk/pkg/processprobe"
)

const (
	// orcaBinary is the process plane CLI. It is looked up on PATH rather than
	// pinned to an install location, matching how the codex catalog probe
	// resolves its binary.
	orcaBinary = "orca"
	// accountListingTimeout bounds the roster read. The gate runs before any
	// execution resource exists, so a hung process plane must degrade to
	// unverified instead of stalling the pipeline.
	accountListingTimeout = 10 * time.Second
	// maxAccountListingBytes bounds the roster payload. A machine holds a
	// handful of accounts; anything larger is a malfunction, not a roster.
	maxAccountListingBytes = 1 << 20
)

// ErrOrcaUnavailable reports that the process plane CLI could not be located.
// It is a sentinel rather than an opaque failure because the caller's response
// is prescribed: no roster means no execution account, which is REQ-009's
// unverified branch and not a pipeline abort.
var ErrOrcaUnavailable = errors.New("execplane: orca CLI is not available on PATH")

// ListOrcaAccounts runs `orca account list --json` and returns the raw payload.
// One call carries both the execution account and the host CLI identity, so the
// gate never issues a second lookup to compare them.
func ListOrcaAccounts(ctx context.Context) ([]byte, error) {
	binary, err := exec.LookPath(orcaBinary)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOrcaUnavailable, err)
	}
	probeCtx, cancel := context.WithTimeout(ctx, accountListingTimeout)
	defer cancel()

	cmd := exec.CommandContext(probeCtx, binary, "account", "list", "--json")
	output, err := processprobe.OutputLimited(cmd, maxAccountListingBytes)
	if err != nil {
		return nil, fmt.Errorf("run orca account listing: %w", err)
	}
	return output, nil
}

// Evidence is everything the gate reads from outside the process before it
// decides. The two entitlements are kept apart by name because they answer
// different questions: what the workload will run under, and what the held
// catalog was probed under.
type Evidence struct {
	Resolution       AccountResolution
	ExecEntitlement  Entitlement
	ProbeEntitlement Entitlement
}

// Prober gathers Evidence. Its three fields are the only seams that touch the
// outside world, and a nil field falls back to the real implementation, so the
// zero Prober behaves exactly like NewProber.
//
// This layer only reads. It creates no worktree, Run, worker, or provider
// session, and it never writes into an orca-managed home.
type Prober struct {
	// AccountListing returns a raw `orca account list --json` payload.
	AccountListing func(ctx context.Context) ([]byte, error)
	// Credential returns the raw credential of one orca-managed account. It
	// takes the provider because one Prober serves every provider in a run,
	// and each keeps its managed credential in its own layout.
	Credential func(provider, accountID string) ([]byte, error)
	// HostCredential returns the raw credential of the provider's local CLI.
	HostCredential func(provider string) ([]byte, error)
}

// NewProber returns a Prober wired to the real process plane and filesystem.
func NewProber() Prober {
	return Prober{
		AccountListing: ListOrcaAccounts,
		Credential:     readManagedCredential,
		HostCredential: readLocalCredential,
	}
}

// Inspect resolves the execution account for one provider and recovers both
// entitlement grades, without a single network call: both providers already
// record the grade in a credential on disk.
//
// The returned Evidence is always safe to hand to Evaluate, including alongside
// an error. A probe that fails yields an indeterminate resolution with a
// non-empty reason, which is the unverified branch rather than a guess.
//
// Both providers record the grade on disk, in different places and shapes, so
// the credential table decides where to read and how to parse it. A provider
// absent from that table leaves both entitlements zero, which CompareEntitlement
// degrades to unverified on its own.
func (p Prober) Inspect(ctx context.Context, provider string) (Evidence, error) {
	listing, err := p.listAccounts(ctx)
	if err != nil {
		return Evidence{Resolution: indeterminate(provider,
			"process plane account listing is unavailable")}, err
	}
	resolution, err := ResolveExecutionAccount(listing, provider)
	if err != nil {
		return Evidence{Resolution: indeterminate(provider,
			"process plane account listing could not be read")}, err
	}

	evidence := Evidence{Resolution: resolution}
	source, known := credentialSourceFor(provider)
	if !known {
		return evidence, nil
	}
	// An unreadable credential is a degradation, not a failure: the grade stays
	// unknown, CompareEntitlement reports why, and the receipt says unverified.
	// Surfacing it as an error here would turn a reportable state into an abort.
	if resolution.Determined() {
		evidence.ExecEntitlement, _ = p.executionEntitlement(source, resolution.Account)
	}
	evidence.ProbeEntitlement, _ = p.hostEntitlement(source, resolution.Probe)
	return evidence, nil
}

// executionEntitlement reads the grade of the account that will run the
// workload, from the credential orca swaps in for it.
func (p Prober) executionEntitlement(source credentialSource, account Account) (Entitlement, error) {
	payload, err := p.readCredential(source, account.ID)
	if err != nil {
		return Entitlement{}, err
	}
	return source.parse(payload,
		credentialLabel(source, account, payload, source.executionScope))
}

// hostEntitlement reads the grade the held catalog was probed under, which is
// the local CLI login and not necessarily the execution account.
func (p Prober) hostEntitlement(source credentialSource, probe Account) (Entitlement, error) {
	payload, err := p.readHostCredential(source)
	if err != nil {
		return Entitlement{}, err
	}
	return source.parse(payload,
		credentialLabel(source, probe, payload, source.probeScope))
}

func (p Prober) listAccounts(ctx context.Context) ([]byte, error) {
	if p.AccountListing != nil {
		return p.AccountListing(ctx)
	}
	return ListOrcaAccounts(ctx)
}

func (p Prober) readCredential(source credentialSource, accountID string) ([]byte, error) {
	if p.Credential != nil {
		return p.Credential(source.provider, accountID)
	}
	return source.readManaged(accountID)
}

func (p Prober) readHostCredential(source credentialSource) ([]byte, error) {
	if p.HostCredential != nil {
		return p.HostCredential(source.provider)
	}
	return source.readHost()
}

// indeterminate builds the resolution used when the process plane could not be
// read at all. The reason is never empty, so the receipt stays reconstructable.
func indeterminate(provider, reason string) AccountResolution {
	return AccountResolution{
		Provider: provider,
		Status:   AccountIndeterminate,
		Reason:   reason,
	}
}
