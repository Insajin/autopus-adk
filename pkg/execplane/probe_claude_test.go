package execplane

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	probeClaudeAccountID = "22222222-2222-4222-8222-222222222222"
	probeClaudeExecEmail = "claude-exec@example.test"
	probeClaudeHostEmail = "claude-host@example.test"
	probeClaudeGrade     = "claude_max"
)

// Claude records the grade in the account record rather than in a token, and
// the managed copy and the host configuration keep it at different depths.
// Both must reach the same grade.
func TestInspectFillsBothClaudeEntitlements(t *testing.T) {
	t.Parallel()

	supportRoot, configDir := t.TempDir(), t.TempDir()
	probeWriteManagedClaudeAuth(t, supportRoot, probeClaudeAccountID, probeClaudeGrade)
	probeWriteHostClaudeConfig(t, configDir, probeClaudeGrade)
	prober := probeClaudeFilesystemProber(t, supportRoot, configDir)

	evidence, err := prober.Inspect(context.Background(), ProviderClaude)
	require.NoError(t, err)
	assert.Equal(t, AccountSoleRegistered, evidence.Resolution.Status)
	assert.Equal(t, Entitlement{Grade: probeClaudeGrade, Source: probeClaudeExecEmail},
		evidence.ExecEntitlement)
	// The claude roster carries no host identity, so the address comes from the
	// configuration that carried the grade rather than from a scope label.
	assert.Equal(t, Entitlement{Grade: probeClaudeGrade, Source: probeClaudeHostEmail},
		evidence.ProbeEntitlement)

	verdict, reason := CompareEntitlement(evidence.ExecEntitlement, evidence.ProbeEntitlement)
	assert.Equal(t, VerdictTrusted, verdict)
	assert.NotEmpty(t, reason)
}

// Two claude accounts on different plans are the case the gate exists for: the
// held catalog is not evidence for the account that will run.
func TestInspectReportsClaudeGradeDifference(t *testing.T) {
	t.Parallel()

	supportRoot, configDir := t.TempDir(), t.TempDir()
	probeWriteManagedClaudeAuth(t, supportRoot, probeClaudeAccountID, "claude_pro")
	probeWriteHostClaudeConfig(t, configDir, probeClaudeGrade)
	prober := probeClaudeFilesystemProber(t, supportRoot, configDir)

	evidence, err := prober.Inspect(context.Background(), ProviderClaude)
	require.NoError(t, err)
	assert.Equal(t, "claude_pro", evidence.ExecEntitlement.Grade)
	assert.Equal(t, probeClaudeGrade, evidence.ProbeEntitlement.Grade)

	verdict, reason := CompareEntitlement(evidence.ExecEntitlement, evidence.ProbeEntitlement)
	assert.Equal(t, VerdictReprobe, verdict)
	assert.NotEmpty(t, reason)
}

// A machine with no claude credential is an ordinary state, not a crash: both
// grades stay unknown and the comparison says why.
func TestInspectDegradesWhenClaudeCredentialIsMissing(t *testing.T) {
	t.Parallel()

	prober := probeClaudeFilesystemProber(t, t.TempDir(), t.TempDir())

	evidence, err := prober.Inspect(context.Background(), ProviderClaude)
	require.NoError(t, err)
	assert.False(t, evidence.ExecEntitlement.Known())
	assert.False(t, evidence.ProbeEntitlement.Known())

	verdict, reason := CompareEntitlement(evidence.ExecEntitlement, evidence.ProbeEntitlement)
	assert.Equal(t, VerdictUnverified, verdict)
	assert.NotEmpty(t, reason)
}

// A credential that is present but malformed degrades exactly like a missing
// one: Inspect returns zero entitlements instead of failing the run.
func TestInspectDegradesWhenClaudeCredentialIsMalformed(t *testing.T) {
	t.Parallel()

	supportRoot, configDir := t.TempDir(), t.TempDir()
	path, err := managedClaudeAuthPath(supportRoot, probeClaudeAccountID)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o600))
	probeWriteHostClaudeConfig(t, configDir, probeClaudeGrade)

	evidence, err := probeClaudeFilesystemProber(t, supportRoot, configDir).
		Inspect(context.Background(), ProviderClaude)
	require.NoError(t, err)
	assert.False(t, evidence.ExecEntitlement.Known())
	assert.True(t, evidence.ProbeEntitlement.Known())

	verdict, reason := CompareEntitlement(evidence.ExecEntitlement, evidence.ProbeEntitlement)
	assert.Equal(t, VerdictUnverified, verdict)
	assert.NotEmpty(t, reason)
}

