package capture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/evidence"
)

func TestSanitize_RedactsSecretsAndKeepsStructure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	index := validIndex(t, dir)
	index.Steps[0].Console.Messages[0].Text = "auth failed for token=sk-live-abcdefghijklmnopqrst"
	index.Steps[1].FailureSummary = "session file /Users/alice/.config/session.json expired"
	index.Steps[0].Actions[0].TargetRef = "origin:0/checkout?apikey=abcdef123456"

	sanitized := Sanitize(index)
	body, err := json.Marshal(sanitized)
	require.NoError(t, err)

	assert.NotContains(t, string(body), "sk-live-abcdefghijklmnopqrst")
	assert.NotContains(t, string(body), "/Users/alice")
	assert.Contains(t, string(body), evidence.RedactedSecret)
	assert.Contains(t, string(body), evidence.RedactedUser)

	// Structure and counters survive redaction: the projection is still evidence.
	require.Len(t, sanitized.Steps, 2)
	assert.Equal(t, index.Totals, sanitized.Totals)
	assert.Equal(t, RetentionLocalOnly, sanitized.Steps[0].Screenshot.Retention)
	require.NoError(t, Validate(sanitized))
	require.NoError(t, AssertPublishable(sanitized))
}

func TestSanitize_ForcesLocalOnlyRetention(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	index := validIndex(t, dir)
	// A producer claiming its raw screenshot is publishable must not be believed.
	index.Steps[0].Screenshot.Retention = "publishable"
	index.Media[0].Retention = "publishable"

	sanitized := Sanitize(index)
	assert.Equal(t, RetentionLocalOnly, sanitized.Steps[0].Screenshot.Retention)
	assert.Equal(t, RetentionLocalOnly, sanitized.Media[0].Retention)
}

func TestSanitize_ClipsOversizedText(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	index := validIndex(t, dir)
	long := make([]byte, MaxTextLen*3)
	for i := range long {
		long[i] = 'x'
	}
	index.Steps[0].Console.Messages[0].Text = string(long)

	sanitized := Sanitize(index)
	assert.LessOrEqual(t, len(sanitized.Steps[0].Console.Messages[0].Text), MaxTextLen+4)
}

func TestWritePublished_WritesValidatedProjection(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	index := validIndex(t, dir)
	path, err := WritePublished(index, dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, PublishedFileName), path)

	body, err := os.ReadFile(path)
	require.NoError(t, err)
	reloaded, err := DecodeIndex(body)
	require.NoError(t, err)
	require.NoError(t, Validate(reloaded))
	assert.Equal(t, index.Totals, reloaded.Totals)

	// No temporary projection is left behind.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, entry := range entries {
		assert.NotContains(t, entry.Name(), ".capture-index-")
	}
}

func TestVerifyLocalMedia_DetectsMissingAndMismatchedFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	index := validIndex(t, dir)
	assert.Empty(t, VerifyLocalMedia(index, dir))

	require.NoError(t, os.Remove(filepath.Join(dir, "traces", "checkout.zip")))
	findings := VerifyLocalMedia(index, dir)
	require.NotEmpty(t, findings)
	assert.Contains(t, joined(findings), "is missing from the capture directory")

	fresh := t.TempDir()
	other := validIndex(t, fresh)
	other.Steps[0].Screenshot.Bytes = 999999
	findings = VerifyLocalMedia(other, fresh)
	require.NotEmpty(t, findings)
	assert.Contains(t, joined(findings), "declares 999999 bytes")
}

// TestAssertPublishable_GatesRawIndexAndSanitizesOnWrite separates the two
// guarantees: a raw index carrying a local user path is not publishable, while
// WritePublished is allowed to succeed precisely because it redacts first. What
// must never happen is the secret reaching the file.
func TestAssertPublishable_GatesRawIndexAndSanitizesOnWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	index := validIndex(t, dir)
	index.Steps[0].Title = "/Users/alice/secrets"
	require.Error(t, AssertPublishable(index))

	path, err := WritePublished(index, dir)
	require.NoError(t, err)
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "/Users/alice")
	assert.Contains(t, string(body), evidence.RedactedUser)
}
