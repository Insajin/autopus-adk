package execplane

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	probeExecAccountID = "11111111-1111-4111-8111-111111111111"
	probeExecEmail     = "exec@example.test"
	probeHostEmail     = "host@example.test"
)

func TestListOrcaAccountsReportsMissingBinaryAsSentinel(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	payload, err := ListOrcaAccounts(context.Background())
	require.ErrorIs(t, err, ErrOrcaUnavailable)
	assert.Nil(t, payload)
}

func TestInspectWithoutOrcaYieldsReceiptReadyUnverifiedEvidence(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	evidence, err := NewProber().Inspect(context.Background(), ProviderCodex)
	require.ErrorIs(t, err, ErrOrcaUnavailable)
	assert.False(t, evidence.Resolution.Determined())
	assert.NotEmpty(t, evidence.Resolution.Reason)

	receipt := Evaluate(
		TierRequest{Provider: ProviderCodex, RequestedTier: "ultra", ResolvedModel: "gpt-5.6-sol"},
		evidence.Resolution, evidence.ExecEntitlement, evidence.ProbeEntitlement, time.Now())
	assert.Equal(t, StatusUnverified, receipt.VerificationStatus)
	assert.NotEmpty(t, receipt.ResolutionReason)
	assert.True(t, receipt.Complete())
}

func TestInspectFillsBothCodexEntitlements(t *testing.T) {
	t.Parallel()

	supportRoot, localHome := t.TempDir(), t.TempDir()
	probeWriteManagedAuth(t, supportRoot, probeExecAccountID, "pro")
	probeWriteLocalAuth(t, localHome, "pro")
	prober := probeFilesystemProber(t, supportRoot, localHome, probeCodexListing())

	evidence, err := prober.Inspect(context.Background(), ProviderCodex)
	require.NoError(t, err)
	assert.Equal(t, AccountActive, evidence.Resolution.Status)
	assert.Equal(t, probeExecEmail, evidence.Resolution.Account.Email)
	assert.Equal(t, Entitlement{Grade: "pro", Source: probeExecEmail}, evidence.ExecEntitlement)
	assert.Equal(t, Entitlement{Grade: "pro", Source: probeHostEmail}, evidence.ProbeEntitlement)

	verdict, reason := CompareEntitlement(evidence.ExecEntitlement, evidence.ProbeEntitlement)
	assert.Equal(t, VerdictTrusted, verdict)
	assert.NotEmpty(t, reason)
}

// The managed account identifier is an internal value; a receipt naming it
// would publish it to every later reader of the run.
func TestInspectKeepsManagedAccountIDOutOfTheReceipt(t *testing.T) {
	t.Parallel()

	supportRoot, localHome := t.TempDir(), t.TempDir()
	probeWriteManagedAuth(t, supportRoot, probeExecAccountID, "pro")
	probeWriteLocalAuth(t, localHome, "pro")
	prober := probeFilesystemProber(t, supportRoot, localHome, probeCodexListing())

	evidence, err := prober.Inspect(context.Background(), ProviderCodex)
	require.NoError(t, err)
	receipt := Evaluate(
		TierRequest{Provider: ProviderCodex, RequestedTier: "ultra", ResolvedModel: "gpt-5.6-sol"},
		evidence.Resolution, evidence.ExecEntitlement, evidence.ProbeEntitlement, time.Now())
	encoded, err := json.Marshal(receipt)
	require.NoError(t, err)

	assert.Equal(t, StatusVerified, receipt.VerificationStatus)
	assert.NotContains(t, string(encoded), probeExecAccountID)
}

func TestInspectDegradesWhenExecutionCredentialIsMissing(t *testing.T) {
	t.Parallel()

	supportRoot, localHome := t.TempDir(), t.TempDir()
	probeWriteLocalAuth(t, localHome, "pro")
	prober := probeFilesystemProber(t, supportRoot, localHome, probeCodexListing())

	evidence, err := prober.Inspect(context.Background(), ProviderCodex)
	require.NoError(t, err)
	assert.False(t, evidence.ExecEntitlement.Known())
	assert.True(t, evidence.ProbeEntitlement.Known())

	verdict, reason := CompareEntitlement(evidence.ExecEntitlement, evidence.ProbeEntitlement)
	assert.Equal(t, VerdictUnverified, verdict)
	assert.NotEmpty(t, reason)
}

