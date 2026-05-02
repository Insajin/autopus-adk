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

func TestFindUnsafeText_FindsProviderBoundLeaks(t *testing.T) {
	t.Parallel()

	findings := FindUnsafeText("Authorization: Bearer sk-proj-qameshfake1234567890", "raw")

	require.NotEmpty(t, findings)
	assert.Equal(t, "secret", findings[0].Type)
	assert.NotContains(t, FormatFindings(findings), "qameshfake")
}
