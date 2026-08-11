package execplane

import (
	"encoding/json"
	"fmt"
)

// credentialSource states, for one provider, where the entitlement grade lives
// and how to recover it. Providers differ in the managed layout, in the local
// CLI's location, and in whether the grade sits in a token claim or in an
// account record. Collecting those differences in one table keeps Inspect free
// of provider branches: a third provider is one entry, not a new fork.
type credentialSource struct {
	provider string
	// parse recovers the grade from a credential payload.
	parse func(payload []byte, source string) (Entitlement, error)
	// identity recovers the signed in address from the credential itself, for
	// providers whose roster entry carries no host identity. It is optional.
	identity func(payload []byte) string
	// readManaged reads the credential of one orca-managed account.
	readManaged func(accountID string) ([]byte, error)
	// readHost reads the credential the local CLI is logged in as.
	readHost func() ([]byte, error)
	// executionScope and probeScope name each location for a human, and stand
	// in as the entitlement source when no address is known.
	executionScope string
	probeScope     string
}

var credentialSources = []credentialSource{
	{
		provider:       ProviderCodex,
		parse:          ParseCodexEntitlement,
		readManaged:    readManagedCodexAuth,
		readHost:       readLocalCodexAuth,
		executionScope: codexExecutionScope,
		probeScope:     codexProbeScope,
	},
	{
		// Claude's roster carries no host identity, so the local address is
		// recovered from the configuration that also carries the grade.
		provider:       ProviderClaude,
		parse:          ParseClaudeEntitlement,
		identity:       claudeCredentialEmail,
		readManaged:    readManagedClaudeAuth,
		readHost:       readLocalClaudeConfig,
		executionScope: claudeExecutionScope,
		probeScope:     claudeProbeScope,
	},
}

// credentialSourceFor returns the entry for one provider. A provider outside
// the table has no credential to read, which leaves both grades unknown and
// the run unverified rather than failing it.
func credentialSourceFor(provider string) (credentialSource, bool) {
	for _, source := range credentialSources {
		if source.provider == provider {
			return source, true
		}
	}
	return credentialSource{}, false
}

// readManagedCredential reads the credential of one orca-managed account. It
// is the default Prober.Credential seam.
func readManagedCredential(provider, accountID string) ([]byte, error) {
	source, known := credentialSourceFor(provider)
	if !known {
		return nil, unknownCredentialSource(provider)
	}
	return source.readManaged(accountID)
}

// readLocalCredential reads the credential the provider's local CLI is logged
// in as. It is the default Prober.HostCredential seam.
func readLocalCredential(provider string) ([]byte, error) {
	source, known := credentialSourceFor(provider)
	if !known {
		return nil, unknownCredentialSource(provider)
	}
	return source.readHost()
}

func unknownCredentialSource(provider string) error {
	return fmt.Errorf("%w: provider %q carries no credential source",
		ErrCredentialUnavailable, provider)
}

// credentialLabel names the account an entitlement was read from, without
// leaking its internal identifier. The address is what a later reader
// recognizes; the managed account UUID stays out of receipts, logs, and
// errors, and a location name is used when no address is known.
func credentialLabel(source credentialSource, account Account, payload []byte, scope string) string {
	if account.Email != "" {
		return account.Email
	}
	if source.identity != nil {
		if email := source.identity(payload); email != "" {
			return email
		}
	}
	return scope
}

// claudeCredentialEmail recovers the signed in address from a Claude
// credential. The orca-managed copy records it at the top level and the host
// CLI nests it under `oauthAccount`, matching where each keeps the grade.
//
// Only the address is read. The account and organization identifiers beside it
// are deliberately left in the payload: they are internal values, and the
// receipt names people, not identifiers.
func claudeCredentialEmail(payload []byte) string {
	var record struct {
		EmailAddress string `json:"emailAddress"`
		OAuthAccount *struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"oauthAccount"`
	}
	if err := json.Unmarshal(payload, &record); err != nil {
		return ""
	}
	if record.EmailAddress != "" {
		return record.EmailAddress
	}
	if record.OAuthAccount != nil {
		return record.OAuthAccount.EmailAddress
	}
	return ""
}