// Claude has no per-account entitlement source, so the credential seams must
// not be consulted and both grades stay unknown without a provider branch.
func TestInspectLeavesClaudeEntitlementsUnknown(t *testing.T) {
	t.Parallel()

	prober := Prober{
		AccountListing: func(context.Context) ([]byte, error) { return probeClaudeListing(), nil },
		Credential: func(string) ([]byte, error) {
			t.Error("claude must not read a codex credential")
			return nil, nil
		},
		HostCredential: func() ([]byte, error) {
			t.Error("claude must not read a codex credential")
			return nil, nil
		},
	}

	evidence, err := prober.Inspect(context.Background(), ProviderClaude)
	require.NoError(t, err)
	assert.Equal(t, AccountSoleRegistered, evidence.Resolution.Status)
	assert.False(t, evidence.ExecEntitlement.Known())
	assert.False(t, evidence.ProbeEntitlement.Known())

	verdict, _ := CompareEntitlement(evidence.ExecEntitlement, evidence.ProbeEntitlement)
	assert.Equal(t, VerdictUnverified, verdict)
}

func TestInspectReportsUnparsableListing(t *testing.T) {
	t.Parallel()

	prober := Prober{
		AccountListing: func(context.Context) ([]byte, error) { return []byte("not json"), nil },
	}

	evidence, err := prober.Inspect(context.Background(), ProviderCodex)
	require.Error(t, err)
	assert.False(t, evidence.Resolution.Determined())
	assert.NotEmpty(t, evidence.Resolution.Reason)
}

// probeFilesystemProber wires the seams to the real path assembly and reader
// against injected roots, so the test exercises the production code path.
func probeFilesystemProber(t *testing.T, supportRoot, localHome string, listing []byte) Prober {
	t.Helper()

	return Prober{
		AccountListing: func(context.Context) ([]byte, error) { return listing, nil },
		Credential: func(accountID string) ([]byte, error) {
			path, err := managedCodexAuthPath(supportRoot, accountID)
			if err != nil {
				return nil, err
			}
			return readCredentialFile(path, executionCredentialScope)
		},
		HostCredential: func() ([]byte, error) {
			path, err := localCodexAuthPath(localHome, "")
			if err != nil {
				return nil, err
			}
			return readCredentialFile(path, probeCredentialScope)
		},
	}
}

func probeCodexListing() []byte {
	return []byte(fmt.Sprintf(`{"ok":true,"result":{"codex":{
		"accounts":[{"id":%q,"email":%q,"organizationUuid":"org-exec"}],
		"activeAccountId":%q,
		"systemDefault":{"hasAuth":true,"email":%q,"providerAccountId":"org-host"}}}}`,
		probeExecAccountID, probeExecEmail, probeExecAccountID, probeHostEmail))
}

func probeClaudeListing() []byte {
	return []byte(fmt.Sprintf(`{"ok":true,"result":{"claude":{
		"accounts":[{"id":%q,"email":%q,"organizationUuid":"org-claude"}],
		"activeAccountId":null}}}`, probeExecAccountID, probeExecEmail))
}

func probeWriteManagedAuth(t *testing.T, supportRoot, accountID, grade string) {
	t.Helper()

	path, err := managedCodexAuthPath(supportRoot, accountID)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, probeAuthPayload(t, grade), 0o600))
}

func probeWriteLocalAuth(t *testing.T, codexHome, grade string) {
	t.Helper()

	path, err := localCodexAuthPath(codexHome, "")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, probeAuthPayload(t, grade), 0o600))
}

// probeAuthPayload builds a credential carrying only the grade claim. The token
// is synthetic: the gate reads a self-reported claim and never verifies it.
func probeAuthPayload(t *testing.T, grade string) []byte {
	t.Helper()

	claims, err := json.Marshal(map[string]any{
		openAIAuthClaim: map[string]any{"chatgpt_plan_type": grade},
	})
	require.NoError(t, err)
	token := "e30." + base64.RawURLEncoding.EncodeToString(claims) + ".signature"
	payload, err := json.Marshal(map[string]any{
		"tokens": map[string]any{"id_token": token},
	})
	require.NoError(t, err)
	return payload
}
