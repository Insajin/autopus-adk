package evidence

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactText_MasksSecretsPathsAndPrivateNotes(t *testing.T) {
	t.Parallel()

	raw := "token=sk-proj-qameshfake1234567890\nsession=secret-cookie\npath=/Users/alice/private/notes.md\nprivate_note_body=customer note"

	redacted := RedactText(raw)

	require.NoError(t, AssertSafeText(redacted, "redacted"))
	assert.Contains(t, redacted, RedactedSecret)
	assert.Contains(t, redacted, "/Users/[REDACTED_USER]/private/notes.md")
	assert.Contains(t, redacted, RedactedPrivateNote)
	assert.NotContains(t, redacted, "sk-proj-qameshfake1234567890")
	assert.NotContains(t, redacted, "secret-cookie")
	assert.NotContains(t, redacted, "alice")
	assert.NotContains(t, redacted, "customer note")
}

func TestRedactText_MasksJSONSensitiveValuesAndCrossPlatformPaths(t *testing.T) {
	t.Parallel()

	raw := `{"access_token":"sk-proj-qameshfake1234567890","cookie":"session=secret-cookie","authorization":"Bearer sk-proj-qameshfake1234567890","localNoteBody":"customer note","linux":"/home/alice/private/notes.md","windows":"C:\\Users\\alice\\private\\notes.md"}`

	redacted := RedactText(raw)

	require.NoError(t, AssertSafeText(redacted, "json"))
	assert.Contains(t, redacted, RedactedSecret)
	assert.Contains(t, redacted, RedactedPrivateNote)
	assert.Contains(t, redacted, "/home/[REDACTED_USER]/private/notes.md")
	assert.Contains(t, redacted, `C:\\Users\\[REDACTED_USER]\\private\\notes.md`)
	assert.NotContains(t, redacted, "qameshfake")
	assert.NotContains(t, redacted, "alice")
	assert.NotContains(t, redacted, "customer note")
}

func TestRedactText_MasksCLIFlagValuesAndCredentialURLs(t *testing.T) {
	t.Parallel()

	raw := "go test ./... --password 'hunter two' --api-key=abc12345 --report https://user:pass@example.test/out?token=tok12345"

	redacted := RedactText(raw)

	require.NoError(t, AssertSafeText(redacted, "command"))
	assert.Contains(t, redacted, RedactedSecret)
	assert.NotContains(t, redacted, "hunter")
	assert.NotContains(t, redacted, "abc12345")
	assert.NotContains(t, redacted, "user:pass@")
	assert.NotContains(t, redacted, "tok12345")
}

func TestFindUnsafeText_FindsProviderBoundLeaks(t *testing.T) {
	t.Parallel()

	findings := FindUnsafeText("Authorization: Bearer sk-proj-qameshfake1234567890", "raw")

	require.NotEmpty(t, findings)
	assert.Equal(t, "secret", findings[0].Type)
	assert.NotContains(t, FormatFindings(findings), "qameshfake")
}

func TestFindUnsafeText_FindsSecretQueryParams(t *testing.T) {
	t.Parallel()

	findings := FindUnsafeText("https://example.test/out?token=tok12345", "url")

	require.NotEmpty(t, findings)
	assert.Contains(t, findingTypes(findings), "sensitive_query")
	assert.NotContains(t, FormatFindings(findings), "tok12345")
}

func TestFindUnsafeText_IgnoresHyphenatedWordsContainingFlagNames(t *testing.T) {
	t.Parallel()

	findings := FindUnsafeText("review failure pattern tie-token should stay searchable", "review")

	assert.Empty(t, findings)
}

func findingTypes(findings []Finding) []string {
	out := make([]string, 0, len(findings))
	for _, finding := range findings {
		out = append(out, finding.Type)
	}
	return out
}
