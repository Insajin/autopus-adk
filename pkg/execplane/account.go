// Package execplane joins the policy plane's tier decision to the process
// plane's execution account, so a tier is only ever validated against evidence
// obtained under the entitlement that will actually run the workload.
//
// See .autopus/specs/SPEC-EXECPLANE-001 for the contract this implements.
package execplane

import (
	"encoding/json"
	"fmt"
)

// Provider names shared with the policy plane's quality vocabulary.
const (
	ProviderClaude = "claude"
	ProviderCodex  = "codex"
)

// AccountStatus records how the execution account was resolved. The process
// plane may leave no account selected, and guessing one is what produces a
// silent downgrade, so an ambiguous roster resolves to Indeterminate instead.
type AccountStatus string

const (
	AccountActive         AccountStatus = "active"
	AccountSoleRegistered AccountStatus = "sole_registered"
	AccountIndeterminate  AccountStatus = "indeterminate"
)

// Account identifies one managed provider account without carrying credentials.
type Account struct {
	ID    string `json:"id,omitempty"`
	Email string `json:"email,omitempty"`
	Org   string `json:"org,omitempty"`
}

// AccountResolution is the process plane's answer to "who will run this".
type AccountResolution struct {
	Provider string        `json:"provider"`
	Status   AccountStatus `json:"status"`
	Account  Account       `json:"account,omitzero"`
	// Probe names the account whose catalog the policy plane already holds. It
	// is the host CLI login, which is not necessarily the execution account.
	Probe  Account `json:"probe,omitzero"`
	Reason string  `json:"reason,omitempty"`
}

// Determined reports whether an execution account was pinned to exactly one
// identity. An indeterminate resolution never carries an account.
func (r AccountResolution) Determined() bool {
	return r.Status == AccountActive || r.Status == AccountSoleRegistered
}

// orcaAccountList mirrors the fields of `orca account list --json` this gate
// depends on. Unknown fields are ignored so an orca release that adds keys
// does not break the parse.
type orcaAccountList struct {
	OK     bool                            `json:"ok"`
	Result map[string]orcaProviderAccounts `json:"result"`
}

type orcaProviderAccounts struct {
	Accounts        []orcaAccount      `json:"accounts"`
	ActiveAccountID string             `json:"activeAccountId"`
	SystemDefault   *orcaSystemDefault `json:"systemDefault"`
}

type orcaAccount struct {
	ID                string `json:"id"`
	Email             string `json:"email"`
	OrganizationUUID  string `json:"organizationUuid"`
	ProviderAccountID string `json:"providerAccountId"`
}

type orcaSystemDefault struct {
	HasAuth           bool   `json:"hasAuth"`
	Email             string `json:"email"`
	ProviderAccountID string `json:"providerAccountId"`
}

func (a orcaAccount) account() Account {
	org := a.OrganizationUUID
	if org == "" {
		org = a.ProviderAccountID
	}
	return Account{ID: a.ID, Email: a.Email, Org: org}
}

// ResolveExecutionAccount reads an `orca account list --json` payload and
// resolves the account that will run workloads for one provider.
//
// Resolution order is the process plane's active selection, then the single
// registered account when the roster holds exactly one, and otherwise
// indeterminate. A roster with two or more accounts and no active selection is
// reported as ambiguous rather than resolved to an arbitrary member.
func ResolveExecutionAccount(payload []byte, provider string) (AccountResolution, error) {
	var listing orcaAccountList
	if err := json.Unmarshal(payload, &listing); err != nil {
		return AccountResolution{}, fmt.Errorf("parse orca account listing: %w", err)
	}
	if !listing.OK {
		return AccountResolution{}, fmt.Errorf("orca account listing reported failure")
	}
	accounts, ok := listing.Result[provider]
	if !ok {
		return AccountResolution{
			Provider: provider, Status: AccountIndeterminate,
			Reason: "orca reports no account roster for provider " + provider,
		}, nil
	}

	resolution := AccountResolution{Provider: provider, Probe: accounts.probeAccount()}
	if active, found := accounts.byID(accounts.ActiveAccountID); found {
		resolution.Status, resolution.Account = AccountActive, active
		return resolution, nil
	}
	switch len(accounts.Accounts) {
	case 1:
		resolution.Status = AccountSoleRegistered
		resolution.Account = accounts.Accounts[0].account()
		resolution.Reason = "no active account selected; adopted the sole registered account"
	case 0:
		resolution.Status = AccountIndeterminate
		resolution.Reason = "no active account selected and none registered"
	default:
		resolution.Status = AccountIndeterminate
		resolution.Reason = fmt.Sprintf(
			"no active account selected and %d accounts registered", len(accounts.Accounts))
	}
	return resolution, nil
}

func (p orcaProviderAccounts) byID(id string) (Account, bool) {
	if id == "" {
		return Account{}, false
	}
	for _, candidate := range p.Accounts {
		if candidate.ID == id {
			return candidate.account(), true
		}
	}
	return Account{}, false
}

// probeAccount returns the host CLI identity orca reports alongside the managed
// roster. Only Codex exposes it today; Claude carries no equivalent field.
func (p orcaProviderAccounts) probeAccount() Account {
	if p.SystemDefault == nil || !p.SystemDefault.HasAuth {
		return Account{}
	}
	return Account{Email: p.SystemDefault.Email, Org: p.SystemDefault.ProviderAccountID}
}
