package execplane_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/insajin/autopus-adk/pkg/execplane"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseClaudeEntitlementReadsBothCredentialLayouts covers S3 / REQ-004:
// the orca-managed credential stores the entitlement fields at the top level
// and the host CLI nests the same field names under `oauthAccount`. Both are
// the same account on the same plan, so a parser that understands only one
// layout would report the other as unknown and force a re-probe that has
// nothing to re-probe.
func TestParseClaudeEntitlementReadsBothCredentialLayouts(t *testing.T) {
	t.Parallel()

	account := fixtureClaudeAccount("claude_max", "default_claude_max_20x")

	managed, err := execplane.ParseClaudeEntitlement(
		fixtureClaudeCredential(t, claudeManagedShape, account), "managed@example.test")
	require.NoError(t, err)
	host, err := execplane.ParseClaudeEntitlement(
		fixtureClaudeCredential(t, claudeHostShape, account), "host-cli@example.test")
	require.NoError(t, err)

	assert.Equal(t, "claude_max", managed.Grade)
	assert.Equal(t, managed.Grade, host.Grade,
		"the same account read from two layouts must land on one grade")
	assert.True(t, managed.Known())
	assert.True(t, host.Known())

	// The source labels stay distinct, so the receipt can still name which
	// credential each side of the comparison came from.
	assert.Equal(t, "managed@example.test", managed.Source)
	assert.Equal(t, "host-cli@example.test", host.Source)

	verdict, reason := execplane.CompareEntitlement(managed, host)
	assert.Equal(t, execplane.VerdictTrusted, verdict)
	assert.True(t, mentionsGrade(reason, "claude_max"), "reason must name the grade: %q", reason)
}

// TestParseClaudeEntitlementIgnoresRateLimitTier covers S3 / REQ-004 and the
// premise S10 fixes: `organizationRateLimitTier` throttles how much of the
// same model set an account may consume, so folding it into the grade would
// declare two equivalent accounts unequal and demand a re-probe that could not
// change any answer.
func TestParseClaudeEntitlementIgnoresRateLimitTier(t *testing.T) {
	t.Parallel()

	twentyX, err := execplane.ParseClaudeEntitlement(
		fixtureClaudeCredential(t, claudeHostShape,
			fixtureClaudeAccount("claude_max", "default_claude_max_20x")),
		"host-cli@example.test")
	require.NoError(t, err)
	fiveX, err := execplane.ParseClaudeEntitlement(
		fixtureClaudeCredential(t, claudeManagedShape,
			fixtureClaudeAccount("claude_max", "default_claude_max_5x")),
		"managed@example.test")
	require.NoError(t, err)

	assert.Equal(t, "claude_max", twentyX.Grade)
	assert.Equal(t, twentyX.Grade, fiveX.Grade,
		"a capacity tier is not a plan; 5x and 20x are the same entitlement")

	verdict, _ := execplane.CompareEntitlement(fiveX, twentyX)
	assert.Equal(t, execplane.VerdictTrusted, verdict,
		"a rate-limit difference must not cost a re-probe")
}

// TestParseClaudeEntitlementRejectsUnusableCredentials covers S8 / REQ-009: a
// credential with no plan claim yields an error and an unknown entitlement
// rather than an assumed grade. Assuming one here would fabricate exactly the
// equality REQ-004 exists to check.
func TestParseClaudeEntitlementRejectsUnusableCredentials(t *testing.T) {
	t.Parallel()

	missing := fixtureClaudeAccount("claude_max", "default_claude_max_20x")
	delete(missing, "organizationType")

	cases := map[string]struct {
		payload      []byte
		wantSentinel bool
	}{
		"managed credential omits the plan field": {
			payload:      fixtureClaudeCredential(t, claudeManagedShape, missing),
			wantSentinel: true,
		},
		"host credential omits the plan field": {
			payload:      fixtureClaudeCredential(t, claudeHostShape, missing),
			wantSentinel: true,
		},
		"plan field is empty": {
			payload: fixtureClaudeCredential(t, claudeHostShape,
				fixtureClaudeAccount("", "default_claude_max_20x")),
			wantSentinel: true,
		},
		"host credential carries no oauth account": {
			payload:      []byte(`{"numStartups":7,"oauthAccount":null}`),
			wantSentinel: true,
		},
		"credential is an empty document": {
			payload:      []byte(`{}`),
			wantSentinel: true,
		},
		"credential is not json": {
			payload: []byte(`{"oauthAccount":`),
		},
	}

	for name, tc := range cases {
		entitlement, err := execplane.ParseClaudeEntitlement(tc.payload, "host-cli@example.test")

		require.Error(t, err, name)
		if tc.wantSentinel {
			assert.True(t, errors.Is(err, execplane.ErrNoEntitlementClaim), name)
		}
		assert.False(t, entitlement.Known(), "%s: a failed parse must not report a grade", name)
	}
}

// TestParseClaudeEntitlementDropsAccountIdentifiers covers the redaction rule
// the gate runs under: the plan grade and the caller's own source label are
// the only things allowed out. The organization and account UUIDs sit in the
// very object the grade is read from, so copying the struct wholesale — the
// obvious shortcut — would push them into every receipt and log line.
func TestParseClaudeEntitlementDropsAccountIdentifiers(t *testing.T) {
	t.Parallel()

	entitlement, err := execplane.ParseClaudeEntitlement(
		fixtureClaudeCredential(t, claudeHostShape,
			fixtureClaudeAccount("claude_max", "default_claude_max_20x")),
		"host-cli@example.test")
	require.NoError(t, err)

	encoded, err := json.Marshal(entitlement)
	require.NoError(t, err)

	assert.NotContains(t, string(encoded), fixtureOrgUUIDClaim)
	assert.NotContains(t, string(encoded), fixtureAccountUUIDClaim)
	assert.NotContains(t, string(encoded), "claude-owner@example.test",
		"the credential's own email is not the source label the caller chose")
}

// TestClaudeAndCodexGradesNeverCompareEqual covers S3 / REQ-004: the two
// providers name their plans in disjoint vocabularies, so a Codex `pro` and a
// Claude `claude_max` are different grades and their comparison demands a
// re-probe. An implementation that normalized grades to a shared ladder would
// silently declare a Codex catalog evidence for a Claude execution account.
func TestClaudeAndCodexGradesNeverCompareEqual(t *testing.T) {
	t.Parallel()

	codex, err := execplane.ParseCodexEntitlement(
		fixtureGradedAuth(t, "pro"), "codex-host@example.test")
	require.NoError(t, err)
	claude, err := execplane.ParseClaudeEntitlement(
		fixtureClaudeCredential(t, claudeHostShape,
			fixtureClaudeAccount("claude_max", "default_claude_max_20x")),
		"claude-host@example.test")
	require.NoError(t, err)

	require.NotEqual(t, codex.Grade, claude.Grade)

	verdict, reason := execplane.CompareEntitlement(claude, codex)
	assert.Equal(t, execplane.VerdictReprobe, verdict)
	assert.True(t, mentionsGrade(reason, "claude_max"), "reason must name both grades: %q", reason)
	assert.True(t, mentionsGrade(reason, "pro"), "reason must name both grades: %q", reason)
}
