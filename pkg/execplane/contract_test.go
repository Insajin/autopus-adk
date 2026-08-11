// Package execplane_test locks the tier-integrity gate to the acceptance
// oracles in .autopus/specs/SPEC-EXECPLANE-001/acceptance.md. Each test names
// the scenario and requirement it pins.
//
// Every fixture here is synthesized. No real credential, token, or account UUID
// belongs in the repository, and the scenarios turn on payload shape and
// entitlement grade rather than on this workstation's account strings — an
// oracle that hardcodes accounts stops meaning anything the moment the roster
// changes (acceptance.md S3, S9).
package execplane_test

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/require"
)

// openAIAuthClaimKey is spelled out instead of imported from the production
// constant, so renaming the claim the gate reads fails a test rather than
// silently moving both sides together.
const openAIAuthClaimKey = "https://api.openai.com/auth"

// fixtureAccountEntry builds one `result.<provider>.accounts[]` member of an
// `orca account list --json` payload.
func fixtureAccountEntry(id, email, org string) map[string]any {
	return map[string]any{
		"id":               id,
		"email":            email,
		"organizationUuid": org,
	}
}

// fixtureProviderBlock builds one `result.<provider>` block. Passing nil for
// active reproduces the `"activeAccountId": null` orca reports when no account
// is selected; an empty systemDefaultEmail omits the host CLI identity the way
// orca's claude block does.
func fixtureProviderBlock(active any, systemDefaultEmail string, accounts ...map[string]any) map[string]any {
	entries := make([]map[string]any, 0, len(accounts))
	entries = append(entries, accounts...)
	block := map[string]any{
		"accounts":        entries,
		"activeAccountId": active,
	}
	if systemDefaultEmail != "" {
		block["systemDefault"] = map[string]any{
			"hasAuth":           true,
			"email":             systemDefaultEmail,
			"providerAccountId": "org-host-cli-fixture",
		}
	}
	return block
}

// fixtureListing renders a successful `orca account list --json` payload.
func fixtureListing(t *testing.T, blocks map[string]any) []byte {
	t.Helper()

	payload, err := json.Marshal(map[string]any{"ok": true, "result": blocks})
	require.NoError(t, err)
	return payload
}

// fixtureCodexAuth assembles a Codex auth.json around the given id_token
// claims. The three JWT segments are built here on purpose: storing a real
// token would leak an account, and the gate never verifies the signature — it
// only reads a self-reported grade — so an arbitrary third segment suffices.
func fixtureCodexAuth(t *testing.T, claims map[string]any) []byte {
	t.Helper()

	body, err := json.Marshal(claims)
	require.NoError(t, err)
	token := strings.Join([]string{
		base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`)),
		base64.RawURLEncoding.EncodeToString(body),
		"signature-is-never-verified",
	}, ".")

	auth, err := json.Marshal(map[string]any{
		"tokens": map[string]any{"id_token": token},
	})
	require.NoError(t, err)
	return auth
}

// fixtureAccountIDClaim is the account identifier that rides along in the same
// claim as the grade. Tests assert it never reaches the parser's output.
const fixtureAccountIDClaim = "fixture-chatgpt-account-id"

// fixtureGradedAuth is the common credential: one account reporting one grade.
func fixtureGradedAuth(t *testing.T, grade string) []byte {
	t.Helper()

	return fixtureCodexAuth(t, map[string]any{
		"sub": "fixture-subject",
		openAIAuthClaimKey: map[string]any{
			"chatgpt_plan_type":  grade,
			"chatgpt_account_id": fixtureAccountIDClaim,
		},
	})
}

// mentionsGrade reports whether reason names grade as a whole word. Plain
// substring matching would accept the word "probe" as evidence that the grade
// "pro" was recorded, hiding a reason that only ever names one side of the
// comparison.
func mentionsGrade(reason, grade string) bool {
	isSeparator := func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_'
	}
	for _, token := range strings.FieldsFunc(reason, isSeparator) {
		if token == grade {
			return true
		}
	}
	return false
}