// CLAUDE_CONFIG_DIR relocates the CLI's configuration directory, so it is
// where the account record is looked for first.
func TestReadLocalClaudeConfigHonorsConfigDir(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv(claudeConfigDirEnv, configDir)
	probeWriteHostClaudeConfig(t, configDir, probeClaudeGrade)

	payload, err := readLocalClaudeConfig()
	require.NoError(t, err)
	entitlement, err := ParseClaudeEntitlement(payload, claudeProbeScope)
	require.NoError(t, err)
	assert.Equal(t, probeClaudeGrade, entitlement.Grade)
	assert.Equal(t, probeClaudeHostEmail, claudeCredentialEmail(payload))
}

// A relocated configuration directory does not always carry the account
// record, and the home copy is the same source rather than a second one.
func TestReadLocalClaudeConfigFallsBackToTheHomeCopy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(claudeConfigDirEnv, t.TempDir())
	probeWriteHostClaudeConfig(t, home, probeClaudeGrade)

	payload, err := readLocalClaudeConfig()
	require.NoError(t, err)
	entitlement, err := ParseClaudeEntitlement(payload, claudeProbeScope)
	require.NoError(t, err)
	assert.Equal(t, probeClaudeGrade, entitlement.Grade)
}

func TestReadLocalClaudeConfigReportsMissingConfiguration(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(claudeConfigDirEnv, t.TempDir())

	payload, err := readLocalClaudeConfig()
	require.ErrorIs(t, err, ErrCredentialUnavailable)
	assert.Nil(t, payload)
	assert.Contains(t, err.Error(), claudeProbeScope)
}

// probeClaudeFilesystemProber wires the seams to the real path assembly and
// reader against injected roots, so the test exercises the production path.
func probeClaudeFilesystemProber(t *testing.T, supportRoot, configDir string) Prober {
	t.Helper()

	return Prober{
		AccountListing: func(context.Context) ([]byte, error) { return probeClaudeListing(), nil },
		Credential: func(provider, accountID string) ([]byte, error) {
			assert.Equal(t, ProviderClaude, provider)
			path, err := managedClaudeAuthPath(supportRoot, accountID)
			if err != nil {
				return nil, err
			}
			return readCredentialFile(path, claudeExecutionScope, maxCredentialBytes)
		},
		HostCredential: func(provider string) ([]byte, error) {
			assert.Equal(t, ProviderClaude, provider)
			path, err := localClaudeConfigPath(configDir, "")
			if err != nil {
				return nil, err
			}
			return readCredentialFile(path, claudeProbeScope, maxHostConfigBytes)
		},
	}
}

// probeClaudeListing mirrors an orca roster for claude, which reports no
// systemDefault: the host identity is not in the roster for this provider.
func probeClaudeListing() []byte {
	return []byte(fmt.Sprintf(`{"ok":true,"result":{"claude":{
		"accounts":[{"id":%q,"email":%q,"organizationUuid":"org-claude"}],
		"activeAccountId":null}}}`, probeClaudeAccountID, probeClaudeExecEmail))
}

// probeWriteManagedClaudeAuth writes the shape orca keeps for a managed
// account: the account record at the top level of the credential.
func probeWriteManagedClaudeAuth(t *testing.T, supportRoot, accountID, grade string) {
	t.Helper()

	path, err := managedClaudeAuthPath(supportRoot, accountID)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path,
		probeClaudeRecord(t, probeClaudeExecEmail, grade), 0o600))
}

// probeWriteHostClaudeConfig writes the shape the host CLI keeps: the same
// record nested under `oauthAccount`, beside unrelated configuration.
func probeWriteHostClaudeConfig(t *testing.T, configDir, grade string) {
	t.Helper()

	path, err := localClaudeConfigPath(configDir, "")
	require.NoError(t, err)
	record := map[string]any{}
	require.NoError(t, json.Unmarshal(probeClaudeRecord(t, probeClaudeHostEmail, grade), &record))
	payload, err := json.Marshal(map[string]any{
		"numStartups":  3,
		"oauthAccount": record,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, payload, 0o600))
}

// probeClaudeRecord builds a synthetic account record. The identifiers in it
// are fixtures: the gate reads the plan and the address and nothing else.
func probeClaudeRecord(t *testing.T, email, grade string) []byte {
	t.Helper()

	payload, err := json.Marshal(map[string]any{
		"emailAddress":              email,
		"organizationType":          grade,
		"organizationUuid":          "00000000-0000-4000-8000-00000000000f",
		"accountUuid":               "00000000-0000-4000-8000-00000000000e",
		"organizationRateLimitTier": "default_claude_max_20x",
		"billingType":               "subscription",
	})
	require.NoError(t, err)
	return payload
}
