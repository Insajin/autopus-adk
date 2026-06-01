package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQAProfileCheckCmd_ReportsMissingCapabilities(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(journeyDir, "auth.yaml"), []byte(`id: auth-safe-shell
title: Auth safe shell
surface: browser
lanes: [gui-explore]
adapter:
  id: node-script
command:
  argv: ["npm", "test"]
  cwd: .
  timeout: 60s
checks:
  - id: auth-safe-shell
    type: deterministic
profile_requirements:
  capabilities:
    - auth-state
`), 0o644))

	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"profile", "check", "--project-dir", dir, "--profile", "ci", "--format", "json"})

	require.NoError(t, cmd.Execute())
	payload := decodeJSONMap(t, out.Bytes())
	assertCommonJSONEnvelope(t, payload, "qa profile check")
	assert.Equal(t, "warn", payload["status"])
	data := payload["data"].(map[string]any)
	assert.Equal(t, "qamesh.profile_check.v1", data["schema_version"])
	assert.Equal(t, "setup_gap", data["status"])
	assert.Contains(t, stringSlice(data["missing_capabilities"]), "auth-state")
	assert.Contains(t, stringSlice(data["next_commands"]), "auto qa profile check --project-dir "+shellWord(dir)+" --profile local --format json")
}

func TestQAProfileCmd_IsRegisteredUnderQA(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()
	profileCmd, _, err := root.Find([]string{"qa", "profile", "check"})
	require.NoError(t, err)
	require.NotNil(t, profileCmd)
}
