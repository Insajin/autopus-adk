package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunPlaywright_ProcessFailure_ReturnsBoundedSanitizedStderr(t *testing.T) {
	// Given
	dir := t.TempDir()
	npxPath := filepath.Join(dir, "npx")
	stderrPath := filepath.Join(dir, "stderr.txt")
	diagnostic := "\x1b[31mreporter failed at /Users/example/private-workspace/e2e.spec.ts API_TOKEN=example-redaction-sentinel\x1b[0m\n"
	stderrFixture := diagnostic + strings.Repeat("0123456789abcdef", maxPlaywrightStderrBytes/16+1024)
	require.NoError(t, os.WriteFile(stderrPath, []byte(stderrFixture), 0o600))

	script := `#!/bin/sh
printf '{"version":2,"nonce":"%s","playwright_version":"1.59.1","update_snapshots":"none","projects":[{"name":"chromium","ignore_snapshots":false,"state":"enabled","source":"public"}]}' "$AUTOPUS_PLAYWRIGHT_SNAPSHOT_PROOF_NONCE" > "$AUTOPUS_PLAYWRIGHT_SNAPSHOT_PROOF_FILE"
printf '{"suites":[]}'
/bin/cat "$VERIFY_STDERR_FIXTURE" >&2
exit 7
`
	require.NoError(t, os.WriteFile(npxPath, []byte(script), 0o755))
	t.Setenv("VERIFY_STDERR_FIXTURE", stderrPath)
	t.Setenv("PATH", dir)

	// When
	_, err := runPlaywright("desktop")

	// Then
	require.Error(t, err)
	message := err.Error()
	assert.Contains(t, message, "reporter failed")
	assert.False(t, strings.Contains(message, "\x1b"), "ANSI control sequences must be stripped")
	assert.False(t, strings.Contains(message, "/Users/example/private-workspace"), "absolute private paths must be redacted")
	assert.False(t, strings.Contains(message, "example-redaction-sentinel"), "credential-like values must be redacted")
	assert.LessOrEqual(t, len(message), 32<<10, "stderr diagnostics must stay reviewable")
}

func TestSanitizePlaywrightStderr_CredentialsAndAbsolutePaths_RedactsEveryPublicDiagnostic(t *testing.T) {
	t.Parallel()

	// Given
	raw := []byte("Authorization: Bearer auth-value\nAuthorization: Basic YmFzaWMtc2VjcmV0\nAuthorization=\"Bearer quoted-secret\"\nAuthorization: Digest username=alice, response=digest-secret\nBearer standalone-value\nDB_PASSWORD=hunter2\nCLIENT_SECRET=shh\nC:\\Users\\alice\\private.spec.ts\n\\\\server\\share\\private.spec.ts\n/home/alice/private.spec.ts\x00")

	// When
	diagnostic := sanitizePlaywrightStderr(raw)

	// Then
	for _, private := range []string{"auth-value", "YmFzaWMtc2VjcmV0", "quoted-secret", "digest-secret", "standalone-value", "hunter2", "shh", `C:\Users\alice`, `\\server\share`, "/home/alice", "\x00"} {
		assert.NotContains(t, diagnostic, private)
	}
	assert.Contains(t, diagnostic, "[REDACTED]")
	assert.Contains(t, diagnostic, "[REDACTED_PATH]")
}

func TestPlaywrightProcessError_RedactsRenderedCauseAndPreservesUnwrap(t *testing.T) {
	t.Parallel()

	// Given
	cause := &os.PathError{Op: "fork/exec", Path: "/Users/alice/private/npx", Err: syscall.ENOEXEC}

	// When
	err := playwrightProcessError(cause, nil)

	// Then
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "/Users/alice/private")
	assert.Contains(t, err.Error(), "[REDACTED_PATH]")
	assert.True(t, errors.Is(err, cause), "the original process error must remain discoverable")
	assert.True(t, errors.Is(err, syscall.ENOEXEC), "the original errno must remain discoverable")
}

func TestPublicPlaywrightError_RedactsReportDiagnostic(t *testing.T) {
	t.Parallel()

	err := errors.New(`playwright failed at \\server\share\private.spec.ts Authorization: Basic c2VjcmV0`)
	diagnostic := publicPlaywrightError(err)

	assert.NotContains(t, diagnostic, `\\server\share`)
	assert.NotContains(t, diagnostic, "c2VjcmV0")
	assert.Contains(t, diagnostic, "[REDACTED]")
}
