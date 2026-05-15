package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQAReleaseCmd_IsRegisteredUnderQA(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()
	releaseCmd, _, err := root.Find([]string{"qa", "release"})
	require.NoError(t, err)
	require.NotNil(t, releaseCmd)
}

func TestQAReleaseCmd_InvalidProfileHasNoOutputSideEffects(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	output := filepath.Join(dir, ".autopus", "qa", "releases")
	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"qa", "release", "--project-dir", dir, "--output", output, "--profile", "invalid", "--format", "json"})

	require.Error(t, cmd.Execute())
	payload := decodeJSONMap(t, out.Bytes())
	assertCommonJSONEnvelope(t, payload, "auto qa release")
	assert.Equal(t, "error", payload["status"])
	assert.Equal(t, "qa_release_invalid_profile", payload["error"].(map[string]any)["code"])
	assert.NoDirExists(t, output)
}

func TestQAReleaseCmd_DryRunAndRoadmapUseJSONEnvelope(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for _, args := range [][]string{
		{"qa", "release", "--project-dir", dir, "--dry-run", "--format", "json"},
		{"qa", "release", "--project-dir", dir, "--roadmap", "--format", "json"},
	} {
		cmd := NewRootCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs(args)

		require.NoError(t, cmd.Execute())
		payload := decodeJSONMap(t, out.Bytes())
		assertCommonJSONEnvelope(t, payload, "auto qa release")
		assert.Equal(t, "ok", payload["status"])
		data := payload["data"].(map[string]any)
		assert.Contains(t, data["schema_version"], "qamesh.release_")
	}
}
